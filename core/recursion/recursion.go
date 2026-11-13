// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package recursion implements parallel (agg) and sequential (seq) task-level recursion.
package recursion

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sync"

	"sculpt/core/context"
	"sculpt/core/ledger"
	"sculpt/core/prompts"
	"sculpt/core/proposer"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/internal/config"
)

const stageAttempts = 2

var notesRE = regexp.MustCompile(`(?s)<notes>(.*?)</notes>`)

// Stage is one improvement stage including retries.
type Stage struct {
	Label           string    `json:"label"`
	Dim             string    `json:"dim"`
	Tree            tree.Tree `json:"-"`
	KeptBase        bool      `json:"kept_base"`
	FellBack        bool      `json:"fell_back"`
	Error           string    `json:"error"`
	USD             float64   `json:"usd"`
	Attempts        int       `json:"attempts"`
	Cached          bool      `json:"cached"`
	Notes           string    `json:"notes"`
	ExecutionFailed bool      `json:"execution_failed"`
}

// ModeResult is the outcome of one recursion mode on one task.
type ModeResult struct {
	Mode       string                    `json:"mode"`
	TaskID     string                    `json:"task_id"`
	K          int                       `json:"k"`
	Stages     []Stage                   `json:"stages"`
	Trees      map[string]tree.Tree      `json:"-"`
	Rollouts   map[string]spec.RunView   `json:"-"`
	Finals     []string                  `json:"finals"`
	Ledger     *ledger.Ledger            `json:"-"`
	Similarity map[string]any            `json:"similarity"`
	Errors     []string                  `json:"errors"`
}

// OK reports whether finals exist without errors/execution failures.
func (m *ModeResult) OK() bool {
	if len(m.Finals) == 0 || len(m.Errors) > 0 {
		return false
	}
	for _, s := range m.Stages {
		if s.ExecutionFailed {
			return false
		}
	}
	return true
}

// ToDict serialises for summary.json.
func (m *ModeResult) ToDict() map[string]any {
	fell, kept := []string{}, []string{}
	stages := make([]map[string]any, 0, len(m.Stages))
	for _, s := range m.Stages {
		if s.FellBack {
			fell = append(fell, s.Label)
		}
		if s.KeptBase {
			kept = append(kept, s.Label)
		}
		err := s.Error
		if len(err) > 400 {
			err = err[:400]
		}
		stages = append(stages, map[string]any{
			"label": s.Label, "dim": s.Dim, "fell_back": s.FellBack,
			"kept_base": s.KeptBase, "execution_failed": s.ExecutionFailed,
			"attempts": s.Attempts, "usd": s.USD, "cached": s.Cached,
			"notes": s.Notes, "error": err,
		})
	}
	var led any
	if m.Ledger != nil {
		led = m.Ledger.ToDict()
	}
	return map[string]any{
		"mode": m.Mode, "task_id": m.TaskID, "k": m.K, "finals": m.Finals,
		"ok": m.OK(), "fell_back": fell, "kept_base": kept, "stages": stages,
		"similarity": m.Similarity, "ledger": led, "errors": m.Errors,
	}
}

func acceptTree(surface tree.SurfaceSpec, adapter spec.BenchmarkAdapter, neu, fallback tree.Tree) (kept bool, errMsg string) {
	_ = tree.Autotrigger(surface, neu, 0)
	chg := tree.DiffSummary(surface, fallback, neu)
	kept = len(chg["added"].([]string)) == 0 && len(chg["removed"].([]string)) == 0 && len(chg["modified"].([]string)) == 0
	reserved := map[string]struct{}{}
	for k := range tree.ReservedScripts {
		reserved[k] = struct{}{}
	}
	for k := range adapter.Spec().ReservedScripts {
		reserved[k] = struct{}{}
	}
	if _, err := tree.Validate(surface, neu, adapter.Spec().AllowSubagents, 0, reserved); err != nil {
		return kept, err.Error()
	}
	if err := adapter.ValidateTree(neu); err != nil {
		return kept, err.Error()
	}
	if err := context.AssertTreeHasNoGraderVerdicts(neu); err != nil {
		return kept, err.Error()
	}
	return kept, ""
}

