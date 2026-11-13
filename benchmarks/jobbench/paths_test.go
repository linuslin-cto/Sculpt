package jobbench_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/jobbench"
	"sculpt/internal/config"
)

func TestBaseHarnessFromThirdParty(t *testing.T) {
	base := config.JbBaseHarness("oc")
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		t.Skipf("jb_runner baseline not present at %s", base)
	}
	a, err := jobbench.New(t.TempDir(), "oc")
	if err != nil {
		t.Fatal(err)
	}
	if a.BaseHarness() == "" {
		t.Fatal("expected BaseHarness auto-resolved from third_party/jb_runner")
	}
	if _, err := os.Stat(filepath.Join(a.BaseHarness(), "systemprompt.md")); err != nil {
		t.Fatal(err)
	}
}

func TestCodexRunnerPath(t *testing.T) {
	p := config.JbCodexRunner()
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		t.Skipf("codex runner missing: %s", p)
	}
}
