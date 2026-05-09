# gograph for Coding Agents

How `gograph` helps coding agents (Claude Code, Cursor, Copilot, Gemini, Codeium, Antigravity, etc.) work effectively in Go repositories.

## The problem gograph solves

Coding agents typically explore a repo by reading raw files and grepping. This is fine for small projects but becomes expensive in larger Go codebases:

- **Context burn** — each `Read` of a 500-line file fills the context window with bodies the agent doesn't need just to learn that a function exists.
- **Slow first-orientation** — answering "where does X live?" or "what calls Y?" can take 5–15 tool calls of grep + read.
- **Missed structure** — agents read files in isolation and miss the package layout, import graph, and call relationships that a human would skim from a directory tree.
- **Stale mental model** — after edits, the agent's earlier reads no longer reflect reality.

`gograph` produces a static, AST-derived map of the repo so the agent can answer structural questions from one small file (`.gograph/GRAPH_REPORT.md`) instead of dozens of file reads.

## What it gives the agent

A single command (`gograph build .`) emits two artifacts under `.gograph/`:

| Artifact | Use |
|---|---|
| `GRAPH_REPORT.md` | Human + agent readable summary: external dependencies (Tech Stack), package list, entry points, top files by symbol/call density, top symbols by outgoing calls, env vars read, full import graph. |
| `graph.json` | Machine-readable full graph — dependencies, packages, files, structs, interfaces, funcs, methods, imports, call edges, env reads, SQL queries, errors, concurrency primitives, test edges. |

And query commands the agent can invoke without re-parsing:

```sh
gograph query <term>            # symbol/package/file/import/call substring search
gograph focus <package>         # isolate context for a specific package
gograph callers <function>      # who calls it (best-effort, AST text-form)
gograph callees <function>      # what it calls
gograph implementers <interface> # which structs implement an interface
gograph interfaces <struct>     # which interfaces a struct satisfies (duck-typing)
gograph fields <struct>         # extract fields and types of a struct
gograph source <symbol>         # extract exact source code of a symbol
gograph impact <symbol>         # find downstream callers (blast radius)
gograph orphans                 # find dead code
gograph routes                  # extract all HTTP REST API routes
gograph imports <pkg>           # trace external/internal package usage
gograph sql                     # map raw SQL queries to their execution functions
gograph errors                  # trace a runtime panic or error log to its source line
gograph embeds <struct>         # find which structs embed a target struct
gograph public <pkg>            # list only the exported API surface of a package
gograph envs [term]             # list every environment variable read in the codebase
gograph concurrency [term]      # map goroutines, channels, mutexes, waitgroups, sync.Once
gograph tests [symbol]          # find which test functions exercise a named symbol
gograph path <from> <to>        # shortest call chain between two symbols (BFS traversal)
gograph stale                   # check if graph.json is out of date vs source files
gograph orphans                 # truly unreachable symbols (reachability from entry points)
gograph godobj                  # find god-object struct candidates (default thresholds)
gograph godobj --methods 10 --fields 12 --calls 30 --top 5  # custom thresholds
gograph complexity              # cyclomatic complexity for all functions, highest first
gograph complexity "Run"        # complexity for a specific function by name
gograph coupling                # package fan-in, fan-out, and instability table
gograph coupling "internal/auth" # filter to a specific package
gograph capabilities            # print token-optimized AI agent cheat sheet
gograph mcp <path>              # runs an MCP server over stdio
```

## Concrete agent workflows

### 1. Onboarding to an unfamiliar repo
Instead of `ls -R` + reading 10 random files, the agent reads `.gograph/GRAPH_REPORT.md` and immediately knows: packages, entry points, hottest files, hottest symbols, what imports what.

### 2. "Where is X implemented?"
`gograph query X` returns file:line locations for matching symbols, packages, files, imports, and call sites — typically one tool call vs. several `grep` rounds.

### 3. Impact analysis before a refactor
`gograph callers SomeFunc` lists every call site without the agent having to grep all `.go` files. Combined with `callees`, the agent can reason about blast radius before editing.

