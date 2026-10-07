<!-- Purpose: W3-U1b current Secure-cookie and feedback-visibility CI repair handoff.
Depends on: merged trunkf8f01b74, TLS source checkpointf24adec9, focused evidence/source-manifest.
Used by: integrator review/GitHub rerun; no production/provider acceptance. -->
# W3-U1b pick-list UI — CI repair

- Branch `unit/w3-u1b-picklist-ui`; trunk `f8f01b74ad80fc26223751ce3bfdc2cb849df6c6` merged as `dc9cd752`. Source checkpoint `f24adec90726d16514bc52e86c192abbdd480aaf` adds the TLS test setup. Final SHA is the commit containing this file (main commit-receipt.json/handoff).
- Supersedes author1b658497 / int97a4b184 CI37501909308. Prior compact controls, visible mobile page-selection and MOU07 work remain. No runtime backend, schema, dependencies, cookie/security fences or test thresholds changed.
- Main evidence root `/Volumes/data/live_commerce_architecture_v1/output/w3-u1b-picklist-ui/ci-37501909308/`. Source binding `41683bbb443dd78908f72b98870ee1f96575db9afdb0bc46c3d31cf8a7a8616a`,22 files in source-manifest.json.
- Parent Codex GPT-6 owns final UI/review; separate ui_worker `codex-w3-u1b-ui-sub-auth` owns only picklist.spec.ts/fixture.mjs/model.test.ts at checkpointf24; inherited configured reasoning is UNKNOWN. Independent read-only review by explorer `codex-product-media-v2-ui-sub-checkout-smoke`, artifact review-result-visibility.md. Never push/release merge.

## Changes

The test installed Secure __Host session/CSRF cookies over HTTP127. APIRequestContext omitted them, so the missing-CSRF request got401 before its403 fence. Test-only picklistTLS wraps the owned loopback Next server with an ephemeral certificate, preserves public Host/Origin and automatically supplied cookies, and sets forwarded HTTPS. Browser/API contexts use HTTPS and accept only the fixture's self-signed certificate. No Cookie header injection or Secure=false; all original403/422/no-backend-dispatch assertions remain.

After authentication was corrected, the unchanged browser spec exposed a hidden UNKNOWN notice. PickList's compact disclosure had no input from batch feedback; arriving/restored uncertainty or results could stay hidden without selected rows. CvsBatch now reports derived busy/confirmation/result/unknown/error/storage feedback through a stable optional callback; PickList automatically opens when feedback exists. Empty initial tools remain compact; deliberate manual collapse is possible. Journal persistence, command keys, status recovery, caps and zero automatic resend are unchanged.

## Commands / exits / evidence

| Check | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/picklist-model.test.ts` before/after TLS |1→0| worker/red.log12PASS/1FAIL401!=403 → worker/green.log13/13 |
| Same focused Node command, root final |0| root-node-final.log13/13; automatic Secure TLS cookie, noCSRF403, anonymous401, validCSRF204, same cookie HTTP401, Host/proto and cert cleanup |
| `pnpm run typecheck:admin` |0| root-types.log |
| `pnpm run build:admin` with synthetic platform/admin/contact and identity/fixture disabled |0| root-build-feedback.log; production standalone packaged |
| `bash -c 'set -euo pipefail; source scripts/dev/test-lock.sh; lc_lock_acquire 600; trap lc_lock_release EXIT; node --test --test-reporter=spec --experimental-strip-types tests/admin/picklist.spec.ts'` before/after feedback fix |1→0| root-browser-feedback-red.log → root-browser-final-green.log; runtime-feedback-red/runtime-final-green |
| `LC_BROWSER_MERCHANT_ORDERS_UI_ACCEPTANCE=1 LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=700s bash scripts/dev/test-focused.sh '^TestBrowserMerchantOrdersUIFocused$'` |0| root-mou07-green.log; actual MOU07 + independent PG read-only facts, runtime-mou07 |
| `bash scripts/dev/check-gates.sh`; `git diff --check` |0 each| root-gates-final.log73 modes/header/vet/format |
| node syntax checks (fixture, spec and model test) |0| worker evidence |
| Standalone strict picklist spec/model tsc |1 baseline| worker/spec-types.log and spec-types-baseline.log: same7 existing TS2339 never[] fixture-array errors before/after; no new diagnostic or threshold change |

E3 is limited to these focused checks. The original picklist spec passes3locales×1440/390, real selection/print/download/confirmation clicks,403/422 zero-dispatch,500 cap/CVS100 refusal, UNKNOWN visible before and after reload, zero automatic resend, GET-only recovery, scoped order navigation and permission reduction.11 click-ledger entries are PASS. MOU07 still uses its unchanged viewport threshold and real page-checkbox clicks. Other MOU/native cases are explicitly NOT_RUN by the focused Go entry.

The MOU07 run refreshed already tracked synthetic orders-v2-fix screenshots/ledger; git add-A includes those evidence files. Main runtime copies retain the focused evidence. Only owned temporary TLS certs/listeners/Next/browser/PG processes were created; harness cleanup completed. No real labels, providers, payments or production data.

## CI gates — integrator runs on GitHub

- `bash scripts/dev/test-local.sh --browser-picklist`
- `bash scripts/dev/test-local.sh --browser-merchant-orders-ui`
- `bash scripts/dev/test-local.sh --browser-cvs`
- `bash scripts/dev/test-local.sh --browser-click-sweep`
- `bash scripts/dev/test-local.sh --browser-visual-lint`

## NOT_RUN / limits

Full modes, full foundation, native headed proof, broad click-sweep/visual-lint and Linux/Xvfb are NOT_RUN locally. Current GitHub rerun is required for full W3 acceptance. Local focused specs were expressly authorized by the latest owner message and executed serially with the current heartbeat lock. No test-local mode or multi-mode batch ran locally. Standalone spec fixture typing retains the seven documented baseline errors; app strict types/build pass.

LC-U1 is not this thread's unit and remains untouched. PM-U1778805b is separately delivered; W4-U1 remains queued until PM-U full green and the approved non-sensitive descriptor fields are frozen.
