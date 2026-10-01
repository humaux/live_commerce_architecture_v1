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
| T4 deploy smoke | `deploy/scripts/smoke.sh static` (S01-S06 incl. shellcheck) and `smoke.sh full` (S07-S45: images, compose stack, preflight negatives, backup/restore/PITR, edge, S45 Studio planning + claims mounted / media 404) | CI job `deploy-smoke` (`.github/workflows/deploy-smoke.yml`, R1 ruling G3), verdict by `.github/scripts/smoke-verdict.py`: any FAIL, NOT_RUN, never-run case or a BLOCKED other than S29m (F11) fails the job | ubuntu-24.04 runner, root, Docker, free 80/443 |
| T3 browser | real Chromium against the packaged Next apps + Go + PG; MOCK IdP/PSP unless the row says SANDBOX | developers, one mode at a time (`bash scripts/dev/test-local.sh <mode>`); NOT_RUN in CI. `scripts/dev/release-gate.sh` runs every mode in this file (R1 acceptance) | Docker, pnpm, Node 24, Playwright Chromium |

CI is deliberately T0 + T1 plus T4 deploy smoke (owner decision: full foundation plus static checks; ruling G3 added the smoke job). Browser modes are
too slow and machine-bound for every push. T3 rows run in automation only through `bash scripts/dev/release-gate.sh`
(R1 acceptance, one PASS/FAIL/NOT_RUN table), or when a developer invokes a mode.

## Accepted NOT_RUN (R1)

`scripts/dev/release-gate.sh` labels a NOT_RUN row `(accepted: <reason>)` only when every item it names is in
its catalogue (function `accepted_notrun`); every other NOT_RUN is labelled `UNACCEPTED:`, keeps the overall
line INCOMPLETE and makes `--strict` exit 3. The catalogue is exactly:

| Item | Reason (ruling) |
| --- | --- |
| Meta LIVE read-only probes (`TestMetaClaimsMCI11LiveReadOnlyProbes`) | G4: needs the owner's Page token |
| populated-database migration upgrade subtests (`*populated_upgrade*`) | G4: R1's first deploy is a fresh DB; runbook requires them before upgrading a live DB |
| rotated-key refund replay (`TestStripeRF10Sandbox/rotated_key*`) | G4: needs a second Stripe test key |
| SP16 29-minute expiry probe | G4: developer-only |
| SP17 real Stripe webhook delivery (`B-stripe-browser+`) | G4: Dashboard test event after deploy |
| RF11(b) refund in the browser against Stripe SANDBOX | G4: covered by RF10 through the API (G07z) |
| T12 SANDBOX tier (`TestBrowserE2EDealLoopSandbox`) | F12: hosted Stripe page proven by SP18 |
| R04 LiveKit input runner (`G06n+`) | F11: live media is not in R1 |
| Stripe SANDBOX tests skipped in G06/G07 (no key in that process) | run in G07z with the test key |
| `admin-fixture` guard test skipped in G06 | runs in G07 (REAL_PG) |
| smoke static S01 without local shellcheck (`G90`), smoke full (`G91`) | G3: run in CI job `deploy-smoke` |

Smoke S29m BLOCKED is accepted in the CI job (F11), not by release-gate.

## test-local.sh modes

`foundation` (no flag) is T1. Every other mode:

