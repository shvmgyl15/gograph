# Gograph Legacy Contributing

## Metadata

- Source ID: SRC-20260614-gograph-legacy-contributing
- Original path: raw/inbox/legacy-llm-wiki/contributing.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy contributing rules for adding commands/MCP tools, code style requirements, package import boundaries, and a release checklist.

## Key Claims

- Design rules require any new command to pass a 4-check filter: token reduction, local-only operation, deterministic output, and search compositionality.
- Commands must be implemented sequentially: pure function in `internal/search` -> CLI wiring and dispatch in `internal/cli` -> MCP tool registration in `internal/mcp` -> help/capabilities update.
- Every CLI command must maintain parity with an equivalent MCP tool: identical parameter mapping, filter/sorting rules, JSON arrays for MCP collections, and error handling.
- Package boundaries enforce strict hierarchy constraints (e.g. `internal/search` may import `internal/graph`, but never `internal/cli` or `internal/mcp`).
- Code complexity must be kept under 20; functions exceeding this value must be decomposed.

## Entities and Concepts

- `gograph boundaries`: Utility to verify package layout import constraints.
- `MCP Tool Pattern`: Strict declaration pattern for new MCP tools with description fields, required types, and param mappings.

## Contradictions or Updates

- The Go version and packaging boundaries are current and verified.

## Derived Pages

- `contributing.md` (to be created/updated)
