package eval_test

import (
	"testing"

	"sculpt/core/eval"
	"sculpt/core/judge"
)

func TestCompareBaseToRefinedDelta(t *testing.T) {
	np0, nc := 0, 1
	np1 := 1
	base := []eval.BaseRow{
		{Task: "ledger-1", Label: "base1", Score: map[string]any{"n_passed": 0, "n_criteria": 1}},
		{Task: "ledger-2", Label: "base1", Score: map[string]any{"n_passed": 0, "n_criteria": 1}},
	}
	refined := []judge.Scored{
		{TaskID: "ledger-1", Mode: "agg", K: 1, Label: "c1", NPassed: &np1, NCriteria: &nc, AllPass: true},
		{TaskID: "ledger-2", Mode: "agg", K: 1, Label: "c1", NPassed: &np1, NCriteria: &nc, AllPass: true},
		{TaskID: "ledger-1", Mode: "seq", K: 1, Label: "c1", NPassed: &np1, NCriteria: &nc, AllPass: true},
		{TaskID: "ledger-2", Mode: "seq", K: 1, Label: "c1", NPassed: &np0, NCriteria: &nc},
	}
	comps := eval.Compare(base, refined, nil, nil, "demo")
	if len(comps) != 2 {
		t.Fatalf("%#v", comps)
	}
	var agg *eval.Comparison
	for i := range comps {
		if comps[i].Mode == "agg" {
			agg = &comps[i]
		}
	}
	if agg == nil || agg.BaseScore == nil || agg.RefinedScore == nil || agg.DeltaPts == nil {
		t.Fatalf("%#v", comps)
	}
	if *agg.BaseScore != 0 || *agg.RefinedScore != 100 || *agg.DeltaPts != 100 {
		t.Fatalf("agg %#v", *agg)
	}
}

func TestPaperGainsTargets(t *testing.T) {
	for _, bench := range []string{"jb", "lab", "wb"} {
		if eval.PaperGains[bench] <= 0 {
			t.Fatalf("missing paper gain for %s", bench)
		}
	}
}
