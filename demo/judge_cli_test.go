package demo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/schedule"
	"sculpt/demo"
)

func TestJudgeCLIWritesMergedJudgedJSON(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	_ = schedule.RunDomain(
		"accounting", []string{"ledger-1"}, 1, adapter, prop,
		adapter.Spec().RunsRoot, filepath.Join(root, "library"), []string{"agg"},
	)
	nsDir := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1")

	modRoot := findModuleRoot(t)
	bin := filepath.Join(t.TempDir(), "harness2-judge"+exeSuffix())
	build := exec.Command("go", "build", "-o", bin, "./cmd/harness2-judge")
	build.Dir = modRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "--bench", "demo", "--output", root, "--k", "1", "--modes", "parallel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); !ok || (ee.ExitCode() != 0 && ee.ExitCode() != 2) {
			t.Fatalf("judge: %v\n%s", err, out)
		}
	}
	rows, err := judge.LoadRows(filepath.Join(nsDir, "judged.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TaskID != "ledger-1" || rows[0].Mode != "agg" {
		t.Fatalf("%#v\n%s", rows, out)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			t.Fatal("go.mod not found")
		}
	}
}

func exeSuffix() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}
