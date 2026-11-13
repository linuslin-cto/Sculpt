# Harness² Go API contract (from Python)

Source: exploration of `harness2` for the Go rewrite. Critical invariants:

1. Three seams: method / substrate / benchmark — **no scores in the method path**.
2. Budget: `R=4k`, `P=3k`, unique labels after every mode.
3. Stage retry: write rejected tree back into `context/harness/` before attempt 2.
4. Selection: finals-only pool; seeded anon shuffle; fallback never panics; judge only after.
5. Resume: inputs fingerprint + harness fingerprint + intact deliverable.
6. Leak tripwires: `leak_globs` + grader-verdict markers before model context.
7. Library: require `cue`+`lesson`+`evidence`; per-mode roots; retain stream prefix.
