// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package substrate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const truncFmt = "…[+%d chars]"

// RenderEvents renders an OpenCode JSONL event stream into the shared observation grammar
// (Python substrate.opencode.render_events). Parsing is defensive across schema variants.
func RenderEvents(path string, obsTrunc, maxChars, argsTrunc int) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(failed to read trajectory: %v)", err)
	}
	textTags := map[string]string{"text": "ASSISTANT", "reasoning": "THINKING"}
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
		if errObj, ok := ev["error"].(map[string]any); ok {
			msg := errorMessage(errObj)
			lines = append(lines, "[ERROR] "+clip(msg, 300))
			continue
		}
		part := ev
		if p, ok := ev["part"].(map[string]any); ok {
			part = p
		}
		ptype, _ := part["type"].(string)
		if ptype == "" {
			ptype, _ = ev["type"].(string)
		}
		if tag, ok := textTags[ptype]; ok {
			txt := firstString(part, "text", "content")
			if strings.TrimSpace(txt) != "" {
				lines = append(lines, fmt.Sprintf("[%s] %s", tag, clip(txt, obsTrunc)))
			}
			continue
		}
		switch ptype {
		case "tool", "tool_use", "tool_call", "tool-invocation":
			name := firstString(part, "tool", "name")
			if name == "" {
				name = firstString(ev, "tool")
			}
			if name == "" {
				name = "tool"
			}
			state := part
			if s, ok := part["state"].(map[string]any); ok {
				state = s
			}
			status, _ := state["status"].(string)
			tin := firstAny(state, "input")
			if tin == nil {
				tin = firstAny(part, "input", "args")
			}
			tout := firstAny(state, "output", "result")
			if tout == nil {
				tout = firstAny(part, "output")
			}
			args := ""
			if tin != nil {
				b, _ := json.Marshal(tin)
				args = string(b)
			}
			lines = append(lines, fmt.Sprintf("[ACTION] %s(%s)", name, clip(args, argsTrunc)))
			if status == "error" {
				lines = append(lines, "[OBSERVATION ERROR] "+clip(fmt.Sprint(state["error"]), obsTrunc))
			} else if tout != nil {
				lines = append(lines, "[OBSERVATION] "+clip(fmt.Sprint(tout), obsTrunc))
			}
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

func errorMessage(errObj map[string]any) string {
	if data, ok := errObj["data"].(map[string]any); ok {
		if msg, ok := data["message"].(string); ok && msg != "" {
			return msg
		}
	}
	if msg, ok := errObj["message"].(string); ok && msg != "" {
		return msg
	}
	b, _ := json.Marshal(errObj)
	if len(b) > 300 {
		return string(b[:300])
	}
	return string(b)
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			return s
		}
	}
	return ""
}

func firstAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf(truncFmt, len(s)-n)
}
