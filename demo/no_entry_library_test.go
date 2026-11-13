package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/library"
	"sculpt/core/proposer"
	"sculpt/core/schedule"
	"sculpt/demo"
)

type noExperienceProposer struct {
	*demo.Proposer
}

func (p *noExperienceProposer) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	if strings.HasPrefix(label, "experience.") {
		if strings.HasSuffix(label, "seq1") {
			return proposer.Reply{}, &proposer.Error{Message: "reflection failed", Attempts: 2, CostUSD: 2}
		}
		return proposer.Reply{Label: label, Text: "NO_ENTRY", OK: true, CostUSD: 1, Attempts: 1}, nil
	}
	return p.Proposer.Call(mission, contextDir, label, harvest, maxAttempts)
}

func TestNoEntryStaysOutOfLibraryOnResume(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := &noExperienceProposer{Proposer: demo.NewProposer(adapter.Spec().Channels["proposer"])}
	libRoot := filepath.Join(root, "library", "1")
	run := func() {
		_ = schedule.RunDomain("accounting", adapter.Tasks("all"), 1, adapter, prop, adapter.Spec().RunsRoot, libRoot, nil)
	}
	run()
	run()
	for _, mode := range []string{"agg", "seq"} {
		lib := &library.Library{Root: filepath.Join(libRoot, mode), Domain: "accounting", Surface: adapter.Substrate().Surface()}
		if ents := lib.Entries(); len(ents) != 0 {
			t.Fatalf("%s library entries %#v", mode, ents)
		}
	}
	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1", "ledger-1", "summary.json")
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}
	sels := summary["selections"].(map[string]any)
	aggUSD := sels["agg"].(map[string]any)["usd"].(float64)
	seqUSD := sels["seq"].(map[string]any)["usd"].(float64)
	if aggUSD != 1 || seqUSD != 2 {
		t.Fatalf("usd agg=%v seq=%v want 1,2", aggUSD, seqUSD)
	}
}
