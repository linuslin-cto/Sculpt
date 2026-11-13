package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sculpt/core/proposer"
	"sculpt/core/schedule"
	"sculpt/core/spec"
	"sculpt/demo"
)

type firstCandidateSelector struct {
	*demo.Proposer
}

func (p *firstCandidateSelector) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	if len(label) >= 7 && label[:7] == "select." {
		ents, _ := os.ReadDir(filepath.Join(contextDir, "candidates"))
		var names []string
		for _, e := range ents {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				names = append(names, e.Name()[:len(e.Name())-3])
			}
		}
		if len(names) == 0 {
			return proposer.Reply{}, &proposer.Error{Message: "no candidates", Attempts: 1}
		}
		// sorted first
		chosen := names[0]
		for _, n := range names[1:] {
			if n < chosen {
				chosen = n
			}
		}
		reply := proposer.Reply{Label: label, Text: "```choice\n" + chosen + "\n```", OK: true, Attempts: 1}
		return reply, nil
	}
	return p.Proposer.Call(mission, contextDir, label, harvest, maxAttempts)
}

type outageProposer struct {
	ch spec.ChannelSpec
}

func (o *outageProposer) Spec() spec.ChannelSpec { return o.ch }
func (o *outageProposer) Calls() []proposer.Reply { return nil }
func (o *outageProposer) Call(mission, contextDir, label string, harvest string, maxAttempts int) (proposer.Reply, error) {
	return proposer.Reply{}, &proposer.Error{Message: "offline outage", Attempts: 2, CostUSD: 0.25}
}

func TestOutagePartialThenHealthyRerun(t *testing.T) {
	for _, k := range []int{1, 2} {
		t.Run(itoa(k), func(t *testing.T) {
			root := t.TempDir()
			adapter, err := demo.NewAdapter(root)
			if err != nil {
				t.Fatal(err)
			}
			specCh := adapter.Spec().Channels["proposer"]
			selector := &firstCandidateSelector{Proposer: demo.NewProposer(specCh)}
			executions := adapter.ExecutionCount
			failed := schedule.RunDomain(
				"accounting", adapter.Tasks("all"), k, adapter,
				&outageProposer{ch: specCh},
				adapter.Spec().RunsRoot, filepath.Join(root, "library", itoa(k)),
				nil, schedule.DomainOptions{Selector: selector},
			)
			if failed["done"] != 0 || failed["failed"] != 2 {
				t.Fatalf("failed stream %#v", failed)
			}
			tasks := failed["tasks"].([]map[string]any)
			for _, row := range tasks {
				if row["status"] != "partial" {
					t.Fatalf("%#v", row)
				}
			}
			for _, taskID := range adapter.Tasks("all") {
				summaryPath := filepath.Join(adapter.Spec().RunsRoot, "demo-oc-agg"+itoa(k), taskID, "summary.json")
				b, err := os.ReadFile(summaryPath)
				if err != nil {
					t.Fatal(err)
				}
				var summary map[string]any
				if err := json.Unmarshal(b, &summary); err != nil {
					t.Fatal(err)
				}
				if summary["status"] != "partial" {
					t.Fatalf("status=%v", summary["status"])
				}
				errs := summary["errors"].([]any)
				if len(errs) != 6*k {
					t.Fatalf("errors len=%d want %d: %v", len(errs), 6*k, errs)
				}
				modes := summary["modes"].(map[string]any)
				sels := summary["selections"].(map[string]any)
				for _, mode := range []string{"agg", "seq"} {
					record := modes[mode].(map[string]any)
					if record["ok"] == true {
						t.Fatal("mode should not be ok")
					}
					if sels[mode].(map[string]any)["ok"] != true {
						t.Fatal("selection should succeed via fallback finals")
					}
					ledger := record["ledger"].(map[string]any)
					if int(ledger["R"].(float64)) != 4*k || int(ledger["P"].(float64)) != 3*k {
						t.Fatalf("ledger %#v", ledger)
					}
					if int(ledger["session_attempts"].(float64)) != 6*k {
						t.Fatalf("session_attempts=%v", ledger["session_attempts"])
					}
					if ledger["session_usd"].(float64) != 0.75*float64(k) {
						t.Fatalf("session_usd=%v", ledger["session_usd"])
					}
					stages := record["stages"].([]any)
					for _, raw := range stages {
						st := raw.(map[string]any)
						if st["execution_failed"] != true || st["fell_back"] != true {
							t.Fatalf("stage %#v", st)
						}
						if int(st["attempts"].(float64)) != 2 || st["usd"].(float64) != 0.25 {
							t.Fatalf("stage cost %#v", st)
						}
					}
				}
			}
			if adapter.ExecutionCount-executions != 14*k {
				t.Fatalf("executions delta %d want %d", adapter.ExecutionCount-executions, 14*k)
			}

			prop := demo.NewProposer(specCh)
			executions = adapter.ExecutionCount
			healthy := schedule.RunDomain(
				"accounting", adapter.Tasks("all"), k, adapter, prop,
				adapter.Spec().RunsRoot, filepath.Join(root, "library", itoa(k)),
				nil, schedule.DomainOptions{Selector: prop},
			)
			htasks := healthy["tasks"].([]map[string]any)
			for _, row := range htasks {
				if row["status"] != "ok" {
					t.Fatalf("healthy %#v", row)
				}
			}
			if adapter.ExecutionCount-executions != 14*k {
				t.Fatalf("healthy executions delta %d", adapter.ExecutionCount-executions)
			}
			executions, calls := adapter.ExecutionCount, len(prop.Calls())
			resumed := schedule.RunDomain(
				"accounting", adapter.Tasks("all"), k, adapter, prop,
				adapter.Spec().RunsRoot, filepath.Join(root, "library", itoa(k)),
				nil, schedule.DomainOptions{Selector: prop},
			)
			for _, row := range resumed["tasks"].([]map[string]any) {
				if row["status"] != "skipped" {
					t.Fatalf("resume %#v", row)
				}
			}
			if adapter.ExecutionCount != executions || len(prop.Calls()) != calls {
				t.Fatal("resume should be cache hit")
			}
		})
	}
}
