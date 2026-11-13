package jobbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/benchmarks/jobbench"
	"sculpt/substrate"
)

func TestNoteExitSignalWording(t *testing.T) {
	cases := []struct {
		code  int
		limit string
		want  string
	}{
		{-9, "2400", "SIGKILL (-9), after the 2400 s time limit or by the system"},
		{137, "2400", "SIGKILL (137), after the 2400 s time limit or by the system"},
		{-11, "2400", "killed by signal 11"},
		{124, "40m", "stopped at the 40m time limit (exit 124)"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			dir := t.TempDir()
			traj := filepath.Join(dir, "t.jsonl")
			log := filepath.Join(dir, "x.log")
			_ = os.WriteFile(traj, nil, 0o644)
			if err := jobbench.NoteExit(traj, log, tc.code, tc.limit, false); err != nil {
				t.Fatal(err)
			}
			rendered := substrate.RenderEvents(traj, 1000, 10_000, 400)
			if !strings.Contains(rendered, tc.want) {
				t.Fatalf("got %q want substring %q", rendered, tc.want)
			}
			if !strings.HasPrefix(strings.TrimSpace(strings.Split(rendered, "\n")[len(strings.Split(rendered, "\n"))-1]), "[ERROR] ") &&
				!strings.Contains(rendered, "[ERROR] ") {
				t.Fatalf("missing ERROR tag: %q", rendered)
			}
			logBody, _ := os.ReadFile(log)
			if !strings.Contains(string(logBody), "EXIT") {
				t.Fatalf("log %#q", logBody)
			}
		})
	}
}

func TestNoteExitCleanIsNoop(t *testing.T) {
	dir := t.TempDir()
	traj := filepath.Join(dir, "t.jsonl")
	_ = os.WriteFile(traj, []byte("{}\n"), 0o644)
	if err := jobbench.NoteExit(traj, "", 0, "30", true); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(traj)
	if strings.Contains(string(body), "error") {
		t.Fatalf("%s", body)
	}
}

func TestNoteExitKeptDeliverableWording(t *testing.T) {
	dir := t.TempDir()
	traj := filepath.Join(dir, "t.jsonl")
	_ = os.WriteFile(traj, nil, 0o644)
	if err := jobbench.NoteExit(traj, "", 124, "1", true); err != nil {
		t.Fatal(err)
	}
	rendered := substrate.RenderEvents(traj, 1000, 10_000, 400)
	if !strings.Contains(rendered, "kept as the deliverable") {
		t.Fatal(rendered)
	}
}
