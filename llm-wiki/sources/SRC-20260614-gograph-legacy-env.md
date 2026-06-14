# Gograph Legacy Env

## Metadata

- Source ID: SRC-20260614-gograph-legacy-env
- Original path: raw/inbox/legacy-llm-wiki/env.md
- Source type: Markdown Document
- Received date: 2026-06-13
- Ingest date: 2026-06-13
- Trust level: trusted-project

## Summary

Legacy environment variable reads list. This report contains a static analysis of environment variables checked by the application.

## Key Claims

- Statically detected only one environment variable lookup in the snapshot: `APPDATA` (via `os.Getenv`) in `getClaudeConfigPath` inside `internal/cli/claude_plugin.go`.

## Entities and Concepts

- `APPDATA`: Environment variable representing application data directory (standard on Windows).

## Contradictions or Updates

- This report is a historical generated snapshot. The active list of environment variable reads is generated dynamically in `llm-wiki/env.md` by building the codebase index and running `gograph wiki` (or the `gograph_wiki` MCP tool).

## Derived Pages

- `env.md` (to be regenerated dynamically)
