package demo_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/eval"
	"sculpt/core/judge"
	"sculpt/core/schedule"
	"sculpt/demo"
)

// TestSelfEvolutionScored mirrors the experiment-guide pipeline:
// refine → judge selections → judge --bases → compare base vs refined scores.
func TestSelfEvolutionScored(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	_ = schedule.RunDomain(
		"accounting", adapter.Tasks("all"), 1, adapter, prop,
		adapter.Spec().RunsRoot, filepath.Join(root, "library", "k1"), nil,
	)
	nsDir := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1")
	modRoot := findModuleRoot(t)

	judgeBin := filepath.Join(t.TempDir(), "sculpt-judge"+exeSuffix())
	build := exec.Command("go", "build", "-o", judgeBin, "./cmd/sculpt-judge")
	build.Dir = modRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build judge: %v\n%s", err, out)
	}

	cmd := exec.Command(judgeBin, "--bench", "demo", "--output", root, "--k", "1",
		"--modes", "parallel,sequential")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("judge: %v\n%s", err, out)
	}
	rows, err := judge.LoadRows(filepath.Join(nsDir, "judged.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected graded selections, got %d", len(rows))
	}
	for _, r := range rows {
		if !r.Graded() || r.Frac() == nil || *r.Frac() != 1 {
			t.Fatalf("refined should be exact pass: %#v", r)
		}
	}

	baseCmd := exec.Command(judgeBin, "--bench", "demo", "--output", root, "--k", "1", "--bases")
	if out, err := baseCmd.CombinedOutput(); err != nil {
		// Exit 2 is allowed only when some bases ungraded; demo should grade all.
		t.Fatalf("judge --bases: %v\n%s", err, out)
	}
	baseRows, err := eval.LoadBaseRows(filepath.Join(nsDir, "base_judged.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(baseRows) == 0 {
		t.Fatal("empty base_judged.json")
	}
	baseMean := eval.MeanFrac(baseRows)
	if baseMean == nil || *baseMean != 0 {
		t.Fatalf("base should score 0%% (positive-only bug), got %v", baseMean)
	}

	comps := eval.Compare(baseRows, rows, nil, nil, "demo")
	if len(comps) == 0 {
		t.Fatal("no comparisons")
	}
	for _, c := range comps {
		if c.DeltaPts == nil || *c.DeltaPts != 100 {
			t.Fatalf("expected +100 pt self-evolution, got %#v", c)
		}
		if c.RefinedScore == nil || *c.RefinedScore != 100 {
			t.Fatalf("refined %#v", c)
		}
	}

	reportBin := filepath.Join(t.TempDir(), "sculpt-report"+exeSuffix())
	rb := exec.Command("go", "build", "-o", reportBin, "./cmd/sculpt-report")
	rb.Dir = modRoot
	if out, err := rb.CombinedOutput(); err != nil {
		t.Fatalf("build report: %v\n%s", err, out)
	}
	rep := exec.Command(reportBin, "--bench", "demo", "--output", root, "--k", "1")
	out, err := rep.CombinedOutput()
	if err != nil {
		t.Fatalf("report: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "Base → refined") || !strings.Contains(text, "+100.00 pt") {
		t.Fatalf("report missing delta block:\n%s", text)
	}
}
