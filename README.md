# Sculpt

Go framework for **recursive agent harnessing**: refine the system around a
fixed-weight model (prompts, tools, memory, scripts) through
**Propose–Probe–Compose**, then persist adopted harnesses and lessons for the
next task.

Inspired by the method in [google-research/harness2](https://github.com/google-research/harness2)
([paper](https://arxiv.org/pdf/2610.14224)). Sculpt is a separate Go project —
binaries, env vars, and packages use the `sculpt` / `SCULPT_*` names.

## Status

Offline demo and core method (tree, ledger, recursion, selection, schedule,
library, judge) are green, including resume / selection-failure / rejudge-merge.
Self-evolution is checked by `TestSelfEvolution*`: buggy base → correct totals,
library persistence, and scored **base 0% → refined 100% (+100 pt)** via
`sculpt-judge` / `--bases` / `sculpt-report`. See [docs/EVAL.md](docs/EVAL.md).
CLI covers dry-run sweep, judge, and report. JobBench / LAB / WorkBuddy adapters
are wired; live Execute still needs OpenCode/Codex checkouts and credentials.

## Requirements

- Go 1.22+
- Python 3.x on `PATH` (demo executor runs `scripts/summarize.py`)

## Quick start

```bash
go test ./...
go run ./cmd/sculpt-demo --output runs/demo --k 2
go run ./cmd/sculpt-demo --output runs/demo-s7 --k 1 --seed 7
go run ./cmd/sculpt --bench demo --output runs/demo --k 1 --seed 7 --dry-run
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1 --modes parallel
go run ./cmd/sculpt-report --bench demo --output runs/demo --k 1
go run ./cmd/sculpt-check --bench demo --output runs/check-demo
```

### Preflight

```bash
go run ./cmd/sculpt-check --bench demo --output runs/check-demo
go run ./cmd/sculpt-check --bench jb --substrate oc --output runs/check-jb
go run ./cmd/sculpt-check --bench lab --substrate cx
go run ./cmd/sculpt-check --bench wb --substrate oc
go run ./cmd/sculpt-check --bench demo --live --output runs/check-demo
```

Concurrency for propose/rollout phases defaults to `SCULPT_PHASE_WIDTH=2`.

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
| `cmd/sculpt*` | CLI entrypoints |

## Self-evolution check

The offline demo is the same evaluation shape as the upstream paper demo:

1. **Base fails** — positive-only `scripts/summarize.py` omits credits.
2. **Task-level PPC** — propose edits, probe on the same ledger, compose a fix
   without reading `reference_answer.txt`.
3. **Harnessing-level** — adopted harness + experience land in `library/` and
   transfer to the next task in the domain stream.

`go test ./demo -run TestSelfEvolution` asserts adoption and scoring.

Scoring pipeline (experiment-guide parity):

```bash
go run ./cmd/sculpt-demo --output runs/demo --k 1
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1 --bases
go run ./cmd/sculpt-report --bench demo --output runs/demo --k 1
```

Paper reference peaks for live benches (qualitative): WorkBuddy +11.5, JobBench
+4.5, LAB +11.8 pp partial — see `eval.PaperGains` / `docs/EVAL.md`.

## License

Apache-2.0
