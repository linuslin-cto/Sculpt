// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package preflight checks runtime readiness before adaptation runs.
package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"sculpt/benchmarks/jobbench"
	"sculpt/benchmarks/lab"
	"sculpt/benchmarks/workbuddy"
	"sculpt/core/spec"
	"sculpt/substrate"
)

// Check reports readiness issues for an adapter.
// live=true also verifies OpenCode/Codex binaries (model-backed runs).
func Check(adapter spec.BenchmarkAdapter, live bool) []string {
	var errors []string

	if _, err := exec.LookPath("python"); err != nil {
		if _, err3 := exec.LookPath("python3"); err3 != nil {
			if _, errPy := exec.LookPath("py"); errPy != nil {
				errors = append(errors, "python interpreter not on PATH (needed for demo scripts and harness validation)")
			}
		}
	}

	if live {
		switch adapter.Substrate().Name() {
		case "oc":
			if err := (substrate.Opencode{}).Preflight(); err != nil {
				errors = append(errors, err.Error())
			}
		case "cx":
			if err := (substrate.Codex{}).Preflight(); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if tasks := adapter.Tasks("all"); len(tasks) == 0 {
		errors = append(errors, "adapter returned zero tasks")
	}

	base := adapter.BaseHarness()
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		errors = append(errors, fmt.Sprintf("base harness missing: %s", base))
	}

	if goos := runtime.GOOS; goos == "" {
		errors = append(errors, "unable to detect GOOS")
	}

	return errors
}

// DemoReady runs offline checks that need no cloud credentials / OpenCode checkout.
func DemoReady(adapter spec.BenchmarkAdapter) []string {
	return Check(adapter, false)
}

// BenchmarkReady constructs the bench adapter stub and returns ReadyErrors (+ python check).
func BenchmarkReady(bench, substrateName, runsRoot string) []string {
	var errors []string
	if substrateName != "oc" && substrateName != "cx" {
		return []string{fmt.Sprintf("unknown substrate %q", substrateName)}
	}
	if _, err := exec.LookPath("python"); err != nil {
		if _, err3 := exec.LookPath("python3"); err3 != nil {
			if _, errPy := exec.LookPath("py"); errPy != nil {
				errors = append(errors, "python interpreter not on PATH")
			}
		}
	}
	switch bench {
	case "jb":
		a, err := jobbench.New(runsRoot, substrateName)
		if err != nil {
			return append(errors, err.Error())
		}
		errors = append(errors, jobbench.ReadyErrors(a)...)
	case "lab":
		a, err := lab.New(runsRoot, substrateName)
		if err != nil {
			return append(errors, err.Error())
		}
		errors = append(errors, lab.ReadyErrors(a)...)
	case "wb":
		a, err := workbuddy.New(runsRoot, substrateName)
		if err != nil {
			return append(errors, err.Error())
		}
		errors = append(errors, workbuddy.ReadyErrors(a)...)
	default:
		return []string{fmt.Sprintf("unknown bench %q", bench)}
	}
	return errors
}

// BenchmarkStub is kept for callers; prefer BenchmarkReady.
func BenchmarkStub(bench, substrateName string) []string {
	return BenchmarkReady(bench, substrateName, "")
}
