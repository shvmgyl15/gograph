# Privacy Policy

**Effective date:** 2026-05-16

## Overview

Gograph is a local, offline static analysis tool. It runs entirely on your machine and does not collect, transmit, or store any personal data or repository code.

## Data Collection

Gograph collects **no data**. Specifically:

- **No code is uploaded.** All AST parsing and graph analysis runs locally against files on your filesystem.
- **No telemetry.** Gograph does not report usage metrics, error reports, or diagnostics to any remote server.
- **No network requests.** Gograph operates fully offline. The only network-adjacent feature is the MCP server, which communicates exclusively over local stdio between `gograph` and the AI client running on the same machine.
- **No accounts or authentication.** Gograph requires no login, API key, or user account.

## Repository Data

When you run `gograph build .`, it reads Go source files in your local directory and writes a graph index (`.gograph/graph.json` and associated Markdown reports) back to that same local directory. This data never leaves your machine.

## Third-Party Services

Gograph has no third-party integrations, analytics, or dependencies on remote services.

## Contact

For questions, open an issue at: https://github.com/ozgurcd/gograph/issues
