// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sculpt/benchmarks/jobbench"
	"sculpt/benchmarks/lab"
	"sculpt/benchmarks/workbuddy"
	"sculpt/core/schedule"
	"sculpt/core/spec"
	"sculpt/demo"
)

func main() {
	bench := flag.String("bench", "demo", "jb | lab | wb | demo")
	substrate := flag.String("substrate", "oc", "oc | cx")
	output := flag.String("output", "runs", "workspace / runs root parent for adapter construction")
	k := flag.Int("k", 1, "scale: R=4k rollouts, P=3k improver sessions per task")
	seedFlag := flag.Int("seed", 0, "stream-order seed; omit for sorted task order")
	modesRaw := flag.String("modes", "parallel,sequential", "comma list: parallel | sequential (agg | seq)")
	modeAlias := flag.String("mode", "", "alias for --modes (single mode)")
	split := flag.String("split", "all", "all | train | test")
	onlyDomains := flag.String("only-domains", "", "comma list restricting the sweep to these domains")
	maxTasks := flag.Int("max-tasks", 0, "per-domain cap on the stream; 0 means no cap")
	dryRun := flag.Bool("dry-run", false, "print stream order and namespaces, then exit")
	flag.Parse()

	seedSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})

	if *k < 1 {
		fatal(fmt.Errorf("--k must be positive"))
	}
	modesIn := *modesRaw
	if *modeAlias != "" {
		modesIn = *modeAlias
	}
	modes, err := schedule.ParseModes(modesIn)
	if err != nil {
		fatal(err)
	}

	root, err := filepath.Abs(*output)
	if err != nil {
		fatal(err)
	}
	adapter, err := makeAdapter(*bench, *substrate, root)
	if err != nil {
		fatal(err)
	}

	var seed *int
	if seedSet {
		s := *seedFlag
		seed = &s
	}
	opts := schedule.TaskStreamOpts{Split: *split, Seed: seed}
	if *onlyDomains != "" {
		opts.OnlyDomains = csv(*onlyDomains)
	}
	if *maxTasks > 0 {
		m := *maxTasks
		opts.MaxTasks = &m
	}

	if !*dryRun {
		fmt.Fprintln(os.Stderr, `error: live sweeps are not wired yet (model-backed jb/lab/wb).

  Preview the seeded stream (no rollouts):
    sculpt --bench demo --output runs/demo --k 1 --seed 7 --dry-run

  Offline Propose–Probe–Compose demo:
    sculpt-demo --output runs/demo --k 2

  Readiness (dataset / binaries):
    sculpt-check --bench demo --output runs/check-demo`)
		os.Exit(2)
	}

	byDomain, stream, err := schedule.TaskStream(adapter, opts)
	if err != nil {
		fatal(err)
	}
	printStream(adapter, *k, seed, modes, *split, byDomain, stream)
	if len(stream) == 0 {
		os.Exit(1)
	}
}

func printStream(
	adapter spec.BenchmarkAdapter,
	k int,
	seed *int,
	modes []string,
	split string,
	byDomain map[string][]string,
	stream [][2]string,
) {
	seedStr := "None"
	if seed != nil {
		seedStr = fmt.Sprintf("%d", *seed)
	}
	labels := make([]string, len(modes))
	for i, m := range modes {
		labels[i] = schedule.ModeLabel(m)
	}
	fmt.Printf("# %s-%s split=%s seed=%s k=%d modes=%s\n",
		adapter.Spec().Name, adapter.Substrate().Name(), split, seedStr, k, strings.Join(labels, ","))
	for _, m := range modes {
		ns := schedule.NS(adapter.Spec().Name, adapter.Substrate().Name(), m, k, seed, "")
		fmt.Printf("#   namespace[%s]: %s\n", schedule.ModeLabel(m), ns)
	}
	summaryNS := schedule.NS(adapter.Spec().Name, adapter.Substrate().Name(), "agg", k, seed, "")
	runsRoot := adapter.Spec().RunsRoot
	fmt.Printf("#   summaries under: %s/%s\n", runsRoot, summaryNS)
	libRoot := libraryRoot(adapter, k, seed)
	fmt.Printf("#   library root:    %s\n", libRoot)
	for i, pair := range stream {
		fmt.Printf("%4d  %-40s %s\n", i+1, pair[0], pair[1])
	}
	fmt.Println("# domains of this split, with task counts (the legal --only-domains names):")
	for _, d := range schedule.DefaultReferenceDomainOrder(byDomain) {
		fmt.Printf("%4d  %s\n", len(byDomain[d]), d)
	}
	fmt.Printf("# %d tasks over %d domains (domains parallel, tasks within a domain sequential)\n",
		len(stream), len(byDomain))
}

func libraryRoot(adapter spec.BenchmarkAdapter, k int, seed *int) string {
	variant := adapter.Substrate().Name()
	if seed != nil {
		variant += fmt.Sprintf("-s%d", *seed)
	}
	return filepath.Join(adapter.Spec().RunsRoot, "library",
		fmt.Sprintf("%s-k%d-%s", adapter.Spec().Name, k, variant))
}

func makeAdapter(bench, substrate, root string) (spec.BenchmarkAdapter, error) {
	switch bench {
	case "demo":
		return demo.NewAdapter(root)
	case "jb":
		return jobbench.New(root, substrate)
	case "lab":
		return lab.New(root, substrate)
	case "wb":
		return workbuddy.New(root, substrate)
	default:
		return nil, fmt.Errorf("unknown --bench %q", bench)
	}
}

func csv(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
