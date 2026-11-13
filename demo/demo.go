// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package demo provides a deterministic local adapter for the adaptation workflow.
package demo

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sculpt/core/proposer"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/internal/config"
	"sculpt/substrate"
)

// BaseScript is the buggy positive-only summarizer (matches Python BASE_SCRIPT).
const BaseScript = `import csv
import sys
from decimal import Decimal

with open(sys.argv[1]) as source:
    amounts = [Decimal(row['amount']) for row in csv.DictReader(source)]
total = sum((amount for amount in amounts if amount > 0), Decimal(0))
print(f'Total: {total:.2f}')
`

// Adapter is a local benchmark adapter with no model credentials.
type Adapter struct {
	Root           string
	substrate      substrate.Opencode
	bench          spec.BenchSpec
	ExecutionCount int
}

// NewAdapter creates the demo adapter under root.
func NewAdapter(root string) (*Adapter, error) {
	ch, err := spec.NewChannelSpec("opencode", "local/deterministic", "high", 120, 3600)
	if err != nil {
		return nil, err
	}
	a := &Adapter{Root: root, substrate: substrate.Opencode{}}
	a.bench = spec.BenchSpec{
		Name:        "demo",
		RunsRoot:    filepath.Join(root, "runs"),
		InstallMode: "workspace_config",
		Prompts:     spec.BenchPrompts{Work: "reconcile signed amounts in a transaction ledger", DomainWord: "domain", DeliverableWord: "deliverable", SelectionConformance: "full", PromptPack: "adapt"},
		Channels:    map[string]spec.ChannelSpec{"proposer": ch, "selector": ch},
		LeakGlobs:   []string{"reference_answer.txt"},
		ObsTrunc:    50000,
	}
	if err := a.bench.Validate(); err != nil {
		return nil, err
	}
	base := tree.Tree{}
	for _, name := range a.substrate.Surface().AlwaysOn {
		base[name] = ""
	}
	base["systemprompt.md"] = "Reconcile the ledger using python scripts/summarize.py.\n"
	base["scripts/summarize.py"] = BaseScript
	if _, err := tree.WriteTree(a.substrate.Surface(), base, a.BaseHarness(), false); err != nil {
		return nil, err
	}
	for _, item := range []struct {
		ID      string
		Amounts string
	}{
		{"ledger-1", "100.25\n-20.10\n5.00\n"},
		{"ledger-2", "35.50\n-10.25\n-5.00\n"},
	} {
		src := a.TaskFiles(item.ID)
		if err := os.MkdirAll(src, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(src, "ledger.csv"), []byte("amount\n"+item.Amounts), 0o644); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (a *Adapter) Spec() spec.BenchSpec          { return a.bench }
func (a *Adapter) Substrate() spec.SubstrateLike { return a.substrate }
func (a *Adapter) Tasks(split string) []string   { return []string{"ledger-1", "ledger-2"} }
func (a *Adapter) DomainOf(taskID string) string { return "accounting" }
func (a *Adapter) TaskPrompt(taskID string) string {
	return "Sum every signed amount in ledger.csv. Return Total: with two decimal places."
}
func (a *Adapter) TaskFiles(taskID string) string {
	return filepath.Join(a.Root, "sources", taskID)
}
func (a *Adapter) BaseHarness() string { return filepath.Join(a.Root, "base") }

func (a *Adapter) ValidateTree(t tree.Tree) error {
	if _, ok := t["scripts/summarize.py"]; !ok {
		return fmt.Errorf("The local executor requires scripts/summarize.py")
	}
	return nil
}

func (a *Adapter) Execute(taskID, harnessDir, runID string, timeout int) (string, error) {
	a.ExecutionCount++
	runDir := filepath.Join(a.Root, "executions", filepath.FromSlash(runID))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}
	base := a.BaseHarness()
	if harnessDir == "" {
		harnessDir = base
	}
	script := filepath.Join(harnessDir, "scripts", "summarize.py")
	ledger := filepath.Join(a.TaskFiles(taskID), "ledger.csv")
	if timeout <= 0 {
		timeout = 10
	}
	out, err := runPython(script, ledger)
	if err != nil {
		return runDir, err
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	_ = os.WriteFile(filepath.Join(runDir, "answer.txt"), []byte(text), 0o644)
	_ = os.WriteFile(filepath.Join(runDir, "trajectory.md"),
		[]byte("[ACTION] Run scripts/summarize.py on ledger.csv\n[OBSERVATION]\n"+text), 0o644)
	return runDir, nil
}

func (a *Adapter) Delivered(runDir string) bool {
	_, err := os.Stat(filepath.Join(runDir, "answer.txt"))
	return err == nil
}

func (a *Adapter) Trajectory(runDir string, obsTrunc int) string {
	b, err := os.ReadFile(filepath.Join(runDir, "trajectory.md"))
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *Adapter) DeliverableText(runDir string) string {
	b, _ := os.ReadFile(filepath.Join(runDir, "answer.txt"))
	return string(b)
}

func (a *Adapter) DeliverableFiles(runDir string) map[string]string {
	return map[string]string{"answer.txt": a.DeliverableText(runDir)}
}

func (a *Adapter) Score(runDir, taskID string) (map[string]any, error) {
	return nil, nil
}

// Proposer applies scripted edits and selection with no model service.
type Proposer struct {
	channel spec.ChannelSpec
	calls   []proposer.Reply
}

// NewProposer constructs the deterministic demo proposer.
func NewProposer(ch spec.ChannelSpec) *Proposer {
	return &Proposer{channel: ch}
}

func (p *Proposer) Spec() spec.ChannelSpec { return p.channel }
func (p *Proposer) Calls() []proposer.Reply {
	return append([]proposer.Reply(nil), p.calls...)
}

func (p *Proposer) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	_ = mission
	_ = harvest
	_ = maxAttempts
	var workspace string
	var text string
	ctx := contextDir
	switch {
	case strings.HasPrefix(label, "select."):
		expected, err := signedTotal(filepath.Join(ctx, "source", "ledger.csv"))
		if err != nil {
			return proposer.Reply{}, err
		}
		want := fmt.Sprintf("Total: %.2f", expected)
		ents, _ := os.ReadDir(filepath.Join(ctx, "candidates"))
		chosen := ""
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			b, _ := os.ReadFile(filepath.Join(ctx, "candidates", e.Name()))
			if strings.Contains(string(b), want) {
				chosen = strings.TrimSuffix(e.Name(), ".md")
				break
			}
		}
		if chosen == "" {
			return proposer.Reply{}, &proposer.Error{Message: "no matching candidate", Attempts: 1}
		}
		text = fmt.Sprintf("<why>The signed total matches the source ledger.</why>\n```choice\n%s\n```", chosen)
	case strings.HasPrefix(label, "experience."):
		cur, _ := os.ReadFile(filepath.Join(ctx, "harness", "scripts", "summarize.py"))
		base, _ := os.ReadFile(filepath.Join(ctx, "base_harness", "scripts", "summarize.py"))
		changed := string(cur) != string(base)
		edits := "<edits>\ncomponent: scripts/summarize.py\ncandidates: inv1\n" +
			"change: dropped the positive-only filter from the amount sum\n" +
			"observed: the probe total included the credits the base run omitted\n" +
			fmt.Sprintf("verdict: %s\n</edits>\n\n", map[bool]string{true: "helped", false: "no observed effect"}[changed])
		if changed {
			text = edits + "<experience>\ncue: When a ledger includes credits and debits\n" +
				"lesson: Include both signs when summing in scripts/summarize.py.\n" +
				"evidence: The scripts/summarize.py row -- removing the positive-only filter " +
				"changed the probe total to include the credits omitted by the base trajectory.\n" +
				"confidence: high for this observed arithmetic contrast\n</experience>"
		} else {
			text = edits + "NO_ENTRY"
		}
	default:
		workspace = filepath.Join(ctx, "harness")
		script := filepath.Join(workspace, "scripts", "summarize.py")
		b, err := os.ReadFile(script)
		if err != nil {
			return proposer.Reply{}, err
		}
		fixed := strings.ReplaceAll(string(b), " if amount > 0", "")
		if err := os.WriteFile(script, []byte(fixed), 0o644); err != nil {
			return proposer.Reply{}, err
		}
		text = "<notes>Keep signed amounts: the base total omitted credits in the source ledger.</notes>"
	}
	reply := proposer.Reply{Label: label, Text: text, OK: true, Attempts: 1, Workspace: workspace}
	p.calls = append(p.calls, reply)
	return reply, nil
}

func runPython(script, ledger string) ([]byte, error) {
	for _, bin := range []string{"python", "python3", "py"} {
		args := []string{script, ledger}
		if bin == "py" {
			args = []string{"-3", script, ledger}
		}
		cmd := exec.Command(bin, args...)
		cmd.Env = config.ScrubEnv(nil)
		out, err := cmd.Output()
		if err == nil {
			return out, nil
		}
	}
	return nil, fmt.Errorf("python interpreter not found or script failed: %s", script)
}

func signedTotal(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return 0, err
	}
	sum := 0.0
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 {
			continue
		}
		var v float64
		if _, err := fmt.Sscanf(strings.TrimSpace(row[0]), "%f", &v); err != nil {
			return 0, err
		}
		sum += v
	}
	return sum, nil
}
