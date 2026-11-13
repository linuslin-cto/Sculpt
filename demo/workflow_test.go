package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/context"
	"sculpt/core/proposer"
	"sculpt/core/schedule"
	"sculpt/core/spec"
	"sculpt/demo"
)

func TestRecursionBudgetAndResume(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])

	_, err = proposer.New(mustChannel(t, "", ""), "", 2)
	if err == nil {
		t.Fatal("empty model should fail Proposer.New")
	}
	_, err = spec.ResolveChannelModel(strPtr("opencode"), strPtr(""), "high", 120, 3600)
	if err == nil {
		t.Fatal("empty opencode model override should fail")
	}

	for _, k := range []int{1, 2} {
		libRoot := filepath.Join(root, "library", itoa(k))
		result := schedule.RunDomain("accounting", adapter.Tasks("all"), k, adapter, prop, adapter.Spec().RunsRoot, libRoot, nil)
		if result["done"] != 2 || result["failed"] != 0 {
			t.Fatalf("k=%d result %#v", k, result)
		}
		for _, taskID := range adapter.Tasks("all") {
			summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg"+itoa(k), taskID, "summary.json")
			b, err := os.ReadFile(summaryPath)
			if err != nil {
				t.Fatal(err)
			}
			var summary map[string]any
			if err := json.Unmarshal(b, &summary); err != nil {
				t.Fatal(err)
			}
			modes := summary["modes"].(map[string]any)
			sels := summary["selections"].(map[string]any)
			for _, mode := range []string{"agg", "seq"} {
				ledger := modes[mode].(map[string]any)["ledger"].(map[string]any)
				if int(ledger["R"].(float64)) != 4*k || int(ledger["P"].(float64)) != 3*k {
					t.Fatalf("k=%d mode=%s ledger R/P = %v/%v", k, mode, ledger["R"], ledger["P"])
				}
				sel := sels[mode].(map[string]any)
				runDir := sel["chosen_run_dir"].(string)
				got := adapter.DeliverableText(runDir)
				want := "Total: 85.15\n"
				if taskID == "ledger-2" {
					want = "Total: 20.25\n"
				}
				if got != want {
					t.Fatalf("task %s mode %s got %q want %q", taskID, mode, got, want)
				}
			}
		}
		executions, calls := adapter.ExecutionCount, len(prop.Calls())
		resumed := schedule.RunDomain("accounting", adapter.Tasks("all"), k, adapter, prop, adapter.Spec().RunsRoot, libRoot, nil)
		if adapter.ExecutionCount != executions || len(prop.Calls()) != calls {
			t.Fatalf("resume should be cache hit")
		}
		tasks := resumed["tasks"].([]map[string]any)
		if tasks[0]["status"] != "skipped" || tasks[1]["status"] != "skipped" {
			t.Fatalf("resume statuses %#v", tasks)
		}
	}

	// Corrupt adopted harness -> force re-run of first task on next stream.
	work := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-seq2", "ledger-1")
	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg2", "ledger-1", "summary.json")
	var summary map[string]any
	b, _ := os.ReadFile(summaryPath)
	_ = json.Unmarshal(b, &summary)
	chosen := summary["selections"].(map[string]any)["seq"].(map[string]any)
	label := chosen["chosen_label"].(string)
	_ = os.WriteFile(filepath.Join(work, "harnesses", label, "scripts", "summarize.py"), []byte("changed after selection\n"), 0o644)
	repaired := schedule.RunDomain("accounting", adapter.Tasks("all"), 2, adapter, prop, adapter.Spec().RunsRoot, filepath.Join(root, "library", "2"), nil)
	tasks := repaired["tasks"].([]map[string]any)
	if tasks[0]["status"] != "ok" {
		t.Fatalf("expected re-run ok, got %#v", tasks)
	}
}

func TestEvidenceAndLibraryPrefix(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(adapter.TaskFiles("ledger-1"), "reference_answer.txt"), []byte(`"pass_rate": 1`), 0o644)
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	_ = schedule.RunDomain("accounting", adapter.Tasks("all"), 2, adapter, prop, adapter.Spec().RunsRoot, filepath.Join(root, "library", "2"), nil)

	work := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-seq2", "ledger-1")
	reflection := filepath.Join(work, "reflection", "seq2")
	required := []string{
		"source/ledger.csv",
		"rollouts/base1.md",
		"rollouts/c2.md",
		"pool/inv1/harness.diff",
		"decisions/c1.md",
		"adoption.json",
		"base_harness/scripts/summarize.py",
		"harness/scripts/summarize.py",
	}
	var missing []string
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(reflection, filepath.FromSlash(name))); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing reflection files: %v", missing)
	}
	if _, err := os.Stat(filepath.Join(work, "context", "inv2", "pool", "c1", "rollout.md")); err != nil {
		t.Fatal("expected sequential pool evidence", err)
	}
	var leaks []string
	_ = filepath.Walk(work, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Name() == "reference_answer.txt" {
			leaks = append(leaks, path)
		}
		return nil
	})
	if len(leaks) > 0 {
		t.Fatalf("leak globs failed: %v", leaks)
	}
	next := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-seq2", "ledger-2", "context", "inv1", "library", "harnesses")
	ents, err := os.ReadDir(next)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "ledger-1" {
		t.Fatalf("library prefix %#v", ents)
	}
	if err := context.AssertTreeHasNoGraderVerdicts(map[string]string{"EXPERIENCE.md": `"pass_rate": 1`}); err == nil {
		t.Fatal("grader verdict should be rejected")
	}
}

func TestChannelResolve(t *testing.T) {
	unresolved, err := spec.ResolveChannelModel(nil, nil, "high", 120, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if unresolved.Model != "" && os.Getenv("HARNESS2_IMPROVER_MODEL") == "" {
		// may be empty by default
	}
	_, err = spec.ResolveChannelModel(strPtr("opencode"), nil, "high", 120, 3600)
	if err == nil {
		t.Fatal("lone override should fail")
	}
}

func mustChannel(t *testing.T, kind, model string) spec.ChannelSpec {
	t.Helper()
	ch, err := spec.NewChannelSpec("opencode", "local/x", "high", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	ch.Model = model
	if kind != "" {
		ch.Kind = kind
	}
	return ch
}

func strPtr(s string) *string { return &s }

func itoa(n int) string {
	return string(rune('0' + n))
}
