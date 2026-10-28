// Purpose: INBOX-V2 signed simplified UI real-click acceptance across all three locales.
// Depends on: existing signed MOCK OIDC/Next inbox runner; bounded MOCK A8/A9/A10/A12/A13 and health shapes.
// Used by: --browser-inbox CI, registered in playwright.config.ts; existing real PG/BFF send tests stay intact.
// Invariants: no LIVE send; no DOM writes; UNKNOWN queries never resend; refresh preserves fixture-server facts only.
import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { evidence, store, ids, login, createInboxLedger, writeInboxLedger } from "./inbox-browser-support";

const { ledger, record } = createInboxLedger();
test.use({ headless: true, trace: "off", video: "off", screenshot: "only-on-failure" });
test.afterEach(async ({}, info) => {
  if (info.status !== info.expectedStatus) ledger.push({ page: "Messages v2", control: info.title,
    action: "scenario", expected: "every named assertion passes", actual: info.status ?? "UNKNOWN", status: "FAIL" });
});
test.afterAll(async () => { await writeInboxLedger(ledger); });

const copies = [
  { locale: "zh-TW", title: "全店訊息", all: "全部", unread: "未讀", assigned: "本店分派",
    details: "顧客資料", paused: "自動回覆已停", read: "店內已讀", unavailable: "訂單與累計：未提供",
    send: "傳送", clear: "清除草稿", query: "查詢狀態", edit: "編輯原文",
    pending: "傳送中；尚未確認送達，勿重送。", sent: "已送出，未代表已讀。",
    failed: "傳送遭拒；原文保留，可編輯。", unknown: "送達待確認；只查詢，不重送。",
    still: "仍待確認；到 Instagram 查看，勿重送。", closed: "已超過回覆時間。",
    connection: "連接待確認；暫不能回覆。", permission: "缺少回覆權限。",
    draft: "我會先核對尺寸，再回覆你。" },
  { locale: "zh-CN", title: "全店消息", all: "全部", unread: "未读", assigned: "店铺分配",
    details: "客户资料", paused: "自动回复已停", read: "店内已读", unavailable: "订单与累计消费：未提供",
    send: "发送", clear: "清空草稿", query: "查询状态", edit: "编辑原文",
    pending: "发送中；尚未确认送达，不要重发。", sent: "已发送，不代表已读。",
    failed: "发送被拒；原文已保留，可以编辑。", unknown: "送达情况待确认；只查询，不重发。",
    still: "尚未确认；请到 Instagram 查看，不要重发。", closed: "已超过可回复时间。",
    connection: "连接待确认；暂时无法回复。", permission: "缺少回复权限。",
    draft: "我先核实尺寸，再回复你。" },
  { locale: "en", title: "Store inbox", all: "All", unread: "Unread", assigned: "Assigned in store",
    details: "Customer details", paused: "Auto-replies paused", read: "Read by store",
    unavailable: "Orders and lifetime spend: not provided", send: "Send", clear: "Clear draft",
    query: "Check status", edit: "Edit original text", pending: "Sending; delivery unconfirmed. Do not resend.",
    sent: "Sent does not mean read.", failed: "Send rejected; original text saved for editing.",
    unknown: "Delivery unconfirmed; check status without resending.",
    still: "Still unconfirmed; check Instagram. Do not resend.", closed: "The reply window has ended.",
    connection: "Connection unconfirmed; replies are unavailable.", permission: "Reply permission is missing.",
    draft: "I will check the dimensions and get back to you." },
] as const;
type Copy = typeof copies[number];
type Delivery = "queued" | "sent" | "failed" | "blocked" | "unknown";
const fixtureBinding = "10000000-0000-4000-8000-000000000071";
const fixtureAgent = "10000000-0000-4000-8000-000000000072";

/** Prepare isolated transport fixtures, never product state via DOM/evaluate or an actual Meta write. */
async function mockInbox(page: Page, copy: Copy) {
  const future = new Date(Date.now() + 6 * 60 * 60 * 1000).toISOString();
  const incoming = copy.locale === "en" ? "What material is it?" : copy.locale === "zh-CN" ? "想了解材质。" : "想了解材質。";
  const state = { mode: "eligible", delivery: null as Delivery | null, original: "", posts: 0, queries: 0,
    methods: [] as string[], read: new Set<string>() };
  const rows = [ids.open, ids.instagram, ids.closed, ids.retry].map((id, i) => ({
    conversation_id: id, platform: i === 1 || i === 2 ? "instagram" : "messenger",
    display_name: `${copy.locale === "en" ? "Buyer" : copy.locale === "zh-CN" ? "买家" : "買家"} ${String.fromCharCode(65 + i)}`,
    last_at: "2026-10-29T06:32:00Z", unread: i === 1 || i === 2, unreplied: false,
    mode: "human", assignee: i < 3 ? fixtureAgent : null, window_open_until: future,
    linked_customer_id: null, link_pending_manual: false,
  }));
  const answer = (body: unknown, status = 200) => ({ status, contentType: "application/json",
    headers: { "Cache-Control": "private, no-store" }, body: JSON.stringify(body) });
  await page.route(`**/api/stores/${store}/meta/health`, async (route) => {
    const health = state.mode === "connection" ? "unknown" : state.mode === "permission" ? "missing_permission" : "ok";
    await route.fulfill(answer({ severity: "none", pages: [{ page_id: "900007320071", page_name: "Fixture channel",
      status: "active", severity: "none", capabilities: ["facebook", "instagram"].map(provider => ({
        binding_id: fixtureBinding, provider, capability: "dm_session", state: health,
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
      state.original = body.text;
      state.delivery = "queued";
      return route.fulfill(answer({ send_state: "queued", takeover_generation: 2 }));
    }
    if (path.endsWith("/messages")) {
      state.queries++;
      return route.fulfill(answer({ items: [
        { direction: "in", seq: 1, at: row.last_at, text: incoming, attachments: null },
        ...(state.delivery ? [{ direction: "out", at: "2026-10-29T06:35:00Z", text: state.original,
          attachments: null, send_state: state.delivery, principal_id: fixtureAgent }] : []),
      ], has_unknown_outbound: state.delivery === "unknown", mode: "human", takeover_generation: 2,
      human_until: future, window_open_until: state.mode === "closed" ? "2020-01-01T00:00:00Z" : future,
      binding_id: fixtureBinding, link_version: 1 }));
    }
    if (path.endsWith("/buyer-panel")) return route.fulfill(answer({ display_name: row.display_name,
      platform: row.platform, purchase_ordinal: 0, claims: [], claim_total_minor: 0,
      linked_customer_id: null, window_open_until: future, link_pending_manual: false }));
    return route.continue();
  });
  return state;
}

async function enter(page: Page, copy: Copy) {
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
async function query(page: Page) {
  const read = page.waitForResponse(response => response.request().method() === "GET" &&
    new URL(response.url()).pathname.includes("/inbox/conversations/") && new URL(response.url()).pathname.endsWith("/messages"));
  await page.getByTestId("inbox-send-status").click();
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
      await page.getByTestId("inbox-filter-assigned").click();
      await expect(page.getByTestId(`conversation-${ids.retry}`)).toBeHidden();
      await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
      await expect(chips.nth(2)).toHaveAttribute("aria-pressed", "true");
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

  test(`INV2 ${copy.locale} states`, async ({ page }) => {
    const state = await enter(page, copy);
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
