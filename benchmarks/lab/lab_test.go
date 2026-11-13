package lab_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/lab"
)

func TestStubConstructs(t *testing.T) {
	a, err := lab.New(t.TempDir(), "cx")
	if err != nil {
		t.Fatal(err)
	}
	if a.Spec().Name != "lab-cx" {
		t.Fatal(a.Spec().Name)
	}
	if len(lab.ReadyErrors(a)) < 1 {
		t.Fatal(lab.ReadyErrors(a))
	}
}

func TestTasksFromRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "legal", "case-1")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "task.json"), []byte(`{"instructions":"x"}`), 0o644)
	a, err := lab.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.TasksRoot = root
	ids := a.Tasks("all")
	if len(ids) != 1 || ids[0] != "legal/case-1" {
		t.Fatalf("%v", ids)
	}
	if a.DomainOf("legal/case-1") != "legal" {
		t.Fatal(a.DomainOf("legal/case-1"))
	}
}
