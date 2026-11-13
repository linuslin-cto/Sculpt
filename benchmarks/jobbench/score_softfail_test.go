package jobbench_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/jobbench"
)

func TestScoreSoftFailsWithoutJudgeConfig(t *testing.T) {
	dataset := t.TempDir()
	task := filepath.Join(dataset, "acct", "task1")
	_ = os.MkdirAll(filepath.Join(task, "task_folder"), 0o755)
	_ = os.WriteFile(filepath.Join(task, "task_folder", "TASK_INSTRUCTIONS.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(task, "RUBRICS.json"), []byte(`{"rubrics":[{"weight":1,"criterion":"a"}]}`), 0o644)

	a, err := jobbench.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = dataset
	runDir := filepath.Join(a.Store, "acct", "task1", "model_output", "base1")
	_ = os.MkdirAll(runDir, 0o755)
	_ = os.WriteFile(filepath.Join(runDir, "answer.txt"), []byte("delivered"), 0o644)

	t.Setenv("SCULPT_JUDGE_BASE_URL", "")
	t.Setenv("SCULPT_JUDGE_API_KEY", "")
	got, err := a.Score(runDir, "acct/task1")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil (judge soft-fail), got %#v", got)
	}
}
