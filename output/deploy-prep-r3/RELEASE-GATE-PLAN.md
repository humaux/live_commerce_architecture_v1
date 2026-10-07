<!-- Purpose: dry-run release acceptance plan; no deployment authorization. Depends on: brief, source DELIVERY notes, release-gate.sh, gates.yml and deploy-smoke.yml. Used by: Claude Opus integrator on the final release SHA. -->
# deploy-prep-r3 release gate plan

DESIGN plan only. Owner: 「等全部做完後再上線」. Prepare every pending unit, merge through Claude Opus, then bind
CI receipts/review to one final release SHA. A unit branch's focused PASS cannot clear release gates. No SSH, host
access, live keys, push or deployment in deploy-prep-r3. All locally executed cases are MOCK/static.

| Required release gate | Execution / acceptance |
| --- | --- |
| G01–G06, G06n | `bash scripts/dev/release-gate.sh --strict --only G01,G02,G03,G04,G05,G06,G06n`: packet, Go build/vet/gofmt, TS, secret grep, generated dependency map, Go/Node units. No unexpected skip, stale map or untracked gate source. |
| G07 | `bash scripts/dev/release-gate.sh --strict --only G07` on GitHub: **full** foundation/REAL_PG, mandatory for all R3 migration/definer work and again after integration. Include the new operator login/provisioning matrix; no focused regex substitute. |
| Registered B-* | Every applicable registered browser mode emitted by `release-gate.sh --list` on the final SHA must pass in GitHub, including real clicks/persistence; later UI units add their own acceptance modes before release. |
| G90 / deploy static | `bash deploy/scripts/smoke.sh static`: S01–S06/S48. `.github/workflows/deploy-smoke.yml` must include shellcheck and reject missing/NOT_RUN cases through `.github/scripts/smoke-verdict.py`. |
| G91 / deploy full | `sudo bash deploy/scripts/smoke.sh full` in **isolated GitHub CI**, via `.github/workflows/deploy-smoke.yml`, Linux/root/free80/443. S44 platform operator startup/readback, S49 exact route refusals, migration/login initialization and rollback/restore groups must pass. No production target. |
| G99 | Final full strict release run: tests must not alter tracked source/design files; archive all evidence against the release SHA. |

Integrator-run `.github/workflows/gates.yml` mode set from the twelve delivery notes (all listed modes exist on base):

```json
["unit","foundation","--meta-consumer","--payment-worker","--payment","--checkout","--merchant-orders","--expiry-worker","--operations-queue","--stripe-browser","--browser-admin-shell","--browser-password-auth","--browser-storefront","--browser-payment","--browser-refund-fulfilment","--browser-product-editor","--browser-catalog-media","--browser-click-sweep","--browser-visual-lint","--browser-customers-billing"]
```

Run full foundation once on the final SHA plus the impacted modes above; no local multi-mode/PG/browser batch.
OPS-01B/02B requires store/tenant suspension, support permissions/principal enrollment and record preservation pins.
Stripe platform/settlement requires SP/RF/SL regression, PF14, read-only sync and payout-record idempotency. Operations
ledger covers UNKNOWN reconcile-first/query budget/CAS; ads unbind covers in-flight/binding-in-use refusals and feed
info. Returns/imports/live-price/keyword models must be included in foundation with their current tests:
`TestReturns`, `TestCustomerImport`, order-history import, live-price-on-pause and keyword tools suites.

Two source deliveries request modes **absent on this base**: `--returns` and `--migration-import`. These are integrator
tasks to register in `test-local.sh` + `docs/delivery/GATES.md` or prove the complete suites run inside G07. Do not dispatch
unknown modes or accept an empty run. Likewise `--identity` is not a registered mode; its regression is covered by
foundation and the actual password/admin-shell modes. Source requests about migration numbers/upgrade counts, shared
OpenAPI/schema and support-order PII/consent remain in CHECKLIST; re-read appended source review resolutions before
calling an old note unresolved. Before LIVE, settle `unmapped_source` via an approved append-only resolution workflow.

Accepted NOT_RUN is limited to the checked-in release-gate catalogue and explicit owner/integrator evidence:

| NOT_RUN boundary | Reason / follow-up |
| --- | --- |
| G07z, PF-SBX, Stripe browser sandbox branches | Owner-authorized sandbox keys absent in this code-only unit; never invent a key or substitute MOCK. R3 platform collection/settlement sandbox acceptance must be tracked separately by integrator when keys are supplied. A mode may PASS its MOCK portion while provider cases remain NOT_RUN. |
| LIVE Stripe/Meta/carriers, actual transfers/ads/messages | Owner approval and real accounts outside unit scope. No LIVE test/deploy or new acceptance waiver here. Must remain labelled NOT_RUN, never PASS. |
| PAYUNi | Cancelled by owner; disabled knob + preflight refusal, no ingress credentials or notify route deployment. No PAYUNi activation gate requested. |
| Existing catalogue entries (e.g. S29m media) | Preserve current documented F11 and other recorded owner exceptions. This document creates no new exception. |
| Local shellcheck and full deploy smoke | Local shellcheck absent and full smoke heavy/Linux-only. G3 accepts local NOT_RUN only because deploy-smoke CI runs both; **CI NOT_RUN/missing-case is a failure**. |

Final release receipt: source SHA/tree, workflow run URLs, group/mode exit codes, unexpected skips zero, review verdict,
backup/restore readiness and each remaining owner-only prerequisite. Only then request a separate owner approval for
release execution. Engineering preparation and CI passing do not authorize deployment.
