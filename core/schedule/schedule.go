// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package schedule names runs, orders the continual-learning stream, and drives tasks.
package schedule

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"sculpt/core/library"
	"sculpt/core/proposer"
	"sculpt/core/recursion"
	"sculpt/core/runmark"
	"sculpt/core/selection"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/internal/config"
)

// Modes are the on-disk recursion tokens.
var Modes = []string{"agg", "seq"}

// runModeFn is the recursion entry used by runOneTask (overridable in tests).
var runModeFn = recursion.RunMode

// SetRunModeForTest replaces recursion.RunMode for tests; restore with nil.
func SetRunModeForTest(fn func(mode, taskID string, k int, base []spec.RunView,
	adapter spec.BenchmarkAdapter, prop proposer.Caller,
	workDir string, runIDOf func(string) string, baseTree tree.Tree, libraryDir string,
) (*recursion.ModeResult, error)) {
	if fn == nil {
		runModeFn = recursion.RunMode
		return
	}
	runModeFn = fn
}

var modeAliases = map[string]string{
	"agg": "agg", "seq": "seq", "parallel": "agg", "sequential": "seq",
}

// ModeToken maps parallel/sequential to agg/seq.
func ModeToken(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	tok, ok := modeAliases[key]
	if !ok {
		return "", fmt.Errorf("unknown mode %q; expected parallel | sequential", name)
	}
	return tok, nil
}

// ModeLabel maps on-disk tokens to human names (agg -> parallel).
func ModeLabel(token string) string {
	switch token {
	case "agg":
		return "parallel"
	case "seq":
		return "sequential"
	default:
		return token
	}
}

// ParseModes parses a comma list of parallel|sequential|agg|seq into tokens in agg,seq order.
func ParseModes(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		tok, err := ModeToken(p)
		if err != nil {
			return nil, fmt.Errorf("--modes: %w (got %q)", err, raw)
		}
		seen[tok] = true
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("--modes takes a comma list of parallel | sequential, got %q", raw)
	}
	var out []string
	for _, tok := range []string{"agg", "seq"} {
		if seen[tok] {
			out = append(out, tok)
		}
	}
	return out, nil
}

