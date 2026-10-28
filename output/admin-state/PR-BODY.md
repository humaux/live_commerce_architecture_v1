CA-ADMIN-STATE shares localized admin empty/loading/error/forbidden displays. FIX-RED repairs the locale loading boundary introduced by this PR: it could stream HTTP 200 before a page rejected an invalid store, query or disabled gallery with notFound().

The approved fix3 relocates the unchanged loading presentation to AdminLoading and places explicit Suspense around customers/orders content only after all server guards finish. AD-09 follows the relocated module and adds a regression assertion forbidding locale-wide loading. Existing rendered loading, accessibility, HTTP status and gate-adapter assertions retain their strength. No ACL or route-guard duplication.

Validation:
- Current pre-fix head a1673cff8fa909866d01051d759a74bf3cf941f5, Gates run 40705999120, tested synthetic merge 0f71a089741768940fcca9da8f60a1fca614cb43. Read all five failed jobs directly.
- RED job 122933199700: disabled gallery returned 200, expected 404. RED job 122933201564: unknown-store shell returned 200, expected 404; also parcel-merge browser failed.
- Other failed jobs: 122933201582 StoreDesign, 122933200181 CardPayments, 122933201753 StaffTeam. Their job wrappers were read; detailed archive reads timed out. These additional causes remain unconfirmed and are not claimed fixed. See output/admin-state/{red.log,DELIVERY.md}.
- Earlier tests-only RED: b02a185628b9b1c6bc07eb538df635c75f8a761f, Foundation run 40645302665. Earlier AD-09 RED: run 40690045203, job 122822227182. These are historical receipts, not current acceptance.
- Static node --check and git diff --check: exit 0. Admin tsc: unavailable (exit 254, compiler not installed). No local tests/servers/builds/PG/browser.
- Merged origin/r3/integration before editing. Regenerated browser-ownership.json with the repository generator (53 modes). No Go dependencies changed; dependency-map regeneration not needed.
- New-head Gates/Foundation CI PENDING; run IDs will be in the handoff comment. Self-review E0; independent review and product acceptance NOT_RUN.

Scope outside the original card, approved by the current integrator instruction:
- apps/admin/components/AdminLoading.tsx (relocated loading presentation)
- apps/admin/app/[locale]/customers/page.tsx and apps/admin/app/[locale]/orders/page.tsx (post-validation Suspense)
- output/ci-select-backend-browser/browser-ownership.json (generated file)

Prior PR scope extensions retained for integrator review: scripts/dev/test-node.sh; tests/admin/customer-tags-scope.test.mjs; tests/admin/import-integration.test.ts; tests/admin/orders-ui.spec.ts. This session makes no further edits to these paths. The integration merge includes upstream changes; they are not additional card edits.

Humaux tools unavailable: search/store/canvas/coord NOT_RUN. No merge, auto-merge or deployment.

🤖 Generated with Codex (dispatched by Claude Code integrator).
