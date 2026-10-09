<!-- Purpose: track the final PR18 K3 sweep batch and approval boundary before Go/DTO changes.
Depends on: pr18-867f650b-sweep packet and current A2 envelope/item contract.
Used by: author and integrator; not final acceptance evidence. -->
# PR18 FINAL sweep

Base867f650b. Items1/4: client-only red→green. Items2/3: inspect A2 seq/cursor evidence first; ask integrator before any Go/DTO change. No push or deploy.

Acceptance: red tests for relogin public guard, bounded deletion after60 arrivals, out-of-order created_at vs seq membership, expired older cursor preserving buffer/selection. Final test-node/adminTS/check-gates/browser-live-console on fixed source, evidence commit and stop.
