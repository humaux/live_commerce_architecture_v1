# Studio BFF: real signed-session transport evidence

Status: **PASS_LOCAL_BFF_TRANSPORT_ONLY** (2026-09-27), main `b18f977`.
This is not the Studio page/visual/three-locale desktop-mobile STU04 gate. UI
composition remains unapproved. No real provider, Cloud or production operation.

## Ownership and source

Frozen interface `887f79e`, existing Studio backend `ab5c3eb` / main `2be9cd2`.
Source author `media_runtime_source`, ui_worker, actual gpt-6-sol / medium,
branch `commerce/studio-bff-source-20260927` in
`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`: `e3eb554` → main `02637b5`.
Only four files: existing catch-all merchant route, auth readBody helper type/BOM
preservation, narrow query helper, existing Next proxy. No dependency/UI/Go changes.
Independent fixed-diff review `livekit_protocol_impl` (actual model/effort not
exposed) found no confirmed P0/P1, Humaux `5cfcb7f6-8348-44e4-b0b6-132341839c7a`.

Independent test author `media_runtime_tests`, test_worker, actual gpt-6-sol / high,
branch `commerce/studio-bff-tests-20260927` in
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`: `eff0d1f` → main `b18f977`.
Four test/runner paths only. Evidence memory `3753ef73-c201-485c-99e8-68f21ce04006`.
Initial actual run failed on fixture cleanup of identity session events after a
successful flow; cleanup order was corrected, no product constraint was weakened.
Failure log `studio-bff-first-20260927.log` is retained.

## What was proved

Actual Chromium performs signed OIDC login and obtains an HttpOnly session; the
test then exercises packaged Next with that cookie using raw HTTP so framework
normalization cannot hide invalid query/body cases. This uses a Node test runner
with installed Playwright Chromium, not screenshots or a simulated Studio UI.

- Six exact method/path shapes: list/create/read/edit/rehearsal Start/Stop, keyset
  navigation, reopen, replay/conflict and no-prepared rejection against actual PG.
- Read-only access, current store binding, foreign/unlisted store denial, revoked/
  expired sessions, cookie-only auth, browser bearer and forwarding-header stripping.
- Same-origin and CSRF rejection before product writes, raw bare `?`, duplicate/
  unknown/encoded query, encoded sibling paths, GET bodies and unsupported methods.
- Fatal invalid UTF-8 and bounded POST/PATCH; duplicate JSON and BOM reach the
  strict backend unchanged and are rejected. A normal signed login still works
  after the shared readBody change.
- 256 KiB upstream response bound, non-JSON handling, error/header sanitization,
  private/no-store, and working legacy fixture positive control with Studio denied.
- No worker runs in this BFF fixture: Stop returns exactly `cancelled_before_start`;
  readback is CANCELLED/UNOBSERVED, stop requested with zero wire attempts.
  Targets contain only ordinal/provider, not fabricated public success.

The backend is the real Go HTTP handler and PG; faulted upstream responses are
explicit local test seams. Actual API/worker process proof is in the separate
[backend evidence](2026-09-27-studio-backend-acceptance.md).

## Commands and evidence

- Independent `bash scripts/dev/test-local.sh --browser-studio-bff`: exit 0;
  query parser 2/2 and signed browser transport 1/1, foundation 6.482s.
  `/Volumes/data/output/studio-bff-final2-20260927.log`, SHA256
  `a3bbe8af9cf4bba7acce1ece05ed107e5320cf49ab6717acc88c23abde52effa`.
  Independent vet/diff and existing order BFF regression also exit 0.
- Root same selector at `b18f977`: actual exit 0, query 2/2, transport 1/1,
  foundation 5.759s; includes production Next build. Log
  `/Volumes/data/output/studio-root-bff-20260927.log`, SHA256
  `ad6dbd962cf2370094d92f0da4a769ec3cb70a196803199625919b2c4c3f5ea4`.
  Detailed evidence: `output/playwright/studio-bff-20260927T031122.987450000/`.
- Root `pnpm run typecheck:admin`: exit 0;
  `/Volumes/data/output/studio-root-bff-typecheck-20260927.log`.
- Root `bash scripts/dev/test-local.sh --browser-merchant-orders-bff` on same
  source: actual exit 0, foundation 6.092s. Log
  `/Volumes/data/output/studio-root-orders-regression-20260927.log`, SHA256
  `98febcc1276920b1bfa103ce79c2212e1ffcb3c5a495fdb08509cdf8755261e1`.
  Detailed evidence: `output/playwright/merchant-orders-bff-20260927T031154.536333000/`.

After both root commands, their fixture containers were absent; owned Next/IdP/
HTTP processes and pools are closed by their checked harness cleanup. No generated
tracked changes remained. Retain evidence and the still-referenced design-choice
server; do not clean unrelated project resources. A low review note remains:
malformed query can yield private 422 in Next proxy even when identity is disabled,
whereas a normal disabled Studio path yields 404; neither grants access.
