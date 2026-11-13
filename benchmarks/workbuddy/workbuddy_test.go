package workbuddy_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/workbuddy"
)

func TestStubConstructs(t *testing.T) {
	a, err := workbuddy.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	if a.Spec().Name != "wb-oc" {
		t.Fatal(a.Spec().Name)
	}
	if len(workbuddy.ReadyErrors(a)) < 1 {
		t.Fatal(workbuddy.ReadyErrors(a))
	}
}

func TestTasksFromRoot(t *testing.T) {
	root := t.TempDir()
	task := filepath.Join(root, "t1")
	_ = os.MkdirAll(task, 0o755)
	_ = os.WriteFile(filepath.Join(task, "task.toml"), []byte("[metadata]\ncategory = \"finance\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(task, "instruction.md"), []byte("do work"), 0o644)

	a, err := workbuddy.NewSubset(t.TempDir(), "oc", "office")
	if err != nil {
		t.Fatal(err)
	}
	a.TasksRoot = root
	ids := a.Tasks("all")
	if len(ids) != 1 || ids[0] != "t1" {
		t.Fatalf("%v", ids)
	}
	if a.DomainOf("t1") != "finance" {
		t.Fatal(a.DomainOf("t1"))
	}
}
