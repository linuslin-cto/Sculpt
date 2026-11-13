package schedule_test

import (
	"testing"

	"sculpt/core/schedule"
	"sculpt/demo"
)

func TestTaskStreamSeedAndMaxTasks(t *testing.T) {
	root := t.TempDir()
	adapter, err := demo.NewAdapter(root)
	if err != nil {
		t.Fatal(err)
	}
	seed := 7
	max := 1
	by, flat, err := schedule.TaskStream(adapter, schedule.TaskStreamOpts{Seed: &seed, MaxTasks: &max})
	if err != nil {
		t.Fatal(err)
	}
	if len(by) != 1 || len(by["accounting"]) != 1 {
		t.Fatalf("%#v", by)
	}
	if len(flat) != 1 || flat[0][0] != "accounting" {
		t.Fatalf("%#v", flat)
	}
}

func TestParseModes(t *testing.T) {
	got, err := schedule.ParseModes("sequential,parallel")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "agg" || got[1] != "seq" {
		t.Fatalf("%v", got)
	}
	if _, err := schedule.ParseModes(""); err == nil {
		t.Fatal("expected empty modes error")
	}
}
