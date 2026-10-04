# Home COD — scoped surface audit

Mode: Operate. Owner references: comps 04 (order operations) and 09 (buyer checkout); their illustrative data and unsupported controls are not product facts. Preserve existing shells, routing, auth and transaction boundaries.

Audit at 0c6d965c:
- P1: strict merchant order parser rejects the new cod_collect_minor field, preventing order reads.
- P1: buyer/merchant money summaries do not distinguish order total from cash collected including the surcharge.
- P2: repeated generic state/amount presentation hides the next real action; use a single prominent collect amount plus fee breakdown in each task context, not a new dashboard/card layer.
- P2: expected surcharge, cap, carrier and refusal codes must cross the existing BFF/client contracts, without trusting browser amounts as authority.

Direction: compact white operational surface, restrained action accent, clear amount hierarchy and textual states. NT$ whole dollars; explicit Taipei time; manual shipping, never carrier API claims. Preserve existing detailed rows and focus protection; collapse/wrap low-priority fields on mobile. Minimum 44px touch targets in this scope.

Verification: red parser/browser evidence before repair; all eight requested gates; zh-TW / zh-CN / en, 390 / 1366 / 1586 widths. Screenshot deliverables: 1586×992 and 390×844 for zh-TW and en. Only local real-PG plus MOCK provider/identity/browser fixtures; production is NOT_RUN. Final statuses belong in SUMMARY.md.
