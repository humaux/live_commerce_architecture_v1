<!-- Purpose: Hand off LC-U1 CI3 driver corrections and the conditional W2 UI queue with exact evidence boundaries.
Depends on: source e6bc38a5, trunk 93233a00, GitHub run 37501896738, frozen LC-U1 gates and existing signed MOCK fixtures.
Used by: Integrator GitHub reruns and non-author acceptance; does not authorize W2 implementation before LC-U1 is green. -->
# LC-U1 CI3 — author candidate, runtime acceptance pending

- Branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Code commit: `e6bc38a5d8898e3e832510da72db05e25accfbb9`; merge `451e106d` includes trunk `93233a00`. Evidence-only follow-up does not change the two spec hashes below.
- Model: runtime name not exposed; read-only explorer inherits parent model with requested medium reasoning. Actual model telemetry NOT_EXPOSED.
- No application, Go/SQL, grants, provider calls, frozen `tests/ui/**`, assertions or thresholds changed in this repair.
- Evidence: E3 for local Node/static/privacy/isolated transport checks; **NOT_RUN** for corrected browser interactions. Independent source review is not runtime acceptance.

## Red attribution and corrections

Run `37501896738` tested `28d6d316`; its tree equals the author merge `451e106d`: `bddbde3743889761f8ef3e7612747945b8b20577`. The older `37497728289` failures were shared Go test compilation (`mustJSON` redeclared), already repaired by trunk `93233a00`; do not count those as UI failures.

| Current CI3 red | Cause / correction | Safety assertions retained |
| --- | --- | --- |
| Console 10 failed / 1 passed: six CSRF negatives and three no-stock negatives received 401 instead of 403 | Node `APIRequestContext` omits Secure authority cookies on the HTTP loopback fixture. `authorityCookie` forwards only the already signed browser session/CSRF cookies, following the existing auth-spec pattern. Isolated transport experiment reproduces the omission. | CSRF-negative still omits the CSRF header; no-stock principal still has neither inventory permission; exact 403/forbidden, unchanged receipts and stock remain. No invented session/grant. |
| Console reauthentication `net::ERR_ABORTED` | Private-view clearing precedes `window.location.replace`. Await real `/en/` DOM navigation and visible Sign-in before the next login. | Private-view purge, rotated CSRF, opaque receipt fence, no repeated operation and disabled writes remain. |
| Studio 2 failed / 4 passed: native date stayed `2030-01-01T08:00` | Trace screencast frames interpose click/ArrowRight/Enter. Disable automatic trace screenshots only; preserve actions, DOM snapshots and source. **Candidate mechanism**, not proven Linux runtime fix. | Native trusted click/keyboard, changed date, version/save/reload/restore, explicit post-selection shot and failure shot unchanged. |
| Studio read-only Start-absent failure | Main case aborted before starting the shared prepared scene; this is its fixture cascade. No separate fix. | Original Start-absent/Stop-disabled and no-write assertions unchanged. |

Read-only non-author `/root/w2_ui_preflight` inspected the driver changes and actual `requireCSRF` cookie+header rule, grants, logout redirect and preserved diagnostics; no source defect reported. No browser was run by that reviewer.

## Local commands on repaired source

