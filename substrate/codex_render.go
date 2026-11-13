// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package substrate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RenderCodexEvents renders a Codex exec/rollout JSONL stream (Python substrate.codex.render_events).
// Covers the common item/payload shapes used by lab/jb trajectories; parsing is defensive.
func RenderCodexEvents(path string, obsTrunc, maxChars, argsTrunc int) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(failed to read trajectory: %v)", err)
	}
	var lines []string
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "{") {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(ln), &ev) != nil {
			continue
		}
		et, _ := ev["type"].(string)
		if item, ok := ev["item"].(map[string]any); ok {
			renderCodexItem(item, &lines, obsTrunc, argsTrunc)
			continue
		}
		if payload, ok := ev["payload"].(map[string]any); ok {
			renderCodexPayload(payload, &lines, obsTrunc, argsTrunc)
			continue
		}
		if et == "turn.completed" {
			if usage, ok := ev["usage"].(map[string]any); ok {
				lines = append(lines, fmt.Sprintf(
					"[SCULPT] turn completed | tokens in=%v cached=%v out=%v reasoning=%v",
					usage["input_tokens"], usage["cached_input_tokens"],
					usage["output_tokens"], usage["reasoning_output_tokens"],
				))
			}
			continue
		}
		if et == "turn.failed" || et == "error" || ev["error"] != nil {
			msg := textOf(ev["error"])
			if msg == "" {
				msg = textOf(ev)
			}
			if msg == "" {
				b, _ := json.Marshal(ev)
				msg = string(b)
				if len(msg) > 300 {
					msg = msg[:300]
				}
			}
			lines = append(lines, "[ERROR] "+clip(msg, 1000))
		}
	}
	rendered := strings.TrimSpace(strings.Join(lines, "\n"))
	if rendered == "" {
		return "(no parseable events in trajectory)"
	}
	if maxChars > 0 && len(rendered) > maxChars {
		dropped := len(rendered) - maxChars
		rendered = rendered[:maxChars] + "\n" + fmt.Sprintf(truncFmt, dropped)
	}
	return rendered
}

func renderCodexItem(item map[string]any, lines *[]string, obsTrunc, argsTrunc int) {
	itype, _ := item["type"].(string)
	switch itype {
	case "command_execution":
		cmd := clip(fmt.Sprint(item["command"]), argsTrunc)
		*lines = append(*lines, "[ACTION] shell: "+cmd)
		status, _ := item["status"].(string)
		exitCode := item["exit_code"]
		nonzero := exitCodeNonzero(exitCode)
		failed := status == "failed" || nonzero
		obsRaw := item["aggregated_output"]
		obs := ""
		if obsRaw != nil {
			obs = clip(fmt.Sprint(obsRaw), obsTrunc)
		}
		if status == "in_progress" && obs == "" {
			obs = "(no output captured — command still in_progress at end of stream)"
		}
		suffix := ""
		if nonzero {
			suffix = fmt.Sprintf(" | exit_code=%v", exitCode)
		}
		tag := "[OBSERVATION]"
		if failed {
			tag = "[OBSERVATION ERROR]"
		}
		*lines = append(*lines, tag+" "+obs+suffix)
	case "agent_message":
		if txt, ok := item["text"].(string); ok && strings.TrimSpace(txt) != "" {
			*lines = append(*lines, "[ASSISTANT] "+clip(txt, obsTrunc))
		}
	case "reasoning":
		if txt := textOf(item); strings.TrimSpace(txt) != "" {
			*lines = append(*lines, "[REASONING] "+clip(txt, obsTrunc))
		}
	case "error":
		*lines = append(*lines, "[SCULPT] "+clip(fmt.Sprint(item["message"]), 1000))
	}
}

func renderCodexPayload(payload map[string]any, lines *[]string, obsTrunc, argsTrunc int) {
	ptype, _ := payload["type"].(string)
	switch ptype {
	case "agent_message", "message":
		role, _ := payload["role"].(string)
		txt := textOf(payload)
		if role == "assistant" || ptype == "agent_message" {
			if strings.TrimSpace(txt) != "" {
				*lines = append(*lines, "[ASSISTANT] "+clip(txt, obsTrunc))
			}
		}
	case "reasoning":
		if txt := textOf(payload); strings.TrimSpace(txt) != "" {
			*lines = append(*lines, "[REASONING] "+clip(txt, obsTrunc))
		}
	case "function_call_output":
		*lines = append(*lines, "[OBSERVATION] "+clip(textOf(payload["output"]), obsTrunc))
	}
}

func exitCodeNonzero(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	default:
		s := fmt.Sprint(v)
		return s != "" && s != "0" && s != "<nil>"
	}
}

func textOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if t, ok := x["text"].(string); ok {
			return t
		}
		if t, ok := x["message"].(string); ok {
			return t
		}
		if parts, ok := x["content"].([]any); ok {
			var b strings.Builder
			for _, p := range parts {
				if m, ok := p.(map[string]any); ok {
					if t, ok := m["text"].(string); ok {
						b.WriteString(t)
					}
				}
			}
			return b.String()
		}
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
