// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package ledger tracks R=4k rollouts and P=3k improvement stages.
package ledger

import (
	"fmt"
	"math"
	"strings"
)

// ExpectedRollouts is R = 4k: k shared base + 2k candidates + k finals.
func ExpectedRollouts(k int) int { return 4 * k }

// ExpectedSessions is P = 3k: k INV + k DOM + k compositions.
func ExpectedSessions(k int) int { return 3 * k }

// Entry is one recorded rollout or session.
type Entry struct {
	Kind     string  `json:"kind"`
	Label    string  `json:"label"`
	USD      float64 `json:"usd"`
	Cached   bool    `json:"cached"`
	Attempts int     `json:"attempts"`
}

// Ledger records everything one mode spent on one task.
type Ledger struct {
	Mode   string  `json:"mode"`
	TaskID string  `json:"task_id"`
	K      int     `json:"k"`
	Entries []Entry `json:"entries"`
}

// Rollout records a rollout label.
func (l *Ledger) Rollout(label string, usd float64, cached bool) {
	l.Entries = append(l.Entries, Entry{Kind: "rollout", Label: label, USD: usd, Cached: cached, Attempts: 1})
}

// Session records an improvement stage.
func (l *Ledger) Session(label string, usd float64, cached bool, attempts int) {
	if attempts < 1 {
		attempts = 1
	}
	l.Entries = append(l.Entries, Entry{Kind: "session", Label: label, USD: usd, Cached: cached, Attempts: attempts})
}

// R is the rollout count.
func (l *Ledger) R() int {
	n := 0
	for _, e := range l.Entries {
		if e.Kind == "rollout" {
			n++
		}
	}
	return n
}

// P is the session count.
func (l *Ledger) P() int {
	n := 0
	for _, e := range l.Entries {
		if e.Kind == "session" {
			n++
		}
	}
	return n
}

// Labels returns labels of the given kind in order.
func (l *Ledger) Labels(kind string) []string {
	var out []string
	for _, e := range l.Entries {
		if e.Kind == kind {
			out = append(out, e.Label)
		}
	}
	return out
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

// USD totals cost.
func (l *Ledger) USD() float64 {
	s := 0.0
	for _, e := range l.Entries {
		s += e.USD
	}
	return round6(s)
}

// AssertIdentity raises unless R=4k and P=3k with no duplicate labels.
func (l *Ledger) AssertIdentity() error {
	wantR, wantP := ExpectedRollouts(l.K), ExpectedSessions(l.K)
	rl, sl := l.Labels("rollout"), l.Labels("session")
	dupes := map[string]struct{}{}
	count := func(labels []string) {
		seen := map[string]int{}
		for _, lb := range labels {
			seen[lb]++
			if seen[lb] > 1 {
				dupes[lb] = struct{}{}
			}
		}
	}
	count(rl)
	count(sl)
	if len(dupes) == 0 && l.R() == wantR && l.P() == wantP {
		return nil
	}
	var detail []string
	detail = append(detail, fmt.Sprintf("%s-%d %s: R=%d (want %d), P=%d (want %d)",
		l.Mode, l.K, l.TaskID, l.R(), wantR, l.P(), wantP))
	if len(rl) == 0 {
		detail = append(detail, "  rollouts: (none)")
	} else {
		detail = append(detail, "  rollouts: "+strings.Join(rl, ", "))
	}
	if len(sl) == 0 {
		detail = append(detail, "  sessions: (none)")
	} else {
		detail = append(detail, "  sessions: "+strings.Join(sl, ", "))
	}
	if len(dupes) > 0 {
		keys := make([]string, 0, len(dupes))
		for k := range dupes {
			keys = append(keys, k)
		}
		detail = append(detail, "  DUPLICATE labels: "+strings.Join(keys, ", "))
	}
	return fmt.Errorf("%s", strings.Join(detail, "\n"))
}

// ToDict serialises the ledger for summaries.
func (l *Ledger) ToDict() map[string]any {
	sessAttempts := 0
	for _, e := range l.Entries {
		if e.Kind == "session" {
			sessAttempts += e.Attempts
		}
	}
	rollUSD, sessUSD := 0.0, 0.0
	for _, e := range l.Entries {
		if e.Kind == "rollout" {
			rollUSD += e.USD
		} else if e.Kind == "session" {
			sessUSD += e.USD
		}
	}
	return map[string]any{
		"mode":            l.Mode,
		"task_id":         l.TaskID,
		"k":               l.K,
		"R":               l.R(),
		"expected_R":      ExpectedRollouts(l.K),
		"P":               l.P(),
		"session_attempts": sessAttempts,
		"expected_P":      ExpectedSessions(l.K),
		"usd":             l.USD(),
		"rollout_usd":     round6(rollUSD),
		"session_usd":     round6(sessUSD),
		"rollouts":        l.Labels("rollout"),
		"sessions":        l.Labels("session"),
	}
}
