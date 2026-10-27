# OC-11 delivery (ui_worker, base r3/integration 33a03bbb8)

Figma (read-only): SF-04 940:6524/6525 (the "publish status" page; the pending state itself is built from the owner text OC-Q10), SF-06 1253:17180 (desktop) / 1253:17204 (mobile). Screenshots: output/oc-11/figma/.
Deviation from Figma: node 1253:17204 says "你已填寫的內容還在"; OC-Q18 says promise no input persistence, so the copy omits it.

## Backend recheck finding (read from code)
- Go `POST design/publish` (internal/httpapi/design.go -> design.Publish, internal/design/store.go) does NOT use Idempotency-Key and has no receipt table / ops.command_results row.
  Safety is compare-and-set on `expected_draft_version`; a replay after commit is 409 `conflict` (newest row is a `publish` with source_version == version).
- A read path exists: `GET design/versions` rows carry `kind` + `source_version`. A committed publish of draft vN is exactly a `publish` row with source_version=N.
- Implemented: uncertain (network error / 5xx / unreadable 2xx) -> pending state, key + version kept, publish button disabled, recheck = GET versions only (`publishLanded`). Found -> success note + live version; not found / read failed -> stays pending, never success. No new publish, no new key.
- Limit: "not found" cannot distinguish "never arrived" from "still in flight"; the UI says so and only offers recheck. No resend path exists (owner may later want a same-key replay; safe by CAS but not built).

## Storefront
error.tsx: single read-only reload (router.refresh + reset, GET only), offline banner via useSyncExternalStore on navigator.onLine + online/offline, title focused on mount, role=alert/status, buttons 48px, no persistence promise.
Copy keys added (admin: publishPending*, publishRecheck*, publishNotYet; storefront: reloadPage, offlineBanner) in en/zh-TW/zh-CN.

## Files
apps/admin/lib/design-client.ts, apps/admin/components/Design.tsx, apps/admin/lib/design-copy.ts (extra), apps/storefront/app/[locale]/error.tsx, apps/storefront/lib/shop-copy.ts (extra), tests/admin/oc/design-publish-pending.spec.ts (extra), tests/storefront/offline-error.spec.ts, output/oc-11/**.

## Evidence / NOT_RUN
- RED: NOT_RUN (see red.log). GREEN: NOT_RUN for the two specs. Only the generic PR CI (build/typecheck gates) applies; run ids in the PR.
- No local tests, no typecheck (no node_modules in the worktree). check_packet.py ran.
- Integrator requests: (1) register tests/admin/oc/design-publish-pending.spec.ts in the store-design browser test (same env as design.spec.ts); (2) a buyer browser mode that serves LC_OC11_ORIGIN + LC_OC11_ERROR_PATH (shop-fake-api `down` state) for tests/storefront/offline-error.spec.ts; then run RED/GREEN.
- Risks: unexecuted specs may need locator fixes; error.tsx unverified under Next 16 build until CI; E1 at best.
