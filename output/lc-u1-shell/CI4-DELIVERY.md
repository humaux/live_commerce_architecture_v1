<!-- Purpose: Deliver LC-U1 real remaining CI repairs with final-source focused red/green evidence and honest full-CI boundaries.
Depends on: code 5012ce96, trunk f8f01b74, frozen Console/Studio assertions, signed MOCK IdP/REAL_PG fixtures and local MOCK media provider.
Used by: Integrator pushes/full GitHub reruns and non-author review; W2 stays gated on full LC-U1 acceptance. -->
# LC-U1 CI4 — focused E3 GREEN, full CI pending

- Branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Source: `5012ce963db5d88dfd0a9101c4738926c682d218`; merge `cd41b475` includes `f8f01b74ad80fc26223751ce3bfdc2cb849df6c6`. Final evidence-only commit does not alter source hashes below.
- Scope: LC-U1 only. No authored PM-U/W3-U1b change. No product Go/SQL, grants, application timezone conversion or frozen `tests/ui/**` change in this repair. No push/deploy/provider mutation.
- Root model telemetry NOT_EXPOSED; independent read-only explorer inherits parent with medium reasoning. Non-author source review found no blocker; it did not execute tests.

## Root causes and retained assertions

1. The browser is genuinely signed in, but Node `APIRequestContext` omits Secure cookies on HTTP loopback. Browser cookie inspection with the HTTP URL also filtered those cookies. Inspect the **same host's HTTPS cookie scope only**, then forward the existing signed session/CSRF cookies to the unchanged BFF origin. CSRF-negative still has **no CSRF header**; read principal still has **no inventory grant**. Exact 403/forbidden, disabled controls, unchanged stock and zero receipt effects remain.
2. Logout clears private view before the hard navigation; Next canonicalizes `/en/` to `/en`. Await the actual same-origin canonical landing and visible Sign-in, then reauthenticate. Rotated-CSRF and opaque receipt fence assertions remain.
3. Retry control disappears at **request start**, before acknowledgement. Wait for the **same-key second receipt**, then retain exact two receipts, identical body hashes and **one effect** assertions. No timeout/threshold was increased.
4. Native calendar trace screencasting interposes in selection. Disable only automatic trace screenshots, retaining actions/DOM/source, explicit post-selection and failure shots. Local selection now changes `2030-01-01T08:00` to `2030-01-02T08:00`, saves/reloads and restores the original; PG proves Taipei `08:00` is UTC `00:00`. No product date/time code changed. Linux CI still needs the full rerun.
5. Added a strictly validated **single known Studio-main focus** for local testing. CLI grep drops the bare-title leading anchor because Playwright matches file+title. Default full mode remains all six cases with **3 faults / 5 keys / 5 issued logins** and native-uncertain persistence checks. Focus has exact **2 / 4 / 2**, retains main authority/UTC/lost-ACK/swapped-login/Start/Stop/worker/secret-log readbacks, and labels excluded cases NOT_RUN. Unknown filter fails before fixture creation.

## Final-source commands

Both browser commands ran serially on source `5012ce96`. Fixture teardown completed; no owned browser/Next/media worker/HTTP/PG task remained. The suite's authoritative evidence is MOCK providers + REAL_PG identity/Studio, not LIVE production.

