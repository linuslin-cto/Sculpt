# Sculpt

Go rewrite of [google-research/harness2](https://github.com/google-research/harness2): recursive agent harnessing for an open world.

Python reference: [google-research/harness2](https://github.com/google-research/harness2) (optional local checkout at `./harness2`).

## Status

Offline demo and core method (tree, ledger, recursion, selection, schedule, library, judge) are green, including resume / selection-failure / rejudge-merge parity. CLI stubs cover dry-run sweep, judge, and report. JobBench stages Execute/Score with isolation + trajectories; LAB/WorkBuddy have discovery, BaseHarness materialisation, cached Score, Delivered, and Trajectory rendering. Live Execute runners still need OpenCode/Codex checkouts and credentials.

## Requirements

- Go 1.22+
- Python 3.x on `PATH` (demo executor runs `scripts/summarize.py`)

## Quick start

```bash
go test ./...
go run ./cmd/harness2-demo --output runs/demo --k 2
go run ./cmd/harness2-demo --output runs/demo-s7 --k 1 --seed 7
go run ./cmd/harness2 --bench demo --output runs/demo --k 1 --seed 7 --dry-run
go run ./cmd/harness2-judge --bench demo --output runs/demo --k 1 --modes parallel
go run ./cmd/harness2-report --bench demo --output runs/demo --k 1
go run ./cmd/harness2-check --bench demo --output runs/check-demo
```

### Preflight

```bash
# Offline demo readiness (no model binaries required)
go run ./cmd/harness2-check --bench demo --output runs/check-demo

# Benchmark adapter gaps (dataset / base harness / discovery)
go run ./cmd/harness2-check --bench jb --substrate oc --output runs/check-jb
go run ./cmd/harness2-check --bench lab --substrate cx
go run ./cmd/harness2-check --bench wb --substrate oc

# Also verify OpenCode/Codex binaries are present
go run ./cmd/harness2-check --bench demo --live --output runs/check-demo
go run ./cmd/harness2-check --bench jb --substrate oc --live --output runs/check-jb
```

Concurrency for propose/rollout phases defaults to `HARNESS2_PHASE_WIDTH=2` (Python parity).

## Layout

| Package | Role |
|---------|------|
| `core/tree` | c1..c8 harness surface read/write/validate |
| `core/spec` | BenchSpec + BenchmarkAdapter seam |
| `core/ledger` | R=4k / P=3k budget identity |
| `core/recursion` | parallel (`agg`) + sequential (`seq`) |
| `core/selection` | blind choice + experience reflection |
| `core/schedule` | namespaces, stream order, domain driver |
| `core/library` | per-domain experience store |
| `demo` | deterministic local adapter/proposer |
| `benchmarks/{jobbench,lab,workbuddy,tasks}` | bench adapters + discovery |
| `substrate` | OpenCode / Codex surface specs |
| `harness2/` | upstream Python reference (not part of the Go module) |

## Parity

Offline demo mirrors Python `harness2-demo`: Propose–Probe–Compose, R/P ledgers, resume, leak filtering, library prefix transfer, and judge merge on failed rejudge. CLI stubs: `harness2 --dry-run`, `harness2-judge`, `harness2-report`. Bench adapters: JobBench Execute/Score/isolation/trajectories; LAB/WorkBuddy Score + Trajectory (+ WorkBuddy Delivered/killed soft-fail). Remaining gaps are mostly live Execute runners, LAB nested deliverables fixtures, and full live sweeps.
