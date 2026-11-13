// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package lab is a stub for the LAB BenchmarkAdapter port.
package lab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sculpt/benchmarks/tasks"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/internal/config"
	"sculpt/substrate"
)

// Name is the short bench token.
const Name = "lab"

// Adapter is a placeholder until the full LAB runner is wired.
type Adapter struct {
	RunsRoot      string
	TasksRoot     string // harvey-labs/tasks
	SplitFile     string
	SubstrateName string
	SkillMode     string // default | full — affects oc base prompt when live compose lands
	sub           spec.SubstrateLike
	specVal       spec.BenchSpec

	mu       sync.Mutex
	cache    map[string]string
	baseOnce sync.Once
	basePath string
	baseErr  error
}

// New constructs a LAB stub adapter.
func New(runsRoot, substrateName string) (*Adapter, error) {
	var sub spec.SubstrateLike
	switch substrateName {
	case "oc", "":
		sub = substrate.Opencode{}
		substrateName = "oc"
	case "cx":
		sub = substrate.Codex{}
	default:
		return nil, fmt.Errorf("unknown substrate %q", substrateName)
	}
	ch, err := spec.NewChannelSpec("opencode", "local/stub", "high", 120, 3600)
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		RunsRoot:      runsRoot,
		TasksRoot:     filepath.Join(config.LabRepo(), "tasks"),
		SubstrateName: substrateName,
		SkillMode:     "default",
		sub:           sub,
	}
	a.specVal = spec.BenchSpec{
		Name:        "lab-" + substrateName,
		RunsRoot:    runsRoot,
		InstallMode: "workspace_config",
		Prompts:     spec.BenchPrompts{Work: "long-horizon agentic tasks", DomainWord: "domain", PromptPack: "adapt", SelectionConformance: "full", DeliverableWord: "deliverable"},
		Channels:    map[string]spec.ChannelSpec{"proposer": ch, "selector": ch},
		ObsTrunc:    50000,
	}
	if err := a.specVal.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adapter) Spec() spec.BenchSpec          { return a.specVal }
func (a *Adapter) Substrate() spec.SubstrateLike { return a.sub }

func (a *Adapter) load() (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cache != nil {
		return a.cache, nil
	}
	m, err := tasks.Discover("lab", "", a.TasksRoot)
	if err != nil {
		return nil, err
	}
	a.cache = m
	return m, nil
}

func (a *Adapter) Tasks(split string) []string {
	m, err := a.load()
	if err != nil {
		return nil
	}
	ids, err := tasks.Select(m, split, a.SplitFile, "lab")
	if err != nil {
		return nil
	}
	return ids
}

func (a *Adapter) DomainOf(taskID string) string {
	if i := strings.Index(taskID, "/"); i > 0 {
		return taskID[:i]
	}
	m, err := a.load()
	if err != nil {
		return "unknown"
	}
	if d, ok := m[taskID]; ok {
		return d
	}
	return "unknown"
}

func (a *Adapter) TaskPrompt(taskID string) string {
	p := filepath.Join(a.TasksRoot, filepath.FromSlash(taskID), "task.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *Adapter) TaskFiles(taskID string) string {
	d := filepath.Join(a.TasksRoot, filepath.FromSlash(taskID), "documents")
	if fi, err := os.Stat(d); err == nil && fi.IsDir() {
		return d
	}
	return ""
}

func (a *Adapter) Execute(taskID, harnessDir, runID string, timeout int) (string, error) {
	return "", fmt.Errorf("lab: Execute not wired")
}

// Delivered is true when run_dir/output contains any file (Python LabAdapter.delivered).
func (a *Adapter) Delivered(runDir string) bool {
	root := filepath.Join(runDir, "output")
	found := false
	_ = filepath.WalkDir(root, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !de.IsDir() {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
func (a *Adapter) Trajectory(runDir string, obsTrunc int) string {
	name := "opencode_session.jsonl"
	if a.SubstrateName == "cx" {
		name = "codex_session.jsonl"
	}
	stream := filepath.Join(runDir, name)
	if fi, err := os.Stat(stream); err != nil || fi.IsDir() {
		return fmt.Sprintf("(no trajectory recorded for %s)", filepath.Base(runDir))
	}
	var (
		text string
		err  error
	)
	if a.SubstrateName == "cx" {
		text, err = (substrate.Codex{}).ReadTrajectory(stream, obsTrunc, 1_000_000_000, 2000)
	} else {
		text, err = (substrate.Opencode{}).ReadTrajectory(stream, obsTrunc, 1_000_000_000, 400)
	}
	if err != nil {
		return ""
	}
	return text
}
func (a *Adapter) DeliverableFiles(runDir string) map[string]string { return nil }
func (a *Adapter) DeliverableText(runDir string) string             { return "" }

// BaseHarness materialises H0 once per adapter (Python LabAdapter.base_harness).
// OpenCode live compose via harvey-labs is not required offline — a stock surface is written.
func (a *Adapter) BaseHarness() string {
	a.baseOnce.Do(func() {
		root := a.specVal.RunsRoot
		if root == "" {
			root = a.RunsRoot
		}
		var dir string
		if a.SubstrateName == "cx" {
			dir = filepath.Join(root, fmt.Sprintf("_base_harness_cx_%d", os.Getpid()))
		} else {
			mode := a.SkillMode
			if mode == "" {
				mode = "default"
			}
			dir = filepath.Join(root, fmt.Sprintf("_base_harness_%s_%d", mode, os.Getpid()))
		}
		var t tree.Tree
		var surface tree.SurfaceSpec
		if a.SubstrateName == "cx" {
			surface = substrate.Codex{}.Surface()
			t = tree.Tree{"AGENTS.md": CodexBaseBody}
		} else {
			surface = substrate.Opencode{}.Surface()
			t = tree.Tree{
				"systemprompt.md":                     OpenCodeBaseBodyStub,
				"guardrails.md":                       "",
				"LongTermMEMORY.md":                   "",
				"tool_descriptions/tool_guidance.md": "",
			}
		}
		if _, err := tree.WriteTree(surface, t, dir, true); err != nil {
			a.baseErr = err
			return
		}
		a.basePath = dir
	})
	if a.baseErr != nil {
		return ""
	}
	return a.basePath
}

func (a *Adapter) ValidateTree(t tree.Tree) error { return nil }
func (a *Adapter) Score(runDir, taskID string) (map[string]any, error) {
	_ = taskID
	p := filepath.Join(runDir, "scores.json")
	b, err := os.ReadFile(p)
	if err != nil {
		// Live evaluation.scoring is not wired; cache miss stays ungraded (Python returns None).
		return nil, fmt.Errorf("lab: scores.json missing at %s (live LAB judge not wired)", p)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("lab: unreadable scores.json: %w", err)
	}
	out := map[string]any{}
	if v, ok := raw["n_passed"]; ok {
		out["n_passed"] = v
	}
	if v, ok := raw["n_criteria"]; ok {
		out["n_criteria"] = v
	}
	if v, ok := raw["all_pass"]; ok {
		out["all_pass"] = v
	}
	return out, nil
}

// ReadyErrors lists what must be ported before real LAB runs.
func ReadyErrors(a *Adapter) []string {
	var out []string
	if a == nil {
		return []string{"lab adapter is nil"}
	}
	if a.BaseHarness() == "" {
		out = append(out, "lab base harness path not configured")
	}
	if _, err := a.load(); err != nil {
		out = append(out, "lab task discovery: "+err.Error())
	} else if len(a.Tasks("all")) == 0 {
		out = append(out, "lab task discovery returned zero tasks")
	}
	return out
}
