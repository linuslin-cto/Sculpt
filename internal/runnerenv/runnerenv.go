// Copyright 2026 The Sculpt Authors.
// Licensed under the Apache License, Version 2.0

// Package runnerenv holds env-var names expected by optional third-party
// JobBench launch scripts. Sculpt itself uses SCULPT_*; these aliases are set
// only when spawning those external runners.
package runnerenv

const (
	// RenderDir is read by legacy jb_* runners for the staged harness tree.
	RenderDir = "HARNESS_RENDER_DIR"
	// CodexBin is read by the legacy Codex jb shell launcher.
	CodexBin = "HARNESS2_CODEX_BIN"
)
