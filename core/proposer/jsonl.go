// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package proposer

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
)

// ParseOpenCodeStdout folds `opencode run --format json` into (final text, error).
// Text events are joined so a trailing terminator cannot swallow the answer.
func ParseOpenCodeStdout(stdout string) (text, errMsg string) {
	var texts, errors []string
	sawJSON := false
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		sawJSON = true
		part, _ := ev["part"].(map[string]any)
		ptype, _ := part["type"].(string)
		if ptype == "" {
			ptype, _ = ev["type"].(string)
		}
		switch ptype {
		case "text":
			if t, ok := part["text"].(string); ok && strings.TrimSpace(t) != "" {
				texts = append(texts, t)
			}
		case "error":
			msg := ""
			switch e := ev["error"].(type) {
			case map[string]any:
				if data, ok := e["data"].(map[string]any); ok {
					msg, _ = data["message"].(string)
				}
				if msg == "" {
					msg, _ = e["name"].(string)
				}
			case string:
				msg = e
			}
			msg = strings.TrimSpace(msg)
			if msg != "" {
				errors = append(errors, msg)
			}
		}
	}
	if !sawJSON {
		return strings.TrimSpace(stripOCBanner(stdout)), ""
	}
	joinedErr := strings.Join(errors, "; ")
	if len(joinedErr) > 300 {
		joinedErr = joinedErr[len(joinedErr)-300:]
	}
	return strings.Join(texts, "\n\n"), joinedErr
}

// OpenCodeCost sums part.cost over step_finish events.
func OpenCodeCost(stdout string) float64 {
	total := 0.0
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		part, _ := ev["part"].(map[string]any)
		ptype, _ := part["type"].(string)
		if ptype == "" {
			ptype, _ = ev["type"].(string)
		}
		if ptype == "step-finish" || ptype == "step_finish" {
			switch c := part["cost"].(type) {
			case float64:
				total += c
			case int:
				total += float64(c)
			}
		}
	}
	return math.Round(total*1e6) / 1e6
}

func stripOCBanner(s string) string {
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "opencode ") || t == "" {
			continue
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

// Harvest copies a relative path out of a session workdir into a durable location.
func Harvest(sessionRoot, rel, dest string) error {
	src := filepath.Join(append([]string{sessionRoot}, strings.Split(filepath.ToSlash(rel), "/")...)...)
	return copyTree(src, dest)
}
