package demo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/library"
	"sculpt/core/proposer"
	"sculpt/core/schedule"
	"sculpt/core/tree"
	"sculpt/demo"
)

type paidReflectionProposer struct {
	*demo.Proposer
}

func (p *paidReflectionProposer) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	reply, err := p.Proposer.Call(mission, contextDir, label, harvest, maxAttempts)
	if err == nil && strings.HasPrefix(label, "experience.") {
		reply.CostUSD = 4.0
	}
	return reply, err
}

func TestLibraryAddFailurePreservesUSDAndClearsSelection(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	libRoot := filepath.Join(root, "library", "1")
	_ = schedule.RunDomain("accounting", adapter.Tasks("all"), 1, adapter, prop, adapter.Spec().RunsRoot, libRoot, nil)

	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1", "ledger-1", "summary.json")
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var initial map[string]any
	if err := json.Unmarshal(raw, &initial); err != nil {
		t.Fatal(err)
	}
	chosen := initial["selections"].(map[string]any)["agg"].(map[string]any)["chosen_run_dir"].(string)
	if err := os.Remove(filepath.Join(chosen, "answer.txt")); err != nil {
		t.Fatal(err)
	}

	library.SetAddForTest(func(l *library.Library, taskID string, harness tree.Tree, why, label, runID string, experience map[string]string) (string, error) {
		_ = l
		_ = harness
		_ = why
		_ = label
		_ = runID
		_ = experience
		if taskID == "ledger-1" {
			return "", fmt.Errorf("library write failed")
		}
		return "", nil
	})
	defer library.SetAddForTest(nil)

	paid := &paidReflectionProposer{Proposer: demo.NewProposer(adapter.Spec().Channels["proposer"])}
	failed := schedule.RunDomain("accounting", adapter.Tasks("all"), 1, adapter, paid, adapter.Spec().RunsRoot, libRoot, nil)
	statuses := []string{
		failed["tasks"].([]map[string]any)[0]["status"].(string),
		failed["tasks"].([]map[string]any)[1]["status"].(string),
	}
	if statuses[0] != "partial" || statuses[1] != "skipped" {
		t.Fatalf("%v %#v", statuses, failed)
	}
	after := readSummary(t, summaryPath)
	for _, mode := range []string{"agg", "seq"} {
		sel := after["selections"].(map[string]any)[mode].(map[string]any)
		if sel["ok"] != false {
			t.Fatalf("%s still ok: %#v", mode, sel)
		}
		if sel["usd"].(float64) != 4.0 {
			t.Fatalf("%s usd=%v want 4", mode, sel["usd"])
		}
	}
}