// RunStage is the exported Propose–validate stage with one invalid-proposal retry.
func RunStage(
	label, dim, job string,
	fallback tree.Tree,
	adapter spec.BenchmarkAdapter,
	prop proposer.Caller,
	ctxDir, taskID string,
	rollouts []spec.RunView,
	harnessDir, libraryDir string,
	pool []context.PoolItem,
	decisions map[string]string,
) Stage {
	return runStage(label, dim, job, fallback, adapter, prop, ctxDir, taskID, rollouts, harnessDir, libraryDir, pool, decisions)
}

func runStage(
	label, dim, job string,
	fallback tree.Tree,
	adapter spec.BenchmarkAdapter,
	prop proposer.Caller,
	ctxDir, taskID string,
	rollouts []spec.RunView,
	harnessDir, libraryDir string,
	pool []context.PoolItem,
	decisions map[string]string,
) Stage {
	surface := adapter.Substrate().Surface()
	built, err := context.Build(filepath.Join(ctxDir, label), adapter, taskID, rollouts, harnessDir, libraryDir, pool, decisions)
	if err != nil {
		return Stage{Label: label, Dim: dim, Tree: tree.Clone(fallback), FellBack: true,
			Error: fmt.Sprintf("could not build evidence: %v", err)}
	}
	mission := context.Mission(built, job)
	errMsg := ""
	spentUSD := 0.0
	spentCalls := 0
	allCached := true
	execFailed := false
	for attempt := 1; attempt <= stageAttempts; attempt++ {
		msg := mission
		if errMsg != "" {
			msg += "\n\n# YOUR PREVIOUS ATTEMPT WAS REJECTED\n\n" + errMsg +
				"\n\nFix it in `context/harness/` and reply again."
		}
		reply, callErr := prop.Call(msg, built.Root, fmt.Sprintf("%s.a%d", label, attempt), "context/harness", config.TransportAttempts)
		if callErr != nil {
			if pe, ok := callErr.(*proposer.Error); ok {
				errMsg = pe.Error()
				spentCalls += pe.Attempts
				if pe.Attempts == 0 {
					spentCalls += config.TransportAttempts
				}
				spentUSD += pe.CostUSD
			} else {
				errMsg = callErr.Error()
				spentCalls += config.TransportAttempts
			}
			execFailed = true
			allCached = false
			break
		}
		spentUSD += reply.CostUSD
		if reply.Cached {
			// no extra attempts
		} else if reply.Attempts > 0 {
			spentCalls += reply.Attempts
		} else {
			spentCalls++
		}
		allCached = allCached && reply.Cached
		if reply.Workspace == "" {
			errMsg = "the harness directory did not come back from the session; edit the files in `context/harness/` in place"
			continue
		}
		var rejected []string
		neu := tree.ReadTree(surface, reply.Workspace, &rejected)
		if len(neu) == 0 {
			errMsg = "`context/harness/` came back empty -- edit the files, do not delete them all"
			continue
		}
		if len(rejected) > 0 {
			errMsg = "these files are off the editable surface and were not applied: " + joinMax(rejected, 6)
			_, _ = tree.WriteTree(surface, neu, filepath.Join(built.Root, "harness"), false)
			continue
		}
		kept, e := acceptTree(surface, adapter, neu, fallback)
		if e != "" {
			errMsg = e
			_, _ = tree.WriteTree(surface, neu, filepath.Join(built.Root, "harness"), false)
			continue
		}
		notes := reply.Text
		if m := notesRE.FindStringSubmatch(reply.Text); m != nil {
			notes = m[1]
		}
		return Stage{Label: label, Dim: dim, Tree: neu, KeptBase: kept, USD: spentUSD,
			Attempts: spentCalls, Cached: allCached, Notes: notes}
	}
	if len(errMsg) > 400 {
		errMsg = errMsg[:400]
	}
	return Stage{Label: label, Dim: dim, Tree: tree.Clone(fallback), FellBack: true,
		Error: errMsg, USD: spentUSD, Attempts: spentCalls, Cached: allCached, ExecutionFailed: execFailed}
}

