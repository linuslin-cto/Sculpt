package selection_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/selection"
	"sculpt/core/spec"
	"sculpt/core/tree"
)

type stubAdapter struct{}

func (stubAdapter) Spec() spec.BenchSpec {
	return spec.BenchSpec{ObsTrunc: 50000}
}
func (stubAdapter) Substrate() spec.SubstrateLike                   { return nil }
func (stubAdapter) Tasks(string) []string                           { return nil }
func (stubAdapter) DomainOf(string) string                          { return "" }
func (stubAdapter) TaskPrompt(string) string                        { return "" }
func (stubAdapter) TaskFiles(string) string                         { return "" }
func (stubAdapter) Execute(string, string, string, int) (string, error) {
	return "", nil
}
func (stubAdapter) Delivered(string) bool { return true }
func (stubAdapter) Trajectory(string, int) string {
	return ""
}
func (stubAdapter) DeliverableFiles(string) map[string]string { return nil }
func (stubAdapter) DeliverableText(runDir string) string {
	b, _ := os.ReadFile(filepath.Join(runDir, "answer.txt"))
	return string(b)
}
func (stubAdapter) BaseHarness() string          { return "" }
func (stubAdapter) ValidateTree(tree.Tree) error { return nil }
func (stubAdapter) Score(string, string) (map[string]any, error) {
	return nil, nil
}

func TestAssignIDsStable(t *testing.T) {
	a := selection.AssignIDs([]string{"agg2", "agg1"}, "t", 2, "agg")
	b := selection.AssignIDs([]string{"agg1", "agg2"}, "t", 2, "agg")
	if len(a) != 2 || a["A"] != b["A"] || a["B"] != b["B"] {
		t.Fatalf("%v vs %v", a, b)
	}
}

func TestSelectionPoolFinalsOnly(t *testing.T) {
	rollouts := map[string]spec.RunView{
		"base1": {Label: "base1"},
		"agg1":  {Label: "agg1"},
		"inv1":  {Label: "inv1"},
	}
	pool := selection.SelectionPool("agg", 1, rollouts)
	if len(pool) != 1 || pool["agg1"].Label != "agg1" {
		t.Fatalf("%#v", pool)
	}
}

func TestTrivialSelection(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "run")
	_ = os.MkdirAll(run, 0o755)
	_ = os.WriteFile(filepath.Join(run, "answer.txt"), []byte("Total: 1.00\n"), 0o644)
	cands := map[string]spec.RunView{
		"agg1": {Label: "agg1", RunID: "r1", RunDir: run, OK: true, Delivered: true},
	}
	sel, err := selection.TrivialSelection("t", "agg", 1, cands, stubAdapter{}, dir)
	if err != nil || !sel.OK || !sel.Trivial || sel.ChosenID != "A" {
		t.Fatalf("%#v %v", sel, err)
	}
}
