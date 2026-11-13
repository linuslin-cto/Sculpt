package tasks_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/tasks"
)

func TestDiscoverJB(t *testing.T) {
	root := t.TempDir()
	mk := func(prof, task string) {
		dir := filepath.Join(root, prof, task, "task_folder")
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "TASK_INSTRUCTIONS.txt"), []byte("do it"), 0o644)
	}
	mk("acct", "task1")
	mk("acct", "task2")
	mk("eng", "task1")
	got, err := tasks.Discover("jb", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["acct/task1"] != "acct" || got["eng/task1"] != "eng" {
		t.Fatalf("%#v", got)
	}
	ids, err := tasks.Select(got, "all", "", "jb")
	if err != nil || len(ids) != 3 || ids[0] != "acct/task1" {
		t.Fatalf("%v %v", ids, err)
	}
}

func TestDiscoverLab(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "legal", "case-1")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "task.json"), []byte(`{}`), 0o644)
	got, err := tasks.Discover("lab", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if got["legal/case-1"] != "legal" {
		t.Fatalf("%#v", got)
	}
}

func TestSelectSplitFile(t *testing.T) {
	tasksMap := map[string]string{"a/t1": "a", "a/t2": "a", "b/t1": "b"}
	path := filepath.Join(t.TempDir(), "split.json")
	_ = os.WriteFile(path, []byte(`{"benchmark":"jb","train":["a/t1"],"test":["b/t1"]}`), 0o644)
	train, err := tasks.Select(tasksMap, "train", path, "jb")
	if err != nil || len(train) != 1 || train[0] != "a/t1" {
		t.Fatal(train, err)
	}
	all, err := tasks.Select(tasksMap, "all", path, "jb")
	if err != nil || len(all) != 2 {
		t.Fatal(all, err)
	}
}
