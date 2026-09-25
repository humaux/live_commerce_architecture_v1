# Buyer public payment transport — bounded acceptance

2026-09-25. **PASS_BOUNDED_NODE_TRANSPORT**, not a payment UI/provider release.
Contract `buyer-payment-public-v1.md` frozen at `a78a108` after independent
preflight `860e8256-e436-470e-9265-37d373e359bb`.
Production implementation `145f05c` (root `c9f380b`); independent tests
`dc93caa` + `acc6325` (root final tested tree `f3de924`). Later root source edits
are explanatory comments only. No Go/SQL/module/runner changes in this slice.

## Delivered boundary

The existing public buyer BFF now admits exactly the owned order payment GET,
keyed prepare POST and no-body/no-key one-shot handoff POST. Origin, context,
signed HttpOnly cookie, private capability and single-fetch checks are reused.
Every handoff error is nonretryable, including errors before route admission.

New shared pure TypeScript guards reject extra/private fields, wrong order IDs,
invalid enums/amounts/calendar dates, duplicate response JSON and hostile form
actions or fields. Prepare is constrained to current TWD whole-dollar admission;
the generic historical order view keeps its broader monetary range. Only an
ISSUED receipt carries the exact bounded form; ALREADY_ISSUED carries none.
No form is persisted by this transport; no new dependency or payment engine.

## Independent work and root verification

Code writer: `integration_worker`, delegated `gpt-6-sol/high`, worktree
`/Volumes/data/worktrees/commerce-buyer-payment-public-20260925`, base `a78a108`.
Write ownership: only `buyer-server.ts`, `buyer-client.ts`, `payment-contract.ts`.
Independent test writer: separate test worktree/base, same delegated model class,
only three new `.mjs` tests and its [author evidence](2026-09-25-buyer-payment-public-test-author.md).
The authors did not alter frozen gates or each other's files.

Root independently ran:

| Gate | Observed result | Evidence in `/Volumes/data/output/` |
| --- | --- | --- |
| Pre-change storefront Node baseline | 34 pass | `buyer-payment-public-baseline-node.log` |
| Final storefront Node suite | 45 pass, 0 fail, 0 skip; includes 11 new independent tests | `buyer-payment-public-root-node-2.log` |
| `pnpm run typecheck:storefront` | exit0 | `buyer-payment-public-root-typecheck-1.log` |
| Disabled-root production `pnpm run build:storefront` | exit0 | `buyer-payment-public-root-build-1.log` |
| `bash scripts/dev/test-local.sh` | exit0; 421 pass, 0 fail, 0 skip; actual isolated PG18, Go race and vet; foundation174.239s | `buyer-payment-public-root-full-1.log` |
| Existing B order/history browser regression `--browser-order` | exit0; 23 actual cases, 6 buyers/7 orders with exact order/hold/job/receipt/reserve counts; case15.58s | `buyer-payment-public-root-browser-regression-1.log` |

The full Go gate ran at `a78a108` before the TS merge; `git diff --quiet` confirms
Go, SQL, module files and the runner unchanged through `f3de924`. This is current
backend regression evidence, not an execution of the new TypeScript in a browser.
Typecheck/build ran on `c9f380b`; later changes were tests/docs/comments only.
After explanatory comments, root repeated all 45 Node tests successfully
(`buyer-payment-public-root-node-3.log`, SHA-256
`230a1d9201be94e8feb98d917ae211bc882c715653ddf6370ca43d845f6afa81`).
The browser regression rebuilt production Next and exercised the existing B
address/order/history flow, **not new payment routes or a provider form**.
Its log SHA-256 is `05b87e95cdf3e7dbc46f6cbb91205019da86286f5a07308b85dc2d62ad8026dd`;
run-specific evidence is `output/playwright/buyer-order-3317134539/`.
Root visually inspected desktop/mobile order captures; no UI was changed.
Fresh captures remain in that run directory; runner-overwritten tracked design
baseline copies were restored only after identical fresh copies were verified.

SHA-256:

- Node: `8365fbc0ff0485f711cdeb23e11947ca1914eabc840e2e255115eacdef105455`
- Typecheck: `de46600b5fb7fd9cd4206a8374bb2562031f87e6f5f259353f2740f3def3cd6f`
- Build: `b5296d38a9530cad74155407d8fed99ae4d93ae3c7f2fd90a2c9c831904a0a1a`
- Go/PG/race/vet: `713706e28ee6f25f08c2d97a20e9b7f8638656897e1f644b5eabc80556d80426`

Independent final read-only review `89b697c3-1975-4037-8516-3ffd289931f0`
examined `a78a108..f3de924` and evidence: no unresolved P0/P1/P2 in this boundary.
It did not independently rerun the full Go suite. Deadline tests use a synthetic
aborted signal; they do not establish actual browser/remote-provider timing.
Author typecheck/build environment limitations are retained, not hidden: the
test worktree lacked tsc, and the code worktree's external node_modules symlink
was rejected by Turbopack. Root's installed workspace passed both checks.

## Mandatory next work — not waived

- Extend the existing selected `OrderDetails` once for new and historical owned
  orders, preserving approved B product geometry and all three languages. No
  Pay action in the sticky product footer or repeated history rows.
- Explicit original-order prepare → one-shot handoff → native PSP form. Keep
  handed-out form only in memory, never a replay journal, URL or logs. Use the
  existing purchase Web Lock; payment markers must not replace purchase journal
  or current-order locator. Lost handoff response/remount/reload means GET only.
- A fixed global return URL cannot infer a tenant's host-only cookie/origin.
  Use a neutral return page, no callback-derived redirect or payment-success
  claim; preserve original store context through browser-tested navigation.
  Independent follow-up research: `d93e807f-4a78-4fc6-9108-a1ee7536e01b`;
  popup alternative remains unverified (`c67cecdc-fe6a-45a5-b3c5-a30deedda6d9`).
- Actual Next→Go→PG→mock PSP payment browser gates: desktop/mobile/three locales,
  popup blocking, double click, closed page, lost response, session change and
  return/reload. No payment-browser acceptance is claimed here.
- Notification/query/capture production assembly, trusted qualification, real
  authorized sandbox, deployment/rollback and full SaaS release gates remain.
  No merchant enablement or customer production setting was changed.

Owned PG fixture from the full regression was removed; worktrees and evidence
remain for traceability. The browser runner also removed its owned PG/server
fixtures. Six files were submitted to incremental graph indexing; three TS files
produced58 entities, rejected0. Five public transport/DTO symbols link to the
independent final review memory. Packet structure and diff checks passed; neither
is a runtime gate. No real provider request, charge or production write.
