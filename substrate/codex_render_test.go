package substrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/substrate"
)

func TestCodexReadTrajectoryCommandAndAssistant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cx.jsonl")
	body := strings.Join([]string{
		`{"type":"item","item":{"type":"command_execution","command":"ls","status":"completed","aggregated_output":"ok","exit_code":0}}`,
		`{"type":"item","item":{"type":"agent_message","text":"done"}}`,
		`{"type":"error","error":"boom"}`,
	}, "\n")
	_ = os.WriteFile(path, []byte(body), 0o644)
	got, err := (substrate.Codex{}).ReadTrajectory(path, 1000, 10_000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[ACTION] shell: ls") || !strings.Contains(got, "[ASSISTANT] done") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "[ERROR]") {
		t.Fatal(got)
	}
}
