#!/usr/bin/env bash
# scripts/gen-release-notes.sh
# Prepends a new version block to RELEASE_NOTES.md based on git log since the last tag.
# Called by the Makefile release target before bump2version runs.
# Usage: scripts/gen-release-notes.sh <new_version>

set -euo pipefail

NEW_VERSION="${1:-}"
if [[ -z "$NEW_VERSION" ]]; then
  echo "Usage: $0 <new_version>" >&2
  exit 1
fi

DATE=$(date -u +%Y-%m-%d)
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
NOTES_FILE="RELEASE_NOTES.md"

# Collect commits since last tag, excluding bump version commits and agent noise
if [[ -n "$LAST_TAG" ]]; then
  LOG=$(git log "${LAST_TAG}..HEAD" --oneline \
    --no-merges \
    --invert-grep \
    --grep="^Bump version" \
    --grep="^Viewed " \
    --grep="^Ran command" \
    2>/dev/null || true)
else
  LOG=$(git log --oneline --no-merges -20 2>/dev/null || true)
fi

# Build the new block header
BLOCK="## v${NEW_VERSION} — ${DATE}"$'\n'
BLOCK+=$'\n'

if [[ -z "$LOG" ]]; then
  BLOCK+="No changes recorded since last release."$'\n'
else
  # Group into New Commands / Fixes / Improvements / Documentation heuristically
  NEW_CMDS=""
  FIXES=""
  IMPROVEMENTS=""
  DOCS=""
  OTHER=""

  while IFS= read -r line; do
    HASH="${line%% *}"
    MSG="${line#* }"
    case "$MSG" in
      feat:*|"feat("*)
        NEW_CMDS+="- ${MSG#feat: }"$'\n'
        ;;
      fix:*|"fix("*)
        FIXES+="- ${MSG#fix: }"$'\n'
        ;;
      docs:*|"docs("*)
        DOCS+="- ${MSG#docs: }"$'\n'
        ;;
      chore:*|"chore("*|refactor:*|"refactor("*|perf:*|"perf("*)
        IMPROVEMENTS+="- ${MSG}"$'\n'
        ;;
      *)
        OTHER+="- ${MSG}"$'\n'
        ;;
    esac
  done <<< "$LOG"

  if [[ -n "$NEW_CMDS" ]]; then
    BLOCK+="### New Features"$'\n'$'\n'
    BLOCK+="${NEW_CMDS}"$'\n'
  fi
  if [[ -n "$FIXES" ]]; then
    BLOCK+="### Fix"$'\n'$'\n'
    BLOCK+="${FIXES}"$'\n'
  fi
  if [[ -n "$IMPROVEMENTS" ]]; then
    BLOCK+="### Improvements"$'\n'$'\n'
    BLOCK+="${IMPROVEMENTS}"$'\n'
  fi
  if [[ -n "$DOCS" ]]; then
    BLOCK+="### Documentation"$'\n'$'\n'
    BLOCK+="${DOCS}"$'\n'
  fi
  if [[ -n "$OTHER" ]]; then
    BLOCK+="### Other"$'\n'$'\n'
    BLOCK+="${OTHER}"$'\n'
  fi
fi

BLOCK+=$'\n'"---"$'\n'

# Prepend the new block to the existing RELEASE_NOTES.md
EXISTING=""
if [[ -f "$NOTES_FILE" ]]; then
  # Preserve the title line (# Release Notes) and insert after it
  TITLE=$(head -1 "$NOTES_FILE")
  REST=$(tail -n +2 "$NOTES_FILE")
  printf '%s\n\n%s\n%s' "$TITLE" "$BLOCK" "$REST" > "$NOTES_FILE"
else
  printf '# Release Notes\n\n%s\n' "$BLOCK" > "$NOTES_FILE"
fi

echo "RELEASE_NOTES.md updated for v${NEW_VERSION}"
