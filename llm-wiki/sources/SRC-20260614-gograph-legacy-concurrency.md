# Gograph Legacy Concurrency

## Metadata

- Source ID: SRC-20260614-gograph-legacy-concurrency
- Original path: raw/inbox/legacy-llm-wiki/concurrency.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy concurrency report. Catalogs detected occurrences of Go concurrency primitives such as channel sends, goroutines, once.Do invocations, and waitgroup operations.

## Key Claims

- Catalogs legacy concurrency occurrences across the codebase snapshot including channels, goroutine starts, `once.Do`, and WaitGroup invocations.

## Entities and Concepts

- `Concurrency Primitives`: Channel, goroutine, and sync package operations (e.g. WaitGroup, Once).

## Contradictions or Updates

- This report is a historical generated snapshot. The active list of concurrency primitives is generated dynamically in `llm-wiki/concurrency.md` by building the codebase index and running `gograph wiki` (or the `gograph_wiki` MCP tool).

## Derived Pages

- `concurrency.md` (to be regenerated dynamically)
