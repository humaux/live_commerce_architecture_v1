# PR #7 round 08a54057 delivery

- Owner: Codex-3, `unit/tz-audit`, `.worktrees/tz-audit`. Packet: `output/integrator/triage/pr7-08a54057.md`. Initial sync: merge `38534f5e` after `git fetch origin && git merge origin/unit/tz-audit && git merge origin/r3/integration` (trunk `18f82636`, includes #10/#11/#12).
- Bank input fix: `8c8186d2`. Shared WebKit/order producer: unchanged `b6131c41` on `unit/fix-webkit-order-gate`. Final local merge SHA is reported in the handoff; `source-hashes.json` binds the tested source. Primary model inherited (exact runtime identifier unavailable); read-only explorer `gpt-6.1-sol` / high reviewed both source and merge resolution. No delegated writers.

## Findings and implementation

1. **Chromium is PR #7's own regression.** CI job `112945544335`, artifact `gate-browser-order`, `buyer-order-3458936403/browser.log` completed the UTC history check and BO02, then failed in BO06 `rememberCookie` on the intentionally closed UTC context. Local `order-red` reproduced exactly. This is engine-independent and distinct from the shared WebKit layout race. The existing `b6131c41` implementation captures cookie flags and storage before intentional closure, then audits all open/closed snapshots against the complete secret list. No privacy assertion is removed.
2. **A second PR #7 regression was masked by that exception.** After fixing closure, all 24 browser observations passed but the Go harness still required 23. This run is retained as FAIL, not claimed green. Reuse `b6131c41`'s exact named-core validator, including explicit history aliases and the UTC observation, instead of another count-only implementation. With its two layout regressions and PR #7's UTC check, the combined flow has 26 observations and retains exact seven-order database facts.
3. **P2 4212526634:** preserve the buyer's existing device-local `datetime-local` input and ISO conversion; add a visible IANA device-zone hint in en/zh-TW/zh-CN and associate it with the input via `aria-describedby`. The stored proof/deadline remain explicitly labelled store time UTC+8. `storefront-v2.md §C` and `0088 checkout.submit_transfer_proof` accept an instant, editable until confirmed, within `order.created_at - 1 day` to `now + 1 hour`; changing this to store-wall input is unnecessary and would change existing input semantics.
4. Old P1 `4208017617` is already answered/resolved per the integrator. Known foundation issues are covered by the requested trunk merge. No GitHub replies or pushes.

The shared unit is merged **locally** to test the combined result. Two order-spec conflicts were resolved by preserving PR #7's formatter/date/UTC assertions and timezone argument, plus the shared close wrapper and both layout regressions. The integrator can land `b6131c41` first, then take this PR; it is the same implementation, not a competing product fix.

## Evidence and browser coverage

Evidence paths below are stored in `evidence.tar.gz`; `MANIFEST.json` verifies archived members. Raw artifacts remain under the main checkout's `output/tz-audit/round-08a54057/`.

- `ci-order-browser.log`: original CI failure. `order-red/`: matching local closed-context failure. `order-green/`: 24 browser passes followed by stale 23-case Go guard failure.
- `input-zone-red/`: real LA-context input is visible, but its accessible description is empty rather than the required device-zone hint. Product unchanged for this RED.
- `node-input-corrected-{red,green}.log`: supplementary three-locale render assertions calibrated on pre-fix/fixed actual component. Two earlier supplementary logs used an expired fixture and are explicitly excluded in `node-calibration.json`; the independent real-browser RED remains valid.
- `bank-green/`: an added test synchronization bug caught an old-document refresh response before locale navigation, whose body was unavailable afterwards. Fixed by waiting for a positive loaded state in the new document and reading the real scoped BFF; no timeout increase or weakened assertion. This failed attempt is excluded from acceptance.
- `--browser-checkout-offline`: existing `tests/storefront/offline-buyer.mjs` and `tests/foundation/browser_checkout_offline_test.go`. Existing LA and UTC buyers submit through actual controls/PUT/GET; the submitted ISO instant is checked against an independently chosen instant. Existing mobile buyer C re-edits and reloads all three locales. A Taipei context uses the same buyer/order via real history navigation, then edits and reopens it. No new orders/stock fixtures. PG asserts exactly five legal proof submissions for C, while original A's two submissions and all financial/stock assertions remain intact.
- DST coverage is explicit about the contract: LA `2020-03-08 01:30` serializes to `09:30Z`, `03:30` to `10:30Z`; both historical submissions receive real `422 invalid_proof`, not a mocked acceptance. Legal current instants verify submission/echo/re-edit/reload and cross-day LA/Taipei rendering. Three 390px locale screenshots passed horizontal overflow checks and were visually inspected for the new hint.
- `--browser-order` and full `--browser-webkit`: existing `tests/storefront/order-gate.mjs`; all foreign-timezone assertions, both shared layout checks and closed-context audit run together. The WebKit command unsets `LC_WEBKIT_STEPS` so all seven registered steps run.

## Final local gate

All final commands exited **0**, with unchanged source digest `8f3238b72131bbbd286560b1d8cb52142ca8753d2f86d35b4cf583b45e6a0d0a`; browser modes ran one at a time. Bank/order passed, combined order evidence has 26 observations, full WebKit passed all seven steps, and focused Go reports `top-level PASS=2 FAIL=0 SKIP=0 exit=0`. Node passed 734 tests; both typechecks, check-gates, browser-tag vet and the exact-order-evidence unit test passed. Results are in `sequential-gates.json` and the final logs:

- `LC_BROWSER_ENGINE=chromium bash scripts/dev/test-local.sh --browser-checkout-offline`
- `LC_BROWSER_ENGINE=chromium bash scripts/dev/test-local.sh --browser-order`
- `env -u LC_WEBKIT_STEPS LC_BROWSER_ENGINE=webkit bash scripts/dev/test-local.sh --browser-webkit`
- `bash scripts/dev/test-focused.sh '^TestBankTransfer(Lifecycle|K303ProofPrivacy)$'`
- `go test -count=1 -run '^TestBuyerOrderEvidenceCompleteness$' ./tests/foundation`.
- `bash scripts/dev/test-node.sh`; `bash scripts/dev/check-gates.sh`; `pnpm exec tsc --noEmit -p .` in admin and storefront; `go vet -tags browser ./tests/foundation`.

Evidence class: BROWSER + REAL_PG, external services MOCK. G07/full foundation, remaining full PR matrix, provider SANDBOX/LIVE, physical iPhone/Safari and independent K3 acceptance are NOT_RUN locally. K3 pre-review/push remain with the integrator. No timeout bumps, retries, fixture relaxation or production actions.

The planner selects 48 CI modes (`pr-modes.json`); the owner-requested local subset above passed. The optional R04 pinned live-input runner is NOT_RUN when its external binary is absent. Task-owned browser/PG runs have finished; durable evidence is retained. Humaux task `3b7bc30f-fbbf-4cdf-bc76-e3f7c804fd53`; index accepted three supported files/45 entities, with `.mjs` symbols unavailable.
