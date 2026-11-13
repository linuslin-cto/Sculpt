package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/library"
	"sculpt/core/schedule"
	"sculpt/demo"
)

// TestSelfEvolutionProposeProbeCompose mirrors the upstream offline
// evaluation contract: a buggy base harness is refined without ground-truth
// feedback into a correct deliverable, then experience is stored for the
// next task (task-level + harnessing-level recursion).
func TestSelfEvolutionProposeProbeCompose(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	libRoot := filepath.Join(root, "library", "k1")

	// 1) Base harness alone fails the signed-sum task (positive-only filter).
	baseRun, err := adapter.Execute("ledger-1", adapter.BaseHarness(), "base-only", 10)
	if err != nil {
		t.Fatal(err)
	}
	baseOut := adapter.DeliverableText(baseRun)
	if baseOut != "Total: 105.25\n" {
		t.Fatalf("base deliverable %q (expected positive-only Total: 105.25)", baseOut)
	}
	if baseOut == "Total: 85.15\n" {
		t.Fatal("base already correct; cannot demonstrate evolution")
	}

	// 2) One Propose–Probe–Compose domain stream (k=1) should adopt a fix.
	result := schedule.RunDomain(
		"accounting", adapter.Tasks("all"), 1, adapter, prop,
		adapter.Spec().RunsRoot, libRoot, nil,
	)
	if result["done"] != 2 || result["failed"] != 0 {
		t.Fatalf("stream result %#v", result)
	}

	for _, taskID := range []string{"ledger-1", "ledger-2"} {
		want := "Total: 85.15\n"
		if taskID == "ledger-2" {
			want = "Total: 20.25\n"
		}
		// Shared summary is anchored under the agg namespace (upstream schedule parity).
		summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1", taskID, "summary.json")
		b, err := os.ReadFile(summaryPath)
		if err != nil {
			t.Fatal(err)
		}
		var summary map[string]any
		if err := json.Unmarshal(b, &summary); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"agg", "seq"} {
			ns := "demo-oc-" + mode + "1"
			sel := summary["selections"].(map[string]any)[mode].(map[string]any)
			label, _ := sel["chosen_label"].(string)
			runDir, _ := sel["chosen_run_dir"].(string)
			got := adapter.DeliverableText(runDir)
			if got != want {
				t.Fatalf("%s/%s deliverable %q want %q (chosen=%s)", taskID, mode, got, want, label)
			}
			if got == baseOut && taskID == "ledger-1" {
				t.Fatalf("%s/%s still on base output after compose", taskID, mode)
			}
			harnessScript := filepath.Join(adapter.Spec().RunsRoot, ns, taskID, "harnesses", label, "scripts", "summarize.py")
			script, err := os.ReadFile(harnessScript)
			if err != nil {
				t.Fatal(err)
			}
			if string(script) == demo.BaseScript {
				t.Fatalf("%s/%s chosen harness still equals BaseScript (no task-level evolution)", taskID, mode)
			}
			if strings.Contains(string(script), "amount > 0") {
				t.Fatalf("%s/%s chosen script still filters positives", taskID, mode)
			}
		}
	}

	// 3) Harnessing-level: library stores adopted harness + experience after first task.
	for _, mode := range []string{"agg", "seq"} {
		lib := &library.Library{
			Root:    filepath.Join(libRoot, mode),
			Domain:  "accounting",
			Surface: adapter.Substrate().Surface(),
		}
		ents := lib.Entries()
		if len(ents) == 0 {
			t.Fatalf("%s library empty — no harnessing-level persistence", mode)
		}
	}

	// 4) Reflection artifacts required by the paper's evidence contract.
	reflection := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-seq1", "ledger-1", "reflection", "seq1")
	for _, name := range []string{"adoption.json", "base_harness/scripts/summarize.py", "harness/scripts/summarize.py"} {
		if _, err := os.Stat(filepath.Join(reflection, filepath.FromSlash(name))); err != nil {
			t.Fatalf("missing reflection %s: %v", name, err)
		}
	}
	adoptRaw, _ := os.ReadFile(filepath.Join(reflection, "adoption.json"))
	var adoption map[string]any
	if err := json.Unmarshal(adoptRaw, &adoption); err != nil {
		t.Fatal(err)
	}
	if len(adoption) == 0 {
		t.Fatal("empty adoption.json")
	}
}
