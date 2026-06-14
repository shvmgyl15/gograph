# Path Traversal Prevention

## Status

Implemented — committed 2026-06-14 (commit `1f62055`).

## Background

gograph processes data from `graph.json` — a file that is built locally but could theoretically be crafted by a malicious actor in a supply-chain or CI scenario. Several internal functions resolved `File` fields from graph edges into absolute filesystem paths via `filepath.Join(root, c.File)` without validating those fields first. A `File` value of `../../etc/passwd` would escape the project root.

Additionally, the `gograph_boundaries` MCP tool and CLI accepted a user-supplied config file path with no sanitization, and the `gograph_api` MCP tool compiled its git-ref allowlist regex on every invocation.

## Changes

### `internal/search/search.go` — `isSafePathSegment`

Added a path guard helper:

```go
func isSafePathSegment(seg string) bool {
    if seg == "" { return false }
    if strings.Contains(seg, "..") { return false }
    return true
}
```

Applied in `Callers`, `Callees`, and `Source` before any `os.ReadFile` call that resolves a graph-supplied `File` field. Entries with unsafe paths are silently skipped (callers/callees) or emit a `// WARNING:` comment in source output.

### `internal/search/boundaries.go` — Config path validation

Both `Boundaries` and `CreateBoundaries` now validate the config path before any file I/O:

- Reject empty path
- Reject paths containing `..` (traversal)
- Reject paths containing `\` (Windows-style separator, unusual on POSIX)

### `internal/mcp/server.go` — `sanitizeGitRef` and `gograph_boundaries` guard

- `sanitizeGitRef(ref string) error` — extracted helper using the pre-compiled package-level `safeGitRef` regex. Called in the `gograph_api` handler before `git archive`.
- `gograph_boundaries` handler validates the config path with `isSafePathSegment` before passing to `search.Boundaries` (defence-in-depth; `search.Boundaries` also validates).

### `internal/session/session.go` — `redactArgs`

Added `redactArgs(args []string) []string` to sanitize CLI arguments before writing them to the session audit JSONL log. Arguments matching any of: `--config=`, `--session=`, `session_`, `session/`, `.gograph/` are replaced with `***REDACTED***`.

## Invariants

- All `os.ReadFile` calls on user-or-graph-supplied paths must pass `isSafePathSegment` first.
- Git ref inputs to `exec.Command` must pass `sanitizeGitRef` before use.
- Config file paths from MCP tool arguments must pass `isSafePathSegment` before use.
