# Codex-3 TZ storefront delivery
- task_id: codex-tz-audit-v2-sub-storefront; branch: unit/tz-audit-storefront; base_commit: 35abffcadaa1240888d291391712e1411cb820a3; head: same (uncommitted handoff); role: ui_worker; actual model/reasoning: UNKNOWN (not exposed to this worker).
- Change paths: apps/storefront/components/{OrderHistory,ClaimLink,BankTransfer,ShopChrome}.tsx; apps/storefront/tests/{timezone,claim}.test.mjs.
- Summary: order, claim, transfer deadline/proof instants use frozen shared displayTime; footer year uses instantToTaipei. Buyer-local transfer input remains local. No contract or backend status changed.
- RED: TZ=UTC node --test apps/storefront/tests/timezone.test.mjs → exit 1 (4 fail); TZ=America/Los_Angeles same → exit 1 (4 fail). Evidence: red-utc.log, red-la.log.
- GREEN: same two commands → exit 0 (4 pass each). Evidence: green-utc.log, green-la.log. Real TSX producers run through SWC; shared format module is imported from source, not mocked.
- Other checks: pnpm install --offline --frozen-lockfile → 0; node --test apps/storefront/tests/claim.test.mjs → 0 (11 pass, claim.log); pnpm --filter @live-commerce/storefront exec tsc --noEmit → 0 (typecheck.log); bash scripts/dev/check-gates.sh → 0 (check-gates.log); git diff --check → 0.
- Evidence class: UNIT/MOCK React hook fixture; no browser, REAL_PG, SANDBOX or LIVE run. Browser click ledger: NOT_RUN; this change has no new control and preamble forbids local RAM-heavy browser gates.
- Risk: shared formatter output shape intentionally changes from locale dateStyle to the frozen store format; root must independently review/replay and assemble commit. Unresolved P0/P1: UNKNOWN pending independent review.
