// Synthetic aggregates only. MOCK DTO fixture, not a PG/Meta acceptance claim.
import type {
  AttributionReport,
  Buyers,
  SessionAttribution,
} from "../../apps/admin/lib/attribution-model.ts";
export const draftID = "11111111-1111-4111-8111-111111111111";
export const secondDraftID = "11111111-1111-4111-8111-111111111112";
export const sessionID = "22222222-2222-4222-8222-222222222222";
const buyers: Buyers = {
  counties: [{ name: "臺北市", orders: 2, net_minor: 2500 }],
  new_buyers: 1,
  returning_buyers: 1,
  average_order_minor: 1250,
  top_products: [
    {
      product_id: "33333333-3333-4333-8333-333333333333",
      name: "Synthetic bag",
      quantity: 2,
    },
  ],
  orders_per_minute: [
    { at: "2026-10-03T12:01:00Z", orders: 2, net_minor: 2500 },
  ],
};
const session: SessionAttribution = {
  session_id: sessionID,
  title: "Synthetic live review",
  starts_at: "2026-10-03T12:00:00Z",
  ends_at: "2026-10-03T13:00:00Z",
  post_ids: ["synthetic_post"],
  draft_ids: [draftID, secondDraftID],
  currency: "TWD",
  spend_minor: 1000,
  orders: 2,
  net_minor: 2500,
  pending_orders: 1,
  pending_minor: 900,
  ambiguous_orders: 1,
  funnel: { comments: 20, claims: 4, checkout_links: 3, paid_orders: 2 },
  buyers,
  timeline: [
    {
      at: "2026-10-03T12:00:00Z",
      spend_minor: 1000,
      orders: 2,
      net_minor: 2500,
      comments: 20,
      claims: 4,
      viewers: null,
    },
  ],
  live_audience: {
    status: "available",
    views: 200,
    peak_concurrent: 30,
    total_view_time_ms: 120000,
    age_gender: [{ bucket: "25-34:female", view_time_ms: 30000 }],
    regions: [{ bucket: "Taipei", view_time_ms: 20000 }],
  },
};
export const attributionFixture: AttributionReport = {
  window: { from: "2026-10-01", to: "2026-10-03" },
  order_timezone: "Asia/Taipei",
  drafts: [
    {
      draft_id: draftID,
      source_ref: "synthetic_post",
      template: "BOOST_POST",
      currency: "TWD",
      meta_account_timezone: "America/Los_Angeles",
      spend_minor: 1000,
      orders: [
        {
          path: "boosted_post",
          orders: 2,
          net_minor: 2500,
          pending_orders: 1,
          pending_minor: 900,
        },
      ],
      meta: { purchases: 7, purchase_value_minor: 9000 },
      roas: 2.5,
      provisional: true,
      breakdowns: ["age_gender", "region", "placement", "device", "hourly"].map(
        (dimension, i) => ({
          dimension: dimension as
            "age_gender" | "region" | "placement" | "device" | "hourly",
          day: "2026-10-03",
          timezone_name: "America/Los_Angeles",
          bucket: [
            "25-34:female",
            "Taipei",
            "facebook:feed",
            "mobile",
            "05:00-05:59",
          ][i],
          hour_start: dimension === "hourly" ? "2026-10-03T12:00:00Z" : null,
          spend_minor: 1000,
          reach: 400,
          impressions: 900,
          clicks: 20,
          engagements: 30,
          comments: 4,
          purchases: 7,
          purchase_value_minor: 9000,
        }),
      ),
      buyers,
    },
  ],
  sessions: [session],
};
