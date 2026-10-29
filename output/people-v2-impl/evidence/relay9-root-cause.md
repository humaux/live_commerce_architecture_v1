# CONT289 relay 9 — second OC setup fix; own-head CI pending

Date: 2026-10-29. Branch/worktree: unit/people-v2-ui / people-v2-ui.
Author E0; syntax E1. No delegation. Humaux/coord/canvas/code-index unavailable.
Required fetch and merge of origin/r3/integration: exit 0, already up to date.
Rollback: 94529539fbdf6c4d00e6b6effea7e4bb6b62cd3d (starting head).

## Exact-head CI and review evidence

Read every failing job of Gates 40831270188, attempt 1, head
94529539fbdf6c4d00e6b6effea7e4bb6b62cd3d, tested merge
62a4fcadbd0b4122fd070d329ca3040172a3f15e:

- bundle08 123309080140: TestBrowserOC exhausts its 1200-second context.
  Five cases spend 240 seconds each after signup/complete 200 and stores GET 200;
  no initial-store POST occurs. Later cases fail because the harness has shut down.
- receipts 123321900203: exactly one failed mode, --browser-password-auth.
- required 123322050579: gate/receipts failure propagates; no independent cause.
- Foundation 40831270133 / 123305794405: success at this starting head.
  bundle07 123309079779: success; customers-billing receipt PASS, test_count 141;
  migration-import receipt PASS, test_count 57. These are mode counts, not browser-only counts.
  This is partial E3 evidence, not whole-PR GREEN or final-head acceptance.

Downloaded original browser artifact using gh run download:
gate-browser-password-auth, artifact 12880886605, run 40831270188.
Local evidence root: relay9-password-artifact/playwright/run.2spNOd0p/.
The OC error-context snapshot shows signed-in first-workspace onboarding; the
Playwright log shows five timeouts, then server connection refusals after shutdown.
The finally/context.close error masks the original wait's stack; there is no trace
(existing suite uses trace off). Do not claim direct trace proof of the final URL.

Original SHA256 (full files retained locally, not committed):

| File | SHA256 |
| --- | --- |
| relay9-bundle08.log | ed8dfbf46fc24f55f26b8ff82c9f07dbe4d2c9e279d1fef435ff00870d889ac7 |
| relay9-receipts.log | efdc3d342db92a867d0a63c7a75e7daf7e5c0ce7c5d88cfc83e1cc45069b6dd5 |
| oc-20261029T071023.651119548/playwright.log | 490261442a4cccbb3b06c95cb218a0edec62376ed20f0a1f827addcdf186dc67 |
| OC zh-TW orders error-context.md | e25614650dd68492f01d500a00c36c7e628e5a448518096e4e667a95fcb6e4ab |

Independent review's final verdict was read from the assigned cxf-rv-289.log.
Its sole P1 is missing causal privacy RED, already supplied by 40803107511 /
123216904686 at d409e4094 (see relay7-red-assertions.log). Restored final-head
GREEN and independent delta review remain required. P2s go to the PR comment:
definite export refusal outside active modal; per-page results labeled total.

## Root-cause inference and bounded repair

The first OC repair (bd45be307) awaited the exact slash-suffixed BFF redirect
after verification. PasswordAuth assigns that redirect; Next config has no
trailingSlash opt-in. The existing password-auth.spec.ts:203 already accepts
the canonical slashless locale root. Successful onboarding with no subsequent
store setup supports an exact-URL mismatch in the newly added wait.

Second targeted OC repair: await the exact same-origin canonical /<locale>
URL with default load completion. Keep the signup response assertion, all
real clicks, authentication/replay assertions, fixtures, thresholds, retries
and timeouts. No product/backend/auth contract edits. This is not a second W6
repair; W6 passed at 94529539 after its separate destination-locator fix.
If the same OC navigation blocker persists, STOP; no third patch.

Additional outside-PLAN path: tests/admin/oc/reauth.spec.ts, already listed
in the PR under explicit same-admin SCOPE PRE-APPROVAL. Generated browser
ownership updated with the official script (53 modes); depmap regenerated
with no diff. node --check and git diff --check exit 0, E1 only.

Acceptance: this push's own Gates + Foundation and independent delta review.
One push; final head and WAIT_RUN IDs posted to PR comment, no evidence-only
follow-up commit. Integrator relays conclusions in a fresh session. No local
tests/builds/servers/browser, tsc, independent runtime or LIVE/production run.
No merge, auto-merge or force push. No Figma/design changes.
