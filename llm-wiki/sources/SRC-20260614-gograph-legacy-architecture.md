# Gograph Legacy Architecture

## Metadata

- Source ID: SRC-20260614-gograph-legacy-architecture
- Original path: raw/inbox/legacy-llm-wiki/architecture.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy architecture overview detailing package couplings, instabilities, and topological layers.

## Key Claims

- Package dependency flows from higher layers (e.g., `internal/cli` with fan-out 11) to lower stable layers (e.g., `internal/graph` and `internal/rootfind` with fan-out 0).
- Instability metrics catalog the coupling ratio of Ce / (Ca + Ce).

## Entities and Concepts

- `Topological Layers`: Packages grouped by fan-out dependency level.
- `Package Instability`: Stability index ranging from 0.0 (fully stable foundation) to 1.0 (maximally dependent).

## Contradictions or Updates

- These package coupling indices represent a static codebase snapshot. Current coupling structures must be generated dynamically using `gograph wiki` (or `gograph_wiki` MCP tool).

## Derived Pages

- `architecture.md` (to be regenerated dynamically)
