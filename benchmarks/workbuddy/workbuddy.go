// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package workbuddy is a stub for the WorkBuddy BenchmarkAdapter port.
package workbuddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sculpt/benchmarks/tasks"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/internal/config"
	"sculpt/substrate"
)

// Name is the short bench token.
const Name = "wb"

// Adapter is a placeholder until the full WorkBuddy runner is wired.
type Adapter struct {
	RunsRoot      string
	Subset        string // office | web | code
	TasksRoot     string // override dataset tasks root
	SplitFile     string
	SubstrateName string
	sub           spec.SubstrateLike
	specVal       spec.BenchSpec

	mu       sync.Mutex
	cache    map[string]string
	baseOnce sync.Once
	basePath string
	baseErr  error
}

// New constructs a WorkBuddy stub adapter (default subset=office).
func New(runsRoot, substrateName string) (*Adapter, error) {
	return NewSubset(runsRoot, substrateName, "office")
}

// NewSubset constructs a WorkBuddy stub for one dataset subset.
func NewSubset(runsRoot, substrateName, subset string) (*Adapter, error) {
	var sub spec.SubstrateLike
	switch substrateName {
	case "oc", "":
		sub = substrate.Opencode{}
		substrateName = "oc"
	case "cx":
		sub = substrate.Codex{}
	default:
		return nil, fmt.Errorf("unknown substrate %q", substrateName)
	}
	if subset == "" {
		subset = "office"
	}
	ch, err := spec.NewChannelSpec("opencode", "local/stub", "high", 120, 3600)
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		RunsRoot:      runsRoot,
		Subset:        subset,
		TasksRoot:     filepath.Join(config.WbRepo(), "datasets", "wb-bench-"+subset+"-v1.0", "tasks"),
		SubstrateName: substrateName,
		sub:           sub,
	}
	a.specVal = spec.BenchSpec{
		Name:        "wb-" + substrateName,
		RunsRoot:    runsRoot,
		InstallMode: "workspace_agents_md",
		Prompts:     spec.BenchPrompts{Work: "workbuddy office tasks", DomainWord: "role", PromptPack: "adapt", SelectionConformance: "full", DeliverableWord: "deliverable"},
		Channels:    map[string]spec.ChannelSpec{"proposer": ch, "selector": ch},
		ObsTrunc:    50000,
	}
	if err := a.specVal.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adapter) Spec() spec.BenchSpec          { return a.specVal }
func (a *Adapter) Substrate() spec.SubstrateLike { return a.sub }

func (a *Adapter) load() (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cache != nil {
		return a.cache, nil
	}
	m, err := tasks.Discover("wb", a.Subset, a.TasksRoot)
	if err != nil {
		return nil, err
	}
	a.cache = m
	return m, nil
}

func (a *Adapter) Tasks(split string) []string {
	m, err := a.load()
	if err != nil {
		return nil
	}
	ids, err := tasks.Select(m, split, a.SplitFile, "wb-"+a.Subset)
	if err != nil {
		return nil
	}
	return ids
}

func (a *Adapter) DomainOf(taskID string) string {
	m, err := a.load()
	if err != nil {
		return "unknown"
	}
	if d, ok := m[taskID]; ok {
		return d
	}
	return "unknown"
}

