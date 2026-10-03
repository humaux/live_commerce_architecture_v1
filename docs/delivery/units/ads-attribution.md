# Unit ads-attribution — what each ad actually brought in (R5)

Owner, 2026-10-03: "广告需要做数据归因哦，好好设计一下吧".
Principle (owner, same day): **we do not rebuild what Meta already does.** Meta models and optimises attribution itself, from the signals we send. Our job is to:
1. send Meta the best signals (CAPI);
2. link orders to ads only where the link is a fact;
3. show the merchant both views side by side, never blended.

## What exists (verified 2026-10-03 at r3/integration)
- **CAPI Purchase** (`internal/attribution`, 0080). Fields: `event_id`, `event_name`, `action_source`, `event_source_url`, `client_user_agent`, `external_id`, `ph`, `value`, `currency`.
  - Sent once per consented CAPTURED payment attempt.
  - Consent comes from `customers.consent_allows`; nothing is resent after an UNKNOWN result.
- **Daily ad spend and status per draft**: `ads.insights_daily`, filled by the insights sweeper.
- **Boosted post**: `ads.campaign_drafts.source_ref` holds the boosted post's `object_story_id`.
- **Live session → Meta post**: `live.claim_sources`.
- **Claims → orders**: claim bundle → order (0103).

## Gaps
1. The storefront captures nothing from the ad click: no `fbclid`, `_fbc`, `_fbp` or ad parameter.
2. CAPI therefore has no `fbc`/`fbp`, so Meta's match quality is low and Meta under-credits our merchants' ads.
3. An order does not record which ad, if any, brought it.
4. There is no merchant report of spend versus real orders and revenue.

