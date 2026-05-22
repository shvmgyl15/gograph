# Release Notes

## v1.4.59 — unreleased

### Improvements

#### `gograph plan --with-context` / MCP `with_context=true`
When set, `plan` bundles full context (source, callers, callees, role, tests) for every symbol in its `inspect_first` list. Eliminates the N sequential `context` calls that normally follow `plan`.

- CLI: `gograph plan <sym> --with-context` prints the plan then each inspect_first symbol's full context block.
- MCP: `gograph_plan` with `with_context=true` adds `inspect_contexts` array to the response — each entry has `symbol`, `role`, `node`, `source`, `callers`, `callees`, `tests`.
- Works with `--uncommitted` too: `gograph plan --uncommitted --with-context`.

**Token-saving benefit:** Reduces `plan + N×context` (N+1 calls) to a single call. In a typical editing session with 3–5 inspect_first symbols, this saves 3–5 tool calls.

---

#### `gograph context` now includes architectural role
Every `context` response now includes a `role` field — a lightweight architectural classification computed from callers, callees, routes, and SQL already fetched during the call. No extra round trips.

Values: `"HTTP handler"`, `"data access"`, `"entry point"`, `"orchestrator"`, `"coordinator"`, `"utility"`, `"internal"`.

- CLI: displayed on the NODE line as `role: <value>`.
- MCP: included in the `risk` map as `risk.role`.
- `context --uncommitted` also includes `role` per symbol.

**Token-saving benefit:** Eliminates the follow-up `explain` call agents make just to get the architectural role. `context` now answers both "what data do I need?" and "what does this symbol do?" in one call.

---

#### `gograph returnusage <function>` / MCP `gograph_returnusage`
Shows how each caller consumes the return value of a named function. Recorded at parse time by classifying the AST statement wrapping each call site.

Labels: `discarded` (`foo()` standalone), `assigned` (`x := foo()`), `partially_ignored` (`_, err := foo()`), `returned` (`return foo()`), `goroutine` (`go foo()`), `deferred` (`defer foo()`), `passed` (nested inside another call).

- New field `ReturnUsage string` on `graph.CallEdge` (schema-compatible, `omitempty`).
- Parser change: `buildReturnUsageMap` walks the function body at the statement level before the existing call-extraction pass, mapping each `CallExpr.Pos()` to a label.

**Gap this fills:** before changing a return signature (adding an error return, changing a type), an agent needs to know which callers silently discard the return value — those will compile but behave incorrectly. `returnusage` shows this in one call; `callers` alone cannot.

---

#### MCP CLI parity — 17 new MCP tools
Added MCP equivalents for CLI commands that had no MCP counterpart:
`gograph_node`, `gograph_envs`, `gograph_interfaces`, `gograph_tests`, `gograph_hotspot`, `gograph_deps`, `gograph_changes`, `gograph_path`, `gograph_stale`, `gograph_complexity`, `gograph_coupling`, `gograph_mutate`, `gograph_arity`, `gograph_concurrency`, `gograph_fixtures`, `gograph_godobj`, `gograph_skeleton`.

CLI and MCP are now at full functional parity for all query and analysis commands. Remaining CLI-only commands (`check`, `gate`, `snapshot`) are CI/automation tools not appropriate for the MCP surface.

---

### Fix

#### `gograph add-claude-plugin` — unused parameter and stale CLAUDE.md rules
- `installMCPServer` had an unused `home string` parameter; removed.
- `claudeMDBlock` (the rules injected into `~/.claude/CLAUDE.md`) updated to reflect current workflow: `plan with_context=true`, `context uncommitted=true`, and the role field on context responses.

---

### Documentation

- `README.md`: added `plan --with-context`, updated context description to mention role.
- `docs/coding-agent-usage.md`: added `plan --with-context` to cheat sheet; updated `gograph_plan` MCP entry.
- `gograph capabilities` and `gograph --help`: updated context and plan entries.

---

## v1.4.58 — 2026-05-22

### New Commands

#### `gograph dependents <package>`
Returns all packages in the repository that import the named package — the inverse of `gograph deps`.

- Accepts short name (`auth`), path suffix (`internal/auth`), or full import path. Case-insensitive.
- Deduplicates by package: even if multiple files in a package import the target, the package appears once.
- Each result includes the source file and import line for immediate navigation.
- Supports `--json`.
- New MCP tool: `gograph_dependents`.

**Gap this fills:** before any package-level refactor, an agent needs every consumer. `deps` shows what a package imports; `dependents` shows what depends on it. Previously the only workaround was multiple per-file `imports` calls that did not aggregate by package. Zero schema changes — purely a query-time inversion of existing `g.Imports` edges.

---

### Improvements