func (a *Adapter) TaskPrompt(taskID string) string {
	p := filepath.Join(a.TasksRoot, taskID, "instruction.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *Adapter) TaskFiles(taskID string) string {
	d := filepath.Join(a.TasksRoot, taskID)
	if fi, err := os.Stat(d); err == nil && fi.IsDir() {
		return d
	}
	return ""
}

func (a *Adapter) Execute(taskID, harnessDir, runID string, timeout int) (string, error) {
	return "", fmt.Errorf("workbuddy: Execute not wired")
}

// Delivered reports whether the rollout produced artifact bodies (Python: body_chars >= 200).
// Until wb_render is ported, this approximates by summing non-agent/non-verifier file sizes.
func (a *Adapter) Delivered(runDir string) bool {
	return bodyCharsApprox(runDir) >= 200
}

// TrajectoryPath is the capture stream under run_dir/agent/.
func (a *Adapter) TrajectoryPath(runDir string) string {
	stream := "oc-output.txt"
	if a.SubstrateName == "cx" {
		stream = "codex-output.jsonl"
	}
	return filepath.Join(runDir, "agent", stream)
}

func (a *Adapter) Trajectory(runDir string, obsTrunc int) string {
	f := a.TrajectoryPath(runDir)
	if fi, err := os.Stat(f); err != nil || fi.IsDir() {
		return "(no trajectory)"
	}
	var (
		text string
		err  error
	)
	const trajMax = 2_000_000
	if a.SubstrateName == "cx" {
		text, err = (substrate.Codex{}).ReadTrajectory(f, obsTrunc, trajMax, 2000)
	} else {
		text, err = (substrate.Opencode{}).ReadTrajectory(f, obsTrunc, trajMax, 400)
	}
	if err != nil {
		return "(no trajectory)"
	}
	return text
}

func (a *Adapter) DeliverableFiles(runDir string) map[string]string { return nil }
func (a *Adapter) DeliverableText(runDir string) string             { return "" }

// BaseHarness materialises H0 once per adapter (Python WbAdapter.base_harness).
func (a *Adapter) BaseHarness() string {
	a.baseOnce.Do(func() {
		root := a.specVal.RunsRoot
		if root == "" {
			root = a.RunsRoot
		}
		dir := filepath.Join(root, fmt.Sprintf("_base_harness_%s_%d", a.SubstrateName, os.Getpid()))
		var t tree.Tree
		var surface tree.SurfaceSpec
		if a.SubstrateName == "cx" {
			surface = substrate.Codex{}.Surface()
			t = tree.Tree{
				"AGENTS.md": "Follow the task instructions and complete the requested work in the " +
					"workspace, writing every deliverable where the instructions say.",
			}
		} else {
			surface = substrate.Opencode{}.Surface()
			t = tree.Tree{
				"systemprompt.md":                     "",
				"guardrails.md":                       "",
				"LongTermMEMORY.md":                   "",
				"tool_descriptions/tool_guidance.md": "",
			}
		}
		if _, err := tree.WriteTree(surface, t, dir, true); err != nil {
			a.baseErr = err
			return
		}
		a.basePath = dir
	})
	if a.baseErr != nil {
		return ""
	}
	return a.basePath
}

func (a *Adapter) ValidateTree(t tree.Tree) error { return nil }
func (a *Adapter) Score(runDir, taskID string) (map[string]any, error) {
	_ = taskID
	if !a.Delivered(runDir) && a.killed(runDir) {
		return nil, fmt.Errorf("workbuddy: rollout killed before deliverable")
	}
	p := filepath.Join(runDir, "verifier", "score.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("workbuddy: verifier/score.json missing at %s", p)
	}
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, fmt.Errorf("workbuddy: unreadable score.json: %w", err)
	}
	sc, ok := scoreFromPayload(payload)
	if !ok {
		return nil, fmt.Errorf("workbuddy: score payload has no usable score")
	}
	nPassed := int(sc*100 + 0.5)
	return map[string]any{
		"n_passed":   nPassed,
		"n_criteria": 100,
		"all_pass":   sc >= 1.0,
		"raw_score":  sc,
	}, nil
}

func (a *Adapter) killed(runDir string) bool {
	raw, err := os.ReadFile(a.TrajectoryPath(runDir))
	if err != nil {
		return false
	}
	lines := strings.Split(string(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		ln := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(ln, "{") {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(ln), &ev) != nil {
			return false
		}
		if et, _ := ev["type"].(string); et == "error" {
			return true
		}
		if _, ok := ev["error"].(map[string]any); ok {
			return true
		}
		return false
	}
	return false
}

func bodyCharsApprox(runDir string) int {
	total := 0
	_ = filepath.WalkDir(runDir, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(runDir, path)
		rel = filepath.ToSlash(rel)
		if de.IsDir() {
			if rel == "agent" || rel == "verifier" || strings.HasPrefix(rel, "agent/") || strings.HasPrefix(rel, "verifier/") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(rel, "agent/") || strings.HasPrefix(rel, "verifier/") {
			return nil
		}
		fi, err := de.Info()
		if err != nil {
			return nil
		}
		total += int(fi.Size())
		return nil
	})
	return total
}

// scoreFromPayload mirrors Python workbuddy.score_from_payload:
// reward > overall > test_pass_rate > tests_passed/tests_total; values outside [0,1] rejected.
func scoreFromPayload(payload map[string]any) (float64, bool) {
	for _, key := range []string{"reward", "overall", "test_pass_rate"} {
		if v, ok := unitFloat(payload[key]); ok {
			return v, true
		}
	}
	passed, okP := asInt(payload["tests_passed"])
	total, okT := asInt(payload["tests_total"])
	if okP && okT && total > 0 {
		sc := float64(passed) / float64(total)
		if sc < 0 {
			sc = 0
		}
		if sc > 1 {
			sc = 1
		}
		return sc, true
	}
	return 0, false
}

func unitFloat(v any) (float64, bool) {
	f, ok := asFloat(v)
	if !ok || f < 0 || f > 1 {
		return 0, false
	}
	return f, true
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

// ReadyErrors lists what must be ported before real WorkBuddy runs.
func ReadyErrors(a *Adapter) []string {
	var out []string
	if a == nil {
		return []string{"workbuddy adapter is nil"}
	}
	if a.BaseHarness() == "" {
		out = append(out, "workbuddy base harness path not configured")
	}
	if _, err := a.load(); err != nil {
		out = append(out, "workbuddy task discovery: "+err.Error())
	} else if len(a.Tasks("all")) == 0 {
		out = append(out, "workbuddy task discovery returned zero tasks")
	}
	return out
}
