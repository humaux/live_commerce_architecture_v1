# RVFIX2-366 lane report (push 1: tests only)

| Finding | Class | Change |
|---|---|---|
| P1 lifecycle controls hidden in Advanced | a (missing assertion) | tests/admin/ads.spec.ts: `outsideAdvanced()` helper (~L126); approve visible+enabled outside Advanced before the existing `auditDraft` (~L411); ads-pause outside Advanced on ACTIVE (~L452); Meta refusal text outside Advanced on FAILED (~L645). Existing `auditDraft` open step kept. |
| REJECTED draft refusal | not covered | the fixture only drives FAILED (`reject/meta`); REJECTED not drivable without fixture work |

Not run locally (owner rule). Expected RED recorded in output/parity-growth/red.log; details in output/parity-growth/RVFIX2.md. Implementation (AdsDraft.tsx / Ads.tsx) left for the relay's next push.

STATUS: DONE "RED pushed"
