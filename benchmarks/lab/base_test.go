package lab_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/lab"
)

func TestBaseHarnessMaterialisesOpenCode(t *testing.T) {
	root := t.TempDir()
	a, err := lab.New(root, "oc")
	if err != nil {
		t.Fatal(err)
	}
	base := a.BaseHarness()
	if base == "" {
		t.Fatal("empty base harness")
	}
	if _, err := os.Stat(filepath.Join(base, "systemprompt.md")); err != nil {
		t.Fatal(err)
	}
	if a.BaseHarness() != base {
		t.Fatal("BaseHarness should be stable")
	}
	errs := lab.ReadyErrors(a)
	for _, e := range errs {
		if e == "lab base harness path not configured" {
			t.Fatalf("ReadyErrors still complains: %v", errs)
		}
	}
}

func TestBaseHarnessMaterialisesCodex(t *testing.T) {
	root := t.TempDir()
	a, err := lab.New(root, "cx")
	if err != nil {
		t.Fatal(err)
	}
	base := a.BaseHarness()
	if _, err := os.Stat(filepath.Join(base, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
}
