# Scrinium Guide

This file was created automatically by Scrinium. It tells you how to use this project's wiki.

## Getting Started

1. Call the `capabilities` tool first. It returns what this server can do, what tools are available, and what governance rules apply.
2. If a project does not have an LLM Wiki yet, call `setup_llm_wiki` to create the standard structure.
3. Call `begin_session` before project changes, then read `index.md` and `agent-rules.md`.
4. Call `finish_session` before reporting completion.

## How to Use the Wiki

The llm-wiki is your persistent memory. Use it constantly — not just at startup.

- **Before making changes:** Read relevant wiki pages to understand existing context, decisions, and rules. Do not assume you know the current state.
- **After making changes:** Update the relevant wiki pages to reflect what you did. If you made a decision, record it. If you changed architecture, update the docs.
- **When you learn something:** If you discover project patterns, constraints, or gotchas that are not documented, write them to the appropriate page so the next agent benefits.
- **Before writing:** Scrinium requires an active session and recorded reads of `index.md` and `agent-rules.md`.
- **After writing:** Scrinium requires `log.md` updates and, for new pages, `index.md` updates before the session can finish.

## Tools

- `capabilities` — Call this FIRST. Returns server info, available tools, and active governance rules.
- `setup_llm_wiki` — Initialize the standard LLM Wiki structure when a project does not have one. Existing pages are left unchanged.
- `begin_session` — Start a tracked work session. Required before wiki writes.
- `session_status` — Show pages read, pages written, and pending maintenance requirements.
- `finish_session` — Verify required log, index, and source-registry updates before completion.
- `lint_llm_wiki` — Check wiki health: missing standard pages, index gaps, log gaps, provenance gaps, and source-instruction risk markers.
- `adopt_llm_wiki` — Inspect an existing manual or non-Scrinium wiki and recommend safe adoption steps.
- `register_source` — Register a raw source and create/update its source summary stub.
- `create_page` — Create a new wiki page only if it does not already exist.
- `move_page` — Rename a wiki page within the wiki root while preserving governance checks.
- `archive_page` — Move an obsolete page under archive/ instead of deleting it.
- `read_wiki_page` — Read any wiki page. No restrictions.
- `update_wiki_page` — Write a wiki page. Blocked for protected files.
- `create_draft` — Propose changes to protected files via the drafts/ directory.
- `append_log` — Append text to a log file. Append-only, bypasses governance except for directly protected files.

## Write Governance

Some files are protected and cannot be modified directly. If you try, you will receive a semantic error explaining what happened and what to do instead. Follow that guidance.

To see which files are protected, call `capabilities` — it returns the live governance rules.