#### `gograph implementers <iface> --test-only`
Adds a `--test-only` flag to `implementers`. When set, results are filtered to structs defined in test or mock files — equivalent to the former `mocks` command.

- `gograph mocks <iface>` is now a one-line alias for `gograph implementers <iface> --test-only`. Kept for compatibility.
- MCP: `gograph_implementers` gains an optional `test_only` boolean parameter.
- `gograph_mocks` MCP tool retained for compatibility; description updated.

#### `gograph errorflow <term> --no-tests`
Adds a `--no-tests` flag to `errorflow`. When set, skips collecting `RelatedTests` from test files.

- `gograph trace <term> [--no-tests]` is now a one-line alias delegating to `errorflow`. Kept for compatibility.
- MCP: `gograph_errorflow` gains an optional `no_tests` boolean parameter. CLI and MCP behaviour are now identical.

---

### Fix

#### `gograph_orphans` MCP tool now uses reachability analysis
The MCP tool `gograph_orphans` was calling `search.Orphans` (simple 0-incoming-calls check) while the CLI `gograph orphans` was calling `search.ReachableOrphans` (full BFS from `main`, HTTP routes, and exported symbols). The MCP tool now calls `search.ReachableOrphans`, matching CLI behaviour. The tool description was updated to reflect this.

---

### Documentation

- `README.md`: added `dependents`, `literals`, `usages`, `context --uncommitted`, `impact --since`, updated `mocks`/`trace` as aliases, fixed unclosed code block.
- `docs/coding-agent-usage.md`: updated cheat sheet and MCP tools list for all new commands.
- `gograph capabilities` and `gograph --help`: updated all affected command entries.

---

## v1.4.57 — 2026-05-22

### New Flags

#### `gograph callers <sym> --depth N` and `gograph callees <sym> --depth N`
Extends `callers` and `callees` with bounded BFS traversal up or down the call graph.

- **Default** (`--depth 1`, unchanged): direct callers/callees only.
- **`--depth 2`**: callers of callers (or callees of callees), one extra hop.
- **`--depth N`** (max 10): expands N hops, deduplicating by symbol ID across levels.
- Each result carries `depth N` in the Detail field so output is level-labelled.
- Combines with `--no-tests` as before.
- `--json` returns the standard machine-readable envelope.

**Gap this fills:** `callers` was depth 1, `impact` was unlimited. Agents doing PR review or tracing a narrow change radius now have a middle option — "2–3 hops up" without the full blast radius noise.

**New search functions:** `search.CallersDepth` and `search.CalleesDepth` in `internal/search/search.go`. Depth 1 delegates to the original functions (no behaviour change).

---

### Documentation

- `README.md`: added `--depth` examples to the callers/callees usage block.
- `docs/coding-agent-usage.md`: updated cheat sheet callers/callees entries with `--depth N`.
- `gograph capabilities`: updated callers/callees one-liners with `--depth N`.
- `gograph --help`: updated CALL GRAPH section entries with `--depth N`.


---


## v1.4.56 — 2026-05-22

### New Commands

#### `gograph stats`
Returns a compact index health summary in a single zero-parse call. Reads `graph.json` and emits:
- `schema_version` — graph schema version (currently `"2"`)
- `generated_at` — UTC timestamp of the last `gograph build` run
- `packages`, `files`, `symbols`, `calls`, `imports` — core graph counts
- `routes`, `sqls`, `env_reads`, `test_edges` — domain-specific signal counts

No flags required. Supports `--json` for machine-readable output (standard JSON envelope).

**Token-saving benefit:** Agents can confirm the graph is populated and check its version/timestamp in one call, without reading `GRAPH_REPORT.md` or running `gograph stale`. Typical use: run at the start of any analysis session as a sanity check.

**MCP tool registered:** `gograph_stats` — no arguments, returns the same payload.

---

### New Flags

#### `gograph changes --git <ref>`
Extends the existing `gograph changes` command with a git-ref mode. Instead of comparing file modification times against `graph.json`, it runs `git diff --name-only <ref>` and returns symbols in the changed files.

- **Default mode** (`gograph changes`) is unchanged: mtime vs `graph.json` generated_at.
- **Git-ref mode** (`gograph changes --git <ref>`) returns `[MODIFIED]` symbols from files git reports as changed since that ref.
- Accepts any valid git ref: branch name, tag, commit SHA (e.g. `--git main`, `--git HEAD~5`, `--git v1.4.50`).
- Ref is validated against a positive allowlist `[A-Za-z0-9._/\-~^]+` to prevent injection.
- `NEW` and `DELETED` classification is not available in git-ref mode (requires a full baseline graph build from that ref). A note is printed in text mode.
- Supports `--json` for the standard machine-readable envelope (`query` field is set to the ref).

