// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

package lab

// CodexBaseBody is the stock Codex AGENTS.md for LAB H0 (Python CODEX_BASE_BODY).
const CodexBaseBody = `You are an AI agent completing a professional work assignment inside a workspace.
The assignment message gives the absolute paths that apply to this run.

## Workspace layout

- ` + "`documents/`" + ` -- the assignment's source documents (the data room). Treat it as
  read-only: never modify, move, or delete anything under it.
- ` + "`output/`" + ` -- the deliverable directory. Every deliverable must be written to
  ` + "`output/<filename>`" + ` explicitly (or to its absolute path). Files left anywhere
  else are not part of the delivered work.
- ` + "`scripts/`" + ` -- helper scripts, run as ` + "`python3 scripts/<name>.py`" + `.

## Reading documents

` + "`.docx`, `.pdf`, `.pptx`, and `.xlsx`" + ` files are binary. To read one, run

    python3 scripts/read_doc.py <path>

which prints the extracted text. Plain text, markdown, and code files can be read
directly with shell tools such as ` + "`cat`" + `.

## Doing the work

- Produce exactly the deliverable files the assignment names, with the exact
  filenames it asks for.
- Build binary deliverables (.docx/.xlsx/.pptx) programmatically with Python,
  then verify each produced file is non-empty and re-opens cleanly (for example
  by reading it back with ` + "`python3 scripts/read_doc.py`" + `) before finishing.
- Ground the work in ` + "`documents/`" + `: keep every figure, name, and claim traceable
  to a source document, and say so explicitly where the record is silent.
`

// OpenCodeBaseBodyStub is the offline H0 systemprompt when harvey-labs compose is unavailable.
// Live Python composes from DEFAULT_SKILLS via opencode.render; this keeps the surface valid.
const OpenCodeBaseBodyStub = `You are completing a LAB professional assignment in this workspace.

Write every deliverable under output/. Treat documents/ as read-only source material.
Use python3 scripts/read_doc.py to extract text from binary office documents.
`
