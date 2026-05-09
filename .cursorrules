# Agent Instructions for the GoGraph Repository

You are an AI coding assistant working in the `gograph` repository. To navigate this codebase efficiently, you must use the `gograph` tool itself (dogfooding!).

## 1. Context Gathering (MANDATORY FIRST STEP)
Before answering any architectural questions, proposing a refactor, or asking "where is X?", you MUST read `.gograph/GRAPH_REPORT.md`. Do not blindly read source files to understand the repository structure.

## 2. Searching and Navigation
Do not use `grep` or `find` to locate symbols. Use the pre-compiled graph via the CLI:
- Run `gograph query "<term>"` to search for symbols, files, or packages.
- Run `gograph focus "<package>"` to isolate context for a specific package (e.g. `gograph focus internal/search`).
- Run `gograph callers "<function>"` to find where a function is used.
- Run `gograph callees "<function>"` to see what internal dependencies a function has.

## 3. Keeping the Map Fresh
Whenever you create a new file, rename a symbol, change a method signature, or modify `go.mod`, you MUST run:
`go build -o bin/gograph ./cmd/gograph && ./bin/gograph build .`
This ensures `.gograph/graph.json` and `.gograph/GRAPH_REPORT.md` stay perfectly in sync with reality.