| Mode | Proves | Tier | Run |
| --- | --- | --- | --- |
| `--browser-identity` | isolated PG + signed MOCK IdP browser chain; fixture removed at exit | T3 browser | `bash scripts/dev/test-local.sh --browser-identity` |
| `--browser-password-auth` | PA10 BFF pure logic (Node) + PA11 isolated Next + Go + PG + loopback SMTP fake password sign-up/login/reset browser chain (Chromium desktop + 390px, zh-CN/zh-TW/en); no real mailbox, no owner secret; plus the staff-team independent chain `TestBrowserStaffTeam` (owner invites a fulfilment user from the Team page -> mail captured over the real SMTP adapter -> invitee signs up + accepts -> orders visible, exact fulfilment powers, direct /team and /billing refused -> role to viewer -> revoke -> next request refused; token hygiene; Chromium desktop + 390px, zh-TW + en; MOCK mail; the `@defect` role-aware-nav test is a recorded gap) | T3 browser | `bash scripts/dev/test-local.sh --browser-password-auth` |
| `--browser-admin-legacy` | admin ledger (fixture bearer) + production fail-closed + identity-mock + entry-mock browser suites; no signed IdP, not production acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-admin-legacy` |
| `--browser-buyer` | isolated PG + real buyer browser transport; not UI/PSP/deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-buyer` |
| `--browser-storefront` | SFR01-SFR09 buyer storefront shell on the REAL stack, SANDBOX tier (unit storefront-integration): production Next build -> real private buyerhttp handler -> isolated PG. The shop is built through the real admin HTTP API (28 active products with option axes / compare-at / stock / photos, a draft and an archived product, two collections with one photo, a designed home saved + published and a newer draft behind a preview token, a delivery policy with a free-shipping threshold) and published through the migration 0081 definers (merchant `set_storefront_published`, operator `operator_bind_domain`; no owner-seeded publication/domain row). Real browser (`LC_BROWSER_ENGINE=webkit` = iPhone 15), 390px: home from the published design (photos from PG), SEO on real data (JSON-LD price range + availability, sitemap = active products only, robots, noindex on cart/checkout), collections with the photo URL built from the amended `id` (`/media/c/{id}/{image}`), sort/filter/load-more, variants + stock hints, id->slug 308, draft/archived/removed `/products/Checkout` 404, cart + free-delivery hint from the real threshold, the quote charging the fee below it and 0 above it (`free_shipping_threshold_minor` on every checkout-options row), preview token, unpublish/republish through the definer, locales. Go readback: no order, reservation or ledger row is written by browsing, carts and quotes. Runs the pure-logic `apps/storefront/tests/shop.test.mjs` first. Not proven: real DNS/TLS (synthetic CONNECT edge), a payment provider, devices, Lighthouse/CWV. `LC_SHOP_MOCK=1 bash scripts/dev/test-local.sh --browser-storefront` is the fast MOCK sibling (SF01-SF11 against `tests/storefront/shop-fake-api.mjs`, no PG/Docker) | T3 browser (SANDBOX; MOCK variant) | `bash scripts/dev/test-local.sh --browser-storefront` |
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
| `--browser-customers-billing` | CB11 isolated admin + storefront Next, Go, PG, real worker; customers list/detail/consent/export/erasure, finance, billing standing + redirect; platform billing = MOCK (independent `billingtest` fake); zh-TW + en, desktop + 390px; CB10 SANDBOX and CB12 LIVE are NOT_RUN | T3 browser | `bash scripts/dev/test-local.sh --browser-customers-billing` |
| `--browser-storefront-publish` | R3 storefront-publish KEY gate, production shape (BROWSER, MOCK edge): isolated admin + storefront Next, Go, PG with NO owner-seeded `storefront_publications`/`storefront_domains` row (asserted empty before and re-derived from the audit trail after); the merchant creates product + SKU and publishes/unpublishes with the Settings storefront card (zh-TW + en, desktop + 390px, confirm dialog, stale second tab = 409 and reload), the built `cmd/store-admin` executable (registrar-shaped login) binds, suspends, detaches, refuses a same-evidence re-bind and re-binds DETACHED with renewed proof, and a fresh anonymous buyer browser opens the product page on `https://buyer.example` (sees the product only while published AND bound AND not suspended/detached; otherwise the not-found page); PG readback of publication version, domain row, audit actions and principal; the pure card-model gate (`storefront-model.test.ts`) runs first under `node --test`. Signed MOCK IdP, synthetic TLS/CONNECT edge, the ownership/TLS evidence is an unverified reference: not DNS/TLS, Caddy `on_demand_tls`, `ops-admin.sh` compose wiring (smoke S44) or provider acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-storefront-publish` |
| `--browser-design` | SDB (unit store-design) isolated admin Next + Go API + PG, signed MOCK IdP, Chromium: per (en, zh-TW) x (desktop 1586x992, 390 px) one merchant journey in its own store - profile + logo upload (bad type/size/sniff refused), hero + product_grid + rich_text sections, a page (hostile markdown escaped in the live preview, bad slug blocks save), save draft (one keyed PUT), unsaved-changes guard (stay, leave, clean), publish v1, edit + publish v2, rollback to v1 (history v1, v2, v3 = rollback of v1, draft untouched), reload; SDB02 BFF probes (allowlist, no query, 64 KiB generic vs 260 KiB design cap, CSRF/key/Origin, identity headers never forwarded, media bytes, preview token no-store, foreign stores refused); SDB03 read-only member; SDB04 stale save conflict; PG facts + audit rows + hashed screenshots checked by the Go test; the pure BFF-grammar and markdown XSS node gate (SDN1/SDN2) runs first. Storefront rendering of the document (unit storefront-shell) and the Preview tab on a real storefront origin are NOT_RUN | T3 browser | `bash scripts/dev/test-local.sh --browser-design` |
| `--browser-meta-ads` | MA09a isolated admin Next, Go API + ads worker, PG: connect via fake OAuth, `state_mismatch`, pick-list refusal, draft → approve → publish → pause → copy, allowance-off and SANDBOX banners, report three blocks; MA09b buyer consent → CAPI context on the production storefront build; Meta = MOCK (`tests/ads/fakegraph`); MA-S1..S4 SANDBOX and MA-L1/L2 LIVE are NOT_RUN | T3 browser | `bash scripts/dev/test-local.sh --browser-meta-ads` |
| `--browser-cvs` | TCV08 isolated admin + storefront Next, Go, PG: ECPay map pick, buyer-entered store, pay-at-pickup order, merchant ECPay connect/settings, label request/print, collection + cancel/restock; ECPay = MOCK (`ecpaytest` fake map/Create + signed status posts); the admin logistics BFF grammar/model tests run first under `node --test`; the `WebKit` subtest re-runs this MOCK stack under Playwright WebKit when it is installed (NOT_RUN otherwise); the ECPay SANDBOX variant is NOT_RUN (stage HashKey/HashIV for merchant 2000933 are not provisioned and the stage map is a multi-hop interactive e-map page that is not scripted) | T3 browser | `bash scripts/dev/test-local.sh --browser-cvs` |
| `--browser-catalog-media` | independent catalog-media gate: isolated admin + storefront Next, Go, PG and signed MOCK IdP; the merchant uploads 2 photos through the real file input, reorders, renames the product, changes a SKU price and archives the other SKU in the Ledger, then a fresh anonymous buyer on the shop origin sees the home grid (cover photo bytes, new name, new lowest price), the product gallery in the new order and no archived SKU; zh-TW + en, desktop + 390px; PG readback of rows/hashes. The publication/domain rows are seeded (TODO unit storefront-publish: use its real publish flow); not provider or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-catalog-media` |
| `--browser-ops-polish` | ops-polish independent gates in real Chromium: storefront (production Next -> buyerhttp over `checkout.Service.WithoutCardPayment()` = COMMERCE_BUYER_PAYMENT_ENABLED off -> PG; `buyer_entered` store, no ECPay profile): zh-TW + en x desktop + 390px, the CVS chain is offered, no home option, no card radio, pay-at-pickup pre-selected, order placed (PG: 4 CONFIRMED pay-at-pickup orders, 0 card orders, 0 payment attempts); admin (production Next + Go + PG, signed MOCK IdP): OP2 merchant order feed on Playwright's fake clock (20 s cadence, no overlapping request, paused while hidden, "new" marker, `(N)` tab title, filter/open order kept; zh-TW + zh-CN marker), OP3 finance page column and CSV link in three locales, OP4 Studio subtitle and removed nav entries absent in three locales; the model/copy Node suite runs first. Its PG halves (OP1 options/Begin/replay in process and through the real `cmd/api` assembly, OP3 finance + CSV in both environment branches, Taipei boundary, REFUNDED_OFFLINE) are `TestOpsPolish*` in the default foundation run. MOCK only: no PSP exists in the buyer stack, ECPay and Stripe are fakes; not provider or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-ops-polish` |
| `--browser-promotions` | independent promotions gate (R4 test author; storefront-v2 §F): isolated admin + storefront Next, Go, PG and signed MOCK IdP; the merchant creates a 10% code with a NT$500 minimum and a Taipei-time window (and a NT$5,000-minimum code) in `/promotions`; an anonymous buyer adds 2 products in the new storefront, an unknown code and the below-minimum code show their messages and change nothing, the real code is applied by the server quote (discount, shipping untouched, total, footer total), removed and re-applied, then a bank_transfer order is placed and the buyer submits the transfer details; the merchant order detail and bank-transfer panel show the discounted amounts, the merchant confirms, finance shows the confirmed amount and `/promotions` the usage; zh-TW + en, desktop + 390px; PG readback of code window (Taipei +08:00), order snapshot, redemption, confirmed amount, finance. BROWSER, MOCK IdP, bank_transfer (no PSP); Stripe SANDBOX and WebKit are NOT_RUN here | T3 browser | `bash scripts/dev/test-local.sh --browser-promotions` |
| `--browser-webkit` | real Safari engine (Playwright WebKit, LC_BROWSER_ENGINE=webkit): the buyer-critical MOCK flows `buyer`, `order`, `payment`, `merchant-buyer`, `cvs` (TCV08 buyer + merchant halves) and `password-auth`, one go test process and fresh PG per step; phone-sized buyer contexts use the iPhone 15 profile, desktop contexts and admin pages Desktop Safari (admin behind a self-signed https front, because WebKit refuses `__Host-` cookies on http loopback). The cvs step also asserts that no visible text control on the iPhone profile is under 16px (iOS Safari focus-zoom). Refuses with exit 2 NOT_RUN when WebKit is not installed (`pnpm exec playwright install webkit`); `LC_WEBKIT_STEPS=payment,cvs` narrows it for debugging, release-gate row `B-browser-webkit` requires all six. Stripe SP18 on WebKit (SANDBOX) is `LC_BROWSER_ENGINE=webkit STRIPE_BROWSER=1 STRIPE_SANDBOX=1 bash scripts/dev/test-local.sh --stripe-browser`. Not real-device iOS, provider or deployment acceptance | T3 browser | `bash scripts/dev/test-local.sh --browser-webkit` |
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
| `tests/admin/password-auth.spec.ts`, `password-bff.test.ts` | `--browser-password-auth` (suite `password-auth`, started by `TestBrowserPasswordAuth`; PA10 runs first under `node --test`) |
| `tests/admin/staff-team.spec.ts` | `--browser-password-auth` (suite `staff-team`, started by `TestBrowserStaffTeam`, same loopback-SMTP harness; `LC_STAFF_TEAM_SPEC_ARGS="--grep-invert @defect"` isolates the known-gap test) |
| `tests/admin/auth.spec.ts` | `--browser-admin-legacy` (suite `identity-mock`, MOCK Go API on :19111) |
| `tests/admin/entry.spec.ts` | `--browser-admin-legacy` (suite `entry-mock`, MOCK Go API on :19111) |
| `tests/admin/claims-ui.spec.ts`, `claims-request.test.ts`, `claim-source.test.ts`, `claims-model.test.ts` | `--browser-live-claims` |
| `tests/admin/input-delivery.spec.ts`, `studio-input.test.ts`, `studio-input-client.test.ts` | `--browser-input-delivery` |
| `tests/admin/studio-bff.spec.ts`, `studio-request.test.ts` | `--browser-studio-bff` |
| `tests/admin/studio-ui.spec.ts` | `--browser-studio-ui` |
| `tests/admin/orders-bff.spec.ts`, `orders-request.test.ts` | `--browser-merchant-orders-bff` |
| `tests/admin/orders-ui.spec.ts`, `orders-model.test.ts` | `--browser-merchant-orders-ui` |
| `tests/admin/manual-fulfilment.spec.ts`, `refund.spec.ts`, `refund-bff.test.ts` | `--browser-refund-fulfilment` |
| `tests/admin/customers-billing.spec.ts`, `customers-bff.test.ts`, `customers-model.test.ts`, `customers-request.test.ts`, `billing-model.test.ts` | `--browser-customers-billing` |
| `tests/admin/ads.spec.ts`, `ads-model.test.ts`, `ads-request.test.ts`; `tests/storefront/ads-consent.mjs` | `--browser-meta-ads` |
| `tests/admin/taiwan-cvs.spec.ts`, `logistics-model.test.ts`, `logistics-request.test.ts`; `tests/storefront/cvs-buyer.mjs` | `--browser-cvs` (and its cvs step under `--browser-webkit`) |
| `tests/storefront/catalog-media-gate.mjs` (driven by `TestBrowserCatalogMedia`) | `--browser-catalog-media` |
| `tests/storefront/shop-real-gate.mjs` (driven by `TestBrowserStorefront`; the MOCK sibling `tests/storefront/shop-gate.mjs` runs under `LC_SHOP_MOCK=1`) | `--browser-storefront` |
| `tests/storefront/browser-gate.mjs`, `order-gate.mjs`, `buyer-payment-browser.mjs`, `privacy-buyer.mjs`, `cvs-buyer.mjs`, `refund-buyer.mjs`, `shipment-buyer.mjs`, `payuni-ui-baseline.mjs`, `stripe-browser.mjs`, `ads-consent.mjs`, `merchant-buyer-gate.mjs`, `tests/e2e/deal-loop.spec.ts` | start on the storefront shell since unit storefront-integration (`tests/storefront/shop-helpers.mjs`: `reachCheckout` product page -> Add to cart -> `/{locale}/checkout`, `switchLocale` = the footer language link) |
| `tests/storefront/promotions-gate.mjs` (driven by `TestBrowserPromotions`, `tests/foundation/browser_promotions_test.go`) | `--browser-promotions` |
| `tests/admin/storefront-model.test.ts`; `tests/storefront/storefront-publish-gate.mjs` | `--browser-storefront-publish` (the Node model test runs first under `node --test`; the `.mjs` driver is started by `TestBrowserStorefrontPublish`) |
| `tests/admin/design.spec.ts`, `design-gate.test.ts`, `design-model.test.ts` | `--browser-design` (suite `store-design`, started by `TestBrowserStoreDesign`; the node tests run first under `node --test`) |
| `tests/admin/ops-polish.spec.ts`, `ops-polish-model.test.ts`; `tests/storefront/cvs-pap-only.mjs` | `--browser-ops-polish` |
| `tests/e2e/deal-loop.spec.ts` | `--browser-e2e` |

