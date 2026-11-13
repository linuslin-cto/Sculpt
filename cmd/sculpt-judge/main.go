// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"sculpt/benchmarks/jobbench"
	"sculpt/benchmarks/lab"
	"sculpt/benchmarks/workbuddy"
	"sculpt/core/judge"
	"sculpt/core/schedule"
	"sculpt/core/spec"
	"sculpt/demo"
)

func main() {
	bench := flag.String("bench", "demo", "jb | lab | wb | demo")
	substrate := flag.String("substrate", "oc", "oc | cx")
	output := flag.String("output", "runs", "workspace used when constructing the adapter")
	k := flag.Int("k", 1, "scale of the sweep namespace")
	seedFlag := flag.Int("seed", 0, "stream-order seed; omit for seedless namespace")
	modesRaw := flag.String("modes", "parallel,sequential", "comma list: parallel | sequential")
	modeAlias := flag.String("mode", "", "alias for --modes")
	bases := flag.Bool("bases", false, "score reference executions into base_judged.json")
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
	ns := schedule.NS(adapter.Spec().Name, adapter.Substrate().Name(), "agg", *k, seed, "")
	nsDir := filepath.Join(adapter.Spec().RunsRoot, ns)
	if fi, err := os.Stat(nsDir); err != nil || !fi.IsDir() {
		fatal(fmt.Errorf("nothing to judge: %s does not exist — has the sweep %q run?", nsDir, ns))
	}

	if *bases {
		code := judgeBases(adapter, nsDir)
		os.Exit(code)
	}

	sels, err := judge.CollectSelections(nsDir, modes)
	if err != nil {
		fatal(err)
	}
	if len(sels) == 0 {
		labels := make([]string, len(modes))
		for i, m := range modes {
			labels[i] = schedule.ModeLabel(m)
		}
		fatal(fmt.Errorf("nothing to judge: no selections for modes %v under %s", labels, nsDir))
	}

	fmt.Printf("judging %d selection(s) under %s ...\n", len(sels), ns)
	rows := judge.Selections(adapter, sels)
	outPath := filepath.Join(nsDir, "judged.json")
	prev, err := judge.LoadRows(outPath)
	if err != nil {
		fatal(err)
	}
	merged := judge.Merge(prev, rows)
	if err := judge.Write(outPath, merged); err != nil {
		fatal(err)
	}

	graded := 0
	for _, r := range merged {
		if r.Graded() {
			graded++
		}
	}
	fmt.Printf("judged.json: %d row(s), %d graded, %d ungraded (judge failure or no pick — NOT zeros)\n",
		len(merged), graded, len(merged)-graded)
	fmt.Printf("wrote %s\n", outPath)

	allGraded := len(rows) > 0
	for _, r := range rows {
		if !r.Graded() {
			allGraded = false
			break
		}
	}
	if !allGraded {
		os.Exit(2)
	}
}

func judgeBases(adapter spec.BenchmarkAdapter, nsDir string) int {
	matches, _ := filepath.Glob(filepath.Join(nsDir, "*", "summary.json"))
	var rows []map[string]any
	legacy := 0
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var summary map[string]any
		if json.Unmarshal(b, &summary) != nil {
			continue
		}
		taskID, _ := summary["task_id"].(string)
		base, _ := summary["base"].([]any)
		for _, entry := range base {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			runDir, _ := m["run_dir"].(string)
			if runDir == "" {
				legacy++
				continue
			}
			label, _ := m["label"].(string)
			runID, _ := m["run_id"].(string)
			var score map[string]any
			fi, err := os.Stat(runDir)
			if err == nil && fi.IsDir() {
				got, err := adapter.Score(runDir, taskID)
				if err == nil && got != nil {
					np, okP := asIntPtr(got["n_passed"])
					nc, okC := asIntPtr(got["n_criteria"])
					if okP && okC && judge.HasGrade(np, nc) {
						score = got
					}
				} else if err != nil {
					fmt.Printf("  %-52s %s ERROR %v\n", trim(taskID, 52), label, err)
				}
			}
			shown := "-"
			if score != nil {
				shown = fmt.Sprintf("%v/%v", score["n_passed"], score["n_criteria"])
			}
			fmt.Printf("  %-52s %s %s\n", trim(taskID, 52), label, shown)
			rows = append(rows, map[string]any{
				"task": taskID, "label": label, "run_id": runID, "score": score,
			})
		}
	}
	if legacy > 0 {
		fmt.Printf("  %d reference execution(s) predate run-directory recording and cannot be scored; re-run those tasks to include them.\n", legacy)
	}
	if len(rows) == 0 {
		fmt.Printf("no reference executions recorded under %s; wrote nothing\n", nsDir)
		return 2
	}
	out := filepath.Join(nsDir, "base_judged.json")
	b, _ := json.MarshalIndent(rows, "", " ")
	_ = os.WriteFile(out, append(b, '\n'), 0o644)
	graded := 0
	for _, r := range rows {
		if r["score"] != nil {
			graded++
		}
	}
	fmt.Printf("wrote %s: %d row(s), %d graded\n", out, len(rows), graded)
	if graded == len(rows) {
		return 0
	}
	return 2
}

func asIntPtr(v any) (*int, bool) {
	switch x := v.(type) {
	case int:
		return &x, true
	case float64:
		n := int(x)
		return &n, true
	default:
		return nil, false
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
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