func joinMax(ss []string, n int) string {
	if len(ss) > n {
		ss = ss[:n]
	}
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func recordSessions(led *ledger.Ledger, stages []Stage) {
	for _, s := range stages {
		led.Session(s.Label, s.USD, s.Cached, s.Attempts)
	}
}

func roll(
	adapter spec.BenchmarkAdapter,
	led *ledger.Ledger,
	taskID, label string,
	t tree.Tree,
	runIDOf func(string) string,
	workDir string,
	record bool,
) spec.RunView {
	var hdir string
	if t != nil {
		hdir, _ = tree.WriteTree(adapter.Substrate().Surface(), t, filepath.Join(workDir, "harnesses", label), true)
	}
	runID := runIDOf(label)
	rd, err := adapter.Execute(taskID, hdir, runID, 0)
	rv := spec.RunView{Label: label, RunID: runID, RunDir: rd}
	if err != nil {
		rv.RunDir = filepath.Join(workDir, "missing")
		rv.Error = err.Error()
	} else {
		rv.OK = true
		rv.Delivered = adapter.Delivered(rd)
	}
	if record {
		led.Rollout(label, 0, false)
	}
	return rv
}

func rollMany(
	adapter spec.BenchmarkAdapter,
	led *ledger.Ledger,
	taskID string,
	items []struct {
		Label string
		Tree  tree.Tree
	},
	runIDOf func(string) string,
	workDir string,
	width int,
) map[string]spec.RunView {
	out := map[string]spec.RunView{}
	if len(items) == 0 {
		return out
	}
	n := len(items)
	if width < 1 {
		width = 1
	}
	if n < width {
		width = n
	}
	if width == 1 {
		for _, it := range items {
			out[it.Label] = roll(adapter, led, taskID, it.Label, it.Tree, runIDOf, workDir, true)
		}
		return out
	}
	// Bounded fan-out: at most `width` concurrent adapter.Execute calls (PhaseWidth default).
	got := make([]spec.RunView, len(items))
	sem := make(chan struct{}, width)
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		go func(i int, it struct {
			Label string
			Tree  tree.Tree
		}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			got[i] = roll(adapter, led, taskID, it.Label, it.Tree, runIDOf, workDir, false)
		}(i, it)
	}
	wg.Wait()
	for i, it := range items {
		led.Rollout(it.Label, 0, false)
		out[it.Label] = got[i]
	}
	return out
}

type labeledTree struct {
	Label string
	Tree  tree.Tree
}

