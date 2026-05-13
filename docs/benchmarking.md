# Benchmarking Gograph vs Gopls

To recreate the performance reports demonstrating `gograph`'s latency and token cost advantages over `gopls` (the standard Go Language Server), you can use the built-in benchmark harness located at `scripts/benchmark.go`.

This script queries both tools sequentially, measures their wall-clock latency, approximates their LLM token payload cost using a multiplier heuristic, and outputs a comparative Unicode bar chart.

## Prerequisites

1. You must have `gograph` built locally.
2. You must have `gopls` installed and available in your `$PATH`.

## Standard Execution

To run a basic benchmark that compares `gograph context` against `gopls workspace_symbol`, simply pass the name of the symbol you want to benchmark:

```bash
go run scripts/benchmark.go --sym "YourSymbolName"
```

Example:
```bash
go run scripts/benchmark.go --sym "Run"
```

## High Precision Execution (Simulating LLM Behavior)

By default, the script compares against `workspace_symbol`. However, if you want a more rigorous comparison against `gopls references` (which accurately simulates an LLM attempting to find call sites), you must provide `gopls` with the exact absolute file path, line, and column of the symbol. 

Use the `--gopls-target` flag for this:

```bash
go run scripts/benchmark.go --sym "Run" --gopls-target "/absolute/path/to/repo/file.go:40:6"
```

## Example Output

The script outputs a visual comparison similar to this:

```text
Benchmarking Run...
------------------------------------------------------------
LATENCY:
Run    🔷 █ 63ms
       🔶 ███████ 391ms

TOKEN COST (Estimated):
Run    🔷 ██████████████████████████████████████████████████████████ 29378
       🔶 ██ 1270
```

### Understanding the Results
- **Latency**: `gograph` (🔷) is significantly faster than `gopls` (🔶), generally resolving in a fraction of the time. This is critical for agentic workflows where 3-second tool hangs derail the LLM's thought process.
- **Token Cost**: `gograph` proactively embeds the actual source code of the symbol, its callers, and its callees into a single payload. `gopls` only returns bare file paths, meaning the LLM must execute several subsequent `Read` tool calls to assemble the equivalent context. The `gopls` token cost in this script includes a simulated penalty to account for those subsequent file reads.
