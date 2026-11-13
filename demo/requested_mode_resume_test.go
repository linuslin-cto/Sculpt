package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/proposer"
	"sculpt/core/schedule"
	"sculpt/demo"
)

// sequentialSelectFailure fails only the sequential selector call (Python SequentialFailure).
type sequentialSelectFailure struct {
	*demo.Proposer
}

func (p *sequentialSelectFailure) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	if label == "select.seq2" {
		return proposer.Reply{}, &proposer.Error{Message: "temporary sequential selector failure", Attempts: 1}
	}
	return p.Proposer.Call(mission, contextDir, label, harvest, maxAttempts)
}

func TestValidParallelModeResumesWhenSequentialSelectionFailed(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := &sequentialSelectFailure{Proposer: demo.NewProposer(adapter.Spec().Channels["proposer"])}
	lib := filepath.Join(root, "library")

	run := func(modes []string) map[string]any {
		return schedule.RunDomain(
			"accounting", []string{"ledger-1"}, 2, adapter, prop,
			adapter.Spec().RunsRoot, lib, modes,
		)
	}

	first := run([]string{"agg", "seq"})
	tasks := first["tasks"].([]map[string]any)
	if tasks[0]["status"] != "partial" {
		t.Fatalf("%#v", first)
	}
	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg2", "ledger-1", "summary.json")
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	aggSel := record["selections"].(map[string]any)["agg"].(map[string]any)
	seqSel := record["selections"].(map[string]any)["seq"].(map[string]any)
	if aggSel["ok"] != true {
		t.Fatalf("agg selection %#v", aggSel)
	}
	if seqSel["ok"] != false {
		t.Fatalf("seq selection %#v", seqSel)
	}

	countsExec, countsCalls := adapter.ExecutionCount, len(prop.Calls())
	for i := 0; i < 2; i++ {
		got := run([]string{"agg"})
		row := got["tasks"].([]map[string]any)[0]
		if row["status"] != "skipped" {
			t.Fatalf("agg-only resume #%d: %#v", i, got)
		}
		if adapter.ExecutionCount != countsExec || len(prop.Calls()) != countsCalls {
			t.Fatalf("agg-only must not re-execute; exec=%d calls=%d", adapter.ExecutionCount, len(prop.Calls()))
		}
	}

	retry := run([]string{"seq"})
	if retry["tasks"].([]map[string]any)[0]["status"] != "partial" {
		t.Fatalf("%#v", retry)
	}
	if adapter.ExecutionCount <= countsExec {
		t.Fatal("requesting failed seq mode must retry")
	}
}
