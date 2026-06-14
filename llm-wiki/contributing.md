---
title: Contributing Guidelines
type: workflow
status: current
updated: 2026-06-13
sources:
  - SRC-20260614-gograph-legacy-contributing
---

# Contributing to gograph

This document outlines the workflows and standards for adding commands, writing MCP tools, maintaining codebase style, and verifying boundaries.

## Adding a New Command
Ensure new commands conform to these design constraints:
- **Token Efficiency**: The command must reduce context usage compared to raw text queries.
- **Local-Only**: Must execute entirely offline with zero network latency.
- **Deterministic**: Output must be predictable.
- **Compositional**: Codebase search features should compose neatly on top of the `*graph.Graph` model.

### Implementation Sequence
1. **Search**: Implement pure algorithmic functions in `internal/search/` (no I/O, operates purely on `*graph.Graph`).
2. **CLI Wiring**: Create the execution handler (`runXxx()`) in `internal/cli/cli.go` and add a matching dispatch block to the `Run()` switch.
3. **MCP Tool**: Register the tool in `internal/mcp/server.go` following the standard MCP tool schema.
4. **Docs & Help**: Update help text constant, update the capabilities list in `gograph_capabilities`, and update the project README.

## CLI/MCP Parity
Every CLI capability must have a corresponding MCP tool:
- Parameter naming and argument types must match (CLI flags → MCP parameters).
- Filtering, sorting, and indexing logics must match.
- Collections in MCP must serialize as empty arrays (`[]`) rather than `null`.
-サーフェイス direct error messages rather than wrapping them.

## Code Style & Import Boundaries
- **Go 1.26**: Strictly use Go 1.26 constructs.
- **Complexity**: Keep function cyclomatic complexity under 20. Decompose functions exceeding this boundary.
- **Boundary Rules**:
  - `internal/graph`: Core structures (no internal package imports; standard library only).
  - `internal/parser`: AST parsing and scope resolution (imports `internal/graph`).
  - `internal/search`: Search/queries (imports `internal/graph`; must not import `internal/cli` or `internal/mcp`).
  - `internal/wiki`: Wiki file generator (imports `internal/graph`, `internal/search`).
  - `internal/cli` and `internal/mcp`: Exposes tools and handles IO (may import all internal packages).

Verify package boundaries via `gograph boundaries`.

## Release Verification Checklist
Before proposing any release or finishing work:
- Run `make test-coverage` and check coverage levels.
- Run `make test-fuzz` to execute fuzz tests.
- Rebuild binary using `make build` (never run `go build` directly, as it bypasses version injection).
- Run `gograph build . --precise` and verify clean graph status.
- Update capabilities list, CLI help text, README, and `RELEASE_NOTES.md` (top block).
