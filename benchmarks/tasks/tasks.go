// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package tasks discovers benchmark task IDs from on-disk datasets (Python: benchmarks/tasks.py).
package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sculpt/internal/config"
)

// Discover returns sorted task IDs mapped to their dataset-provided domain.
// rootOverride, when non-empty, replaces the default dataset root (for tests).
func Discover(bench, subset, rootOverride string) (map[string]string, error) {
	var root string
	tasks := map[string]string{}
	switch bench {
	case "jb":
		root = rootOverride
		if root == "" {
			root = filepath.Join(config.JbRepo(), "dataset", "main")
		}
		matches, err := filepath.Glob(filepath.Join(root, "*", "*", "task_folder", "TASK_INSTRUCTIONS.txt"))
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		for _, p := range matches {
			// root/<profession>/<task>/task_folder/TASK_INSTRUCTIONS.txt
			taskDir := filepath.Dir(filepath.Dir(p))
			rel, err := filepath.Rel(root, taskDir)
			if err != nil {
				return nil, err
			}
			id := filepath.ToSlash(rel)
			parts := strings.Split(id, "/")
			tasks[id] = parts[0]
		}
	case "lab":
		root = rootOverride
		if root == "" {
			root = filepath.Join(config.LabRepo(), "tasks")
		}
		matches, err := filepath.Glob(filepath.Join(root, "*", "*", "task.json"))
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		for _, p := range matches {
			taskDir := filepath.Dir(p)
			rel, err := filepath.Rel(root, taskDir)
			if err != nil {
				return nil, err
			}
			id := filepath.ToSlash(rel)
			parts := strings.Split(id, "/")
			tasks[id] = parts[0]
		}
	case "wb":
		if subset != "office" && subset != "web" && subset != "code" {
			return nil, fmt.Errorf("unknown benchmark/subset: %s/%s", bench, subset)
		}
		root = rootOverride
		if root == "" {
			root = filepath.Join(config.WbRepo(), "datasets", "wb-bench-"+subset+"-v1.0", "tasks")
		}
		matches, err := filepath.Glob(filepath.Join(root, "*", "task.toml"))
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		field := "category"
		if subset == "code" {
			field = "role"
		}
		for _, p := range matches {
			domain, err := tomlMetadataField(p, field)
			if err != nil {
				return nil, err
			}
			taskDir := filepath.Dir(p)
			if _, err := os.Stat(filepath.Join(taskDir, "instruction.md")); err != nil {
				return nil, fmt.Errorf("missing task instructions: %s", filepath.Join(taskDir, "instruction.md"))
			}
			tasks[filepath.Base(taskDir)] = domain
		}
	default:
		return nil, fmt.Errorf("unknown benchmark/subset: %s/%s", bench, subset)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no %s tasks found at %s; run sculpt-setup %s", bench, root, bench)
	}
	return tasks, nil
}

// Select returns task IDs for a split. path may be empty when split is "all".
func Select(tasks map[string]string, split, path, benchmark string) ([]string, error) {
	if split != "all" && split != "train" && split != "test" {
		return nil, fmt.Errorf("unknown split %q; use all, train, or test", split)
	}
	if path == "" {
		if split != "all" {
			return nil, fmt.Errorf("--split train/test requires --split-file")
		}
		out := make([]string, 0, len(tasks))
		for id := range tasks {
			out = append(out, id)
		}
		sort.Strings(out)
		return out, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data struct {
		Benchmark string   `json:"benchmark"`
		Train     []string `json:"train"`
		Test      []string `json:"test"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	if data.Benchmark != benchmark {
		return nil, fmt.Errorf("%s: expected a split for %s", path, benchmark)
	}
	train, test := data.Train, data.Test
	if err := checkHalf(path, "train", train); err != nil {
		return nil, err
	}
	if err := checkHalf(path, "test", test); err != nil {
		return nil, err
	}
	trainSet, testSet := toSet(train), toSet(test)
	for id := range trainSet {
		if _, ok := testSet[id]; ok {
			return nil, fmt.Errorf("%s: train and test overlap", path)
		}
	}
	unknown := []string{}
	for id := range trainSet {
		if _, ok := tasks[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	for id := range testSet {
		if _, ok := tasks[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		if len(unknown) > 5 {
			unknown = unknown[:5]
		}
		return nil, fmt.Errorf("%s: task IDs absent from the dataset: %v", path, unknown)
	}
	switch split {
	case "train":
		out := append([]string{}, train...)
		sort.Strings(out)
		return out, nil
	case "test":
		out := append([]string{}, test...)
		sort.Strings(out)
		return out, nil
	default:
		out := append(append([]string{}, train...), test...)
		sort.Strings(out)
		return out, nil
	}
}

func checkHalf(path, name string, ids []string) error {
	if ids == nil {
		return fmt.Errorf("%s: %s must be a list of task IDs", path, name)
	}
	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%s: duplicate task IDs in %s", path, name)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func toSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

// tomlMetadataField is a tiny TOML reader for metadata.<field> = "..." (wb tasks).
func tomlMetadataField(path, field string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	inMeta := false
	prefix := field + " ="
	prefixQ := field + "="
	for _, line := range strings.Split(string(b), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "[metadata]" {
			inMeta = true
			continue
		}
		if strings.HasPrefix(trim, "[") {
			inMeta = false
			continue
		}
		if !inMeta {
			continue
		}
		low := strings.TrimSpace(line)
		if strings.HasPrefix(low, prefix) || strings.HasPrefix(strings.ReplaceAll(low, " ", ""), prefixQ) {
			_, val, ok := strings.Cut(low, "=")
			if !ok {
				continue
			}
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			if strings.TrimSpace(val) == "" {
				return "", fmt.Errorf("%s: metadata.%s must be a nonempty string", path, field)
			}
			return strings.TrimSpace(val), nil
		}
	}
	return "", fmt.Errorf("%s: metadata.%s must be a nonempty string", path, field)
}
