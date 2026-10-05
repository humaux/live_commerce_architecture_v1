#!/usr/bin/env bash
# File: scripts/dev/check-headers.sh
# Purpose: documentation ratchet (owner 2026-10-05: "every file says what it does and what it depends on, so problems can be
#   diagnosed and the project does not rot"). Every source file ADDED or CHANGED since the base must open with a header
#   comment (within its first 25 lines) carrying three labelled lines:
#     Purpose:      what the file owns, in one or two sentences
#     Depends on:   what it calls — packages/modules, SQL functions/tables, external services (Meta, PSP, ECPay, SMTP), env vars
#     Used by:      who calls it (routes, workers, other packages, tests)
#   Go: `// Purpose:` … (before `package`); TS/TSX/JS/MJS: `// Purpose:` …; shell: `# Purpose:` …; SQL migrations: `-- Purpose:` …
#   Files that existed before the base and were not touched are NOT checked (ratchet: the backlog is paid down by the
#   doc-headers unit, never by loosening this script).
# Depends on: git (diff against the base), grep. No network, no secrets.
# Used by: scripts/dev/check-gates.sh (runs in CI and in every unit's self-check); docs/delivery/AGENT-PREAMBLE.md §3.
# Usage: bash scripts/dev/check-headers.sh [base-ref]   (default: merge-base with r3/integration, else HEAD~1). Exit 1 on findings.
# Skips: tests (*_test.go, *.test.*, *.spec.*, tests/**), generated code (*.gen.go, *_gen.go, .next/**, node_modules/**), docs.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
base=${1:-${LC_HEADER_BASE:-}}
if [[ -z "$base" ]]; then
  if git rev-parse -q --verify r3/integration >/dev/null && [[ "$(git rev-parse HEAD)" != "$(git rev-parse r3/integration)" ]]; then
    base=$(git merge-base HEAD r3/integration)
  else
    base=$(git rev-parse -q --verify HEAD~1 || git rev-parse HEAD)
  fi
fi
added=$(git diff --name-only --diff-filter=A "$base" --; git ls-files --others --exclude-standard)
missing=0
while IFS= read -r f; do
  [[ -f "$f" ]] || continue
  case "$f" in
    *_test.go|*.test.*|*.spec.*|tests/*|*/tests/*|*.gen.go|*_gen.go|*/.next/*|*/node_modules/*|docs/*|*.md|output/*) continue ;;
    *.go|*.ts|*.tsx|*.mjs|*.js|*.sh|migrations/*.sql) ;;
    *) continue ;;
  esac
  head=$(head -n 25 "$f")
  # ponytail: two tiers until the doc-headers backlog unit lands (then set LC_HEADERS_STRICT=1 as the default):
  # ADDED files need all three labels; MODIFIED pre-existing files need at least a leading doc comment.
  if [[ "${LC_HEADERS_STRICT:-0}" != 1 ]] && ! grep -qxF "$f" <<<"$added"; then
    grep -qE "^[[:space:]]*(//|#|--)[[:space:]]*[[:alnum:]]" <<<"$(head -n 8 "$f")" && continue
    echo "check-headers: $f has no leading doc comment" >&2; missing=$((missing + 1)); continue
  fi
  for label in "Purpose:" "Depends on:" "Used by:"; do
    if ! grep -qE "^[[:space:]]*(//|#|--|\*)[[:space:]]*${label}" <<<"$head"; then
      echo "check-headers: $f lacks a '${label}' header line (see docs/delivery/AGENT-PREAMBLE.md §3)" >&2
      missing=$((missing + 1)); break
    fi
  done
done < <(git diff --name-only --diff-filter=AM "$base" -- ; git ls-files --others --exclude-standard)
if ((missing)); then echo "check-headers: $missing file(s) without the Purpose/Depends on/Used by header" >&2; exit 1; fi
echo "check-headers: OK (base $base)"