**Token-saving benefit:** Agents can scope symbol changes to a PR branch (`--git main`) or a release (`--git v1.4.50`) without reading files or rebuilding the graph.

---

### Documentation

- `README.md`: added `stats` to the features list and usage block; updated Change Detection bullet with `--git` flag.
- `docs/coding-agent-usage.md`: added `gograph stats`, `gograph changes --git <ref>` to the cheat sheet; `gograph_stats` to MCP tool registry; expanded change detection section.
- `gograph capabilities`: added `stats` and `changes --git <ref>` entries.
- `gograph --help`: added `stats` to INDEXING section; `changes --git <ref>` to CODE QUALITY section.

---

## v1.4.55 — 2026-05-22

### Other

- fix scripts/gen-release-notes.sh
- style: refactor code and tests with consistent indentation and add CLAUDE.md to .gitignore
- +RELEASE_NOTES.md file


---


## v1.4.55 — 2026-05-22

### Other

- fix scripts/gen-release-notes.sh
- style: refactor code and tests with consistent indentation and add CLAUDE.md to .gitignore
- +RELEASE_NOTES.md file


---


## v1.4.54 — 2026-05-18

### New Commands

#### `gograph explain <symbol>`
LLM-ready architectural narrative for any function, struct, or interface. Synthesizes callers (prod vs test split), callees (cross-package ratio), cyclomatic complexity, SQL exposure, env reads, HTTP routes, concurrency primitives, test coverage, interface satisfaction, and struct metadata into a single prompt-ready prose block with an opinionated role classification (e.g. high-traffic leaf utility, service orchestrator, HTTP handler, data transfer object). Designed to collapse 6-8 separate tool calls into one. Supports `--json`.

#### `gograph gate`
First enforcement command in gograph. Reads thresholds from `.gograph.yml` at the repository root and exits with a non-zero code if any configured metric is violated, making it suitable as a CI/CD pipeline step. Does not trigger a rebuild — operates on the already-built `graph.json`. Warns if the graph is stale.

Supported thresholds:

| Field | Type | Description |
|---|---|---|
| `max_complexity` | integer | Maximum cyclomatic complexity for any single function |
| `max_instability` | float | Maximum instability score (0.0–1.0) for any package |
| `max_god_object_methods` | integer | Maximum methods on any single struct |
| `allow_new_orphans` | bool | If false, any increase in unreachable symbol count fails the gate |
| `max_new_coupling_edges` | integer | Maximum new import edges versus the last build |

Each check prints a pass/fail status line with the configured threshold, actual worst value, and location. Baseline orphan and coupling edge counts are captured automatically on each `gograph build` run.

#### `gograph snapshot`
Captures the current architectural metric state under a named label. Snapshots are stored in `.gograph/snapshots/` as JSON files.

Subcommands:

| Subcommand | Description |
|---|---|
| `snapshot save <name>` | Capture metrics (symbols, orphans, god objects, complexity, instability, coupling edges) |
| `snapshot diff <name>` | Compare current graph against a snapshot — marks each metric as improved or WORSE |
| `snapshot list` | Tabular list of all saved snapshots |
| `snapshot drop <name>` | Delete a named snapshot |

Useful for tracking architectural health trends across a sprint, measuring refactor impact, or generating PR-level regression data.

---

### Improvements

- **Graph baseline persistence**: `gograph build` now captures the previous orphan count and coupling edge count before overwriting `graph.json`. This baseline is embedded in the new graph and consumed by `gograph gate` for delta comparisons — no separate state file required.
- **MCP server**: `gograph_explain` registered as a first-class MCP tool alongside all existing tools. Capabilities registry updated for agent auto-discovery.

---

### Documentation

- `README.md`: added `gate` and `snapshot` command examples to the command reference block.
- `docs/coding-agent-usage.md`: added `explain`, `gate`, and all four `snapshot` subcommands to the AI agent cheat sheet and MCP tool registry.
- `gograph capabilities`: updated with `gate` and `snapshot` entries.
- `gograph --help`: updated CODE QUALITY section with `gate` and `snapshot` descriptions.

---

## v1.4.53 — 2026-05-17

### New Commands

#### `gograph explain <symbol>`
*(Initial implementation shipped in this tag — see v1.4.54 for full description.)*

---

## v1.4.49 — 2026-05-16

### Fix

- **MCP auto-build on startup**: `gograph mcp` now automatically runs a graph build when started if no `graph.json` is found. Prevents agents from receiving empty results on a fresh clone without a manual build step.
- **Plugin installer path**: `gograph add-claude-plugin` now uses the absolute project path when writing the MCP server config, preventing path resolution failures when Claude Desktop launches from a different working directory.

---

## v1.4.47 — 2026-05-15

### New Commands

