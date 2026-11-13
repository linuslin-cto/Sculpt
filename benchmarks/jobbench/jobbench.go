// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package jobbench ports the JobBench BenchmarkAdapter (discovery + execute stage).
package jobbench

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"sculpt/benchmarks/tasks"
	"sculpt/core/runmark"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/internal/config"
	"sculpt/substrate"
)

// Name is the short bench token.
const Name = "jb"

// Adapter implements BenchmarkAdapter for JobBench.
type Adapter struct {
	RunsRoot        string
	Dataset         string // job-bench-eval/dataset/main
	Store           string // runs/jb/dataset
	SplitFile       string
	SubstrateName   string
	Timeout         int
	BaseHarnessPath string
	Launcher        substrate.Launcher
	sub             substrate.Substrate
	specVal         spec.BenchSpec

	mu    sync.Mutex
	cache map[string]string
}

// New constructs a JobBench adapter.
func New(runsRoot, substrateName string) (*Adapter, error) {
	switch substrateName {
	case "oc", "":
		substrateName = "oc"
	case "cx":
	default:
		return nil, fmt.Errorf("unknown substrate %q", substrateName)
	}
	ch, err := spec.NewChannelSpec("opencode", "local/stub", "high", 120, 3600)
	if err != nil {
		return nil, err
	}
	store := filepath.Join(config.RunsDir, "jb", "dataset")
	if runsRoot != "" {
		store = filepath.Join(runsRoot, "jb", "dataset")
	}
	installMode := "workspace_agents_md"
	if substrateName == "cx" {
		installMode = "codex_root"
	}
	base := config.JbBaseHarness(substrateName)
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		base = ""
	}
	a := &Adapter{
		RunsRoot:        runsRoot,
		Dataset:         filepath.Join(config.JbRepo(), "dataset", "main"),
		Store:           store,
		SubstrateName:   substrateName,
		Timeout:         3600,
		BaseHarnessPath: base,
	}
	a.Launcher = a.defaultLaunch
	a.bindSubstrate()
	a.specVal = spec.BenchSpec{
		Name:        "jb-" + substrateName,
		RunsRoot:    runsRoot,
		InstallMode: installMode,
		Prompts:     spec.BenchPrompts{Work: "professional job-bench tasks", DomainWord: "occupation", PromptPack: "adapt", SelectionConformance: "full", DeliverableWord: "deliverable"},
		Channels:    map[string]spec.ChannelSpec{"proposer": ch, "selector": ch},
		ObsTrunc:    50000,
	}
	if err := a.specVal.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adapter) bindSubstrate() {
	if a.SubstrateName == "cx" {
		a.sub = substrate.Codex{Launcher: a.Launcher}
		return
	}
	a.sub = substrate.Opencode{Launcher: a.Launcher}
}

// SetLauncher replaces the substrate launcher (tests / live runner wiring).
func (a *Adapter) SetLauncher(l substrate.Launcher) {
	a.Launcher = l
	a.bindSubstrate()
}

func (a *Adapter) Spec() spec.BenchSpec          { return a.specVal }
func (a *Adapter) Substrate() spec.SubstrateLike { return a.sub }

func (a *Adapter) load() (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cache != nil {
		return a.cache, nil
	}
	m, err := tasks.Discover("jb", "", a.Dataset)
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
	ids, err := tasks.Select(m, split, a.SplitFile, "jb")
	if err != nil {
		return nil
	}
	return ids
}

func (a *Adapter) DomainOf(taskID string) string {
	if i := strings.Index(taskID, "/"); i > 0 {
		return taskID[:i]
	}
	return "unknown"
}

func (a *Adapter) datasetTask(taskID string) (string, error) {
	if !strings.Contains(taskID, "/") || strings.Contains(taskID, "..") || strings.HasPrefix(taskID, "/") {
		return "", fmt.Errorf("malformed JobBench task id %q (expected '<profession>/task<N>')", taskID)
	}
	return filepath.Join(a.Dataset, filepath.FromSlash(taskID)), nil
}

func (a *Adapter) TaskPrompt(taskID string) string {
	base, err := a.datasetTask(taskID)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(base, "task_folder", "TASK_INSTRUCTIONS.txt"))
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *Adapter) TaskFiles(taskID string) string {
	base, err := a.datasetTask(taskID)
	if err != nil {
		return ""
	}
	d := filepath.Join(base, "task_folder")
	if fi, err := os.Stat(d); err == nil && fi.IsDir() {
		return d
	}
	return ""
}

func (a *Adapter) outDir(taskID, label string) string {
	return filepath.Join(a.Store, filepath.FromSlash(taskID), "model_output", label)
}

func (a *Adapter) trajDir(taskID, label string) string {
	return filepath.Join(a.Store, filepath.FromSlash(taskID), "model_traj", label)
}

func (a *Adapter) ensureStoreTask(taskID string) (string, error) {
	dst := filepath.Join(a.Store, filepath.FromSlash(taskID), "task_folder")
	if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
		return dst, nil
	}
	srcBase, err := a.datasetTask(taskID)
	if err != nil {
		return "", err
	}
	src := filepath.Join(srcBase, "task_folder")
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("dataset task inputs missing: %s", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	tmp := dst + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := copyTree(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.RemoveAll(tmp)
		if fi, e := os.Stat(dst); e == nil && fi.IsDir() {
			return dst, nil
		}
		return "", err
	}
	return dst, nil
}

