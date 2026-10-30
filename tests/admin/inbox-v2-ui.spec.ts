// Purpose: INBOX-V2 signed simplified UI real-click acceptance across all three locales.
// Depends on: existing signed MOCK OIDC/Next inbox runner and actor selector; bounded MOCK A8/A9/A10/A12/A13 and health shapes.
// Used by: --browser-inbox CI, registered in playwright.config.ts; existing real PG/BFF send tests stay intact.
// Invariants: no LIVE send; no DOM writes; UNKNOWN queries never resend; refresh preserves fixture-server facts only.
import { expect, test, type Page, type APIRequestContext } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { api, required, evidence, store, ids, login, createInboxLedger, writeInboxLedger } from "./inbox-browser-support";

const { ledger, record } = createInboxLedger();
test.use({ headless: true, trace: "off", video: "off", screenshot: "only-on-failure" });
test.afterEach(async ({ request }, info) => {
  if (info.status !== info.expectedStatus) ledger.push({ page: "Messages v2", control: info.title,
    action: "scenario", expected: "every named assertion passes", actual: info.status ?? "UNKNOWN", status: "FAIL" });
  await actor(request, "operator");
});
test.afterAll(async () => { await writeInboxLedger(ledger); });

const copies = [
  { locale: "zh-TW", title: "全店訊息", all: "全部", unread: "未讀", assigned: "本店分派",
    details: "顧客資料", paused: "自動回覆已停", read: "店內已讀", unavailable: "訂單與累計：未提供",
    send: "傳送", clear: "清除草稿", query: "查詢狀態", refresh: "重新整理", edit: "編輯原文",
    pending: "傳送中；尚未確認送達，勿重送。", sent: "已送出，未代表已讀。",
    failed: "傳送遭拒；原文保留，可編輯。", unknown: "送達待確認；只查詢，不重送。",
    still: "仍待確認；到 Instagram 查看，勿重送。", closed: "已超過回覆時間。",
    connection: "連接待確認；暫不能回覆。", permission: "缺少回覆權限。",
    draft: "我會先核對尺寸，再回覆你。" },
  { locale: "zh-CN", title: "全店消息", all: "全部", unread: "未读", assigned: "店铺分配",
    details: "客户资料", paused: "自动回复已停", read: "店内已读", unavailable: "订单与累计消费：未提供",
    send: "发送", clear: "清空草稿", query: "查询状态", refresh: "刷新", edit: "编辑原文",
    pending: "发送中；尚未确认送达，不要重发。", sent: "已发送，不代表已读。",
    failed: "发送被拒；原文已保留，可以编辑。", unknown: "送达情况待确认；只查询，不重发。",
    still: "尚未确认；请到 Instagram 查看，不要重发。", closed: "已超过可回复时间。",
    connection: "连接待确认；暂时无法回复。", permission: "缺少回复权限。",
    draft: "我先核实尺寸，再回复你。" },
  { locale: "en", title: "Store inbox", all: "All", unread: "Unread", assigned: "Assigned in store",
    details: "Customer details", paused: "Auto-replies paused", read: "Read by store",
    unavailable: "Orders and lifetime spend: not provided", send: "Send", clear: "Clear draft",
    query: "Check status", refresh: "Refresh", edit: "Edit original text", pending: "Sending; delivery unconfirmed. Do not resend.",
    sent: "Sent does not mean read.", failed: "Send rejected; original text saved for editing.",
    unknown: "Delivery unconfirmed; check status without resending.",
    still: "Still unconfirmed; check Instagram. Do not resend.", closed: "The reply window has ended.",
    connection: "Connection unconfirmed; replies are unavailable.", permission: "Reply permission is missing.",
    draft: "I will check the dimensions and get back to you." },
] as const;
type Copy = typeof copies[number];
type Delivery = "queued" | "sent" | "failed" | "blocked" | "unknown";
// B1 identifies a provider binding, not the Page. Sharing one ID would duplicate
// binding_id:dm_session and correctly fail the unchanged health parser closed.
const fixtureBindings = {
  facebook: "10000000-0000-4000-8000-000000000071",
  instagram: "10000000-0000-4000-8000-000000000073",
} as const;
const fixtureAgent = "10000000-0000-4000-8000-000000000072";
const fixtureOperation = "10000000-0000-4000-8000-000000000075";

// Signed frames are comparison targets, never fixtures injected into the product DOM.
async function inboxParity(page: Page, frame: string) {
  await expect(page).toHaveURL(/\/(zh-TW|zh-CN|en)\/messages/);
  // GitHub artifacts reject colons in paths; retain the signed frame ID in the attachment label.
  const path = test.info().outputPath("parity", `${frame.replaceAll(":", "-")}.png`);
  await page.screenshot({ path });
  await test.info().attach(`parity/${frame}.png`, { path, contentType: "image/png" });
}

