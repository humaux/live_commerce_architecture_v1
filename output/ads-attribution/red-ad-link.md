# AT1 locale-hop regression (RED)

Base: d282c98816a6bcab46d1eb862974608d297dd298 plus the new assertion only.
Command: `node --test --experimental-strip-types apps/storefront/tests/ad-link-route.test.mjs`
Exit: 1. Tests 3; PASS 2; FAIL 1; SKIP 0.

The assertion expected the relative locale redirect to retain `lc_ad`, `fbclid`, and the original query string. The baseline returned only `/zh-TW/products/<product-id>`, discarding the whole query. Existing UUID and no-open-redirect assertions remained unchanged.

This is a Node unit red, **not** full AT1 PG/BFF acceptance.
