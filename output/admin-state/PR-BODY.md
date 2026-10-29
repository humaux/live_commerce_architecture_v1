CA-ADMIN-STATE / FIX-RED #268 shares admin error, empty, loading and permission displays while preserving route validation, server-owned scope and existing customer/order controls.

Relay3 fixes the current Foundation failure at head 5db5ec3d72f57c44be4c9cdcbf5998f0007331f7. The new AD-10 SSR paragraph matcher also matched the preceding SVG path, capturing icon/title markup instead of guidance. Restrict the matcher to a real paragraph element; every existing assertion remains. Caller-specific denial and neutral guidance remain visible in all three locales. The approved fix3 loading relocation is already implemented in customer/order pages, so it is not replayed.

Evidence:
- Current-before-repair RED: Foundation run 40789708532 / job 123171584335. New AD-10 contextual assertion fails with Customers en neutral guidance; 9 passed / 1 failed. This is a harness failure, not proof of missing product guidance. Raw excerpt: output/admin-state/relay3-foundation.log; red.log records the head/run/job.
- All current failed checks were read directly; Gates run 40789708692 build remained pending. Older-head results are historical only.
- Static node --check exit 0; browser ownership regenerated (53 modes). Staged diff check exit 0. No local tests or servers. Author self-review E0; new-head GREEN PENDING. Exact-head WAIT_RUN IDs follow in a comment.
- Local runtime/browser/PG, SANDBOX, LIVE, production and independent review NOT_RUN. No design change or new visual acceptance claim. UI-process sections and click ledger: output/admin-state/DELIVERY.md.
- Rollback: revert the correction commit. No merge, auto-merge, force-push, status posting or deployment.

Outside original card write_paths (prior scope extensions retained for integrator review):
- apps/admin/components/AdminLoading.tsx; apps/admin/app/[locale]/customers/page.tsx; apps/admin/app/[locale]/orders/page.tsx — approved fix3 loading relocation, already present before relay3.
- output/ci-select-backend-browser/browser-ownership.json — pre-approved generated source hashes; relay3 changes only the corrected test hash.
- output/admin-state/** — delivery and raw CI evidence.
- scripts/dev/test-node.sh; tests/admin/customer-tags-scope.test.mjs; tests/admin/import-integration.test.ts; tests/admin/orders-ui.spec.ts — earlier scope extensions, untouched in relay3.

Relay3 authors no product-code, migration, contract, OpenAPI, dependency, other-app or backend changes. No tests, thresholds, fixtures or exemption lists weakened.

🤖 Generated with Codex (dispatched by Claude Code integrator).
