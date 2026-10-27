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
Integrator-granted: tests/foundation/browser_store_design_test.go (spec added to the playwright invocation), tests/storefront/shop-real-gate.mjs (calls the check; not counted in cases=9), tests/foundation/browser_storefront_test.go (asserts the check reported; harness-only `api-down`/`api-up` control that makes the buyer API answer 503 so error.tsx renders; the app has no test route).
Needed beyond the grant (flag for the integrator): playwright.config.ts, one entry "oc/design-publish-pending.spec.ts" in the store-design testMatch (without it the spec is never selected).
Dropped: the earlier shop-gate.mjs registration (runs only under LC_SHOP_MOCK=1, never in CI).

## Evidence
- RED (deliberate mutation: recheck republishes with a new key; offline banner disabled): see output/oc-11/red.log. Admin run 40462789529 job 122056953409 failed at OC11-A2; storefront run 40502862891 job 122191823469 failed at OC11-B2. Setup errors before that are listed there and not counted.
- GREEN: mutations reverted; run/job ids are in the PR comment and the final line of red.log (filled after the run).
- Local: none (no node_modules). Level: E3 once green, MOCK/real-stack Next + Go harness, no device.
- Not covered: Safari/WebKit (offline events differ), real network loss mid-payment, the "still in flight vs never arrived" ambiguity of recheck.
- Risks: the admin spec uses the BFF-probe store and leaves one extra published version there; recheck cannot resend (by design).
