// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package runmark fingerprints harnesses and task inputs for resume safety.
package runmark

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Mark is the completion marker filename.
const Mark = ".adapt_complete"

// Version is the marker schema version (matches Python).
const Version = 1

func ignored(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		if p == "__pycache__" {
			return true
		}
	}
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".pyc") || base == ".DS_Store" || base == Mark
}

// Fingerprint hashes the harness tree; empty → "base"; missing → "absent".
func Fingerprint(dir string) string {
	if dir == "" {
		return "base"
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return "absent"
	}
	h := sha1.New()
	var files []string
	_ = filepath.WalkDir(dir, func(path string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if ignored(rel) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		h.Write([]byte(rel))
		h.Write([]byte{0})
		if err != nil {
			h.Write([]byte("<unreadable>"))
		} else {
			h.Write(b)
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// InputsFingerprint digests the task prompt plus source material.
func InputsFingerprint(prompt, filesDir string) string {
	h := sha1.New()
	h.Write([]byte("prompt"))
	h.Write([]byte{0})
	_, _ = io.WriteString(h, prompt)
	h.Write([]byte{0})
	if filesDir != "" {
		if fi, err := os.Stat(filesDir); err == nil && fi.IsDir() {
			var files []string
			_ = filepath.WalkDir(filesDir, func(path string, de os.DirEntry, err error) error {
				if err != nil || de.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(filesDir, path)
				rel = filepath.ToSlash(rel)
				if ignored(rel) {
					return nil
				}
				files = append(files, rel)
				return nil
			})
			sort.Strings(files)
			for _, rel := range files {
				b, err := os.ReadFile(filepath.Join(filesDir, filepath.FromSlash(rel)))
				_, _ = io.WriteString(h, rel)
				h.Write([]byte{0})
				if err != nil {
					h.Write([]byte("<unreadable>"))
				} else {
					h.Write(b)
				}
				h.Write([]byte{0})
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// HasExperience mirrors library.has_experience field checks.
func HasExperience(exp map[string]string) bool {
	if exp == nil {
		return false
	}
	for _, key := range []string{"cue", "lesson", "evidence"} {
		if strings.TrimSpace(exp[key]) == "" {
			return false
		}
	}
	return true
}

// MarkComplete writes .adapt_complete under where.
func MarkComplete(where, runID, harnessDir string, inputs *string, extra map[string]any) (string, error) {
	if err := os.MkdirAll(where, 0o755); err != nil {
		return "", err
	}
	doc := map[string]any{
		"version": Version,
		"run_id":  runID,
		"at":      float64(time.Now().Unix()) + float64(time.Now().Nanosecond())/1e9,
		"harness": Fingerprint(harnessDir),
	}
	if inputs != nil {
		doc["inputs"] = *inputs
	}
	for k, v := range extra {
		doc[k] = v
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(where, Mark)
	tmp := path + fmt.Sprintf(".%d.tmp", os.Getpid())
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// ReadMark loads the completion marker, or nil.
func ReadMark(where string) map[string]any {
	b, err := os.ReadFile(filepath.Join(where, Mark))
	if err != nil {
		return nil
	}
	var doc map[string]any
	if json.Unmarshal(b, &doc) != nil {
		return nil
	}
	return doc
}

func inputsMatch(m map[string]any, inputs *string) bool {
	recorded, ok := m["inputs"].(string)
	if !ok || inputs == nil {
		return true
	}
	return recorded == *inputs
}

// IsComplete reports whether a prior mark still matches the harness (and optional inputs).
func IsComplete(where, harnessDir string, matchHarness bool, inputs *string) bool {
	m := ReadMark(where)
	if m == nil {
		return false
	}
	if ver, ok := m["version"].(float64); !ok || int(ver) != Version {
		return false
	}
	if !inputsMatch(m, inputs) {
		return false
	}
	if !matchHarness {
		return true
	}
	got, _ := m["harness"].(string)
	return got == Fingerprint(harnessDir)
}

// WhyNot explains why a marker is or is not reusable.
func WhyNot(where, harnessDir string, inputs *string) string {
	m := ReadMark(where)
	if m == nil {
		return fmt.Sprintf("no %s -- never finished, or finished before completion was recorded", Mark)
	}
	if ver, ok := m["version"].(float64); !ok || int(ver) != Version {
		return fmt.Sprintf("marker version %v != %d", m["version"], Version)
	}
	if !inputsMatch(m, inputs) {
		return fmt.Sprintf(
			"task inputs %v != %v -- the task prompt or its source files changed since this rollout finished",
			m["inputs"], *inputs,
		)
	}
	want, got := Fingerprint(harnessDir), m["harness"]
	if want != got {
		return fmt.Sprintf("harness fingerprint %v != %v -- produced by a different configuration", got, want)
	}
	return "reusable"
}
