# gograph

[![Go Report Card](https://goreportcard.com/badge/github.com/ozgurcd/gograph)](https://goreportcard.com/report/github.com/ozgurcd/gograph)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

`gograph` is a local AST/type-aware Go repository context indexer for AI coding agents.

![Gograph Demo](gograph-demo.gif)

It builds a compact graph of packages, symbols, calls, routes, config reads, tests, and code-quality signals so agents can navigate Go repositories with fewer raw file reads.

> **Note on Language Support:** I originally built `gograph` specifically for **Golang** because that is what I needed for my own workflows. It currently only parses and maps Go codebases. However, the architecture is extensible! If you want to add support for other languages (Python, TypeScript, Rust, etc.), **contributions are more than welcome.** Please see the [Contributing Guide](CONTRIBUTING.md) to get started.

## Why not use a Language Server (`gopls`)?

While `gopls` has access to similar AST and type data, connecting an AI coding agent to a Language Server is notoriously difficult and inefficient:

1. **Protocol Mismatch:** AI agents operate inside terminal environments. `gopls` communicates via JSON-RPC over `stdin/stdout`. While you can invoke some `gopls` CLI commands, it usually returns raw file coordinates (`file:line:col`). This forces the agent to burn tokens running `cat` or `sed` to actually read the referenced code.
2. **LLM-Optimized Output:** `gograph` doesn't just find coordinates; it physically extracts the exact structural slice (the struct body, the interface, the method) and formats it natively in Markdown. The AI reads exactly what it needs in one shot with zero surrounding file noise.
3. **Graph-Level Diagnostics:** Language servers are built for point-in-time human IDE features (like hover or go-to-definition). `gograph` is built for systemic graph traversal. For example, `gograph trace "parse failed"` performs a reverse-BFS from an error string all the way up the call stack to the HTTP entry point. `gograph impact` calculates the full blast radius of a code change. `gopls` doesn't natively perform graph-traversal diagnostics like this out of the box.

In short: `gopls` is optimized for human IDEs. `gograph` is optimized for terminal-based LLMs trying to save context tokens.

## Features
- **Local Only:** Graph building performs no network calls and sends no source code to external APIs. MCP integration is local stdio-based.
- **Go Focused:** Maps Go project structures, packages, and dependencies using the standard AST.
- **Targeted Focus:** Extract incredibly targeted context for a single package using `focus` to save LLM tokens.
- **Token-Saving Context Bundle:** `context <symbol>` replaces 4–5 separate tool calls — returns node, source, callers, callees, and tests in one response.
- **Hotspot Ranking:** `hotspot` ranks functions by incoming call count so agents know which functions to study first.
- **Code Quality Analysis:** Cyclomatic complexity (`complexity`), god-object detection (`godobj`), and package coupling/instability (`coupling`).
- **Change Detection:** `changes` surfaces new/modified/deleted symbols since the last build without re-reading source files.
- **Dependency Trees:** `deps <pkg> [--transitive]` shows direct or full transitive import closures for any package.
- **Tech Stack Extraction:** Automatically parses `go.mod` to summarize your external dependencies (like `gin` or `pgx`) so agents instantly understand your stack.
- **Concurrency Mapping:** Detects goroutine spawns, channel sends, mutex locks, WaitGroup usage, and `sync.Once.Do` calls across the entire codebase.
- **Interface Satisfaction:** Best-effort duck-typing analysis that tells you which interfaces any struct satisfies — without running the compiler.
- **Test Coverage Map:** Best-effort mapping that links `Test*` functions to the production symbols they likely exercise.
- **Environment Config:** Surfaces every `os.Getenv` / `viper.Get*` read with file, line, and enclosing function.
- **Pathfinding:** `path <from> <to>` finds the shortest call chain between any two symbols via BFS.
- **Dead Code Detection:** `orphans` uses full reachability analysis from entry points — stricter than simple 0-call-count checks.
- **Clean Graph (No Generated Files):** Uses strict line-based detection to automatically exclude generated files like mocks or protobufs.
- **Fast:** Written in Go for high performance.

## Non-goals
- No multi-language parsing.
- No AI/model API calls.
- No embeddings.
- No SaaS backend.
- No telemetry.
- No replacement for compiler/type-checker correctness.
- No guarantee that heuristic extractors find every route, SQL query, test relation, or dynamic call.

## Correctness model
- **Default mode** uses Go AST parsing and best-effort heuristics. It tolerates incomplete or non-compiling repositories.
- **Precise mode** uses type-checked enrichment and requires compilable packages.
- Heuristic extractors such as routes, SQL, tests, and error mapping are navigation aids, not authoritative program analysis.

## Installation

```bash
# MacOS / Linux (via Homebrew)
brew install ozgurcd/tap/gograph

# Or using Go:
go install github.com/ozgurcd/gograph/cmd/gograph@latest
```

## Usage

**1. Generate the Graph (Run this after every major code change):**
```bash
gograph build .
# OR for more precise type-checked analysis (slower, but provides exact dynamic dispatch & interface satisfaction proofs):
gograph build . --precise
```
*This instantly generates `.gograph/graph.json` and `.gograph/GRAPH_REPORT.md`.*

**2. Query the Graph (Lightning fast, no re-parsing):**
```bash
gograph boundaries [--config]     # Verify package architecture constraints using boundaries.json
gograph callees "InitServer"      # See what InitServer calls (with exact source snippet)
gograph callers "ValidateToken"   # See what functions call ValidateToken (with exact source snippet)
gograph complexity                # Cyclomatic complexity for all functions (highest first)
gograph complexity "Run"          # Complexity for a specific function
gograph concurrency               # Map all goroutines, channels, mutexes, and sync primitives
gograph coupling                  # Package fan-in, fan-out, instability table
gograph coupling "internal/auth"  # Filter to a specific package
gograph embeds "Mutex"            # See which structs embed a target struct
gograph envs                      # List every environment variable read in the codebase
gograph errors                    # Map every custom error and panic to its function
gograph fields "User"             # Extract all fields and types of a struct
gograph focus "internal/auth"     # Generate a highly targeted context for one package
gograph godobj                    # Find god-object struct candidates
gograph godobj --methods 10 --fields 12 --calls 30 --top 5  # Custom thresholds
gograph impact "ValidateToken"    # View the full blast radius (all downstream callers)
gograph impact --uncommitted      # Calculate the blast radius of all your uncommitted code changes
gograph implementers "AuthService" # See which structs implement an interface
gograph imports "redis"           # Find all files that import a specific external package
gograph interfaces "UserService"  # See which interfaces a struct satisfies (type-checked if --precise was used)
gograph node "UserStruct"         # Get detailed AST info about a specific node
gograph orphans                   # List functions and methods with 0 explicit incoming calls (dead code)
gograph path "CreateUser" "sql"   # Shortest call chain between two symbols
gograph public "internal/auth"    # Filter graph to only show exported public symbols
gograph query "Auth"              # Search for symbols, files, or packages
gograph routes                    # Extract all HTTP REST API routes (e.g. GET /api)
gograph endpoint "CreateUser"     # Full vertical slice: handler → call chain → SQL → env reads (PREFERRED: use handler name)
gograph endpoint "POST /api/users" # Same but via route pattern (ONLY works for flat routers — fails with Gin/Echo/Chi groups)
gograph source "ValidateToken"    # Extract the source code for a specific symbol
gograph sql                       # Extract database SQL queries from the AST
gograph stale                     # Check if graph.json is out of date vs source files
gograph tests "ValidateToken"     # Find which test functions exercise a named symbol
# --- STATIC GUARDS ---
gograph check                     # Run static policy checks using .gograph/checks.json
gograph check --uncommitted       # Run checks, including uncommitted code
gograph check --since main        # Run checks, including API drift against main
gograph boundaries                # Verify package architecture constraints against boundaries.json
# --- TOKEN SAVERS ---
gograph api --since main          # Identify breaking API and contract changes since a git reference
gograph arity --min 5             # Find functions with many arguments (long parameter list smell)
gograph changes                   # New/modified/deleted symbols since last build
gograph constructors "User"       # Find factory functions returning the named struct
gograph context "ValidateToken"   # Node + source + callers + callees + tests in ONE call
gograph deps "internal/auth"      # Direct import dependencies of a package
gograph deps "internal/auth" --transitive  # Full transitive closure
gograph fixtures "internal/auth"  # Find test helper structs and functions in test files
gograph globals "internal/auth"   # Find pkg-level vars, consts, and functions mutating them
gograph hotspot                   # Top 10 most-called functions (where to focus first)
gograph hotspot --top 20          # Expand to top 20
gograph mocks "AuthService"       # Find structs implementing an interface in test/mock files
gograph mutate "User.Status"      # Find functions that mutate a specific struct field
gograph plan "ValidateToken"      # Generate an operational change plan (callers, tests, risk profile) before editing a symbol
gograph plan --uncommitted        # Generate a change plan for all currently uncommitted modified symbols
gograph review "ValidateToken"    # Generate a post-edit final review report for a modified symbol
gograph review --uncommitted      # Generate a post-edit final review report for all uncommitted changes
gograph schema "users"            # Find structs mapped to a database table/schema via tags
gograph skeleton                  # Output the whole repository's API signatures (bodies stripped)
gograph trace "parse failed"      # Trace an error string backwards to entry points
gograph errorflow "invalid token" # Trace an error's path from definition up to HTTP handlers (heuristic, NO SSA)
# endpoint: full vertical slice for one HTTP endpoint. IMPORTANT: route patterns only work with flat routers.
# With Gin/Echo/Chi Group() routing, the prefix is lost in the AST. Use handler symbol name instead.
gograph endpoint "CreateUser"     # RECOMMENDED: always works regardless of routing style [--depth N] [--json]
gograph endpoint "POST /api/users" # route pattern: only works if path is a flat string literal (no Group() prefix)

**3. Architecture Boundary Enforcement:**
You can configure `gograph` to actively enforce clean architecture by defining boundaries. Create a `.gograph/boundaries.json` file in your root directory:
```json
{
  "layers": [
    {
      "name": "domain",
      "packages": ["internal/domain/**"],
      "may_import": []
    },
    {
      "name": "handler",
      "packages": ["internal/handler/**"],
      "may_import": [
        "internal/service/**",
        "internal/domain/**"
      ]
    }
  ]
}
```
*Note: Standard library imports are implicitly allowed. Imports within the same layer are also implicitly allowed.*

Run the enforcement check:
```bash
gograph boundaries
```
*If a violation is found (e.g., `handler` imports `internal/repository` directly), it will exit with code 1 and print the exact file that violated the rule. Extremely useful for CI/CD or Agent `CLAUDE.md` instructions!*

**4. Agent JSON Integration:**
All search and query commands support the `--json` flag to emit strictly formatted, machine-parseable JSON envelopes.

For specific instructions on how to configure agents to use `gograph`, read the [Claude Code Integration Guide](docs/claude-code-integration.md).

```bash
gograph callers "ValidateToken" --json
```
*Returns: `{"schema_version": "1", "command": "callers", "status": "ok", "count": 2, "results": [...]}`*


**5. Run as an MCP Server (For AI Agents):**
If you want to give your AI agent native tool execution capabilities, `gograph` has a built-in [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server.

To install the plugin automatically on macOS, Windows, or Linux, run:
```bash
gograph add-claude-plugin
```

This single command does three things:
1. **Registers the MCP server** in `claude_desktop_config.json` (Claude Desktop) so `gograph` tools are available to Claude natively.
2. **Injects steering rules** into `~/.claude/CLAUDE.md` — Claude reads this automatically and knows to use `gograph_query` instead of `grep` for Go symbol searches.
3. **Installs a smart `PreToolUse` hook** at `~/.claude/hooks/gograph-guard.sh` — this intercepts `grep`/`rg` calls targeting Go symbols and redirects Claude to the appropriate `gograph` MCP tool, saving tokens and improving precision.

The hook is **smart**: it only blocks grep when the search pattern looks like a Go identifier (PascalCase/camelCase, 3+ chars). Legitimate raw-text searches in YAML, Markdown, SQL, or comment files are allowed through unchanged.

For Claude Code (CLI) users, also run:
```bash
claude mcp add gograph -- gograph mcp .
```

You can also run the MCP server manually over stdio:
```bash
gograph mcp .
```

## 🤖 Integrating with AI Agents (Cursor, Claude Code, Copilot)

To get the best results from your AI coding assistant, run `gograph add-claude-plugin`. It automatically configures everything:
- MCP server registration for native tool access
- `CLAUDE.md` rules that steer Claude to use `gograph` instead of `grep`
- A `PreToolUse` hook that enforces Go symbol lookups go through `gograph`

If you prefer manual setup, add this to your `.cursorrules`, `CLAUDE.md`, or AI system instructions:

> **System Prompt:**
> Before answering architecture or repository questions, inspect the available `gograph_*` MCP tools for the current project and use them instead of grep/find. Each project ships its own gograph MCP server; pick the matching one. If using the CLI directly, run `gograph capabilities` first.

## Example Output

When you run `gograph build .`, the generated `GRAPH_REPORT.md` gives your AI a condensed, highly-dense context map that looks like this:

**External Dependencies (Tech Stack)**
| Module | Version |
|--------|---------|
| `github.com/gin-gonic/gin` | `v1.9.1` |
| `github.com/jackc/pgx/v5` | `v5.5.5` |

**Important Symbols (Top by outgoing calls)**
| Symbol | Kind | File | Line | Calls out |
|--------|------|------|------|-----------|
| `(Server).Start` | method | `server.go` | 42 | 18 |
| `ValidateAuth` | function | `auth.go` | 12 | 14 |

## Contributing

We love pull requests! See the [CONTRIBUTING.md](CONTRIBUTING.md) file for guidelines on how to build, test, and contribute to the project. If you are adding support for a new language, please open an issue first to discuss the design.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
