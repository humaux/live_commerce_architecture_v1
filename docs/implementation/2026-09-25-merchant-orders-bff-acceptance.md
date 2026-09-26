# Merchant order BFF — evidence register

2026-09-25, final replay 2026-09-26. **PASS_BOUNDED_LOCAL_TRANSPORT**. This connects the accepted
merchant order backend to the existing authenticated browser transport; it is
not an order page, shipping/refund capability or production deployment.
[Frozen contract MBT01–04](../../contracts/merchant-orders-bff-v1.md).

## Ownership and implementation

- Root `codex-commerce-build-20260920` owns contract, runner, Playwright suite
  registration and integration. Frozen `e13a858`; runner `ff458d5`.
- `merchant_orders_bff`, ui_worker, gpt-6-sol/medium, isolated
  `/Volumes/data/live-commerce-merchant-orders-bff`, base `e13a858`, produced
  `1151241`; root integrated as `5d63b26`. Only the catch-all route changed.
- `merchant_orders_bff_test`, test_worker, gpt-6-sol/high, isolated
  `/Volumes/data/live-commerce-merchant-orders-bff-test`, base `e13a858` with
  explicit runner and implementation prerequisites; owns independent test files.
- `hosted_contract_security_review` independently reviewed frozen contract and
  source, retaining its existing inherited session settings (no new model
  override). Preflight `12efd2a6-e5ec-4afb-b3e3-1d6b132bc4c0`; source and real-chain
  review `8481c198-2602-48dc-b854-932baec30443`; final test/log supplement
  `916c939c-83df-4c3a-9f82-464f8d677016`. No open P0/P1/P2 in this bounded change.

Author source fix `06883c6` is integrated as `0f71453`: official
`skipProxyUrlNormalize`, API Proxy ingress guard and one shared
`validOrdersQuery`, reused by the route. Proxy performs syntax checks, not
identity/tenant authorization. Decoding the path once for order-route recognition
also covers encoded resource names/slashes; malformed paths do not gain authority.
The pure-validator check runs with the dedicated gate (`f9d31bd`).

The route reuses existing HttpOnly session extraction, authenticated store list,
server-only Bearer construction, safe errors and cookie clearing. It adds two
read paths, a bounded raw-query validator and `private, no-store` successes; no
new authentication/client layer, provider, dependencies or database changes.
The fixture bearer is explicitly unavailable for these recipient-data reads.

## Evidence

Root strict admin typecheck at `5d63b26`: exit 0,
`/Volumes/data/output/merchant-orders-bff-root-types-1.log`.
Author typecheck/build/format passed; not substituted for independent runtime
acceptance. Root existing identity/settings/process-restart browser regression:
3 top-level tests PASS, foundation 12.133s, exit 0, at
`/Volumes/data/output/merchant-orders-bff-root-identity-1.log` (SHA256
`38001fe425434d5efb3c49af09e57d992cb1babd7ea836c64061963cc896c479`).
Root merchant-to-buyer production-build regression: 8 cases / 3 locales PASS,
foundation 5.612s, exit 0, at
`/Volumes/data/output/merchant-orders-bff-root-purchase-entry-1.log` (SHA256
`04b5b93439895cbc38d90d9b4a62168796ea641b913f67bd08deeda048fec47d`).
Both use actual Next→Go→disposable PostgreSQL; only the external identity issuer
is mocked. These are existing-flow regressions, not order-page acceptance.
Independent tests `58fb4cd`, `af8d1a9`, `1003c6b`, `80224c4` are integrated through
root `5cd40fa`. Author fixed-source MBT run: foundation 5.686s, exit 0.
Root actual Next→Go→PG repeat `real-3`: 2 Node tests and 1 combined Playwright
scenario PASS; foundation 5.933s, exit 0. Log
`/Volumes/data/output/merchant-orders-bff-root-real-3.log`, SHA256
`9bbf985a4584b507c71bb8a3b5f5f896dd681bac1fe7de8678548c81e15e7371`.
Artifacts: `output/playwright/merchant-orders-bff-20260925T052504.868771000`.

After the ingress fix, root repeated both existing browser gates:

- Identity/settings/process restart: 3 top-level PASS, foundation 16.584s, exit 0.
  Log `/Volumes/data/output/merchant-orders-bff-root-identity-2.log`, SHA256
  `98a03ffc350f61337a3944ec1ca223b445457aaa8970daa8293c7355ebe963c0`.
- Merchant-to-buyer: 8 cases / 3 locales PASS, foundation 5.014s, exit 0.
  Log `/Volumes/data/output/merchant-orders-bff-root-purchase-entry-2.log`, SHA256
  `88ed033d2cde00c2e360b779ecb5a6734e3607917139b55d4d6c85420ce4d0c2`.

Strict admin TypeScript and 4 i18n Node tests also passed. No Go production code or SQL changed in this slice;
the prior 458-test backend result is a prior baseline, not a fresh run here.