The `--browser-admin-legacy` gate replaced a manual five-step procedure (`docs/implementation/
2026-09-20-admin-ledger-acceptance.md` ss "Repeatable local run"): three of its specs had no runner
after the identity work. Its specs write screenshots under `output/playwright/ledger-review/`
(gitignored), no longer into tracked `.impeccable/review/`.

## Focused PG gates without a mode

`tests/foundation/catalog_media_test.go` (`TestCatalogMedia*`, REAL_PG: migration 0082 surface/RLS/definers, upload -> buyer catalog -> public bytes -> Meta feed `image_link`, reorder/delete/8-photo cap/idempotency, magic-byte sniffing, BCAT07 same-404, media route authority, archived product) runs inside the T1 foundation suite; focused loop: `bash scripts/dev/test-focused.sh '^TestCatalogMedia'`.

## PG gates that are not a test-local.sh mode (R3 storefront-publish, REAL_PG, evidence label MOCK for the synthetic proof)

`tests/foundation/storefront_publish_test.go` runs in T1 (full suite, CI) and focused with `bash scripts/dev/test-focused.sh '^TestStorefrontPublishSPW'`:
SPW01 schema/ACL of migration 0081 (definer owner, `search_path`, PUBLIC revoked, writer RLS bounds); SPW02 0081 as an upgrade of the 0080 head (own PG; the
whole R2 set is `TestR2IntegrationUpgradeFromReleaseHead`, now 14 files); SPW03 merchant scope from server auth, cross-store/tenant refusal, permission
re-check inside the definer; SPW04 compare-and-set 409, no-op rules, audit rows, HTTP statuses, no merchant domain route; SPW05 operator lifecycle
ACTIVE -> SUSPENDED -> DETACHED -> re-bind (renewed proof) with `buyer.resolve_published_store` checked after every step; SPW06 operator input
boundaries in SQL (origin grammar, evidence, 400-day `valid_until`); SPW07 EXECUTE matrix across every other service login shape; SPW08 `lc_store_registrar`
login shape against the provision-logins membership matrix; SPW09 `cmd/store-admin` built and run as a process (exit codes, one JSON line, fixed stderr codes, no DSN leak);
SPW10 static deploy wiring (manifest, compose, ops-admin allowlist, provision-logins). NOT_RUN here: smoke S13/S44 (compose stack, CI job `deploy-smoke`).
## Focused real-PG gates (no test-local.sh mode; `bash scripts/dev/test-focused.sh '<regex>'`, also part of the T1 foundation run)
| Test | Proves | Run |
| --- | --- | --- |
| `TestStoreDesignGate` (tests/foundation/store_design_gate_test.go) | store-design REAL_PG gate over the real admin (bearer) and buyer (BFF key + origin) handlers: SD01 isolation (tenant/store/permission/session on every route, RLS, media, preview, origins), SD02 publish/save/rollback races + media cap race, SD02b delete-vs-reference race, SD03 preview token expiry/staleness/binding/hash at rest, SD04 422 `details.path` for unknown keys, caps, enums and XSS payloads in every string/URL/markdown field, SD04b plain-text fields carry no HTML, SD05 media sniff/size/60 cap/delete-while-referenced, SD06 append-only history + audit, SD07 buyer routes refuse bearer/cookie/query/method/origin + unpublished 404 + normalised shape, SD08 envelope/CAS/size cap | `bash scripts/dev/test-focused.sh '^TestStoreDesignGate$'` |
| `TestStoreDesignRealPG` (tests/foundation/store_design_test.go) | the implementer's author smoke (happy path), not acceptance | `bash scripts/dev/test-focused.sh '^TestStoreDesignRealPG$'` |

