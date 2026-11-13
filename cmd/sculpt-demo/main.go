// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"sculpt/core/schedule"
	"sculpt/demo"
)

func main() {
	output := flag.String("output", "runs/demo", "demo output root")
	k := flag.Int("k", 2, "task-level recursion budget")
	mode := flag.String("mode", "both", "parallel | sequential | both")
	seedFlag := flag.Int("seed", 0, "stream-order / namespace seed; omit for seedless namespaces")
	flag.Parse()
	seedSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})
	if *k < 1 {
		fmt.Fprintln(os.Stderr, "--k must be positive")
		os.Exit(2)
	}
	root, err := filepath.Abs(*output)
	if err != nil {
		fatal(err)
	}
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		fatal(err)
	}
	prop := demo.NewProposer(adapter.Spec().Channels["proposer"])
	var modes []string
	switch *mode {
	case "both":
		modes = []string{"agg", "seq"}
	default:
		tok, err := schedule.ModeToken(*mode)
		if err != nil {
			fatal(err)
		}
		modes = []string{tok}
	}
	var opts []schedule.DomainOptions
	if seedSet {
		s := *seedFlag
		opts = append(opts, schedule.DomainOptions{Seed: &s})
	}
	result := schedule.RunDomain(
		"accounting",
		adapter.Tasks("all"),
		*k,
		adapter,
		prop,
		adapter.Spec().RunsRoot,
		filepath.Join(root, "library", fmt.Sprintf("k%d", *k)),
		modes,
		opts...,
	)
	result["execution_count"] = adapter.ExecutionCount
	result["editor_calls"] = len(prop.Calls())
	report := filepath.Join(root, "demo.json")
	b, _ := json.MarshalIndent(result, "", "  ")
	_ = os.WriteFile(report, append(b, '\n'), 0o644)
	fmt.Printf("Local demo: %v tasks completed; %v failed. %s\n", result["done"], result["failed"], report)
	if failed, _ := result["failed"].(int); failed > 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
