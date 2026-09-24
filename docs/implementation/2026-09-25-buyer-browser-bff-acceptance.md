# Buyer public browser transport — bounded acceptance

Date: 2026-09-25. Source `d2c5187` (Go `48ce85b` + causal gates `e9297fa`;
Next author `9b6d570` -> integration `de6b22c`; own-key fix `d949462` -> `a07d79e`).
This is local **REAL_BROWSER + REAL_HTTP + REAL_PG** transport acceptance, not UI,
DNS/TLS deployment, PSP, CVS or full SaaS acceptance. Public feature stays off
unless explicitly configured. No customer live service or real funds changed.

## Scope and ownership

- [Frozen contract](../../contracts/buyer-browser-bff-v1.md), preflight memory
  `0f7a42be-75e3-4692-be74-e565ecb63d10`.
- Root owns0022, Go admission/retirement, package/build config and independent PG/
  browser gates. Isolated `commerce_worker` author used gpt-6-sol/high, base
  `6cc265c`, branch `commerce/buyer-public-bff-20260925`, worktree
  `/Volumes/data/live-commerce-buyer-http-20260925`; only two lib modules,
  catch-all route and two focused test files. No recursive delegation.
- Separate read-only reviewer closed two quota-fixture P2 items at `e9297fa`
  (memory `36f1d666-5fb9-4556-818e-ec22421fbb6b`) and independently reproduced the
  inherited JSON shape-key bypass before its own-property fix at `a07d79e`.
- Final independent source/evidence review at `d2c5187` found no remaining
  P0/P1/P2 (memory `32fde5a5-2faa-4ea2-bcdd-b45085ea1e5b`); reviewer independently
  reran7 focused tests and checked all retained gate hashes, not a second PG run.
- Standard Node/Web APIs and installed Next/React/TypeScript only. No second
  payment engine, credential framework, provider call or new identity table.

## Actual gates

All commands run at `/Volumes/data/live_commerce_architecture_v1`. No skipped
database tests were counted. Output below is a retained actual process result.

| Gate | Actual result | Evidence |
| --- | --- | --- |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --buyer-http` | exit0,22 top-level PASS,0FAIL/0SKIP,9.908s | `root-public-admission-causal-fixed.log` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh` | exit0,366 top-level PASS,0FAIL/0SKIP,foundation122.173s; race + vet | `root-public-admission-full.log` |
| `node --test apps/storefront/tests/*.test.mjs` | exit0,7PASS,0FAIL/0SKIP | `root-bff-focused-final.log` |
| `pnpm run typecheck:storefront` | exit0 | root tool session71920 |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-buyer` | exit0; actual Next production build; 11 browser scenarios; Test6.01s/foundation8.266s | `root-browser-fourth.log` |

The `.log` files above are under
`/Volumes/data/output/live-commerce-buyer-registration-tests/` (retained prior
registration directory). SHA256:

```text
root-public-admission-causal-fixed.log 38a70a855f7f571c118bc04c000b68ccb226a70a48696443c12a4fdb070f8f99
root-public-admission-full.log 2d2b2ac5448037c0504e376c6a895986c54cd48de7c522a775067dce6926be27
root-bff-focused-final.log 609f939bf1457d2303506270839705011bc6ccd0af7945c5f0948b5170ee9b15
root-browser-fourth.log a823f24facfa13b94e448e68f1279d253c50f107c3fa433178ce5cee8045a2bc
```

Actual browser artifacts:
`/Volumes/data/live_commerce_architecture_v1/output/playwright/buyer-real-396173793/`.
`browser.log` SHA256 `0185bdfb91f888c8c52c20e996db6fa7072fe836d5d3aa9dbf0a98f66c8502c9`;
`result.json` SHA256 `0d909627d013b3d9e255d6cfa323494daf8966a97f9f34cc9e39c2ce3fd5ade7`.
Test-only HTML loads the actual stripped client module; it is not a shipping UI.
An owned TLS/CONNECT edge preserves Host/Origin to two actual production Next
instances, without a shared HTTP cookie jar. Synthetic certificate/domain
publication is not evidence of real merchant DNS/TLS verification.

## Verified behavior and corrected failures

- Multi-tab initialization retains one cookie/context; raw bearer absent from DOM,
  storage and safe API projections. Context mismatch prevents stale-tab writes.
- Actual catalog/options -> cart -> quote -> synthetic home destination ->
  checkout -> order read; independent Go/PG asserts exactly one DRAFT order and
  one inventory hold after replay. Real Next restart preserves access/readback.
- Definitive parsed negative prepare clears the journal. Lost prepare before
  headers blocks another application prepare; lost activation recovers through
  current DB state. Headers-before-body abort and tab death retain the exact
  cookie, not merely some active session. Closed-tab late activation loses to
  successfully committed retirement. Logout leaves an inert cookie.
- Real PG retirement handles unknown/concurrent/uncommitted hashes. Quota is
  shared per-store; seeded599 + an uncommitted winner causes a different-token
  independent issuer to wait on `advisory`, then429 with only one new fact set.
  Replay/known retirement work at quota; other stores remain separate; stale
  repeatable-read snapshots fail closed. Seeds clean up only owned fixture IDs.
- RED evidence retained: initial new quota CTE used reserved `window` (42601),
  corrected to `quota_window`; first browser launch used an unopened WriteStream
  for spawn stdio; second readiness probe used Node24 fetch that ignored Host
  (standalone diagnostic: fetch403 vs rawHTTP200); corrected to exact raw relay.
- Third browser RED exposed a fixture assumption: Chromium can transparently
  resend a reused-connection empty response, so one dropped packet did not cause
  a failed fetch. The fourth gate sustains the fault until fetch actually fails,
  then asserts the next initialize sends no new prepare. No threshold weakened.
- Independent source review found `shape[key]` accepting Object.prototype keys.
  `Object.hasOwn` now rejects `__proto__`/`constructor` for empty and nested
  bodies, with no Set-Cookie/upstream side effect. The focused regression passed.

## Operations, limits and next gates

Use the contract's five environment variables; never put keys in NEXT_PUBLIC,
browser data, URLs or logs. Match the Go BFF key and TTL, keep signing key stable
across replicas, and set fixed private API origin. Disabled mode ignores other
configuration. Invalid enabled config yields safe503 without writes.

The public registration ceiling is600/store/UTC-minute, not a measured production
capacity or a promise covering legacy private issue. Per-client ingress limits,
trusted DNS/TLS publishing, domain administration and deploy packaging remain
release gates. Expired/invalid states never silently remint; irrecoverable unknown
cookie delivery may require a fresh isolated browsing context. No buyer-account
merge or authentication identity expansion was added.

NOT_RUN/unfinished: approved mobile/desktop three-language buyer UI; automatic
merchant product share URL; trusted CVS source; provider-hosted order checkout,
real sandbox payment/webhook/reconciliation; live deployment/performance/recovery.
Product-entry PE01–PE10 and full SaaS G01–G15 remain incomplete.

The runner removes owned PG and generated fixture certificates, stops its two
Next children/browser/proxy, and preserves logs. A fresh isolated process group
provides timeout cleanup; no existing process/container/port-name kill is used.
