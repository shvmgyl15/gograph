---
title: "Agent Workflows"
weight: 4
description: "Recommended operational workflows for developers and AI agents using gograph."
---

To maximize efficiency, reduce latency, and guarantee safety when modifying codebases, we recommend following these standardized workflows. These steps can be programmed into AI system instructions or executed manually.

---

## Workflow 1: Onboarding to a New Repo

When opening an unfamiliar Go repository, use this workflow to get oriented in seconds:

1. **Verify Index Status**:
   ```bash
   gograph stats
   gograph stale
   ```
   If stale or missing, run `gograph build .`.
2. **Find High-Risk Hotspots**:
   ```bash
   gograph hotspot --top 10
   ```
   Identifies the most heavily referenced functions in the codebase.
3. **Map Package Coupling**:
   ```bash
   gograph coupling
   ```
   Gives a high-level table showing package stability and dependencies.
4. **Inspect the Global API Surface**:
   ```bash
   gograph skeleton
   ```
   Exposes every package signature with bodies stripped.

---

## Workflow 2: Safe-Edit Symbol Lifecycle

Before changing the signature or behavior of any function, method, or struct, follow this cycle:

```
[ plan <sym> ] ──► [ context <sym> ] ──► [ Edit Code ] ──► [ build . --precise ] ──► [ review --uncommitted ]
```

1. **Plan first**:
   ```bash
   gograph plan <symbol>
   ```
   This automatically checks for callers, associated tests, and risk profiles (e.g. database transactions or env reads).
2. **Extract Symbol Context**:
   ```bash
   gograph context <symbol>
   ```
   Bundles raw AST info, the exact source block of the target, and immediate dependencies in one call.
3. **Perform the Edit**: Modify the code as needed.
4. **Type-checked build**:
   ```bash
   gograph build . --precise
   ```
   Verifies compilation and computes type-checked interface implementers.
5. **Post-edit review**:
   ```bash
   gograph review --uncommitted
   ```
   Validates complexity drift, test coverage, and security risk introductions before making a commit.

---

## Workflow 3: Package-Level Refactoring

Before splitting, merging, or moving a Go package, execute this check:

1. **Discover all external consumers**:
   ```bash
   gograph dependents <package>
   ```
   Finds every other package that imports your target. A package with high fan-in (low instability) is difficult to change without sweeping breaking changes.
2. **Review public API contracts**:
   ```bash
   gograph public <package>
   ```
   Ensure you know exactly what symbols are exported and consumed externally.
