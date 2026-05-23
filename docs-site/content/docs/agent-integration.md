---
title: "AI Agent Integration & Token Reduction"
weight: 3
description: "Integrate gograph with Claude Code, Cursor, and Google Antigravity. Leverage MCP and semantic search to achieve massive token reduction and context savings."
---

gograph is built from the ground up to be an **agent-first tool**. While humans can query the CLI, the primary design target is AI coding agents (such as Claude Code, Cursor, Copilot, Google Antigravity, or OpenCode) that need accurate, structured repository intelligence without context-window inflation.

---

## 🚫 The Problem: Unix Tools are Expensive & Blind

In modern AI agent loops, models are highly habituated to using standard Unix text-processing commands (`grep`, `find`, `sed`) to understand and navigate code. For Go, this is an **inefficient, token-expensive, and error-prone** anti-pattern. 

Go's unique structural features—such as implicit (duck-typed) interfaces, wrapping error chains, and package boundaries—cannot be solved by text patterns.

### Why Generic Unix Tools Fail Agents:

1. **Massive Token Waste**: A broad `grep` forces the model to ingest thousands of lines of search noise (test logs, mock code, comments) just to locate a single method caller. This rapidly consumes context windows and inflates API costs.
2. **Implicit Interfaces**: In Go, a struct satisfies an interface implicitly (no `implements` keyword). To find what implements an interface using Unix tools, an agent must parse method sets and grep receivers across hundreds of files—a complex task that consumes thousands of context tokens and is highly prone to omission.
3. **Wrapped Errors**: Go errors are often wrapped (`fmt.Errorf("user not found: %w", err)`). The original error symbol and its bubble-up path are lost in string wrapping. Generic Unix tools cannot trace this structural relationship.
4. **Structural Blindness**: `grep` matches comments, inactive mock files, test fixtures, and raw strings equally. Agents get overwhelmed by noise and are highly prone to hallucinating architectural relationships.

---

## ⚡ Comparative Analysis: Unix vs. gograph

### Real-World Production Impact
In actual production evaluations conducted by development teams running **35 specialized Claude Code agents** on large Go codebases, utilizing raw `grep` across 80+ endpoints resulted in a **measured hallucination rate of ~12%** on routine calls. 

When replacing generic text searches with gograph's AST-accurate call graph and pruned context, the agent hallucination rate **dropped to 2%**.

---

| Objective | The Unix Way (`grep`, `find`) | The `gograph` Way | Token Cost Comparison | Accuracy |
|---|---|---|---|---|
| **Find callers of a method** | `grep -rn "Update" .` <br>*(Scans mocks, comments, other types)* | `gograph callers UserStore.Update` <br>*(AST-accurate call edges)* | **Extremely high context footprint** <br>*(model reads irrelevant source lines)* vs. **Minimal footprint** *(single-line caller symbols)* | ❌ High hallucination risk (~12% error)<br>🎯 100% Structural Ground Truth (~2% error) |
| **Find interface implementers** | Multi-step grep searches of method receivers and receivers sets | `gograph implementers Connection` <br>*(Duck-typing interface satisfaction)* | **Massive context inflation** <br>*(reading dozens of files to match signatures)* vs. **Minimal footprint** *(only concrete struct types)* | ❌ Highly prone to omissions<br>🎯 100% Structural Ground Truth |
| **Trace wrapped errors** | Grepping for substring matches inside formatting blocks | `gograph errorflow "invalid token"` <br>*(Reversed call graph BFS of `%w` wraps)* | **High context inflation** <br>*(scanning format strings across call paths)* vs. **Minimal footprint** *(structured error flow paths)* | ❌ Misses wrapping context<br>🎯 100% Structural Ground Truth |

---


### Concrete Workflow Example: Modifying a Struct Field

Suppose an agent needs to add a required field to a struct named `Config`. 

* **The Unix Way (Manual Grepping)**:
  1. Agent runs `grep -rn "type Config struct" .` to locate the type definition.
  2. Agent tries to locate every composite literal declaration to see where it is initialized: `grep -rn "Config{" .`.
  3. Because `Config` is a common word, it matches test utilities, local files, config parser variables, and external package docs.
  4. The agent is forced to inspect multiple different files manually to see if the block matches.
  5. **Total cost**: Massive token consumption, 5-10 sequential tool calls, high risk of missing initialization sites inside test suites.

* **The gograph Way (Structural Call)**:
  1. Agent runs a single tool call:
     ```bash
     gograph literals Config
     ```
  2. gograph queries the AST database and returns the exact file, line, and code snippet of every composite literal initialization site (`Config{...}`) in milliseconds.
  3. **Total cost**: Extremely low token cost, 1 tool call, complete structural certainty.

---

## 🔬 Empirical Case Study: gograph Codebase Benchmark

To verify these savings, we ran both generic Unix tools and `gograph` directly against the **`gograph` repository itself** (which consists of **16 Go packages, 70 Go files, 518 AST symbols, and 5,443 call edges**).

Here are the concrete, measured results:

### Case 1: Broad Symbol Queries
An agent needs to locate symbols matching the word `"Symbol"`.
* **The Unix Way (`grep -rn "Symbol" .`)**: Returns **842 matching lines** from comments, markdown guides, local variables, and unrelated documentation blocks, completely flooding the context window.
* **The `gograph` Way (`gograph query Symbol`)**: Returns **exactly 83 structured results** (only actual structs, functions, and active call edges), immediately filtering out **90% of search noise** and saving massive token costs.

