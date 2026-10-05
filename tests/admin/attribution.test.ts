// MOCK parser / BFF grammar / route tests. Browser click coverage is attribution.spec.ts; these do not close AT4/AT5/AT9.
import assert from "node:assert/strict";
import { test } from "node:test";
import { parseAttributionReport } from "../../apps/admin/lib/attribution-model.ts";
import { attributionCopy } from "../../apps/admin/lib/attribution-copy.ts";
import {
  adsRoutes,
  adsBodyless,
  adsKeyless,
  validAdsQuery,
} from "../../apps/admin/lib/ads-request.ts";
import { canOpen, matchRoute } from "../../apps/admin/src/routes.ts";
import { attributionFixture } from "./attribution.fixture.ts";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { money, displayTime } from "../../packages/format/src/index.ts";
import { formatROAS } from "../../apps/admin/lib/attribution-format.ts";
import { pageTitle } from "../../apps/admin/src/page-title.ts";

// MOCK SSR: execute the actual panel JSX; only navigation and the separate read control are stubbed.
// This supplements, never replaces, the real PG click gate in attribution.spec.ts.
const requireApp = createRequire(
  new URL("../../apps/admin/package.json", import.meta.url),
);
const React = requireApp("react"),
  { renderToStaticMarkup } = requireApp("react-dom/server");
