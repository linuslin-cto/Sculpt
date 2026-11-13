package tree_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/tree"
	"sculpt/substrate"
)

func TestNormalizeAndAllow(t *testing.T) {
	s := substrate.OpenCodeSurface
	if _, err := tree.NormalizeRel("../x"); err == nil {
		t.Fatal("expected escape error")
	}
	if !tree.IsAllowed(s, "systemprompt.md") {
		t.Fatal("always-on should be allowed")
	}
	if !tree.IsAllowed(s, "scripts/summarize.py") {
		t.Fatal("script should be allowed")
	}
	if tree.IsAllowed(s, "secrets/key.txt") {
		t.Fatal("off-surface should be rejected")
	}
}

func TestWriteReadValidate(t *testing.T) {
	s := substrate.OpenCodeSurface
	dir := t.TempDir()
	tr := tree.Tree{
		"systemprompt.md":                     "hello",
		"guardrails.md":                       "",
		"LongTermMEMORY.md":                   "",
		"tool_descriptions/tool_guidance.md":  "",
		"scripts/summarize.py":                "print(1)\n",
	}
	if _, err := tree.WriteTree(s, tr, dir, false); err != nil {
		t.Fatal(err)
	}
	got := tree.ReadTree(s, dir, nil)
	if got["scripts/summarize.py"] != "print(1)\n" {
		t.Fatalf("roundtrip failed: %#v", got["scripts/summarize.py"])
	}
	if _, err := tree.Validate(s, got, true, 0, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDiffAndDiversity(t *testing.T) {
	s := substrate.OpenCodeSurface
	a := tree.Tree{"systemprompt.md": "a"}
	b := tree.Tree{"systemprompt.md": "b", "scripts/x.py": "1"}
	d := tree.DiffSummary(s, a, b)
	if len(d["modified"].([]string)) != 1 || len(d["added"].([]string)) != 1 {
		t.Fatalf("diff %#v", d)
	}
	div := tree.Diversity(map[string]tree.Tree{"a": a, "b": b})
	if div["n"].(int) != 2 {
		t.Fatalf("diversity %#v", div)
	}
}

func TestPathEscapeWrite(t *testing.T) {
	dir := t.TempDir()
	s := substrate.OpenCodeSurface
	_, err := tree.WriteTree(s, tree.Tree{"../escape.txt": "x"}, filepath.Join(dir, "h"), false)
	if err == nil {
		t.Fatal("expected escape error")
	}
	_ = os.RemoveAll(dir)
}
