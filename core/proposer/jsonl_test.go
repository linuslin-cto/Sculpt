package proposer_test

import (
	"math"
	"testing"

	"sculpt/core/proposer"
)

func TestParseOpenCodeStdoutJoinsText(t *testing.T) {
	stdout := "" +
		`{"type":"text","part":{"type":"text","text":"hello"}}` + "\n" +
		`{"type":"text","part":{"type":"text","text":"world"}}` + "\n" +
		`{"type":"step-finish","part":{"type":"step-finish","cost":0.12}}` + "\n"
	text, err := proposer.ParseOpenCodeStdout(stdout)
	if err != "" || text != "hello\n\nworld" {
		t.Fatalf("text=%q err=%q", text, err)
	}
	if c := proposer.OpenCodeCost(stdout); math.Abs(c-0.12) > 1e-9 {
		t.Fatalf("cost=%v", c)
	}
}

func TestParseOpenCodeStdoutErrorKeepsCost(t *testing.T) {
	stdout := "" +
		`{"type":"text","part":{"type":"text","text":"partial"}}` + "\n" +
		`{"type":"error","error":{"data":{"message":"boom"}}}` + "\n" +
		`{"type":"step_finish","part":{"type":"step_finish","cost":1.5}}` + "\n"
	text, err := proposer.ParseOpenCodeStdout(stdout)
	if text != "partial" || err != "boom" {
		t.Fatalf("text=%q err=%q", text, err)
	}
	if c := proposer.OpenCodeCost(stdout); math.Abs(c-1.5) > 1e-9 {
		t.Fatalf("cost=%v", c)
	}
}

func TestParseOpenCodePlainFallback(t *testing.T) {
	text, err := proposer.ParseOpenCodeStdout("opencode 1.0\nfinal answer\n")
	if err != "" || text != "final answer" {
		t.Fatalf("text=%q err=%q", text, err)
	}
}
