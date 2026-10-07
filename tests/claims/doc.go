// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package claimsadversarial holds the independent K3 adversarial corpus run (kwc-v1-adversarial.json) against the exported
// kwc-v2 keyword-claim grammar entry points (internal/claims/grammar). It owns no production code.
//
// It never touches a database or Meta, and never persists comment text beyond the committed fixture (I11); an empty
// corpus file is a failure, not a pass (I18).
package claimsadversarial
