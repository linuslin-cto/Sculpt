package judge_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/spec"
	"sculpt/core/tree"
)

type stubAdapter struct {
	score map[string]any
	err   error
}

func (s stubAdapter) Spec() spec.BenchSpec          { return spec.BenchSpec{} }
func (s stubAdapter) Substrate() spec.SubstrateLike { return nil }
func (s stubAdapter) Tasks(string) []string         { return nil }
func (s stubAdapter) DomainOf(string) string        { return "" }
func (s stubAdapter) TaskPrompt(string) string      { return "" }
func (s stubAdapter) TaskFiles(string) string       { return "" }
func (s stubAdapter) Execute(string, string, string, int) (string, error) {
	return "", nil
}
func (s stubAdapter) Delivered(string) bool                   { return true }
func (s stubAdapter) Trajectory(string, int) string            { return "" }
func (s stubAdapter) DeliverableFiles(string) map[string]string { return nil }
func (s stubAdapter) DeliverableText(string) string            { return "" }
func (s stubAdapter) BaseHarness() string                     { return "" }
func (s stubAdapter) ValidateTree(tree.Tree) error            { return nil }
func (s stubAdapter) Score(string, string) (map[string]any, error) {
	return s.score, s.err
}

func TestJudgeOneGraded(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "run"), 0o755)
	np, nc := 3, 4
	a := stubAdapter{score: map[string]any{"n_passed": np, "n_criteria": nc}}
	s := judge.One(a, "t", "agg", 1, "agg1", "rid", filepath.Join(dir, "run"), false, "")
	if !s.Graded() || *s.NPassed != 3 || s.AllPass {
		t.Fatalf("%#v", s)
	}
	if s.Frac() == nil || *s.Frac() != 0.75 {
		t.Fatalf("frac %#v", s.Frac())
	}
}

func TestJudgeNilIsUngraded(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "run"), 0o755)
	s := judge.One(stubAdapter{}, "t", "seq", 1, "c1", "rid", filepath.Join(dir, "run"), false, "")
	if s.Graded() || s.Error == "" {
		t.Fatalf("nil judge must be ungraded: %#v", s)
	}
}

func TestHasGrade(t *testing.T) {
	z := 0
	n := 1
	if judge.HasGrade(&z, &z) || !judge.HasGrade(&z, &n) {
		t.Fatal("zero criteria invalid; zero passes with positive criteria valid")
	}
}
