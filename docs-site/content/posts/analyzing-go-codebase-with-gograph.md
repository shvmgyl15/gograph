---
title: "Analyzing a Go Codebase with gograph — Starting with Itself"
date: 2026-05-23
description: "What happens when you run a Go call graph analysis tool on its own source code? Real output, real insights."
tags: ["golang", "go", "call-graph", "static-analysis", "developer-tools"]
showToc: true
---

## Why analyze gograph with gograph?

The fastest way to understand what a Go codebase analysis tool actually reveals is to point it at something you already know well. So I ran gograph on its own source code.

The numbers came back immediately:

    found 70 Go files to parse
    packages: 16  files: 70  symbols: 518  calls: 5443

518 symbols. 5443 calls. Indexed in under a second. Here is what the call graph actually showed.

## The hotspot analysis

The first question worth asking about any Go codebase is: which functions carry the most load? Not lines of code — incoming calls.

    gograph hotspot --top 5

    1.  132 calls  formatResults  (internal/mcp/server.go)
    2.  116 calls  PrintJSON      (internal/cli/output.go)
    3.  112 calls  loadGraph      (internal/cli/cli.go)
    4.   68 calls  printResults   (internal/cli/cli.go)
    5.   66 calls  sortResults    (internal/search/search.go)

Three of the top five are output formatting functions. That tells you something immediately: in a CLI tool, how results are formatted and printed is the most shared, most depended-upon layer. Any change to formatResults or PrintJSON has the widest blast radius in the codebase. This is the kind of insight that takes hours to reconstruct manually — gograph surfaces it in one command.

## Callers of Build

Tracing who calls the core Build function reveals the full dependency structure at a glance. The output shows Build is called from the CLI entrypoint, the MCP server via a rebuild closure, the baseline comparison engine, and directly from tests. What stands out is that NewServer in the MCP layer calls rebuild over 25 times across different line numbers — meaning the MCP server maintains a long-lived graph and refreshes it at the start of every tool handler.

This is not obvious from reading server.go. The call graph makes it structural.

## Zero orphans

    gograph orphans
    No unreachable symbols found.

Every exported and unexported symbol in the codebase has at least one caller. No dead code. For a 70-file project this is a meaningful signal — it suggests the codebase has been actively pruned and the test suite covers real execution paths.

## What this kind of analysis is actually useful for

Running gograph against your own Go codebase answers questions that are otherwise slow and error-prone:

- Before a refactor: which functions will break if I change this interface?
- During code review: is this new function actually called anywhere?
- Onboarding: what calls what in this unfamiliar service?
- Dependency auditing: which packages does this handler actually pull in?

The call graph does not replace reading code. It tells you where to look.

## Try it

    brew install ozgurcd/tap/gograph

    go install github.com/ozgurcd/gograph@latest

    gograph build .
    gograph hotspot --top 10
    gograph callers YourFunction

The index builds once and queries run in milliseconds against the cached graph.
