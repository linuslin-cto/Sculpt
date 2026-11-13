// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"sculpt/benchmarks/jobbench"
	"sculpt/benchmarks/lab"
	"sculpt/benchmarks/workbuddy"
	"sculpt/demo"
	"sculpt/internal/preflight"
)

func main() {
	bench := flag.String("bench", "demo", "jb | lab | wb | demo")
	substrate := flag.String("substrate", "oc", "oc | cx")
	output := flag.String("output", "runs/check", "workspace for demo / jb store root")
	live := flag.Bool("live", false, "also check OpenCode/Codex binaries")
	flag.Parse()

	root, err := filepath.Abs(*output)
	if err != nil {
		fatal(err)
	}

	switch *bench {
	case "demo":
		adapter, err := demo.NewAdapter(root)
		if err != nil {
			fatal(err)
		}
		var errors []string
		if *live {
			errors = preflight.Check(adapter, true)
		} else {
			errors = preflight.DemoReady(adapter)
		}
		report(adapter.Spec().Name, adapter.Substrate().Name(), len(adapter.Tasks("all")), errors)
	case "jb":
		a, err := jobbench.New(root, *substrate)
		if err != nil {
			fatal(err)
		}
		errors := preflight.BenchmarkReady("jb", *substrate, root)
		if *live {
			errors = append(errors, preflight.Check(a, true)...)
		}
		report(a.Spec().Name, a.Substrate().Name(), len(a.Tasks("all")), uniq(errors))
	case "lab":
		a, err := lab.New(root, *substrate)
		if err != nil {
			fatal(err)
		}
		errors := preflight.BenchmarkReady("lab", *substrate, root)
		report(a.Spec().Name, a.Substrate().Name(), len(a.Tasks("all")), uniq(errors))
	case "wb":
		a, err := workbuddy.New(root, *substrate)
		if err != nil {
			fatal(err)
		}
		errors := preflight.BenchmarkReady("wb", *substrate, root)
		report(a.Spec().Name, a.Substrate().Name(), len(a.Tasks("all")), uniq(errors))
	default:
		fmt.Fprintf(os.Stderr, "unknown --bench %q\n", *bench)
		os.Exit(2)
	}
}

func report(name, sub string, n int, errors []string) {
	for _, e := range errors {
		fmt.Println("FAIL:", e)
	}
	if len(errors) > 0 {
		fmt.Printf("Setup incomplete: %d issue(s). %s/%s; discovered %d tasks.\n", len(errors), name, sub, n)
		os.Exit(1)
	}
	fmt.Printf("Ready: %s/%s; %d tasks. Model access is not exercised.\n", name, sub, n)
}

func uniq(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
