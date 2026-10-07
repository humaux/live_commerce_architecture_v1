// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package msgtemplates owns the merchant message templates of the live console (W2-05B, contracts/live-console-v1.md):
// template ids and versions, publishing, listing, resolving a published version for a send, and the public_safe check
// that a public-reply template must pass (§3.5).
//
// It never generates free text (no LLM, no translation; automated messages are fixed templates), never sends a message
// itself (internal/inbox and the claims-worker adapters do), and never lets a driver message leave the process: only the
// fixed error vocabulary of errors.go does.
package msgtemplates
