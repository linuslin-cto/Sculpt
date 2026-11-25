# Internationalization (i18n)

Sculpt documentation is maintained with a **primary English tree** and
locale-specific entry points that stay in sync for user-facing pages.

## Languages

| Locale | Code | Entry | Status |
|--------|------|-------|--------|
| English | `en` | [README.md](../../README.md) | Canonical |
| 简体中文 | `zh-CN` | [README.zh-CN.md](../../README.zh-CN.md) | Maintained |
| 日本語 | `ja` | [README.ja.md](../../README.ja.md) | Maintained |
| 한국어 | `ko` | [README.ko.md](../../README.ko.md) | Maintained |
| Español | `es` | [README.es.md](../../README.es.md) | Maintained |
| Français | `fr` | [README.fr.md](../../README.fr.md) | Maintained |
| Deutsch | `de` | [README.de.md](../../README.de.md) | Maintained |
| Português (BR) | `pt-BR` | [README.pt-BR.md](../../README.pt-BR.md) | Maintained |

### Supporting docs

| Topic | English | 简体中文 |
|-------|---------|----------|
| Setup | [SETUP.md](../SETUP.md) | [SETUP.zh-CN.md](../SETUP.zh-CN.md) |
| Evaluation | [EVAL.md](../EVAL.md) | [EVAL.zh-CN.md](../EVAL.zh-CN.md) |
| API contract | [API_CONTRACT.md](../API_CONTRACT.md) | [API_CONTRACT.zh-CN.md](../API_CONTRACT.zh-CN.md) |
| Glossary | — | [glossary.zh-CN.md](glossary.zh-CN.md) |

Supporting docs for other locales may fall back to English until translated.

## How to add a language

1. Copy `README.md` → `README.<locale>.md` (BCP-47, e.g. `it`, `ru`).
2. Translate prose only. **Do not** translate:
   - commands (`go run ./cmd/sculpt-demo`)
   - flags (`--k`, `--modes`, `--dry-run`)
   - env vars (`SCULPT_*`)
   - package / file paths
   - code fences and BibTeX
3. Keep the shared language switcher at the top of every localized README:
   ```md
   <a href="README.md">English</a> ·
   <a href="README.zh-CN.md">简体中文</a> ·
   <a href="README.ja.md">日本語</a> ·
   <a href="README.ko.md">한국어</a> ·
   <a href="README.es.md">Español</a> ·
   <a href="README.fr.md">Français</a> ·
   <a href="README.de.md">Deutsch</a> ·
   <a href="README.pt-BR.md">Português</a> ·
   <a href="docs/i18n/README.md">More</a>
   ```
4. Add a row to the table above and link from [README.md](../../README.md).
5. Prefer Mermaid diagrams over binary screenshots so locales share one source.

## Style

- Lead with what the reader can **run in 60 seconds**.
- Put badges and language links above the fold.
- One architecture diagram early; details later.
- Mirror section order across locales so anchors stay predictable.

## Ownership

English is the source of truth for behavior and flag semantics.
Locales may adapt metaphors and examples, but must not invent APIs.
