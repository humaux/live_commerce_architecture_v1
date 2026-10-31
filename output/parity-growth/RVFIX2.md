# RVFIX2-366 (P1: lifecycle controls hidden in Advanced)

| Finding | Change | RED run | GREEN run |
|---|---|---|---|
| P1 approve hidden | tests/admin/ads.spec.ts outsideAdvanced() + approve visible/enabled assertion before auditDraft (review screen) | PENDING CI | PENDING impl push |
| P1 pause hidden | same file, ads-pause visible outside Advanced on ACTIVE status | PENDING CI | PENDING |
| P1 Meta refusal | same file, refusal text outside Advanced on FAILED draft (REJECTED not drivable by the fixture) | PENDING CI | PENDING |
| P2-6 SHA refresh | PR body/parity-link SHA = head of this push (git rev-parse HEAD after the guard); impl head follows | | |

Implementation (AdsDraft.tsx / Ads.tsx) is not in this push, by card order.