/** Prepare isolated transport fixtures, never product state via DOM/evaluate or an actual Meta write. */
async function mockInbox(page: Page, copy: Copy) {
  const future = new Date(Date.now() + 6 * 60 * 60 * 1000).toISOString();
  const incoming = copy.locale === "en" ? "What material is it?" : copy.locale === "zh-CN" ? "想了解材质。" : "想了解材質。";
  const state = { mode: "eligible", delivery: null as Delivery | null, omitOutbound: false, concurrentSent: false, original: "", posts: 0, queries: 0,
    laterDelivery: null as Delivery | null, omitLater: false, concurrentAfterLater: false, laterInbound: true,
    methods: [] as string[], read: new Set<string>(), operationConversation: "", operationReads: 0, sendKeys: [] as string[] };
  const rows = [ids.open, ids.instagram, ids.closed, ids.retry].map((id, i) => ({
    conversation_id: id, platform: i === 1 || i === 2 ? "instagram" : "messenger",
    display_name: `${copy.locale === "en" ? "Buyer" : copy.locale === "zh-CN" ? "买家" : "買家"} ${String.fromCharCode(65 + i)}`,
    last_at: "2026-10-29T06:32:00Z", unread: i === 1 || i === 2, unreplied: false,
    mode: "human", assignee: i < 3 ? fixtureAgent : null, window_open_until: future,
    linked_customer_id: null, link_pending_manual: false,
  }));
  const answer = (body: unknown, status = 200) => ({ status, contentType: "application/json",
    headers: { "Cache-Control": "private, no-store" }, body: JSON.stringify(body) });
  // Transport preparation: the existing W6-05B detail read is tied to the actual A12 receipt.
  await page.route(`**/api/stores/${store}/operations/${fixtureOperation}`, async route => {
    expect(route.request().method()).toBe("GET");
    state.operationReads++;
    const row = rows.find(row => row.conversation_id === state.operationConversation)!;
    const states = { queued: "READY", sent: "SUCCEEDED", failed: "FAILED_FINAL", blocked: "BLOCKED_POLICY", unknown: "UNKNOWN" };
    await route.fulfill(answer({ operation_id: fixtureOperation, provider: row.platform === "instagram" ? "instagram" : "facebook",
      action: "meta.dm_send", purpose: "service", state: states[state.delivery ?? "queued"], reason_code: "", attempts: 1,
      created_at: "2026-10-29T06:35:00Z", updated_at: "2026-10-29T06:35:30Z",
      object: { kind: "conversation", id: state.operationConversation },
      actions: Object.fromEntries(["query", "cancel", "retry"].map(key => [key, { available: false, reason: "operation_closed" }])), events: [] }));
  });
  await page.route(`**/api/stores/${store}/meta/health`, async (route) => {
    const health = state.mode === "connection" ? "unknown" : state.mode === "permission" ? "missing_permission" : "ok";
    await route.fulfill(answer({ severity: "none", pages: [{ page_id: "900007320071", page_name: "Fixture channel",
      status: "active", severity: "none", capabilities: (["facebook", "instagram"] as const).map(provider => ({
        binding_id: fixtureBindings[provider], provider, capability: "dm_session", state: health,
        reason: "fixture_only", evidence: "MOCK",
      })) }] }));
  });
  await page.route(`**/api/stores/${store}/inbox/**`, async (route) => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    state.methods.push(`${request.method()} ${path}`);
    if (path.endsWith("/conversations")) {
      // A8 already supports all/unread/channel filters. Assignment must be derived from
      // returned assignee facts, never a new unsupported server filter or Figma's sample count.
      const filter = url.searchParams.get("filter") ?? "all";
      expect(["all", "unread", "unreplied", "messenger", "instagram", "live_comment"]).toContain(filter);
      const items = rows.map(row => ({ ...row, unread: row.unread && !state.read.has(row.conversation_id) }));
      const filtered = items.filter(row => filter === "all" || filter === "unread" && row.unread || row.platform === filter);
      return route.fulfill(answer({ items: filtered, unread_total: items.filter(row => row.unread).length, next_cursor: "" }));
    }
    const id = url.searchParams.get("conversation_id") ?? path.split("/").at(-2)!;
    const row = rows.find(row => row.conversation_id === id) ?? rows[0];
    if (path.endsWith("/read")) {
      state.read.add(id);
      return route.fulfill(answer({ read_seq: 1 }));
    }
    if (path.endsWith("/messages") && request.method() === "POST") {
      state.posts++;
      const body = request.postDataJSON();
      expect(body.text).toBe(copy.draft);
      expect(request.headers()["idempotency-key"]).toMatch(/^[A-Za-z0-9_-]{8,128}$/);
      state.sendKeys.push(request.headers()["idempotency-key"]);
      state.original = body.text;
      state.delivery = "queued";
      state.operationConversation = id;
      return route.fulfill(answer({ send_state: "queued", takeover_generation: 2,
        operation_id: fixtureOperation, outbound_id: "10000000-0000-4000-8000-000000000076" }));
    }
    if (path.endsWith("/messages")) {
      state.queries++;
      const items = state.omitOutbound ? Array.from({ length: 50 }, (_, index) => ({
        direction: "in", seq: index + 2, at: "2026-10-29T06:36:00Z", text: incoming, attachments: null,
      })) : [
        { direction: "in", seq: 1, at: row.last_at, text: incoming, attachments: null },
        ...(state.delivery ? [{ direction: "out", outbound_id: "10000000-0000-4000-8000-000000000076", at: "2026-10-29T06:35:00Z", text: state.original,
          attachments: null, send_state: state.delivery, principal_id: fixtureAgent }] : []),
      ];
      // MOCK fixture: a different staff member completes B before the first A9 read of our ACKed A.
      if (state.concurrentSent && state.delivery) items.push(
        { direction: "out", outbound_id: "10000000-0000-4000-8000-000000000077", at: state.concurrentAfterLater ? "2026-10-29T06:36:30Z" : "2026-10-29T06:35:30Z", text: "MOCK_OTHER_STAFF_SENT", attachments: null,
          send_state: "sent", principal_id: "10000000-0000-4000-8000-000000000074" } as typeof items[number],
        { direction: "in", seq: 100, at: "2026-10-29T06:36:00Z", text: incoming, attachments: null },
      );
      // MOCK transport: after our operation succeeds, another staff send can remain unresolved.
      if (state.laterDelivery) {
        if (!state.omitLater) items.push({ direction: "out", outbound_id: "10000000-0000-4000-8000-000000000078", at: "2026-10-29T06:36:00Z",
          text: "MOCK_LATER_UNRESOLVED_STAFF", attachments: null, send_state: state.laterDelivery,
          principal_id: "10000000-0000-4000-8000-000000000074" } as typeof items[number]);
        if (state.laterInbound) items.push({ direction: "in", seq: 101, at: "2026-10-29T06:37:00Z", text: incoming, attachments: null });
      }
      return route.fulfill(answer({ items, has_unknown_outbound: state.delivery === "unknown", mode: "human", takeover_generation: 2,
      human_until: future, window_open_until: state.mode === "closed" ? "2020-01-01T00:00:00Z" : future,
      binding_id: fixtureBindings[row.platform === "instagram" ? "instagram" : "facebook"], link_version: 1 }));
    }
    if (path.endsWith("/buyer-panel")) return route.fulfill(answer({ display_name: row.display_name,
      platform: row.platform, purchase_ordinal: 0, claims: [], claim_total_minor: 0,
      linked_customer_id: null, window_open_until: future, link_pending_manual: false }));
    return route.continue();
  });
  return state;
}

