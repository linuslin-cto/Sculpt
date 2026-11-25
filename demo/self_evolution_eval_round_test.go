package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/eval"
	"sculpt/core/judge"
	"sculpt/core/schedule"
	"sculpt/demo"
)

// TestSelfEvolutionEvalRound is one offline self-evolution scored round:
// base fails → PPC refines → judge selected + base → Δ = +100 pt.
// Designed for forced multi-round stress: go test -run TestSelfEvolutionEvalRound -count=N
func TestSelfEvolutionEvalRound(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	libRoot := filepath.Join(root, "library", "k1")

	baseRun, err := adapter.Execute("ledger-1", adapter.BaseHarness(), "base-only", 10)
	if err != nil {
		t.Fatal(err)
	}
	baseScore, err := adapter.Score(baseRun, "ledger-1")
	if err != nil {
		t.Fatal(err)
	}
	if baseScore["n_passed"].(int) != 0 {
		t.Fatalf("base should fail exact match, got %#v", baseScore)
	}

	result := schedule.RunDomain(
		"accounting", adapter.Tasks("all"), 1, adapter, prop,
		adapter.Spec().RunsRoot, libRoot, nil,
	)
	if result["done"] != 2 || result["failed"] != 0 {
		t.Fatalf("stream %#v", result)
	}

	nsDir := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1")
	sels, err := judge.CollectSelections(nsDir, []string{"agg", "seq"})
	if err != nil {
		t.Fatal(err)
	}
	refined := judge.Selections(adapter, sels)
	if len(refined) == 0 {
		t.Fatal("no refined selections")
	}
	for _, r := range refined {
		if !r.Graded() || r.Frac() == nil || *r.Frac() != 1 {
			t.Fatalf("refined must be full pass: %#v", r)
		}
	}

	var baseRows []eval.BaseRow
	matches, _ := filepath.Glob(filepath.Join(nsDir, "*", "summary.json"))
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var summary map[string]any
		if json.Unmarshal(b, &summary) != nil {
			continue
		}
		taskID, _ := summary["task_id"].(string)
		base, _ := summary["base"].([]any)
		for _, entry := range base {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			runDir, _ := m["run_dir"].(string)
			label, _ := m["label"].(string)
			runID, _ := m["run_id"].(string)
			sc, err := adapter.Score(runDir, taskID)
			if err != nil || sc == nil {
				continue
			}
			baseRows = append(baseRows, eval.BaseRow{Task: taskID, Label: label, RunID: runID, Score: sc})
		}
	}
	if len(baseRows) == 0 {
		t.Fatal("no base rows scored")
	}
	if mean := eval.MeanFrac(baseRows); mean == nil || *mean != 0 {
		t.Fatalf("base mean want 0 got %v", mean)
	}

	comps := eval.Compare(baseRows, refined, nil, nil, "demo")
	if len(comps) == 0 {
		t.Fatal("no comparisons")
	}
	for _, c := range comps {
		if c.DeltaPts == nil || *c.DeltaPts != 100 {
			t.Fatalf("want +100 pt self-evolution, got %#v", c)
		}
	}
}
