# RVFIX2-366 (P1: lifecycle controls hidden in Advanced)

| Finding | Change | RED run | GREEN run |
|---|---|---|---|
| P1 approve hidden | tests/admin/ads.spec.ts outsideAdvanced() + approve visible/enabled assertion before auditDraft (review screen) | PENDING CI | PENDING impl push |
| P1 pause hidden | same file, ads-pause visible outside Advanced on ACTIVE status | PENDING CI | PENDING |
| P1 Meta refusal | same file, refusal text outside Advanced on FAILED draft (REJECTED not drivable by the fixture) | PENDING CI | PENDING |
| P2-6 SHA refresh | PR body/parity-link SHA = head of this push (git rev-parse HEAD after the guard); impl head follows | | |

## Implementation (relay 1)

RED confirmed: head eb7527d04, run 41251144024, job 124685707869 (bundle:03 `--browser-meta-ads` FAIL; only that mode red).

| Finding | Change | RED run | GREEN run |
|---|---|---|---|
| P1 approve/publish/pause/end hidden | apps/admin/components/AdsDraft.tsx: `ads-actions` block + end-confirm + hints moved before `<details data-testid="ads-advanced">`; ops/remote/facts stay inside | 41251144024 | PENDING next-head CI |
| P1 Meta refusal | AdsDraft.tsx: `refusal` (latest op error_user_msg) rendered as `ads-refusal` for FAILED/REJECTED, outside the disclosure | 41251144024 | PENDING |

Ads.tsx needed no change (it only hosts AdsDraft). `ads-edit` and `ads-edit-advanced` both remain (P2-7, left alone).
Head SHA for the PR body / parity link: the implementation commit's `git rev-parse HEAD` after the guard (WAIT_RUN).