async function actor(request: APIRequestContext, mode: "operator" | "qualified") {
  // Select an existing synthetic principal only; real signed OIDC still admits the session.
  // Never add grants to live_operator or bypass the product's integration:read check.
  const response = await request.post(`${api}/__test/inbox-actor`, {
    headers: { "X-Inbox-Fixture-Key": required("LC_BROWSER_INBOX_CONTROL_KEY") }, data: { mode },
  });
  expect(response.status(), "bounded fixture actor selection").toBe(204);
}
async function enter(page: Page, copy: Copy, mode: "operator" | "qualified" = "operator") {
  await actor(page.request, mode);
  await login(page);
  if (copy.locale !== "en") {
    await page.getByTestId("locale-switch").selectOption(copy.locale);
    await expect(page).toHaveURL(new RegExp(`/${copy.locale}/messages`));
  }
  const state = await mockInbox(page, copy);
  await page.reload();
  await expect(page.getByTestId("inbox-page")).toBeVisible();
  return state;
}
async function open(page: Page, id: string) {
  await page.getByTestId(`conversation-${id}`).click();
  await expect(page.getByTestId("inbox-thread")).toBeVisible();
  await expect(page.getByTestId("inbox-thread")).toHaveAttribute("aria-busy", "false");
  await expect(page.getByTestId("inbox-thread").locator('time[datetime="2026-10-29T06:32:00Z"]')).toHaveText("14:32");
}
async function query(page: Page, refreshLabel?: string) {
  const read = page.waitForResponse(response => response.request().method() === "GET" &&
    new URL(response.url()).pathname.includes("/inbox/conversations/") && new URL(response.url()).pathname.endsWith("/messages"));
  // Confirmed DM results reopen the composer. Subsequent observations use the signed thread Refresh action.
  const status = page.getByTestId("inbox-send-status");
  const label = refreshLabel ?? copies.find(copy => new URL(page.url()).pathname.startsWith(`/${copy.locale}/`))!.refresh;
  await (!refreshLabel && await status.isVisible() ? status
    : page.getByTestId("inbox-thread").getByRole("button", { name: label, exact: true })).click();
  expect((await read).status()).toBe(200);
  await expect(page.getByTestId("inbox-thread")).toHaveAttribute("aria-busy", "false");
}
async function send(page: Page, copy: Copy) {
  await page.getByTestId("reply-text").fill(copy.draft);
  await expect(page.getByTestId("reply-send")).toHaveText(copy.send);
  await expect(page.getByTestId("reply-send")).toBeEnabled();
  const ack = page.waitForResponse(response => response.request().method() === "POST" &&
    new URL(response.url()).pathname.endsWith("/messages"));
  await page.getByTestId("reply-send").click();
  expect((await ack).status()).toBe(200);
  await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.pending);
  await expect(page.getByTestId("reply-text")).toBeHidden();
  await expect(page.getByTestId("inbox-send-locked")).toBeVisible();
  await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
  await expect(page.getByTestId("inbox-thread").getByText(copy.draft, { exact: true })).toHaveCount(1);
  record(`send ${copy.locale}`, "fill then click Send", "one MOCK POST, one original outgoing body, composer folded after ACK");
}

/** READ/MEASURE only: visible DOM text nodes, hidden ancestors excluded, UTF-16 lengths, no viewport exclusion. */
async function density(page: Page) {
  return page.getByTestId("inbox-page").evaluate(root => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const blocks: string[] = [];
    let node: Node | null;
    while ((node = walker.nextNode())) {
      const text = node.textContent?.trim();
      if (!text) continue;
      let parent = node.parentElement, visible = true;
      while (parent) {
        const style = getComputedStyle(parent);
        if (style.display === "none" || style.visibility === "hidden" || parent.hidden) visible = false;
        if (parent === root) break;
        parent = parent.parentElement;
      }
      if (visible) blocks.push(text);
    }
    return { blocks: blocks.length, chars: blocks.reduce((total, text) => total + text.length, 0) };
  });
}