## Node unit suites (no Docker; `bash scripts/dev/test-node.sh`, run by CI and release-gate G06n)

| Suite | Covers | Note |
| --- | --- | --- |
| `apps/storefront/tests/*.test.mjs` | storefront buyer client/server, payment contract and return | CI, always |
| `packages/markdown-lite/tests/*.test.ts`, `tests/admin/design-model.test.ts`, `tests/admin/design-gate.test.ts` | store-design pure logic: restricted-markdown renderer + validator (incl. XSS payload sweep and seeded fuzz), editor model, admin BFF allowlist grammar and the 260 KiB cap constant | CI, always (also run first by `--browser-design`) |
| `apps/storefront/tests/*.test.mjs` | storefront buyer client/server, payment contract and return; `promo.test.mjs` = discount-code contract, quote validator, journal grammar and BFF refusal relay (storefront-v2 §F, MOCK) | CI, always |
| `tests/admin/promotions-model.test.ts` | admin discount-code parser, Taipei-time conversion, form-to-body builders, BFF route grammar and copy parity (storefront-v2 §F, MODEL_ONLY) | CI, always (named in `test-node.sh`) |
| `packages/i18n/tests/*.test.ts` | locale resolution and catalogs | CI, always |
| `tests/admin/team-model.test.ts`, `tests/admin/team-bff.test.ts` | staff-team: request/answer grammar (author) and the real `/api/team/[action]` route against a loopback Go fake: exact Origin, double-submit CSRF, no query, strict body, BFF key + bearer forwarding, one call and never a retry, allow-listed error rebuild, token never echoed (independent gate) | CI, always |
| `tests/admin/notify-model.test.ts` | buyer-comms: new-order mail opt-out model, BFF grammar entry, three-locale copy parity | CI, always (listed in `test-node.sh`) |
| `tests/media/r04-input-runner.test.mjs` | R04 local LiveKit input probe | needs `COMMERCE_R04_LIVEKIT_BINARY` (pinned binary). Without it `test-node.sh` prints `NOT_RUN` (CI does); `--require-r04` turns that into exit 2 |

