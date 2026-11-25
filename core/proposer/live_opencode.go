// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package proposer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sculpt/internal/config"
)

const openCodePrompt = "Read MISSION.md first. Do what it asks. Reply with the requested output as " +
	"your final message AND save it verbatim to output/answer.md."

func bunAvailable(bun string) bool {
	if bun == "" {
		return false
	}
	if filepath.IsAbs(bun) {
		fi, err := os.Stat(bun)
		return err == nil && !fi.IsDir()
	}
	_, err := exec.LookPath(bun)
	return err == nil
}

// OpenCodeArgv builds the bun + OpenCode CLI argv (Python proposer._opencode).
func OpenCodeArgv(bun, ocDir, cwd, model, effort string) ([]string, error) {
	if bun == "" {
		bun = config.Bun
	}
	if ocDir == "" {
		ocDir = config.OpenCodeDir
	}
	entry := filepath.Join(ocDir, "packages", "opencode", "src", "index.ts")
	if fi, err := os.Stat(entry); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("no OpenCode checkout at %s (OPENCODE_DIR); clone pinned opencode under dependencies/", ocDir)
	}
	pkg := filepath.Join(ocDir, "packages", "opencode")
	cmd := []string{
		bun, "run", "--cwd", pkg, "--conditions=browser", "src/index.ts",
		"run", openCodePrompt,
		"--dir", cwd,
		"--model", model,
		"--format", "json",
	}
	if effort != "" {
		cmd = append(cmd, "--variant", effort)
	}
	return cmd, nil
}

// WriteOpenCodeSessionConfig writes opencode.json for the session workdir.
func WriteOpenCodeSessionConfig(cwd string, steps int) error {
	if steps <= 0 {
		steps = 120
	}
	doc := map[string]any{"agent": map[string]any{"build": map[string]any{"steps": steps}}}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cwd, "opencode.json"), b, 0o644)
}

// OpenCodeSessionEnv builds the scrubbed env for a live OpenCode session.
func OpenCodeSessionEnv(cwd string, extra []string) []string {
	perm, _ := json.Marshal(map[string]any{
		"external_directory": map[string]string{
			"*":         "deny",
			cwd:         "allow",
			cwd + "/**": "allow",
		},
	})
	keep := []string{
		"PATH", "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "TEMP", "TMP",
		"SYSTEMROOT", "COMSPEC", "LANG", "LC_ALL", "TZ",
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY",
		"GOOGLE_API_KEY", "XAI_API_KEY", "DEEPSEEK_API_KEY",
		"OPENAI_BASE_URL", "OPENAI_API_BASE",
	}
	env := config.ScrubEnv(keep)
	env = append(env,
		"OPENCODE_PERMISSION="+string(perm),
		"XDG_DATA_HOME="+filepath.Join(cwd, ".ocdata"),
		"XDG_STATE_HOME="+filepath.Join(cwd, ".ocstate"),
	)
	env = append(env, extra...)
	return env
}

func (o *OpenCodeSession) runLive(cwd, mission, system string, wallSec int, extraEnv []string) (string, error) {
	_ = mission
	if system != "" {
		existing, _ := os.ReadFile(filepath.Join(cwd, "MISSION.md"))
		body := string(existing)
		if !strings.HasPrefix(body, system) {
			_ = os.WriteFile(filepath.Join(cwd, "MISSION.md"), []byte(system+"\n\n---\n\n"+body), 0o644)
		}
	}
	if err := WriteOpenCodeSessionConfig(cwd, o.ChannelSpec.Steps); err != nil {
		return "", err
	}
	bun := o.BunBin
	if bun == "" {
		bun = config.Bun
	}
	ocDir := o.OpenCodeDir
	if ocDir == "" {
		ocDir = config.OpenCodeDir
	}
	argv, err := OpenCodeArgv(bun, ocDir, cwd, o.ChannelSpec.Model, o.ChannelSpec.Effort)
	if err != nil {
		return "", err
	}
	if wallSec <= 0 {
		wallSec = o.ChannelSpec.Timeout
	}
	if wallSec <= 0 {
		wallSec = 3600
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wallSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = OpenCodeSessionEnv(cwd, extraEnv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	out := stdout.String()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("timeout after %ds", wallSec)
	}
	if err != nil && strings.TrimSpace(out) == "" {
		tail := strings.TrimSpace(stderr.String())
		if len(tail) > 300 {
			tail = tail[len(tail)-300:]
		}
		return out, fmt.Errorf("rc=%v: %s", err, tail)
	}
	return out, nil
}
