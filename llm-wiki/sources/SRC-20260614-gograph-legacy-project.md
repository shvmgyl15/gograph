# Gograph Legacy Project

## Metadata

- Source ID: SRC-20260614-gograph-legacy-project
- Original path: raw/inbox/legacy-llm-wiki/project.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy project description document from the previous version of the `gograph` LLM wiki. Explains the purpose of `gograph` as a local, AST-aware Go repository intelligence tool that builds a static `.gograph/graph.json` index to answer code analysis queries instantly. Documents key model features, non-goals, differences with `gopls`, correctness models, package layouts, and metrics.

## Key Claims

- `gograph` is local, AST-aware, entirely offline, does not execute code, and targets Go repositories.
- Non-goals include: multi-language parsing, AI/model API calls, embeddings or SaaS backends, telemetry, replacing compilers/type-checkers, and treating heuristic extractors as proofs.
- Unlike `gopls`, `gograph` provides targeted source slices, cross-graph blast radius calculations, and bundled multi-symbol context queries.
- Correctness model uses:
  - Default: Heuristic duck-typing from AST. Tolerates non-compiling code.
  - Precise: Go type-checker type systems. Requires compilable packages.
- Internal package layout structures:
  - `internal/graph`: Core data models.
  - `internal/parser`: AST parsing.
  - `internal/search`: Algorithmic query search.
  - `internal/cli`: CLI runner.
  - `internal/mcp`: MCP stdio server.
  - `internal/wiki`: LLM wiki generator.

## Entities and Concepts

- `gograph`: AST-aware Go repository intelligence tool.
- `gopls`: Go Language Server (IDE tool contrasted with `gograph`).
- `.gograph/graph.json`: The serialized AST-aware repository graph index.

## Contradictions or Updates

- None identified.

## Derived Pages

- `project.md` (to be created/updated)
