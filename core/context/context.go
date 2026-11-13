// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

// Package context builds improver evidence workspaces.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sculpt/core/spec"
	"sculpt/core/tree"
)

var graderTokens = []string{
	`"pass_rate"`, `"score"`, "rubric", "grader", "evaluation", "benchmark",
}

// AssertTreeHasNoGraderVerdicts rejects trees that leak grading language.
func AssertTreeHasNoGraderVerdicts(t tree.Tree) error {
	for rel, body := range t {
		low := strings.ToLower(body)
		for _, bad := range graderTokens {
			if strings.Contains(low, strings.ToLower(bad)) {
				return fmt.Errorf("harness file %q contains grader/measurement token %q", rel, bad)
			}
		}
	}
	return nil
}

// Built is a constructed evidence context.
type Built struct {
	Root string
}

// Build materialises evidence under ctxDir/label for one improvement stage.
func Build(
	ctxDir string,
	adapter spec.BenchmarkAdapter,
	taskID string,
	rollouts []spec.RunView,
	harnessDir, libraryDir string,
	pool []PoolItem,
	decisions map[string]string,
) (Built, error) {
	root := filepath.Join(ctxDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Built{}, err
	}
	surface := adapter.Substrate().Surface()

	// Source material (excluding leak globs).
	if src := adapter.TaskFiles(taskID); src != "" {
		dst := filepath.Join(root, "source")
		if err := copyDirFiltered(src, dst, adapter.Spec().LeakGlobs); err != nil {
			return Built{}, err
		}
	}

	// Task prompt.
	_ = os.WriteFile(filepath.Join(root, "task.md"), []byte(adapter.TaskPrompt(taskID)), 0o644)

	// Base / current harness.
	hDest := filepath.Join(root, "harness")
	if harnessDir != "" {
		ht := tree.ReadTree(surface, harnessDir, nil)
		if _, err := tree.WriteTree(surface, ht, hDest, false); err != nil {
			return Built{}, err
		}
		baseDest := filepath.Join(root, "base_harness")
		if _, err := tree.WriteTree(surface, ht, baseDest, false); err != nil {
			return Built{}, err
		}
	}

	// Rollouts.
	rDir := filepath.Join(root, "rollouts")
	_ = os.MkdirAll(rDir, 0o755)
	for _, rv := range rollouts {
		body := ""
		if rv.OK {
			body = adapter.Trajectory(rv.RunDir, adapter.Spec().ObsTrunc)
		} else {
			body = "[ERROR] " + rv.Error
		}
		_ = os.WriteFile(filepath.Join(rDir, rv.Label+".md"), []byte(body), 0o644)
	}

	// Pool evidence.
	for _, item := range pool {
		pDir := filepath.Join(root, "pool", item.Label)
		_ = os.MkdirAll(pDir, 0o755)
		if item.HarnessDir != "" {
			pt := tree.ReadTree(surface, item.HarnessDir, nil)
			if _, err := tree.WriteTree(surface, pt, filepath.Join(pDir, "harness"), false); err != nil {
				return Built{}, err
			}
			if harnessDir != "" {
				before := tree.ReadTree(surface, harnessDir, nil)
				diff := tree.DiffSummary(surface, before, pt)
				_ = os.WriteFile(filepath.Join(pDir, "harness.diff"), []byte(fmt.Sprintf("%v\n", diff)), 0o644)
			}
		}
		if item.Run != nil && item.Run.OK {
			_ = os.WriteFile(filepath.Join(pDir, "rollout.md"), []byte(adapter.Trajectory(item.Run.RunDir, adapter.Spec().ObsTrunc)), 0o644)
		}
	}

	// Decisions.
	if len(decisions) > 0 {
		dDir := filepath.Join(root, "decisions")
		_ = os.MkdirAll(dDir, 0o755)
		for label, notes := range decisions {
			_ = os.WriteFile(filepath.Join(dDir, label+".md"), []byte(notes), 0o644)
		}
	}

	// Library prefix.
	if libraryDir != "" {
		libDest := filepath.Join(root, "library")
		_ = copyDirFiltered(libraryDir, libDest, nil)
	}

	return Built{Root: root}, nil
}

// PoolItem is one candidate harness + optional rollout for evidence.
type PoolItem struct {
	Label      string
	HarnessDir string
	Run        *spec.RunView
}

// Mission prepends a job brief to the standard improver framing.
func Mission(b Built, job string) string {
	return job + "\n\nEvidence root: `" + filepath.ToSlash(b.Root) + "`.\nEdit `context/harness/` in place.\n"
}

func copyDirFiltered(src, dst string, leakGlobs []string) error {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		base := filepath.Base(path)
		for _, g := range leakGlobs {
			if matched, _ := filepath.Match(g, base); matched {
				if de.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if de.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		return os.WriteFile(target, b, 0o644)
	})
}
