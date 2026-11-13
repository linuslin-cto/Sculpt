// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package proposer defines improver session types and the Proposer interface.
package proposer

import (
	"fmt"
	"strings"

	"sculpt/core/spec"
)

// Error is a failed improver call that must not be swallowed into an empty string.
type Error struct {
	Message  string
	Attempts int
	CostUSD  float64
}

func (e *Error) Error() string { return e.Message }

// Reply is one improver session result.
type Reply struct {
	Label     string
	Text      string
	OK        bool
	CostUSD   float64
	WallS     float64
	Attempts  int
	Cached    bool
	Channel   string
	Error     string
	Audit     string
	Workspace string
}

// Caller runs agentic sessions on one channel.
type Caller interface {
	Spec() spec.ChannelSpec
	Call(mission, contextDir, label string, harvest string, maxAttempts int) (Reply, error)
	Calls() []Reply
}

// Base is a model-backed proposer (stub for future OpenCode/Claude CLI wiring).
type Base struct {
	ChannelSpec spec.ChannelSpec
	AuditDir    string
	Attempts    int
	calls       []Reply
}

// New constructs a Base proposer; empty model is rejected (matches Python).
func New(ch spec.ChannelSpec, auditDir string, attempts int) (*Base, error) {
	if strings.TrimSpace(ch.Model) == "" {
		return nil, fmt.Errorf("Set SCULPT_IMPROVER_MODEL before starting model sessions")
	}
	if attempts <= 0 {
		attempts = 2
	}
	return &Base{ChannelSpec: ch, AuditDir: auditDir, Attempts: attempts}, nil
}

// Spec returns the channel spec.
func (b *Base) Spec() spec.ChannelSpec { return b.ChannelSpec }

// Calls returns recorded replies.
func (b *Base) Calls() []Reply { return append([]Reply(nil), b.calls...) }

// Call is not implemented for the live model channel in this scaffold.
func (b *Base) Call(mission, contextDir, label string, harvest string, maxAttempts int) (Reply, error) {
	return Reply{}, &Error{Message: "live proposer not wired; use demo.Proposer for offline runs"}
}
