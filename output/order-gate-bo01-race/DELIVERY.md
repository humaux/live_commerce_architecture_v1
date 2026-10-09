<!-- Purpose: causal BO01 destination-read race fix and frozen-source browser verification.
Depends on: order-gate, real Next/BFF/Go/PG, native locale links and CI failure brief.
Used by: integrator review/CI; no provider/production claim. -->
# order-gate BO01 race

- Branch `unit/order-gate-bo01-race`, base `30ddfb109059340070dd7aa4eabd0469f38983de`; final tested source `75d4917baec234effbcdade2b24a8346be510eda`. Source commits `318a5516`, `2d4acb53`, `d9914b38`, `75d4917b`; final delivery commit adds evidence only.
- Scope: order browser driver, its Go harness flag forwarding, existing deadline unit test. No product/contract/dependency changes, retries, timeout increases or weakened assertions.
- Role: Codex-3 implements and runs runtime gates (GPT-6 family; exact deployment/effort not exposed). Existing read-only reviewer configured `gpt-6.1-sol/high` inspected d9914b38 and final75d4917b, ran two deadline tests and independently checked JSON proof; no P1/P2 found. Browser runtime was run by root, not independently rerun by reviewer.

## Root cause and fix

`quotePage` waited for the address section DOM, which can appear before its English document's initial GET /api/buyer/destination reaches the proxy. `stableLocaleTarget` then installed a global one-shot path/method hook before clicking zh-TW. That pending English read could consume the hook. The real new zh-TW document's mount read then completed normally, so the full form appeared and there was no loading paragraph to observe. This was a test request-selection race; native locale links perform full document navigation, and the API read is no-store.

The hook now matches the target document Referer path **and the actual observed owner's HttpOnly secure cookie** before consuming the one-shot hold. The probe asserts source/owner/status, retains the exact loading visibility assertion, scrolls and measures the footer, releases the held response, asserts exact top equality and actually clicks/taps English. Cookie values remain in memory only; proof JSON contains source path, status/match booleans and geometry.

## Causal red and repeat proof

Calibration pauses the actual initial English UI GET in Playwright routing and releases it after the global hook is armed. For the old matcher, the unheld target form is explicitly allowed to hydrate before the loading probe, matching the CI artifact's early-resolution condition. At 2d4acb53 the original assertion fails after **5000 ms**: old English GET = 200/held, target zh-TW GET = 200/unheld, failure page is a complete form. Evidence `evidence/red/bo01-head-reads.json`, `browser.log`, `failure-0.txt/png`.

`LC_BO01_REPEAT=10` runs the unchanged layout assertions in a bounded loop, desktop/mobile; the core observation is emitted once after completion. Go explicitly forwards both safe calibration flags. Chrome proof has **20 probes**, exactly iterations 1..10 per device; all target/owner matches true and before/after footer top equal. Existing **26 core observations, 6 buyers and 7 orders**, exact order/hold/job/receipt/reserve SQL checks remain required and passed.

First attempted run had no node_modules and stopped at `next: command not found`; it is an environment setup failure, not causal RED. `pnpm install --offline --frozen-lockfile` exited 0 with no lockfile change, then the real RED was run. Logs retained.

A first WebKit pressure run at d9914b38 passed desktop10 and mobile6, then failed initial zh-TW navigation on mobile iteration7. No target zh-TW GET arrived for that iteration; the old English setup had only been checked for section visibility. Final75d4917b settles the old form before entering the measured view and uses native mobile WebKit tap for that setup navigation. The actual measured target still remains loading with its GET held until footer measurement; no layout assertion was removed. Failure is retained in `evidence/webkit-setup-failure/` and the d991 WebKit JSONL.

## Gates

| Command / condition | Exit | Evidence |
|---|---:|---|
| `LC_BO01_EARLY_HEAD=1 bash scripts/dev/test-local.sh --browser-order` (old matcher) | 1 expected | `old-early-head-real-red.log` |
| `LC_BO01_EARLY_HEAD=1 LC_BO01_REPEAT=10 bash scripts/dev/test-local.sh --browser-order` | 0 | `chromium-final-early-head-10.log`; 10/10 desktop + 10/10 mobile |
| `bash scripts/dev/test-local.sh --browser-order` (ordinary conditions) | 0 | `chromium-final-normal.log`; exact original 26/6/7 facts |
| `LC_BROWSER_ENGINE=webkit LC_WEBKIT_STEPS=order LC_BO01_EARLY_HEAD=1 LC_BO01_REPEAT=10 bash scripts/dev/test-local.sh --browser-webkit` | 0 | `webkit-order-final-10.log`; order subset only |
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `browser-vet-final.log` |
| `node --test apps/storefront/tests/order-gate-timeout.test.mjs` | 0 | `timeout-unit-final.log`; unchanged 10000-ms missing-head diagnostic and timer cleanup |

Adjacent runner status JSON records HEAD/source hashes, exits, bounded wall-clock budgets and unchanged-source checks. Browser invocations ran serially. Original Node driver deadline 180 seconds and all layout assertions/timeouts remain unchanged.

## Acceptance boundary

**E3 at75d4917b**: Chrome and WebKit order-subset each passed20 probes (10 per device), target/owner/footer equality; ordinary Chrome and all listed static gates passed. Real production Next/BFF/Go/isolated PG plus synthetic TLS/request-order calibration. All owned runners/contexts/fixture containers exited through harness cleanup; unique failure evidence remains. No tracked evidence restoration was needed; source changes are the three listed test files only. Byte-original logs/screenshots remain in ignored per-run directories; committed text logs normalize trailing whitespace only. Full WebKit modes, full CI/foundation and providers are NOT_RUN; local subset must not be reported as full WebKit acceptance. CI gates: `--browser-order` and full `--browser-webkit` required set after integrator push. No push or GitHub thread reply.
