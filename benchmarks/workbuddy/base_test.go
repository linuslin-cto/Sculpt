package workbuddy_test

import (
	"os"
	"path/filepath"
	"testing"

	"sculpt/benchmarks/workbuddy"
)

func TestBaseHarnessMaterialises(t *testing.T) {
	for _, sub := range []string{"oc", "cx"} {
		t.Run(sub, func(t *testing.T) {
			root := t.TempDir()
			a, err := workbuddy.New(root, sub)
			if err != nil {
				t.Fatal(err)
			}
			base := a.BaseHarness()
			if base == "" {
				t.Fatal("empty")
			}
			want := "systemprompt.md"
			if sub == "cx" {
				want = "AGENTS.md"
			}
			if _, err := os.Stat(filepath.Join(base, want)); err != nil {
				t.Fatal(err)
			}
			for _, e := range workbuddy.ReadyErrors(a) {
				if e == "workbuddy base harness path not configured" {
					t.Fatalf("%v", workbuddy.ReadyErrors(a))
				}
			}
		})
	}
}
