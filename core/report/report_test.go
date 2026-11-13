package report_test

import (
	"strings"
	"testing"

	"sculpt/core/judge"
	"sculpt/core/report"
)

func TestSummariseExcludesUngraded(t *testing.T) {
	np, nc := 1, 1
	rows := []judge.Scored{
		{TaskID: "a", Mode: "agg", K: 1, Label: "agg1", NPassed: &np, NCriteria: &nc, AllPass: true},
		{TaskID: "b", Mode: "agg", K: 1, Label: "agg1", Error: "judge failed"},
		{TaskID: "c", Mode: "agg", K: 1, Label: "base1", NPassed: &np, NCriteria: &nc},
	}
	s := report.Summarise("agg", 1, rows)
	if s.N != 2 || s.NUngraded != 1 || s.ChoseFinal != 2 || s.KeptBase != 1 {
		t.Fatalf("%#v", s)
	}
	line := s.Line("demo")
	if !strings.Contains(line, "score") || !strings.Contains(line, "ungraded 1") {
		t.Fatal(line)
	}
}
