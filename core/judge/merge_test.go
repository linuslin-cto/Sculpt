package judge_test

import (
	"testing"

	"sculpt/core/judge"
)

func TestMergeKeepsGradeOnFailedRejudge(t *testing.T) {
	one, two := 1, 1
	graded := []judge.Scored{{
		TaskID: "ledger-1", Mode: "agg", K: 1, RunID: "r1", SelectedAt: "t1",
		NPassed: &one, NCriteria: &two,
	}}
	failed := []judge.Scored{{
		TaskID: "ledger-1", Mode: "agg", K: 1, RunID: "r1", SelectedAt: "t1",
		Error: "judge endpoint unavailable",
	}}
	merged := judge.Merge(graded, failed)
	if len(merged) != 1 || !merged[0].Graded() {
		t.Fatalf("%#v", merged)
	}
}

func TestMergeReplacesOnReselect(t *testing.T) {
	one, two := 1, 1
	graded := []judge.Scored{{
		TaskID: "ledger-1", Mode: "agg", K: 1, RunID: "r1", SelectedAt: "t1",
		NPassed: &one, NCriteria: &two,
	}}
	reselected := []judge.Scored{{
		TaskID: "ledger-1", Mode: "agg", K: 1, RunID: "r1", SelectedAt: "t2",
		Error: "adopted run directory is missing",
	}}
	merged := judge.Merge(graded, reselected)
	if len(merged) != 1 || merged[0].Graded() || merged[0].SelectedAt != "t2" {
		t.Fatalf("%#v", merged)
	}
}
