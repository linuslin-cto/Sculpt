// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package jobbench

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sculpt/internal/config"
	"sculpt/substrate"
)

func (a *Adapter) defaultLaunch(req substrate.RunRequest, install substrate.Install) ([]string, error) {
	if len(req.RunIDs) != 1 {
		return nil, fmt.Errorf("JobBench rolls one run per invocation; got %d", len(req.RunIDs))
	}
	switch a.SubstrateName {
	case "oc":
		return a.launchOpenCodeRunner(req, install)
	case "cx":
		return a.launchCodexRunner(req, install)
	default:
		return nil, fmt.Errorf("jobbench: unknown substrate %q", a.SubstrateName)
	}
}

func (a *Adapter) launchOpenCodeRunner(req substrate.RunRequest, install substrate.Install) ([]string, error) {
	runner := config.JbOpenCodeRunner()
	if fi, err := os.Stat(runner); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("jobbench: jb_opencode_runner missing at %s (set HARNESS2_JB_OC_RUNNER)", runner)
	}
	py, err := findPython()
	if err != nil {
		return nil, err
	}
	if err := (substrate.Opencode{}).Preflight(); err != nil {
		return nil, err
	}
	taskID := req.TaskRef
	label := strings.ReplaceAll(req.RunIDs[0], "/", "__")
	wall := req.Timeout
	if wall <= 0 {
		wall = a.Timeout
	}
	render := install.StagingDir
	if render == "" {
		return nil, fmt.Errorf("jobbench: empty install staging dir")
	}
	ocCfg := filepath.Join(render, ".opencode")
	if fi, err := os.Stat(ocCfg); err != nil || !fi.IsDir() {
		_ = os.MkdirAll(ocCfg, 0o755)
	}
	model := config.SolverModelOC()
	if model == "" {
		return nil, fmt.Errorf("jobbench: set HARNESS2_SOLVER_MODEL before live OpenCode rollouts")
	}
	env := append(os.Environ(),
		"TASK_IDS="+taskID,
		"LABEL="+label,
		"HARNESS_RENDER_DIR="+render,
		"AGENT_NAME=",
		"TASKS_BASE_DIR="+a.Store,
		"OPENCODE_DIR="+config.OpenCodeDir,
		"MODEL_ID="+model,
		"TIMEOUT_PER_TASK="+fmt.Sprintf("%d", wall),
		"MAX_RETRIES=2",
		"MAX_CONCURRENT=1",
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*wall+600)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, runner)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("runner wall-timeout (%ds) for %s; artifacts left UNMARKED", 2*wall+600, label)
		}
		return nil, fmt.Errorf("jb_opencode_runner failed for %s: %w", label, err)
	}
	out := a.outDir(taskID, label)
	if !a.Delivered(out) {
		return nil, fmt.Errorf("JobBench produced no output for %s @ %s -- left UNMARKED", taskID, label)
	}
	return []string{out}, nil
}

func (a *Adapter) launchCodexRunner(req substrate.RunRequest, install substrate.Install) ([]string, error) {
	runner := config.JbCodexRunner()
	if fi, err := os.Stat(runner); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("jobbench: Codex runner missing at %s (set HARNESS2_JB_CX_RUNNER)", runner)
	}
	if err := (substrate.Codex{}).Preflight(); err != nil {
		return nil, err
	}
	taskID := req.TaskRef
	label := strings.ReplaceAll(req.RunIDs[0], "/", "__")
	wall := req.Timeout
	if wall <= 0 {
		wall = a.Timeout
	}
	render := install.StagingDir
	if render == "" {
		return nil, fmt.Errorf("jobbench: empty install staging dir")
	}
	model := config.SolverModelCX()
	if model == "" {
		return nil, fmt.Errorf("jobbench: set HARNESS2_SOLVER_MODEL_CX (or HARNESS2_SOLVER_MODEL) before Codex rollouts")
	}
	codexHome := config.CodexHome()
	env := append(os.Environ(),
		"TASK_IDS="+taskID,
		"LABEL="+label,
		"HARNESS_RENDER_DIR="+render,
		"TASKS_BASE_DIR="+a.Store,
		"BENCHMARK_MODELS="+model,
		"MAX_CONCURRENT_PER_MODEL=1",
		"TIMEOUT_PER_TASK="+fmt.Sprintf("%d", wall),
		"CODEX_HOME="+codexHome,
		"HARNESS2_CODEX_BIN="+filepath.Join(config.CodexDist(), "codex"),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wall+600)*time.Second)
	defer cancel()
	// Shell launcher (bash); on Windows requires Git Bash / WSL bash on PATH.
	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, fmt.Errorf("jobbench: bash not on PATH (needed for Codex jb runner): %w", err)
	}
	cmd := exec.CommandContext(ctx, bash, runner)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("runner wall-timeout (%ds) for %s; artifacts left UNMARKED", wall+600, label)
		}
		return nil, fmt.Errorf("jb Codex runner failed for %s: %w", label, err)
	}
	out := a.outDir(taskID, label)
	if !a.Delivered(out) {
		return nil, fmt.Errorf("JobBench produced no output for %s @ %s -- left UNMARKED", taskID, label)
	}
	return []string{out}, nil
}

func findPython() (string, error) {
	for _, name := range []string{"python", "python3", "py"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("python interpreter not on PATH (needed for jb_opencode_runner)")
}
