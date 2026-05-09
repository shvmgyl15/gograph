# Agent Instructions for the GoGraph Repository (Antigravity & OpenCode)

You are an AI coding assistant (like Antigravity or OpenCode) working in the `gograph` repository. To navigate this codebase efficiently, you must use the `gograph` tool itself (dogfooding!).

## 1. Context Gathering (MANDATORY FIRST STEP)
Before answering any architectural questions, proposing a refactor, or asking "where is X?", you MUST read `.gograph/GRAPH_REPORT.md`. Do not blindly read source files to understand the repository structure.

## 2. Searching and Navigation
Do not use `grep` or `find` to locate symbols. 
If you have MCP access to `gograph`, use your native tools (`gograph_query`, `gograph_focus`, `gograph_callers`, `gograph_callees`).
If you do not have MCP access, use the pre-compiled graph via the CLI:
- Run `gograph query "<term>"` to search for symbols, files, or packages.
- Run `gograph focus "<package>"` to isolate context for a specific package (e.g. `gograph focus internal/search`).
- Run `gograph callers "<function>"` to find where a function is used.
- Run `gograph callees "<function>"` to see what internal dependencies a function has.

## 3. Keeping the Map Fresh
Because `gograph` builds a structural map, you only need to update it after **structural changes**.
- **DO NOT** rebuild after minor logic changes (updating an `if` statement, fixing a bug in a function body).
- **DO** rebuild after structural changes (creating/deleting files, adding a new `struct`/`func`, renaming symbols, or modifying `go.mod`).
To rebuild, run:
`go build -o bin/gograph ./cmd/gograph && ./bin/gograph build .`
