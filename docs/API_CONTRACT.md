# Sculpt API contract

Critical invariants for the Go framework (aligned with the recursive harnessing method):

1. **Budget identity** — each mode runs with `R = 4k` rollouts and `P = 3k`
   improver sessions per task.
2. **Propose–Probe–Compose** — candidates are probed on the same task; selection
   is blind to ground truth (leak globs filtered from context).
3. **Adoption** — chosen harness is materialised under `harnesses/<label>/` and
   recorded in `reflection/*/adoption.json`.
4. **Library** — successful experience is stored per domain/mode and prefixed
   into later tasks in the stream.
5. **Resume** — completed summaries are cache hits unless adopted artifacts are
   corrupted or modes are re-requested.
6. **Env** — deployment knobs use `SCULPT_*` (for example `SCULPT_PHASE_WIDTH`,
   `SCULPT_IMPROVER_MODEL`, `SCULPT_RUNS_DIR`).
