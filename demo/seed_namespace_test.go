package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/schedule"
	"sculpt/demo"
)

func TestSeedEntersNamespaceAndSummary(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	seed := 7
	libRoot := filepath.Join(root, "library", "k1")
	result := schedule.RunDomain(
		"accounting", []string{"ledger-1"}, 1, adapter, prop,
		adapter.Spec().RunsRoot, libRoot, []string{"agg"},
		schedule.DomainOptions{Seed: &seed},
	)
	if result["seed"] != 7 {
		t.Fatalf("result seed %#v", result["seed"])
	}
	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1-s7", "ledger-1", "summary.json")
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["seed"] != float64(7) {
		t.Fatalf("summary seed %#v", summary["seed"])
	}
	if result["done"] != 1 {
		t.Fatalf("%#v", result)
	}
}
