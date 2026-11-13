package demo_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/schedule"
	"sculpt/demo"
)

type flakyScoreAdapter struct {
	*demo.Adapter
	fail bool
	calls int
}

func (a *flakyScoreAdapter) Score(runDir, taskID string) (map[string]any, error) {
	a.calls++
	if a.fail {
		return nil, errJudgeDown
	}
	return map[string]any{"n_passed": 1, "n_criteria": 1}, nil
}

var errJudgeDown = errString("judge endpoint unavailable")

type errString string

func (e errString) Error() string { return string(e) }

func TestFailedRejudgeKeepsGradeOnlyForSameSelection(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	libRoot := filepath.Join(root, "library", "1")
	_ = schedule.RunDomain("accounting", adapter.Tasks("all"), 1, adapter, prop, adapter.Spec().RunsRoot, libRoot, nil)

	nsDir := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg1")
	first, err := judge.CollectSelections(nsDir, []string{"agg"})
	if err != nil {
		t.Fatal(err)
	}
	byTask := map[string]map[string]any{}
	for _, sel := range first {
		tid, _ := sel["task_id"].(string)
		byTask[tid] = sel
		if _, ok := sel["selected_at"].(string); !ok || sel["selected_at"] == "" {
			t.Fatalf("missing selected_at: %#v", sel)
		}
	}
	scoring := &flakyScoreAdapter{Adapter: adapter}
	graded := judge.Selections(scoring, first)
	if len(graded) != 2 || !graded[0].Graded() || !graded[1].Graded() {
		t.Fatalf("%#v", graded)
	}
	scoring.fail = true
	failed := judge.Selections(scoring, first)
	merged := judge.Merge(graded, failed)
	for _, r := range merged {
		if !r.Graded() {
			t.Fatalf("failed rejudge erased grade: %#v", r)
		}
	}

	_ = os.Remove(filepath.Join(byTask["ledger-1"]["chosen_run_dir"].(string), "answer.txt"))
	_ = schedule.RunDomain("accounting", adapter.Tasks("all"), 1, adapter, prop, adapter.Spec().RunsRoot, libRoot, nil)
	again, err := judge.CollectSelections(nsDir, []string{"agg"})
	if err != nil {
		t.Fatal(err)
	}
	againBy := map[string]map[string]any{}
	for _, sel := range again {
		againBy[sel["task_id"].(string)] = sel
	}
	if againBy["ledger-1"]["chosen_run_id"] != byTask["ledger-1"]["chosen_run_id"] {
		t.Fatalf("run id changed unexpectedly")
	}
	if againBy["ledger-1"]["selected_at"] == byTask["ledger-1"]["selected_at"] {
		t.Fatal("ledger-1 should get a new selected_at after reselect")
	}
	if againBy["ledger-2"]["selected_at"] != byTask["ledger-2"]["selected_at"] {
		t.Fatal("ledger-2 should keep selected_at on resume skip")
	}

	scoring.fail = true
	failedAgain := judge.Selections(scoring, again)
	merged2 := judge.Merge(graded, failedAgain)
	got := map[string]bool{}
	for _, r := range merged2 {
		got[r.TaskID] = r.Graded()
	}
	if got["ledger-1"] != false || got["ledger-2"] != true {
		t.Fatalf("%v", got)
	}
}
