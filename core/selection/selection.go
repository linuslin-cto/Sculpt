// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package selection chooses an anonymized final output, then reflects.
package selection

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sculpt/core/context"
	"sculpt/core/prompts"
	"sculpt/core/proposer"
	"sculpt/core/spec"
	"sculpt/core/tree"
)

func jsonMarshal(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// applySelectorFallback picks a neutral stand-in after selector failure.
// Prefer a delivered base label; else the first sorted final. Never sets OK.
func applySelectorFallback(sel *Selection, pool map[string]spec.RunView, labels []string, errMsg string) {
	var bases []string
	for _, lbl := range labels {
		if strings.HasPrefix(lbl, "base") {
			bases = append(bases, lbl)
		}
	}
	sort.Strings(bases)
	sort.Strings(labels)
	chosen := ""
	if len(bases) > 0 {
		chosen = bases[0]
	} else if len(labels) > 0 {
		chosen = labels[0]
	}
	if chosen == "" {
		sel.Error = errMsg + "; no delivered base artifact was available for a neutral fallback"
		return
	}
	rv := pool[chosen]
	sel.ChosenLabel = chosen
	sel.ChosenRunID = rv.RunID
	sel.ChosenRunDir = rv.RunDir
	sel.ChosenIsBase = strings.HasPrefix(chosen, "base")
	for anon, lbl := range sel.IDMap {
		if lbl == chosen {
			sel.ChosenID = anon
			break
		}
	}
	sel.Fallback = true
	sel.Error = errMsg
	if len(sel.Error) > 400 {
		sel.Error = sel.Error[:400]
	}
	sel.OK = false
}

var (
	choiceRE = regexp.MustCompile("(?s)```choice\\s*\\n?\\s*([A-Za-z0-9_-]+)\\s*\\n?\\s*```")
	whyRE    = regexp.MustCompile(`(?s)<why>(.*?)</why>`)
	expRE    = regexp.MustCompile(`(?s)<experience>(.*?)</experience>`)
	editsRE  = regexp.MustCompile(`(?s)<edits>(.*?)</edits>`)
)

// Selection is the result of blind answer selection + reflection.
type Selection struct {
	TaskID       string            `json:"task_id"`
	Mode         string            `json:"mode"`
	K            int               `json:"k"`
	ChosenID     string            `json:"chosen_id"`
	ChosenLabel  string            `json:"chosen_label"`
	ChosenRunID  string            `json:"chosen_run_id"`
	ChosenRunDir string            `json:"chosen_run_dir"`
	ChosenIsBase bool              `json:"chosen_is_base"`
	Why          string            `json:"why"`
	IDMap        map[string]string `json:"id_map"`
	Dropped      []string          `json:"dropped"`
	NCandidates  int               `json:"n_candidates"`
	OK           bool              `json:"ok"`
	Fallback     bool              `json:"fallback"`
	Error        string            `json:"error"`
	USD          float64           `json:"usd"`
	Experience   map[string]string `json:"experience"`
	Trivial      bool              `json:"trivial"`
	Evidence     string            `json:"evidence"`
	HarnessFP    string            `json:"harness_fingerprint,omitempty"`
}

// ToDict returns a JSON-friendly map.
func (s Selection) ToDict() map[string]any {
	return map[string]any{
		"task_id": s.TaskID, "mode": s.Mode, "k": s.K,
		"chosen_id": s.ChosenID, "chosen_label": s.ChosenLabel,
		"chosen_run_id": s.ChosenRunID, "chosen_run_dir": s.ChosenRunDir,
		"chosen_is_base": s.ChosenIsBase, "why": s.Why, "id_map": s.IDMap,
		"dropped": s.Dropped, "n_candidates": s.NCandidates, "ok": s.OK,
		"fallback": s.Fallback, "error": s.Error, "usd": s.USD,
		"experience": s.Experience, "trivial": s.Trivial, "evidence": s.Evidence,
		"harness_fingerprint": s.HarnessFP,
	}
}

// AssignIDs maps anon id -> label via seeded shuffle (stable across reruns).
func AssignIDs(labels []string, taskID string, k int, mode string) map[string]string {
	seedBytes := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%s", taskID, k, mode)))
	hex12 := fmt.Sprintf("%x", seedBytes)[:12]
	var seedU uint64
	_, _ = fmt.Sscanf(hex12, "%x", &seedU)
	ordered := append([]string(nil), labels...)
	sort.Strings(ordered)
	r := rand.New(rand.NewSource(int64(seedU)))
	r.Shuffle(len(ordered), func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	out := make(map[string]string, len(ordered))
	for i, lbl := range ordered {
		out[anonID(i)] = lbl
	}
	return out
}

func anonID(index int) string {
	out := ""
	n := index + 1
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}

// TrivialSelection delivers the single k=1 final without a selector session.
func TrivialSelection(taskID, mode string, k int, candidates map[string]spec.RunView, adapter spec.BenchmarkAdapter, workDir string) (Selection, error) {
	if len(candidates) > 1 {
		keys := make([]string, 0, len(candidates))
		for k := range candidates {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return Selection{}, fmt.Errorf("trivial_selection expects at most one candidate, got %v", keys)
	}
	sel := Selection{
		TaskID: taskID, Mode: mode, K: k, Trivial: true,
		Evidence: "deliverable", Experience: map[string]string{},
	}
	pool := map[string]spec.RunView{}
	for lbl, rv := range candidates {
		if !(rv.OK && rv.Delivered) {
			sel.Dropped = append(sel.Dropped, lbl)
			continue
		}
		if strings.TrimSpace(adapter.DeliverableText(rv.RunDir)) == "" {
			sel.Dropped = append(sel.Dropped, lbl)
			continue
		}
		pool[lbl] = rv
	}
	sel.NCandidates = len(pool)
	if len(pool) == 0 {
		sel.Error = "no candidate produced an artifact (finals-only pool has no base fallback)"
		return sel, nil
	}
	var label string
	for lbl := range pool {
		label = lbl
		break
	}
	rv := pool[label]
	sel.OK = true
	sel.ChosenID = "A"
	sel.ChosenLabel = label
	sel.IDMap = map[string]string{"A": label}
	sel.ChosenRunID = rv.RunID
	sel.ChosenRunDir = rv.RunDir
	sel.ChosenIsBase = strings.HasPrefix(label, "base")
	sel.Why = "trivial pool: single final at k=1; delivered without a selector session"
	root := filepath.Join(workDir, "selection", fmt.Sprintf("%s%d", mode, k))
	_ = os.RemoveAll(root)
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(filepath.Join(root, "selection.json"), mustJSON(sel.ToDict()), 0o644)
	return sel, nil
}

func mustJSON(v map[string]any) []byte {
	b, err := jsonMarshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// SelectionPool returns exactly the k finals of mode (agg1..aggk or c1..ck).
func SelectionPool(mode string, k int, rollouts map[string]spec.RunView) map[string]spec.RunView {
	var labels []string
	if mode == "agg" {
		for i := 1; i <= k; i++ {
			labels = append(labels, fmt.Sprintf("agg%d", i))
		}
	} else {
		for i := 1; i <= k; i++ {
			labels = append(labels, fmt.Sprintf("c%d", i))
		}
	}
	out := map[string]spec.RunView{}
	for _, lbl := range labels {
		if rv, ok := rollouts[lbl]; ok {
			out[lbl] = rv
		}
	}
	return out
}

// Run performs selection + experience reflection for one mode result.
func Run(
	taskID, mode string,
	k int,
	rollouts map[string]spec.RunView,
	base []spec.RunView,
	adapter spec.BenchmarkAdapter,
	prop proposer.Caller,
	workDir string,
	baseTree tree.Tree,
	trees map[string]tree.Tree,
) Selection {
	_ = base
	sel := Selection{
		TaskID: taskID, Mode: mode, K: k,
		Evidence: "deliverable", Experience: map[string]string{},
	}
	candidates := SelectionPool(mode, k, rollouts)
	pool := map[string]spec.RunView{}
	artifacts := map[string]string{}
	for lbl, rv := range candidates {
		if !(rv.OK && rv.Delivered) {
			sel.Dropped = append(sel.Dropped, lbl)
			continue
		}
		rendered := adapter.DeliverableText(rv.RunDir)
		if strings.TrimSpace(rendered) == "" {
			sel.Dropped = append(sel.Dropped, lbl)
			continue
		}
		pool[lbl] = rv
		artifacts[lbl] = rendered
	}
	sel.NCandidates = len(pool)
	if len(pool) == 0 {
		sel.Error = "no delivered candidates"
		sel.Fallback = true
		// fall back to first base if any
		for _, rv := range base {
			if rv.OK && rv.Delivered {
				sel.ChosenLabel = rv.Label
				sel.ChosenRunID = rv.RunID
				sel.ChosenRunDir = rv.RunDir
				sel.ChosenIsBase = true
				sel.OK = true
				sel.Why = "fallback to base"
				break
			}
		}
		return sel
	}

	labels := make([]string, 0, len(pool))
	for lbl := range pool {
		labels = append(labels, lbl)
	}
	sel.IDMap = AssignIDs(labels, taskID, k, mode)

	// Materialise anonymized candidates.
	ctxRoot := filepath.Join(workDir, "selection", mode)
	_ = os.RemoveAll(ctxRoot)
	_ = os.MkdirAll(filepath.Join(ctxRoot, "candidates"), 0o755)
	if src := adapter.TaskFiles(taskID); src != "" {
		_ = copyDirFiltered(src, filepath.Join(ctxRoot, "source"), adapter.Spec().LeakGlobs)
	}
	for anon, lbl := range sel.IDMap {
		_ = os.WriteFile(filepath.Join(ctxRoot, "candidates", anon+".md"), []byte(artifacts[lbl]), 0o644)
	}

	bp := spec.ResolvePrompts(adapter, taskID)
	reply, err := prop.Call(prompts.Select(bp), ctxRoot, fmt.Sprintf("select.%s%d", mode, k), "", 2)
	if err != nil {
		if pe, ok := err.(*proposer.Error); ok {
			sel.USD += pe.CostUSD
		}
		applySelectorFallback(&sel, pool, labels, err.Error())
		return sel // ok stays false — Python parity (fallback is not a successful selection)
	}
	sel.USD += reply.CostUSD
	m := choiceRE.FindStringSubmatch(reply.Text)
	if m == nil {
		applySelectorFallback(&sel, pool, labels, "no choice block")
		return sel
	}
	sel.ChosenID = m[1]
	lbl, ok := sel.IDMap[sel.ChosenID]
	if !ok {
		applySelectorFallback(&sel, pool, labels, fmt.Sprintf("unknown choice %q", sel.ChosenID))
		return sel
	}
	rv := pool[lbl]
	sel.ChosenLabel = lbl
	sel.ChosenRunID = rv.RunID
	sel.ChosenRunDir = rv.RunDir
	sel.ChosenIsBase = strings.HasPrefix(lbl, "base")
	if wm := whyRE.FindStringSubmatch(reply.Text); wm != nil {
		sel.Why = strings.TrimSpace(wm[1])
	}
	sel.OK = true

	// Reflection
	refRoot := filepath.Join(workDir, "reflection", fmt.Sprintf("%s%d", mode, k))
	_ = os.RemoveAll(refRoot)
	_ = os.MkdirAll(refRoot, 0o755)
	if src := adapter.TaskFiles(taskID); src != "" {
		_ = copyDirFiltered(src, filepath.Join(refRoot, "source"), adapter.Spec().LeakGlobs)
	}
	surface := adapter.Substrate().Surface()
	chosenTree := trees[sel.ChosenLabel]
	if chosenTree == nil {
		chosenTree = tree.ReadTree(surface, filepath.Join(workDir, "harnesses", sel.ChosenLabel), nil)
	}
	_, _ = tree.WriteTree(surface, chosenTree, filepath.Join(refRoot, "harness"), false)
	_, _ = tree.WriteTree(surface, baseTree, filepath.Join(refRoot, "base_harness"), false)
	_ = os.WriteFile(filepath.Join(refRoot, "adoption.json"), []byte(fmt.Sprintf(`{"label":%q,"run_id":%q}`, sel.ChosenLabel, sel.ChosenRunID)), 0o644)
	rDir := filepath.Join(refRoot, "rollouts")
	_ = os.MkdirAll(rDir, 0o755)
	for _, br := range base {
		_ = os.WriteFile(filepath.Join(rDir, br.Label+".md"), []byte(adapter.Trajectory(br.RunDir, adapter.Spec().ObsTrunc)), 0o644)
	}
	if fr, ok := rollouts[sel.ChosenLabel]; ok {
		_ = os.WriteFile(filepath.Join(rDir, fr.Label+".md"), []byte(adapter.Trajectory(fr.RunDir, adapter.Spec().ObsTrunc)), 0o644)
	}
	// Pool diffs for INV/DOM stages present.
	pDir := filepath.Join(refRoot, "pool")
	for label, t := range trees {
		if strings.HasPrefix(label, "inv") || strings.HasPrefix(label, "dom") {
			item := filepath.Join(pDir, label)
			_ = os.MkdirAll(item, 0o755)
			diff := tree.DiffSummary(surface, baseTree, t)
			_ = os.WriteFile(filepath.Join(item, "harness.diff"), []byte(fmt.Sprintf("%v\n", diff)), 0o644)
		}
	}
	dDir := filepath.Join(refRoot, "decisions")
	_ = os.MkdirAll(dDir, 0o755)
	_ = os.WriteFile(filepath.Join(dDir, "c1.md"), []byte(sel.Why), 0o644)

	distilExperience(&sel, prop, bp, refRoot)
	return sel
}

// ReflectAdoption runs post-adoption reflection (Python selection.reflect_adoption).
// Used for k=1 trivial selection; k>1 reflection already runs inside Run.
func ReflectAdoption(
	taskID string, sel *Selection, rollouts map[string]spec.RunView,
	adapter spec.BenchmarkAdapter, prop proposer.Caller, workDir string,
	baseTree tree.Tree, trees map[string]tree.Tree,
) {
	if sel == nil || !sel.OK {
		return
	}
	chosen, ok := rollouts[sel.ChosenLabel]
	if !ok {
		return
	}
	_ = chosen
	mode, k := sel.Mode, sel.K
	refRoot := filepath.Join(workDir, "reflection", fmt.Sprintf("%s%d", mode, k))
	_ = os.RemoveAll(refRoot)
	_ = os.MkdirAll(refRoot, 0o755)
	if src := adapter.TaskFiles(taskID); src != "" {
		_ = copyDirFiltered(src, filepath.Join(refRoot, "source"), adapter.Spec().LeakGlobs)
	}
	surface := adapter.Substrate().Surface()
	chosenTree := trees[sel.ChosenLabel]
	if chosenTree == nil {
		chosenTree = tree.ReadTree(surface, filepath.Join(workDir, "harnesses", sel.ChosenLabel), nil)
	}
	if chosenTree == nil {
		chosenTree = baseTree
	}
	_, _ = tree.WriteTree(surface, chosenTree, filepath.Join(refRoot, "harness"), false)
	_, _ = tree.WriteTree(surface, baseTree, filepath.Join(refRoot, "base_harness"), false)
	_ = os.WriteFile(filepath.Join(refRoot, "adoption.json"), []byte(fmt.Sprintf(
		`{"chosen_label":%q,"candidate_ids":%s,"reason":%q}`,
		sel.ChosenLabel, jsonString(sel.IDMap), sel.Why)), 0o644)
	rDir := filepath.Join(refRoot, "rollouts")
	_ = os.MkdirAll(rDir, 0o755)
	for lbl, fr := range rollouts {
		_ = os.WriteFile(filepath.Join(rDir, lbl+".md"), []byte(adapter.Trajectory(fr.RunDir, adapter.Spec().ObsTrunc)), 0o644)
	}
	bp := spec.ResolvePrompts(adapter, taskID)
	if err := context.AssertTreeHasNoGraderVerdicts(chosenTree); err != nil {
		sel.Experience = map[string]string{}
		return
	}
	distilExperience(sel, prop, bp, refRoot)
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func distilExperience(sel *Selection, prop proposer.Caller, bp spec.BenchPrompts, refRoot string) {
	expReply, err := prop.Call(prompts.Reflect(bp), refRoot, fmt.Sprintf("experience.%s%d", sel.Mode, sel.K), "", 2)
	if err != nil {
		if pe, ok := err.(*proposer.Error); ok {
			sel.USD += pe.CostUSD
		}
		sel.Experience = map[string]string{}
		return
	}
	sel.USD += expReply.CostUSD
	if strings.Contains(expReply.Text, "NO_ENTRY") {
		sel.Experience = map[string]string{}
		return
	}
	exp := map[string]string{}
	if em := expRE.FindStringSubmatch(expReply.Text); em != nil {
		body := em[1]
		for _, key := range []string{"cue", "lesson", "evidence", "confidence"} {
			re := regexp.MustCompile(`(?i)` + key + `:\s*(.+)`)
			if m := re.FindStringSubmatch(body); m != nil {
				exp[key] = strings.TrimSpace(m[1])
			}
		}
	}
	if ed := editsRE.FindStringSubmatch(expReply.Text); ed != nil {
		exp["edits"] = strings.TrimSpace(ed[1])
	}
	if !(exp["cue"] != "" && exp["lesson"] != "" && exp["evidence"] != "") {
		sel.Experience = map[string]string{}
		return
	}
	sel.Experience = exp
}

func copyDirFiltered(src, dst string, leakGlobs []string) error {
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		base := filepath.Base(path)
		for _, g := range leakGlobs {
			if matched, _ := filepath.Match(g, base); matched {
				if de.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if de.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		return os.WriteFile(target, b, 0o644)
	})
}
