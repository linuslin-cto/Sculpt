package proposer_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/proposer"
	"sculpt/core/spec"
)

func TestOpenCodeRunnerHarvest(t *testing.T) {
	ch, err := spec.NewChannelSpec("opencode", "local/test", "high", 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	base, err := proposer.New(ch, t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	oc := proposer.NewOpenCode(base)
	oc.Runner = func(cwd, mission, system string, wallSec int, env []string) (string, error) {
		_ = mission
		_ = system
		_ = wallSec
		_ = env
		harness := filepath.Join(cwd, "context", "harness")
		_ = os.MkdirAll(harness, 0o755)
		_ = os.WriteFile(filepath.Join(harness, "systemprompt.md"), []byte("edited\n"), 0o644)
		return `{"type":"text","part":{"type":"text","text":"<notes>ok</notes>"}}` + "\n", nil
	}
	ctx := t.TempDir()
	_ = os.MkdirAll(filepath.Join(ctx, "harness"), 0o755)
	_ = os.WriteFile(filepath.Join(ctx, "harness", "systemprompt.md"), []byte("base\n"), 0o644)

	reply, err := oc.Call("edit", ctx, "inv1", "context/harness", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reply.OK || reply.Workspace == "" {
		t.Fatalf("%#v", reply)
	}
	got, _ := os.ReadFile(filepath.Join(reply.Workspace, "systemprompt.md"))
	if string(got) != "edited\n" {
		t.Fatalf("harvest=%q", got)
	}
}
