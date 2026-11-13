package workbuddy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/benchmarks/workbuddy"
)

func TestDeliveredCountsArtifactBodies(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.MkdirAll(filepath.Join(run, "agent"), 0o755)
	_ = os.WriteFile(filepath.Join(run, "agent", "oc-output.txt"), []byte(strings.Repeat("x", 500)), 0o644)
	if a.Delivered(run) {
		t.Fatal("agent stream alone is not a deliverable")
	}
	_ = os.WriteFile(filepath.Join(run, "memo.docx"), []byte(strings.Repeat("a", 200)), 0o644)
	if !a.Delivered(run) {
		t.Fatal("expected delivered")
	}
}

func TestTrajectoryRendersOpenCodeCapture(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.MkdirAll(filepath.Join(run, "agent"), 0o755)
	_ = os.WriteFile(filepath.Join(run, "agent", "oc-output.txt"),
		[]byte(`{"type":"text","part":{"type":"text","text":"wb hello"}}`+"\n"), 0o644)
	got := a.Trajectory(run, 1000)
	if !strings.Contains(got, "[ASSISTANT] wb hello") {
		t.Fatalf("%q", got)
	}
}

func TestScoreRejectsKilledWithoutDeliverable(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	_ = os.MkdirAll(filepath.Join(run, "agent"), 0o755)
	_ = os.MkdirAll(filepath.Join(run, "verifier"), 0o755)
	_ = os.WriteFile(filepath.Join(run, "agent", "oc-output.txt"),
		[]byte(`{"type":"error","error":{"data":{"message":"cut short"}}}`+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(run, "verifier", "score.json"),
		[]byte(`{"reward":0}`), 0o644)
	if _, err := a.Score(run, "t"); err == nil {
		t.Fatal("killed undelivered rollout must soft-fail")
	}
}
