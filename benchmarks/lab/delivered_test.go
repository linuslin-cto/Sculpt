package lab_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/lab"
)

func TestDeliveredRequiresOutputFile(t *testing.T) {
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	if a.Delivered(run) {
		t.Fatal("empty run should not be delivered")
	}
	_ = os.MkdirAll(filepath.Join(run, "output"), 0o755)
	if a.Delivered(run) {
		t.Fatal("empty output/ should not be delivered")
	}
	_ = os.WriteFile(filepath.Join(run, "output", "report.txt"), []byte("hi"), 0o644)
	if !a.Delivered(run) {
		t.Fatal("expected delivered")
	}
}
