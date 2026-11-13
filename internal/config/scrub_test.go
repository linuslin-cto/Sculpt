package config_test

import (
	"os"
	"strings"
	"testing"

	"sculpt/internal/config"
)

func TestScrubEnvDropsSecrets(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret")
	t.Setenv("ANTHROPIC_API_KEY", "ant-secret")
	t.Setenv("HARNESS2_JUDGE_API_KEY", "judge-secret")
	t.Setenv("PATH", os.Getenv("PATH"))

	scrubbed := config.ScrubEnv([]string{"PATH"})
	joined := strings.Join(scrubbed, "\n")
	for _, bad := range []string{"sk-secret", "ant-secret", "judge-secret", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "HARNESS2_JUDGE_API_KEY"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("scrub leaked %q in %q", bad, joined)
		}
	}
	foundPath := false
	for _, e := range scrubbed {
		if strings.HasPrefix(e, "PATH=") {
			foundPath = true
		}
	}
	if !foundPath {
		t.Fatal("expected PATH to be kept")
	}
}

func TestScrubEnvKeepsAllowlist(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-keep")
	out := config.ScrubEnv([]string{"OPENAI_API_KEY"})
	found := false
	for _, e := range out {
		if e == "OPENAI_API_KEY=sk-keep" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", out)
	}
}
