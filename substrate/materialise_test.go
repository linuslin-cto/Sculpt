package substrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/substrate"
)

func TestOpencodeMaterialisePrivateCopy(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "systemprompt.md"), []byte("hi\n"), 0o644)
	inst, err := (substrate.Opencode{}).Materialise(src, "workspace_agents_md", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(inst.StagingDir) })
	if inst.StagingDir == src {
		t.Fatal("staging must not be the source tree")
	}
	if _, err := os.Stat(filepath.Join(inst.StagingDir, "systemprompt.md")); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(inst.StagingDir, "systemprompt.md"), []byte("mut\n"), 0o644)
	got, _ := os.ReadFile(filepath.Join(src, "systemprompt.md"))
	if string(got) != "hi\n" {
		t.Fatalf("source polluted: %q", got)
	}
}
