# gograph

[![Go Report Card](https://goreportcard.com/badge/github.com/ozgurcd/gograph)](https://goreportcard.com/report/github.com/ozgurcd/gograph)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

`gograph` is a local-only CLI tool designed to generate repository structures and improve codebase context awareness. 

It is the **ideal companion tool to pair with AI coding agents** like Claude Code, OpenCode, and Google Antigravity. By feeding `gograph`'s output to these agents, you drastically improve their understanding of your project architecture and dependency graph.

> **Note on Language Support:** I originally built `gograph` specifically for **Golang** because that is what I needed for my own workflows. It currently only parses and maps Go codebases. However, the architecture is extensible! If you want to add support for other languages (Python, TypeScript, Rust, etc.), **contributions are more than welcome.** Please see the [Contributing Guide](CONTRIBUTING.md) to get started.

## Features
- **Local Only:** No network calls or external API dependencies. All analysis is done securely on your machine.
- **Go Focused:** Deeply understands Go project structures, packages, and dependencies.
- **Targeted Focus:** Extract incredibly targeted context for a single package using `focus` to save LLM tokens.
- **Tech Stack Extraction:** Automatically parses `go.mod` to summarize your external dependencies (like `gin` or `pgx`) so agents instantly understand your stack.
- **Concurrency Mapping:** Detects goroutine spawns, channel sends, mutex locks, WaitGroup usage, and `sync.Once.Do` calls across the entire codebase.
- **Interface Satisfaction:** Duck-typing analysis that tells you which interfaces any struct satisfies — without running the compiler.
- **Test Coverage Map:** Links every `Test*` function to the production symbols it exercises — instantly see what's tested.
- **Environment Config:** Surfaces every `os.Getenv` / `viper.Get*` read with file, line, and enclosing function.
- **Fast:** Written in Go for high performance.

## Installation

```bash
go install github.com/ozgurcd/gograph@latest
```

## Usage

**1. Generate the Graph (Run this after every major code change):**
```bash
gograph build .
```
*This instantly generates `.gograph/graph.json` and `.gograph/GRAPH_REPORT.md`.*

**2. Query the Graph (Lightning fast, no re-parsing):**
```bash
gograph query "Auth"              # Search for symbols, files, or packages
gograph focus "internal/auth"     # Generate a highly targeted context for one package
gograph callers "ValidateToken"   # See exactly what functions call ValidateToken
gograph callees "InitServer"      # See exactly what InitServer calls
gograph implementers "AuthService" # See exactly which structs implement an interface
gograph interfaces "UserService"  # See which interfaces a struct satisfies (duck-typing)
gograph fields "User"             # Extract all fields and types of a struct
gograph source "ValidateToken"    # Extract the exact source code for a specific symbol
gograph impact "ValidateToken"    # View the full blast radius (all downstream callers)
gograph orphans                   # List functions and methods with 0 explicit incoming calls
gograph node "UserStruct"         # Get detailed AST info about a specific node
gograph routes                    # Extract all HTTP REST API routes (e.g. GET /api)
gograph imports "redis"           # Find all files that import a specific external package
gograph sql                       # Extract database SQL queries from the AST
gograph errors                    # Map every custom error and panic to its function
gograph embeds "Mutex"            # See exactly which structs embed a target struct
gograph public "internal/auth"    # Filter graph to only show exported public symbols
gograph envs                      # List every environment variable read in the codebase
gograph concurrency               # Map all goroutines, channels, mutexes, and sync primitives
gograph tests "ValidateToken"     # Find which test functions exercise a named symbol
```

**3. Run as an MCP Server (For AI Agents):**
If you want to give your AI agent native tool execution capabilities, `gograph` has a built-in [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server.
```bash
gograph mcp .
```
You can add this to your AI client's configuration (like Claude Desktop or VS Code extensions like Cline) so the AI can run these graph queries autonomously!

## 🤖 Integrating with AI Agents (Cursor, Claude Code, Copilot)

To get the absolute best results from your AI coding assistant, you no longer need to copy-paste giant instruction files. Just add this one-liner to your `.cursorrules`, `CLAUDE.md`, or AI system instructions:

> **System Prompt:**
> Before answering architecture or repository questions, run `gograph capabilities` and follow the instructions it prints.

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