| Command | Exit / result | Evidence |
| --- | --- | --- |
| `LC_TEST_LOCK_WAIT=14400 LC_BROWSER_CONSOLE_GREP='en-1586\|unknown receipt\|LC-U1 en:' bash scripts/dev/test-local.sh --browser-live-console` | 0; Playwright 3/3; Go focused chain 1 PASS | `ci4-console-final.log`, `CI4-evidence/console/playwright.log` |
| `LC_TEST_LOCK_WAIT=14400 LC_BROWSER_STUDIO_GREP='^STU04 signed Studio UI through packaged Next, Go, PG and local MOCK worker$' bash scripts/dev/test-local.sh --browser-studio-ui` | 0; Playwright 1/1; main Go/PG/worker readbacks PASS | `ci4-studio-focused.log`, `CI4-evidence/studio/playwright.log`, `browser-summary.json` |
| `bash scripts/dev/test-node.sh` | 0; 510 passed, 0 failed/skipped | `ci4-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `ci4-tsc.log` |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext --moduleResolution bundler --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/live-console.spec.ts tests/admin/studio-ui.spec.ts` | 0 | `ci4-spec-tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0; 73 documented modes, header base f8f01b74 | `ci4-check-gates.log` |
| From `tests/foundation`: `go test -run '^TestCvsNoIframeAndPrivacy$' -count=1 taiwan_cvs_guards_test.go` | 0; original guard unchanged, no PG | `ci4-privacy.log` |
| `bash -n scripts/dev/test-local.sh`; `gofmt -l tests/foundation/browser_studio_ui_test.go`; `git diff --check` | 0; no format/conflict output | Author receipts |

Console business snapshot/receipts are explicitly MOCK; its PG proves signed identity/authority, not LC-B1/LC-B7 SQL behavior. Studio uses production Go/PG and a local MOCK media provider. Its main case includes real three-locale 1586/390 screenshots, but does **not** execute the separate STU05/read-only/native-conceal cases.

Exact copyable focused commands (serial; only under the owner's local-focus authorization):

```sh
LC_TEST_LOCK_WAIT=14400 LC_BROWSER_CONSOLE_GREP='en-1586|unknown receipt|LC-U1 en:' bash scripts/dev/test-local.sh --browser-live-console
LC_TEST_LOCK_WAIT=14400 LC_BROWSER_STUDIO_GREP='^STU04 signed Studio UI through packaged Next, Go, PG and local MOCK worker$' bash scripts/dev/test-local.sh --browser-studio-ui
```

## Click ledger (selected scope)

| Surface/action | Expected and observed |
| --- | --- |
| Console sign-in + main start/offer/stock/recommend/end/copy | Real clicks; visible phase/values and redacted fixture readback pass |
| Missing-CSRF direct request (supplemental authority negative) | Existing signed session reaches 403; no command receipt/effect |
| Narrow stock edit + no-stock principal | Real editable/disabled controls; bounds/refusal, persisted quantity and supplemental 403/no-effect pass |
| UNKNOWN → explicit retry | Same key/body, two transport receipts and one effect; no automatic resend |
| UNKNOWN → real Account logout/login/reload | Private view purged, CSRF changes, durable fence blocks new writes |
| Studio native calendar click + ArrowRight/Enter | Jan 1 → Jan 2; visible text matches; save/version/reload/restore pass |
| Studio three-language desktop/phone/main edits | Actual UI navigation and persistent values pass; selected shots committed |
| Studio lost ACK + swapped login + Start/Stop | Frozen key/grant/authority assertions and exact PG/worker readbacks pass |

## Red records — retained

- `ci4-console-setup-red.log`: exit 2 before PG/browser, merge missed Console in the accepted-mode guard; usage/dispatch already contained it. Corrected without dropping the trunk Meta-health mode.
- `ci4-console-focused.log`, `CI4-evidence/red/console-cookie-url.log`: 3 failures at source `8df4fb7b`, HTTP-cookie inspection and noncanonical logout URL.
- `ci4-console-green.log` (legacy filename, **RED**), `CI4-evidence/red/console-receipt-race.log`: source `c25b51de`, 2 PASS/1 FAIL; security negatives now pass, same-key receipt read raced its acknowledgement. Fixed in `2cc828b4`.
- `ci4-studio-setup-red.log`, `CI4-evidence/red/studio-selector-setup.log`: no tests found; CLI full-title prefix versus bare anchored name. This is setup NOT_RUN, not a date regression.
- The earlier CI3 actual native-calendar and 401 reds remain in CI3-DELIVERY. Nothing was deleted or weakened.

## Source / evidence binding

| File | SHA256 |
| --- | --- |
| `tests/admin/live-console.spec.ts` | `3e0e9d98b406090d9bf8e07dc9266c3d45e6829fd9e26aa34f2283989b0e86fb` |
| `tests/admin/studio-ui.spec.ts` | `4396f094c953ab1026c9d3e0e3f5281453c91b0b222e09713c598d2a97beab82` |
| `tests/foundation/browser_studio_ui_test.go` | `b646bbd6d0c5a5b55ffa776d3e2bc34edd455eb1cefdbc1edf8c0cb157aa56b3` |
| `scripts/dev/test-local.sh` | `00326d0860f2bccd20e09c646318a824d54a9cc5bd34659f0ea30696e02be629` |
| `CI4-evidence/console/playwright.log` | `da684e886cff62d11c9e18c9cb8f7183f7631ef013db8aef0a982d17bd82aefe` |
| `CI4-evidence/studio/playwright.log` | `123a555b7785ffb66bc9e42b05b59af2a1ae53400b43eece2defad8e7e29d648` |

## CI gates / NOT_RUN

Integrator should rerun full `--browser-live-console` and `--browser-studio-ui` on the delivered SHA **with both focus env vars unset**. Standing regression modes remain `--browser-studio-bff`, `--browser-live-claims`, `--browser-admin-shell`, `--browser-click-sweep`, `--browser-visual-lint`, plus D3 `--browser-e2e`.

NOT_RUN locally: full 11-case Console locale/width matrix; Studio's excluded cases; Linux native picker; full foundation; global sweep/visual/all-mode batches; provider/SANDBOX/LIVE; optional R04 pinned media binary (unset). **No whole-unit completion or E4 claim.** W2-U2 and W2-U3 are still queued behind full LC-U1 green and integrator-approved base.

Humaux semantic search had an upstream embedding request error; exact grep/store remained available. Standalone Doctor got unauthenticated 401, which does not establish that the app's authenticated MCP credentials are invalid. No config/key changes made.
