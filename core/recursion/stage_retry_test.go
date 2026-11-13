package recursion_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sculpt/core/proposer"
	"sculpt/core/recursion"
	"sculpt/core/spec"
	"sculpt/core/tree"
	"sculpt/demo"
)

type retryProposer struct {
	channel spec.ChannelSpec
	defect  string
	extra   string
	seen    []string
	missions []string
	calls   []proposer.Reply
}

func (p *retryProposer) Spec() spec.ChannelSpec { return p.channel }
func (p *retryProposer) Calls() []proposer.Reply {
	return append([]proposer.Reply(nil), p.calls...)
}

func (p *retryProposer) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	_ = label
	_ = harvest
	_ = maxAttempts
	harness := filepath.Join(contextDir, "harness")
	script := filepath.Join(harness, "scripts", "summarize.py")
	body, err := os.ReadFile(script)
	if err != nil {
		return proposer.Reply{}, err
	}
	p.seen = append(p.seen, string(body))
	p.missions = append(p.missions, mission)
	if !strings.Contains(mission, "PREVIOUS ATTEMPT WAS REJECTED") {
		_ = os.WriteFile(filepath.Join(harness, filepath.FromSlash(p.defect)), []byte(p.extra), 0o644)
		fixed := strings.ReplaceAll(string(body), " if amount > 0", "")
		_ = os.WriteFile(script, []byte(fixed), 0o644)
	} else if _, err := os.Stat(filepath.Join(harness, filepath.FromSlash(p.defect))); err == nil {
		_ = os.Remove(filepath.Join(harness, filepath.FromSlash(p.defect)))
	}
	reply := proposer.Reply{
		Label: label, Text: "<notes>Keep signed amounts.</notes>", OK: true, Attempts: 1,
		Workspace: harness,
	}
	p.calls = append(p.calls, reply)
	return reply, nil
}

func TestRetryKeepsRejectedAttemptEdits(t *testing.T) {
	cases := []struct {
		defect string
		extra  string
	}{
		{"scripts/check.py", "def broken(:\n"},
		{"scripts/summarize.py.bak", "# an editor's backup, off the editable surface\n"},
	}
	for _, tc := range cases {
		t.Run(tc.defect, func(t *testing.T) {
			root := t.TempDir()
			adapter, err := demo.NewAdapter(root)
			if err != nil {
				t.Fatal(err)
			}
			prop := &retryProposer{
				channel: adapter.Spec().Channels["proposer"],
				defect:  tc.defect,
				extra:   tc.extra,
			}
			fallback := tree.ReadTree(adapter.Substrate().Surface(), adapter.BaseHarness(), nil)
			work := filepath.Join(root, strings.ReplaceAll(tc.defect, "/", "_"))
			stage := recursion.RunStage(
				"inv1", "INV", "Repair the script",
				fallback, adapter, prop,
				filepath.Join(work, "context"), "ledger-1", nil,
				adapter.BaseHarness(), "", nil, nil,
			)
			if len(prop.missions) < 2 {
				t.Fatalf("expected retry, missions=%d", len(prop.missions))
			}
			if !strings.Contains(prop.missions[1], tc.defect) && !strings.Contains(prop.missions[1], filepath.Base(tc.defect)) {
				// feedback should mention the defect path or basename
				if !strings.Contains(prop.missions[1], "REJECTED") {
					t.Fatalf("retry mission missing rejection: %s", prop.missions[1])
				}
			}
			if strings.Contains(prop.seen[1], " if amount > 0") {
				t.Fatal("retry should start from attempt-1 edits, not base script")
			}
			if stage.FellBack || stage.KeptBase || stage.Attempts != 2 {
				t.Fatalf("stage %#v", stage)
			}
			if strings.Contains(stage.Tree["scripts/summarize.py"], " if amount > 0") {
				t.Fatal("accepted tree still filters positives")
			}
			if _, ok := stage.Tree[tc.defect]; ok {
				t.Fatalf("defect %s should not remain in tree", tc.defect)
			}
		})
	}
}
