package substrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/substrate"
)

func TestOpencodeReadTrajectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	_ = os.WriteFile(path, []byte(`{"type":"text","part":{"type":"text","text":"hello"}}`+"\n"), 0o644)
	got, err := (substrate.Opencode{}).ReadTrajectory(path, 1000, 10_000, 400)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[ASSISTANT] hello") {
		t.Fatalf("%q", got)
	}
}
