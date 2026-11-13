package runmark_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/runmark"
)

func TestInputsFingerprintStableAndSensitive(t *testing.T) {
	root := t.TempDir()
	files := filepath.Join(root, "documents")
	_ = os.MkdirAll(files, 0o755)
	_ = os.WriteFile(filepath.Join(files, "source.txt"), []byte("version-one"), 0o644)

	first := runmark.InputsFingerprint("Use version one.", files)
	second := runmark.InputsFingerprint("Use version one.", files)
	if first != second {
		t.Fatal("unstable digest")
	}
	if first == runmark.InputsFingerprint("Use version two.", files) {
		t.Fatal("prompt change ignored")
	}
	_ = os.WriteFile(filepath.Join(files, "source.txt"), []byte("version-two"), 0o644)
	changed := runmark.InputsFingerprint("Use version one.", files)
	if first == changed {
		t.Fatal("file content change ignored")
	}
	_ = os.WriteFile(filepath.Join(files, "extra.txt"), []byte("more"), 0o644)
	added := runmark.InputsFingerprint("Use version one.", files)
	if changed == added {
		t.Fatal("membership change ignored")
	}

	before := runmark.InputsFingerprint("Use version one.", files)
	_ = os.MkdirAll(filepath.Join(files, "__pycache__"), 0o755)
	_ = os.WriteFile(filepath.Join(files, "__pycache__", "stale.pyc"), []byte{0}, 0o644)
	if before != runmark.InputsFingerprint("Use version one.", files) {
		t.Fatal("pycache should be ignored")
	}
	if runmark.InputsFingerprint("Use version one.", "") != runmark.InputsFingerprint("Use version one.", filepath.Join(root, "absent")) {
		t.Fatal("missing material should match empty")
	}
}

func TestMarkerInputIdentity(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	aaaa, bbbb := "aaaa", "bbbb"
	if _, err := runmark.MarkComplete(runDir, "r1", "", &aaaa, nil); err != nil {
		t.Fatal(err)
	}
	if !runmark.IsComplete(runDir, "", true, &aaaa) {
		t.Fatal("expected reusable")
	}
	if runmark.IsComplete(runDir, "", true, &bbbb) {
		t.Fatal("changed inputs must invalidate")
	}
	if runmark.WhyNot(runDir, "", &aaaa) != "reusable" {
		t.Fatal(runmark.WhyNot(runDir, "", &aaaa))
	}
	why := runmark.WhyNot(runDir, "", &bbbb)
	if why == "reusable" || !strings.Contains(why, "task inputs aaaa != bbbb") {
		t.Fatal(why)
	}

	// Markers written before input identity existed must not invalidate on upgrade.
	legacy := filepath.Join(t.TempDir(), "legacy")
	if _, err := runmark.MarkComplete(legacy, "r1", "", nil, nil); err != nil {
		t.Fatal(err)
	}
	if runmark.ReadMark(legacy)["inputs"] != nil {
		t.Fatal("legacy marker must omit inputs")
	}
	if !runmark.IsComplete(legacy, "", true, &aaaa) {
		t.Fatal("legacy marker stays reusable under new inputs")
	}
	if runmark.WhyNot(legacy, "", &aaaa) != "reusable" {
		t.Fatal(runmark.WhyNot(legacy, "", &aaaa))
	}

	// Caller without inputs keeps previous behaviour.
	if !runmark.IsComplete(runDir, "", true, nil) {
		t.Fatal("nil inputs must not invalidate")
	}
	if runmark.WhyNot(runDir, "", nil) != "reusable" {
		t.Fatal(runmark.WhyNot(runDir, "", nil))
	}
}