for (const copy of copies) {
  test(`INV2 ${copy.locale} main`, async ({ page }) => {
    const state = await enter(page, copy);
    const scope = page.getByTestId("inbox-page");
    await expect(scope.getByRole("heading", { level: 1 })).toHaveText(copy.title);
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      const chips = scope.locator('[data-testid^="inbox-filter-"]').filter({ visible: true });
      await expect(chips.nth(0)).toHaveText(`${copy.all} 4`);
      await expect(chips.nth(1)).toHaveText(`${copy.unread} 2`);
      await expect(chips.nth(2)).toHaveText(`${copy.assigned} 3`);
      await expect(chips.nth(0)).toHaveAttribute("aria-pressed", "true");
      await expect(chips.nth(0)).toBeDisabled();
      await expect(chips.nth(1)).toBeEnabled();
      await expect(chips.nth(2)).toBeEnabled();
      await page.getByTestId("inbox-filter-assigned").click();
      await expect(page.getByTestId(`conversation-${ids.retry}`)).toBeHidden();
      await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
      await expect(chips.nth(2)).toHaveAttribute("aria-pressed", "true");
      await expect(chips.nth(2)).toBeDisabled();
      await expect(chips.nth(0)).toBeEnabled();
      await page.getByTestId("inbox-filter-unread").click();
      await expect(page.getByTestId(`conversation-${ids.open}`)).toBeHidden();
      await open(page, ids.instagram);
      await expect(page.getByTestId("inbox-thread")).toContainText(copy.read);
      const disclosure = page.getByTestId("customer-details-toggle");
      await expect(disclosure).toContainText(copy.details);
      await expect(disclosure).toContainText(copy.paused);
      await expect(disclosure).toHaveAttribute("aria-expanded", "false");
      await expect(page.getByTestId("customer-details")).toBeHidden();
      await disclosure.click();
      await expect(disclosure).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByTestId("customer-details")).toBeVisible();
      await expect(page.getByTestId("customer-details")).toContainText(copy.unavailable);
      await disclosure.click();
      await expect(page.getByTestId("customer-details")).toBeHidden();
      await page.getByTestId("reply-text").fill(copy.draft);
      await page.getByTestId("inbox-clear-draft").click();
      await expect(page.getByTestId("reply-text")).toHaveValue("");
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      await page.getByTestId("inbox-back").click();
      await expect(page.getByTestId("inbox-thread")).toHaveCount(0);
      await expect(page.getByTestId("inbox-list")).toBeVisible();
      await page.reload();
      await page.getByTestId("inbox-filter-unread").click();
      await expect(page.getByTestId(`conversation-${ids.instagram}`)).toBeHidden();
      await page.getByTestId("inbox-filter-all").click();
      await page.getByTestId("inbox-filter-instagram").click();
      await expect(page.getByTestId("inbox-filter-instagram")).toBeDisabled();
      await expect(page.getByTestId(`conversation-${ids.open}`)).toBeHidden();
      await expect(page.getByTestId(`conversation-${ids.instagram}`)).toBeVisible();
      await page.getByTestId("inbox-filter-all").click();
      // READ/MEASURE: row/parent geometry only, no element or product-state writes.
      const widths = await page.getByTestId(`conversation-${ids.open}`).evaluate(row => ({
        row: row.getBoundingClientRect().width, parent: row.parentElement!.getBoundingClientRect().width,
        overflow: document.documentElement.scrollWidth > innerWidth,
      }));
      expect(Math.abs(widths.row - widths.parent), "row fills available parent width").toBeLessThanOrEqual(1);
      expect(widths.overflow).toBe(false);
      record(`main ${copy.locale}/${width}`, "click filters, row, disclosure, clear; reload", "fixed slots; unread persists; customer facts folded; full-width rows");
      await page.screenshot({ path: resolve(evidence, `inbox-v2-main-${copy.locale}-${width}.png`), fullPage: true });
      // Reset read fixture only for the next viewport. This is fixture-server setup, not UI acceptance.
      state.read.clear();
      await page.reload();
    }
  });

  for (const omitted of [false, true]) {
    test(`INV2 ${copy.locale} ACK before first projection ${omitted ? "omitted" : "visible"}`, async ({ page }) => {
      const state = await enter(page, copy, "qualified");
      await page.setViewportSize({ width: 390, height: 844 });
      await open(page, ids.instagram);
      // Prepare transport facts before the native send; no direct API or DOM write performs the action.
      state.concurrentSent = true;
      state.omitOutbound = omitted;
      await page.getByTestId("reply-text").fill(copy.draft);
      await expect(page.getByTestId("reply-send")).toBeEnabled();
      const ack = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname.endsWith("/messages"));
      const firstRead = page.waitForResponse(r => r.request().method() === "GET" && new URL(r.url()).pathname.endsWith("/messages"));
      await page.getByTestId("reply-send").click();
      expect((await ack).status()).toBe(200);
      expect((await firstRead).status()).toBe(200);
      await expect(page.getByTestId("inbox-thread")).toHaveAttribute("aria-busy", "false");
      for (let queryCount = 0; queryCount < 3; queryCount++) {
        if (queryCount) await query(page);
        await expect(page.getByTestId("inbox-thread")).toContainText("MOCK_OTHER_STAFF_SENT");
        await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.pending);
        await expect(page.getByTestId("reply-send")).toBeDisabled();
        await expect(page.getByTestId("reply-text")).toBeHidden();
        await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
        expect(state.posts).toBe(1);
      }
      expect(state.operationReads).toBe(3);
      state.delivery = "sent"; // MOCK worker completion; only the native status click observes it.
      await query(page);
      await expect(page.getByTestId("reply-text")).toBeVisible();
      await page.getByTestId("reply-text").fill(copy.draft);
      await expect(page.getByTestId("reply-send")).toBeEnabled();
      expect(state.posts).toBe(1);
      record(`ACK concurrency ${copy.locale}/${omitted ? "omitted" : "visible"}`, "fill, click Send, click status twice",
        "pending retained despite unrelated sent + newer inbound; exact-operation success alone reopens composer; only one POST");
    });
  }

  for (const later of ["queued", "blocked"] as const) {
    test(`INV2 ${copy.locale} successful ACK then distinct ${later} rejection`, async ({ page }) => {
      const state = await enter(page, copy, "qualified");
      await page.setViewportSize({ width: 390, height: 844 });
      await open(page, ids.instagram);
      await send(page, copy);
      // Fixture-server facts only. Native refresh observes A's completion and B's later rejection.
      state.delivery = "sent";
      state.laterInbound = false;
      await query(page);
      await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.sent);
      state.laterDelivery = later;
      await query(page);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      state.laterDelivery = "failed";
      for (let refresh = 0; refresh < 2; refresh++) {
        await query(page);
        await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.failed);
        await expect(page.getByTestId("inbox-edit-original")).toBeVisible();
        expect(state.posts).toBe(1);
      }
      await page.getByTestId("inbox-edit-original").click();
      await expect(page.getByTestId("reply-text")).toHaveValue("MOCK_LATER_UNRESOLVED_STAFF");
      // A fresh component epoch must show the same server-backed failure and recovery target.
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.failed);
      await page.getByTestId("inbox-edit-original").click();
      await expect(page.getByTestId("reply-text")).toHaveValue("MOCK_LATER_UNRESOLVED_STAFF");
      expect(state.posts).toBe(1);
      record(`independent rejection ${copy.locale}/${later}`,
        "fill, Send, status A success, status B pending then failed twice, Edit original, reload/reopen, Edit original",
        "B rejection visible without later inbound; B text restored before/after reload; exactly one POST");
    });

    for (const timing of ["with-completion", "observed-before-completion"] as const) {
      test(`INV2 ${copy.locale} local ACK older ${later} ${timing}`, async ({ page }) => {
        const state = await enter(page, copy, "qualified");
        await page.setViewportSize({ width: 390, height: 844 });
        await open(page, ids.instagram);
        // Transport setup: actual local A ACK, then older B below sent C and a newer inbound.
        state.concurrentSent = true;
        state.concurrentAfterLater = true;
        await send(page, copy);
        state.laterDelivery = later;
        if (timing === "with-completion") state.delivery = "sent";
        await query(page, copy.refresh);
        const assertLocked = async (pendingAck = false) => {
          await expect(page.getByTestId("reply-text")).toBeHidden();
          await expect(page.getByTestId("reply-send")).toBeDisabled();
          await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
          await expect(page.getByTestId("inbox-delivery-state")).toHaveText(pendingAck || later === "queued" ? copy.pending
            : copy.locale === "en" ? "Sending locked" : copy.locale === "zh-CN" ? "发送已锁定" : "已鎖定");
          expect(state.posts).toBe(1);
        };
        await assertLocked(timing === "observed-before-completion");
        // A completes as B disappears from A9; omission cannot settle B observed during pending A.
        state.delivery = "sent";
        state.omitLater = true;
        await query(page, copy.refresh);
        await assertLocked();
        state.omitLater = false;
        await query(page, copy.refresh);
        await assertLocked();
        expect(state.operationReads).toBeGreaterThanOrEqual(2);
        // Persisted server projection B must retain the same lock in a fresh component epoch.
        await page.reload();
        await open(page, ids.instagram);
        await assertLocked();
        state.omitLater = true;
        await query(page, copy.refresh);
        await assertLocked();
        state.omitLater = false;
        state.laterDelivery = "sent";
        await query(page, copy.refresh);
        await expect(page.getByTestId("reply-text")).toBeVisible();
        await page.getByTestId("reply-text").fill(copy.draft);
        await expect(page.getByTestId("reply-send")).toBeEnabled();
        expect(state.posts).toBe(1);
        record(`local ACK older obligation ${copy.locale}/${later}/${timing}`,
          "fill, Send, refresh own ACK + older B under sent C; omit/reappear B; reload/reopen; refresh explicit B settlement",
          "one POST; composer folded and Send disabled until exact B settlement, including after reload");
      });
    }

    test(`INV2 ${copy.locale} completed ACK before later ${later}`, async ({ page }) => {
      const state = await enter(page, copy, "qualified");
      await page.setViewportSize({ width: 390, height: 844 });
      await open(page, ids.instagram);
      await send(page, copy);
      state.delivery = "sent"; // Transport setup; native status click observes exact own success.
      state.concurrentSent = true;
      await query(page);
      await expect(page.getByTestId("reply-text")).toBeVisible();
      await page.getByTestId("reply-text").fill(copy.draft);
      await expect(page.getByTestId("reply-send")).toBeEnabled();
      state.laterDelivery = later;
      for (const omitted of [false, true, false]) {
        state.omitLater = omitted;
        // Own success unfolded the composer and removed its status action; refresh remains available.
        await query(page, copy.refresh);
        await expect(page.getByTestId("inbox-delivery-state")).toHaveText(later === "queued" ? copy.pending
          : copy.locale === "en" ? "Sending locked" : copy.locale === "zh-CN" ? "发送已锁定" : "已鎖定");
        await expect(page.getByTestId("reply-text")).toBeHidden();
        await expect(page.getByTestId("reply-send")).toBeDisabled();
        await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
        expect(state.posts).toBe(1);
      }
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      state.omitLater = true;
      await query(page);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      expect(state.posts).toBe(1);
      record(`completed ACK ${copy.locale}/${later}`, "fill, Send, status, thread refresh with later row visible/omitted; reload and status",
        "own success never resolves later pending/policy-blocked send; composer locked; one POST; reload retains visible server result");

      // Transport setup only: start a new component epoch with projected A and no local ACK.
      // A settles on the same refresh that exposes unresolved B; a newer buyer row is already present.
      state.delivery = "queued";
      state.laterDelivery = null;
      state.omitOutbound = false;
      state.omitLater = false;
      state.concurrentSent = false;
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      state.delivery = "sent";
      state.laterDelivery = later;
      for (const omitted of [false, true, false]) {
        state.omitLater = omitted;
        await query(page, copy.refresh);
        await expect(page.getByTestId("inbox-delivery-state")).toHaveText(later === "queued" ? copy.pending
          : copy.locale === "en" ? "Sending locked" : copy.locale === "zh-CN" ? "发送已锁定" : "已鎖定");
        await expect(page.getByTestId("reply-text")).toBeHidden();
        await expect(page.getByTestId("reply-send")).toBeDisabled();
        await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
        expect(state.posts).toBe(1);
      }
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      state.laterDelivery = "sent";
      await query(page, copy.refresh);
      await expect(page.getByTestId("reply-text")).toBeVisible();
      await page.getByTestId("reply-text").fill(copy.draft);
      await expect(page.getByTestId("reply-send")).toBeEnabled();
      expect(state.posts).toBe(1);
      record(`projected handoff ${copy.locale}/${later}`, "thread refresh A sent + B pending, omit B, reload, refresh B sent, fill",
        "B remains locked through omission and reload; only all-settled server facts permit a next draft; no extra POST");

      // Both rows are observed before newer B settles and older A leaves the display page.
      state.delivery = later;
      state.laterDelivery = later;
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      state.laterDelivery = "sent";
      state.omitOutbound = true;
      await query(page, copy.refresh);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      expect(state.posts).toBe(1);
      record(`multiple projected obligations ${copy.locale}/${later}`, "reload two pending rows; refresh newer B sent while older A is omitted",
        "all observed obligations remain tracked independently; no extra POST");

      // Transport setup: older A remains unresolved underneath newer sent B on initial entry.
      state.delivery = later;
      state.omitOutbound = false;
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      state.omitOutbound = true;
      await query(page, copy.refresh);
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      expect(state.posts).toBe(1);
      record(`older projected obligation ${copy.locale}/${later}`, "reload then thread refresh omitting older unresolved A beneath sent B",
        "initial entry and omission keep the composer folded and Send disabled; no fresh POST");
    });
  }

  test(`INV2 ${copy.locale} ACK without operation read permission`, async ({ page }) => {
    for (const terminal of ["sent", "failed"] as const) {
      for (const refresh of [false, true]) {
        // Fixture preparation: each variant must sign in afresh before installing its own transport.
        await page.unrouteAll({ behavior: "wait" });
        await page.context().clearCookies();
        // The shared signed-login helper uses desktop navigation; mobile coverage resumes after entry.
        await page.setViewportSize({ width: 1280, height: 900 });
        const state = await enter(page, copy);
        await page.setViewportSize({ width: 390, height: 844 });
        await open(page, ids.instagram);
        state.concurrentSent = true; // Transport preparation: unrelated sent B never settles our ACK A.
        await send(page, copy);
        await expect(page.getByTestId("inbox-thread")).toHaveAttribute("aria-busy", "false");
        for (const pending of ["omitted", "queued", "blocked"] as const) {
          state.delivery = pending === "omitted" ? "sent" : pending;
          state.omitOutbound = pending === "omitted";
          await query(page, refresh ? copy.refresh : undefined);
          await expect(page.getByTestId("inbox-thread")).toContainText("MOCK_OTHER_STAFF_SENT");
          await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.pending);
          await expect(page.getByTestId("reply-text")).toBeHidden();
          await expect(page.getByTestId("reply-send")).toBeDisabled();
          await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
          expect(state.operationReads).toBe(0);
          expect(state.posts).toBe(1);
        }
        state.omitOutbound = false;
        state.delivery = terminal;
        await query(page, refresh ? copy.refresh : undefined);
        await expect(page.getByTestId("inbox-delivery-state")).toHaveText(terminal === "sent" ? copy.sent : copy.failed);
        if (terminal === "failed") {
          await page.getByTestId("inbox-edit-original").click();
          await expect(page.getByTestId("reply-text")).toHaveValue(copy.draft);
        } else {
          await expect(page.getByTestId("reply-text"), "own A9 sent row unlocks without integration:read").toBeVisible();
          await expect(page.getByTestId("reply-text")).toHaveValue("");
          await page.getByTestId("reply-text").fill(copy.draft);
        }
        await expect(page.getByTestId("reply-send")).toBeEnabled();
        expect(state.operationReads).toBe(0);
        expect(state.posts).toBe(1);
        if (terminal === "sent") {
          await page.reload();
          await open(page, ids.instagram);
          await page.getByTestId("reply-text").fill(copy.draft);
          await expect(page.getByTestId("reply-send")).toBeEnabled();
          expect(state.posts).toBe(1);
        }
        record(`ACK permission ${copy.locale}/${terminal}/${refresh ? "refresh" : "status"}`,
          "fill, Send, query omitted/queued/blocked own row, query terminal own row; edit rejection or reload sent",
          "only own terminal A9 ledger row settles ACK without operation read; next draft usable; exactly one POST");
      }
    }
  });

  test(`INV2 ${copy.locale} states`, async ({ page }) => {
    const state = await enter(page, copy, "qualified");
    const helpCopy = copy.locale === "en" ? {
      window: "The reply window has ended; wait for the buyer to message again.",
      connection: "Check the connection and reply permissions in Settings before sending.",
    } : copy.locale === "zh-CN" ? {
      window: "已超过可回复时间；请等待买家再次发消息。",
      connection: "请在设置中检查连接和回复权限；确认前无法发送。",
    } : {
      window: "已超過回覆時間；等買家再次傳訊息。",
      connection: "到設定檢查連接與回覆權限；確認前無法傳送。",
    };
    await page.setViewportSize({ width: 390, height: 844 });
    const densities: Array<{ blocks: number; chars: number }> = [];
    const capture = async (name: string) => {
      densities.push(await density(page));
      await page.screenshot({ path: resolve(evidence, `inbox-v2-${copy.locale}-${name}.png`), fullPage: true });
    };
    await open(page, ids.instagram);
    await expect(page.getByTestId("customer-details-toggle")).toHaveAttribute("aria-expanded", "false");
    await capture("eligible");
    await send(page, copy);
    expect(state.posts).toBe(1);
    await capture("queued");
    // Fixture setup: the A9 display floor now hides the unresolved outbound, with explicit false UNKNOWN authority.
    state.omitOutbound = true;
    for (let refresh = 0; refresh < 2; refresh++) {
      await query(page);
      await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.pending);
      await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      expect(state.posts).toBe(1);
    }
    record(`queued omission ${copy.locale}`, "click status twice", "inbound-only refresh retains pending result and send lock; no fresh POST");
    state.omitOutbound = false;
    state.delivery = "sent"; // MOCK worker result; only the subsequent real status click observes it.
    await query(page);
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.sent);
    await capture("sent");
    await page.reload();
    await open(page, ids.instagram);
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.sent);
    expect(state.posts).toBe(1);
    record(`sent ${copy.locale}`, "click status and reload", "server-fixture sent result survives refresh; no resend");

    state.delivery = "failed"; // Known final rejection is distinct from uncertain delivery.
    await page.reload();
    await open(page, ids.instagram);
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.failed);
    await capture("failed");
    await page.getByTestId("inbox-edit-original").click();
    await expect(page.getByTestId("reply-text")).toHaveValue(copy.draft);
    expect(state.posts).toBe(1);
    record(`rejected ${copy.locale}`, "click Edit original text", "original restored for editing, no automatic resubmission");

    state.delivery = "unknown";
    await page.reload();
    await open(page, ids.instagram);
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.unknown);
    await expect(page.getByTestId("reply-send")).toBeDisabled();
    await expect(page.getByTestId("reply-text")).toBeHidden();
    await expect(page.getByTestId("inbox-edit-original")).toBeHidden();
    await capture("unknown");
    const reads = state.queries;
    await query(page);
    expect(state.queries).toBeGreaterThan(reads);
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.still);
    await capture("still-unconfirmed");
    await page.reload();
    await open(page, ids.instagram);
    await expect(page.getByTestId("reply-send")).toBeDisabled();
    expect(state.posts).toBe(1);
    record(`unknown ${copy.locale}`, "click status, reload and reopen", "only query; no resend; Instagram named correctly; sending remains locked");

    state.delivery = "blocked";
    await page.reload();
    await open(page, ids.instagram);
    await expect(page.getByTestId("reply-send")).toBeDisabled();
    await capture("blocked");
    expect(state.posts).toBe(1);
    const blockedResult = await page.getByTestId("inbox-delivery-state").textContent();
    state.omitOutbound = true; // Fixture setup only; the real status clicks observe the changed A9 display page.
    for (let refresh = 0; refresh < 2; refresh++) {
      await query(page);
      await expect(page.getByTestId("inbox-delivery-state")).toHaveText(blockedResult!);
      await expect(page.getByTestId("inbox-send-locked")).toBeDisabled();
      await expect(page.getByTestId("reply-text")).toBeHidden();
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      expect(state.posts).toBe(1);
    }
    record(`blocked omission ${copy.locale}`, "click status twice", "inbound-only refresh retains blocked result and send lock; no fresh POST");
    state.omitOutbound = false;

    state.delivery = null;
    for (const [mode, reason] of [["closed", copy.closed], ["connection", copy.connection], ["permission", copy.permission]]) {
      state.mode = mode;
      await page.reload();
      await open(page, ids.instagram);
      await expect(page.getByTestId("inbox-eligibility")).toContainText(reason);
      await expect(page.getByTestId("reply-send")).toBeDisabled();
      await capture(mode);
      // The owning form disclosure reveals the concrete restriction explanation.
      const helpButton = page.getByTestId("inbox-eligibility").locator("..").locator('button[aria-expanded]');
      await helpButton.click();
      await expect(helpButton).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByTestId("inbox-thread")).toContainText(mode === "closed" ? helpCopy.window : helpCopy.connection);
      await capture(`${mode}-help`);
      await helpButton.click();
      await expect(helpButton).toHaveAttribute("aria-expanded", "false");
      expect(state.posts).toBe(1);
      record(`${mode} ${copy.locale}`, "open conversation", "named restriction visible; send disabled; no POST");
    }
    // Visible text read only: internal codes and fixture principal identifiers never become seller copy.
    await expect(page.getByTestId("inbox-page")).not.toContainText(/\b(?:MOCK|UNKNOWN|queued)\b/);
    await expect(page.getByTestId("inbox-page")).not.toContainText(fixtureAgent);
    await writeFile(resolve(evidence, `inbox-v2-density-${copy.locale}.json`), JSON.stringify(densities, null, 2));
    if (copy.locale === "zh-TW") {
      const median = (key: "blocks" | "chars") => [...densities].sort((a, b) => a[key] - b[key])[Math.floor(densities.length / 2)][key];
      expect(median("blocks"), "signed mobile text-block median").toBeLessThanOrEqual(12);
      expect(median("chars"), "signed mobile UTF-16 character median").toBeLessThanOrEqual(100);
      expect(Math.max(...densities.map(row => row.blocks)), "no mobile state over 20 blocks").toBeLessThanOrEqual(20);
    }
  });
}

