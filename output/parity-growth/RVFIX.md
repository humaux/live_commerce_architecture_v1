# RVFIX-366 (Opus review of PR #366 at 97c0f508a)

| finding | change (file:line) | RED run | GREEN run |
|---|---|---|---|
| P1-1 /promotions claims ads off without reading ads state | RED: tests/storefront/promotions-gate.mjs `growthEnabledAndDeepLink` (called from `scenario`, index 0); off-store assertions in `growthMarketingParity` kept. Impl: apps/admin/components/Promotions.tsx:45-60 (ads settings+drafts read via useGuardedRead), :72-95 (off frame only when `max_active_budget_minor===0`; drafts table; loading/error otherwise) | a12f5391b run 41237371015: bundle:05 `--browser-promotions` FAIL | PENDING next-head CI |
| P2-1 codes view lost on reload; Refresh re-reads promotions | RED: same function, `?view=codes` + reload. Impl: Promotions.tsx:39-41 (`go()` pushes ?view=codes), :94 Refresh -> `ads.reload`; app/[locale]/promotions/page.tsx (query grammar `store`,`view=codes`, `codes` prop) | same run, same member | PENDING |

Rule used for "server says off": `settings.max_active_budget_minor === 0` (same as Ads.tsx:232 `allowanceOff`); no new endpoint.
Enabled fixture is route-fulfilled `ads/settings` + `ads/drafts` reads (preparation only; no write path faked).
Left alone: P2-2..P2-4 (go to an issue).