`tests/admin/claims-request.test.ts` and siblings are also run inside their browser mode (table above); `claim.test.mjs`
runs in both places.

## Focused PG gates of unit promotions (storefront-v2 §F, migration 0091; REAL_PG, no PSP, no browser)

Run with `bash scripts/dev/test-focused.sh '^TestPromotion'` (also part of the default T1 foundation run). Pure logic runs without Docker:
`go test ./internal/pricing ./internal/promotions ./internal/buyerhttp ./internal/httpapi`.

| Test | Proves |
| --- | --- |
| `TestPromotionQuoteApplication` | a code (any case) is priced inside the quote, frozen in the snapshot, shipping untouched, fixed amount capped at the subtotal, every typed refusal is a coded 422 through the real buyer route and stores no quote |
| `TestPromotionAtomicLimit` | 6 concurrent placements of a `total_limit` 1 code place exactly one order (losers roll back whole); expiry releases the use with no hook |
| `TestPromotionPerBuyerAndChange` | per-buyer limit by phone identity across two capabilities; `promo_changed` after a pause and `promo_expired` after a natural expiry leave zero facts |
| `TestPromotionOrderMoney` | order total = discounted quote total, redemption + snapshot + merchant order view carry the discount; a free-shipping-threshold policy still places (RevalidateQuote pointer-compare regression) |
| `TestPromotionAdminGuards` | permissions, idempotent replay, duplicate code, stale version, rule violations, audit rows |

