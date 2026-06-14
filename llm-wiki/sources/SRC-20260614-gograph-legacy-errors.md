# Gograph Legacy Errors

## Metadata

- Source ID: SRC-20260614-gograph-legacy-errors
- Original path: raw/inbox/legacy-llm-wiki/errors.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy error sites report containing static analysis of error definitions, constructor invocations, and sentinel errors across the legacy codebase snapshot.

## Key Claims

- Details occurrence of error definitions via `errors.New` or `fmt.Errorf`.
- Catalogs occurrences of sentinel error definitions and standard wrapping patterns.

## Entities and Concepts

- `Sentinel Errors`: Pre-declared error variables checked with `errors.Is`.
- `Error wrapping`: Using `%w` format verbs or explicit wrapper types to retain underlying error contexts.

## Contradictions or Updates

- These error sites represent a static historical codebase snapshot. The active list of error constructors, wrapping patterns, and sentinels must be generated dynamically using `gograph wiki` (or the `gograph_wiki` MCP tool).

## Derived Pages

- `errors.md` (to be regenerated dynamically)
