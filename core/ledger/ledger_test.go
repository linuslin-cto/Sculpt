package ledger_test

import (
	"fmt"
	"testing"

	"sculpt/core/ledger"
)

func TestBudgetIdentity(t *testing.T) {
	for _, k := range []int{1, 2, 3} {
		l := &ledger.Ledger{Mode: "agg", TaskID: "t", K: k}
		for i := 0; i < ledger.ExpectedRollouts(k); i++ {
			l.Rollout(fmt.Sprintf("r%d", i), 0, false)
		}
		for i := 0; i < ledger.ExpectedSessions(k); i++ {
			l.Session(fmt.Sprintf("s%d", i), 0.1, false, 1)
		}
		if err := l.AssertIdentity(); err != nil {
			t.Fatal(err)
		}
		d := l.ToDict()
		if d["R"] != ledger.ExpectedRollouts(k) || d["P"] != ledger.ExpectedSessions(k) {
			t.Fatalf("%#v", d)
		}
	}
}

func TestDuplicateFails(t *testing.T) {
	l := &ledger.Ledger{Mode: "seq", TaskID: "t", K: 1}
	l.Rollout("base1", 0, false)
	l.Rollout("base1", 0, false)
	if err := l.AssertIdentity(); err == nil {
		t.Fatal("expected duplicate error")
	}
}
