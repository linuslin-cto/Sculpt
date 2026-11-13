// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package jobbench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sculpt/internal/config"
)

// Score judges one run. A nil map (with nil error) means the judge failed — not a zero.
func (a *Adapter) Score(runDir, taskID string) (map[string]any, error) {
	label := filepath.Base(runDir)
	reporting, own := a.detailsCandidates(taskID, label)
	for _, details := range []*string{reporting, &own} {
		if details == nil || *details == "" {
			continue
		}
		got := a.readDetails(*details)
		if got == nil {
			continue
		}
		parsed := parseDetails(got)
		if parsed != nil {
			return parsed, nil
		}
	}
	if !a.Delivered(runDir) {
		return a.noOutputScore(taskID), nil
	}
	if !a.runJudge(taskID, label, runDir, own) {
		return nil, nil
	}
	got := a.readDetails(own)
	if got == nil {
		return nil, nil
	}
	return parseDetails(got), nil
}

// runJudge invokes upstream eval/judge.py when present. Returns false on any soft failure.
func (a *Adapter) runJudge(taskID, label, runDir, detailsPath string) bool {
	judgePy := filepath.Join(config.JbRepo(), "eval", "judge.py")
	if fi, err := os.Stat(judgePy); err != nil || fi.IsDir() {
		fmt.Printf("  [jb] upstream judge missing (%s); judge not run\n", judgePy)
		return false
	}
	base, err := a.datasetTask(taskID)
	if err != nil {
		return false
	}
	rubrics := filepath.Join(base, "RUBRICS.json")
	if fi, err := os.Stat(rubrics); err != nil || fi.IsDir() {
		fmt.Printf("  [jb] task rubrics missing (%s); judge not run\n", rubrics)
		return false
	}
	py := config.JbJudgePython()
	if fi, err := os.Stat(py); err != nil || fi.IsDir() {
		if p, e := findPython(); e == nil {
			py = p
		} else {
			fmt.Printf("  [jb] judge interpreter missing (%s); judge not run\n", config.JbJudgePython())
			return false
		}
	}
	apiBase := config.JbJudgeAPIBase()
	if apiBase == "" {
		fmt.Printf("  [jb] SCULPT_JUDGE_BASE_URL empty; judge not run\n")
		return false
	}
	token := config.JbJudgeAPIKey()
	if token == "" {
		token = gcloudAccessToken()
	}
	if token == "" {
		fmt.Printf("  [jb] no judge API key / gcloud token; judge not run\n")
		return false
	}
	_ = os.MkdirAll(filepath.Dir(detailsPath), 0o755)
	cmd := exec.Command(py, judgePy,
		"--output-dir", runDir,
		"--rubrics-file", rubrics,
		"--details-file", detailsPath,
		"--judge-model", config.JbJudgeModel(),
		"--api-base", apiBase,
		"--max-workers", "8",
		"--max-retries", "3",
		"--timeout-per-rubric", "240",
		"--evaluated-model", label,
	)
	cmd.Env = append(config.ScrubEnv([]string{
		"PATH", "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "TEMP", "TMP",
		"SYSTEMROOT", "COMSPEC", "LANG", "LC_ALL", "TZ", "CLOUDSDK_CONFIG",
	}),
		"JUDGE_API_KEY="+token,
		"JUDGE_REASONING_EFFORT="+os.Getenv("SCULPT_JUDGE_REASONING_EFFORT"),
		"VERTEXAI_LOCATION="+os.Getenv("VERTEXAI_LOCATION"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 300 {
			tail = tail[len(tail)-300:]
		}
		fmt.Printf("  [jb] judge failed for %s: %v (%s)\n", label, err, tail)
		return false
	}
	return true
}

func gcloudAccessToken() string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "application-default", "print-access-token")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (a *Adapter) detailsCandidates(taskID, label string) (*string, string) {
	er := filepath.Join(a.Store, filepath.FromSlash(taskID), "eval_result")
	safe := config.JbJudgeModel()
	if i := strings.LastIndex(safe, "/"); i >= 0 {
		safe = safe[i+1:]
	}
	safe = strings.ReplaceAll(safe, ".", "-")
	own := filepath.Join(er, "eval_"+label, safe+"_judge.json")
	pattern := os.Getenv("SCULPT_JB_REPORTING_JUDGE_DETAILS")
	if pattern == "" {
		return nil, own
	}
	rep := filepath.Join(er, strings.ReplaceAll(pattern, "{label}", label))
	return &rep, own
}

func (a *Adapter) readDetails(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var got map[string]any
	if json.Unmarshal(b, &got) != nil {
		return nil
	}
	if judgeFailed(got) {
		_ = os.Rename(path, path+".judge_failed")
		return nil
	}
	rubrics, _ := got["rubrics"].([]any)
	if tc, ok := asInt(got["total_count"]); ok && tc > 0 && len(rubrics) < tc {
		fmt.Printf("  [jb] partial judge details (%d/%d rubrics) at %s; not served\n", len(rubrics), tc, path)
		return nil
	}
	return got
}

func (a *Adapter) noOutputScore(taskID string) map[string]any {
	base, err := a.datasetTask(taskID)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(base, "RUBRICS.json"))
	if err != nil {
		return nil
	}
	var doc struct {
		Rubrics []map[string]any `json:"rubrics"`
	}
	if json.Unmarshal(b, &doc) != nil || len(doc.Rubrics) == 0 {
		return nil
	}
	var mx float64
	nC := 0
	for _, r := range doc.Rubrics {
		mx += asFloat(r["weight"])
		switch c := r["criterion"].(type) {
		case string:
			nC++
		case []any:
			nC += len(c)
		}
	}
	if mx == 0 {
		return nil
	}
	return map[string]any{
		"n_passed":        0.0,
		"n_criteria":      mx,
		"all_pass":        false,
		"metric":          "weighted",
		"criteria_passed": 0,
		"criteria_count":  nC,
		"no_output":       true,
	}
}

func parseDetails(got map[string]any) map[string]any {
	rubrics, _ := got["rubrics"].([]any)
	nC, nP := 0, 0
	for _, raw := range rubrics {
		r, _ := raw.(map[string]any)
		res, _ := r["result"].(map[string]any)
		if res == nil {
			res = map[string]any{}
		}
		nC += asIntDefault(res["criteria_count"])
		nP += asIntDefault(res["criteria_passed"])
	}
	total := asFloat(got["total_score"])
	mx := asFloat(got["max_score"])
	if mx == 0 {
		if nC <= 0 {
			return nil
		}
		return map[string]any{
			"n_passed":   nP,
			"n_criteria": nC,
			"all_pass":   nP == nC,
			"metric":     "unweighted-fallback",
		}
	}
	return map[string]any{
		"n_passed":        total,
		"n_criteria":      mx,
		"all_pass":        nC > 0 && nP == nC,
		"metric":          "weighted",
		"criteria_passed": nP,
		"criteria_count":  nC,
	}
}

func judgeFailed(got map[string]any) bool {
	if err, ok := got["error"].(string); ok && strings.TrimSpace(err) != "" {
		if got["rubrics"] == nil && got["details"] == nil {
			return true
		}
	}
	for _, key := range []string{"overall_reasoning", "reasoning", "error"} {
		if v, ok := got[key].(string); ok && strings.Contains(strings.ToLower(v), "judge failed") {
			return true
		}
	}
	return false
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

func asIntDefault(v any) int {
	n, _ := asInt(v)
	return n
}