// PARITY-INBOX-EDITOR: literal signed copy/controls on the real route. Existing
// ACK/UNKNOWN/adversarial assertions above stay intact; preview is INBOX-PREVIEW.
for (const copy of copies) {
  test(`PARITY inbox ${copy.locale}: signed list controls and no legacy toolbar`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await enter(page, copy);
    const list = page.getByTestId("inbox-list");
    const toolbar = page.getByTestId("inbox-filter-all").locator("..");
    await expect(list).toBeVisible();
    await expect(toolbar.getByRole("button", { name: `${copy.all} 4`, exact: true })).toBeVisible();
    await expect(toolbar.getByRole("button", { name: `${copy.unread} 2`, exact: true })).toBeVisible();
    await expect(toolbar.getByRole("button", { name: `${copy.assigned} 3`, exact: true })).toBeVisible();
    await expect(toolbar.getByRole("button", { name: copy.refresh, exact: true }), "unsigned list refresh is removed").toHaveCount(0);
    await expect(page.getByTestId("inbox-more-filters"), "unsigned ellipsis is removed").toHaveCount(0);
    await open(page, ids.open);
    if (copy.locale === "zh-TW") await inboxParity(page, "1429:37361");
    record(`PARITY list ${copy.locale}`, "open real Messages and click row", "signed filters; no extra refresh or ellipsis");
  });

  test(`PARITY inbox ${copy.locale}: own sent DM reopens follow-ups without another inbound`, async ({ page }) => {
    const state = await enter(page, copy);
    await open(page, ids.instagram);
    await send(page, copy);
    state.delivery = "sent"; // MOCK worker completion only; merchant observes it through the status control.
    await query(page);
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.sent);
    await expect(page.getByTestId("reply-text"), "24h DM follow-up needs no new buyer message").toBeVisible();
    await expect(page.getByTestId("reply-text")).toHaveValue("");
    expect(state.concurrentSent).toBe(false);
    expect(state.laterDelivery).toBeNull();
    await page.reload();
    await open(page, ids.instagram);
    await expect(page.getByTestId("reply-text"), "confirmed DM follow-up persists after refresh").toBeVisible();
    await page.getByTestId("reply-text").fill(copy.draft);
    await expect(page.getByRole("button", { name: copy.send, exact: true })).toBeEnabled();
    await page.getByRole("button", { name: copy.send, exact: true }).click();
    await expect(page.getByTestId("inbox-delivery-state")).toHaveText(copy.pending);
    await expect(page.getByTestId("reply-text")).toBeHidden();
    expect(state.posts).toBe(2);
    expect(new Set(state.sendKeys).size, "follow-up is a new intent, never a replay of the first ACK").toBe(2);
    record(`PARITY follow-up ${copy.locale}`, "Send, status, reload, fill, Send", "two distinct DM intents; no new inbound; pending still locks");
  });
}

