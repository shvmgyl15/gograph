# Gograph Legacy Overview

## Metadata

- Source ID: SRC-20260614-gograph-legacy-overview
- Original path: raw/inbox/legacy-llm-wiki/overview.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy codebase metrics overview showing stats (13 packages, 89 files, 678 symbols) and top hotspots like `PrintJSON`, `formatResults`, and `loadGraph`.

## Key Claims

- Codebase metrics at the snapshot: 13 packages, 89 files, 678 symbols, 438 import edges.
- Instability is highest for `scripts` (1.00) and `internal/cli` (0.85).
- Reference pages for details include `architecture.md`, `api-surface.md`, `hotspots.md`, and `env.md`.

## Entities and Concepts

- `gograph wiki`: Tool to generate analytical overview metrics.
- `instability`: Metric representing package coupling ratio of Ce / (Ca + Ce).

## Contradictions or Updates

- These metrics represent a static historical codebase snapshot. Active metrics must be generated dynamically using `gograph build . --precise` followed by `gograph wiki` (or the `gograph_wiki` MCP tool).

## Derived Pages

- `overview.md` (to be regenerated dynamically)