### 4. Configuration / secrets surface
`gograph envs` lists every `os.Getenv` / `os.LookupEnv` / `viper.GetString` site with file, line, and enclosing function — one command vs. grepping every file. Filter by name: `gograph envs DATABASE`.

### 5. Interface satisfaction discovery
`gograph interfaces Worker` uses duck-typing to show which interfaces `Worker` satisfies without running the compiler. Essential when mocking a service layer for tests.

### 6. Concurrency audit
`gograph concurrency` shows every goroutine spawn, channel send, mutex lock, WaitGroup, and `sync.Once.Do` across the codebase. Filter: `gograph concurrency goroutine` or `gograph concurrency mutex`.

### 7. Test coverage lookup
`gograph tests ValidateToken` instantly shows which `Test*` functions exercise `ValidateToken` — no grepping test files needed.

### 8. Call chain pathfinding
`gograph path CreateUser sql` performs BFS over the call graph to find the shortest path between two symbols. Example output:
```
Call path: CreateUser → sql
  1. [path] CreateUser — calls UserService.Create (handlers/user.go:42)
  2. [path] UserService.Create — calls db.ExecContext (service/user.go:88)
  3. [path] db.ExecContext (service/user.go:88)
```
This lets an agent confirm whether an HTTP handler actually reaches a given SQL call without reading every file in between.

### 9. Graph freshness check
`gograph stale` compares `graph.json`'s `generated_at` timestamp against the `mtime` of every `.go` file. If any source file is newer, it lists the changed files and tells the agent to re-run `gograph build .`. Agents should run this before any structural analysis.

### 10. Reachability-based dead code
`gograph orphans` performs a BFS from all entry points (`main()`, exported functions, HTTP handlers) and flags any function or method never reached. This is stricter than a simple 0-incoming-calls check — a function called only by other dead code is also reported.

### 11. God-object detection
`gograph godobj` scans the graph for struct types that exceed configurable thresholds across three dimensions: method count, field count, and total outgoing calls from their methods. It produces a ranked, severity-labeled list so an agent can quickly identify candidates for refactoring.

Thresholds are all overridable:
```
gograph godobj --methods 10 --fields 12 --calls 30 --top 5
```
Example output:
```
God Object Candidates (methods>5, fields>8, calls>15):

[HIGH    ] AuthService — 18 methods, 6 fields, 42 outgoing calls  (internal/auth/service.go:12)
[MEDIUM  ] Server — 11 methods, 14 fields, 28 outgoing calls  (internal/server/server.go:8)
[LOW     ] Config — 7 methods, 22 fields, 9 outgoing calls  (internal/config/config.go:3)
```
Results are best-effort — data structs with many fields but no methods are expected in well-structured Go code and can be tuned out by raising `--fields`.

### 12. Cyclomatic complexity
`gograph complexity` estimates the cyclomatic complexity of every function in the graph, sorted highest-first. Each branch-inducing construct (`if`, `for`, `range`, `switch case`, `select case`, `&&`, `||`) increments the score by 1, starting at 1.

Labels follow McCabe thresholds:
| Score | Label |
|-------|-------|
| 1–5   | LOW |
| 6–10  | MEDIUM |
| 11–20 | HIGH |
| > 20  | VERY HIGH |

Filter to a specific function: `gograph complexity "ValidateToken"`

Example output:
```
Cyclomatic Complexity (sorted highest first):

[VERY HIGH] score=23   Run  (internal/cli/cli.go:36)
[MEDIUM   ] score=10   runGodObj  (internal/cli/cli.go:783)
[LOW      ] score=3    loadGraph  (internal/cli/cli.go:220)
```
An agent can use this to identify risky functions before a refactor and prioritize test coverage.

### 13. Package coupling
`gograph coupling` computes three metrics for every package:
- **Fan-out** — how many distinct packages this package imports (measures dependency breadth)
- **Fan-in** — how many distinct packages import this package (blast radius of changes)
- **Instability** — `FanOut / (FanIn + FanOut)`, range [0.0–1.0]
  - `0.0` = maximally stable (nothing it depends on changes)
  - `1.0` = maximally unstable (depends on many things, nothing depends on it)