Not covered here (NOT_RUN): browser pages for `/[locale]/promotions` and the storefront code field, real WebKit, and a Stripe SANDBOX payment of a discounted order.
## Real-PG gates of the R4 wave (T1 foundation; focused loop `bash scripts/dev/test-focused.sh '<regex>'`)
| Test | Proves (REAL_PG, MOCK mailbox) | Focused run |
| --- | --- | --- |
| `tests/foundation/staff_team_gate_test.go` `TestStaffGate*` (SG01-SG08) | staff-team independent gate from contracts/storefront-v2.md §D: accept refusals identical and non-consuming (unknown/malformed/wrong e-mail/OIDC-only/expired/revoked/used), single use under concurrency and against revoke, 72 h CHECK, token bound to its store + role; the five role bundles exactly as §D for every permission of the live CHECK plus refund/billing/order-actions over the real HTTP handler; owner floor through the product, under concurrency and by direct DML (owner pool and `commerce_staff_writer`, deferred trigger); app logins cannot write staff tables; role change and revoke effective on the next request (concurrent hammer); only the owner manages the team, tenant/store scope server-side; token never in SQL text, bound args, rows, logs or API URLs; 17-action audit; mail locale/link/resend. NOT the author smoke `staff_team_smoke_test.go`. | `bash scripts/dev/test-focused.sh '^TestStaffGate'` |
## Buyer communications (unit buyer-comms, contracts/storefront-v2.md §E, migration 0090)
| Gate | Command | Label | Proves / does not prove |
| --- | --- | --- | --- |
| Go units | `go test ./internal/notify ./internal/buyerhttp ./cmd/expiry-worker` | MOCK | renderer (6 kinds x 3 locales, escaping, link rules), worker recording against a fake queue and mailer, lookup normalisation and route table, worker env validation. No database, no SMTP |
| PG smoke | `bash scripts/dev/test-focused.sh 'TestBuyerCommsOutbox\|TestGuestOrderLookup\|TestNotifySettingsRoute'` | REAL_PG + MOCK mail | triggers inside the real order transactions, exactly-once keys, caps, UNKNOWN / retry / stale / erasure rows, guest lookup through the real buyer HTTP handler (match, identical 404, throttles, working capability), merchant toggle route. The mailer is a fake: no SMTP wire |
| Node units | `bash scripts/dev/test-node.sh` | MOCK | storefront lookup contract + BFF route (`apps/storefront/tests/lookup.test.mjs`), admin toggle model |
NOT_RUN: real SMTP delivery of buyer mail (the SMTP adapter has its own MOCK gate in `internal/mail`), a browser run of `/[locale]/orders/lookup`
and `/[locale]/orders/{id}`, any SANDBOX or LIVE mailbox. The independent tester owns the gate tests; the PG smoke above is the author's.

