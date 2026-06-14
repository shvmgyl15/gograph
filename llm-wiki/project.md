---
title: Project Identity and Architecture
type: project
status: current
updated: 2026-06-13
sources:
  - SRC-20260614-gograph-legacy-project
---

# Project: gograph

`gograph` is a local, AST-aware Go repository intelligence tool designed for AI coding agents. It parses a Go codebase, builds a static `.gograph/graph.json` index, and answers structural queries (callers, callees, routes, environment reads, error flows, package coupling) instantly without having to repeatedly parse raw source files.

## Core Promise and Features
- **Go-Focused**: Performs static AST analysis on Go repositories.
- **Zero Network Dependency**: Runs completely offline, assuring local security.
- **AST-Aware**: Extracts targeted source slices, traces change impact blast radiuses, and packages full symbol contexts in single queries.
- **Fast Queries**: Exposes findings via CLI and a 50+ tool Model Context Protocol (MCP) server.

## Non-Goals (Hard Boundaries)
- No multi-language parsing.
- No AI/LLM API calls or integrations within the engine.
- No vector embeddings or SaaS backend dependencies.
- Zero telemetry or remote usage logs.
- Does not replace compiler type-checking (heuristic extractors are for navigation, not proof).

## Correctness Models
- **Default (`gograph build .`)**: Uses AST heuristics and duck-typing; tolerates uncompilable or messy code states.
- **Precise (`gograph build . --precise`)**: Uses Go type-checking (`go/types` CHA); requires a compilable package environment.

## Package Architecture Layout
- `internal/graph`: Core data models (lightweight, JSON-serializable, stdlib only).
- `internal/parser`: AST inspection, scope resolution, and metadata extraction.
- `internal/search`: Algorithmic query search, BFS traversals, and filtering.
- `internal/wiki`: Code stats and LLM wiki page generator.
- `internal/cli`: CLI runner, command line parsing, and table output.
- `internal/mcp`: MCP stdio server.
