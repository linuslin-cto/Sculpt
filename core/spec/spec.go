// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package spec is the benchmark seam: BenchSpec, channels, and adapter contracts.
package spec

import (
	"fmt"
	"path/filepath"
	"strings"

	"sculpt/core/tree"
	"sculpt/internal/config"
)

// Channels lists supported proposer/selector channel kinds.
var Channels = []string{"claude_cli", "opencode"}

// InstallModes lists supported harness install modes.
var InstallModes = []string{"workspace_agents_md", "home_dir_jinja", "workspace_config", "codex_root"}

// ChannelSpec describes how to reach one model on one channel.
type ChannelSpec struct {
	Kind    string `json:"kind"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
	Steps   int    `json:"steps"`
	Timeout int    `json:"timeout"`
}

// NewChannelSpec validates and constructs a ChannelSpec.
func NewChannelSpec(kind, model, effort string, steps, timeout int) (ChannelSpec, error) {
	ok := false
	for _, c := range Channels {
		if c == kind {
			ok = true
			break
		}
	}
	if !ok {
		return ChannelSpec{}, fmt.Errorf("unknown channel %q; expected one of %v", kind, Channels)
	}
	if effort == "" {
		effort = "high"
	}
	if steps == 0 {
		steps = 120
	}
	if timeout == 0 {
		timeout = 3600
	}
	return ChannelSpec{Kind: kind, Model: model, Effort: effort, Steps: steps, Timeout: timeout}, nil
}

// ResolveChannelModel is the one channel->model mapping; a lone override of either raises.
func ResolveChannelModel(kind, model *string, effort string, steps, timeout int) (ChannelSpec, error) {
	useDefault := kind == nil && model == nil
	if (kind == nil) != (model == nil) {
		k, m := "<nil>", "<nil>"
		if kind != nil {
			k = *kind
		}
		if model != nil {
			m = *model
		}
		return ChannelSpec{}, fmt.Errorf("channel and model must be overridden TOGETHER; got kind=%s, model=%s", k, m)
	}
	k := config.DefaultProposerChannel
	if kind != nil {
		k = *kind
	}
	ok := false
	for _, c := range Channels {
		if c == k {
			ok = true
			break
		}
	}
	if !ok {
		return ChannelSpec{}, fmt.Errorf("unknown channel %q; expected one of %v", k, Channels)
	}
	m := config.ProposerModelByChannel[k]
	if model != nil {
		m = *model
	}
	if strings.TrimSpace(m) == "" {
		if useDefault {
			return NewChannelSpec(k, "", effort, steps, timeout)
		}
		return ChannelSpec{}, fmt.Errorf("model override must not be empty")
	}
	if k == "opencode" && !strings.Contains(m, "/") {
		return ChannelSpec{}, fmt.Errorf("opencode channel needs a provider-prefixed model id, got %q", m)
	}
	if k == "claude_cli" && strings.Contains(m, "/") {
		return ChannelSpec{}, fmt.Errorf("claude_cli channel takes a bare Anthropic id, got %q", m)
	}
	return NewChannelSpec(k, m, effort, steps, timeout)
}

// BenchPrompts is the benchmark-shaped text shared prompts interpolate.
type BenchPrompts struct {
	Work                 string
	DomainWord           string
	InvLevers            string
	DomLevers            string
	Yardstick            string
	DeliverableWord      string
	DomainGuidance       string
	SelectionChecks      string
	SelectionLead        string
	SelectionConformance string
	PromptPack           string
}

// Validate checks BenchPrompts invariants.
func (p BenchPrompts) Validate() error {
	if p.PromptPack == "" {
		p.PromptPack = "adapt"
	}
	if p.PromptPack != "adapt" && p.PromptPack != "tts" {
		return fmt.Errorf("BenchPrompts.prompt_pack must be 'adapt' or 'tts', got %q", p.PromptPack)
	}
	visible := map[string]string{
		"work": p.Work, "inv_levers": p.InvLevers, "dom_levers": p.DomLevers,
		"yardstick": p.Yardstick, "domain_guidance": p.DomainGuidance,
		"selection_checks": p.SelectionChecks,
	}
	for field, value := range visible {
		low := strings.ToLower(value)
		for _, bad := range []string{"benchmark", "evaluation", "grader", "rubric", "score"} {
			if strings.Contains(low, bad) {
				return fmt.Errorf("BenchPrompts.%s must describe the WORK, not its measurement (found %q)", field, bad)
			}
		}
	}
	return nil
}

// BenchSpec is a static description of one (benchmark, substrate) cell.
type BenchSpec struct {
	Name                 string
	RunsRoot             string
	InstallMode          string
	Prompts              BenchPrompts
	Channels             map[string]ChannelSpec
	DomainWord           string
	LeakGlobs            []string
	AllowSubagents       bool
	ReservedScripts      map[string]struct{}
	ProposerDeliverables string
	StateDeliverable     bool
	ObsTrunc             int
}

// Validate checks BenchSpec invariants.
func (b *BenchSpec) Validate() error {
	ok := false
	for _, m := range InstallModes {
		if m == b.InstallMode {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("unknown install_mode %q; expected one of %v", b.InstallMode, InstallModes)
	}
	if b.ProposerDeliverables == "" {
		b.ProposerDeliverables = "contents"
	}
	if b.ProposerDeliverables != "contents" && b.ProposerDeliverables != "names" {
		return fmt.Errorf("BenchSpec.proposer_deliverables must be 'contents' or 'names'")
	}
	for _, role := range []string{"proposer", "selector"} {
		if _, ok := b.Channels[role]; !ok {
			return fmt.Errorf("BenchSpec.channels must define %q", role)
		}
	}
	if b.ObsTrunc == 0 {
		b.ObsTrunc = 50000
	}
	if b.ObsTrunc < 10000 {
		return fmt.Errorf("obs_trunc=%d is in the range that fabricates defects; use >= 10000", b.ObsTrunc)
	}
	if b.DomainWord == "" {
		b.DomainWord = "domain"
	}
	for s := range b.ReservedScripts {
		if strings.Contains(s, "/") || !strings.HasSuffix(s, ".py") {
			return fmt.Errorf("BenchSpec.reserved_scripts takes bare .py BASENAMES, not paths: %q", s)
		}
	}
	return b.Prompts.Validate()
}

// RunView is what the method knows about one finished rollout. Never carries a score.
type RunView struct {
	Label     string  `json:"label"`
	RunID     string  `json:"run_id"`
	RunDir    string  `json:"run_dir"`
	OK        bool    `json:"ok"`
	Delivered bool    `json:"delivered"`
	CostUSD   float64 `json:"cost_usd"`
	WallS     float64 `json:"wall_s"`
	Error     string  `json:"error"`
}

// SubstrateLike is what core needs of a substrate.
type SubstrateLike interface {
	Name() string
	Surface() tree.SurfaceSpec
}

// BenchmarkAdapter is the seam every benchmark implements.
type BenchmarkAdapter interface {
	Spec() BenchSpec
	Substrate() SubstrateLike
	Tasks(split string) []string
	DomainOf(taskID string) string
	TaskPrompt(taskID string) string
	TaskFiles(taskID string) string // may be empty
	Execute(taskID string, harnessDir string, runID string, timeout int) (string, error)
	Delivered(runDir string) bool
	Trajectory(runDir string, obsTrunc int) string
	DeliverableFiles(runDir string) map[string]string
	DeliverableText(runDir string) string
	BaseHarness() string
	ValidateTree(t tree.Tree) error
	Score(runDir, taskID string) (map[string]any, error) // nil map means judge failed
}

// ResolvePrompts returns the task-specific prompt profile when available.
func ResolvePrompts(adapter BenchmarkAdapter, taskID string) BenchPrompts {
	if p, ok := adapter.(interface {
		PromptsFor(taskID string) BenchPrompts
	}); ok {
		return p.PromptsFor(taskID)
	}
	return adapter.Spec().Prompts
}

// AbsJoin joins path parts under root with slash normalisation for display.
func AbsJoin(parts ...string) string {
	return filepath.Clean(filepath.Join(parts...))
}
