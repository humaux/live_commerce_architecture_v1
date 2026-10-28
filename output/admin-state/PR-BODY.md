CA-ADMIN-STATE / FIX-RED #268 shares admin error, empty, loading and permission displays while preserving page validation and merchant-facing read results.

The ancestor loading boundary was moved to post-validation customer/order content so it cannot commit HTTP200 before route guards return404. Relay2 fixes the remaining omission: shared AdminForbidden keeps each caller's existing three-language denial alongside the approved neutral guidance. Both Customers and MerchantOrders pass their localized denial. ACL, privacy purge, filters and retry behavior stay unchanged.

Evidence:
- Current-before-repair head8a671a9a: Gates40774523865/job123156443677 bundle09 failed only merchant-orders-ui. Artifact12860707314 proves parcel-merge.spec.ts:462 expects the missing order denial on403; red.log and relay2-playwright-parcel.log retain the evidence.
- Foundation40774523875/job123120190295 passed for that old head only. New-head acceptance PENDING; exact Gates/Foundation WAIT_RUN IDs will follow in a comment.
- Added localized caller-description SSR coverage without removing any assertions. Browser privacy tests, gate-adapter assertions and timeouts remain unchanged.
- node --check and git diff --check exit0; deterministic browser ownership regenerated after integration merge. No local tests/servers. tsc NOT_RUN (compiler absent); independent review, SANDBOX, LIVE and production NOT_RUN. Self-review E0.
- Humaux fix d7b6ff19-c178-47b6-baf2-e515d738cfcd; child canvas updated. Rollback via revert of repair commit; no PR merge/auto-merge or deployment.

Outside original card write_paths:
- apps/admin/components/AdminLoading.tsx; apps/admin/app/[locale]/customers/page.tsx; apps/admin/app/[locale]/orders/page.tsx — approved fix3 loading relocation, already present before relay2.
- output/ci-select-backend-browser/browser-ownership.json — generated source hashes, pre-approved.
- output/admin-state/** — delivery and CI failure evidence.
- Prior scope extensions retained for integrator review: scripts/dev/test-node.sh; tests/admin/customer-tags-scope.test.mjs; tests/admin/import-integration.test.ts; tests/admin/orders-ui.spec.ts. Relay2 does not further edit these paths.

Integration merge content is upstream; relay2 authors no migration, contract, OpenAPI, dependency or other-app changes. Full current details and UI-process limitations are in output/admin-state/DELIVERY.md.

🤖 Generated with Codex (dispatched by Claude Code integrator).
