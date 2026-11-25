# Setup

[English](SETUP.md) · [简体中文](SETUP.zh-CN.md)

How to get Sculpt running locally — offline first, then optional live benches.

## Requirements

| Need | Offline demo | Live benches |
|------|--------------|--------------|
| Go 1.22+ | Required | Required |
| Python 3 on `PATH` (`python`, `python3`, or `py`) | Required | Required |
| OpenCode (`oc`) or Codex (`cx`) | — | Required for `--live` / Execute |
| Dataset / runner checkouts | — | Per bench |
| API credentials in `.env` | — | Required for model calls |

Never commit `.env` or API keys.

## Offline install

```bash
git clone https://github.com/linuslin-cto/Sculpt.git
cd Sculpt
go test ./...
go run ./cmd/sculpt-check --bench demo --output runs/check-demo
go run ./cmd/sculpt-demo --output runs/demo --k 1
```

No cloud login is required for the deterministic demo.

## Environment variables

Create a local `.env` in the repo root (gitignored). Shell-exported values win over the file.

| Variable | Purpose |
|----------|---------|
| `SCULPT_PHASE_WIDTH` | Propose/rollout concurrency (default `2`) |
| `SCULPT_TRANSPORT_ATTEMPTS` | Transport retries (default `2`) |
| `SCULPT_IMPROVER_MODEL` | Improver / OpenCode proposer model |
| `SCULPT_CLAUDE_MODEL` | Claude CLI proposer model |
| `SCULPT_SOLVER_MODEL` | Solver model for live execute |
| `SCULPT_SOLVER_MODEL_CX` | Codex-specific solver override |
| `SCULPT_RUNS_DIR` | Override runs root |
| `SCULPT_DEPENDENCIES` | Dependencies / third-party root |
| `SCULPT_BUN` | Bun binary path (if needed) |
| `SCULPT_CODEX_DIST` | Codex distribution path |
| `SCULPT_JB_RUNNER_DIR` | JobBench runner checkout |
| `SCULPT_JB_OC_RUNNER` / `SCULPT_JB_CX_RUNNER` | JobBench OpenCode / Codex runner entrypoints |
| `SCULPT_JUDGE_MODEL` / `SCULPT_JUDGE_BASE_URL` / `SCULPT_JUDGE_API_KEY` | Live judge (JobBench) |

## Substrates

| Flag | Name | Notes |
|------|------|--------|
| `--substrate oc` | OpenCode | Default |
| `--substrate cx` | Codex | Use when Codex is installed and configured |

Preflight with binaries:

```bash
go run ./cmd/sculpt-check --bench demo --live --output runs/check-demo
go run ./cmd/sculpt-check --bench jb --substrate oc --output runs/check-jb
go run ./cmd/sculpt-check --bench lab --substrate cx
go run ./cmd/sculpt-check --bench wb --substrate oc
```

## Benchmarks

| Alias | Focus | Typical extras |
|-------|--------|----------------|
| `demo` | Offline ledger / PPC | Python only |
| `jb` | JobBench | Runner dir + judge env |
| `lab` | LAB | Dataset / toolchain as required by adapter |
| `wb` | WorkBuddy-Bench | Dataset / Docker or runner as required |

Adapters are wired; full live Execute still needs working substrate checkouts and credentials. Until then, prefer `--dry-run` on `sculpt` and the offline `sculpt-demo` path.

```bash
go run ./cmd/sculpt --bench demo --output runs/demo --k 1 --seed 7 --dry-run
```

## Verify

```bash
go test ./...
go test ./demo -run TestSelfEvolution
go run ./cmd/sculpt-demo --output runs/demo --k 1
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1 --bases
go run ./cmd/sculpt-report --bench demo --output runs/demo --k 1
```

Expected offline Δ: **base 0% → refined 100% (+100 pt)**. See [EVAL.md](EVAL.md).

## Related

- [API_CONTRACT.md](API_CONTRACT.md) — runtime invariants
- [EVAL.md](EVAL.md) — judge / report pipeline
- [i18n/README.md](i18n/README.md) — documentation locales
