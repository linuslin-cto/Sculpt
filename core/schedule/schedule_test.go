package schedule_test

import (
	"strings"
	"testing"

	"sculpt/core/schedule"
)

func TestNSAndModeToken(t *testing.T) {
	if schedule.NS("demo", "oc", "agg", 2, nil, "") != "demo-oc-agg2" {
		t.Fatal(schedule.NS("demo", "oc", "agg", 2, nil, ""))
	}
	seed := 7
	if schedule.NS("demo", "oc", "seq", 1, &seed, "x") != "demo-oc-seq1-s7-x" {
		t.Fatal("seed/tag ns")
	}
	tok, err := schedule.ModeToken("parallel")
	if err != nil || tok != "agg" {
		t.Fatal(tok, err)
	}
	tok, err = schedule.ModeToken("sequential")
	if err != nil || tok != "seq" {
		t.Fatal(tok, err)
	}
	if err := schedule.ValidateTag("s1"); err == nil {
		t.Fatal("seed-like tag should fail")
	}
	if err := schedule.ValidateTag("ok-tag"); err != nil {
		t.Fatal(err)
	}
	if err := schedule.ValidateTag("-bad"); err == nil {
		t.Fatal("leading dash should fail")
	}
}

func TestStreamOrderStable(t *testing.T) {
	by := map[string][]string{"b": {"t2", "t1"}, "a": {"x"}}
	out, err := schedule.StreamOrder(by, nil, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0][0] != "a" || out[1][1] != "t1" {
		t.Fatalf("%#v", out)
	}
}

func TestStreamOrderSeedDeterministic(t *testing.T) {
	by := map[string][]string{"b": {"t2", "t1"}, "a": {"x", "y"}}
	seed := 42
	a, err := schedule.StreamOrder(by, &seed, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := schedule.StreamOrder(by, &seed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatal(len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("%v vs %v", a, b)
		}
	}
	other := 7
	c, err := schedule.StreamOrder(by, &other, nil)
	if err != nil {
		t.Fatal(err)
	}
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds should reshuffle stream")
	}
}

func TestStreamOrderRequiresReferenceCoverage(t *testing.T) {
	by := map[string][]string{"a": {"t"}, "orphan": {"x"}}
	_, err := schedule.StreamOrder(by, nil, []string{"a"})
	if err == nil {
		t.Fatal("expected uncovered domain error")
	}
}

func TestNamespaceRunIDAndTaskDir(t *testing.T) {
	n := schedule.Namespace{Benchmark: "demo", Substrate: "oc", Mode: "agg", K: 2}
	if n.String() != "demo-oc-agg2" {
		t.Fatal(n.String())
	}
	if n.RunID("ledger-1", "c1") != "demo-oc-agg2/ledger-1/c1" {
		t.Fatal(n.RunID("ledger-1", "c1"))
	}
	dir := n.TaskDir("runs", "a/b")
	if !strings.Contains(dir, "demo-oc-agg2") || !strings.Contains(dir, "a__b") {
		t.Fatal(dir)
	}
}
