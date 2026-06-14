# Gograph Legacy Agent Contract

## Metadata

- Source ID: SRC-20260614-gograph-legacy-agent-contract
- Original path: raw/inbox/legacy-llm-wiki/agent-contract.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy agent contract document defining the expected lifecycle, tools mapping, and checklists for AI coding agents working on the `gograph` codebase. Integrates with session auditing checks.

## Key Claims

- Session lifecycle expects:
  1. `gograph session create [word]` to start tracking.
  2. `gograph plan <symbol>` before editing a Go symbol.
  3. `gograph build . --precise` and `gograph review --uncommitted` after editing a symbol.
  4. `gograph session end` to close tracking, and `gograph session audit` to view compliance grade.
- Skipping `plan` or `review` triggers a compliance violation.
- Pre-edit checklist:
  - Check graph freshness via `gograph stale`, rebuild if needed.
  - Run `gograph plan <symbol>`.
  - Run `gograph context <symbol>`.
  - Check `gograph tests <symbol>` to see affected tests.
- Post-edit checklist:
  - Rebuild graph with `gograph build . --precise`.
  - Verify changes via `gograph review --uncommitted`.
  - Regenerate generated wiki pages with `gograph wiki`.
  - Run tests with `make test-coverage`.
  - Update README, usage docs, help text, and CLI capabilities.
  - Ensure parity between CLI flags and MCP tool parameters.
- Tool selection matrix:
  - Find Go symbols: `gograph_query` / `gograph_node` (never `grep`).
  - Read function: `gograph_source` (never `cat`).
  - Find callers: `gograph_callers` (never `grep`).
  - Find fields: `gograph_fields` (never `grep`).
  - Get context: `gograph_context`.
  - Planning: `gograph_plan`.
  - Reviewing: `gograph_review`.
- Compliance grading axes: Plan rule, Review rule, and Efficiency (composability preference to bundle tool calls like using `gograph_context` instead of multiple calls).

## Entities and Concepts

- `gograph session`: Command suite for recording developer actions and grading compliance.
- `gograph plan`: Planning utility for calculating risk and downstream impacts of symbol changes.
- `gograph review`: Review utility for inspecting uncommitted changes against the graph.

## Contradictions or Updates

- The `gograph session` command, `plan` command, and `review` command are referenced as active commands. We should verify their existence in the current codebase or tool help text (we will document them as current or note if they are deprecated/stale in practice).

## Derived Pages

- `agent-contract.md` (to be created/updated)
