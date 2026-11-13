package jobbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sculpt/benchmarks/jobbench"
)

func TestTrajectoryRendersNewestJSONL(t *testing.T) {
	runs := t.TempDir()
	a, err := jobbench.New(runs, "oc")
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(a.Store, "acct", "task1", "model_output", "base1")
	trajDir := filepath.Join(a.Store, "acct", "task1", "model_traj", "base1")
	_ = os.MkdirAll(runDir, 0o755)
	_ = os.MkdirAll(trajDir, 0o755)
	old := filepath.Join(trajDir, "a1.jsonl")
	newer := filepath.Join(trajDir, "a2.jsonl")
	_ = os.WriteFile(old, []byte(`{"type":"error","error":{"data":{"message":"old"}}}`+"\n"), 0o644)
	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(newer, []byte(`{"type":"error","error":{"data":{"message":"Rollout runner: stopped"}}}`+"\n"), 0o644)

	got := a.Trajectory(runDir, 1000)
	if !strings.Contains(got, "[ERROR] Rollout runner: stopped") {
		t.Fatalf("%q", got)
	}
	if strings.Contains(got, "old") {
		t.Fatalf("should prefer newest: %q", got)
	}
}

func TestTrajectoryMissingMessage(t *testing.T) {
	runs := t.TempDir()
	a, err := jobbench.New(runs, "oc")
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(a.Store, "acct", "task1", "model_output", "base1")
	_ = os.MkdirAll(runDir, 0o755)
	got := a.Trajectory(runDir, 1000)
	if !strings.Contains(got, "(no trajectory for acct/task1 @ base1)") {
		t.Fatalf("%q", got)
	}
}
