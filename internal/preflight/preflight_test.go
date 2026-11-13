package preflight_test

import (
	"strings"
	"testing"

	"sculpt/internal/preflight"
)

func TestBenchmarkReadyJB(t *testing.T) {
	errs := preflight.BenchmarkReady("jb", "oc", t.TempDir())
	if len(errs) == 0 {
		t.Fatal("expected readiness gaps without dataset")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "jobbench") && !strings.Contains(joined, "discovery") && !strings.Contains(joined, "harness") {
		t.Fatalf("%v", errs)
	}
}

func TestBenchmarkReadyUnknown(t *testing.T) {
	errs := preflight.BenchmarkReady("nope", "oc", "")
	if len(errs) != 1 || !strings.Contains(errs[0], "unknown bench") {
		t.Fatalf("%v", errs)
	}
}