### Final 2026-09-26 replay

Author `c7db16a` (root `cb1a55a`) adds only two encoded-prefix probes and four
unprefixed-locale redirect probes. The previous turn stopped on an agent usage
limit; its uncommitted changes were preserved, independently inspected, then
the complete command was rerun before submission. No production source changed.

- Author full command: exit 0, foundation 8.348s; one Go integration test contains
  one combined Playwright scenario. Log
  `/Volumes/data/live-commerce-merchant-orders-bff-test/output/playwright/merchant-orders-bff-final-20260926.log`,
  SHA256 `d9fdc99f77cdfbeb4ed7a0db0633c6d9a3a49460d6b35b77c8d94eaf63d15394`.
- Root exact `cb1a55a`: exit 0, 2 Node tests PASS, one Go integration test PASS
  (5.02s; foundation 6.924s), one combined Playwright scenario PASS (1.4s suite).
  Log `/Volumes/data/output/merchant-orders-bff-root-final-20260926.log`, SHA256
  `1748789b361a7361c6a680049fe61e2f581a9354a44ef316e14bd96ec1fadd50`.
  Artifacts: `output/playwright/merchant-orders-bff-20260926T090616.761897000`.
- Fresh strict admin TypeScript exit 0:
  `/Volumes/data/output/merchant-orders-bff-root-types-20260926.log`.

Encoded prefixes cannot reach the order backend. For the prefix that receives
a 307, the test checks Location's protocol/port and loopback host, then requests
that pathname/query from the configured local test service and requires rejection
with unchanged order-call count. This is not a literal-origin redirect-following
or production-domain test. Language probes cover en/zh-CN/zh-TW and cookie priority,
preserve the query and assert private/no-store without invoking orders.

The gate also proves signed-login issuance via its actual session event, real
same-store cursor pagination, cross-store isolation, all-method upstream counts,
fixture-only denial with a working warehouse positive control, safe errors, both
auth-cookie removals, and unchanged order/payment/inventory/event/queue facts.
The task-owned servers and labelled PG container were closed; logs and worktrees
remain. No customer, PSP, shipment or production call was made.
Seven source/test files were indexed (42 entities, zero rejected); the final test
update was indexed separately. The shared validator and Go gate are linked to
the root-cause record. Packet-structure and 85 local-document links passed; these
checks do not substitute for the runtime evidence above.

### Failures retained while closing the gate

- Root `244b37c` first dedicated run failed before tests because the initial
  harness referenced nonexistent expired/revoked fixture tokens. Log:
  `/Volumes/data/output/merchant-orders-bff-root-real-1.log`. This is a test-fixture
  defect, not proof of an application failure; the author created explicit
  expired/revoked sessions in `af8d1a9`.
- The author's corrected actual-Next run reached the application and exposed
  ingress query normalization: bare `?`, trailing `&`, double `&&`, encoded
  name/value and detail bare `?` returned 200 and reached the order backend.
  Author evidence:
  `/Volumes/data/live-commerce-merchant-orders-bff-test/output/playwright/merchant-orders-bff-20260925T051656.269579000/playwright.log`.
  A plain `new Request(...)` test did not model this framework boundary.
  Preserve strict MBT02; fix validation before the normalization rather than
  changing the rejected inputs to accepted ones. Fixed by `06883c6` and verified
  in the root `real-3` run above.
- Independent test review requires all-method upstream counting, a separate
  actual fixture-only Next process, and exact session plus CSRF cookie clearing.
  These were added and exercised by `real-3`, not satisfied by source review alone.
- Root `real-2` passed its Playwright scenario but failed the final session count
  (3 instead of 1): the new manually seeded expired/revoked sessions shared the
  principal. `80224c4` now counts the genuine `session.issued` receipt instead of
  accepting 3. Log `/Volumes/data/output/merchant-orders-bff-root-real-2.log` is
  retained. The fixture's generated Next type file is restored only when its
  content equals the known dev rewrite; unrelated changes are preserved.

The framework configuration is documented by
[Next.js Proxy advanced flags](https://nextjs.org/docs/app/api-reference/file-conventions/proxy#advanced-proxy-flags).
The installed version and actual HTTP behavior, not documentation alone, decide
whether the fix passes. The final review covers this transport change only.

## Design boundary and next gate

The order-page audit and three composition choices are separately recorded in
[the visual decision record](2026-09-25-merchant-orders-ui-audit.md). No UI choice
has been received. The decision page closed unanswered; a structured question
remains with A side-by-side, B sequential receipt and C inline expansion.
Generated comps and exact embedded prompts are committed; they are unapproved.
Generated labels that exceed current backend enums are not functionality claims.

After choice: implement the selected layout using this transport, with real
store selection, no persisted address data, stale-result suppression across
store/session/locale changes, then actual desktop/mobile/three-locale browser
and independent visual gates. Do not infer that this transport gate covers them.
