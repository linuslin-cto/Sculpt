package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/proposer"
	"sculpt/core/schedule"
	"sculpt/demo"
)

type failedSelector struct {
	*demo.Proposer
	scoreCalls int
}

func (p *failedSelector) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	if strings.HasPrefix(label, "select.") {
		return proposer.Reply{}, &proposer.Error{Message: "selection failed", Attempts: 2, CostUSD: 3}
	}
	return p.Proposer.Call(mission, contextDir, label, harvest, maxAttempts)
}

func TestFailedSelectionIsNeverJudged(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := &failedSelector{Proposer: demo.NewProposer(adapter.Spec().Channels["proposer"])}
	_ = schedule.RunDomain("accounting", []string{"ledger-1"}, 2, adapter, prop, adapter.Spec().RunsRoot, filepath.Join(root, "library"), nil)

	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg2", "ledger-1", "summary.json")
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}
	sels := summary["selections"].(map[string]any)
	for _, mode := range []string{"agg", "seq"} {
		usd := sels[mode].(map[string]any)["usd"].(float64)
		if usd != 3 {
			t.Fatalf("%s usd=%v", mode, usd)
		}
		if sels[mode].(map[string]any)["ok"] != false {
			t.Fatalf("%s should not be ok", mode)
		}
	}
	var list []map[string]any
	for _, mode := range []string{"agg", "seq"} {
		list = append(list, sels[mode].(map[string]any))
	}
	scoring := &countingScoreAdapter{Adapter: adapter, calls: &prop.scoreCalls}
	rows := judge.Selections(scoring, list)
	if prop.scoreCalls != 0 {
		t.Fatalf("Score called %d times", prop.scoreCalls)
	}
	if len(rows) != 2 || rows[0].Graded() || rows[1].Graded() {
		t.Fatalf("%#v", rows)
	}
}

type countingScoreAdapter struct {
	*demo.Adapter
	calls *int
}

func (a *countingScoreAdapter) Score(runDir, taskID string) (map[string]any, error) {
	*a.calls++
	return map[string]any{"n_passed": 1, "n_criteria": 1}, nil
}
