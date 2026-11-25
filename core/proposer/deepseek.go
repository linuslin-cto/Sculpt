// Copyright 2026 The Sculpt Authors.
// Licensed under the Apache License, Version 2.0

package proposer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sculpt/core/spec"
)

// DeepSeek is an OpenAI-compatible chat improver (live model, no OpenCode binary).
// Used for live self-evolution when OpenCode checkout is unavailable.
type DeepSeek struct {
	ChannelSpec spec.ChannelSpec
	APIKey      string
	BaseURL     string // e.g. https://api.deepseek.com/v1
	HTTPClient  *http.Client
	calls       []Reply
}

// NewDeepSeek builds a live DeepSeek chat proposer.
func NewDeepSeek(ch spec.ChannelSpec) (*DeepSeek, error) {
	if strings.TrimSpace(ch.Model) == "" {
		return nil, fmt.Errorf("Set SCULPT_IMPROVER_MODEL before starting model sessions")
	}
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("set DEEPSEEK_API_KEY (or OPENAI_API_KEY) for live DeepSeek sessions")
	}
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" {
		base = "https://api.deepseek.com/v1"
	}
	base = strings.TrimRight(base, "/")
	return &DeepSeek{
		ChannelSpec: ch,
		APIKey:      key,
		BaseURL:     base,
		HTTPClient:  &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (d *DeepSeek) Spec() spec.ChannelSpec { return d.ChannelSpec }
func (d *DeepSeek) Calls() []Reply         { return append([]Reply(nil), d.calls...) }

// Call sends the mission (+ optional context files) to DeepSeek chat and returns the reply text.
// When harvest is set, copies context/harness into a workspace and applies ```python / ```choice blocks.
func (d *DeepSeek) Call(mission, contextDir, label string, harvest string, maxAttempts int) (Reply, error) {
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	start := time.Now()
	var last string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		text, err := d.chat(mission, contextDir, label)
		if err != nil {
			last = err.Error()
			continue
		}
		if strings.TrimSpace(text) == "" {
			last = "empty reply"
			continue
		}
		reply := Reply{
			Label: label, Text: text, OK: true,
			WallS: time.Since(start).Seconds(), Attempts: attempt,
			Channel: "deepseek",
		}
		if harvest != "" && contextDir != "" {
			ws, err := os.MkdirTemp("", "sculpt-ds-*")
			if err != nil {
				return Reply{}, err
			}
			src := filepath.Join(contextDir, "harness")
			if fi, e := os.Stat(src); e != nil || !fi.IsDir() {
				src = contextDir
			}
			if err := copyTree(src, ws); err != nil {
				_ = os.RemoveAll(ws)
				last = "harvest copy: " + err.Error()
				continue
			}
			applyScriptEdits(ws, text)
			reply.Workspace = ws
		}
		d.calls = append(d.calls, reply)
		return reply, nil
	}
	r := Reply{Label: label, OK: false, Attempts: maxAttempts, Channel: "deepseek", Error: last, WallS: time.Since(start).Seconds()}
	d.calls = append(d.calls, r)
	return r, &Error{Message: fmt.Sprintf("[%s] deepseek: %s", label, last), Attempts: maxAttempts}
}

func (d *DeepSeek) chat(mission, contextDir, label string) (string, error) {
	model := d.ChannelSpec.Model
	// API wants bare model id; strip provider prefix if present.
	if i := strings.LastIndex(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	var ctxNote strings.Builder
	if contextDir != "" {
		_ = filepath.WalkDir(contextDir, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() {
				return err
			}
			if de.Name() == "summarize.py" || strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".csv") {
				b, rerr := os.ReadFile(path)
				if rerr != nil || len(b) > 8000 {
					return nil
				}
				rel, _ := filepath.Rel(contextDir, path)
				ctxNote.WriteString("\n--- file: " + rel + " ---\n")
				ctxNote.Write(b)
			}
			return nil
		})
	}
	user := mission
	if ctxNote.Len() > 0 {
		user += "\n\n# Context files\n" + ctxNote.String()
	}
	user += "\n\n# Label\n" + label + "\nRespond with the exact tags/fences the mission requests."

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are the Sculpt harness improver. Follow the mission exactly. Prefer minimal correct edits."},
			{"role": "user", "content": user},
		},
		"temperature": 0.2,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, d.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.APIKey)
	res, err := d.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", res.StatusCode, clipBytes(respBody, 400))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return parsed.Choices[0].Message.Content, nil
}

func applyScriptEdits(workspace, text string) {
	// Prefer fenced python block → scripts/summarize.py
	if i := strings.Index(text, "```python"); i >= 0 {
		rest := text[i+len("```python"):]
		if j := strings.Index(rest, "```"); j >= 0 {
			code := strings.TrimSpace(rest[:j])
			_ = os.MkdirAll(filepath.Join(workspace, "scripts"), 0o755)
			_ = os.WriteFile(filepath.Join(workspace, "scripts", "summarize.py"), []byte(code+"\n"), 0o644)
		}
	}
}

func clipBytes(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n]
	}
	return s
}
