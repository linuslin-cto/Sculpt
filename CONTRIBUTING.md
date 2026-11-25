# Contributing to Sculpt

Thanks for helping improve Sculpt. This project is a Go implementation of
recursive agent harnessing ideas; keep the surface small, tested, and named
`sculpt` / `SCULPT_*`.

## Development setup

```bash
git clone https://github.com/linuslin-cto/Sculpt.git
cd Sculpt
go test ./...
```

Offline demo and most unit tests need Go + Python 3 on `PATH`. Do not commit
`.env`, API keys, or run artifacts under `runs/`.

## Workflow

1. Prefer a focused change (one concern per PR).
2. Match existing package layout and error/resume semantics in `docs/API_CONTRACT.md`.
3. Add or update tests next to the code you touch.
4. Run at least:

   ```bash
   go test ./...
   go test ./demo -run TestSelfEvolution
   ```

5. Update docs when behavior or flags change — English first, then locales
   (see [docs/i18n/README.md](docs/i18n/README.md)).

## Code style

- Keep public names and CLI flags stable; rename only with a clear migration note.
- Prefer small helpers over deep abstraction.
- Never log or persist secrets; scrub env in judge/score paths when dumping context.

## Documentation & translations

- `README.md` is the source of truth (English).
- Localized READMEs: `README.<locale>.md` (e.g. `README.zh-CN.md`).
- Keep section order aligned across languages.
- Do **not** translate commands, paths, flag names, or `SCULPT_*` variables.

## Issues & PRs

- Describe the problem or goal, what you changed, and how you tested.
- Link related issues when applicable.
- For live-bench work, say which substrate (`oc` / `cx`) and whether credentials
  were required (never paste secrets).

## License

By contributing, you agree that your contributions are licensed under the
Apache License 2.0.
