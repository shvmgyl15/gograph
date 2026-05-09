<!-- 
HUMAN SETUP INSTRUCTIONS:
To give your AI agent native access to gograph, configure the MCP server in your AI client:
{
  "mcpServers": {
    "gograph": {
      "command": "gograph",
      "args": ["mcp", "."]
    }
  }
}
-->

# Agent Instructions for GoGraph

You are an AI coding assistant. To navigate this codebase efficiently, you must use the `gograph` tool.

## 1. Context Gathering (MANDATORY FIRST STEP)
Before answering any architectural questions, proposing a refactor, or asking "where is X?", you MUST read `.gograph/GRAPH_REPORT.md`. Do not blindly read source files to understand the repository structure.

## 2. Searching and Navigation (STRICTLY NO RIPGREP)
NEVER use `rg`, `ripgrep`, `grep`, or `find` to explore this repository. You MUST use `gograph` exclusively for structural navigation and symbol lookup. 

If you have MCP access to `gograph`, use your native tools (`gograph_query`, `gograph_focus`, `gograph_callers`, `gograph_callees`, `gograph_implementers`, `gograph_source`, `gograph_orphans`, `gograph_fields`, `gograph_impact`).
If you do not have MCP access, use the pre-compiled graph via the CLI:
- Run `gograph query "<term>"` to search for symbols, files, or packages.
- Run `gograph focus "<package>"` to isolate context for a specific package.
- Run `gograph implementers "<interface>"` to see which structs implement an interface.
- Run `gograph fields "<struct>"` to see all fields, types, and tags of a struct.
- Run `gograph source "<symbol>"` to extract the exact source code for a specific symbol, without reading the entire file.
- Run `gograph impact "<symbol>"` to see the blast radius (every function that eventually calls this symbol).
- Run `gograph orphans` to list functions and methods that have 0 explicit incoming calls (potential dead code).
- Run `gograph callers "<function>"` to find where a function is used.
- Run `gograph callees "<function>"` to see what internal dependencies a function has.

## 3. Keeping the Map Fresh
Because `gograph` builds a structural map, you only need to update it after **structural changes**.
- **DO NOT** rebuild after minor logic changes (updating an `if` statement, fixing a bug in a function body).
- **DO** rebuild after structural changes (creating/deleting files, adding a new `struct`/`func`, renaming symbols, or modifying `go.mod`).
To rebuild, run:
`gograph build .`