| Command | Exit / result | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0; 496 passed, 0 failed, 0 skipped | `ci3-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `ci3-tsc.log` |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext --moduleResolution bundler --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/live-console.spec.ts tests/admin/studio-ui.spec.ts` | 0 | `ci3-spec-tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0; 72 modes documented, header base `93233a00` | `ci3-check-gates.log` |
| From `tests/foundation`: `go test -run '^TestCvsNoIframeAndPrivacy$' -count=1 taiwan_cvs_guards_test.go` | 0; unchanged source/privacy guard, no PG | `ci3-privacy.log` |
| `pnpm exec node --input-type=module` — isolated experiment below | 0 | `ci3-secure-cookie-probe.log` |
| `git diff --check` | 0 | Author command receipt |

Initial ad-hoc spec typecheck used NodeNext rather than this repository's bundler/type-root configuration and exited 1; retained in `ci3-spec-tsc-wrong-config.log`. Correct existing configuration above passes. It was not a product type defect. The optional pinned R04 binary suite remains NOT_RUN (binary env unset).

### Reproduce the isolated Secure-cookie observation

Run with the existing installed Playwright dependency; this starts one temporary loopback HTTP server, no browser/PG/provider, and closes it in `finally`. The value is synthetic, not an actual authority token.

```js
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { request } from '@playwright/test';
const observed = [];
const server = createServer((req, res) => { observed.push(req.headers.cookie ?? ''); res.end('ok'); });
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const port = server.address().port;
const api = await request.newContext({ storageState: { cookies: [{ name: '__Host-commerce_session', value: 'fixture-only', domain: '127.0.0.1', path: '/', expires: Math.floor(Date.now()/1000)+60, httpOnly: true, secure: true, sameSite: 'Lax' }], origins: [] } });
try {
  await api.get(`http://127.0.0.1:${port}/`);
  await api.get(`http://127.0.0.1:${port}/`, { headers: { Cookie: '__Host-commerce_session=fixture-only' } });
  assert.equal(observed[0], '');
  assert.equal(observed[1], '__Host-commerce_session=fixture-only');
} finally {
  await api.dispose();
  await new Promise((resolve) => server.close(resolve));
}
```

## CI gates — GitHub only

**New-source runtime is NOT_RUN. Integrator pushes and dispatches; author did neither.** Immediately rerun `bash scripts/dev/test-local.sh --browser-live-console` and `--browser-studio-ui` on the new candidate. Standing regression modes: `--browser-studio-bff`, `--browser-live-claims`, `--browser-admin-shell`, `--browser-click-sweep`, `--browser-visual-lint`; D3 also requires the earlier `--browser-e2e` real-click mode. Full foundation remains integrator-owned.

The pre-fix run `37501896738` has green Claims, admin shell, visual lint and `^Test[B-K]` shard; Console/Studio are red as above. Sweep was still running at this evidence cut. `ci3-jobs.json` is the final fetched snapshot, not a result for `e6bc38a5`.

## Source / red-evidence binding

| File | SHA256 |
| --- | --- |
| `tests/admin/live-console.spec.ts` | `fcf0cf3da4ea90225232688d929f4ab72f29b5aaf009f74ca04a682146bb4cad` |
| `tests/admin/studio-ui.spec.ts` | `4396f094c953ab1026c9d3e0e3f5281453c91b0b222e09713c598d2a97beab82` |
| `ci3-artifacts/ci-gates/live-console-3845819996/playwright.log` | `a47a3cfbb665efd0cb460b24a3fea072f7d9f2e2d83939bd1f9fcefe9292746e` |
| `ci3-artifacts/gate-6/playwright/studio-ui-20261006T171911.426009600/playwright.log` | `7bb0d56bd75ed60749369e76fda34647a186cecf5c4c740a5a4eaba2cacfb8b6` |

Raw downloads/traces remain ignored in `ci3-artifacts/`; they were not deleted. No owned browser/PG/container process was started. The temporary Node experiment server and API context were closed. No shared lock/process was stopped.

## Conditional queue — not started

1. W2-U2: coord `ff5e286e-7937-475f-a7ed-c59eef33de6e`, depends on LC-U1 `cf37239e-65bc-4369-b613-710ae52b0c09` green + integrator-confirmed base. Planned `unit/w2-u2-console-stream` / `.worktrees/w2-u2-console-stream`. Reuse U1 shell/transport/fences; add merged comments/DM, shared buyer panel and Inbox only. Actual comments cadence 3 s, console 5 s, inbox 10 s; visible-only and bounded backoff. 0151 `private_reply.kind=out_of_stock` spends the same one-comment budget; 0154 `claim.reason=restricted` plus separate blocklist-check warning. All 24h/private quota/public-content/UNKNOWN rules remain server authoritative. CI: new formal `--browser-inbox`, extended `--browser-live-console`, shell/sweep/visual regression; real clicks three locales at 1440/390.
2. W2-U3: coord `e794b0e7-9c1d-46d3-8a1b-6682429c5dff`, depends on accepted W2-U2 + frozen LC-B6. Reuse/extract ManualOrder form/options, explicit-linked-customer A15 prefill only; A16 `/orders/for-buyer` has a 14 s bounded pipeline and needs a narrowly suitable BFF budget. Preserve order-created/not-sent result when no DM window, same-key replay and restricted-buyer warning. CI: extended `--browser-manual-order`, Inbox/Console plus shell/sweep/visual; three locales at 1440/390.

Neither worktree exists yet. Once approved, install with `pnpm install --offline --frozen-lockfile`, merge latest trunk (93233a00+), claim the exact task, and implement red→green. Per unit final step is `git add -A && git commit`, reporting SHA/DELIVERY CI gates. No push/deploy/provider mutation.