// DefaultReferenceDomainOrder is descending task count, then name (Python parity).
func DefaultReferenceDomainOrder(byDomain map[string][]string) []string {
	type pair struct {
		d string
		n int
	}
	pairs := make([]pair, 0, len(byDomain))
	for d, tasks := range byDomain {
		pairs = append(pairs, pair{d, len(tasks)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].n != pairs[j].n {
			return pairs[i].n > pairs[j].n
		}
		return pairs[i].d < pairs[j].d
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.d
	}
	return out
}

// TaskStreamOpts filters/orders the adaptation task stream.
type TaskStreamOpts struct {
	Split       string
	Seed        *int
	OnlyDomains []string
	OnlyTasks   []string
	MaxTasks    *int
}

// TaskStream resolves the ordered task stream by domain (Python schedule.task_stream).
// flat is the (domain, task) walk in stream order after filters (do not reshuffle).
func TaskStream(adapter spec.BenchmarkAdapter, opts TaskStreamOpts) (byDomain map[string][]string, flat [][2]string, err error) {
	split := opts.Split
	if split == "" {
		split = "all"
	}
	raw := map[string][]string{}
	for _, t := range adapter.Tasks(split) {
		d := adapter.DomainOf(t)
		raw[d] = append(raw[d], t)
	}
	if len(opts.OnlyDomains) > 0 {
		keep := map[string]bool{}
		for _, d := range opts.OnlyDomains {
			keep[d] = true
		}
		var missing []string
		for d := range keep {
			if _, ok := raw[d]; !ok {
				missing = append(missing, d)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			known := make([]string, 0, len(raw))
			for d := range raw {
				known = append(known, d)
			}
			sort.Strings(known)
			return nil, nil, fmt.Errorf("only_domains names unknown domain(s) %v; split %q has %v", missing, split, known)
		}
		filtered := map[string][]string{}
		for d, v := range raw {
			if keep[d] {
				filtered[d] = v
			}
		}
		raw = filtered
	}
	wantedTasks := map[string]bool{}
	for _, t := range opts.OnlyTasks {
		wantedTasks[t] = true
	}
	if len(wantedTasks) > 0 {
		have := map[string]bool{}
		for _, v := range raw {
			for _, t := range v {
				have[t] = true
			}
		}
		var missing []string
		for t := range wantedTasks {
			if !have[t] {
				missing = append(missing, t)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			return nil, nil, fmt.Errorf("only_tasks names unknown task id(s) %v for split %q", missing, split)
		}
	}
	if opts.MaxTasks != nil && *opts.MaxTasks < 1 {
		return nil, nil, fmt.Errorf("max_tasks must be positive, got %d", *opts.MaxTasks)
	}
	stream, err := StreamOrder(raw, opts.Seed, DefaultReferenceDomainOrder(raw))
	if err != nil {
		return nil, nil, err
	}
	ordered := map[string][]string{}
	for _, pair := range stream {
		d, t := pair[0], pair[1]
		if len(wantedTasks) > 0 && !wantedTasks[t] {
			continue
		}
		if opts.MaxTasks != nil && len(ordered[d]) >= *opts.MaxTasks {
			continue
		}
		ordered[d] = append(ordered[d], t)
		flat = append(flat, pair)
	}
	return ordered, flat, nil
}

// ValidateTag checks a tag's shape.
func ValidateTag(tag string) error {
	if tag == "" {
		return nil
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9_.][A-Za-z0-9_.\-]*$`, tag); !ok {
		return fmt.Errorf("tag %q must be [A-Za-z0-9_.-] with no leading '-'", tag)
	}
	if matched, _ := regexp.MatchString(`^s\d+(-|$)`, tag); matched {
		return fmt.Errorf("tag %q spells a seed segment; pass --seed instead", tag)
	}
	return nil
}

// Namespace is every name for one (bench, substrate, mode, k, seed, tag) setting.
type Namespace struct {
	Benchmark string
	Substrate string
	Mode      string
	K         int
	Seed      *int
	Tag       string
}

func (n Namespace) String() string {
	return NS(n.Benchmark, n.Substrate, n.Mode, n.K, n.Seed, n.Tag)
}

// RunID builds a namespaced run id.
func (n Namespace) RunID(taskID, label string) string {
	return fmt.Sprintf("%s/%s/%s", n.String(), taskID, label)
}

// TaskDir is the on-disk task directory.
func (n Namespace) TaskDir(root, taskID string) string {
	return filepath.Join(root, n.String(), strings.ReplaceAll(taskID, "/", "__"))
}

// NS builds the namespace string.
func NS(benchmark, substrate, mode string, k int, seed *int, tag string) string {
	if mode != "agg" && mode != "seq" {
		panic(fmt.Sprintf("unknown mode %q", mode))
	}
	if substrate == "" {
		panic("namespace needs a substrate name")
	}
	_ = ValidateTag(tag)
	out := fmt.Sprintf("%s-%s-%s%d", benchmark, substrate, mode, k)
	if seed != nil {
		out += fmt.Sprintf("-s%d", *seed)
	}
	if tag != "" {
		out += "-" + tag
	}
	return out
}

// StreamOrder builds the continual-learning (domain, task) stream.
func StreamOrder(tasksByDomain map[string][]string, seed *int, referenceDomainOrder []string) ([][2]string, error) {
	domains := make([]string, 0, len(tasksByDomain))
	for d := range tasksByDomain {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	var ordered []string
	if seed == nil {
		have := map[string]struct{}{}
		for _, d := range referenceDomainOrder {
			have[d] = struct{}{}
		}
		for _, d := range domains {
			if _, ok := have[d]; !ok {
				return nil, fmt.Errorf("reference_domain_order does not cover domain(s) [%s]", d)
			}
		}
		for _, d := range referenceDomainOrder {
			if _, ok := tasksByDomain[d]; ok {
				ordered = append(ordered, d)
			}
		}
	} else {
		ordered = append([]string(nil), domains...)
		r := rand.New(rand.NewSource(hashSeed(fmt.Sprintf("%d:__domains__", *seed))))
		r.Shuffle(len(ordered), func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	}
	var out [][2]string
	for _, d := range ordered {
		tasks := append([]string(nil), tasksByDomain[d]...)
		sort.Strings(tasks)
		if seed != nil {
			r := rand.New(rand.NewSource(hashSeed(fmt.Sprintf("%d:%s", *seed, d))))
			r.Shuffle(len(tasks), func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })
		}
		for _, t := range tasks {
			out = append(out, [2]string{d, t})
		}
	}
	return out, nil
}

func hashSeed(s string) int64 {
	h := int64(0)
	for _, c := range s {
		h = h*31 + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// TaskResult is one task's summary record.
type TaskResult struct {
	TaskID     string         `json:"task_id"`
	Domain     string         `json:"domain"`
	K          int            `json:"k"`
	Seed       *int           `json:"seed"`
	Status     string         `json:"status"`
	Inputs     string         `json:"inputs,omitempty"`
	Modes      map[string]any `json:"modes"`
	Selections map[string]any `json:"selections"`
	Base       []any          `json:"base"`
	Errors     []string       `json:"errors"`
}

var summaryMu sync.Mutex

func writeSummary(path string, res TaskResult) error {
	summaryMu.Lock()
	defer summaryMu.Unlock()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	merged := res
	if b, err := os.ReadFile(path); err == nil {
		var old TaskResult
		if json.Unmarshal(b, &old) == nil {
			inputsChanged := res.Inputs != "" && old.Inputs != "" && old.Inputs != res.Inputs
			if !inputsChanged {
				if merged.Modes == nil {
					merged.Modes = map[string]any{}
				}
				for k, v := range old.Modes {
					if _, ok := merged.Modes[k]; !ok {
						merged.Modes[k] = v
					}
				}
				if merged.Selections == nil {
					merged.Selections = map[string]any{}
				}
				for k, v := range old.Selections {
					if _, ok := merged.Selections[k]; !ok {
						merged.Selections[k] = v
					}
				}
			}
		}
	}
	if merged.Status == "ok" || merged.Status == "partial" {
		merged.Status = mergedStatus(merged)
	}
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func mergedStatus(m TaskResult) string {
	if len(m.Modes) == 0 || len(m.Errors) > 0 {
		return "partial"
	}
	for mode, value := range m.Modes {
		vm, _ := value.(map[string]any)
		ok, _ := vm["ok"].(bool)
		sel, _ := m.Selections[mode].(map[string]any)
		sok, _ := sel["ok"].(bool)
		label, _ := sel["chosen_label"].(string)
		if !(ok && sok && label != "") {
			return "partial"
		}
	}
	return "ok"
}

func tryResume(summaryPath, taskID string, k int, adapter spec.BenchmarkAdapter, runsRoot string, modes []string, seed *int, tag string, modeLibs map[string]*library.Library, res *TaskResult) bool {
	b, err := os.ReadFile(summaryPath)
	if err != nil {
		return false
	}
	var got TaskResult
	if json.Unmarshal(b, &got) != nil {
		return false
	}
	if got.Status != "ok" && got.Status != "partial" {
		return false
	}
	if got.Inputs != "" && res.Inputs != "" && got.Inputs != res.Inputs {
		fmt.Printf(
			"  [%s] task inputs %s != %s -- the task prompt or its source files changed since this summary was written; re-running\n",
			taskID, got.Inputs, res.Inputs,
		)
		return false
	}
	for _, mode := range modes {
		vm, _ := got.Modes[mode].(map[string]any)
		sel, _ := got.Selections[mode].(map[string]any)
		ok, _ := vm["ok"].(bool)
		sok, _ := sel["ok"].(bool)
		label, _ := sel["chosen_label"].(string)
		if !(ok && sok && label != "") {
			return false
		}
		n := Namespace{Benchmark: adapter.Spec().Name, Substrate: adapter.Substrate().Name(), Mode: mode, K: k, Seed: seed, Tag: tag}
		harnessDir := filepath.Join(n.TaskDir(runsRoot, taskID), "harnesses", label)
		runDir, _ := sel["chosen_run_dir"].(string)
		if runDir == "" {
			return false
		}
		if fi, err := os.Stat(runDir); err != nil || !fi.IsDir() {
			return false
		}
		artifact := adapter.DeliverableText(runDir)
		if !adapter.Delivered(runDir) || strings.TrimSpace(artifact) == "" {
			return false
		}
		if !tree.IsTree(adapter.Substrate().Surface(), harnessDir) {
			return false
		}
		if fp, _ := sel["harness_fingerprint"].(string); fp != "" && runmark.Fingerprint(harnessDir) != fp {
			return false
		}
	}
	res.Status = "skipped"
	res.Base = got.Base
	res.Modes = got.Modes
	res.Selections = got.Selections
	res.Errors = got.Errors
	restoreLibraries(taskID, k, adapter, runsRoot, modes, seed, tag, modeLibs, res.Selections)
	return true
}

func restoreLibraries(taskID string, k int, adapter spec.BenchmarkAdapter, runsRoot string, modes []string, seed *int, tag string, libraries map[string]*library.Library, selections map[string]any) {
	surface := adapter.Substrate().Surface()
	for _, mode := range modes {
		lib := libraries[mode]
		sel, _ := selections[mode].(map[string]any)
		label, _ := sel["chosen_label"].(string)
		ok, _ := sel["ok"].(bool)
		expRaw, _ := sel["experience"].(map[string]any)
		expStr := map[string]string{}
		for key, v := range expRaw {
			expStr[key] = fmt.Sprint(v)
		}
		if lib == nil || label == "" || !ok || !library.HasExperience(expStr) {
			continue
		}
		n := Namespace{Benchmark: adapter.Spec().Name, Substrate: adapter.Substrate().Name(), Mode: mode, K: k, Seed: seed, Tag: tag}
		t := tree.ReadTree(surface, filepath.Join(n.TaskDir(runsRoot, taskID), "harnesses", label), nil)
		if len(t) == 0 {
			continue
		}
		why, _ := sel["why"].(string)
		runID, _ := sel["chosen_run_id"].(string)
		_, _ = lib.Add(taskID, t, why, label, runID, expStr)
	}
}

// DomainOptions are optional RunDomain knobs. Seed/Tag enter every namespace
// (Python run_sweep --seed/--tag). Selector overrides prop for selection/reflection.
type DomainOptions struct {
	Seed     *int
	Tag      string
	Selector proposer.Caller
}

// RunDomain runs recursion modes over a domain task stream.
func RunDomain(
	domain string,
	taskIDs []string,
	k int,
	adapter spec.BenchmarkAdapter,
	prop proposer.Caller,
	runsRoot, libraryRoot string,
	modes []string,
	opts ...DomainOptions,
) map[string]any {
	if len(modes) == 0 {
		modes = []string{"agg", "seq"}
	}
	var o DomainOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	selProp := prop
	if o.Selector != nil {
		selProp = o.Selector
	}
	surface := adapter.Substrate().Surface()
	modeLibs := map[string]*library.Library{}
	for _, mode := range modes {
		modeLibs[mode] = &library.Library{
			Root:    filepath.Join(libraryRoot, mode),
			Domain:  domain,
			Surface: surface,
		}
	}
	baseTree := tree.ReadTree(surface, adapter.BaseHarness(), nil)
	var tasks []map[string]any
	done, failed := 0, 0
	for i, taskID := range taskIDs {
		_ = i
		res := runOneTask(domain, taskID, k, adapter, prop, selProp, runsRoot, modes, modeLibs, baseTree, o.Seed, o.Tag)
		row := map[string]any{"task": taskID, "task_id": taskID, "status": res.Status, "domain": res.Domain}
		tasks = append(tasks, row)
		if res.Status == "ok" || res.Status == "skipped" {
			done++
		} else {
			failed++
		}
	}
	out := map[string]any{"done": done, "failed": failed, "tasks": tasks, "domain": domain, "k": k, "n_tasks": len(taskIDs)}
	if o.Seed != nil {
		out["seed"] = *o.Seed
	}
	if o.Tag != "" {
		out["tag"] = o.Tag
	}
	return out
}

func runOneTask(
	domain, taskID string, k int,
	adapter spec.BenchmarkAdapter, prop, selector proposer.Caller,
	runsRoot string, modes []string,
	modeLibs map[string]*library.Library, baseTree tree.Tree,
	seed *int, tag string,
) TaskResult {
	res := TaskResult{
		TaskID: taskID, Domain: domain, K: k, Seed: seed, Status: "running",
		Modes: map[string]any{}, Selections: map[string]any{},
	}
	res.Inputs = runmark.InputsFingerprint(adapter.TaskPrompt(taskID), adapter.TaskFiles(taskID))

	// Python anchors the shared summary under the agg namespace.
	baseNS := Namespace{Benchmark: adapter.Spec().Name, Substrate: adapter.Substrate().Name(), Mode: "agg", K: k, Seed: seed, Tag: tag}
	work := baseNS.TaskDir(runsRoot, taskID)
	_ = os.MkdirAll(work, 0o755)
	summaryPath := filepath.Join(work, "summary.json")

	if tryResume(summaryPath, taskID, k, adapter, runsRoot, modes, seed, tag, modeLibs, &res) {
		return res
	}

	dead := filepath.Join(work, "missing")
	_ = os.MkdirAll(dead, 0o755)
	width := config.PhaseWidth
	type pair struct{ Label, RID string }
	pending := make([]pair, 0, k)
	for i := 1; i <= k; i++ {
		lbl := fmt.Sprintf("base%d", i)
		pending = append(pending, pair{lbl, baseNS.RunID(taskID, lbl)})
	}
	rolled := map[string]spec.RunView{}
	if len(pending) == 1 || width <= 1 {
		for _, p := range pending {
			rolled[p.Label] = rollBase(adapter, taskID, p.Label, p.RID, dead)
		}
	} else {
		sem := make(chan struct{}, width)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, p := range pending {
			wg.Add(1)
			go func(p pair) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				rv := rollBase(adapter, taskID, p.Label, p.RID, dead)
				mu.Lock()
				rolled[p.Label] = rv
				mu.Unlock()
			}(p)
		}
		wg.Wait()
	}

	baseViews := make([]spec.RunView, 0, k)
	for i := 1; i <= k; i++ {
		lbl := fmt.Sprintf("base%d", i)
		rv := rolled[lbl]
		baseViews = append(baseViews, rv)
		res.Base = append(res.Base, map[string]any{"label": rv.Label, "run_id": rv.RunID, "run_dir": rv.RunDir})
	}

	if !anyOK(baseViews) {
		err := "no base rollout produced anything"
		res.Status = "base-failed"
		res.Errors = append(res.Errors, err)
		for _, mode := range modes {
			res.Modes[mode] = map[string]any{"ok": false, "error": err}
			res.Selections[mode] = selection.Selection{TaskID: taskID, Mode: mode, K: k, Error: err}.ToDict()
		}
		_ = writeSummary(summaryPath, res)
		return res
	}

	for _, mode := range modes {
		n := Namespace{Benchmark: adapter.Spec().Name, Substrate: adapter.Substrate().Name(), Mode: mode, K: k, Seed: seed, Tag: tag}
		workDir := n.TaskDir(runsRoot, taskID)
		_ = os.MkdirAll(workDir, 0o755)

		libDir := ""
		if lib := modeLibs[mode]; lib != nil {
			libDir = lib.ContextDir()
		}
		runIDOf := func(label string) string { return n.RunID(taskID, label) }
		mr, err := runModeFn(mode, taskID, k, baseViews, adapter, prop, workDir, runIDOf, baseTree, libDir)
		if err != nil {
			prevUSD := 0.0
			if prev, ok := res.Selections[mode].(map[string]any); ok {
				prevUSD, _ = prev["usd"].(float64)
			}
			res.Errors = append(res.Errors, mode+": "+err.Error())
			res.Modes[mode] = map[string]any{"ok": false, "error": err.Error()}
			failSel := selection.Selection{TaskID: taskID, Mode: mode, K: k, Error: err.Error(), USD: prevUSD}
			res.Selections[mode] = failSel.ToDict()
			res.Status = "partial"
			_ = writeSummary(summaryPath, res)
			continue
		}
		for _, st := range mr.Stages {
			if st.ExecutionFailed {
				res.Errors = append(res.Errors, fmt.Sprintf(
					"%s: %s: proposer execution failed: %s", mode, st.Label, st.Error))
			}
		}
		res.Modes[mode] = mr.ToDict()

		var sel selection.Selection
		if k == 1 {
			cands := selection.SelectionPool(mode, k, mr.Rollouts)
			var err error
			sel, err = selection.TrivialSelection(taskID, mode, k, cands, adapter, workDir)
			if err != nil {
				res.Errors = append(res.Errors, mode+": "+err.Error())
				res.Status = "partial"
				_ = writeSummary(summaryPath, res)
				continue
			}
			if modeLibs[mode] != nil && sel.OK {
				selection.ReflectAdoption(taskID, &sel, mr.Rollouts, adapter, selector, workDir, baseTree, mr.Trees)
			}
		} else {
			sel = selection.Run(taskID, mode, k, mr.Rollouts, baseViews, adapter, selector, workDir, baseTree, mr.Trees)
		}
		if sel.ChosenLabel != "" {
			sel.HarnessFP = runmark.Fingerprint(filepath.Join(workDir, "harnesses", sel.ChosenLabel))
		}
		d := sel.ToDict()
		d["selected_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		if sel.ChosenLabel != "" {
			if rv, ok := mr.Rollouts[sel.ChosenLabel]; ok && rv.RunDir != "" {
				d["chosen_run_dir"] = rv.RunDir
			}
		}
		res.Selections[mode] = d

		if lib := modeLibs[mode]; lib != nil && sel.OK && library.HasExperience(sel.Experience) {
			chosen := mr.Trees[sel.ChosenLabel]
			if chosen == nil {
				chosen = tree.ReadTree(adapter.Substrate().Surface(), filepath.Join(workDir, "harnesses", sel.ChosenLabel), nil)
			}
			if _, err := lib.Add(taskID, chosen, sel.Why, sel.ChosenLabel, sel.ChosenRunID, sel.Experience); err != nil {
				// Preserve usd (reflection already paid); clear adoption so judges skip.
				usd := sel.USD
				selectedAt, _ := d["selected_at"].(string)
				sel.OK = false
				sel.Error = err.Error()
				sel.ChosenLabel = ""
				sel.ChosenRunID = ""
				sel.ChosenRunDir = ""
				sel.ChosenID = ""
				sel.Experience = map[string]string{}
				sel.USD = usd
				d = sel.ToDict()
				d["selected_at"] = selectedAt
				res.Selections[mode] = d
				res.Errors = append(res.Errors, fmt.Sprintf("%s: library write failed: %v", mode, err))
			}
		}
		if !sel.OK {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: selection failed: %s", mode, sel.Error))
		}
		res.Status = "partial"
		_ = writeSummary(summaryPath, res)
	}

	res.Status = mergedStatus(res)
	_ = writeSummary(summaryPath, res)
	return res
}

func anyOK(views []spec.RunView) bool {
	for _, v := range views {
		if v.OK {
			return true
		}
	}
	return false
}

func rollBase(adapter spec.BenchmarkAdapter, taskID, label, rid, dead string) spec.RunView {
	rd, err := adapter.Execute(taskID, "", rid, 0)
	if err != nil {
		return spec.RunView{Label: label, RunID: rid, RunDir: dead, Error: err.Error()}
	}
	return spec.RunView{Label: label, RunID: rid, RunDir: rd, OK: true, Delivered: adapter.Delivered(rd)}
}
