# PR7 admin browser timezone gate

task_id: tz-browser-admin
base_commit: 5176e490f79a820fb28d36f6ac5095141eaa28c5
branch: unit/tz-browser-admin
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/tz-browser-admin
role: test_worker; model: gpt-6-sol; reasoning: high
write_paths: tests/admin/ads.spec.ts; tests/admin/design.spec.ts; tests/storefront/manual-order-link-gate.mjs; output/tz-browser-admin/

Implementation: Existing real-browser ads draft Save captures the 201 response, checks supplied Taipei wall times against the persisted instants, asserts detail/edit fields, then reloads and reopens the saved draft by real click. Existing design publish captures GET /design/versions, checks the published timestamp in the version row, then reloads and clicks the versions tab. Existing manual order captures POST receipt expires_at, checks the admin result, follows the existing buyer link, checks the persisted bank deadline before/after reload, and fills paid_at in the browser's LA timezone. Default Playwright contexts and explicit buyer contexts use America/Los_Angeles, checked through browser Intl. A UTC read-only context reopens the same saved ads draft and published design version with the same signed storage state. No fixture/order/schema/API was added.

Validation at base plus these uncommitted files:
- `git diff --check` exit 0.
- `node --experimental-strip-types --check tests/admin/ads.spec.ts` exit 0.
- `node --experimental-strip-types --check tests/admin/design.spec.ts` exit 0.
- `node --check tests/storefront/manual-order-link-gate.mjs` exit 0.
- `pnpm --filter admin exec tsc --noEmit` exit 254: this worktree has no node_modules; `tsc` not found.
- `bash scripts/dev/test-local.sh --browser-meta-ads` NOT_RUN (root CI/integration owner).
- `bash scripts/dev/test-local.sh --browser-design` NOT_RUN (root CI/integration owner).
- `bash scripts/dev/test-local.sh --browser-manual-order` NOT_RUN (root CI/integration owner).

File SHA256: ads.spec.ts 3827a1f29051b8d88d286bba49ad91689aa9fa06ee1a91dce4329b856fc5bc48; design.spec.ts f5a5602415c042374b498925af65ffe1647118da4293997819035d23928cc800; manual-order-link-gate.mjs fc37f9783b269ffd5f40de615617a83c7fedeb2c5671004d5f88f1fd5991bb2e.

Evidence level: E1 static syntax only. Real Go/PG/Next/Playwright gate, red mutation run, and independent review are NOT_RUN. Admin ManualOrder's placed receipt is local component state and disappears on refresh by design; the persistence assertion uses the already created buyer order's bank deadline, which is sourced from order expiry in migrations/0088_checkout_offline.sql. No claim of durable admin receipt UI.
