// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package substrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"sculpt/core/tree"
	"sculpt/internal/config"
)

// Codex is the Codex CLI solver substrate.
type Codex struct {
	Launcher Launcher
}

// Name returns the short substrate token.
func (Codex) Name() string { return "cx" }

// Surface returns the Codex editable surface.
func (Codex) Surface() tree.SurfaceSpec { return CodexSurface }

// Materialise copies into a private temp staging dir for codex_root (Python parity).
func (c Codex) Materialise(treeDir, installMode string, ctx map[string]string) (Install, error) {
	_ = ctx
	if installMode != "" && installMode != "codex_root" {
		return Install{}, fmt.Errorf("codex only supports install_mode=codex_root, got %q", installMode)
	}
	if fi, err := os.Stat(treeDir); err != nil || !fi.IsDir() {
		return Install{}, fmt.Errorf("codex materialise: tree missing: %s", treeDir)
	}
	staged, err := stageTree("adapt-cx-root-", treeDir)
	if err != nil {
		return Install{}, err
	}
	return Install{
		StagingDir:  staged,
		Fingerprint: treeDir,
		Extras:      map[string]string{"mode": "codex_root"},
	}, nil
}

// Execute runs via the configured launcher.
func (c Codex) Execute(taskRef string, install Install, runIDs []string, timeout int) ([]string, error) {
	if c.Launcher == nil {
		return nil, fmt.Errorf("codex: launcher not configured")
	}
	return c.Launcher(RunRequest{TaskRef: taskRef, RunIDs: runIDs, Timeout: timeout}, install)
}

// ReadTrajectory renders a Codex exec/rollout JSONL stream (Python CodexSubstrate.read_trajectory).
func (Codex) ReadTrajectory(streamPath string, obsTrunc, maxChars, argsTrunc int) (string, error) {
	if streamPath == "" {
		return "", fmt.Errorf("codex: empty trajectory path")
	}
	if fi, err := os.Stat(streamPath); err != nil || fi.IsDir() {
		return "", fmt.Errorf("codex: trajectory missing: %s", streamPath)
	}
	if argsTrunc <= 0 {
		argsTrunc = 2000
	}
	return RenderCodexEvents(streamPath, obsTrunc, maxChars, argsTrunc), nil
}

// Preflight checks Codex CLI availability.
func (Codex) Preflight() error {
	dist := config.CodexDist()
	name := "codex"
	if runtime.GOOS == "windows" {
		name = "codex.exe"
	}
	bin := filepath.Join(dist, name)
	if fi, err := os.Stat(bin); err == nil && !fi.IsDir() {
		return nil
	}
	if _, err := exec.LookPath("codex"); err == nil {
		return nil
	}
	return fmt.Errorf("Codex CLI missing at %s and not on PATH (set SCULPT_CODEX_DIST)", bin)
}

// CodexSurface mirrors Python CODEX_SURFACE.
var CodexSurface = tree.SurfaceSpec{
	Name: "codex",
	AlwaysOn: []string{
		"AGENTS.md",
	},
	AlwaysOnMax: 8000,
	AlwaysOnCapReason: "AGENTS.md is injected whole into EVERY turn, and codex additionally " +
		"truncates it silently at 32,768 bytes",
	SkillDir:      ".agents/skills",
	SkillNameRule: "overrides_dir",
	TriggerHint: "Fix it by adding a trigger line naming `{name}` to AGENTS.md, e.g. \"When " +
		"<the situation this skill handles>, read .agents/skills/{name}/SKILL.md and follow it.\"",
	AutotriggerFile:      "AGENTS.md",
	TriggerLine:          "- When {desc}, read .agents/skills/{name}/SKILL.md and follow it.\n",
	SubagentRequiresMode: false,
	PluginDir:            "",
	HooksFile:            ".codex/hooks.json",
	HookEvents: map[string]struct{}{
		"PreToolUse": {}, "PostToolUse": {}, "PermissionRequest": {},
		"PreCompact": {}, "PostCompact": {}, "SessionStart": {},
		"SubagentStart": {}, "SubagentStop": {}, "UserPromptSubmit": {}, "Stop": {},
	},
	CompileSkillScripts: true,
	RejectEmptyAlwaysOn: true,
	Stubs: [][2]string{
		{"scripts/README.md", "<!-- c7: deterministic helpers -->\n"},
		{"sub_agents/README.md", "<!-- c5: delegated roles -->\n"},
		{".codex/README.md", "<!-- c8: lifecycle hooks -->\n"},
		{".agents/skills/README.md", "<!-- c6: reusable workflows -->\n"},
	},
}
