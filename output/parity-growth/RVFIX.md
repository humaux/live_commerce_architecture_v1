# RVFIX-366 (Opus review of PR #366 at 97c0f508a)

| finding | change (file:line) | RED run | GREEN run |
|---|---|---|---|
| P1-1 /promotions claims ads off without reading ads state | RED: tests/storefront/promotions-gate.mjs `growthEnabledAndDeepLink` (called from `scenario`, index 0); off-store assertions in `growthMarketingParity` kept. Impl pending at apps/admin/components/Promotions.tsx:52-61 | PENDING CI on this push | PENDING implementation push |
| P2-1 codes view lost on reload; Refresh re-reads promotions | RED: same function, `?view=codes` + reload. Impl pending at Promotions.tsx:42,59 | PENDING CI | PENDING |

Rule used for "server says off": `settings.max_active_budget_minor === 0` (same as Ads.tsx:232 `allowanceOff`); no new endpoint.
Enabled fixture is route-fulfilled `ads/settings` + `ads/drafts` reads (preparation only; no write path faked).
Left alone: P2-2..P2-4 (go to an issue).
