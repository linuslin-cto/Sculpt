package jobbench_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/benchmarks/jobbench"
)

func TestIsolationEnvUsesPrivateSandbox(t *testing.T) {
	base := t.TempDir()
	jobbench.SetTempBaseForTest(base)
	t.Cleanup(func() { jobbench.SetTempBaseForTest("") })

	ws := filepath.Join(base, "long-task-id_base1_4194304")
	env := jobbench.IsolationEnv(ws, "/usr/bin")
	if env["NPM_CONFIG_PREFIX"] == "" || env["PYTHONUSERBASE"] == "" || env["TMPDIR"] == "" {
		t.Fatalf("%#v", env)
	}
	if filepath.Clean(env["TMPDIR"]) == filepath.Clean(os.TempDir()) {
		t.Fatal("TMPDIR must not be the host temp")
	}
	if !strings.HasPrefix(env["TMPDIR"], jobbench.SandboxDir(ws)) {
		t.Fatalf("tmpdir %#v sandbox %#v", env["TMPDIR"], jobbench.SandboxDir(ws))
	}
	sep := string(os.PathListSeparator)
	head := strings.Split(env["PATH"], sep)
	if len(head) < 2 {
		t.Fatalf("PATH %#v", env["PATH"])
	}
	if head[0] != filepath.Join(env["NPM_CONFIG_PREFIX"], "bin") {
		t.Fatalf("PATH head %#v", head[0])
	}
	if head[1] != filepath.Join(env["PYTHONUSERBASE"], "bin") {
		t.Fatalf("PATH head2 %#v", head[1])
	}
}

func TestPermissionFenceBlocksSiblingRollouts(t *testing.T) {
	base := t.TempDir()
	jobbench.SetTempBaseForTest(base)
	t.Cleanup(func() { jobbench.SetTempBaseForTest("") })

	ws := filepath.Join(base, "t1_base1_1")
	raw, err := jobbench.PermissionJSON(ws)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	ext := body["external_directory"].(map[string]any)
	if ext["*"] != "deny" {
		t.Fatalf("%#v", ext)
	}
	wsAllow := filepath.ToSlash(ws) + "/**"
	if ext[wsAllow] != "allow" {
		t.Fatalf("missing workspace allow %#v", ext)
	}
}