#### `gograph add-claude-plugin`
Single command that performs three installation steps:
1. Registers the MCP server in `claude_desktop_config.json` (Claude Desktop).
2. Injects steering rules into `~/.claude/CLAUDE.md` so Claude knows to use `gograph_*` tools instead of `grep` for Go symbol searches.
3. Installs a smart `PreToolUse` hook at `~/.claude/hooks/gograph-guard.sh` that intercepts `grep`/`rg` calls targeting Go symbols and redirects Claude to the appropriate `gograph` MCP tool.

The hook only intercepts patterns that look like Go identifiers (PascalCase/camelCase, 3+ characters). Legitimate searches in YAML, Markdown, SQL, or comment files are passed through unchanged.

---

## v1.4.45 — 2026-05-14

### New Commands

#### `gograph check`
Static policy checks using `.gograph/checks.json`. Supports `--uncommitted` to include staged changes and `--since <ref>` to include API drift against a baseline git reference.

#### `gograph boundaries`
Enforce package architecture layering constraints using `.gograph/boundaries.json`. Exits non-zero and prints the violating file if any package imports a layer it is not permitted to depend on. `--create` auto-generates a baseline `boundaries.json` from the current import graph.

---

## v1.4.44 — 2026-05-13

### Improvements

- **MCP tool parity**: expanded MCP server to full parity with CLI. All major query commands now registered as MCP tools. Capabilities registry made machine-readable for agent auto-discovery.

---

## v1.4.42 — 2026-05-12

### New Commands

#### `gograph endpoint <route>`
Full vertical slice for a single HTTP endpoint. Composes route resolution, handler symbol lookup, full BFS callee chain, SQL emitted, and env vars read into one response. Supports `--depth N`, `--json`, and `--include-tests`. Accepts a route pattern (e.g. `POST /api/users`) or a handler symbol name. Handler name is preferred — route pattern lookup only resolves flat string literals and fails with grouped routers (Gin Group, Echo Group, Chi).

---

## v1.4.41 — 2026-05-11

### New Commands

#### `gograph api --since <ref>`
Detects breaking API and contract changes between the current graph and a git reference. Identifies removed exported functions, changed signatures, and deleted types.

---

## v1.4.40 — 2026-05-10

### Fix

- **Multiline SQL in markdown report**: SQL queries containing newlines or carriage returns (raw string literals with embedded line breaks) now have whitespace collapsed before insertion into the markdown table. Fixes malformed report output.

---

## v1.4.39 — 2026-05-09

### New Commands

#### `gograph errorflow <term>`
Traces an error string heuristically from its definition up through the call chain to HTTP handlers. Complements `gograph trace` (which traverses backwards from entry points). Uses AST heuristics — no SSA required. Accepts a search term matching error message text.

#### `gograph review <symbol>` / `gograph review --uncommitted`
Post-edit review report. Aggregates the current AST state of modified files and answers: are all callers tested, did complexity increase, were new SQL or env reads introduced, were any interfaces broken. Run after editing, before committing.

---

## v1.4.38 — 2026-05-08

### New Commands

#### `gograph plan <symbol>` / `gograph plan --uncommitted`
Pre-edit change plan. Aggregates callers, tests, blast radius, SQL/env/route exposure into a single checklist before modifying a symbol. Designed to be run before any edit as the primary safety check.

#### `gograph boundaries` *(initial)*
*(See v1.4.45 for full release notes.)*

---

## v1.4.37 — 2026-05-08

### Fix

- **`gograph trace` performance**: rewrote the trace engine to use a precomputed reverse adjacency map and a single reverse BFS per matched error. Previous implementation performed a full forward BFS from every entry point to every error instance, causing combinatorial explosion on large codebases. Now resolves instantly regardless of codebase size.

---

## v1.4.36 — 2026-05-08

### Fix

- **Precise call graph enrichment**: `gograph build --precise` no longer overwrites the heuristic call edges collected in the base build. Enrichment is now additive — type-checked edges are merged in without discarding AST-inferred edges.

---

## v1.4.35 — 2026-05-08

### New Commands

#### `gograph fixtures <pkg>`
Find test helper structs and functions in test files for a given package. Distinct from `gograph tests` (which maps coverage to a symbol) — `fixtures` surfaces the test infrastructure itself.

#### `gograph globals <pkg>`
Find package-level variables, constants, and the functions that mutate them. Extended to include constants in this release.

---

## v1.4.32 — 2026-05-08

### New Commands

#### `gograph source <symbol>` — polymorphic method support
`gograph source` now returns all concrete implementations of a method when the named symbol is defined on an interface. Previously returned only the interface definition.

---

## v1.4.31 — 2026-05-08

### Improvements

- **`--files-only` flag**: all search and query commands now accept `--files-only`, which strips all structural output and returns a flat deduplicated list of file paths. Token-efficient for building file checklists without full context.