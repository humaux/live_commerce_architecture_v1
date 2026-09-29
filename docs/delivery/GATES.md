# Gates: what runs, what it proves, how to run it

Status: hand-kept, mechanically checked by `bash scripts/dev/check-gates.sh` (CI). The script fails when a
mode in the usage line of `scripts/dev/test-local.sh` has no row here, when a row names a mode that no
longer exists, when any tracked `*.spec.*|*.test.*` file (whole repo) is run by no gate, or when CI stops invoking `scripts/dev/test-node.sh`. Nothing may exist that
no gate runs (PROCESS.md, unit maintainability). Evidence labels follow `AGENTS.md` (DESIGN, MODEL_ONLY,
MOCK, SANDBOX, LIVE, NOT_RUN): a pass here is only ever as strong as the label in its "proves" cell.

## Tiers

| Tier | What | Where it runs | Needs |
| --- | --- | --- | --- |
| T0 static | `check_packet.py`, `go vet`, secret-literal grep, `depmap.sh --check`, `check-pkgdocs.sh`, `check-gates.sh`, `test-node.sh` (Node unit suites) | CI (`.github/workflows/foundation.yml`) | Go, Python 3, Node 24 |
| T1 foundation | default `bash scripts/dev/test-local.sh` (no flag): the whole Go module under `-race` on a disposable real PostgreSQL 18 | CI | Docker, pinned PG image |
| T2 subset | one focused slice of the T1 suite (`--checkout`, `--meta-inbox`, `--live-media-*`, ...); faster feedback, never acceptance on its own | developers; T1 in CI covers the same code | Docker |
| T3 browser | real Chromium against the packaged Next apps + Go + PG; MOCK IdP/PSP unless the row says SANDBOX | developers, one mode at a time (`bash scripts/dev/test-local.sh <mode>`); NOT_RUN in CI. `scripts/dev/release-gate.sh` runs every mode in this file (R1 acceptance) | Docker, pnpm, Node 24, Playwright Chromium |

CI is deliberately T0 + T1 only (owner decision: full foundation plus static checks). Browser modes are
too slow and machine-bound for every push. T3 rows run in automation only through `bash scripts/dev/release-gate.sh`
(R1 acceptance, one PASS/FAIL/NOT_RUN table), or when a developer invokes a mode.

## test-local.sh modes

`foundation` (no flag) is T1. Every other mode:

