// Copyright 2026 The Sculpt Authors.
// Licensed under the Apache License, Version 2.0

// Package eval compares base-harness and refined-harness grades the way the
// upstream experiment guide describes: judge selections into judged.json,
// judge reference executions into base_judged.json, then report the delta.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"sculpt/core/judge"
	"sculpt/core/report"
)

// PaperGains are published peak gains (percentage points over the base
// harness) from the method paper's reported configurations. Live Sculpt runs
// use these as qualitative targets, not hard CI gates.
var PaperGains = map[string]float64{
	"wb":  11.5, // WorkBuddy-Bench score points
	"jb":  4.5,  // JobBench score points
	"lab": 11.8, // LAB partial pass-rate percentage points
}

// BaseRow is one reference-execution grade from base_judged.json.
type BaseRow struct {
	Task   string         `json:"task"`
	Label  string         `json:"label"`
	RunID  string         `json:"run_id"`
	Score  map[string]any `json:"score"`
}

// LoadBaseRows reads base_judged.json.
func LoadBaseRows(path string) ([]BaseRow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []BaseRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// MeanFrac returns mean pass fraction * 100 for graded base rows (nil if none).
func MeanFrac(rows []BaseRow) *float64 {
	var fracs []float64
	for _, r := range rows {
		if r.Score == nil {
			continue
		}
		np, okP := asInt(r.Score["n_passed"])
		nc, okC := asInt(r.Score["n_criteria"])
		if !okP || !okC || !judge.HasGrade(&np, &nc) {
			continue
		}
		fracs = append(fracs, 100*float64(np)/float64(nc))
	}
	if len(fracs) == 0 {
		return nil
	}
	sum := 0.0
	for _, f := range fracs {
		sum += f
	}
	mean := sum / float64(len(fracs))
	return &mean
}

// Comparison is base vs refined for one (mode, k).
type Comparison struct {
	Mode          string
	K             int
	BaseScore     *float64 // percent
	RefinedScore  *float64 // percent
	DeltaPts      *float64 // refined - base, percentage points
	NBase         int
	NRefined      int
	ClearsNoise   bool // when both modes at k=1 exist, delta vs parallel/seq gap is N/A here
	MeetsPaperMin bool // true when DeltaPts >= PaperGains[bench] (if known)
}

// Line formats one comparison row.
func (c Comparison) Line(bench string) string {
	bs, rs, ds := "n/a", "n/a", "n/a"
	if c.BaseScore != nil {
		bs = fmt.Sprintf("%6.2f%%", *c.BaseScore)
	}
	if c.RefinedScore != nil {
		rs = fmt.Sprintf("%6.2f%%", *c.RefinedScore)
	}
	if c.DeltaPts != nil {
		sign := "+"
		if *c.DeltaPts < 0 {
			sign = ""
		}
		ds = fmt.Sprintf("%s%.2f pt", sign, *c.DeltaPts)
	}
	name := fmt.Sprintf("%s k=%d", report.ModeNames[c.Mode], c.K)
	extra := ""
	if target, ok := PaperGains[bench]; ok && c.DeltaPts != nil {
		if *c.DeltaPts+1e-9 >= target {
			extra = fmt.Sprintf(" | clears paper peak (+%.1f)", target)
		} else {
			extra = fmt.Sprintf(" | paper peak +%.1f (target)", target)
		}
	}
	return fmt.Sprintf("  %s: base %s → refined %s | Δ %s (n_base=%d n_ref=%d)%s",
		name, bs, rs, ds, c.NBase, c.NRefined, extra)
}

// Compare pairs base_judged grades with refined Summarise figures.
func Compare(base []BaseRow, refined []judge.Scored, fellBack map[report.ModeK]int, usd map[report.ModeK]float64, bench string) []Comparison {
	baseMean := MeanFrac(base)
	nBase := 0
	for _, r := range base {
		if r.Score != nil {
			nBase++
		}
	}
	sums := report.SummariseAll(refined, fellBack, usd)
	out := make([]Comparison, 0, len(sums))
	for _, s := range sums {
		c := Comparison{
			Mode: s.Mode, K: s.K,
			BaseScore: baseMean, RefinedScore: s.Score,
			NBase: nBase, NRefined: s.N,
		}
		if baseMean != nil && s.Score != nil {
			d := *s.Score - *baseMean
			c.DeltaPts = &d
			if target, ok := PaperGains[bench]; ok {
				c.MeetsPaperMin = d+1e-9 >= target
			}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].K != out[j].K {
			return out[i].K < out[j].K
		}
		return out[i].Mode < out[j].Mode
	})
	return out
}

// RenderBaseDelta prints the base→refined block.
func RenderBaseDelta(comps []Comparison, title, bench string) string {
	if len(comps) == 0 {
		return ""
	}
	lines := []string{title, "------------------------------------------------------------"}
	any := false
	for _, c := range comps {
		if c.BaseScore != nil || c.RefinedScore != nil {
			any = true
		}
		lines = append(lines, c.Line(bench))
	}
	if !any {
		return ""
	}
	if target, ok := PaperGains[bench]; ok {
		lines = append(lines, "",
			fmt.Sprintf("  Paper reference peak gain for %s: +%.1f pt over base harness.", bench, target),
			"  Offline demo is exact-match (0%→100%); live benches use their own metrics.",
		)
	}
	return strings.Join(lines, "\n")
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		return int(n), err == nil
	default:
		return 0, false
	}
}