// RunParallel probes 2k candidates then k independent compositions.
func RunParallel(
	taskID string, k int, base []spec.RunView,
	adapter spec.BenchmarkAdapter, prop proposer.Caller,
	workDir string, runIDOf func(string) string, baseTree tree.Tree, libraryDir string,
	proposeWorkers, rolloutWorkers int,
) *ModeResult {
	if proposeWorkers <= 0 {
		proposeWorkers = config.PhaseWidth
	}
	if rolloutWorkers <= 0 {
		rolloutWorkers = config.PhaseWidth
	}
	surface := adapter.Substrate().Surface()
	led := &ledger.Ledger{Mode: "agg", TaskID: taskID, K: k}
	for _, r := range base {
		led.Rollout(r.Label, 0, false)
	}
	result := &ModeResult{Mode: "agg", TaskID: taskID, K: k, Ledger: led,
		Trees: map[string]tree.Tree{}, Rollouts: map[string]spec.RunView{}}
	for _, r := range base {
		result.Rollouts[r.Label] = r
	}
	ctxDir := filepath.Join(workDir, "context")
	bp := spec.ResolvePrompts(adapter, taskID)
	baseDir, _ := tree.WriteTree(surface, baseTree, filepath.Join(workDir, "harnesses", "base"), true)

	type specT struct{ Label, Dim string; Index int }
	var specs []specT
	for i := 1; i <= k; i++ {
		specs = append(specs, specT{Label: fmt.Sprintf("inv%d", i), Dim: "INV", Index: i})
		specs = append(specs, specT{Label: fmt.Sprintf("dom%d", i), Dim: "DOM", Index: i})
	}

	stages := make([]Stage, len(specs))
	sem := make(chan struct{}, min(len(specs), proposeWorkers))
	var wg sync.WaitGroup
	for i, s := range specs {
		wg.Add(1)
		go func(i int, s specT) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			stages[i] = runStage(s.Label, s.Dim, prompts.Propose(bp, s.Dim, surface, k, s.Index, nil),
				baseTree, adapter, prop, ctxDir, taskID, base, baseDir, libraryDir, nil, nil)
		}(i, s)
	}
	wg.Wait()
	recordSessions(led, stages)
	items := make([]labeledTree, 0, len(stages))
	for _, st := range stages {
		result.Stages = append(result.Stages, st)
		result.Trees[st.Label] = st.Tree
		items = append(items, labeledTree{st.Label, st.Tree})
	}
	for lbl, rv := range rollMany(adapter, led, taskID, toItems(items), runIDOf, workDir, rolloutWorkers) {
		result.Rollouts[lbl] = rv
	}

	poolEv := make([]context.PoolItem, 0, len(stages))
	decisions := map[string]string{}
	for _, st := range stages {
		rv := result.Rollouts[st.Label]
		poolEv = append(poolEv, context.PoolItem{Label: st.Label, HarnessDir: filepath.Join(workDir, "harnesses", st.Label), Run: &rv})
		if st.Notes != "" {
			decisions[st.Label] = st.Notes
		} else {
			decisions[st.Label] = st.Error
		}
	}

	aggSpecs := make([]string, 0, k)
	for i := 1; i <= k; i++ {
		aggSpecs = append(aggSpecs, fmt.Sprintf("agg%d", i))
	}
	aggs := make([]Stage, len(aggSpecs))
	sem2 := make(chan struct{}, min(len(aggSpecs), proposeWorkers))
	var wg2 sync.WaitGroup
	for i, lbl := range aggSpecs {
		wg2.Add(1)
		go func(i int, lbl string) {
			defer wg2.Done()
			sem2 <- struct{}{}
			defer func() { <-sem2 }()
			idx := i + 1
			aggs[i] = runStage(lbl, "AGG", prompts.Aggregate(bp, surface, k, idx),
				baseTree, adapter, prop, ctxDir, taskID, base, baseDir, libraryDir, poolEv, decisions)
		}(i, lbl)
	}
	wg2.Wait()
	recordSessions(led, aggs)
	aggItems := make([]labeledTree, 0, len(aggs))
	for _, st := range aggs {
		result.Stages = append(result.Stages, st)
		result.Trees[st.Label] = st.Tree
		result.Finals = append(result.Finals, st.Label)
		aggItems = append(aggItems, labeledTree{st.Label, st.Tree})
	}
	for lbl, rv := range rollMany(adapter, led, taskID, toItems(aggItems), runIDOf, workDir, rolloutWorkers) {
		result.Rollouts[lbl] = rv
	}
	simTrees := map[string]tree.Tree{}
	for _, st := range aggs {
		simTrees[st.Label] = st.Tree
	}
	result.Similarity = tree.Diversity(simTrees)
	_ = led.AssertIdentity()
	return result
}

