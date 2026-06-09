# Architectural Guidelines

This document outlines the core architectural philosophy, constraints, and development standards for `gograph`. Any new features, commands, or optimizations must adhere to these principles.

## 1. Core Philosophy & Technical Constraints

`gograph` is designed as a local, AST-aware repository navigation tool tailored specifically for AI coding agents.

- **Local Only:** Graph building and querying must perform **zero network calls**. Source code and telemetry must never be sent to external APIs.
- **No Code Execution:** The tool must statically analyze code. It does not run the target project's tests or binaries.
- **Performance First:** The initial `build` step should be reasonably fast, generating a static `.gograph/graph.json` index. All subsequent `query` and diagnostic commands must be **instantaneous** and execute entirely against this static JSON payload without re-reading or re-parsing source files.
- **Token Efficiency:** The output of CLI commands must be concise and targeted to save LLM context window tokens.

## 2. Correctness Model

- **Default Mode (Heuristic):** The default `gograph build .` uses raw Go AST parsing (`go/ast`, `go/parser`). It uses duck-typing and structural heuristics. It **must** tolerate incomplete, uncompilable, or messy codebases.
- **Precise Mode (Type-Checked):** The `gograph build . --precise` command uses `go/types` for Class Hierarchy Analysis (CHA) and exact interface satisfaction. It is allowed to fail or drop precision if the target codebase does not compile.
- **Navigation Aids, Not Proofs:** Heuristic extractors (such as REST route mappers, SQL query extractors, or test edge mappers) are strictly navigation aids for AI agents. They are not guaranteed to find every dynamic invocation. Do not use hyperbolic language (e.g., "cryptographic proof") to describe AST analysis.

## 3. Package Architecture

The codebase is organized into strict domains:
- **`internal/graph`**: Defines the core data models (`SymbolNode`, `MutationEdge`, `Dependency`, etc.). Keep this lightweight and easily serializable to JSON.
- **`internal/parser`**: Handles AST inspection, scope resolution, and metadata extraction. All logic for extracting structural data (functions, globals, concurrency primitives) belongs here.
- **`internal/search`**: Contains the logic for query processing, graph traversal (BFS), duck-typing, and filtering. This layer operates **only** on the data structures provided by `internal/graph`.
- **`internal/cli`**: Orchestrates the user-facing commands, argument parsing, and CLI formatting.
- **`internal/mcp`**: Handles the Model Context Protocol stdio server wrapper around the search functions.

## 4. Development Standards

- **Go Version:** The project strictly targets **Go 1.26**. Never default to or generate code for older versions.
- **Build Pipeline:** Always compile the binary using `make build`. Never use raw `go build`, as the Makefile handles version injection (`ldflags`) via `bump2version`.
- **Documentation Discipline:** Every new feature, command, or flag must be immediately documented across all relevant targets:
  1. `README.md`
  2. `docs/coding-agent-usage.md`
  3. `gograph capabilities` (`internal/cli/cli.go`)
  4. `gograph --help` (`internal/cli/cli.go`)
  5. `llm-wiki/README.md` — update the generated pages table if a new page type is added
  6. `llm-wiki/agent-contract.md` — update tool selection rules if a new command changes workflow