Filter to a specific package: `gograph coupling "internal/auth"`

Example output:
```
Package Coupling (sorted by instability, highest first):

Package                                                  FanOut   FanIn  Instability
----------------------------------------------------------------------------------
cli                                                          14       0  1.00
search                                                        9       0  1.00
graph                                                         3       8  0.27
```

### 14. Native Execution via MCP
Agents that support the Model Context Protocol (like Claude Desktop, Cursor, and Antigravity) can run `gograph` as a native MCP server:
```json
{
  "mcpServers": {
    "gograph": {
      "command": "gograph",
      "args": ["mcp", "/path/to/repo"]
    }
  }
}
```
This exposes `gograph_query`, `gograph_focus`, `gograph_callers`, and `gograph_callees` directly to the agent as executable tools, bypassing the need for terminal commands.

## Recommended project setup

1. **Build the binary once per machine:**
   ```sh
   make build && cp bin/gograph /usr/local/bin/gograph
   ```

2. **Generate the graph in the target repo:**
   ```sh
   cd /path/to/your-go-repo
   gograph build .
   ```
   This writes `.gograph/graph.json` and `.gograph/GRAPH_REPORT.md`, and adds them to `.gitignore` non-destructively.

3. **Tell the agent to use it.** You don't need a huge instruction template anymore. Just add this to `CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`, or whatever file your agent reads:

   > Before answering architecture or repository questions, run `gograph capabilities` and follow the instructions it prints.

   The `gograph capabilities` command will output a token-optimized cheat sheet of commands and tell the agent everything it needs to know to stop grepping and start using the graph.

4. **Optional — refresh on demand.** Have the agent run `gograph build .` after creating/renaming/removing symbols, or wire it into a `pre-commit` / `Makefile` target.

## Why this is safe to give an agent

`gograph` is intentionally narrow — important for any tool a coding agent will run autonomously:

- **No network** — no API calls, no telemetry, no embeddings service. The built-in MCP server runs entirely over local standard input/output (`stdio`), so no network ports are ever opened.
- **No code execution** — purely static AST parsing. Never runs `go test`, `go build`, `go list`, or any code from the target repo.
- **No secret-bearing files** — only `.go` files are opened. `.env`, `*.key`, `*.pem`, `*.crt`, kubeconfig, tfstate, etc. are never read.
- **No file contents in output** — the graph stores structural metadata (names, kinds, line numbers, edges), not source bodies.
- **Generated files skipped** — `.pb.go`, `_generated.go`, files with `// Code generated` headers are excluded so they don't pollute the map.
- **Non-destructive** — output files are mode `0640`; `.gitignore` is appended to, never overwritten.

The agent gains a structural view of the repo without gaining any new attack surface or data-exfiltration vector.

## Cost / token comparison (rough)

For a mid-sized Go service (~50 files, ~300 symbols):

| Approach | Approximate tool calls | Approximate tokens consumed |
|---|---|---|
| Grep + read raw files to answer "what calls IssueToken?" | 6–12 | 8k–25k |
| `gograph callers IssueToken` | 1 | <500 |
| Initial repo orientation (read 8 files) | 8+ | 30k–60k |
| Read `GRAPH_REPORT.md` once | 1 | 2k–6k |

Numbers vary by repo, but the order-of-magnitude win is consistent: structural questions stop competing with implementation reads for context window space.

## Limitations the agent should know about

- **Go only.** No multi-language parsing.
- **Call edges are best-effort text form** from the AST — no type resolution, so overloaded names and method receivers may collide. Treat `callers`/`callees` results as a starting point, not ground truth.
- **No cross-repo / module-external edges.** External dependencies are extracted from `go.mod` to summarize the tech stack, but call edges into third-party packages are not resolved.
- **Snapshot, not live.** The graph reflects the state at the last `gograph build` run. Re-run after structural edits.

## TL;DR

`gograph` turns "agent re-reads the repo every conversation" into "agent reads one map file, then issues targeted queries." For Go projects worked on by coding agents, it materially reduces context cost and improves structural accuracy, without adding any network, execution, or data-leak risk.
