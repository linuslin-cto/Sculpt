// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package config holds deployment settings. Shell env overrides .env.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MethodVersion is the on-disk method schema version.
const MethodVersion = 2

var (
	// DefaultProposerChannel is used when no channel override is set.
	DefaultProposerChannel = "opencode"
	// ProposerModelByChannel maps channel -> default model id.
	ProposerModelByChannel = map[string]string{
		"opencode":   env("SCULPT_IMPROVER_MODEL", ""),
		"claude_cli": env("SCULPT_CLAUDE_MODEL", ""),
	}
	// PhaseWidth is the default concurrent width for propose/rollout phases (Python default: 2).
	PhaseWidth = envInt("SCULPT_PHASE_WIDTH", 2)
	// TransportAttempts is proposer transport retries.
	TransportAttempts = envInt("SCULPT_TRANSPORT_ATTEMPTS", 2)
	// RunsDir is the results root.
	RunsDir = env("SCULPT_RUNS_DIR", "")
	// DependenciesDir holds cloned benchmark / OpenCode checkouts (Python: dependencies/).
	DependenciesDir = ""
	// OpenCodeDir is the OpenCode monorepo used by bun (Python: config.OPENCODE_DIR).
	OpenCodeDir = env("OPENCODE_DIR", "")
	// Bun is the bun binary (Python: config.BUN); empty means look up "bun" on PATH.
	Bun = env("SCULPT_BUN", "")
)

func init() {
	LoadDotEnv(".env")
	// refresh after dotenv
	ProposerModelByChannel["opencode"] = env("SCULPT_IMPROVER_MODEL", ProposerModelByChannel["opencode"])
	ProposerModelByChannel["claude_cli"] = env("SCULPT_CLAUDE_MODEL", ProposerModelByChannel["claude_cli"])
	wd, _ := os.Getwd()
	if RunsDir == "" {
		RunsDir = filepath.Join(wd, "runs")
	}
	if DependenciesDir == "" {
		DependenciesDir = env("SCULPT_DEPENDENCIES", filepath.Join(wd, "dependencies"))
	}
	if OpenCodeDir == "" {
		OpenCodeDir = filepath.Join(DependenciesDir, "opencode")
	}
	if Bun == "" {
		Bun = filepath.Join(DependenciesDir, "tools", "node_modules", ".bin", "bun")
		if _, err := os.Stat(Bun); err != nil {
			Bun = "bun"
		}
	}
	PhaseWidth = envInt("SCULPT_PHASE_WIDTH", PhaseWidth)
	TransportAttempts = envInt("SCULPT_TRANSPORT_ATTEMPTS", TransportAttempts)
}

// JbRepo is the job-bench-eval checkout root.
func JbRepo() string { return filepath.Join(DependenciesDir, "job-bench-eval") }

// LabRepo is the harvey-labs checkout root.
func LabRepo() string { return filepath.Join(DependenciesDir, "harvey-labs") }

// WbRepo is the workbuddy-bench checkout root.
func WbRepo() string { return filepath.Join(DependenciesDir, "workbuddy-bench") }

// OpenCodeEntry is packages/opencode/src/index.ts under OpenCodeDir.
func OpenCodeEntry() string {
	return filepath.Join(OpenCodeDir, "packages", "opencode", "src", "index.ts")
}

// CodexDist is the Codex CLI dist directory.
func CodexDist() string {
	if v := env("SCULPT_CODEX_DIST", ""); v != "" {
		return v
	}
	return filepath.Join(DependenciesDir, "codex", "dist")
}

// CodexHome is CODEX_HOME for Codex rollouts.
func CodexHome() string {
	if v := env("CODEX_HOME", ""); v != "" {
		return v
	}
	return filepath.Join(DependenciesDir, "codex", "home")
}

// JbRunnerDir is third_party/jb_runner (baseline harnesses + codex shell launcher).
func JbRunnerDir() string {
	if v := env("SCULPT_JB_RUNNER_DIR", ""); v != "" {
		return v
	}
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "third_party", "jb_runner"),
		filepath.Join(wd, "upstream", "third_party", "jb_runner"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return candidates[0]
}

// JbOpenCodeRunner is the Python jb_opencode_runner.py path (env or optional upstream checkout).
func JbOpenCodeRunner() string {
	if v := env("SCULPT_JB_OC_RUNNER", ""); v != "" {
		return v
	}
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "third_party", "jb_opencode_runner.py"),
		filepath.Join(wd, "upstream", "benchmarks", "jb_opencode_runner.py"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return candidates[0]
}

// JbCodexRunner is the bash Codex launcher under jb_runner.
func JbCodexRunner() string {
	if v := env("SCULPT_JB_CX_RUNNER", ""); v != "" {
		return v
	}
	return filepath.Join(JbRunnerDir(), "run_benchmark_codex_cli_adapt.sh")
}

// JbBaseHarness returns the baseline harness tree for oc|cx.
func JbBaseHarness(substrate string) string {
	dir := JbRunnerDir()
	if substrate == "cx" {
		return filepath.Join(dir, "codex_base_harness")
	}
	return filepath.Join(dir, "harness_baseline_oc")
}

// SolverModelOC is the OpenCode solver model id.
func SolverModelOC() string { return env("SCULPT_SOLVER_MODEL", env("SCULPT_IMPROVER_MODEL", "")) }

// SolverModelCX is the Codex solver model id.
func SolverModelCX() string { return env("SCULPT_SOLVER_MODEL_CX", env("SCULPT_SOLVER_MODEL", "")) }

// JbJudgeModel is the JobBench judge model id.
func JbJudgeModel() string { return env("SCULPT_JUDGE_MODEL", "judge/stub") }

// JbJudgeAPIBase is the judge OpenAI-compatible API base URL.
func JbJudgeAPIBase() string { return env("SCULPT_JUDGE_BASE_URL", "") }

// JbJudgeAPIKey is an optional static judge API key.
func JbJudgeAPIKey() string { return env("SCULPT_JUDGE_API_KEY", "") }

// JbJudgePython is the venv python used for JobBench judging.
func JbJudgePython() string {
	if v := env("SCULPT_JB_JUDGE_PYTHON", ""); v != "" {
		return v
	}
	return filepath.Join(JbRepo(), ".venv", "bin", "python")
}

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("%s must be an integer, got %q", name, v))
	}
	return n
}

// LoadDotEnv loads KEY=VAL pairs from path into the process environment.
// Existing shell variables win (upstream Harness² config parity).
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[len("export "):])
		}
		key, val, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		} else if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimRight(val[:i], " ")
		}
		if _, ok := os.LookupEnv(key); !ok {
			_ = os.Setenv(key, val)
		}
	}
}

// ScrubEnv returns a minimal environment for child processes.
func ScrubEnv(keep []string) []string {
	allow := map[string]struct{}{
		"PATH": {}, "SYSTEMROOT": {}, "TEMP": {}, "TMP": {}, "HOME": {},
		"USERPROFILE": {}, "HOMEDRIVE": {}, "HOMEPATH": {}, "COMSPEC": {},
		"LANG": {}, "LC_ALL": {}, "TZ": {},
	}
	for _, k := range keep {
		allow[k] = struct{}{}
	}
	var out []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if _, ok := allow[k]; ok {
			out = append(out, e)
		}
	}
	return out
}
