# Gograph Legacy API Surface

## Metadata

- Source ID: SRC-20260614-gograph-legacy-api-surface
- Original path: raw/inbox/legacy-llm-wiki/api-surface.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy API surface report containing static analysis of all exported package symbols, functions, structures, and their signatures.

## Key Claims

- Catalogs legacy exported Go struct fields, signatures, and interfaces across the codebase snapshot.

## Entities and Concepts

- `Exported Symbols`: Go packages public functions and struct declarations.

## Contradictions or Updates

- This report is a historical generated snapshot. The active list of exported signatures is generated dynamically in `llm-wiki/api-surface.md` by building the codebase index and running `gograph wiki` (or the `gograph_wiki` MCP tool).

## Derived Pages

- `api-surface.md` (to be regenerated dynamically)
