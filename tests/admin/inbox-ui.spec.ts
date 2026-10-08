// Purpose: LC-U2b independent real-click inbox, authority and privacy acceptance.
// Depends on: real Next/BFF/Go/PG fixture, signed MOCK OIDC, native-device.ts; LC_BROWSER_INBOX_* env.
// Used by: browser_inbox_ui_test.go via --browser-inbox on GitHub runners; live Meta remains NOT_RUN.
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { nativePage } from "./fixtures/native-device";
import { required, origin, api, evidence, store, otherStore, ids, sentinels, dm,
  createInboxLedger, writeInboxLedger, selectFixtureStore, login, privateBoundary } from "./inbox-browser-support";
const { ledger, record } = createInboxLedger();
const consoleMessages: string[] = [];


test.use({
  baseURL: origin,
  headless: false,
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});
test.beforeAll(async () => {
  await mkdir(evidence, { recursive: true });
});
test.beforeEach(async ({ page }) => {
  page.on("console", (message) => consoleMessages.push(message.text()));
  page.on("pageerror", (error) => consoleMessages.push(error.message));
});
test.afterEach(async ({}, info) => {
  if (info.status !== info.expectedStatus)
    ledger.push({
      page: "Messages",
      control: info.title,
      action: "scenario",
      expected: "all named assertions pass",
      actual: info.status ?? "UNKNOWN",
      status: "FAIL",
    });
});
test.afterAll(async () => {
  await writeInboxLedger(ledger);
  await writeFile(resolve(evidence, "browser-console.json"), JSON.stringify(consoleMessages), { mode: 0o600 });
});

