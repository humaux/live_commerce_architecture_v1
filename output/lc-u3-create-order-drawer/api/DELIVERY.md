# LC-U3 API unit delivery

- task_id: LC-U3-api
- base_commit: d00d04c7401da987cf778973ff2e8fca493eaa08
- branch/commit: unit/lc-u3-api-worker / 2c95c68126624d7a18d0daf3f3add354844dfe32
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u3-api-worker
- role/model/effort: UI API implementer / configured gpt-6.1-sol / high (parent spawn record)
- Contract changes: none. Consumes FROZEN live-console-v1 §5 A15/A16.
- Evidence: MOCK, E3 for the focused automated cases only, bound to current commit and SHA256SUMS. Independent review pending.

Changed paths:
- apps/admin/lib/create-order-model.ts
- apps/admin/lib/create-order-client.ts
- apps/admin/lib/merchant-tools-model.ts
- apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts
- tests/admin/create-order-model.test.ts
- tests/admin/create-order-client.test.ts
- tests/admin/create-order-bff.test.ts

Implementation:
- Closed A15/A16 request/response DTOs; exact single XOR UUID selector. No price or tenant inputs. A15 inactive delivery omitted, A16 inactive delivery null.
- Prefill only explicitly linked customer/delivery; no owner/name inference. Duplicate SKU claim lines aggregate quantities only; unit references use the existing shared formatter. More than five bundles refused without truncation.
- Opt-in configured/null buyer-link replay parsing leaves default manual parser strict. Only queued(operation_id UUID) or not_sent(reason) send outcomes; unknown reason strings become the fixed unknown fallback.
- Actual sessionBoundary/csrfCookie before and after asynchronous response parsing. Immutable key/body passthrough, client timeout 18s, upstream 16s. No automatic retry. Network/malformed/unknown replies and idempotency_conflict are uncertain.
- Real Request/BFF auth/origin/CSRF helpers with only synthetic upstream HTTP boundary. Bounded closed success and safe coded errors; 409 bundle_already_ordered details.order_id UUID/null is the only propagated detail.

Actual commands and exit codes:
- pnpm install --offline --frozen-lockfile -> 0
- node --test --experimental-strip-types tests/admin/create-order-model.test.ts -> 1 (preimplementation missing model, red.log)
- node --test --experimental-strip-types tests/admin/create-order-client.test.ts tests/admin/create-order-bff.test.ts -> 1 (preimplementation client missing/BFF absent routes, appended red.log)
- node --test --experimental-strip-types tests/admin/create-order-model.test.ts tests/admin/create-order-client.test.ts tests/admin/create-order-bff.test.ts tests/admin/merchant-tools-model.test.ts -> 0 (20 PASS, 0 FAIL, 0 SKIP; green.log)
- pnpm --filter @live-commerce/admin typecheck -> 0 (typecheck.log)
- bash scripts/dev/check-gates.sh -> 1 initially (formatter ratchet rejected local Intl; fixed by shared format.money)
- bash scripts/dev/check-gates.sh -> 0 before git-add (check-gates.log; new untracked test files were not part of registration check)
- bash scripts/dev/check-gates.sh -> 1 at committed SHA (check-gates-committed.log; new tests require parent-owned test-node.sh registration)
- git diff --check -> 0
- git commit -m 'feat(admin): add closed for-buyer order transport and prefill model' -> 0

Evidence paths (main checkout, durable):
- output/lc-u3-create-order-drawer/api/red.log
- output/lc-u3-create-order-drawer/api/green.log
- output/lc-u3-create-order-drawer/api/typecheck.log
- output/lc-u3-create-order-drawer/api/check-gates.log
- output/lc-u3-create-order-drawer/api/check-gates-committed.log
- output/lc-u3-create-order-drawer/api/SHA256SUMS

Unresolved / NOT_RUN:
- Parent must register three new Node files in scripts/dev/test-node.sh and rerun integrated check-gates/test-node.
- Full test-node, browser/PG, calibration, CI and independent review NOT_RUN in this subunit. No PG/browser process started.
- Parent owns Drawer integration, three-locale copy, A9/B1 capability handling, immutable attempt lifecycle, Humaux store/canvas. No Humaux tools exposed to this agent.
- No push/merge/deploy. No product Go/migrations/lockfile modifications. Worktree has no uncommitted tracked changes.

Parent integration addendum: these tests are registered; root-node/root-gates/root-typecheck all pass on tested source 16e8ac45852239ae02f80174697bfc84f96003e63459dd38e705124badb5e988. Runtime and calibration results are in ../DELIVERY.md.
