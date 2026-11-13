// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package library stores per-domain adopted harnesses and lessons.
package library

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sculpt/core/context"
	"sculpt/core/runmark"
	"sculpt/core/tree"
)

// Slug replaces path separators in task ids.
func Slug(taskID string) string {
	return strings.ReplaceAll(taskID, "/", "__")
}

// DomainDir collapses separators in a domain name.
func DomainDir(domain string) string {
	re := regexp.MustCompile(`\s*[/\\]\s*`)
	out := strings.TrimSpace(re.ReplaceAllString(domain, "__"))
	if out == "" {
		return "unknown"
	}
	return out
}

// HasExperience reports whether experience has cue/lesson/evidence.
func HasExperience(exp map[string]string) bool {
	return runmark.HasExperience(exp)
}

// Library is one (run, domain) experience store.
type Library struct {
	Root    string
	Domain  string
	Surface tree.SurfaceSpec
}

// AddFn overrides Library.Add when non-nil (tests).
var AddFn func(l *Library, taskID string, harness tree.Tree, why, label, runID string, experience map[string]string) (string, error)

// SetAddForTest installs or clears the Add override.
func SetAddForTest(fn func(l *Library, taskID string, harness tree.Tree, why, label, runID string, experience map[string]string) (string, error)) {
	AddFn = fn
}

// Dir is the domain library directory.
func (l *Library) Dir() string {
	return filepath.Join(l.Root, DomainDir(l.Domain))
}

// Index is the INDEX.md path.
func (l *Library) Index() string {
	return filepath.Join(l.Dir(), "INDEX.md")
}

// EntryDir is the per-task entry directory.
func (l *Library) EntryDir(taskID string) string {
	return filepath.Join(l.Dir(), "harnesses", Slug(taskID))
}

// Exists reports whether INDEX.md is present.
func (l *Library) Exists() bool {
	_, err := os.Stat(l.Index())
	return err == nil
}

// ContextDir is handed to improver sessions, or empty when the domain is empty.
func (l *Library) ContextDir() string {
	if !l.Exists() {
		return ""
	}
	found := false
	_ = filepath.WalkDir(l.Dir(), func(path string, de os.DirEntry, err error) error {
		if err == nil && !de.IsDir() && de.Name() == "EXPERIENCE.md" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if !found {
		return ""
	}
	return l.Dir()
}

// Entries lists stored task slugs.
func (l *Library) Entries() []string {
	d := filepath.Join(l.Dir(), "harnesses")
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Add stores an adopted harness and experience; returns the entry dir or "".
func (l *Library) Add(taskID string, harness tree.Tree, why, label, runID string, experience map[string]string) (string, error) {
	if AddFn != nil {
		return AddFn(l, taskID, harness, why, label, runID, experience)
	}
	if !HasExperience(experience) {
		return "", nil
	}
	if err := context.AssertTreeHasNoGraderVerdicts(harness); err != nil {
		return "", err
	}
	blob := fmt.Sprintf("%v%s", experience, why)
	if err := context.AssertTreeHasNoGraderVerdicts(tree.Tree{"EXPERIENCE.md": blob}); err != nil {
		return "", err
	}
	d := l.EntryDir(taskID)
	_ = os.RemoveAll(filepath.Join(d, "harness"))
	_ = os.MkdirAll(filepath.Join(d, "harness"), 0o755)
	if _, err := tree.WriteTree(l.Surface, harness, filepath.Join(d, "harness"), false); err != nil {
		return "", err
	}
	var lines []string
	lines = append(lines, "# "+taskID, "",
		fmt.Sprintf("- adopted: `%s`", label)+func() string {
			if runID != "" {
				return fmt.Sprintf("  (run `%s`)", runID)
			}
			return ""
		}(),
		"",
	)
	if strings.TrimSpace(why) != "" {
		lines = append(lines, "## Why this was chosen", strings.TrimSpace(why), "")
	}
	if experience["edits"] != "" {
		lines = append(lines, "## What each edit did", experience["edits"], "")
	}
	if experience["lesson"] != "" {
		lines = append(lines, "## Transferable lesson")
		for _, f := range []string{"cue", "lesson", "evidence", "confidence"} {
			if experience[f] != "" {
				lines = append(lines, fmt.Sprintf("- **%s**: %s", f, strings.TrimSpace(experience[f])))
			}
		}
		lines = append(lines, "")
	}
	if err := os.WriteFile(filepath.Join(d, "EXPERIENCE.md"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	if err := l.RebuildIndex(); err != nil {
		return "", err
	}
	return d, nil
}

// RebuildIndex rewrites INDEX.md from disk entries.
func (l *Library) RebuildIndex() error {
	_ = os.MkdirAll(l.Dir(), 0o755)
	ents := l.Entries()
	var b strings.Builder
	b.WriteString("# Experience library\n\n")
	for _, e := range ents {
		b.WriteString(fmt.Sprintf("- [%s](harnesses/%s/EXPERIENCE.md)\n", e, e))
	}
	return os.WriteFile(l.Index(), []byte(b.String()), 0o644)
}
