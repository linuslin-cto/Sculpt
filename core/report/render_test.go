package report_test

import (
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/report"
)

func TestRenderNoiseFloorNote(t *testing.T) {
	np, nc := 1, 2
	rows := []judge.Scored{
		{TaskID: "a", Mode: "agg", K: 1, Label: "agg1", NPassed: &np, NCriteria: &nc},
		{TaskID: "a", Mode: "seq", K: 1, Label: "c1", NPassed: &np, NCriteria: &nc},
	}
	summaries := report.SummariseAll(rows, nil, nil)
	text := report.Render(summaries, "demo-oc modes=parallel,sequential", "demo")
	if !strings.Contains(text, "NOISE FLOOR") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "parallel k=1") || !strings.Contains(text, "sequential k=1") {
		t.Fatal(text)
	}
}

func TestLoadAndWriteJudgedRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "judged.json")
	np, nc := 2, 2
	rows := []judge.Scored{
		{TaskID: "t1", Mode: "agg", K: 1, Label: "agg1", NPassed: &np, NCriteria: &nc, AllPass: true},
	}
	if err := judge.Write(path, rows); err != nil {
		t.Fatal(err)
	}
	got, err := judge.LoadRows(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Graded() || *got[0].NPassed != 2 {
		t.Fatalf("%#v", got)
	}
}
