---
title: Agent Workflow Contract
type: workflow
status: current
updated: 2026-06-13
sources:
  - SRC-20260614-gograph-legacy-agent-contract
---

# Agent Workflow Contract

This contract defines the operational workflows, tool matrices, and verification checklist that AI agents must follow when modifying the `gograph` codebase.

## Session Lifecycle
Ensure the following CLI or MCP workflow is executed for session auditing:
1. **Start**: Create a tracked audit session via `gograph session create [word]`.
2. **Pre-edit**: Call `gograph plan <symbol>` before editing any Go symbol.
3. **Rebuild**: Rebuild the graph using `gograph build . --precise` (if compiling) or `gograph build .`.
4. **Post-edit**: Verify the changes using `gograph review --uncommitted`.
5. **End**: Close tracking via `gograph session end` and view the compliance grade using `gograph session audit`.

*Skipping planning or review violates the workflow contract and degrades the session compliance grade.*

## Pre-Edit Checklist
Before modifying any Go code:
- Ensure the graph is fresh (check via `gograph stale`; rebuild if needed).
- Run `gograph plan <symbol>` to inspect targets, risk factors, dependent routes, environment variables, and downstream tests.
- Run `gograph context <symbol>` to view the declaration, callers, and callees in a single, token-efficient action.
- Check `gograph tests <symbol>` to identify which tests will need to be updated.

## Post-Edit Checklist
After editing Go code:
- Run `gograph build . --precise` to compile and regenerate the graph index.
- Run `gograph review --uncommitted` to verify the exact blast radius.
- Rebuild the LLM wiki generated pages using `gograph wiki` so that they reflect the latest structural updates.
- Verify tests using `make test-coverage` and ensure code compiles with `make build`.
- Verify parity between CLI flags and MCP tool parameters (names, typing, filter rules, and JSON shapes).

## Tool Selection matrix

| Task | Preferred Tool | Never Use |
|---|---|---|
| Find Go symbol | `gograph_query` / `gograph_node` | `grep` / `find` |
| Read function body | `gograph_source` | `cat` / raw file reading |
| Find callers | `gograph_callers` | `grep` for string pattern |
| Find struct fields | `gograph_fields` | `grep` / `sed` |
| Pre-edit analysis | `gograph_plan` / `gograph_context` | Guessing / reading full files |
| Post-edit review | `gograph_review` | Skipping review |

Prefer bundled calls (like `gograph_context` or `gograph_plan`) to reduce round-trips and optimize token usage.