### Case 2: Tracking Callers of a Helper Function
An agent needs to track callers of the function `loadGraph`.
* **The Unix Way (`grep -rn "loadGraph" .`)**: Returns **158 matching lines** across comments, markdown docs, function declarations, and call expressions.
* **The `gograph` Way (`gograph callers loadGraph`)**: Returns **exactly 56 structural caller edges** mapped precisely to their AST call sites.

### Case 3: Viewing Function Source Code
An agent needs to read the definition of `normalizeSymbolName`.
* **The Unix Way**: Agent greps for the declaration and then must call `view_file` to read the entire `internal/search/advanced.go` file (or guess line ranges).
* **The `gograph` Way (`gograph source normalizeSymbolName`)**: Instantly returns **exactly 12 lines of code** containing only the clean function block.

---




## Model Context Protocol (MCP): Open & Client-Agnostic

gograph implements a complete native [Model Context Protocol](https://modelcontextprotocol.io) server. Because MCP is an open-standard protocol, **gograph is completely client-agnostic**. 

Any editor, CLI wrapper, custom Python/Node script, or agent framework (such as LangChain, LlamaIndex, or AutoGPT) that supports standard MCP over stdio can connect to and leverage gograph out-of-the-box.

### Starting the Server (Standard I/O)

To start the MCP JSON-RPC server over standard I/O:
```bash
gograph mcp [path]
```
If `.gograph/graph.json` does not exist when the server starts, it automatically runs `gograph build` first to ensure a completely frictionless, zero-barrier client connection.

---

## 🛠️ Client Integration Examples

Since the MCP server communicates over standard stdio streams, it plugs seamlessly into any compliant client environment. Here are configurations for several common developer setups:


### 🧭 Cursor MCP Configuration

To add gograph as an MCP server in **Cursor**:

1. Open Cursor **Settings** (`Cmd + ,` or `Ctrl + ,`).
2. Navigate to **Features** -> **MCP**.
3. Click **+ Add New MCP Server**.
4. Configure the fields:
   - **Name**: `gograph`
   - **Type**: `command`
   - **Command**: `gograph mcp` *(if not globally in PATH, specify the absolute path: `/opt/homebrew/bin/gograph mcp`)*
5. Click **Save**. 

Cursor will automatically start the background stdio session, parse the JSON schemas, and register every `gograph` capability as a native agent tool in Composer and Chat!

### 🌊 Windsurf MCP Configuration

To configure gograph inside the **Windsurf IDE**:

Add the following block to your global or workspace-local `mcp_config.json` file (typically located under `~/.codeium/windsurf/mcp_config.json`):

```json
{
  "mcpServers": {
    "gograph": {
      "command": "gograph",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

Save the file. Windsurf will instantly hot-reload the configuration and expose all `gograph` analytical features to the AI critic and coder loop!

---


### 🤖 Claude Code Integration

gograph includes native automation to integrate directly with Claude Code.

### Automatic Plugin Installation

Run the following command inside your Go repository:
```bash
gograph add-claude-plugin
```

This single command performs three critical setup steps:

1. **Claude Configuration**: Registers the `gograph mcp` server under the local Claude desktop/CLI configuration.
2. **Workspace Steering**: Configures or appends gograph rules to your repository's `CLAUDE.md` to instruct the model to prefer gograph over general grep search.
3. **Pre-Tool-Use Hook Guard**: Installs a Git hook that intercepts incoming tool calls.

---

## The Hook Guard: Blocking general grep

AI agents are highly habituated to executing raw `grep` or `find` commands to look for code patterns. In large Go codebases, this causes two severe issues:
1. **Hallucination**: The agent gets overwhelmed by noise (mocks, test helpers, matches inside unexported package dependencies) and hallucinates.
2. **Token Waste**: Grepping large chunks of source files blows up the context window.

gograph solves this structurally with the **Hook Guard**:

```
[Agent wants to run grep]
           │
           ▼
   .git/hooks/PreToolUse
           │
           ▼
   gograph hook-guard  ◄── Intercepts symbol query
           │
 ┌─────────┴─────────┐
 │                   │
 ▼ (Generic grep)    ▼ (Gograph equivalent)
[BLOCKED]           [ALLOWED]
"Use gograph instead"
```

If the agent tries to run `grep -rn "MyStruct" .`, the hook guard blocks the command with exit code `2` and returns a helpful message:
> *Blocked: Do not use grep to look for Go symbols. Run `gograph query MyStruct` or `gograph callers MyStruct` instead for precise results.*

This structurally forces the agent to use AST-accurate call graphs and dependency queries, dropping the hallucination rate from 12% to under 2%.

---

## Workspace Steering Rules

When `add-claude-plugin` runs, it adds these explicit directives to `CLAUDE.md` to keep the model on track:

```markdown
# Go Codebase Navigation Steering

- NEVER use general bash `grep`, `find`, or `sed` to locate Go symbols, structs, or callers.
- ALWAYS use the native gograph MCP tools (`gograph_query`, `gograph_callers`, `gograph_source`, etc.).
- ALWAYS run `gograph_plan` prior to editing any symbol to discover downstream caller risks.
- ALWAYS run `gograph_build` with `--precise` and run `gograph_review` after editing to verify test coverage.
```