## Decisions
**D1 — Two views, never mixed.** The ad report shows two separately labelled numbers and never adds them together.
- **「訂單實績」(ours):** real orders net of refunds, linked by rules D3/D4 only.
- **「Meta 回報」(Meta's):** purchases and purchase value that Meta attributes, read from the Insights API (`actions`/`action_values` for purchase).

The two numbers differ by design (cross-device, modelled conversions, view-through). The UI explains this in one sentence.

**D2 — Click capture is first-party and has no pixel.**
- Every ad link we create carries `lc_ad=<draft id>`. We create the ads, so we control `link_url` for PRODUCT_TRAFFIC.
- Meta appends `fbclid` to the link.
- On landing, the storefront BFF sets two first-party, httpOnly cookies scoped to the store host:
  - `lc_fbc = fb.1.<ms>.<fbclid>` (Meta's documented `fbc` format);
  - `lc_fbp = fb.1.<ms>.<random>`, created once per browser.
- It also records a touch `{draft_id, clicked_at}`.
- No Meta Pixel and no third-party script: CAPI already carries the purchase, and a pixel adds third-party JS, ad-blocker loss and a consent surface for no extra fact.

**D3 — Click path: last click wins, 7-day window** (Meta's default click window).
- At checkout Begin, the latest touch within 7 days is frozen into the order snapshot as `attribution = {path:"ad_click", draft_id, clicked_at}`.
- Later clicks overwrite earlier ones. Expired touches are ignored.
- No multi-touch model.

**D4 — Comment path (live selling): post level, labelled as such.**
- A claim made from a comment on a post that a draft boosted, while that draft was live, freezes `attribution = {path:"boosted_post", draft_id, post_id}` into the order.
- Live means between the first ACTIVE and the last PAUSE/END, per `ads.insights_daily` and the operation history.
- The link runs via `live.claim_sources.source_object_id = draft.source_ref`.
- Meta does not tell us whether a commenter saw the ad or found the post organically. The report therefore labels this column 「受推廣貼文帶來」 (orders from the promoted post), never 「廣告帶來」 (orders from the ad), and never claims more precision than that.

**D5 — CAPI gets better signals; no new consent.** For consented buyers only (the existing gate):
- send `fbc` and `fbp` from the order's frozen cookies;
- send `client_ip_address` from the checkout request;
- send `em` (SHA-256, normalised) when the buyer gave an e-mail.

`event_id` stays as today (deduplication). Non-consented orders still count in our own 「訂單實績」, because nothing leaves our system for them.

**D6 — Refunds reduce our revenue.**
- The report uses net revenue: captured minus refunded, from the existing refund facts.
- Cash on delivery counts once it is collected.
- Orders that are still uncollected are shown separately as 「待收款」.

**D7 — Report.** In the ads area of the admin, per draft and per day range (Taipei day):
- spend;
- our orders and net revenue by path;
- our ROAS = net revenue / spend;
- Meta-reported purchases and value.

Insights for the last 3 days are marked 「Meta 數據可能仍會更新」 (Meta restates late data). Everything is read-only, with no new writes to Meta.

**D8 — Privacy.**
- Cookies hold no PII.
- The order snapshot stores only the draft id, timestamps, and `fbc`/`fbp`, which are pseudonymous Meta identifiers. They are deleted together with the order snapshot under the existing erasure path.
- The privacy policy and cookie notice gain one line about the ad-measurement cookies.

**D9 — Audience and session post-mortem (owner, 2026-10-03).**
Merchants buy FB ads mainly to bring viewers into their FB live selling, so the post-mortem needs "who watched, who engaged, who bought". Each source is used for what it truly knows. Meta's aggregated demographics and our order facts sit side by side and are never joined at person level.

- **Ad audience (Meta Insights `breakdowns`, read-only).** Per draft and day, with the same metrics within each breakdown (Meta-attributed):
  - **Metrics:** spend, reach, impressions, clicks, post engagements / comments, purchases.
  - **Breakdowns:**
    - `age` (18-24, 25-34, …) × `gender`;
    - `region` (Taiwan counties and cities as Meta names them);
    - `publisher_platform` / `platform_position` (FB feed, IG, Reels, …);
    - `device_platform`;
    - `hourly_stats_aggregated_by_advertiser_time_zone`, so ad spend can be laid over the live-session timeline.
  - Stored as daily snapshots by the insights sweeper (new `ads.insights_breakdowns`: draft, day, dimension, bucket, metrics). Meta's thresholds and restatement apply.
- **Live audience (Meta Page / live-video insights, read-only).** For a live session bound through `live.claim_sources`:
  - viewers;
  - peak concurrent viewers;
  - total view time;
  - viewers / view time by `age_bucket_and_gender` and by region, where Meta returns them for that video.
  - Needs `read_insights` and `pages_read_engagement`. Both join the App Review permission list (contract O3).
- **Verified 2026-10-03 with read-only probes** (`output/meta-ads-validate/insights-breakdown-probe-20261003.txt`).
  - **Ads Insights:** these breakdowns are all accepted with `spend,reach,impressions,clicks,actions,action_values` at campaign level: `age,gender`, `region`, `country`, `publisher_platform,platform_position`, `device_platform`, `hourly_stats_aggregated_by_advertiser_time_zone`.
  - **Live videos:** `/{page}/live_videos` lists them. On `/{video}/video_insights` these metrics are valid and need `read_insights`:
    - `total_video_views` (34 for the latest 大夢甄選女包 live);
    - `total_video_view_time_by_age_bucket_and_gender`;
    - `total_video_view_time_by_region_id`;
    - `total_video_views_by_distribution_type`.
  - `live_video_views_by_age_bucket_and_gender` does not exist.
  - The demographic metrics came back EMPTY for a 34-view live, because of Meta's privacy thresholds. The UI must show 「觀眾數不足，Meta 未提供輪廓」 rather than a blank panel or zeros. Live demographics are view TIME by bucket, not unique viewers, and the label must say so.
- **Buyers (ours, real orders).** Per session and per draft:
  - orders and net revenue by the buyer's ship-to county/city (Taiwan 縣市, from the frozen order destination; store pickup uses the store's county);
  - new versus returning buyers;
  - average order value;
  - top products;
  - the funnel comments → claims → checkout links → paid/collected orders;
  - orders per minute along the live timeline.
- **Session post-mortem page (「直播復盤」).** One page per live session, on one timeline:
  - ad spend by hour;
  - viewers;
  - comments, claims and orders;
  - revenue and ROAS.
  - Two audience panels, side by side and labelled:
    - 「觀眾輪廓（Meta 統計）」: age, gender, region;
    - 「買家分佈（訂單實績）」: county, new/returning.
  - A draft report page shows the same data per ad.
- **Privacy.**
  - We do not collect buyers' age or gender (no birthday field is added).
  - Meta demographics stay aggregated as Meta returns them and are never attached to an order or a buyer.
  - Our buyer panels are aggregates of the merchant's own orders.

## Not doing (rejected, with reasons)
- **Meta Pixel / browser events.** CAPI covers purchases. A pixel would add third-party JS, consent surface and ad-blocker gaps, for no extra fact.
- **Our own modelled or multi-touch attribution, or view-through.** Meta already does this; duplicating it gives a third, conflicting number.
- **Our own incrementality/lift tests.** Meta Conversion Lift exists. Revisit once spend justifies it.
- **Splitting paid from organic comments on a boosted post.** Not observable through the API. Claiming it would be fake precision (same lesson as the Reddit-bio attribution).
- **Collecting buyer age or gender ourselves, or joining Meta demographics to individual buyers.** It is privacy-invasive, Meta forbids re-identification, and the aggregated Meta panels already answer the post-mortem question.
- **UTM tracking for non-Meta channels.** Not needed now (YAGNI). The touch table takes another `path` value when a second channel arrives.

## Acceptance gates (red first, then green; MOCK / SANDBOX labelled)
| ID | Gate |
|---|---|
| AT1 | PG/BFF: landing with `lc_ad` + `fbclid` → cart → Begin. The order snapshot has `ad_click` and the right draft. Expiry after 7 days, last-click overwrite, cookie scoped to one store host (no cross-store leak), malformed `lc_ad` ignored. |
| AT2 | CAPI payload: consented → `fbc`/`fbp`/`client_ip_address` (and `em` when given) present and correctly formatted. Not consented → nothing sent, and the order still counts in our report. `event_id` unchanged. |
| AT3 | Comment path: claim on a boosted post during the boost window → `boosted_post`. Same post outside the window → none. A non-boosted post → none. A claim from another store's post → none. |
| AT4 | Report sums equal PG fixture sums exactly: spend, orders, net revenue after a partial refund, ROAS, uncollected COD in 「待收款」. The Meta column comes from MOCK Insights and is never summed into ours. |
| AT5 | Browser (real clicks): ad URL → storefront → checkout → admin ad report shows the order under that draft. Three locales, 390 and 1586. |
| AT6 | SANDBOX: S4 (CAPI with `test_event_code`) shows a Purchase with `fbc` in Events Manager. Needs the owner's dataset id and test event code. |
| AT7 | Full G07 (migration). Click sweep of the new report page. |
| AT8 | Breakdowns: the MOCK Insights fixture with age×gender, region, placement and hourly rows is stored per draft/day and shown exactly. Re-reading the same day replaces it and never duplicates it. Cross-store isolation holds. |
| AT9 | 直播復盤 (PG + browser, real clicks): a session with a bound post, two boosted drafts, claims, a partial refund and COD orders produces exact funnel counts, county distribution, new/returning split, an hourly spend overlay, and Meta live-audience panels from MOCK. Three locales, 390 and 1586. SANDBOX/LIVE read of a real live video's insights once `read_insights` is approved. |

## Known limits → upgrade signals
- **Cross-device journeys and in-app browser cookie loss** make our click path under-count. Meta's column covers them. Signal: a large, persistent gap between our number and Meta's for the same draft.
- **The comment path is post-level.** Signal: Meta exposes comment-to-ad attribution, or spend justifies a lift test.
- **Insights restate for about 72h.** Provisional marking as in D7.

## Split (PROCESS §2)
- **Migration 0112**, reserved: order snapshot attribution field (inside the existing JSON snapshot if it fits the contract; otherwise a narrow column), touch capture and report definer reads.
- **Backend and UI together: Codex** (owner allows full-stack for Codex; DeepSeek balance is near its reserve):
  - capture in the storefront BFF;
  - freezing at Begin;
  - CAPI fields;
  - claim-path link;
  - report API;
  - admin report page;
  - privacy copy;
  - the D9 breakdown sweeper and the live insights read;
  - the 直播復盤 page.
- **Independent review:** a non-author agent, with the security focus on cookies, PII and cross-store isolation.
