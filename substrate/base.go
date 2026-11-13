// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package substrate

import (
	"sculpt/core/tree"
)

// Install is a materialised harness ready for a launcher.
type Install struct {
	StagingDir  string
	Fingerprint string
	Extras      map[string]string
}

// RunRequest is one execution request handed to a benchmark launcher.
type RunRequest struct {
	TaskRef string
	RunIDs  []string
	Timeout int
}

// Launcher runs rollouts and returns one run directory per run id.
type Launcher func(req RunRequest, install Install) ([]string, error)

// Substrate is the full agent-framework interface.
type Substrate interface {
	Name() string
	Surface() tree.SurfaceSpec
	Materialise(treeDir string, installMode string, ctx map[string]string) (Install, error)
	Execute(taskRef string, install Install, runIDs []string, timeout int) ([]string, error)
	ReadTrajectory(streamPath string, obsTrunc, maxChars, argsTrunc int) (string, error)
	Preflight() error
}
