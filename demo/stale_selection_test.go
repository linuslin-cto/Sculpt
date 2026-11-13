package demo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/proposer"
	"sculpt/core/recursion"
	"sculpt/core/schedule"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/demo"
)

func TestFailedRerunClearsStaleSelectionsInSummary(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	libRoot := filepath.Join(root, "library", "1")
	run := func(p proposer.Caller) map[string]any {
		return schedule.RunDomain("accounting", adapter.Tasks("all"), 1, adapter, p, adapter.Spec().RunsRoot, libRoot, nil)
	}

	first := run(prop)
	if first["done"] != 2 || first["failed"] != 0 {
		t.Fatalf("%#v", first)
	}
	summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1", "ledger-1", "summary.json")
	initial := readSummary(t, summaryPath)
	agg := initial["selections"].(map[string]any)["agg"].(map[string]any)
	if agg["ok"] != true {
		t.Fatalf("%#v", agg)
	}
	chosen := agg["chosen_run_dir"].(string)
	if err := os.Remove(filepath.Join(chosen, "answer.txt")); err != nil {
		t.Fatal(err)
	}

	schedule.SetRunModeForTest(func(mode, taskID string, k int, base []spec.RunView,
		adapter spec.BenchmarkAdapter, prop proposer.Caller,
		workDir string, runIDOf func(string) string, baseTree tree.Tree, libraryDir string,
	) (*recursion.ModeResult, error) {
		return nil, fmt.Errorf("recursion failed on rerun")
	})
	defer schedule.SetRunModeForTest(nil)

	failedMode := run(prop)
	rows := failedMode["tasks"].([]map[string]any)
	if rows[0]["status"] != "partial" || rows[1]["status"] != "skipped" {
		t.Fatalf("%#v", failedMode)
	}
	afterMode := readSummary(t, summaryPath)
	for _, mode := range []string{"agg", "seq"} {
		m := afterMode["modes"].(map[string]any)[mode].(map[string]any)
		s := afterMode["selections"].(map[string]any)[mode].(map[string]any)
		if m["ok"] != false || s["ok"] != false {
			t.Fatalf("%s modes=%#v sels=%#v", mode, m, s)
		}
		if label, _ := s["chosen_label"].(string); label != "" {
			t.Fatalf("%s chosen_label=%q", mode, label)
		}
	}
	var selList []map[string]any
	for _, mode := range []string{"agg", "seq"} {
		selList = append(selList, afterMode["selections"].(map[string]any)[mode].(map[string]any))
	}
	scored := judge.Selections(adapter, selList)
	for _, r := range scored {
		if r.Graded() {
			t.Fatalf("failed selection graded: %#v", r)
		}
	}

	// Repair, then exercise base-failed clearing.
	schedule.SetRunModeForTest(nil)
	if got := run(prop); got["tasks"].([]map[string]any)[0]["status"] != "ok" {
		t.Fatalf("repair %#v", got)
	}
	restored := readSummary(t, summaryPath)
	runDir, _ := restored["selections"].(map[string]any)["agg"].(map[string]any)["chosen_run_dir"].(string)
	_ = os.Remove(filepath.Join(runDir, "answer.txt"))
	failExec := &failingExecuteAdapter{Adapter: adapter}
	failedBase := schedule.RunDomain("accounting", adapter.Tasks("all"), 1, failExec, prop, adapter.Spec().RunsRoot, libRoot, nil)
	statuses := []string{
		failedBase["tasks"].([]map[string]any)[0]["status"].(string),
		failedBase["tasks"].([]map[string]any)[1]["status"].(string),
	}
	if statuses[0] != "base-failed" || statuses[1] != "skipped" {
		t.Fatalf("%v %#v", statuses, failedBase)
	}
	afterBase := readSummary(t, summaryPath)
	if afterBase["status"] != "base-failed" {
		t.Fatalf("%#v", afterBase)
	}
	for _, mode := range []string{"agg", "seq"} {
		if afterBase["modes"].(map[string]any)[mode].(map[string]any)["ok"] != false {
			t.Fatalf("%s mode still ok", mode)
		}
		if afterBase["selections"].(map[string]any)[mode].(map[string]any)["ok"] != false {
			t.Fatalf("%s selection still ok", mode)
		}
	}
}

func readSummary(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type failingExecuteAdapter struct {
	*demo.Adapter
}

func (a *failingExecuteAdapter) Execute(taskID, harnessDir, runID string, timeout int) (string, error) {
	a.ExecutionCount++
	return "", fmt.Errorf("executor down")
}
