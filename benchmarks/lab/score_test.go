package lab_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/lab"
	"sculpt/core/judge"
)

func TestScoreReadsCachedScoresJSON(t *testing.T) {
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	payload := map[string]any{"n_passed": 1, "n_criteria": 2, "all_pass": false}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(run, "scores.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := a.Score(run, "contracts/task-1")
	if err != nil {
		t.Fatal(err)
	}
	if got["n_passed"] != float64(1) || got["n_criteria"] != float64(2) {
		t.Fatalf("%#v", got)
	}
	scored := judge.One(a, "contracts/task-1", "agg", 1, "agg1", "r1", run, false, "")
	if !scored.Graded() || *scored.NPassed != 1 || *scored.NCriteria != 2 {
		t.Fatalf("%#v", scored)
	}
}

func TestScoreMissingCacheIsUngraded(t *testing.T) {
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_, err = a.Score(run, "contracts/task-1")
	if err == nil {
		t.Fatal("expected cache miss error")
	}
	scored := judge.One(a, "contracts/task-1", "agg", 1, "agg1", "r1", run, false, "")
	if scored.Graded() {
		t.Fatalf("missing scores must be ungraded: %#v", scored)
	}
}

func TestScoreNonPositiveCriteriaUngraded(t *testing.T) {
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	for _, criteria := range []any{nil, 0, -1} {
		run := t.TempDir()
		payload := map[string]any{"n_passed": 0, "all_pass": false}
		if criteria != nil {
			payload["n_criteria"] = criteria
		}
		b, _ := json.Marshal(payload)
		_ = os.WriteFile(filepath.Join(run, "scores.json"), b, 0o644)
		scored := judge.One(a, "t", "agg", 1, "agg1", "r1", run, false, "")
		if scored.Graded() {
			t.Fatalf("criteria=%v should be ungraded: %#v", criteria, scored)
		}
	}
}