| Mode | Proves | Tier | Run |
| --- | --- | --- | --- |
| `--browser-identity` | isolated PG + signed MOCK IdP browser chain; fixture removed at exit | T3 browser | `bash scripts/dev/test-local.sh --browser-identity` |
| `--browser-admin-legacy` | admin ledger (fixture bearer) + production fail-closed + identity-mock + entry-mock browser suites; no signed IdP, not production acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-admin-legacy` |
| `--browser-buyer` | isolated PG + real buyer browser transport; not UI/PSP/deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-buyer` |
| `--browser-merchant-buyer` | isolated merchant-to-buyer browser chain; not provider payment or real DNS/TLS deployment proof | T3 browser | `bash scripts/dev/test-local.sh --browser-merchant-buyer` |
| `--browser-merchant-orders-bff` | isolated Next + Go + PG merchant-order read transport; not merchant UI or provider acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-merchant-orders-bff` |
| `--browser-merchant-orders-ui` | isolated merchant C order UI; signed MOCK IdP and local payment fixtures, not production/provider acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` |
| `--browser-input-delivery` | isolated HTTPS signed browser + Next + Go + PG input token transport; not decoded SFU media, recovery or production acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-input-delivery` |
| `--browser-studio-bff` | isolated signed OIDC + Next + Go + PG Studio BFF transport; not Studio page/UI, Cloud or provider acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-studio-bff` |
| `--browser-studio-ui` | isolated Studio B UI with signed MOCK IdP and local MOCK Egress; not Cloud or production acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-studio-ui` |
| `--browser-live-claims` | KC16 isolated admin + storefront Next, Go and PG claims chain; signed MOCK IdP, MOCK manual ingress; no provider or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-live-claims` |
| `--browser-order` | isolated buyer address/order UI gate; not provider payment or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-order` |
| `--browser-payment` | isolated buyer payment UI/native POST gate with a local mock PSP; not provider payment or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-payment` |
| `--stripe-browser` | SP18/SU05-SU09 buyer Stripe Checkout in Chromium: card 4242, decline, 3DS, plus the PAYUNi baseline; SANDBOX steps need STRIPE_BROWSER=1 STRIPE_SANDBOX=1 and a Stripe test key in secrets.env (else NOT_RUN, exit 2); one go test process per step, parsed from go test -json | T3 browser | `bash scripts/dev/test-local.sh --stripe-browser` |
| `--browser-refund-fulfilment` | MF07 + RF11(a) isolated admin + storefront Next, Go, PG, real worker and the MOCK Stripe fake; RF11(b) SANDBOX is NOT_RUN unless it says otherwise above; not provider or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-refund-fulfilment` |
| `--browser-e2e` | T12 deal loop in one real-browser chain: signed Meta MOCK comment -> claim -> private reply -> cart -> Stripe MOCK pay -> order -> manual ship -> partial refund, admin + storefront, PG facts, negatives, tenant isolation, leak scan; SANDBOX tier NOT_RUN (F12), CVS pickup NOT_RUN (F3) | T3 browser | `bash scripts/dev/test-local.sh --browser-e2e` |
| `--checkout` | checkout subset only; full regression still required | T2 subset | `bash scripts/dev/test-local.sh --checkout` |
| `--payment` | payment start/query subset only; full regression still required | T2 subset | `bash scripts/dev/test-local.sh --payment` |
| `--payment-worker` | isolated payment worker subset; no real-provider or deployment claim | T2 subset | `bash scripts/dev/test-local.sh --payment-worker` |
| `--expiry-worker` | isolated expiry worker subset; no production or recovery-SLO claim | T2 subset | `bash scripts/dev/test-local.sh --expiry-worker` |
| `--storefront-resolver` | published-origin resolver subset only; not public HTTP or provider proof | T2 subset | `bash scripts/dev/test-local.sh --storefront-resolver` |
| `--buyer-http` | private buyer HTTP subset only; not public BFF/browser or provider proof | T2 subset | `bash scripts/dev/test-local.sh --buyer-http` |
| `--purchase-entry` | merchant purchase-entry real PG/HTTP subset only; not buyer UI or provider checkout | T2 subset | `bash scripts/dev/test-local.sh --purchase-entry` |
| `--merchant-orders` | isolated merchant order read subset; no merchant UI/provider/deployment claim | T2 subset | `bash scripts/dev/test-local.sh --merchant-orders` |
| `--meta-inbox` | isolated Meta inbox subset only; no public mount/provider qualification claim | T2 subset | `bash scripts/dev/test-local.sh --meta-inbox` |
| `--meta-consumer` | isolated Meta social consumer subset only; no public mount/provider qualification claim | T2 subset | `bash scripts/dev/test-local.sh --meta-consumer` |
| `--meta-runtime` | isolated Meta API/worker runtime subset only; no public deployment/provider qualification claim | T2 subset | `bash scripts/dev/test-local.sh --meta-runtime` |
| `--legacy-isolation` | isolated legacy-family subset only; not full regression/provider/production acceptance | T2 subset | `bash scripts/dev/test-local.sh --legacy-isolation` |
| `--local-recovery` | isolated logical restore/cold-start subset only; not PITR, production RPO/RTO or deployment acceptance | T2 subset | `bash scripts/dev/test-local.sh --local-recovery` |
| `--live-planning` | isolated live draft planning only; no broadcast, HTTP, provider or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-planning` |
| `--live-authority` | isolated MOCK media authority registry and draft planning; no controller, LIVE intake, provider or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-authority` |
| `--live-media-plan` | isolated MOCK media start intent and native queue; no provider execution, LIVE intake or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-media-plan` |
| `--live-media-execution` | isolated MOCK media execution and recovery; no Stop, LIVE provider, resource reclamation or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-media-execution` |
| `--live-browser-input` | isolated BRW SQL/executor and Go HTTP subset; HTTPS browser/SFU and recovery gates remain separate | T2 subset | `bash scripts/dev/test-local.sh --live-browser-input` |
| `--live-media-input` | isolated BIC custody subset only; full media and BIC05 regression still required | T2 subset | `bash scripts/dev/test-local.sh --live-media-input` |
| `--live-media-crash` | isolated LMR05 crash diagnostic only; Stop and full regression still required | T2 subset | `bash scripts/dev/test-local.sh --live-media-crash` |
| `--live-media-stop` | isolated bounded MOCK media Stop; no Cloud, LIVE intake, operator escalation recovery or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-media-stop` |
| `--live-media-recovery` | isolated MRR observer SQL/process gates only; no Cloud, human alert delivery, LIVE intake or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-media-recovery` |
| `--live-media-runtime` | isolated actual media command, PG18 and local TLS runtime; no Cloud, LIVE intake or G06 acceptance | T2 subset | `bash scripts/dev/test-local.sh --live-media-runtime` |
| `--studio-backend` | isolated Studio backend/API and local MOCK media gate; not BFF/browser, Cloud, LIVE intake or full Studio acceptance | T2 subset | `bash scripts/dev/test-local.sh --studio-backend` |

## Browser specs and the gate that runs each

| Spec | Mode |
| --- | --- |
| `tests/admin/auth-real.spec.ts`, `settings-real.spec.ts` | `--browser-identity` |
| `tests/admin/ledger.spec.ts`, `production.spec.ts`, `visual-states.spec.ts` | `--browser-admin-legacy` (suite `ledger`: `admin-fixture` PG + Go API, dev Next fixture adapter on :3100, packaged production Next on :3101) |
| `tests/admin/auth.spec.ts` | `--browser-admin-legacy` (suite `identity-mock`, MOCK Go API on :19111) |
| `tests/admin/entry.spec.ts` | `--browser-admin-legacy` (suite `entry-mock`, MOCK Go API on :19111) |
| `tests/admin/claims-ui.spec.ts`, `claims-request.test.ts`, `claim-source.test.ts`, `claims-model.test.ts` | `--browser-live-claims` |
| `tests/admin/input-delivery.spec.ts`, `studio-input.test.ts`, `studio-input-client.test.ts` | `--browser-input-delivery` |
| `tests/admin/studio-bff.spec.ts`, `studio-request.test.ts` | `--browser-studio-bff` |
| `tests/admin/studio-ui.spec.ts` | `--browser-studio-ui` |
| `tests/admin/orders-bff.spec.ts`, `orders-request.test.ts` | `--browser-merchant-orders-bff` |
| `tests/admin/orders-ui.spec.ts`, `orders-model.test.ts` | `--browser-merchant-orders-ui` |
| `tests/admin/manual-fulfilment.spec.ts`, `refund.spec.ts`, `refund-bff.test.ts` | `--browser-refund-fulfilment` |
| `tests/e2e/deal-loop.spec.ts` | `--browser-e2e` |

The `--browser-admin-legacy` gate replaced a manual five-step procedure (`docs/implementation/
2026-09-20-admin-ledger-acceptance.md` ss "Repeatable local run"): three of its specs had no runner
after the identity work. Its specs write screenshots under `output/playwright/ledger-review/`
(gitignored), no longer into tracked `.impeccable/review/`.

## Node unit suites (no Docker; `bash scripts/dev/test-node.sh`, run by CI)

| Suite | Covers | Note |
| --- | --- | --- |
| `apps/storefront/tests/*.test.mjs` | storefront buyer client/server, payment contract and return | CI, always |
| `packages/i18n/tests/*.test.ts` | locale resolution and catalogs | CI, always |
| `tests/media/r04-input-runner.test.mjs` | R04 local LiveKit input probe | needs `COMMERCE_R04_LIVEKIT_BINARY` (pinned binary). Without it `test-node.sh` prints `NOT_RUN` (CI does); `--require-r04` turns that into exit 2 |

`tests/admin/claims-request.test.ts` and siblings are also run inside their browser mode (table above); `claim.test.mjs`
runs in both places.
