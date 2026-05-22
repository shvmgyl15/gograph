# Release Notes

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
- `docs/TODO.md`: marked items 1 (`gograph stats`) and 2 (`gograph changed <git-ref>`) as done.

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