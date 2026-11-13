// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package substrate provides agent framework adapters (OpenCode, Codex).
package substrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sculpt/core/tree"
	"sculpt/internal/config"
)

// Opencode is the OpenCode agent framework as a solver substrate.
type Opencode struct {
	Launcher Launcher
}

// Name returns the short substrate token.
func (Opencode) Name() string { return "oc" }

// Surface returns the editable c1..c8 OpenCode surface.
func (Opencode) Surface() tree.SurfaceSpec { return OpenCodeSurface }

// Materialise copies the editable tree into a private temp staging dir (Python parity).
func (o Opencode) Materialise(treeDir, installMode string, ctx map[string]string) (Install, error) {
	_ = ctx
	mode := installMode
	if mode == "" {
		mode = "workspace_config"
	}
	switch mode {
	case "workspace_agents_md", "home_dir_jinja", "workspace_config":
	default:
		return Install{}, fmt.Errorf("unknown opencode install_mode %q", installMode)
	}
	if fi, err := os.Stat(treeDir); err != nil || !fi.IsDir() {
		return Install{}, fmt.Errorf("opencode materialise: tree missing: %s", treeDir)
	}
	staged, err := stageTree("adapt-oc-"+mode+"-", treeDir)
	if err != nil {
		return Install{}, err
	}
	return Install{
		StagingDir:  staged,
		Fingerprint: treeDir,
		Extras:      map[string]string{"mode": mode},
	}, nil
}

// Execute runs rollouts via the benchmark-supplied launcher.
func (o Opencode) Execute(taskRef string, install Install, runIDs []string, timeout int) ([]string, error) {
	if o.Launcher == nil {
		return nil, fmt.Errorf("opencode: launcher not configured")
	}
	return o.Launcher(RunRequest{TaskRef: taskRef, RunIDs: runIDs, Timeout: timeout}, install)
}

// ReadTrajectory renders an OpenCode JSONL event stream (Python OpencodeSubstrate.read_trajectory).
func (Opencode) ReadTrajectory(streamPath string, obsTrunc, maxChars, argsTrunc int) (string, error) {
	if streamPath == "" {
		return "", fmt.Errorf("opencode: empty trajectory path")
	}
	if fi, err := os.Stat(streamPath); err != nil || fi.IsDir() {
		return "", fmt.Errorf("opencode: trajectory missing: %s", streamPath)
	}
	return RenderEvents(streamPath, obsTrunc, maxChars, argsTrunc), nil
}

// Preflight checks OpenCode availability before spending money.
func (Opencode) Preflight() error {
	entry := config.OpenCodeEntry()
	if fi, err := os.Stat(entry); err != nil || fi.IsDir() {
		return fmt.Errorf("OpenCode missing at %s (set OPENCODE_DIR or clone under dependencies/opencode)", config.OpenCodeDir)
	}
	bun := config.Bun
	if filepath.IsAbs(bun) {
		if fi, err := os.Stat(bun); err != nil || fi.IsDir() {
			return fmt.Errorf("bun binary missing at %s (set SCULPT_BUN)", bun)
		}
		return nil
	}
	if _, err := exec.LookPath(bun); err != nil {
		return fmt.Errorf("bun not on PATH (set SCULPT_BUN): %w", err)
	}
	return nil
}

// OpenCodeSurface is the editable surface for OpenCode.
var OpenCodeSurface = tree.SurfaceSpec{
	Name: "opencode",
	AlwaysOn: []string{
		"systemprompt.md",
		"guardrails.md",
		"LongTermMEMORY.md",
		"tool_descriptions/tool_guidance.md",
	},
	AlwaysOnMax:       24000,
	AlwaysOnCapReason: "every token here competes with the agent's own output budget",
	SkillDir:          "skills",
	SkillNameRule:     "must_match",
	TriggerHint: "Fix it by adding a trigger line naming `{name}` to systemprompt.md or " +
		"guardrails.md, e.g. \"When <the situation this skill handles>, load the `{name}` skill.\"",
	AutotriggerFile:      "guardrails.md",
	TriggerLine:          "- When {desc}, load the `{name}` skill.\n",
	SubagentRequiresMode: true,
	PluginDir:            "plugin",
	HooksFile:            "",
	Stubs: [][2]string{
		{"scripts/README.md", "<!-- c7: deterministic helpers, run as `python3 scripts/<name>.py`. Add one by emitting scripts/<name>.py. -->\n"},
		{"sub_agents/README.md", "<!-- c5: delegated subagents. Add one as sub_agents/<name>/AGENT.md with frontmatter `mode: subagent` and `description:`. -->\n"},
		{"plugin/README.md", "<!-- c8: OpenCode plugins. Add one as plugin/<name>.ts. -->\n"},
		{"skills/README.md", "<!-- c6: reusable workflows. Add one as skills/<name>/SKILL.md with frontmatter `name:` matching the directory and a `description:`. -->\n"},
	},
}
