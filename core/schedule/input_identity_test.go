// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package schedule

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/runmark"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/substrate"
)

// fakeInputAdapter only implements the methods task-input identity reads.
type fakeInputAdapter struct {
	prompt string
	files  string
}

func (f fakeInputAdapter) Spec() spec.BenchSpec {
	return spec.BenchSpec{Name: "fake"}
}
func (f fakeInputAdapter) Substrate() spec.SubstrateLike { return substrate.Opencode{} }
func (f fakeInputAdapter) Tasks(string) []string         { return nil }
func (f fakeInputAdapter) DomainOf(string) string        { return "d" }
func (f fakeInputAdapter) TaskPrompt(string) string      { return f.prompt }
func (f fakeInputAdapter) TaskFiles(string) string       { return f.files }
func (f fakeInputAdapter) Execute(string, string, string, int) (string, error) {
	return "", nil
}
func (f fakeInputAdapter) Delivered(string) bool                   { return false }
func (f fakeInputAdapter) Trajectory(string, int) string           { return "" }
func (f fakeInputAdapter) DeliverableFiles(string) map[string]string { return nil }
func (f fakeInputAdapter) DeliverableText(string) string           { return "" }
func (f fakeInputAdapter) BaseHarness() string                     { return "" }
func (f fakeInputAdapter) ValidateTree(tree.Tree) error            { return nil }
func (f fakeInputAdapter) Score(string, string) (map[string]any, error) {
	return nil, nil
}

func taskInputs(a spec.BenchmarkAdapter, taskID string) string {
	return runmark.InputsFingerprint(a.TaskPrompt(taskID), a.TaskFiles(taskID))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestSummaryChangedSourceNotResumed(t *testing.T) {
	root := t.TempDir()
	files := filepath.Join(root, "documents")
	_ = os.MkdirAll(files, 0o755)
	_ = os.WriteFile(filepath.Join(files, "ledger.csv"), []byte("amount\n1.00\n"), 0o644)
	adapter := fakeInputAdapter{prompt: "Sum ledger.csv.", files: files}
	summary := filepath.Join(root, "summary.json")
	res := TaskResult{
		TaskID: "task-1", Domain: "d", K: 1, Status: "ok",
		Inputs:     taskInputs(adapter, "task-1"),
		Modes:      map[string]any{"agg": map[string]any{"ok": true}},
		Selections: map[string]any{"agg": map[string]any{"ok": true, "chosen_label": "c1"}},
	}
	if err := writeSummary(summary, res); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(files, "ledger.csv"), []byte("amount\n1.00\n2.00\n"), 0o644)
	fresh := TaskResult{TaskID: "task-1", Domain: "d", K: 1, Inputs: taskInputs(adapter, "task-1")}
	printed := captureStdout(t, func() {
		if tryResume(summary, "task-1", 1, adapter, root, []string{"agg"}, nil, "", nil, &fresh) {
			t.Fatal("expected no resume after source change")
		}
	})
	if !strings.Contains(printed, "changed since this summary was written") {
		t.Fatalf("printed=%q", printed)
	}
}

func TestSummaryChangedPromptNotResumed(t *testing.T) {
	root := t.TempDir()
	files := filepath.Join(root, "documents")
	_ = os.MkdirAll(files, 0o755)
	_ = os.WriteFile(filepath.Join(files, "ledger.csv"), []byte("amount\n1.00\n"), 0o644)
	adapter := fakeInputAdapter{prompt: "Sum ledger.csv.", files: files}
	summary := filepath.Join(root, "summary.json")
	res := TaskResult{
		TaskID: "task-1", Domain: "d", K: 1, Status: "ok",
		Inputs:     taskInputs(adapter, "task-1"),
		Modes:      map[string]any{"agg": map[string]any{"ok": true}},
		Selections: map[string]any{"agg": map[string]any{"ok": true, "chosen_label": "c1"}},
	}
	if err := writeSummary(summary, res); err != nil {
		t.Fatal(err)
	}
	changed := fakeInputAdapter{prompt: "Sum ledger.csv twice.", files: files}
	fresh := TaskResult{TaskID: "task-1", Domain: "d", K: 1, Inputs: taskInputs(changed, "task-1")}
	printed := captureStdout(t, func() {
		if tryResume(summary, "task-1", 1, changed, root, []string{"agg"}, nil, "", nil, &fresh) {
			t.Fatal("expected no resume after prompt change")
		}
	})
	if !strings.Contains(printed, "task inputs") {
		t.Fatalf("printed=%q", printed)
	}
}

func TestSummaryWithoutInputsNotInvalidatedByMessage(t *testing.T) {
	root := t.TempDir()
	files := filepath.Join(root, "documents")
	_ = os.MkdirAll(files, 0o755)
	adapter := fakeInputAdapter{prompt: "Sum ledger.csv.", files: files}
	summary := filepath.Join(root, "summary.json")
	if err := os.WriteFile(summary, []byte(`{"status":"ok"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := TaskResult{TaskID: "task-1", Domain: "d", K: 1, Inputs: taskInputs(adapter, "task-1")}
	printed := captureStdout(t, func() {
		if tryResume(summary, "task-1", 1, adapter, root, []string{"agg"}, nil, "", nil, &fresh) {
			t.Fatal("expected no resume for incomplete modes")
		}
	})
	if strings.Contains(printed, "task inputs") {
		t.Fatalf("legacy summary must not print input mismatch: %q", printed)
	}
}

func TestWriteSummaryDropsModesFromOlderInputIdentity(t *testing.T) {
	root := t.TempDir()
	files := filepath.Join(root, "documents")
	_ = os.MkdirAll(files, 0o755)
	_ = os.WriteFile(filepath.Join(files, "ledger.csv"), []byte("amount\n1.00\n"), 0o644)
	adapter := fakeInputAdapter{prompt: "Sum ledger.csv.", files: files}
	summary := filepath.Join(root, "summary.json")
	old := TaskResult{
		TaskID: "task-1", Domain: "d", K: 1, Status: "ok",
		Inputs: taskInputs(adapter, "task-1"),
		Modes: map[string]any{
			"agg": map[string]any{"ok": true},
			"seq": map[string]any{"ok": true},
		},
		Selections: map[string]any{
			"agg": map[string]any{"ok": true, "chosen_label": "c1"},
			"seq": map[string]any{"ok": true, "chosen_label": "c1"},
		},
	}
	b, _ := json.MarshalIndent(old, "", "  ")
	if err := os.WriteFile(summary, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(files, "ledger.csv"), []byte("amount\n1.00\n2.00\n"), 0o644)
	res := TaskResult{
		TaskID: "task-1", Domain: "d", K: 1, Status: "ok",
		Inputs:     taskInputs(adapter, "task-1"),
		Modes:      map[string]any{"agg": map[string]any{"ok": true}},
		Selections: map[string]any{"agg": map[string]any{"ok": true, "chosen_label": "c2"}},
	}
	if err := writeSummary(summary, res); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(summary)
	if err != nil {
		t.Fatal(err)
	}
	var merged TaskResult
	if err := json.Unmarshal(raw, &merged); err != nil {
		t.Fatal(err)
	}
	if merged.Inputs != res.Inputs {
		t.Fatalf("inputs=%s want %s", merged.Inputs, res.Inputs)
	}
	if _, ok := merged.Modes["agg"]; !ok {
		t.Fatal("agg missing")
	}
	if _, ok := merged.Modes["seq"]; ok {
		t.Fatal("stale seq mode must be dropped")
	}
	if _, ok := merged.Selections["seq"]; ok {
		t.Fatal("stale seq selection must be dropped")
	}
}