## Independent focused PG gates of unit promotions (R4 test author; storefront-v2 §F; REAL_PG, Stripe lines MOCK)

`tests/foundation/promotions_gate_test.go`, written from the contract and not from the implementation or the author smoke above. Run with
`bash scripts/dev/test-focused.sh '^TestPromoGate'` (also part of the default T1 foundation run). Each gate has one temporary-mutation red run on record
(`output/promotions/tests/R*.log`, see `output/promotions/tests/SUMMARY.md`).

| Test | Proves |
| --- | --- |
| `TestPromoGateMath` | percent 1/90 and fixed boundaries, the TWD whole-dollar floor (never up), a zero discount refused `promo_invalid`, a fixed code above the cart capped, goods never below zero, total = subtotal - discount + shipping, line discounts proportional and summing to the discount, admin bounds |
| `TestPromoGateWindowAndMinimum` | minimum subtotal inclusive and compared with the PRE-discount subtotal (quote and Begin); `+08:00` Taipei wall times are real instants (valid, ended, not started); starts_at/ends_at crossed in real time |
| `TestPromoGateShipping` | a code never discounts shipping (even one worth more than the cart); the free-shipping threshold compares the PRE-discount subtotal at, above and one unit below it |
| `TestPromoGateOneCode` | a second code replaces the first, lists/combined strings/unknown keys refused, client-supplied amounts and ids never change the price, one redemption per order |
| `TestPromoGateBeginErrors` | used up / expired / edited between quote and Begin: coded 422 through the real buyer route, zero facts, a re-quote then places |
| `TestPromoGateConcurrency` | 10 racing placements of different SKUs on a `total_limit` 3 code admit exactly 3; 6 racing placements of one identity on `per_buyer_limit` 1 admit exactly 1; sequential limit 2 |
| `TestPromoGateBankTransfer` | per-buyer identity by e-mail and by phone, a new identity admitted; the discounted total reaches the buyer's transfer amount, the merchant order view (totals + lines), finance JSON/CSV, the confirmed amount and the offline refund |
| `TestPromoGateRelease` | bank-transfer expiry frees a use with no hook; a CONFIRMED and an offline-refunded order keep counting |
| `TestPromoGateStripeRefundCap` | card order through the real capture path (Stripe MOCK): captured = discounted total, Stripe session amount = discounted total, refund cap = paid amount (pre-discount and +1 dollar refused), finance captured |
| `TestPromoGateAdminAuthority` | 401/403 for no, buyer, expired and revoked sessions; the five role bundles (owner/admin write, viewer reads, fulfilment and live_operator neither); no row written by a refused role; audit rows carry the principal |
| `TestPromoGateIsolation` | same code text in two tenants and two stores; list/create/update refused across tenant and store and never leaked; a buyer only meets his own store's codes (indistinguishable from unknown) |

`TestPromoGateStripeRefundCap` sets `pgStripeRevoke` (payment_runtime_test.go) to work around BASE defect B1 (two trigger functions keep PUBLIC EXECUTE and every
Stripe login fails its privilege validation); see `output/promotions/tests/DEFECTS.md`.
