package substrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/substrate"
)

func TestRenderEventsErrorAndTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.jsonl")
	body := strings.Join([]string{
		`{"type":"tool_use","part":{"type":"tool","tool":"write","state":{"status":"completed","input":{"filePath":"output/memo.docx"},"output":"written"}}}`,
		`{"type":"error","error":{"name":"RunnerExit","data":{"message":"Rollout runner: boom"}}}`,
	}, "\n")
	_ = os.WriteFile(path, []byte(body), 0o644)
	got := substrate.RenderEvents(path, 1000, 10_000, 400)
	if !strings.Contains(got, "[OBSERVATION] written") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "[ERROR] Rollout runner: boom") {
		t.Fatal(got)
	}
}
