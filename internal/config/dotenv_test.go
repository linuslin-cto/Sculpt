package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/internal/config"
)

func TestLoadDotEnvPreservesShellPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("SCULPT_TEST_DOTENV=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCULPT_TEST_DOTENV", "from-shell")
	config.LoadDotEnv(path)
	if got := os.Getenv("SCULPT_TEST_DOTENV"); got != "from-shell" {
		t.Fatalf("shell should win: got %q", got)
	}
}

func TestLoadDotEnvFillsUnsetKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("SCULPT_TEST_DOTENV_UNSET=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("SCULPT_TEST_DOTENV_UNSET")
	config.LoadDotEnv(path)
	if got := os.Getenv("SCULPT_TEST_DOTENV_UNSET"); got != "from-file" {
		t.Fatalf("unset key should load from file: got %q", got)
	}
	_ = os.Unsetenv("SCULPT_TEST_DOTENV_UNSET")
}
