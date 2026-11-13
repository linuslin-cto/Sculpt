package lab_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/benchmarks/lab"
)

func TestTrajectoryRendersOpenCodeSession(t *testing.T) {
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.WriteFile(filepath.Join(run, "opencode_session.jsonl"),
		[]byte(`{"type":"text","part":{"type":"text","text":"lab hello"}}`+"\n"), 0o644)
	got := a.Trajectory(run, 1000)
	if !strings.Contains(got, "[ASSISTANT] lab hello") {
		t.Fatalf("%q", got)
	}
}

func TestTrajectoryRendersCodexSession(t *testing.T) {
	a, err := lab.New(t.TempDir(), "cx")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.WriteFile(filepath.Join(run, "codex_session.jsonl"),
		[]byte(`{"type":"item","item":{"type":"agent_message","text":"cx hello"}}`+"\n"), 0o644)
	got := a.Trajectory(run, 1000)
	if !strings.Contains(got, "[ASSISTANT] cx hello") {
		t.Fatalf("%q", got)
	}
}

func TestTrajectoryMissingMessage(t *testing.T) {
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	got := a.Trajectory(run, 1000)
	if !strings.Contains(got, "(no trajectory recorded for") {
		t.Fatalf("%q", got)
	}
}