func toItems(in []labeledTree) []struct {
	Label string
	Tree  tree.Tree
} {
	out := make([]struct {
		Label string
		Tree  tree.Tree
	}, len(in))
	for i, x := range in {
		out[i].Label = x.Label
		out[i].Tree = x.Tree
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RunSequential nests k Propose–Probe–Compose passes.
func RunSequential(
	taskID string, k int, base []spec.RunView,
	adapter spec.BenchmarkAdapter, prop proposer.Caller,
	workDir string, runIDOf func(string) string, baseTree tree.Tree, libraryDir string,
	proposeWorkers, rolloutWorkers int,
) *ModeResult {
	if proposeWorkers <= 0 {
		proposeWorkers = config.PhaseWidth // match parallel; Python SCULPT_PHASE_WIDTH default 2
	}
	if rolloutWorkers <= 0 {
		rolloutWorkers = config.PhaseWidth
	}
	surface := adapter.Substrate().Surface()
	led := &ledger.Ledger{Mode: "seq", TaskID: taskID, K: k}
	for _, r := range base {
		led.Rollout(r.Label, 0, false)
	}
	result := &ModeResult{Mode: "seq", TaskID: taskID, K: k, Ledger: led,
		Trees: map[string]tree.Tree{}, Rollouts: map[string]spec.RunView{}}
	for _, r := range base {
		result.Rollouts[r.Label] = r
	}
	ctxDir := filepath.Join(workDir, "context")
	bp := spec.ResolvePrompts(adapter, taskID)
	current := tree.Clone(baseTree)
	var history []context.PoolItem

	for t := 1; t <= k; t++ {
		currentLabel := "base"
		if t > 1 {
			currentLabel = fmt.Sprintf("c%d", t-1)
		}
		currentDir, _ := tree.WriteTree(surface, current, filepath.Join(workDir, "harnesses", currentLabel), true)
		type pair struct{ Label, Dim string }
		specs := []pair{{fmt.Sprintf("inv%d", t), "INV"}, {fmt.Sprintf("dom%d", t), "DOM"}}
		stages := make([]Stage, 2)
		var wg sync.WaitGroup
		decisions := map[string]string{}
		for _, st := range result.Stages {
			if st.Notes != "" {
				decisions[st.Label] = st.Notes
			} else {
				decisions[st.Label] = st.Error
			}
		}
		roundT := t
		var rt *int
		if k > 1 {
			rt = &roundT
		}
		for i, s := range specs {
			wg.Add(1)
			go func(i int, s pair) {
				defer wg.Done()
				stages[i] = runStage(s.Label, s.Dim, prompts.Propose(bp, s.Dim, surface, k, t, rt),
					current, adapter, prop, ctxDir, taskID, base, currentDir, libraryDir, history, decisions)
			}(i, s)
		}
		wg.Wait()
		recordSessions(led, stages)
		items := make([]labeledTree, 0, 2)
		for _, st := range stages {
			result.Stages = append(result.Stages, st)
			result.Trees[st.Label] = st.Tree
			items = append(items, labeledTree{st.Label, st.Tree})
		}
		for lbl, rv := range rollMany(adapter, led, taskID, toItems(items), runIDOf, workDir, rolloutWorkers) {
			result.Rollouts[lbl] = rv
		}
		roundPool := append([]context.PoolItem{}, history...)
		for _, st := range stages {
			rv := result.Rollouts[st.Label]
			roundPool = append(roundPool, context.PoolItem{Label: st.Label, HarnessDir: filepath.Join(workDir, "harnesses", st.Label), Run: &rv})
		}
		ck := fmt.Sprintf("c%d", t)
		compJob := prompts.Aggregate(bp, surface, 1, 1)
		if k > 1 {
			compJob = prompts.ComposeRound(bp, surface, t, k)
		}
		for _, st := range result.Stages {
			if st.Notes != "" {
				decisions[st.Label] = st.Notes
			} else {
				decisions[st.Label] = st.Error
			}
		}
		comp := runStage(ck, "COMPOSE", compJob, current, adapter, prop, ctxDir, taskID, base, currentDir, libraryDir, roundPool, decisions)
		recordSessions(led, []Stage{comp})
		result.Stages = append(result.Stages, comp)
		result.Trees[ck] = comp.Tree
		result.Rollouts[ck] = roll(adapter, led, taskID, ck, comp.Tree, runIDOf, workDir, true)
		rv := result.Rollouts[ck]
		history = append(roundPool, context.PoolItem{Label: ck, HarnessDir: filepath.Join(workDir, "harnesses", ck), Run: &rv})
		current = comp.Tree
	}
	for i := 1; i <= k; i++ {
		result.Finals = append(result.Finals, fmt.Sprintf("c%d", i))
	}
	_ = led.AssertIdentity()
	return result
}

// RunMode dispatches on "agg" or "seq".
func RunMode(mode string, taskID string, k int, base []spec.RunView,
	adapter spec.BenchmarkAdapter, prop proposer.Caller,
	workDir string, runIDOf func(string) string, baseTree tree.Tree, libraryDir string,
) (*ModeResult, error) {
	switch mode {
	case "agg":
		return RunParallel(taskID, k, base, adapter, prop, workDir, runIDOf, baseTree, libraryDir, 0, 0), nil
	case "seq":
		return RunSequential(taskID, k, base, adapter, prop, workDir, runIDOf, baseTree, libraryDir, 0, 0), nil
	default:
		return nil, fmt.Errorf("unknown mode %q; expected 'agg' or 'seq'", mode)
	}
}
