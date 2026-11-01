# RVFIX-366 relay 2 lane report

| failing job / finding | class | change |
|---|---|---|
| run 41240042189 head 39a5c1c80: gate bundle:05 member `--browser-promotions` FAIL (browser_promotions_test.go:147); receipts + required are consequences | b (own regression) | apps/admin/components/Promotions.tsx:56-62 ads `not-found` (ads routes unmounted, the server's "off") maps to `{settings:null}`; :66 `off` is true for null settings or budget 0. The off-store frame 1194:7740/7796 renders again; the enabled/fixture path is unchanged |

Not run locally (owner rule). The node stderr is only in the uploaded evidence, not the CI log. The cause is inferred from internal/httpapi/ads.go registerAdsRoutes (nil service = routes unmounted) and the failing member. WAIT_RUN
STATUS: DONE
