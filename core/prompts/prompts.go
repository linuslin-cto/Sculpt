// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package prompts builds Propose–Probe–Compose mission text.
package prompts

import (
	"fmt"
	"strings"

	"sculpt/core/spec"
	"sculpt/core/tree"
)

// Propose builds an INV/DOM proposal mission.
func Propose(p spec.BenchPrompts, dim string, surface tree.SurfaceSpec, k, index int, roundT *int) string {
	levers := p.InvLevers
	if dim == "DOM" {
		levers = p.DomLevers
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Propose (%s) edit #%d of %d\n\n", dim, index, k))
	b.WriteString("Work: ")
	b.WriteString(p.Work)
	b.WriteString("\n\n")
	if levers != "" {
		b.WriteString("Levers:\n")
		b.WriteString(levers)
		b.WriteString("\n\n")
	}
	if roundT != nil {
		b.WriteString(fmt.Sprintf("Sequential round t=%d of k=%d.\n\n", *roundT, k))
	}
	b.WriteString("Edit files under `context/harness/` (omitted paths are inherited). ")
	b.WriteString(fmt.Sprintf("Editable surface: %s. Always-on: %s.\n", surface.Name, strings.Join(surface.AlwaysOn, ", ")))
	b.WriteString("Reply with <notes>...</notes> describing the change.\n")
	return b.String()
}

// Aggregate builds a parallel composition mission for agg{index}.
func Aggregate(p spec.BenchPrompts, surface tree.SurfaceSpec, k, index int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Compose aggregate harness #%d of %d\n\n", index, k))
	b.WriteString("Work: ")
	b.WriteString(p.Work)
	b.WriteString("\n\nCompose a refined harness from candidate evidence in `context/pool/`. ")
	b.WriteString("Edit `context/harness/` in place. Reply with <notes>...</notes>.\n")
	_ = surface
	return b.String()
}

// ComposeRound builds a sequential compose mission for round t.
func ComposeRound(p spec.BenchPrompts, surface tree.SurfaceSpec, t, k int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Compose sequential round t=%d of k=%d\n\n", t, k))
	b.WriteString("Work: ")
	b.WriteString(p.Work)
	b.WriteString("\n\nCarry forward useful edits; edit `context/harness/`. Reply with <notes>...</notes>.\n")
	_ = surface
	return b.String()
}

// Select builds a blind selection mission.
func Select(p spec.BenchPrompts) string {
	var b strings.Builder
	b.WriteString("# Select the best anonymized candidate\n\n")
	b.WriteString("Work: ")
	b.WriteString(p.Work)
	b.WriteString("\n\nChoose one candidate id. Reply with <why>...</why> and ```choice\\nID\\n```.\n")
	if p.SelectionChecks != "" {
		b.WriteString("\nChecks:\n")
		b.WriteString(p.SelectionChecks)
	}
	return b.String()
}

// Reflect builds an experience-reflection mission.
func Reflect(p spec.BenchPrompts) string {
	var b strings.Builder
	b.WriteString("# Reflect on adopted harness edits\n\n")
	b.WriteString("Work: ")
	b.WriteString(p.Work)
	b.WriteString("\n\nEmit <edits>...</edits> and <experience> with cue/lesson/evidence/confidence, or NO_ENTRY.\n")
	return b.String()
}