// Execute stages one JobBench rollout (reuse via runmark when intact).
func (a *Adapter) Execute(taskID, harnessDir, runID string, timeout int) (string, error) {
	label := strings.ReplaceAll(runID, "/", "__")
	out := a.outDir(taskID, label)
	mark := a.trajDir(taskID, label)
	inputs := runmark.InputsFingerprint(a.TaskPrompt(taskID), a.TaskFiles(taskID))
	if runmark.IsComplete(mark, harnessDir, true, &inputs) && a.ArtifactsIntact(out) {
		_ = os.MkdirAll(out, 0o755)
		return out, nil
	}
	_ = os.RemoveAll(out)
	_ = os.RemoveAll(mark)
	if _, err := a.ensureStoreTask(taskID); err != nil {
		return "", err
	}
	src := harnessDir
	if src == "" {
		src = a.BaseHarness()
	}
	if src == "" {
		return "", fmt.Errorf("jobbench: no harness_dir and BaseHarness not configured")
	}
	install, err := a.sub.Materialise(src, a.specVal.InstallMode, map[string]string{})
	if err != nil {
		return "", err
	}
	if install.StagingDir != "" && install.StagingDir != src {
		defer os.RemoveAll(install.StagingDir)
	}

	wall := timeout
	if wall <= 0 {
		wall = a.Timeout
	}
	dirs, err := a.sub.Execute(taskID, install, []string{runID}, wall)
	if err != nil {
		return "", err
	}
	_ = os.MkdirAll(out, 0o755)
	extra := map[string]any{}
	if fp := a.outputFingerprint(out); fp != "" {
		extra["output_fingerprint"] = fp
	}
	if _, err := runmark.MarkComplete(mark, runID, harnessDir, &inputs, extra); err != nil {
		return "", err
	}
	if len(dirs) > 0 && dirs[0] != "" {
		return dirs[0], nil
	}
	return out, nil
}

func (a *Adapter) outputFingerprint(runDir string) string {
	if !a.Delivered(runDir) {
		return ""
	}
	return runmark.Fingerprint(runDir)
}

// ArtifactsIntact mirrors Python artifacts_intact.
func (a *Adapter) ArtifactsIntact(runDir string) bool {
	taskID, label, err := a.taskAndLabel(runDir)
	if err != nil {
		return false
	}
	marker := runmark.ReadMark(a.trajDir(taskID, label))
	if marker == nil {
		return false
	}
	got, _ := marker["output_fingerprint"].(string)
	return got != "" && got == a.outputFingerprint(runDir) &&
		(a.Delivered(runDir) || hasJSONLEvents(a.trajDir(taskID, label)))
}

func (a *Adapter) taskAndLabel(runDir string) (string, string, error) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(runDir)), "/")
	for i, p := range parts {
		if p == "model_output" && i >= 2 && i+1 < len(parts) {
			return parts[i-2] + "/" + parts[i-1], parts[i+1], nil
		}
	}
	return "", "", fmt.Errorf("%s is not a JobBench model_output run directory", runDir)
}

func hasJSONLEvents(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, p := range matches {
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), `"type":`) {
			return true
		}
	}
	return false
}

func (a *Adapter) Delivered(runDir string) bool {
	fi, err := os.Stat(runDir)
	if err != nil || !fi.IsDir() {
		return false
	}
	found := false
	_ = filepath.WalkDir(runDir, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !de.IsDir() {
			found = true
			return io.EOF
		}
		return nil
	})
	return found
}

func (a *Adapter) Trajectory(runDir string, obsTrunc int) string {
	taskID, label, err := a.taskAndLabel(runDir)
	if err != nil {
		return ""
	}
	td := a.trajDir(taskID, label)
	matches, _ := filepath.Glob(filepath.Join(td, "*.jsonl"))
	if len(matches) == 0 {
		return fmt.Sprintf("(no trajectory for %s @ %s)", taskID, label)
	}
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return matches[i] > matches[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	text, err := a.sub.ReadTrajectory(matches[0], obsTrunc, 8_000_000, 400)
	if err != nil {
		return ""
	}
	return text
}
func (a *Adapter) DeliverableFiles(runDir string) map[string]string { return nil }
func (a *Adapter) DeliverableText(runDir string) string {
	if !a.Delivered(runDir) {
		return ""
	}
	var b strings.Builder
	_ = filepath.WalkDir(runDir, func(path string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(runDir, path)
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		fmt.Fprintf(&b, "=== %s ===\n%s\n", filepath.ToSlash(rel), data)
		return nil
	})
	return b.String()
}

func (a *Adapter) BaseHarness() string            { return a.BaseHarnessPath }
func (a *Adapter) ValidateTree(t tree.Tree) error { return nil }

// ReadyErrors lists what must be ported before real JobBench runs.
func ReadyErrors(a *Adapter) []string {
	var out []string
	if a == nil {
		return []string{"jobbench adapter is nil"}
	}
	if a.BaseHarness() == "" {
		out = append(out, "jobbench base harness path not configured")
	} else if fi, err := os.Stat(a.BaseHarness()); err != nil || !fi.IsDir() {
		out = append(out, "jobbench base harness missing on disk: "+a.BaseHarness())
	}
	if _, err := a.load(); err != nil {
		out = append(out, "jobbench task discovery: "+err.Error())
	} else if len(a.Tasks("all")) == 0 {
		out = append(out, "jobbench task discovery returned zero tasks")
	}
	return out
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if de.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
