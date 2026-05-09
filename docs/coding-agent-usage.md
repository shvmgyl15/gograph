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
| `graph.json` | Machine-readable full graph — dependencies, packages, files, structs, interfaces, funcs, methods, imports, call edges, env reads. |

And four query commands the agent can invoke without re-parsing:

```sh
gograph query <term>            # symbol/package/file/import/call substring search
gograph focus <package>         # isolate context for a specific package
gograph callers <function>      # who calls it (best-effort, AST text-form)
gograph callees <function>      # what it calls
gograph implementers <interface> # which structs implement an interface
gograph source <symbol>         # extract exact source code of a symbol
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
The "Environment Variables" section lists every `os.Getenv` / `os.LookupEnv` / `viper.GetString` site with file, line, and enclosing function — useful when the agent is asked about config without reading source.

### 5. Keeping the map fresh
After structural edits (new files, renamed symbols, new packages), the agent re-runs `gograph build .` so its repo map matches the current code. Cheap: parsing-only, no network, no compilation.

### 6. Native Execution via MCP
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

3. **Tell the agent to use it.** Add this to `CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`, or whatever project-instruction file the agent reads:

   > Before answering architecture, dependency, or "where is X?" questions about
   > this repository, read `.gograph/GRAPH_REPORT.md` first. Use it as the repo
   > map before searching raw files. For symbol lookup, use
   > `gograph query "<term>"`, `gograph callers "<function>"`, and
   > `gograph callees "<function>"`. After structural code changes, run
   > `gograph build .`.

   The same instruction is appended to every generated `GRAPH_REPORT.md`, so agents that read the report pick it up automatically even without explicit project rules.

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