for (const mobile of [false, true]) {
  const copy = copies[0];
  for (const scenario of ["connection", "closed", "permission", "failed", "queued", "unknown", "still", "sent"] as const) {
    for (const buyer of ["A", "B"] as const) {
      if (buyer === "B" && !["queued", "unknown", "still"].includes(scenario)) continue;
    test(`PARITY inbox ${mobile ? "mobile" : "desktop"} ${scenario} ${buyer}: signed send state`, async ({ page }) => {
      await page.setViewportSize(mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 });
      const state = await enter(page, copy);
      if (["connection", "closed", "permission"].includes(scenario)) state.mode = scenario;
      await open(page, buyer === "A" ? ids.open : ids.instagram);
      const frames = {
        connection: ["1429:37998", "1429:39190"], closed: ["1429:38090", "1429:39229"],
        permission: ["1429:38182", "1429:39268"], failed: ["1429:38274", "1429:39307"],
        queued: ["1429:38366", "1429:39346"], unknown: ["1429:38458", "1429:39385"],
        still: ["1429:38550", "1429:39424"], sent: ["1429:38642", "1429:39463"],
      };
      if (scenario === "connection" || scenario === "closed" || scenario === "permission") {
        await expect(page.getByTestId("inbox-eligibility")).toContainText(copy[scenario]);
        await expect(page.getByRole("button", { name: copy.send, exact: true })).toBeDisabled();
      } else {
        await send(page, copy);
        if (scenario !== "queued") {
          state.delivery = scenario === "still" ? "unknown" : scenario;
          await query(page);
        }
        if (scenario === "still") await query(page);
        const expected = { queued: copy.pending, failed: copy.failed, unknown: copy.unknown,
          still: buyer === "A" ? copy.still.replace("Instagram", "Messenger") : copy.still, sent: copy.sent };
        await expect(page.getByTestId("inbox-delivery-state")).toHaveText(expected[scenario]);
        if (scenario === "sent") await expect(page.getByTestId("reply-text")).toBeVisible();
        else await expect(page.getByTestId("reply-text")).toBeHidden();
      }
      if (buyer === "B" && (scenario === "queued" || scenario === "unknown" || scenario === "still")) {
        const b = { queued: ["1429:40688", "1429:41042"], unknown: ["1429:40780", "1429:41081"], still: ["1429:40872", "1429:41120"] };
        await inboxParity(page, b[scenario][mobile ? 1 : 0]);
      } else await inboxParity(page, frames[scenario][mobile ? 1 : 0]);
      if (scenario === "failed") {
        await page.getByRole("button", { name: copy.edit, exact: true }).click();
        await expect(page.getByTestId("reply-text")).toHaveValue(copy.draft);
        expect(state.posts).toBe(1);
      }
      record(`PARITY ${mobile ? "mobile" : "desktop"}/${scenario}`, "open, fill/Send and status or restriction", "signed state copy and safe action observed");
    });
    }
  }
}

