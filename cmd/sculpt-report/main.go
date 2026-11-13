// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sculpt/benchmarks/jobbench"
	"sculpt/benchmarks/lab"
	"sculpt/benchmarks/workbuddy"
	"sculpt/core/judge"
	"sculpt/core/report"
	"sculpt/core/schedule"
	"sculpt/core/spec"
	"sculpt/demo"
)

func main() {
	bench := flag.String("bench", "demo", "jb | lab | wb | demo")
	substrate := flag.String("substrate", "oc", "oc | cx")
	output := flag.String("output", "runs", "workspace used when constructing the adapter")
	kRaw := flag.String("k", "1", "comma list of scales, e.g. 1,2")
	seedFlag := flag.Int("seed", 0, "stream-order seed; omit for seedless namespace")
	modesRaw := flag.String("modes", "parallel,sequential", "comma list: parallel | sequential")
	modeAlias := flag.String("mode", "", "alias for --modes")
	flag.Parse()

	seedSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})

	modesIn := *modesRaw
	if *modeAlias != "" {
		modesIn = *modeAlias
	}
	modes, err := schedule.ParseModes(modesIn)
	if err != nil {
		fatal(err)
	}
	ks, err := parseKs(*kRaw)
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

	var rows []judge.Scored
	fellBack := map[report.ModeK]int{}
	usd := map[report.ModeK]float64{}
	for _, k := range ks {
		ns := schedule.NS(adapter.Spec().Name, adapter.Substrate().Name(), "agg", k, seed, "")
		nsDir := filepath.Join(adapter.Spec().RunsRoot, ns)
		judged := filepath.Join(nsDir, "judged.json")
		got, err := judge.LoadRows(judged)
		if err != nil {
			fatal(err)
		}
		if len(got) == 0 {
			fmt.Printf("  ! no judged rows at %s — run judge for k=%d first\n", judged, k)
			continue
		}
		for _, r := range got {
			for _, m := range modes {
				if r.Mode == m {
					rows = append(rows, r)
					break
				}
			}
		}
		fb, sp := spendAndFallbacks(nsDir, modes, k)
		for key, v := range fb {
			fellBack[key] += v
		}
		for key, v := range sp {
			usd[key] += v
		}
	}
	if len(rows) == 0 {
		fatal(fmt.Errorf("nothing to report"))
	}

	title := adapter.Spec().Name + "-" + adapter.Substrate().Name()
	if seed != nil {
		title += fmt.Sprintf(" seed=%d", *seed)
	}
	labels := make([]string, len(modes))
	for i, m := range modes {
		labels[i] = schedule.ModeLabel(m)
	}
	title += " modes=" + strings.Join(labels, ",")

	summaries := report.SummariseAll(rows, fellBack, usd)
	fmt.Println(report.Render(summaries, title, adapter.Spec().Name))
	spend := 0.0
	for _, s := range summaries {
		spend += s.USD
	}
	if spend > 0 {
		fmt.Printf("\n  improver (editors, composer, selection, reflection) spend across reported modes: $%.2f\n", spend)
	}
}

func spendAndFallbacks(nsDir string, modes []string, k int) (map[report.ModeK]int, map[report.ModeK]float64) {
	fell := map[report.ModeK]int{}
	usd := map[report.ModeK]float64{}
	matches, _ := filepath.Glob(filepath.Join(nsDir, "*", "summary.json"))
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var summary map[string]any
		if json.Unmarshal(b, &summary) != nil {
			continue
		}
		modesMap, _ := summary["modes"].(map[string]any)
		sels, _ := summary["selections"].(map[string]any)
		for _, mode := range modes {
			key := report.ModeK{Mode: mode, K: k}
			if rec, ok := modesMap[mode].(map[string]any); ok {
				if fb, ok := rec["fell_back"].([]any); ok {
					fell[key] += len(fb)
				}
				if stages, ok := rec["stages"].([]any); ok {
					for _, st := range stages {
						if m, ok := st.(map[string]any); ok {
							usd[key] += asFloat(m["usd"])
						}
					}
				}
			}
			if sel, ok := sels[mode].(map[string]any); ok {
				usd[key] += asFloat(sel["usd"])
			}
		}
	}
	return fell, usd
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	default:
		return 0
	}
}

func parseKs(raw string) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("--k takes a comma list of ints, got %q", raw)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--k named no scale")
	}
	return out, nil
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

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
