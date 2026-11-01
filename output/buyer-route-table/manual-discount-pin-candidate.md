# FEAT-MANUAL-DISCOUNT rebased pin candidate: buyerhttp.Routes

Candidate only; pending independent whole-closure review, never author approval.
Authority: integrator REBASE card 2026-11-01 and docs/engineering/contractdrift.md.
PR #394, unit/feat-manual-discount -> r3/integration. Role: dispatched implementation lane,
Codex GPT-6 runtime; exact model identifier/reasoning setting unavailable. No delegation.

## Source binding and base approval

- Integration base: `87c67e33c329a43bb6fc0d51d9160ac4c9826b98` (#396).
- Pre-merge PR head: `65d02773ac43f7663e66bee255eeac70d0e6036b`.
- Base hash: `31499ffb3ce0f159bcacb0f380bcbfe7fb94c220f5b91231d91e5edd840fbbd7`.
- Merged candidate hash: `a816f2699027fc26ca5fa3b7c87258e8b809ebb3c97e24baff797f88b9e9ba30`.
- Base `reviewed` retained verbatim in this PIN note: `output/buyer-route-table/FEAT-LIVE-REPLAY-FIX2-pin-candidate.md`.
  The REBASE card states that #396's base re-pin is reviewed; that statement does not approve this candidate.
- Candidate `reviewed`: `pending independent review (FEAT-MANUAL-DISCOUNT adjustments projection; base re-pin by #396 is reviewed)`.
- Functions: 83 -> 83; declarations: 154 -> 154.
  Added/removed functions: none. Added/removed declarations: none. Both lists stay byte-for-byte equal to base.
  Every other table entry stays equal to base.

## Exact closure delta against the new base

`git diff 87c67e33c329a43bb6fc0d51d9160ac4c9826b98 -- internal/buyerhttp` contains only the following two changes
in `internal/buyerhttp/projections.go`; all other files are equal to the new base.

| Declaration | Source | Delta | Authority |
| --- | --- | --- | --- |
| type orderQuoteResponse | projections.go:285 (field :290) | Adds `Adjustments []pricing.Adjustment` with `json:"adjustments,omitempty"`; other fields only align under gofmt. | contracts/cart-quote-v1.md amendment 0220 requires frozen quote adjustments on buyer order/link reads. |
| func projectOrder | projections.go:366 (assignment :382) | Adds `Adjustments: quote.Adjustments` alongside existing promotion projection. | Copies stored quote money without repricing or accepting client amounts. |

`pricing.Adjustment` is imported, so introduces no same-package closure member. Its fields contain
kind, currency, normalized name and amount only; no staff identifier, reason, buyer contact or secret.
The replay handlers/types/routes and constant-block changes introduced by #396 remain intact.
No new URL, method, authorization, tenant scope or other closure declaration changes against that base.

## Static scanner receipts (E1 only)

- `go run ./scripts/dev/contractdrift` with BASE's pin: exit 1, printed
  `buyerhttp.Routes a816f2699027fc26ca5fa3b7c87258e8b809ebb3c97e24baff797f88b9e9ba30 want 31499ffb3ce0f159bcacb0f380bcbfe7fb94c220f5b91231d91e5edd840fbbd7`.
  Receipt: output/feat-manual-discount/rebase-scanner-before.log.
- Same unchanged scanner after this candidate: exit 0, errors=0. It checks both exact function and
  declaration lists, so unchanged base lists resolve the merged closure without any exemption.
  Receipt: output/feat-manual-discount/rebase-scanner-after.log.
- Source projections.go Git blob: `16c1f89d9d05b99b0fbb460ac953e4f4785d19b5`;
  SHA-256: `68d5cd2348fcdacfcec0314fac5698272041cdb17b0511d6030543a09b51e6e1`.
- Pin file SHA-256: `46dd697c08dd2d67b38c3719844f08814f06641dc41a79996f708e9d0b56d4c3`.

Review must read the entire current closure (83 functions / 154 declarations, enumerated in the pin),
including #396's replay additions, then explicitly approve this candidate. Current PR CI and E4 independent
review remain pending. No scanner code, baseline, allowlist, routes or tests were relaxed. No local tests/server.
