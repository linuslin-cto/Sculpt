// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package judge measures the run that selection chose, after it chose it.
// A judge failure is ungraded, never a fabricated zero.
package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sculpt/core/spec"
)

// HasGrade reports whether a score has a numerator and positive denominator.
func HasGrade(nPassed, nCriteria *int) bool {
	return nPassed != nil && nCriteria != nil && *nCriteria > 0
}

// Scored is one judged selection.
type Scored struct {
	TaskID       string         `json:"task_id"`
	Mode         string         `json:"mode"`
	K            int            `json:"k"`
	Label        string         `json:"label"`
	RunID        string         `json:"run_id"`
	ChosenIsBase bool           `json:"chosen_is_base"`
	SelectedAt   string         `json:"selected_at"`
	NPassed      *int           `json:"n_passed"`
	NCriteria    *int           `json:"n_criteria"`
	AllPass      bool           `json:"all_pass"`
	Raw          map[string]any `json:"raw"`
	Error        string         `json:"error"`
}

// Graded reports whether the judge succeeded (not whether the run scored well).
func (s Scored) Graded() bool { return HasGrade(s.NPassed, s.NCriteria) }

// Frac is n_passed / n_criteria when graded.
func (s Scored) Frac() *float64 {
	if !s.Graded() {
		return nil
	}
	v := float64(*s.NPassed) / float64(*s.NCriteria)
	return &v
}

// Selections judges a list of selection dicts. Failed selections (ok=false) are never scored.
func Selections(adapter spec.BenchmarkAdapter, sels []map[string]any) []Scored {
	var out []Scored
	for _, sel := range sels {
		ok, _ := sel["ok"].(bool)
		taskID, _ := sel["task_id"].(string)
		mode, _ := sel["mode"].(string)
		k := asIntDefault(sel["k"])
		label, _ := sel["chosen_label"].(string)
		runID, _ := sel["chosen_run_id"].(string)
		runDir, _ := sel["chosen_run_dir"].(string)
		chosenIsBase, _ := sel["chosen_is_base"].(bool)
		selectedAt, _ := sel["selected_at"].(string)
		if !ok {
			out = append(out, Scored{
				TaskID: taskID, Mode: mode, K: k, Label: label, RunID: runID,
				ChosenIsBase: chosenIsBase, SelectedAt: selectedAt,
				Error: "selection failed; not judged",
			})
			continue
		}
		out = append(out, One(adapter, taskID, mode, k, label, runID, runDir, chosenIsBase, selectedAt))
	}
	return out
}

func asIntDefault(v any) int {
	n, _ := asInt(v)
	return n
}

// One judges a single selected run via the adapter.Score seam.
func One(adapter spec.BenchmarkAdapter, taskID, mode string, k int, label, runID, runDir string, chosenIsBase bool, selectedAt string) Scored {
	s := Scored{
		TaskID: taskID, Mode: mode, K: k, Label: label, RunID: runID,
		ChosenIsBase: chosenIsBase, SelectedAt: selectedAt, Raw: map[string]any{},
	}
	fi, err := os.Stat(runDir)
	if err != nil || !fi.IsDir() {
		s.Error = "adopted run directory is missing"
		return s
	}
	raw, err := adapter.Score(runDir, taskID)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	if raw == nil {
		s.Error = "judge returned nil"
		return s
	}
	s.Raw = raw
	if v, ok := asInt(raw["n_passed"]); ok {
		s.NPassed = &v
	}
	if v, ok := asInt(raw["n_criteria"]); ok {
		s.NCriteria = &v
	}
	if b, ok := raw["all_pass"].(bool); ok {
		s.AllPass = b
	} else if s.Graded() {
		s.AllPass = *s.NPassed == *s.NCriteria
	}
	return s
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

// Merge keeps grades across failed re-judges of the same selection (Python cli.judge._merge).
// A changed chosen run or re-selected task (different selected_at) replaces; otherwise a
// graded prior row wins over an ungraded re-judge.
func Merge(old, newRows []Scored) []Scored {
	type key struct {
		TaskID string
		Mode   string
		K      int
	}
	by := map[key]Scored{}
	var order []key
	seen := map[key]struct{}{}
	for _, r := range old {
		k := key{r.TaskID, r.Mode, r.K}
		by[k] = r
		if _, ok := seen[k]; !ok {
			order = append(order, k)
			seen[k] = struct{}{}
		}
	}
	for _, r := range newRows {
		k := key{r.TaskID, r.Mode, r.K}
		prev, ok := by[k]
		if ok &&
			prev.RunID == r.RunID &&
			prev.SelectedAt == r.SelectedAt &&
			prev.Graded() &&
			!r.Graded() &&
			!strings.HasPrefix(r.Error, "selection") {
			continue
		}
		if _, ok := seen[k]; !ok {
			order = append(order, k)
			seen[k] = struct{}{}
		}
		by[k] = r
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if a.TaskID != b.TaskID {
			return a.TaskID < b.TaskID
		}
		if a.Mode != b.Mode {
			return a.Mode < b.Mode
		}
		return a.K < b.K
	})
	out := make([]Scored, 0, len(order))
	for _, k := range order {
		out = append(out, by[k])
	}
	return out
}

// CollectSelections loads every requested mode's selection from task summaries under nsDir.
func CollectSelections(nsDir string, modes []string) ([]map[string]any, error) {
	matches, err := filepath.Glob(filepath.Join(nsDir, "*", "summary.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var out []map[string]any
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var summary map[string]any
		if json.Unmarshal(b, &summary) != nil {
			continue
		}
		sels, _ := summary["selections"].(map[string]any)
		for _, mode := range modes {
			raw, ok := sels[mode]
			if !ok {
				continue
			}
			sel, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if _, has := sel["mode"]; !has {
				if arm, ok := sel["arm"].(string); ok {
					sel = copyMap(sel)
					sel["mode"] = arm
					delete(sel, "arm")
				}
			}
			out = append(out, sel)
		}
	}
	return out, nil
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Write persists Scored rows as judged.json (Python core.judge.write shape).
func Write(path string, rows []Scored) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// LoadRows rebuilds Scored rows from a judged.json cache (Python cli.judge._load_rows).
func LoadRows(path string) ([]Scored, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("unreadable judged cache %s: %w", path, err)
	}
	out := make([]Scored, 0, len(raw))
	for _, row := range raw {
		if _, ok := row["mode"]; !ok {
			if arm, ok := row["arm"].(string); ok {
				row["mode"] = arm
				delete(row, "arm")
			}
		}
		s := Scored{
			TaskID: asString(row["task_id"]),
			Mode:   asString(row["mode"]),
			K:      asIntDefault(row["k"]),
			Label:  asString(row["label"]),
			RunID:  asString(row["run_id"]),
			Error:  asString(row["error"]),
		}
		if v, ok := row["chosen_is_base"].(bool); ok {
			s.ChosenIsBase = v
		}
		s.SelectedAt = asString(row["selected_at"])
		if v, ok := asInt(row["n_passed"]); ok {
			s.NPassed = &v
		}
		if v, ok := asInt(row["n_criteria"]); ok {
			s.NCriteria = &v
		}
		if v, ok := row["all_pass"].(bool); ok {
			s.AllPass = v
		}
		if m, ok := row["raw"].(map[string]any); ok {
			s.Raw = m
		}
		out = append(out, s)
	}
	return out, nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
