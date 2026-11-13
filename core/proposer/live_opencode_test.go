package proposer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/proposer"
)

func TestOpenCodeArgvShape(t *testing.T) {
	oc := t.TempDir()
	entry := filepath.Join(oc, "packages", "opencode", "src")
	_ = os.MkdirAll(entry, 0o755)
	_ = os.WriteFile(filepath.Join(entry, "index.ts"), []byte("// stub\n"), 0o644)

	argv, err := proposer.OpenCodeArgv("bun", oc, "/work", "provider/model", "high")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"bun", "run", "--cwd", "src/index.ts", "run", "--dir", "/work", "--model", "provider/model", "--format", "json", "--variant", "high"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestWriteOpenCodeSessionConfig(t *testing.T) {
	dir := t.TempDir()
	if err := proposer.WriteOpenCodeSessionConfig(dir, 42); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if !strings.Contains(string(b), `"steps":42`) && !strings.Contains(string(b), `"steps": 42`) {
		t.Fatalf("%s", b)
	}
}

func TestOpenCodeArgvMissingCheckout(t *testing.T) {
	_, err := proposer.OpenCodeArgv("bun", t.TempDir(), "/work", "m", "")
	if err == nil {
		t.Fatal("expected missing checkout error")
	}
}
