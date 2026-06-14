# Gograph Legacy Rules

## Metadata

- Source ID: SRC-20260614-gograph-legacy-rules
- Original path: raw/inbox/legacy-llm-wiki/rules.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy rules and constraints document from the previous version of the `gograph` LLM wiki. Contains rules on git disciplines, build procedures, Go version constraint, testing commands, documentation standards, and architectural layout boundaries.

## Key Claims

- Git commits or pushes are forbidden without explicit user commands (such as "push", "commit", "deploy"). General approvals like "go ahead" or "do it" are not sufficient.
- Go code analysis must prioritize the AST-aware `gograph` tool suite (`gograph build`, `gograph plan`, `gograph review`) rather than plain text processing tools like `grep`, `find`, or `sed`.
- Compilations must be done via `make build` rather than direct `go build` to ensure proper versioning via `bump2version` and `ldflags`.
- The project strictly targets Go 1.26.
- Verification requires passing unit, integration, and fuzz tests (`make test-coverage` and `make test-fuzz`).
- Documentation requires updating `README.md`, `docs/coding-agent-usage.md`, CLI capabilities (`internal/cli/cli.go`), and `RELEASE_NOTES.md`.
- CLI and MCP parity must be maintained: every CLI command must have an equivalent MCP tool with matching parameters, filter/sort logic, and output shapes.
- Architectural constraints include local-only operations, zero code execution on target projects, token-efficient concise outputs, and strict package dependency rules (e.g. `internal/search` must not import `internal/cli`).

## Entities and Concepts

- `gograph`: AST-aware Go repository intelligence tool.
- `bump2version`: Tool used for version bumping and Makefile dependency.
- `mcp`: Model Context Protocol server inside `internal/mcp`.

## Contradictions or Updates

- The Go version claim (strictly Go 1.26) was verified against `go.mod` (which specifies `go 1.26`) and is current.

## Derived Pages

- `rules.md` (to be created/updated)