// Execute the frozen presentation primitives too: only CSS class names and pathname are supplied by this SSR harness.
const presentation: Record<string, any> = {};
runInNewContext(ts.transpileModule(readFileSync(new URL("../../packages/ui/src/Presentation.tsx", import.meta.url), "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText, {
  exports: presentation,
  require: (name: string) => name.endsWith(".css") ? { __esModule: true, default: new Proxy({}, { get: (_, key) => String(key) }) } : requireApp(name),
});
const header: Record<string, any> = {};
runInNewContext(ts.transpileModule(readFileSync(new URL("../../apps/admin/components/AdminPageHeader.tsx", import.meta.url), "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText, {
  exports: header,
  require: (name: string) => name === "@live-commerce/ui" ? presentation : name === "@/src/page-title" ? { pageTitle } : name === "next/navigation" ? { usePathname: () => "/en/ads/attribution" } : requireApp(name),
});
const exports: Record<string, any> = {};
let reportForRender: any = null,
  stateCalls = 0;
const code = ts.transpileModule(
  readFileSync(
    new URL("../../apps/admin/components/Attribution.tsx", import.meta.url),
    "utf8",
  ) + "\nexport { DraftPanel, SessionPanel };",
  {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      jsx: ts.JsxEmit.ReactJSX,
    },
  },
).outputText;
runInNewContext(code, {
  exports,
  require: (name: string) => {
    if (name === "react")
      return {
        ...React,
        useState: (initial: any) => {
          stateCalls++;
          return React.useState(
            stateCalls === 1 && reportForRender
              ? {
                  key: "test-store:2026-10-01:2026-10-03",
                  report: reportForRender,
                }
              : initial,
          );
        },
      };
    if (name === "react/jsx-runtime") return requireApp(name);
    if (name === "next/navigation") return { useRouter: () => ({}) };
    if (name === "next/link")
      return {
        __esModule: true,
        default: ({ children, ...props }: any) =>
          React.createElement("a", props, children),
      };
    if (name === "@live-commerce/format") return { money, displayTime };
    if (name === "@live-commerce/ui") return presentation;
    if (name === "./AdminPageHeader") return header;
    if (name === "@/lib/attribution-format") return { formatROAS };
    if (name === "@/lib/attribution-copy") return { attributionCopy };
    if (name === "@/src/routes") return { matchRoute };
    if (name === "./AttributionAudienceRead")
      return {
        AttributionAudienceRead: () =>
          React.createElement("div", {
            "data-testid": "attribution-audience-read",
          }),
      };
    if (name === "./WorkspaceFrame")
      return { WorkspaceFrame: ({ children }: any) => children };
    return {};
  },
});
const renderPanel = (name: string, props: object) =>
  renderToStaticMarkup(React.createElement(exports[name], props));

for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`shared presentation preserves registry title and one empty buyer message in ${locale}`, () => {
    const session = structuredClone(attributionFixture.sessions[0]);
    session.buyers.counties = [];
    session.buyers.top_products = [];
    session.buyers.orders_per_minute = [];
    session.timeline = [];
    const html = renderPanel("SessionPanel", { c: attributionCopy[locale], locale, session, store: "test-store" });
    assert.equal(html.split(attributionCopy[locale].noBuyers).length - 1, 1);
    assert.equal(html.split(attributionCopy[locale].noTimeline).length - 1, 1);
    assert.ok(html.includes(attributionCopy[locale].newBuyers));
    assert.ok(html.includes(attributionCopy[locale].average));
    assert.match(html, /role="region"[^>]*aria-label=/);
    reportForRender = structuredClone(attributionFixture);
    stateCalls = 0;
    const page = renderPanel("Attribution", { locale, store: { id: "test-store", name: "Synthetic", currency: "TWD" }, from: "2026-10-01", to: "2026-10-03", draftID: "", sessionID: "", initialError: null });
    assert.ok(page.includes(`<h1>${pageTitle(locale, "/ads/attribution")}</h1>`));
    assert.match(page, /id="attribution-from"[^>]*lang=/);
    assert.ok(page.includes('for="attribution-session"'));
    reportForRender = null;
  });

  test(`R11 large linked-draft list is complete but collapsed in ${locale}`, () => {
    const session = structuredClone(attributionFixture.sessions[0]);
    session.draft_ids = Array.from(
      { length: 100 },
      (_, i) => `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
    );
    const html = renderPanel("SessionPanel", {
      c: attributionCopy[locale],
      locale,
      session,
      store: "test-store",
    });
    const details = html.match(
      /<details[^>]*data-testid="attribution-linked-drafts"[^>]*>[\s\S]*?<\/details>/,
    )?.[0];
    assert.ok(
      details,
      "secondary identifiers must not push the report facts below a text wall",
    );
    assert.doesNotMatch(details, /<details[^>]*\sopen(?:=|\s|>)/);
    assert.ok(details.includes(`${attributionCopy[locale].draftIds} (100)`));
    assert.equal((details.match(/<li>/g) ?? []).length, 100);
    for (const id of session.draft_ids) assert.ok(details.includes(id));
  });
}

for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`R11 timeline distinguishes local unknown spend from explicit Meta zero in ${locale}`, () => {
    const r: any = structuredClone(attributionFixture),
      c = attributionCopy[locale];
    r.sessions[0].timeline = [
      { ...r.sessions[0].timeline[0], spend_minor: null },
      {
        ...r.sessions[0].timeline[0],
        at: "2026-10-03T12:01:00Z",
        spend_minor: 0,
      },
    ];
    const parsed = parseAttributionReport(r);
    assert.equal(parsed.sessions[0].timeline[0].spend_minor, null);
    assert.equal(parsed.sessions[0].timeline[1].spend_minor, 0);
    const html = renderPanel("SessionPanel", {
      c,
      locale,
      session: parsed.sessions[0],
      store: "test-store",
    });
    const table =
      html.match(
        /data-testid="attribution-timeline"[\s\S]*?<tbody>([\s\S]*?)<\/tbody>/,
      )?.[1] ?? "";
    const spendCells = [...table.matchAll(/<tr>(.*?)<\/tr>/g)].map(
      (row) => row[1].match(/<td>(.*?)<\/td>/)?.[1],
    );
    assert.deepEqual(spendCells, [c.unknown, money(locale, "TWD", 0)]);
  });
}

test("R11 I12 nullable Meta evidence survives without becoming zero", () => {
  const r: any = structuredClone(attributionFixture);
  r.truncated = true;
  r.drafts[0].spend_minor = null;
  r.drafts[0].roas = null;
  r.sessions[0].spend_minor = null;
  r.drafts[0].breakdowns_unavailable = [
    { day: "2026-10-03", dimensions: ["region", "device"] },
  ];
  for (const key of [
    "spend_minor",
    "reach",
    "impressions",
    "clicks",
    "engagements",
    "comments",
    "purchases",
    "purchase_value_minor",
  ])
    r.drafts[0].breakdowns[0][key] = null;
  const parsed: any = parseAttributionReport(r);
  assert.equal(parsed.truncated, true);
  assert.equal(parsed.drafts[0].spend_minor, null);
  assert.equal(parsed.sessions[0].spend_minor, null);
  assert.deepEqual(
    parsed.drafts[0].breakdowns_unavailable,
    r.drafts[0].breakdowns_unavailable,
  );
  for (const key of [
    "spend_minor",
    "reach",
    "impressions",
    "clicks",
    "engagements",
    "comments",
  ])
    assert.equal(parsed.drafts[0].breakdowns[0][key], null);
});

const r11Labels = {
  en: {
    reconnect: "Reconnect Facebook to authorize audience insights",
    sessionSpend:
      "Ad spend promoting this live post within the selected period",
    unavailable: "Breakdowns unavailable",
    cap: "Only the first 100 records are shown",
  },
  "zh-TW": {
    reconnect: "需重新連接 Facebook 以授權觀眾數據",
    sessionSpend: "所選期間內推廣此直播貼文的廣告花費",
    unavailable: "無法取得分類資料",
    cap: "僅顯示前 100 筆",
  },
  "zh-CN": {
    reconnect: "需重新连接 Facebook 以授权观众数据",
    sessionSpend: "所选期间内推广此直播帖文的广告花费",
    unavailable: "无法取得分类数据",
    cap: "仅显示前 100 条",
  },
};
for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`R11 actual panel rendering preserves unknowns and reconnect route in ${locale}`, () => {
    const r: any = structuredClone(attributionFixture),
      c = attributionCopy[locale];
    r.drafts[0].spend_minor =
      r.drafts[0].roas =
      r.sessions[0].spend_minor =
        null;
    r.drafts[0].breakdowns_unavailable = [
      { day: "2026-10-03", dimensions: ["region"] },
    ];
    for (const key of [
      "spend_minor",
      "reach",
      "impressions",
      "clicks",
      "engagements",
      "comments",
      "purchases",
      "purchase_value_minor",
    ])
      r.drafts[0].breakdowns[0][key] = null;
    r.sessions[0].live_audience.status = "not_authorized";
    const draft = renderPanel("DraftPanel", { c, locale, draft: r.drafts[0] });
    assert.match(draft, /<dt>[^<]+<\/dt><dd>—<\/dd>/);
    assert.ok(draft.includes(r11Labels[locale].unavailable));
    assert.ok(draft.includes("2026-10-03"));
    assert.equal((draft.match(/<dd>—<\/dd>/g) ?? []).length, 2);
    const unknownRow =
      draft.match(
        /<tr><td>2026-10-03<\/td>.*?<th scope="row">25-34:female<\/th>(.*?)<\/tr>/,
      )?.[1] ?? "";
    assert.equal(
      (unknownRow.match(new RegExp(`<td>${c.unknown}<\\/td>`, "g")) ?? [])
        .length,
      8,
    );
    const session = renderPanel("SessionPanel", {
      c,
      locale,
      session: r.sessions[0],
      store: "11111111-1111-4111-8111-111111111111",
    });
    assert.ok(session.includes(r11Labels[locale].sessionSpend));
    assert.ok(session.includes(r11Labels[locale].reconnect));
    assert.ok(
      session.includes(
        `href="/${locale}${matchRoute("/settings")!.path}?store=11111111-1111-4111-8111-111111111111"`,
      ),
    );
    assert.match(session, /<dd>—<\/dd>/);
    assert.ok(!session.includes("Infinity") && !session.includes("0.00×"));
    assert.equal((c as any).truncated, r11Labels[locale].cap);
    // Inject only the initial server report into the page's React state; JSX remains the real component.
    reportForRender = r;
    const pageProps = {
      locale,
      store: { id: "test-store", name: "Synthetic", currency: "TWD" },
      from: "2026-10-01",
      to: "2026-10-03",
      draftID: "",
      sessionID: "",
      initialError: null,
    };
    for (const truncated of [false, true]) {
      r.truncated = truncated;
      stateCalls = 0;
      const page = renderPanel("Attribution", pageProps);
      assert.equal(
        page.includes('data-testid="attribution-truncated"'),
        truncated,
      );
      if (truncated) assert.ok(page.includes(r11Labels[locale].cap));
    }
    reportForRender = null;
  });
}

test("D1, D6, R7: exact amounts and sources remain independent, account days stay labelled", () => {
  const r = parseAttributionReport(attributionFixture);
  assert.deepEqual(r, attributionFixture);
  assert.equal(r.drafts[0].orders[0].net_minor, 2500);
  assert.equal(r.drafts[0].meta.purchase_value_minor, 9000);
  assert.equal(r.drafts[0].meta_account_timezone, "America/Los_Angeles");
  assert.equal(r.drafts[0].breakdowns[4].hour_start, "2026-10-03T12:00:00Z");
});
test("R12: null stays unknown and unread / unauthorised / insufficient remain different states", () => {
  for (const status of [
    "not_read",
    "not_authorized",
    "insufficient",
  ] as const) {
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
for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`R12 granted but unread audience offers a read, never reconnect in ${locale}`, () => {
    const r: any = structuredClone(attributionFixture);
    r.sessions[0].live_audience = {
      status: "not_read",
      views: null,
      peak_concurrent: null,
      total_view_time_ms: null,
      age_gender: [],
      regions: [],
    };
    parseAttributionReport(r);
    const html = renderPanel("SessionPanel", {
      c: attributionCopy[locale],
      locale,
      session: r.sessions[0],
      store: "11111111-1111-4111-8111-111111111111",
    });
    assert.ok(html.includes('data-testid="attribution-not-read"'));
    assert.ok(!html.includes('data-testid="attribution-reconnect"'));
    assert.ok(!html.includes('data-testid="attribution-insufficient"'));
    assert.ok(html.includes('data-testid="attribution-audience-read"'));
  });
}
test("I12 forged / missing frozen fields never silently become zeros", () => {
  const mutations = [
    (r: any) => {
      delete r.truncated;
    },
    (r: any) => {
      r.truncated = "true";
    },
    (r: any) => {
      delete r.drafts[0].breakdowns_unavailable;
    },
    (r: any) => {
      r.drafts[0].breakdowns_unavailable = [
        { day: "2026-02-30", dimensions: ["region"] },
      ];
    },
    (r: any) => {
      r.drafts[0].breakdowns_unavailable = [
        { day: "2026-10-03", dimensions: [null] },
      ];
    },
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
  assert.doesNotMatch(
    path,
    adsKeyless,
    "audience-read still needs its idempotency key",
  );
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
