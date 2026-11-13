package jobbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/benchmarks/jobbench"
	"sculpt/substrate"
)

func TestStubConstructs(t *testing.T) {
	a, err := jobbench.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	if a.Spec().Name != "jb-oc" {
		t.Fatal(a.Spec().Name)
	}
	errs := jobbench.ReadyErrors(a)
	if len(errs) < 1 {
		t.Fatalf("%v", errs)
	}
}

func TestTasksFromDataset(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nurse", "task1", "task_folder")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "TASK_INSTRUCTIONS.txt"), []byte("care plan"), 0o644)

	a, err := jobbench.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = root
	ids := a.Tasks("all")
	if len(ids) != 1 || ids[0] != "nurse/task1" {
		t.Fatalf("%v", ids)
	}
	if a.DomainOf("nurse/task1") != "nurse" {
		t.Fatal(a.DomainOf("nurse/task1"))
	}
	if a.TaskPrompt("nurse/task1") != "care plan" {
		t.Fatal(a.TaskPrompt("nurse/task1"))
	}
}

func TestExecuteReuseWithLauncher(t *testing.T) {
	dataset := t.TempDir()
	taskFolder := filepath.Join(dataset, "acct", "task1", "task_folder")
	_ = os.MkdirAll(taskFolder, 0o755)
	_ = os.WriteFile(filepath.Join(taskFolder, "TASK_INSTRUCTIONS.txt"), []byte("sum"), 0o644)
	_ = os.WriteFile(filepath.Join(taskFolder, "ledger.csv"), []byte("1\n"), 0o644)

	runs := t.TempDir()
	harness := filepath.Join(t.TempDir(), "harness")
	_ = os.MkdirAll(harness, 0o755)
	_ = os.WriteFile(filepath.Join(harness, "systemprompt.md"), []byte("base\n"), 0o644)

	a, err := jobbench.New(runs, "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = dataset
	a.BaseHarnessPath = harness
	launches := 0
	a.SetLauncher(func(req substrate.RunRequest, install substrate.Install) ([]string, error) {
		launches++
		label := strings.ReplaceAll(req.RunIDs[0], "/", "__")
		out := filepath.Join(a.Store, "acct", "task1", "model_output", label)
		_ = os.MkdirAll(out, 0o755)
		_ = os.WriteFile(filepath.Join(out, "answer.txt"), []byte("ok"), 0o644)
		return []string{out}, nil
	})

	first, err := a.Execute("acct/task1", harness, "jb-oc-agg1/acct__task1/base1", 30)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Execute("acct/task1", harness, "jb-oc-agg1/acct__task1/base1", 30)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 {
		t.Fatalf("expected reuse, launches=%d", launches)
	}
	if first != second {
		t.Fatalf("%s vs %s", first, second)
	}
	if !a.Delivered(first) {
		t.Fatal("expected delivered")
	}
}

func TestExecuteRerollsOnInputChange(t *testing.T) {
	dataset := t.TempDir()
	taskFolder := filepath.Join(dataset, "acct", "task1", "task_folder")
	_ = os.MkdirAll(taskFolder, 0o755)
	_ = os.WriteFile(filepath.Join(taskFolder, "TASK_INSTRUCTIONS.txt"), []byte("v1"), 0o644)

	runs := t.TempDir()
	harness := filepath.Join(t.TempDir(), "harness")
	_ = os.MkdirAll(harness, 0o755)
	_ = os.WriteFile(filepath.Join(harness, "systemprompt.md"), []byte("base\n"), 0o644)

	a, err := jobbench.New(runs, "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = dataset
	a.BaseHarnessPath = harness
	launches := 0
	a.SetLauncher(func(req substrate.RunRequest, install substrate.Install) ([]string, error) {
		launches++
		label := strings.ReplaceAll(req.RunIDs[0], "/", "__")
		out := filepath.Join(a.Store, "acct", "task1", "model_output", label)
		_ = os.MkdirAll(out, 0o755)
		_ = os.WriteFile(filepath.Join(out, "answer.txt"), []byte(a.TaskPrompt("acct/task1")), 0o644)
		return []string{out}, nil
	})

	if _, err := a.Execute("acct/task1", harness, "ns/t/base1", 30); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(taskFolder, "TASK_INSTRUCTIONS.txt"), []byte("v2"), 0o644)
	out, err := a.Execute("acct/task1", harness, "ns/t/base1", 30)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 2 {
		t.Fatalf("launches=%d", launches)
	}
	got, _ := os.ReadFile(filepath.Join(out, "answer.txt"))
	if string(got) != "v2" {
		t.Fatalf("%q", got)
	}
}

func TestExecuteUsesPrivateStagingDir(t *testing.T) {
	dataset := t.TempDir()
	taskFolder := filepath.Join(dataset, "acct", "task1", "task_folder")
	_ = os.MkdirAll(taskFolder, 0o755)
	_ = os.WriteFile(filepath.Join(taskFolder, "TASK_INSTRUCTIONS.txt"), []byte("sum"), 0o644)

	runs := t.TempDir()
	harness := filepath.Join(t.TempDir(), "harness")
	_ = os.MkdirAll(harness, 0o755)
	_ = os.WriteFile(filepath.Join(harness, "systemprompt.md"), []byte("base\n"), 0o644)
	_ = os.WriteFile(filepath.Join(harness, "marker.txt"), []byte("src"), 0o644)

	a, err := jobbench.New(runs, "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = dataset
	a.BaseHarnessPath = harness
	var seenStaging string
	a.SetLauncher(func(req substrate.RunRequest, install substrate.Install) ([]string, error) {
		seenStaging = install.StagingDir
		if install.StagingDir == "" || install.StagingDir == harness {
			t.Fatalf("staging must be a private copy, got %q", install.StagingDir)
		}
		if _, err := os.Stat(filepath.Join(install.StagingDir, "marker.txt")); err != nil {
			t.Fatalf("staged tree missing marker: %v", err)
		}
		// Mutate staging; source harness must stay clean.
		_ = os.WriteFile(filepath.Join(install.StagingDir, "marker.txt"), []byte("mutated"), 0o644)
		label := strings.ReplaceAll(req.RunIDs[0], "/", "__")
		out := filepath.Join(a.Store, "acct", "task1", "model_output", label)
		_ = os.MkdirAll(out, 0o755)
		_ = os.WriteFile(filepath.Join(out, "answer.txt"), []byte("ok"), 0o644)
		return []string{out}, nil
	})

	if _, err := a.Execute("acct/task1", harness, "ns/t/base1", 30); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(filepath.Join(harness, "marker.txt"))
	if string(src) != "src" {
		t.Fatalf("source harness polluted: %q", src)
	}
	if seenStaging == "" {
		t.Fatal("launcher not called")
	}
	if _, err := os.Stat(seenStaging); !os.IsNotExist(err) {
		t.Fatalf("staging should be cleaned after execute, still at %s (%v)", seenStaging, err)
	}
}

func TestFailedExecuteLeavesUnmarked(t *testing.T) {
	dataset := t.TempDir()
	taskFolder := filepath.Join(dataset, "acct", "task1", "task_folder")
	_ = os.MkdirAll(taskFolder, 0o755)
	_ = os.WriteFile(filepath.Join(taskFolder, "TASK_INSTRUCTIONS.txt"), []byte("sum"), 0o644)

	runs := t.TempDir()
	harness := filepath.Join(t.TempDir(), "harness")
	_ = os.MkdirAll(harness, 0o755)
	_ = os.WriteFile(filepath.Join(harness, "systemprompt.md"), []byte("base\n"), 0o644)

	a, err := jobbench.New(runs, "oc")
	if err != nil {
		t.Fatal(err)
	}
	a.Dataset = dataset
	a.BaseHarnessPath = harness
	a.SetLauncher(func(req substrate.RunRequest, install substrate.Install) ([]string, error) {
		return nil, errLaunchBoom
	})
	_, err = a.Execute("acct/task1", harness, "ns/t/base1", 30)
	if err == nil {
		t.Fatal("expected launch error")
	}
	mark := filepath.Join(a.Store, "acct", "task1", "model_traj", "ns__t__base1")
	if runmarkComplete(mark) {
		t.Fatal("failed execute must not mark complete")
	}
}

var errLaunchBoom = errString("launcher boom")

type errString string

func (e errString) Error() string { return string(e) }

func runmarkComplete(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*"))
	return len(matches) > 0
}