async function session(context: BrowserContext, token: string) {
  // Synthetic role credentials are fixture setup; admission is checked through real navigation/API.
  await context.addCookies([
    {
      name: "__Host-commerce_session",
      value: token,
      url: origin.replace(/^http:/, "https:"),
      secure: true,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
}

async function open(page: Page, id: string) {
  await page.getByTestId(`conversation-${id}`).click();
  await expect(page.getByTestId("inbox-thread")).toBeVisible();
}

async function facts(page: Page, conversation = "open") {
  // Read-only owner fixture observation; not a substitute for clicking a product control.
  const response = await page.request.get(`${api}/__test/inbox-facts?conversation=${conversation}`);
  expect(response.status()).toBe(200);
  return (await response.json()) as {
    read_seq: number;
    generation: number;
    mode: string;
    outbounds: number;
    reads: number;
    writes: number;
  };
}

async function action(page: Page, id: string, suffix: string, click: () => Promise<unknown>) {
  const pending = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/api/stores/${store}/inbox/conversations/${id}/${suffix}`,
  );
  await click();
  const response = await pending;
  expect(response.status(), `${suffix} real BFF result`).toBeGreaterThanOrEqual(200);
  expect(response.status(), `${suffix} real BFF result`).toBeLessThan(300);
  expect(response.headers()["cache-control"]).toContain("no-store");
  expect(response.request().headers()["idempotency-key"]).toMatch(/^[A-Za-z0-9_-]{8,128}$/);
  return response.json();
}

test("INU01 live_operator actual list/filter/read/takeover/send/release persists in PG", async ({ page }) => {
  await login(page);
  const refreshedList = page.waitForResponse(
    (r) => r.request().method() === "GET" && new URL(r.url()).pathname === `/api/stores/${store}/inbox/conversations`,
  );
  await page.getByTestId("inbox-page").getByRole("button", { name: "Refresh", exact: true }).click();
  expect((await refreshedList).status()).toBe(200);
  await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
  record("list refresh", "click Refresh", "real A8 list read and named conversation restored");
  const before = await facts(page);
  await page.getByTestId("inbox-filter").selectOption("instagram");
  await expect(page.getByTestId(`conversation-${ids.instagram}`)).toBeVisible();
  await expect(page.getByTestId(`conversation-${ids.open}`)).toHaveCount(0);
  record("filter", "select Instagram", "only Instagram conversation visible");
  await page.getByTestId("inbox-filter").selectOption("messenger");
  await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
  await page.getByTestId("inbox-filter").selectOption("messenger");
  await expect(
    page.getByTestId(`conversation-${ids.open}`),
    "same filter must not clear the loaded page",
  ).toBeVisible();
  await expect(page.getByTestId(`conversation-${ids.instagram}`)).toHaveCount(0);
  await page.getByTestId("inbox-filter").selectOption("unreplied");
  await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
  await page.getByTestId("inbox-filter").selectOption("live_comment");
  await expect(page.getByTestId(`conversation-${ids.open}`)).toHaveCount(0);
  await page.getByTestId("inbox-filter").selectOption("all");
  record(
    "filter",
    "select Messenger/unreplied/live comment/all",
    "server filtered rows match platform and reply status",
  );
  const read = page.waitForResponse(
    (r) => r.request().method() === "POST" && r.url().includes(`/conversations/${ids.open}/read`),
  );
  await open(page, ids.open);
  expect((await read).status()).toBe(200);
  await expect(page.getByTestId("inbox-thread")).toContainText(dm);
  await expect(page.getByTestId("buyer-panel")).toBeVisible();
  await expect.poll(async () => (await facts(page)).read_seq).toBeGreaterThan(0);
  record("conversation", "click open thread", "decrypted DM visible and real PG read_seq advanced");
  const refreshedThread = page.waitForResponse(
    (r) =>
      r.request().method() === "GET" &&
      new URL(r.url()).pathname === `/api/stores/${store}/inbox/conversations/${ids.open}/messages`,
  );
  await page.getByTestId("inbox-thread").getByRole("button", { name: "Refresh", exact: true }).click();
  expect((await refreshedThread).status()).toBe(200);
  await expect(page.getByTestId("inbox-thread")).toContainText(dm);
  const older = page.waitForResponse(
    (r) =>
      r.request().method() === "GET" &&
      r.url().includes(`/conversations/${ids.open}/messages`) &&
      new URL(r.url()).searchParams.has("before_seq"),
  );
  await page.getByTestId("inbox-thread").getByRole("button", { name: "Older messages", exact: true }).click();
  expect((await older).status()).toBe(200);
  await expect(
    page.getByTestId("inbox-thread").getByRole("button", { name: "Older messages", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByTestId("inbox-thread")).toContainText(dm);
  record(
    "thread refresh/older",
    "click Refresh then Older messages",
    "real authorized A9 reads; empty older page removes pager without losing thread",
  );
  await action(page, ids.open, "takeover", () => page.getByTestId("takeover").click());
  await expect.poll(async () => (await facts(page)).mode).toBe("human");
  await page.reload();
  await open(page, ids.open);
  await expect(page.getByTestId("release")).toBeEnabled();
  record("takeover", "click then refresh/reopen", "human takeover persisted in real PG");
  await page.getByTestId("reply-text").fill(sentinels[3]);
  await action(page, ids.open, "messages", () => page.getByTestId("reply-send").click());
  await expect(page.getByTestId("inbox-thread")).toContainText(sentinels[3]);
  await expect.poll(async () => (await facts(page)).outbounds).toBe(before.outbounds + 1);
  await page.reload();
  await open(page, ids.open);
  await expect(page.getByTestId("inbox-thread")).toContainText(sentinels[3]);
  record(
    "reply",
    "fill and click send then refresh/reopen",
    "one encrypted outbound appears in authoritative readback",
  );
  await action(page, ids.open, "release", () => page.getByTestId("release").click());
  await expect.poll(async () => (await facts(page)).mode).toBe("auto");
  await page.reload();
  await open(page, ids.open);
  await expect(page.getByTestId("takeover")).toBeEnabled();
  record("release", "click then refresh/reopen", "auto mode persisted in real PG");
  await privateBoundary(page, consoleMessages);
});

async function browserInboxRead(page: Page, path: string) {
  // G-UI8 audit [READ]: same-origin GET uses the already authenticated browser cookie policy.
  // Playwright APIRequestContext omits Secure session cookies on HTTP 127.0.0.1; no cookie/header injection.
  return page.evaluate(async (path) => {
    const response = await fetch(path, { method: "GET", credentials: "same-origin", cache: "no-store" });
    return { status: response.status, text: await response.text() };
  }, path);
}

test("INU02 inbox reader never POSTs; viewer cannot enter; cross-store thread is 404", async ({ page, context }) => {
  await login(page);
  const cross = await browserInboxRead(page, `/api/stores/${store}/inbox/conversations/${ids.foreign}/messages`);
  expect(cross.status).toBe(404);
  expect(cross.text).not.toContain(dm);
  await session(context, required("LC_BROWSER_INBOX_READER_TOKEN"));
  const writes: string[] = [];
  page.on("request", (r) => {
    if (r.method() === "POST" && r.url().includes("/inbox/")) writes.push(r.url());
  });
  const before = await facts(page);
  await page.goto(`/en/messages?store=${store}`);
  await expect(page.getByTestId("inbox-page")).toBeVisible();
  await open(page, ids.open);
  await expect(page.getByTestId("inbox-thread")).toContainText(dm);
  await expect(page.getByTestId("reply-send")).toBeDisabled();
  await page.reload();
  await open(page, ids.open);
  expect(writes).toEqual([]);
  expect((await facts(page)).writes).toBe(before.writes);
  record(
    "reader admission",
    "open and refresh with inbox:read only",
    "reads admitted; no POST/read marker or reply mutation",
  );
  await session(context, required("LC_BROWSER_INBOX_VIEWER_TOKEN"));
  await page.goto(`/en/messages?store=${store}`);
  await expect(page.getByTestId("inbox-thread")).toHaveCount(0);
  await expect(page.getByTestId("nav-group-messages")).toHaveCount(0);
  const denied = await browserInboxRead(page, `/api/stores/${store}/inbox/conversations`);
  expect(denied.status).toBe(403);
  expect(denied.text).not.toContain(dm);
  record("viewer denial", "navigate with catalogue viewer", "inbox nav absent; real backend denies 403 with no DM");
  await privateBoundary(page, consoleMessages);
});

test("INU08 committed send with lost acknowledgment retries exact body/key once", async ({ page }) => {
  await login(page);
  await open(page, ids.retry);
  await expect(page.getByTestId("inbox-thread")).toContainText("SYNTHETIC-INBOX-RETRY-INBOUND");
  const before = await facts(page, "retry");
  const endpoint = `/api/stores/${store}/inbox/conversations/${ids.retry}/messages`;
  const submissions: Array<{ key: string; body: string }> = [];
  let committed: { outbound_id: string; operation_id: string } | undefined;
  let first = true;
  await page.route(`**${endpoint}`, async (route) => {
    if (route.request().method() !== "POST") {
      await route.continue();
      return;
    }
    submissions.push({
      key: route.request().headers()["idempotency-key"],
      body: route.request().postData() ?? "",
    });
    // Fault injection AFTER the real backend commits. No API fixture replaces the user's send click.
    const response = await route.fetch();
    expect(response.status()).toBeGreaterThanOrEqual(200);
    expect(response.status()).toBeLessThan(300);
    const receipt = (await response.json()) as {
      outbound_id: string;
      operation_id: string;
    };
    if (first) {
      first = false;
      committed = receipt;
      await route.abort("failed");
    } else {
      expect(receipt.outbound_id).toBe(committed!.outbound_id);
      expect(receipt.operation_id).toBe(committed!.operation_id);
      await route.fulfill({ response });
    }
  });
  await page.getByTestId("reply-text").fill(sentinels[4]);
  await page.getByTestId("reply-send").click();
  await expect(page.getByTestId("inbox-thread").getByRole("alert")).toHaveText("Messages are unavailable. Try again.");
  await expect(page.getByTestId("reply-send")).toHaveText("Retry this submission");
  await expect(page.getByTestId("reply-text")).toBeDisabled();
  await expect.poll(async () => (await facts(page, "retry")).outbounds).toBe(before.outbounds + 1);
  const second = page.waitForResponse((r) => r.request().method() === "POST" && new URL(r.url()).pathname === endpoint);
  await page.getByTestId("reply-send").click();
  expect((await second).status()).toBeGreaterThanOrEqual(200);
  await expect(page.getByTestId("reply-send")).toHaveText("Send reply");
  expect(submissions).toHaveLength(2);
  expect(submissions[0].key).toMatch(/^[A-Za-z0-9_-]{8,128}$/);
  expect(submissions[1]).toEqual(submissions[0]);
  expect(JSON.parse(submissions[0].body).text).toBe(sentinels[4]);
  expect((await facts(page, "retry")).outbounds).toBe(before.outbounds + 1);
  await page.reload();
  await open(page, ids.retry);
  await expect(page.getByTestId("inbox-thread")).toContainText(sentinels[4]);
  await privateBoundary(page, consoleMessages);
  record(
    "ambiguous acknowledgment",
    "click send; committed response dropped; click Retry this submission",
    "identical key/body, same server operation/outbound id, exactly one PG outbound survives refresh",
  );
});

test("INU03 closed message window and missing capability disable sends with visible reason", async ({ page }) => {
  await login(page);
  await open(page, ids.closed);
  await expect(page.getByTestId("inbox-thread")).toContainText("SYNTHETIC-CLOSED-WINDOW");
  await expect(page.getByTestId("reply-send")).toBeDisabled();
  await expect(page.getByTestId("inbox-thread")).toContainText(/24|window|窗口/);
  record("closed window", "open expired conversation", "send disabled with 24h window reason");
  // Explicit fault injection of a capability read, preserving real admission and real decrypted thread.
  await page.route(`**/api/stores/${store}/meta/health`, async (route) => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    const body = await response.json();
    for (const p of body.pages)
      for (const c of p.capabilities)
        if (c.capability === "dm_session") {
          c.state = "missing_permission";
          c.reason = "perm_pages_messaging";
        }
    await route.fulfill({ response, json: body });
  });
  await page.reload();
  await open(page, ids.open);
  await expect(page.getByTestId("reply-send")).toBeDisabled();
  await expect(page.getByTestId("inbox-thread")).toContainText(/permission|capability|權限/);
  record("missing capability", "reload and open conversation", "send disabled with capability reason");
  await privateBoundary(page, consoleMessages);
});

test("INU04 stale decrypted responses cannot repaint another conversation or store", async ({ page }) => {
  await login(page);
  let release = () => {};
  let seen = () => {};
  let delivered = () => {};
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const intercepted = new Promise<void>((resolve) => {
    seen = resolve;
  });
  const finished = new Promise<void>((resolve) => {
    delivered = resolve;
  });
  await page.route(`**/api/stores/${store}/inbox/conversations/${ids.open}/messages*`, async (route) => {
    const response = await route.fetch();
    seen();
    await held;
    try {
      await route.fulfill({ response });
    } finally {
      // Aborted old reads may reject fulfillment; wake the waiter and retain the original failure.
      delivered();
    }
  });
  try {
    await page.getByTestId(`conversation-${ids.open}`).click();
    await intercepted;
    await open(page, ids.instagram);
    await expect(page.getByTestId("inbox-thread")).toContainText("SYNTHETIC-INSTAGRAM-DM");
    release();
    await finished;
    // G-UI8 audit [READ/MEASURE]: let the completed response's React paint settle before inspecting it.
    await page.evaluate(
      () => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))),
    );
    await page.unroute(`**/api/stores/${store}/inbox/conversations/${ids.open}/messages*`);
    await expect(page.getByTestId("inbox-thread")).not.toContainText(dm);
    let switched = () => {};
    let arrived = () => {};
    let done = () => {};
    const switchHold = new Promise<void>((resolve) => {
      switched = resolve;
      release = resolve;
    });
    const switchSeen = new Promise<void>((resolve) => {
      arrived = resolve;
    });
    const switchDone = new Promise<void>((resolve) => {
      done = resolve;
    });
    await page.route(`**/api/stores/${store}/inbox/conversations/${ids.open}/messages*`, async (route) => {
      const response = await route.fetch();
      arrived();
      await switchHold;
      try {
        await route.fulfill({ response });
      } finally {
        done();
      }
    });
    await page.getByTestId(`conversation-${ids.open}`).click();
    await switchSeen;
    await selectFixtureStore(page, otherStore);
    await expect(page.getByTestId("inbox-thread")).toHaveCount(0);
    switched();
    await switchDone;
    await page.getByTestId("nav-group-messages").click();
    await expect(page.getByTestId(`conversation-${ids.foreign}`)).toBeVisible();
    await expect(page.getByTestId(`conversation-${ids.open}`)).toHaveCount(0);
    await expect(page.getByTestId("inbox-thread")).toHaveCount(0);
    await privateBoundary(page, consoleMessages);
    record(
      "late response/store switch",
      "open pending A then B and switch shell store",
      "old private response never repaints B/other store",
    );
  } finally {
    release();
  }
});

test("INU05 native hide clears private thread; visible return reauthorizes before repaint", async () => {
  test.setTimeout(90_000);
  const { page, close } = await nativePage(evidence, "inbox-native-");
  page.on("console", (message) => consoleMessages.push(message.text()));
  let cover: Page | undefined;
  let release = () => {};
  try {
    await login(page);
    await open(page, ids.open);
    await expect(page.getByTestId("inbox-thread")).toContainText(dm);
    // G-UI8 audit [READ/MEASURE]: records isTrusted visibility; the actual hide is a native tab switch.
    await page.evaluate(() => {
      const w = window as typeof window & {
        inboxVisibility?: { state: string; trusted: boolean }[];
      };
      w.inboxVisibility = [];
      document.addEventListener("visibilitychange", (event) =>
        w.inboxVisibility?.push({
          state: document.visibilityState,
          trusted: event.isTrusted,
        }),
      );
    });
    // Wait for already-authorized reads before measuring the hidden-page no-new-read boundary.
    await expect(page.getByTestId("inbox-thread")).toHaveAttribute("aria-busy", "false");
    await expect(page.getByTestId("buyer-panel")).toHaveAttribute("aria-busy", "false");
    const beforeHide = (await facts(page)).reads;
    cover = await page.context().newPage();
    await cover.goto(`${origin}/en/settings`);
    await cover.bringToFront();
    // G-UI8 audit [READ/MEASURE]: reads native tab visibility.
    await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("hidden");
    // Calibration must fail THIS assertion when the real hidden-state clearing is bypassed.
    await expect(page.getByTestId("inbox-thread"), "INU05 hidden thread retains private DM").toHaveCount(0);
    await expect(page.locator("body")).not.toContainText(dm);
    await expect(page.locator("body")).not.toContainText(sentinels[1]);
    await privateBoundary(page, consoleMessages);
    const whileHidden = (await facts(page)).reads;
    expect(whileHidden).toBe(beforeHide);
    let seen = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    const intercepted = new Promise<void>((resolve) => {
      seen = resolve;
    });
    await page.route(`**/api/stores/${store}/inbox/conversations/${ids.open}/messages*`, async (route) => {
      const response = await route.fetch();
      seen();
      await held;
      await route.fulfill({ response });
    });
    await page.bringToFront();
    // Selection and private bodies were deliberately discarded on hide. Reopen with an actual click.
    await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
    await page.getByTestId(`conversation-${ids.open}`).click();
    await intercepted;
    await expect(page.getByTestId("inbox-thread")).not.toContainText(dm);
    release();
    await expect(page.getByTestId("inbox-thread")).toContainText(dm);
    const afterReturn = (await facts(page)).reads;
    expect(afterReturn).toBeGreaterThan(whileHidden);
    // G-UI8 audit [READ/MEASURE]: reads the trusted event recorder.
    const events = await page.evaluate(
      () => (window as typeof window & { inboxVisibility?: unknown[] }).inboxVisibility,
    );
    expect(events).toEqual([
      { state: "hidden", trusted: true },
      { state: "visible", trusted: true },
    ]);
    await writeFile(
      resolve(evidence, "native-visibility.json"),
      JSON.stringify({
        events,
        beforeHide,
        whileHidden,
        afterReturn,
        piiCleared: true,
        revalidated: true,
      }),
      { mode: 0o600 },
    );
    record(
      "native visibility",
      "activate another native tab then return",
      "trusted hidden clears DM; authorized A9 read required before repaint",
    );
  } finally {
    release();
    await cover?.close().catch(() => {});
    await close();
  }
});

const localeCases = [
  { locale: "zh-TW", title: "訊息", send: "送出回覆", buyer: "買家資料", back: "返回對話列表" },
  { locale: "zh-CN", title: "消息", send: "发送回复", buyer: "买家资料", back: "返回对话列表" },
  { locale: "en", title: "Messages", send: "Send reply", buyer: "Buyer details", back: "Back to conversations" },
] as const;

async function switchLocale(page: Page, locale: (typeof localeCases)[number]["locale"]) {
  const selector = page.getByTestId("locale-switch");
  // Every switch changes the value: same-value navigation can otherwise leave the old privacy scope suspended.
  if ((await selector.inputValue()) === locale) {
    const other = locale === "en" ? localeCases[0] : localeCases[2];
    await selector.selectOption(other.locale);
    await expect(page).toHaveURL(new RegExp(`/${other.locale}/messages`));
    await expect(page.getByTestId("inbox-page").getByRole("heading", { level: 1 })).toHaveText(other.title);
    await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
  }
  await selector.selectOption(locale);
  const copy = localeCases.find((item) => item.locale === locale)!;
  await expect(page).toHaveURL(new RegExp(`/${locale}/messages`));
  await expect(page.getByTestId("inbox-page").getByRole("heading", { level: 1 })).toHaveText(copy.title);
  await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
}

for (const copy of localeCases) {
  test(`INU06 ${copy.locale} desktop1440/mobile390 real controls and no overflow`, async ({ page }) => {
    await login(page);
    await switchLocale(page, copy.locale);
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      if (width === 390) {
        await page.getByRole("button", { name: copy.back, exact: true }).click();
        await expect(page.getByTestId("inbox-list")).toBeVisible();
        await expect(page.getByTestId("inbox-thread")).toHaveCount(0);
        await expect(page.getByTestId("buyer-panel")).toHaveCount(0);
        record(`mobile back ${copy.locale}`, "click back", "private detail removed before reopening the list");
      }
      await page.getByTestId("inbox-filter").selectOption("messenger");
      await open(page, ids.open);
      await expect(page.getByTestId("inbox-thread")).toContainText(dm);
      await expect(page.getByTestId("buyer-panel")).toBeVisible();
      // G-UI8 audit [READ/MEASURE]: geometry after real locale/filter/row clicks.
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: resolve(evidence, `inbox-${copy.locale}-${width}.png`), fullPage: true });
      await privateBoundary(page, consoleMessages);
      record(
        `viewport ${copy.locale}/${width}`,
        "select Messenger/open conversation",
        "thread and BuyerPanel visible without horizontal overflow",
      );
    }
  });

  test(`INU07 ${copy.locale} localized controls at1440/mobile390`, async ({ page }) => {
    await login(page);
    await switchLocale(page, copy.locale);
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      if (width === 390) await page.getByRole("button", { name: copy.back, exact: true }).click();
      await page.getByTestId("inbox-filter").selectOption("messenger");
      await open(page, ids.open);
      await expect(page.getByTestId("inbox-thread")).toContainText(dm);
      // Independent literal expectations catch zh-CN falling back to Traditional Chinese.
      await expect(page.getByTestId("reply-send")).toHaveText(copy.send);
      await expect(page.getByTestId("buyer-panel").getByRole("heading", { level: 2 })).toHaveText(copy.buyer);
      await expect(page.getByTestId("inbox-page").getByRole("heading", { level: 1 })).toHaveText(copy.title);
      await privateBoundary(page, consoleMessages);
      record(
        `locale ${copy.locale}/${width}`,
        "switch locale and open conversation",
        "exact localized heading/composer/buyer labels verified",
      );
    }
  });
}
