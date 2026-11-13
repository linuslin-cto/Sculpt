// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package jobbench

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TempBase is the JobBench OpenCode workspace root (Python jb_opencode_runner.TEMP_BASE).
// Overridable in tests via SetTempBaseForTest.
var TempBase = filepath.Join(os.TempDir(), "hevo_ws")

// SetTempBaseForTest overrides TempBase; pass "" to restore default.
func SetTempBaseForTest(dir string) {
	if dir == "" {
		TempBase = filepath.Join(os.TempDir(), "hevo_ws")
		return
	}
	TempBase = dir
}

// SandboxDir is this rollout's npm/python/tmp fence (Python _sandbox).
func SandboxDir(workspace string) string {
	sum := sha1.Sum([]byte(filepath.Base(workspace)))
	return filepath.Join(filepath.Dir(workspace), fmt.Sprintf("sb-%x", sum[:6]))
}

// IsolationEnv sends npm/pip/temp installs into the rollout sandbox (Python _isolation_env).
func IsolationEnv(workspace, pathEnv string) map[string]string {
	sandbox := SandboxDir(workspace)
	npm := filepath.Join(sandbox, "npm-global")
	user := filepath.Join(sandbox, "python-user")
	parts := []string{
		filepath.Join(npm, "bin"),
		filepath.Join(user, "bin"),
	}
	if pathEnv != "" {
		parts = append(parts, pathEnv)
	}
	return map[string]string{
		"NPM_CONFIG_PREFIX": npm,
		"PYTHONUSERBASE":    user,
		"TMPDIR":            filepath.Join(sandbox, "tmp"),
		"PATH":              strings.Join(parts, string(os.PathListSeparator)),
	}
}

// PermissionJSON is the OpenCode external_directory fence for one workspace.
func PermissionJSON(workspace string) (string, error) {
	base := filepath.ToSlash(TempBase)
	ws := filepath.ToSlash(workspace)
	sb := filepath.ToSlash(SandboxDir(workspace))
	body := map[string]any{
		"external_directory": map[string]string{
			"*":          "deny",
			"/tmp":       "allow",
			"/tmp/*":     "allow",
			"/tmp/**":    "allow",
			base + "/*":  "allow",
			base + "/**": "deny",
			sb + "/**":   "allow",
			ws + "/**":   "allow",
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// StageSandbox prepares npm/python/tmp dirs under the sandbox (Python _stage_sandbox).
func StageSandbox(workspace string) error {
	sandbox := SandboxDir(workspace)
	_ = os.RemoveAll(sandbox)
	for _, name := range []string{"npm-global", "python-user", "tmp"} {
		if err := os.MkdirAll(filepath.Join(sandbox, name), 0o755); err != nil {
			return err
		}
	}
	return nil
}
