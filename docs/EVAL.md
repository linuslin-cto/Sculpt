# Evaluation and self-evolution scoring

[English](EVAL.md) · [简体中文](EVAL.zh-CN.md)

Sculpt follows the upstream experiment pipeline:

1. **Refine** — `sculpt-demo` / `sculpt` runs Propose–Probe–Compose and writes
   task `summary.json` plus adopted harnesses.
2. **Judge refined** — `sculpt-judge` scores selected runs into `judged.json`.
3. **Judge bases** — `sculpt-judge --bases` scores reference executions into
   `base_judged.json`.
4. **Report** — `sculpt-report` prints refined figures and, when bases exist,
   the **base → refined Δ** (self-evolution gain in percentage points).

Ungraded judge failures are excluded from the denominator (never forced to zero).

## Offline demo contract

The deterministic ledger demo is exact-match graded (`n_passed` / `n_criteria`):

| Stage | Expected |
|-------|----------|
| Base harness (positive-only sum) | 0% |
| After one PPC step | 100% |
| Δ | **+100 pt** |

`go test ./demo -run TestSelfEvolution` covers adoption/library;
`TestSelfEvolutionScored` covers judge → bases → report scoring.

## Live paper reference peaks

Qualitative targets from the method paper’s reported peaks (not CI gates):

| Bench | Peak gain over base |
|-------|---------------------|
| WorkBuddy (`wb`) | +11.5 pt |
| JobBench (`jb`) | +4.5 pt |
| LAB (`lab`) | +11.8 pp partial pass rate |

These live in `eval.PaperGains` and appear in the report delta block when the
bench name matches.
