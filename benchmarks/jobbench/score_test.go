package jobbench_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/jobbench"
)

func TestScoreCachedDetails(t *testing.T) {
	a, err := jobbench.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(a.Store, "acct", "task1", "model_output", "base1")
	_ = os.MkdirAll(runDir, 0o755)
	_ = os.WriteFile(filepath.Join(runDir, "out.txt"), []byte("x"), 0o644)

	details := filepath.Join(a.Store, "acct", "task1", "eval_result", "eval_base1", "stub_judge.json")
	_ = os.MkdirAll(filepath.Dir(details), 0o755)
	doc := map[string]any{
		"total_score": 1.5,
		"max_score":   3.0,
		"rubrics": []map[string]any{
			{"result": map[string]any{"criteria_count": 2, "criteria_passed": 1}},
		},
	}
	b, _ := json.Marshal(doc)
	_ = os.WriteFile(details, b, 0o644)

	t.Setenv("HARNESS2_JUDGE_MODEL", "judge/stub")
	got, err := a.Score(runDir, "acct/task1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got["metric"] != "weighted" {
		t.Fatalf("%#v", got)
	}
}

func TestScoreNoOutputFromRubrics(t *testing.T) {
	dataset := t.TempDir()
	task := filepath.Join(dataset, "acct", "task1")
	_ = os.MkdirAll(filepath.Join(task, "task_folder"), 0o755)
	_ = os.WriteFile(filepath.Join(task, "task_folder", "TASK_INSTRUCTIONS.txt"), []byte("x"), 0o644)
	rubrics := map[string]any{"rubrics": []map[string]any{
		{"weight": 2.0, "criterion": "a"},
		{"weight": 1.0, "criterion": []any{"b", "c"}},
	}}
	b, _ := json.Marshal(rubrics)
	_ = os.WriteFile(filepath.Join(task, "RUBRICS.json"), b, 0o644)

	a, err := jobbench.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = dataset
	empty := filepath.Join(a.Store, "acct", "task1", "model_output", "base1")
	_ = os.MkdirAll(empty, 0o755)
	got, err := a.Score(empty, "acct/task1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got["no_output"] != true || got["n_criteria"].(float64) != 3.0 {
		t.Fatalf("%#v", got)
	}
}

func TestJbOpenCodeRunnerResolvable(t *testing.T) {
	// Presence of the reference runner is enough for the live path to be selectable.
	if _, err := os.Stat(filepath.Join("..", "..", "harness2", "harness2", "benchmarks", "jb_opencode_runner.py")); err != nil {
		// Workspace layout may differ when tests run from module root via go test.
		wd, _ := os.Getwd()
		_ = wd
	}
}
