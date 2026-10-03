// MOCK parser / BFF grammar / route tests. Browser click coverage is attribution.spec.ts; these do not close AT4/AT5/AT9.
import assert from "node:assert/strict";
import { test } from "node:test";
import { parseAttributionReport } from "../../apps/admin/lib/attribution-model.ts";
import { attributionCopy } from "../../apps/admin/lib/attribution-copy.ts";
import { adsRoutes, adsBodyless, adsKeyless, validAdsQuery } from "../../apps/admin/lib/ads-request.ts";
import { canOpen, matchRoute } from "../../apps/admin/src/routes.ts";
import { attributionFixture } from "./attribution.fixture.ts";

test("D1, D6, R7: exact amounts and sources remain independent, account days stay labelled", () => {
  const r = parseAttributionReport(attributionFixture);
  assert.deepEqual(r, attributionFixture);
  assert.equal(r.drafts[0].orders[0].net_minor, 2500);
  assert.equal(r.drafts[0].meta.purchase_value_minor, 9000);
  assert.equal(r.drafts[0].meta_account_timezone, "America/Los_Angeles");
  assert.equal(r.drafts[0].breakdowns[4].hour_start, "2026-10-03T12:00:00Z");
});
test("R8: null stays unknown and unauthorised / insufficient remain different states", () => {
  for (const status of ["not_authorized", "insufficient"] as const) {
    const r = structuredClone(attributionFixture);
    r.drafts[0].meta = { purchases: null, purchase_value_minor: null };
    r.drafts[0].roas = null;
    r.sessions[0].live_audience = {
      status,
      views: null,
      peak_concurrent: null,
      total_view_time_ms: null,
      age_gender: [],
      regions: [],
    };
    const parsed = parseAttributionReport(r);
    assert.equal(parsed.drafts[0].meta.purchases, null);
    assert.equal(parsed.sessions[0].live_audience.status, status);
    assert.equal(parsed.sessions[0].timeline[0].viewers, null);
  }
});
test("I12 forged / missing frozen fields never silently become zeros", () => {
  const mutations = [
    (r: any) => {
      delete r.drafts[0].meta.purchases;
    },
    (r: any) => {
      delete r.sessions[0].buyers;
    },
    (r: any) => {
      r.drafts[0].orders[0].path = "modelled";
    },
    (r: any) => {
      r.sessions[0].live_audience.status = "enabled";
    },
    (r: any) => {
      r.drafts[0].spend_minor = "1000";
    },
    (r: any) => {
      r.drafts[0].spend_minor = -1;
    },
    (r: any) => {
      r.drafts[0].orders[0].orders = 1.5;
    },
    (r: any) => {
      r.drafts[0].roas = NaN;
    },
    (r: any) => {
      r.drafts[0].breakdowns[4].hour_start = "05:00";
    },
    (r: any) => {
      r.drafts[0].breakdowns[4].hour_start = null;
    },
    (r: any) => {
      r.drafts[0].breakdowns[0].hour_start = "2026-10-03T12:00:00Z";
    },
    (r: any) => {
      r.drafts[0].meta_account_timezone = "Taipei-like";
    },
    (r: any) => {
      r.drafts[0].orders.push(r.drafts[0].orders[0]);
    },
    (r: any) => {
      r.drafts.push(r.drafts[0]);
    },
    (r: any) => {
      r.window.from = "2026-02-30";
    },
    (r: any) => {
      r.order_timezone = "UTC";
    },
  ];
  for (const mutate of mutations) {
    const r = structuredClone(attributionFixture);
    mutate(r);
    assert.throws(() => parseAttributionReport(r));
  }
});
test("only whitelisted aggregates survive: extra identifiers and HTML are never interpreted", () => {
  const r = structuredClone(attributionFixture) as any;
  r.client_ip = "synthetic sentinel";
  r.drafts[0].buyers.email = "synthetic sentinel";
  const parsed = parseAttributionReport(r) as any;
  assert.equal(parsed.client_ip, undefined);
  assert.equal(parsed.drafts[0].buyers.email, undefined);
});
test("attribution is a read-only BFF resource with exact bounded query grammar", () => {
  assert.match("ads/attribution", new RegExp(`^${adsRoutes.GET}$`));
  for (const method of ["POST", "PUT"] as const)
    assert.doesNotMatch(
      "ads/attribution",
      new RegExp(`^${adsRoutes[method]}$`),
    );
  assert.equal(
    validAdsQuery(
      "https://local.invalid?from=2026-10-01&to=2026-10-03",
      "ads/attribution",
    ),
    true,
  );
  for (const query of [
    "",
    "?",
    "?from=2026-10-01",
    "?from=2026-10-01&to=2026-10-03&tenant_id=fake",
    "?from=2026-10-01&to=2026-10-03&from=2026-10-02",
    "?from=2026-02-30&to=2026-03-01",
    "?from=2026-01-01&to=2026-04-30",
  ])
    assert.equal(
      validAdsQuery(`https://local.invalid${query}`, "ads/attribution"),
      false,
      query,
    );
});

test("audience-read is exactly the new local intention POST and never a data GET or queried write", () => {
  const path = `ads/sessions/${attributionFixture.sessions[0].session_id}/audience-read`;
  assert.match(path, new RegExp(`^${adsRoutes.POST}$`));
  assert.match(path, adsBodyless, "Go audience-read rejects any body");
  assert.doesNotMatch(path, adsKeyless, "audience-read still needs its idempotency key");
  assert.doesNotMatch(`${path}/extra`, adsBodyless);
  assert.doesNotMatch(path.replace("audience-read", "unknown"), adsBodyless);
  for (const method of ["GET", "PUT"] as const)
    assert.doesNotMatch(path, new RegExp(`^${adsRoutes[method]}$`));
  assert.equal(validAdsQuery("https://local.invalid", path), true);
  assert.equal(
    validAdsQuery("https://local.invalid?from=2026-10-01&to=2026-10-03", path),
    false,
  );
});
test("route fails closed without ads:read and is not a duplicate navigation entry; locale keys match", () => {
  const route = matchRoute("/ads/attribution")!;
  assert.equal(route.permission, "ads:read");
  assert.equal(route.nav, false);
  assert.equal(canOpen(route, null), false);
  assert.equal(
    canOpen(route, { role: "staff", permissions: ["orders:read"] }),
    false,
  );
  assert.equal(
    canOpen(route, { role: "staff", permissions: ["ads:read"] }),
    true,
  );
  for (const locale of ["zh-TW", "zh-CN"] as const)
    assert.deepEqual(
      Object.keys(attributionCopy[locale]).sort(),
      Object.keys(attributionCopy.en).sort(),
    );
});
