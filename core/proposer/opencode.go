// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package proposer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sculpt/internal/config"
)

// SessionRunner executes one OpenCode CLI invocation (injectable for tests).
type SessionRunner func(cwd, mission, system string, wallSec int, env []string) (stdout string, err error)

// OpenCodeSession is the live OpenCode improver channel.
type OpenCodeSession struct {
	*Base
	BunBin      string
	OpenCodeDir string
	Runner      SessionRunner
}

// NewOpenCode builds a live-channel proposer.
func NewOpenCode(b *Base) *OpenCodeSession {
	return &OpenCodeSession{Base: b, BunBin: config.Bun, OpenCodeDir: config.OpenCodeDir}
}

// Call stages an isolated context workspace, runs OpenCode (or Runner), and harvests.
func (o *OpenCodeSession) Call(mission, contextDir, label string, harvest string, maxAttempts int) (Reply, error) {
	if maxAttempts <= 0 {
		maxAttempts = o.Attempts
	}
	start := time.Now()
	spent := 0.0
	last := ""
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		reply, attemptLast, cost, ok := o.oneAttempt(mission, contextDir, label, harvest, start, attempt)
		spent += cost
		if ok {
			reply.CostUSD = spent
			o.calls = append(o.calls, reply)
			return reply, nil
		}
		if attemptLast != "" {
			last = attemptLast
		}
	}
	r := Reply{
		Label: label, OK: false, WallS: time.Since(start).Seconds(),
		Attempts: maxAttempts, Channel: "opencode", Error: last, CostUSD: spent,
	}
	o.calls = append(o.calls, r)
	return r, &Error{
		Message:  fmt.Sprintf("[%s] opencode/%s: %s", label, o.ChannelSpec.Model, last),
		Attempts: maxAttempts,
		CostUSD:  spent,
	}
}

func (o *OpenCodeSession) oneAttempt(mission, contextDir, label, harvest string, start time.Time, attempt int) (Reply, string, float64, bool) {
	work, err := os.MkdirTemp("", "sculpt-oc-*")
	if err != nil {
		return Reply{}, err.Error(), 0, false
	}
	defer os.RemoveAll(work)

	ctxCopy := filepath.Join(work, "context")
	if contextDir != "" {
		if err := copyTree(contextDir, ctxCopy); err != nil {
			return Reply{}, fmt.Sprintf("copy context: %v", err), 0, false
		}
	} else {
		_ = os.MkdirAll(ctxCopy, 0o755)
	}
	_ = os.MkdirAll(filepath.Join(work, "output"), 0o755)
	_ = os.WriteFile(filepath.Join(work, "MISSION.md"), []byte(mission), 0o644)

	cacheKey := sha256.Sum256([]byte(label + "|" + mission + "|" + contextDir))
	key := hex.EncodeToString(cacheKey[:])[:16]
	audit := ""
	if o.AuditDir != "" {
		audit = filepath.Join(o.AuditDir, label+"-"+key)
		_ = os.MkdirAll(audit, 0o755)
	}

	var stdout string
	switch {
	case o.Runner != nil:
		stdout, err = o.Runner(work, mission, "", o.ChannelSpec.Timeout, nil)
	default:
		bun := o.BunBin
		if bun == "" {
			bun = config.Bun
		}
		if !bunAvailable(bun) {
			return Reply{}, "opencode live path: bun not on PATH; set up OpenCode or use demo.Proposer", 0, false
		}
		stdout, err = o.runLive(work, mission, "", o.ChannelSpec.Timeout, nil)
	}
	cost := OpenCodeCost(stdout)
	if err != nil && strings.TrimSpace(stdout) == "" {
		return Reply{}, err.Error(), cost, false
	}
	text, parseErr := ParseOpenCodeStdout(stdout)
	if parseErr != "" {
		return Reply{}, parseErr, cost, false
	}
	if ans := filepath.Join(work, "output", "answer.md"); fileNonEmpty(ans) {
		if b, rerr := os.ReadFile(ans); rerr == nil && strings.TrimSpace(string(b)) != "" {
			text = string(b)
		}
	}
	if strings.TrimSpace(text) == "" {
		return Reply{}, "empty reply", cost, false
	}
	reply := Reply{
		Label: label, Text: text, OK: true,
		WallS: time.Since(start).Seconds(), Attempts: attempt,
		Channel: "opencode", Audit: audit,
	}
	if harvest != "" {
		dest := filepath.Join(work, "_harvest")
		if hErr := Harvest(work, harvest, dest); hErr != nil {
			return Reply{}, "harvest miss: " + hErr.Error(), cost, false
		}
		keep := ""
		if o.AuditDir != "" {
			keep = filepath.Join(o.AuditDir, label+"-"+key+"-workspace")
			_ = os.RemoveAll(keep)
		} else {
			keep, _ = os.MkdirTemp("", "sculpt-harvest-*")
		}
		_ = copyTree(dest, keep)
		reply.Workspace = keep
	}
	return reply, "", cost, true
}

func fileNonEmpty(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Size() > 0
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if de.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
