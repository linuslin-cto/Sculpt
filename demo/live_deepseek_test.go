package demo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/proposer"
	"sculpt/core/spec"
	"sculpt/demo"
)

func deepSeekConfigured() bool {
	return os.Getenv("DEEPSEEK_API_KEY") != "" || os.Getenv("OPENAI_API_KEY") != ""
}

func TestLiveDeepSeekPing(t *testing.T) {
	if !deepSeekConfigured() {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	ch, err := spec.NewChannelSpec("opencode", "deepseek/deepseek-chat", "high", 32, 120)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := proposer.NewDeepSeek(ch)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := ds.Call("Reply with exactly: PONG", "", "ping", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(reply.Text), "PONG") {
		t.Fatalf("unexpected reply %q", reply.Text)
	}
}

// TestLiveSelfEvolutionDeepSeek uses the real DeepSeek model to repair the
// buggy base summarizer, then scores base vs repaired (live self-evolution smoke).
func TestLiveSelfEvolutionDeepSeek(t *testing.T) {
	if !deepSeekConfigured() {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}

	baseRun, err := adapter.Execute("ledger-1", adapter.BaseHarness(), "base-only", 10)
	if err != nil {
		t.Fatal(err)
	}
	baseScore, err := adapter.Score(baseRun, "ledger-1")
	if err != nil {
		t.Fatal(err)
	}
	if baseScore["n_passed"].(int) != 0 {
		t.Fatalf("base should fail, got %#v", baseScore)
	}

	ch, err := spec.NewChannelSpec("opencode", "deepseek/deepseek-chat", "high", 64, 180)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := proposer.NewDeepSeek(ch)
	if err != nil {
		t.Fatal(err)
	}

	mission := `Fix scripts/summarize.py so it sums EVERY signed amount in the CSV
(including negative credits). The current script filters amount > 0 and is wrong.

Return ONLY a python fenced block with the full corrected scripts/summarize.py:
` + "```python\n...\n```"
	ctx := t.TempDir()
	_ = os.MkdirAll(filepath.Join(ctx, "harness", "scripts"), 0o755)
	_ = os.WriteFile(filepath.Join(ctx, "harness", "scripts", "summarize.py"), []byte(demo.BaseScript), 0o644)
	_ = os.WriteFile(filepath.Join(ctx, "ledger.csv"), []byte("amount\n100.25\n-20.10\n5.00\n"), 0o644)

	reply, err := ds.Call(mission, ctx, "live.fix", "harness", 2)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Workspace == "" {
		t.Fatal("expected harvested workspace with fixed script")
	}
	fixed := filepath.Join(reply.Workspace, "scripts", "summarize.py")
	b, err := os.ReadFile(fixed)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == demo.BaseScript || strings.Contains(string(b), "amount > 0") {
		t.Fatalf("model did not remove positive-only filter:\n%s", b)
	}

	liveRun, err := adapter.Execute("ledger-1", reply.Workspace, "live-fixed", 10)
	if err != nil {
		t.Fatal(err)
	}
	liveScore, err := adapter.Score(liveRun, "ledger-1")
	if err != nil {
		t.Fatal(err)
	}
	if liveScore["n_passed"].(int) != 1 {
		t.Fatalf("live-fixed should pass exact match, got %#v (out=%q)", liveScore, adapter.DeliverableText(liveRun))
	}
	t.Logf("live self-evolution: base 0/1 → repaired 1/1 (+100 pt); model=%s wall=%.1fs", ch.Model, reply.WallS)
}
