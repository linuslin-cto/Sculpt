// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package report turns judged selections into the figures each benchmark reports.
// Ungraded rows are excluded from the denominator rather than counted as failures.
package report

import (
	"fmt"
	"sort"
	"strings"

	"sculpt/core/judge"
)

// ModeNames maps on-disk tokens to display names.
var ModeNames = map[string]string{"agg": "parallel", "seq": "sequential"}

// Summary is one (mode, k) figure line.
type Summary struct {
	Mode           string
	K              int
	N              int
	NUngraded      int
	Score          *float64
	AllPass        *float64
	KeptBase       int
	ChoseFinal     int
	ChoseCandidate int
	FellBack       int
	USD            float64
}

// Name is the display label.
func (s Summary) Name() string {
	return fmt.Sprintf("%s k=%d", ModeNames[s.Mode], s.K)
}

// Line formats one summary row.
func (s Summary) Line(bench string) string {
	if s.N == 0 {
		return fmt.Sprintf("  %s: no graded task", s.Name())
	}
	var figures string
	if bench == "lab" && s.Score != nil && s.AllPass != nil {
		figures = fmt.Sprintf("partial %6.2f%% | pass %5.1f%%", *s.Score, *s.AllPass)
	} else if s.Score != nil {
		figures = fmt.Sprintf("score %6.2f%%", *s.Score)
	} else {
		figures = "score    n/a"
	}
	pick := fmt.Sprintf(" | picked output: final %d / candidate %d / base %d",
		s.ChoseFinal, s.ChoseCandidate, s.KeptBase)
	fb, ug := "", ""
	if s.FellBack > 0 {
		fb = fmt.Sprintf(" | fell back %d", s.FellBack)
	}
	if s.NUngraded > 0 {
		ug = fmt.Sprintf(" | ungraded %d", s.NUngraded)
	}
	return fmt.Sprintf("  %s: n=%3d | %s%s%s%s", s.Name(), s.N, figures, pick, fb, ug)
}

func kind(r judge.Scored) string {
	lbl := r.Label
	if strings.HasPrefix(lbl, "base") {
		return "base"
	}
	if r.Mode == "agg" {
		if strings.HasPrefix(lbl, "agg") {
			return "final"
		}
		return "candidate"
	}
	if r.Mode == "seq" {
		for i := 1; i <= r.K; i++ {
			if lbl == fmt.Sprintf("c%d", i) {
				return "final"
			}
		}
		return "candidate"
	}
	return "candidate"
}

// Summarise aggregates graded rows for one mode.
func Summarise(mode string, k int, rows []judge.Scored) Summary {
	s := Summary{Mode: mode, K: k}
	var fracs []float64
	var passes int
	for _, r := range rows {
		if r.Mode != mode || r.K != k {
			continue
		}
		switch kind(r) {
		case "base":
			s.KeptBase++
		case "final":
			s.ChoseFinal++
		default:
			s.ChoseCandidate++
		}
		if !r.Graded() {
			s.NUngraded++
			continue
		}
		s.N++
		fracs = append(fracs, *r.Frac())
		if r.AllPass {
			passes++
		}
	}
	if len(fracs) > 0 {
		sum := 0.0
		for _, f := range fracs {
			sum += f
		}
		mean := 100 * sum / float64(len(fracs))
		s.Score = &mean
		ap := 100 * float64(passes) / float64(len(fracs))
		s.AllPass = &ap
	}
	return s
}

// ModeK is a (mode, k) report key.
type ModeK struct {
	Mode string
	K    int
}

// SummariseAll builds one Summary per (mode, k) present in rows.
// Optional fellBack/usd maps overlay spend and fallback counts from task summaries.
func SummariseAll(rows []judge.Scored, fellBack map[ModeK]int, usd map[ModeK]float64) []Summary {
	seen := map[ModeK]struct{}{}
	var keys []ModeK
	for _, r := range rows {
		k := ModeK{r.Mode, r.K}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].K != keys[j].K {
			return keys[i].K < keys[j].K
		}
		return keys[i].Mode < keys[j].Mode
	})
	out := make([]Summary, 0, len(keys))
	for _, k := range keys {
		s := Summarise(k.Mode, k.K, rows)
		if fellBack != nil {
			s.FellBack = fellBack[k]
		}
		if usd != nil {
			s.USD = usd[k]
		}
		out = append(out, s)
	}
	return out
}

// Render prints the summary block (Python report.render).
func Render(summaries []Summary, title, bench string) string {
	var lines []string
	if title != "" {
		lines = append(lines, title, strings.Repeat("=", max(40, len(title))))
	}
	sorted := append([]Summary(nil), summaries...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].K != sorted[j].K {
			return sorted[i].K < sorted[j].K
		}
		return sorted[i].Mode < sorted[j].Mode
	})
	for _, s := range sorted {
		lines = append(lines, s.Line(bench))
	}
	at1 := map[string]Summary{}
	for _, s := range summaries {
		if s.K == 1 && s.Score != nil {
			at1[s.Mode] = s
		}
	}
	if len(at1) == 2 && at1["agg"].Score != nil && at1["seq"].Score != nil {
		gap := *at1["agg"].Score - *at1["seq"].Score
		if gap < 0 {
			gap = -gap
		}
		lines = append(lines, "",
			"  NOTE  at k=1 the parallel and sequential modes are the same procedure "+
				"(one INVARIANT draw, one DOMAIN draw, one composition).",
			fmt.Sprintf("        The %.2f pt between them is therefore a NOISE FLOOR, not a "+
				"result -- an effect at k>=2 has to clear it.", gap),
		)
	}
	for _, s := range summaries {
		if s.N > 0 && float64(s.ChoseFinal) <= 0.25*float64(s.N) {
			lines = append(lines, "",
				fmt.Sprintf("  WARNING  %s: the mode's own final was chosen on only %d/%d tasks. "+
					"These records include outputs outside the published pool of %d composed finals.",
					s.Name(), s.ChoseFinal, s.N, s.K),
			)
		}
	}
	for _, s := range summaries {
		if s.N > 0 && float64(s.KeptBase) >= 0.8*float64(s.N) {
			lines = append(lines, "",
				fmt.Sprintf("  WARNING  %s: selection chose a reference output on %d/%d tasks, "+
					"outside the published pool of composed outputs.", s.Name(), s.KeptBase, s.N),
			)
		}
	}
	for _, s := range summaries {
		if s.FellBack > 0 {
			lines = append(lines, "",
				fmt.Sprintf("  WARNING  %s: %d stage(s) fell back to the inherited harness. "+
					"Inspect the stage errors when interpreting these results.", s.Name(), s.FellBack),
			)
		}
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
