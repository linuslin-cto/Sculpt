package workbuddy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/workbuddy"
	"sculpt/core/judge"
)

func TestScoreReadsVerifierScoreJSON(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.MkdirAll(filepath.Join(run, "verifier"), 0o755)
	payload := map[string]any{"reward": 0.5}
	b, _ := json.Marshal(payload)
	_ = os.WriteFile(filepath.Join(run, "verifier", "score.json"), b, 0o644)

	got, err := a.Score(run, "finance/t1")
	if err != nil {
		t.Fatal(err)
	}
	if got["n_passed"] != 50 || got["n_criteria"] != 100 || got["all_pass"] != false {
		t.Fatalf("%#v", got)
	}
	scored := judge.One(a, "finance/t1", "agg", 1, "agg1", "r1", run, false, "")
	if !scored.Graded() || *scored.NPassed != 50 {
		t.Fatalf("%#v", scored)
	}
}

func TestScoreTestsPassedRatio(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.MkdirAll(filepath.Join(run, "verifier"), 0o755)
	payload := map[string]any{"tests_passed": 2, "tests_total": 4}
	b, _ := json.Marshal(payload)
	_ = os.WriteFile(filepath.Join(run, "verifier", "score.json"), b, 0o644)
	got, err := a.Score(run, "t")
	if err != nil {
		t.Fatal(err)
	}
	if got["n_passed"] != 50 {
		t.Fatalf("%#v", got)
	}
}

func TestScoreRejectsOutOfRangeReward(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.MkdirAll(filepath.Join(run, "verifier"), 0o755)
	payload := map[string]any{"reward": 1.5}
	b, _ := json.Marshal(payload)
	_ = os.WriteFile(filepath.Join(run, "verifier", "score.json"), b, 0o644)
	if _, err := a.Score(run, "t"); err == nil {
		t.Fatal("expected reject")
	}
}