for (const mobile of [false, true]) {
  test(`PARITY inbox ${mobile ? "mobile" : "desktop"}: fixed filter slots, list/thread and help frames`, async ({ page }) => {
    const copy = copies[0];
    const state = await enter(page, copy);
    await page.setViewportSize(mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 });
    const shot = (desktop: string, phone: string) => inboxParity(page, mobile ? phone : desktop);
    if (mobile) await inboxParity(page, "1429:38826");
    await page.getByRole("button", { name: `${copy.unread} 2`, exact: true }).click();
    await expect(page.getByTestId(`conversation-${ids.open}`)).toBeHidden();
    await shot("1429:37452", "1429:38876");
    await open(page, ids.instagram);
    await expect(page.getByTestId("inbox-thread")).toContainText(copy.read);
    await shot("1429:37543", "1429:39114");
    await page.getByTestId("inbox-back").click();
    await expect(page.getByTestId(`conversation-${ids.instagram}`)).toBeHidden();
    await shot("1429:37634", "1429:38926");
    await page.getByTestId("inbox-filter-assigned").click();
    await expect(page.getByTestId(`conversation-${ids.retry}`)).toBeHidden();
    if (mobile) await inboxParity(page, "1429:38976");
    await open(page, ids.open);
    await shot("1429:37725", "1429:58696");
    await page.getByTestId("inbox-back").click();
    await page.getByTestId("inbox-filter-instagram").click();
    await expect(page.getByTestId(`conversation-${ids.open}`)).toBeHidden();
    if (mobile) await inboxParity(page, "1429:39026");
    await open(page, ids.instagram);
    await shot("1429:37816", "1429:58822");
    await page.getByTestId("inbox-back").click();
    await page.getByTestId("inbox-filter-all").click();
    await open(page, ids.instagram);
    await shot("1429:37907", "1429:39152");
    await page.getByTestId("inbox-back").click();
    await open(page, ids.open);
    await page.getByTestId("reply-text").fill(copy.draft);
    if (mobile) await inboxParity(page, "1429:39076");
    await page.getByRole("button", { name: copy.clear, exact: true }).click();
    await expect(page.getByTestId("reply-text")).toHaveValue("");
    for (const mode of ["connection", "closed"] as const) {
      state.mode = mode; // Transport restriction only; native reopening reads the server-shaped response.
      await page.reload();
      await open(page, ids.open);
      await expect(page.getByTestId("inbox-eligibility")).toContainText(copy[mode]);
      const name = mode === "closed" ? "查看窗口說明" : "檢查連接權限";
      await page.getByRole("button", { name, exact: true }).click();
      await expect(page.getByRole("button", { name, exact: true })).toHaveAttribute("aria-expanded", "true");
      await shot(mode === "closed" ? "1429:61183" : "1429:61092", mode === "closed" ? "1429:61350" : "1429:61312");
    }
    record(`PARITY filters/help ${mobile}`, "click filters, rows, Back, clear, help", "stable filters; store unread facts; restriction help; no send");
  });
}
