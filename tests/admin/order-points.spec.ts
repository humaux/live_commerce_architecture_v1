// Purpose: M7 order-points browser acceptance — LPU03 (award 30 -> order panel -> deduct -30 -> balance 0 -> persisted linked
//   ledger -> award-sized deduct refused), LPU04 (permission-shaped controls and direct API refusal) and LPU07 (MOCK BFF-only
//   not_awarded DTO, all four LP18 reasons). v2 (pre-publication correction 1): ONE suite process and ONE admin Next/Go lifecycle;
//   the permission phases full -> readonly -> noread -> orders-readonly run here, each with a fresh signed context after a fixture-only
//   permission-control call (LC_POINTS_CONTROL, X-Gate-Key; no public API change, no timeout increase, no legacy replay).
//   Authority: contracts/loyalty-points-v1.md LP7/LP17/LP18/LP19, LPU03/LPU04/LPU07 and the frozen selector seam in
//   FLEET-NEXT-1115/M7-ORDER-POINTS/FROZEN-SEAM.md. Not authoritative here: UI copy that the contract does not spell out (only the
//   zh-TW strings 「積分未贈送」/「積分不足」 are asserted literally; other locales assert the frozen data-* attributes).
// Depends on: the production admin Next build started by tests/storefront/manual-order-link-gate.mjs, the REAL Go API and PG that the
//   Go test prepared, the signed MOCK IdP session, and DOM selectors frozen in FROZEN-SEAM.md (manual-member-*, order-points-*).
//   Env: LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_ORDER_STORE (the points store), LC_BROWSER_EVIDENCE, LC_POINTS_CONTROL,
//   LC_POINTS_CONTROL_KEY, LC_POINTS_CANARY.
// Used by: tests/foundation/browser_manual_order_points_test.go through the order-points hook in manual-order-link-gate.mjs and the
//   "order-points" suite in playwright.config.ts.
// Invariants: every placement/deduction below is a real click on the real admin UI and real Go/PG writes; the only MOCK is the LPU07
//   BFF-boundary DTO (documented fixture prep, never a positive replacement). No page.evaluate mutation, no forced actions, no sleeps,
//   no first-match hiding of ambiguity. Search terms and member names never travel in a URL, storage, console or analytics.
import { expect, test, type Browser, type BrowserContext, type Page, type Request } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { browserEvidenceDirectory } from "../browser-evidence.mjs";
import { isWebkitCancelledFetch } from "../storefront/browser-engine.mjs";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_ORDER_STORE");
const control = required("LC_POINTS_CONTROL");
const controlKey = required("LC_POINTS_CONTROL_KEY");
const canary = process.env.LC_POINTS_CANARY ?? "PointsCanary";
const productName = process.env.LC_LINK_PRODUCT_NAME || process.env.LC_POINTS_PRODUCT || "Synthetic manual order product";
const evidence = browserEvidenceDirectory("order-points");

if (!/^[A-Za-z0-9][A-Za-z0-9-]{5,40}$/.test(canary)) throw new Error("LC_POINTS_CANARY must be a short synthetic marker");

// Fixture-only permission control (runner-owned httptest endpoint, X-Gate-Key): the REAL API still checks every permission itself.
async function setLevel(level: "full" | "readonly" | "noread" | "orders-readonly") {
  const reply = await fetch(`${control}/points-permissions`, { method: "POST", headers: { "X-Gate-Key": controlKey, "Content-Type": "application/json" }, body: JSON.stringify({ level }) });
  if (reply.status !== 200) throw new Error(`permission control ${level}: ${reply.status}`);
}

type Locale = "zh-TW" | "zh-CN" | "en";
type Cell = { locale: Locale; device: "desktop" | "mobile"; viewport: { width: number; height: number } };
const cells: Cell[] = (["zh-TW", "zh-CN", "en"] as Locale[]).flatMap((locale) => [
  { locale, device: "desktop" as const, viewport: { width: 1440, height: 900 } },
  { locale, device: "mobile" as const, viewport: { width: 375, height: 812 } },
]);
const createOrderLabel = { "zh-TW": "建立訂單", "zh-CN": "创建订单", en: "Create an order" };
const pointsPath = `/api/stores/${store}/tools/orders/manual`;
const adjustPath = (member: string) => `/api/stores/${store}/members/${member}/adjustments`;
const memberCreatePath = `/api/stores/${store}/members`;
const searchPath = `/api/stores/${store}/members/search`;
const entryPath = `/api/stores/${store}/members/entries`;
const recoveryPath = (order: string) => `/api/stores/${store}/orders/${order}/member-award-recovery`;
const abs = (path: string) => `${origin}${path}`;
const randomKey = () => `points-${crypto.randomUUID()}`;
const uuid = () => crypto.randomUUID();

type Award = { member_id: string; points: number; state: string; entry_id?: string; reason?: string };
type Placed = { order_id: string; commercial_state: string; payment_mode: string; member_award?: Award };
type Adjust = { entry: { id: string; delta: number; order_id: string | null; note: string | null; reason: string }; balance: number };

/** Evidence row written by each REAL journey; the Go test uses only ids/numbers (never a display name or note). */
const realOrders: Array<Record<string, unknown>> = [];
const realMembers: Array<Record<string, unknown>> = [];
const mockAwards: Array<Record<string, unknown>> = [];
const permissionEvidence: Record<string, unknown> = {};
let cases = 0;

/** One signed-in session shared by every cell: the mock IdP maps its subject to the same fixture principal. */
let session: Awaited<ReturnType<BrowserContext["storageState"]>> | undefined;

async function signIn(page: Page) {
  await page.goto(abs(`/en`));
  const dashboard = page.getByTestId("dashboard-page");
  if (!(await dashboard.isVisible().catch(() => false))) {
    await page.getByRole("button", { name: "Sign in with identity service", exact: true }).click();
  }
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
}

async function openContext(browser: Browser, cell: Pick<Cell, "locale" | "viewport">, state = session): Promise<BrowserContext> {
  return browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
    storageState: state,
    viewport: cell.viewport,
    timezoneId: "Asia/Taipei",
  });
}

function watch(page: Page) {
  const errors: Array<{ name: string; message: string }> = [];
  const consoleText: string[] = [];
  page.on("pageerror", (error) => errors.push({ name: error.name, message: error.message }));
  page.on("console", (message) => consoleText.push(message.text()));
  return {
    errors,
    consoleText,
    assertClean(terms: string[]) {
      const real = errors.filter((error) => !isWebkitCancelledFetch(error));
      expect(real, "uncaught page errors").toEqual([]);
      expect(page.url(), "the address bar carries no member name or search term").not.toContain(canary);
      for (const term of terms) expect(page.url()).not.toContain(term);
      expect(consoleText.join("\n"), "the console carries no member name or search term").not.toContain(canary);
    },
  };
}

async function storedPrivateTerms(page: Page): Promise<string> {
  return page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
}

async function openManualOrder(page: Page, locale: Locale) {
  await page.goto(abs(`/${locale}/orders/new?store=${store}`));
  await expect(page.getByTestId("manual-order-page")).toBeVisible();
  await expect(page.getByTestId("manual-order-form")).toBeVisible();
}

async function addProduct(page: Page) {
  await page.getByTestId("mo-open-picker").click();
  await page.getByTestId("mo-search").fill(productName);
  await page.getByTestId("mo-search-button").click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog.locator(".mt-variants")).toBeVisible();
  // The product name is unique in this fixture, so its single variant list has exactly one sellable button.
  await dialog.locator(".mt-variants li button").first().click();
  await page.getByTestId("mo-apply-picker").click();
  await expect(page.getByTestId("mo-lines").locator("li")).toHaveCount(1);
}

/** Picks the home-delivery row by its visible behaviour (address fields), never by copy or by positional first-match. */
async function chooseHomeDelivery(page: Page) {
  const select = page.getByTestId("mo-option");
  const values = await select.locator("option").evaluateAll((nodes) =>
    nodes.map((node) => (node as HTMLOptionElement).value).filter((value) => value !== ""),
  );
  for (const value of values) {
    await select.selectOption(value);
    if (await page.getByTestId("mo-city").isVisible().catch(() => false)) return value;
  }
  throw new Error("the manual options carry no home-delivery row");
}

async function fillRecipient(page: Page, name: string, phone: string) {
  await page.getByTestId("mo-name").fill(name);
  await page.getByTestId("mo-phone").fill(phone);
  await page.getByTestId("mo-city").fill("Taipei");
  await page.getByTestId("mo-line1").fill("1 Order Points Road");
  await expect(page.getByTestId("mo-mode-bank_transfer")).toBeVisible();
  await page.getByTestId("mo-mode-bank_transfer").check();
}

/** Real two-click placement: the "Review order" submit, then manual-order-submit. Returns the real request body and answer. */
async function reviewAndPlace(page: Page, checkReview?: () => Promise<void>) {
  await page.getByTestId("manual-order-form").locator('button[type="submit"]').click();
  await expect(page.getByTestId("manual-order-submit")).toBeVisible();
  await checkReview?.();
  const placed = page.waitForResponse(
    (reply) => reply.request().method() === "POST" && new URL(reply.url()).pathname === pointsPath,
  );
  const request = page.waitForRequest(
    (sent) => sent.method() === "POST" && new URL(sent.url()).pathname === pointsPath,
  );
  const [reply, sent] = await Promise.all([placed, request, page.getByTestId("manual-order-submit").click()]);
  const body = JSON.parse(sent.postData() ?? "{}") as Record<string, unknown>;
  return { status: reply.status(), answer: (await reply.json()) as Placed, body };
}

/** Creates the member through the inline manual-order group (real clicks) and returns the server's member id. */
async function createMember(page: Page, name: string, contact: string, kind: string) {
  await expect(page.getByTestId("manual-member-fields")).toBeVisible();
  await page.getByTestId("manual-member-create-open").click();
  await page.getByTestId("manual-member-create-name").fill(name);
  await page.getByTestId("manual-member-create-contact").fill(contact);
  let createPosts = 0;
  const countCreate = (request: Request) => {
    if (request.method() === "POST" && new URL(request.url()).pathname === memberCreatePath) createPosts++;
  };
  page.on("request", countCreate);
  try {
    await page.getByTestId("manual-member-create-submit").click();
    await expect(page.getByTestId("manual-member-create-name")).toBeDisabled();
    await expect(page.getByTestId("manual-member-create-submit")).toBeEnabled();
    expect(createPosts, "local member review sends no create POST").toBe(0);
    const created = page.waitForResponse(
      (reply) => reply.request().method() === "POST" && new URL(reply.url()).pathname === memberCreatePath,
    );
    await page.getByTestId("manual-member-create-submit").click();
    const reply = await created;
    expect(reply.status(), "member create").toBe(201);
    const answer = (await reply.json()) as { member: { id: string; display_name: string; balance: number } };
    expect(answer.member.display_name).toBe(name);
    expect(answer.member.balance).toBe(0); // the member starts at the authoritative balance 0
    await expect(page.getByTestId("manual-member-selected")).toBeVisible();
    realMembers.push({ member_id: answer.member.id, kind, display_name_length: Array.from(name).length });
    expect(createPosts, "confirmed member creation sends exactly one POST").toBe(1);
    return { memberId: answer.member.id, memberName: name };
  } finally {
    page.off("request", countCreate);
  }
}

/** Real "change member" -> search -> pick flow. Asserts the search travels in the POST body only (LPU06/HTTP2). */
async function pickMemberBySearch(page: Page, name: string, memberId: string) {
  const searches: Array<{ url: string; body: string }> = [];
  page.on("request", (request) => {
    if (request.method() === "POST" && new URL(request.url()).pathname === searchPath) {
      searches.push({ url: request.url(), body: request.postData() ?? "" });
    }
  });
  // A selected member offers "change"; a fresh form already starts in the query state (documented seam difference).
  if (await page.getByTestId("manual-member-change").isVisible().catch(() => false)) await page.getByTestId("manual-member-change").click();
  const intended = page.waitForResponse(reply => reply.request().method() === "POST"
    && new URL(reply.url()).pathname === searchPath && reply.request().postDataJSON().q === name);
  await page.getByTestId("manual-member-query").fill(name);
  const reply = await intended;
  expect(reply.status(), "typed member search").toBe(200);
  const results = page.getByTestId("manual-member-results");
  const option = results.locator(`[data-member-id="${memberId}"]`);
  await expect(option).toBeVisible();
  await option.click();
  await expect(page.getByTestId("manual-member-selected")).toContainText(name);
  expect(searches.length, "the member search is a POST").toBeGreaterThan(0);
  for (const search of searches) expect(new URL(search.url).search, "the search term never travels in a URL").toBe("");
  expect(reply.request().postDataJSON().q, "the intended search carries the typed term in the body").toBe(name);
}

async function openOrderPanel(page: Page, locale: Locale, orderId: string, role: RegExp | string = "Ready") {
  await page.goto(abs(`/${locale}/orders/${orderId}?store=${store}`));
  const panel = page.getByTestId("order-points-panel");
  await expect(panel).toBeVisible();
  await expect(panel).toHaveAttribute("data-order-points-role", role);
  return panel;
}

/** Opens the LP19 deduct dialog and asserts the frozen prefilled amount comes from the order's award. */
async function openDeduct(page: Page) {
  await page.getByTestId("order-points-deduct-open").click();
  const dialog = page.getByTestId("order-points-deduct-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAttribute("data-order-points-role", "Confirm");
  const amount = page.getByTestId("order-points-deduct-amount");
  await expect(amount).toHaveValue("30");
  return { dialog, amount, note: page.getByTestId("order-points-deduct-note") };
}

async function submitDeduct(page: Page, memberId: string) {
  const request = page.waitForRequest(
    (sent) => sent.method() === "POST" && new URL(sent.url()).pathname === adjustPath(memberId),
  );
  const response = page.waitForResponse(
    (reply) => reply.request().method() === "POST" && new URL(reply.url()).pathname === adjustPath(memberId),
  );
  const [reply, sent] = await Promise.all([response, request, page.getByTestId("order-points-deduct-submit").click()]);
  return { status: reply.status(), answer: reply.status() < 300 ? ((await reply.json()) as Adjust) : undefined, body: JSON.parse(sent.postData() ?? "{}") };
}

// Only the zh-TW wording is spelled out by the contract (LPU02/LPU07); en/zh-CN copy is not frozen, so those locales assert the
// frozen data-award-state/data-award-reason and data-order-points-role attributes instead of an invented translation.
const notAwardedHant = "積分未贈送";
const insufficientHant = "積分不足";

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ browser }) => {
  console.info("LPU_PHASE full.before");
  await setLevel("full"); // phase 1 of the one suite process
  console.info("LPU_PHASE full.after");
  console.info("LPU_PHASE freshSession.before");
  const state = await freshSession(browser);
  console.info("LPU_PHASE freshSession.after");
  session = state;
});

/** A fresh signed context after a permission change: the session must be issued under the new grants. */
async function freshSession(browser: Browser) {
  const context = await browser.newContext({ baseURL: origin, ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  console.info("LPU_PHASE signIn.before");
  await signIn(page);
  console.info("LPU_PHASE signIn.after");
  const state = await context.storageState();
  await context.close();
  return state;
}

test.afterAll(async () => {
  await writeFile(
    resolve(evidence, "result.json"),
    JSON.stringify(
      {
        phases: ["full", "readonly", "noread", "orders-readonly"],
        cases,
        real_orders: realOrders,
        real_members: realMembers,
        mock_awards: mockAwards,
        permission: permissionEvidence,
        evidence: { label: "MOCK IdP session; REAL_PG for every listed order/member; LPU07 DTOs are MOCK BFF boundary only" },
      },
      null,
      2,
    ),
    { mode: 0o600 },
  );
});

const sameNameMembers = new Map<Locale, string>();
for (const cell of cells) {
    test(`LPU03 award 30 -> order panel +30 -> deduct -30 -> balance 0 -> persisted ledger -> award-sized deduct refused (${cell.locale}/${cell.device})`, async ({ browser }) => {
      test.setTimeout(240_000);
      console.info("LPU_PHASE context.before", cell.locale, cell.device);
      const context = await openContext(browser, cell);
      console.info("LPU_PHASE context.after", cell.locale, cell.device);
      const page = await context.newPage();
      const watched = watch(page);
      const memberName = `Synthetic member ${canary} ${cell.locale}`;
      const orderName = `Synthetic customer ${canary} ${cell.locale} ${cell.device}`; // merchant-edited, independent of the member (LP17)
      const phone = "0912000000";
      try {
        console.info("LPU_PHASE manual.before", cell.locale, cell.device);
        let placementRequests = 0;
        page.on("request", request => {
          if (request.method() === "POST" && new URL(request.url()).pathname === pointsPath) placementRequests++;
        });
        await page.goto(abs(`/${cell.locale}/orders?store=${store}&state=unshipped`));
        await page.getByTestId("orders-search").fill("0001");
        const searched = page.waitForResponse(reply => reply.request().method() === "POST" &&
          new URL(reply.url()).pathname === `/api/stores/${store}/orders/search`);
        await page.getByTestId("orders-apply").click();
        expect((await searched).status()).toBe(200);
        const heading = page.getByTestId("merchant-orders").locator("header.signed-orders-heading");
        await expect(heading.locator(`a[href="/${cell.locale}/orders/shipping?store=${store}"]`)).toBeVisible();
        await page.screenshot({ path: resolve(evidence, `m11-list-${cell.locale}-${cell.device}.png`) });
        const create = heading.getByRole("link", { name: createOrderLabel[cell.locale], exact: true });
        await expect(create, "M11 order list exposes its create-order link").toBeVisible();
        await create.click();
        await expect(page).toHaveURL(abs(`/${cell.locale}/orders/new?store=${store}`));
        await expect(page.getByTestId("manual-order-form")).toBeVisible();
        await expect(page.getByTestId("store-selector")).toHaveValue(store);
        await expect(page.getByTestId("mo-option")).toBeEnabled();
        await expect(page.getByTestId("mo-name")).toHaveValue("");
        await expect(page.getByTestId("mo-phone")).toHaveValue("");
        await expect(page.getByTestId("mo-lines")).toHaveCount(0);
        await expect(page.getByTestId("manual-member-selected")).toHaveCount(0);
        await expect(page.getByTestId("manual-order-return-conversation")).toHaveCount(0);
        expect(placementRequests, "opening the form does not place an order").toBe(0);
        await page.screenshot({ path: resolve(evidence, `m11-blank-${cell.locale}-${cell.device}.png`) });
        console.info("LPU_PHASE manual.after", cell.locale, cell.device);
        console.info("LPU_PHASE product.before", cell.locale, cell.device);
        await addProduct(page);
        console.info("LPU_PHASE product.after", cell.locale, cell.device);
        const optionKey = await chooseHomeDelivery(page);
        if (cell.device === "mobile") await fillRecipient(page, orderName, phone);
        console.info("LPU_PHASE create.before", cell.locale, cell.device);
        const contact = `line-${canary}-${cell.locale}-${cell.device}`;
        const { memberId } = await createMember(page, memberName, contact, "awarded");
        console.info("LPU_PHASE create.after", cell.locale, cell.device);
        const selected = page.getByTestId("manual-member-selected");
        const points = page.getByTestId("manual-member-points");
        if (cell.device === "desktop") {
          sameNameMembers.set(cell.locale, memberId);
          await expect(page.getByTestId("mo-name"), "LP17 prefills a blank recipient").toHaveValue(memberName);
          await expect(page.getByTestId("mo-name")).toBeEditable();
          await fillRecipient(page, orderName, phone);
          await pickMemberBySearch(page, memberName, memberId);
        } else {
          await expect(page.getByTestId("mo-name"), "create and select preserves the recipient").toHaveValue(orderName);
          const otherId = sameNameMembers.get(cell.locale)!;
          expect(memberId).not.toBe(otherId);
          await points.fill("47");
          await page.getByTestId("manual-member-change").click();
          await expect(points, "changing member clears the former draft points").toHaveValue("");
          await pickMemberBySearch(page, memberName, otherId);
          await expect(selected).toContainText(`line-${canary}-${cell.locale}-desktop`);
          await expect(points).toHaveValue("");
          await expect(page.getByTestId("mo-name"), "selecting a different ID preserves the recipient").toHaveValue(orderName);
          await points.fill("53");
          await page.getByTestId("manual-member-remove").click();
          await expect(points, "removing member clears the former draft points").toHaveValue("");
          await pickMemberBySearch(page, memberName, memberId);
        }
        await expect(selected).toContainText(contact);
        await expect(page.getByTestId("mo-name"), "selecting a search result preserves the recipient").toHaveValue(orderName);
        await expect(page.getByTestId("mo-phone")).toHaveValue(phone);
        await expect(page.getByTestId("mo-option")).toHaveValue(optionKey);
        await expect(page.getByTestId("mo-city")).toHaveValue("Taipei");
        await expect(page.getByTestId("mo-line1")).toHaveValue("1 Order Points Road");
        await points.fill("30");

        console.info("LPU_PHASE place.before", cell.locale, cell.device);
        const placed = await reviewAndPlace(page, async () => {
          const recipient = page.locator(".ops-review > dl");
          await expect(recipient.getByText(orderName, { exact: true })).toBeVisible();
          await expect(recipient.getByText(phone, { exact: true })).toBeVisible();
          await expect(recipient.getByText("Taipei 1 Order Points Road", { exact: true })).toBeVisible();
          const awardReview = page.getByTestId("manual-member-award-review");
          await expect(awardReview.getByText(memberName, { exact: true })).toBeVisible();
          await expect(awardReview).toContainText("+30");
          await page.screenshot({ path: resolve(evidence, `lpu03-review-${cell.locale}-${cell.device}.png`), fullPage: true });
        });
        console.info("LPU_PHASE place.after", cell.locale, cell.device);
        expect(placed.status, "manual order placement").toBe(201);
        expect(placementRequests).toBe(1);
        expect(placed.body).not.toHaveProperty("source_context");
        expect(placed.body.customer).toEqual({ name: orderName, phone, email: "" });
        expect(placed.body.delivery).toEqual({
          option_key: optionKey,
          home_address: { region: "", city: "Taipei", postal_code: "", line1: "1 Order Points Road", line2: "" },
          cvs: null,
        });
        expect(placed.body.member_award).toEqual({ member_id: memberId, points: 30 });
        const award = placed.answer.member_award;
        expect(award).toBeDefined();
        expect(award!.state).toBe("awarded");
        expect(award!.points).toBe(30);
        expect(award!.member_id).toBe(memberId);
        expect(award!.entry_id, "an awarded order carries its ledger entry id").toMatch(/^[0-9a-f-]{36}$/);
        await writeFile(resolve(evidence, `lpu03-submit-${cell.locale}-${cell.device}.json`),
          JSON.stringify(placed, null, 2), { mode: 0o600 });
        const orderId = placed.answer.order_id;
        const result = page.getByTestId("manual-order-result");
        await expect(result).toContainText(orderName);
        const awardResult = result.getByTestId("manual-member-award-result");
        await expect(awardResult).toHaveAttribute("data-award-state", "awarded");
        await expect(awardResult).toContainText(memberName);
        await page.screenshot({ path: resolve(evidence, `m11-receipt-${cell.locale}-${cell.device}.png`), fullPage: true });

        // The order page reads the ledger entries of this order: 本單積分 shows the +30 award.
        console.info("LPU_PHASE orderPanel.before", cell.locale, cell.device);
        const panel = await openOrderPanel(page, cell.locale, orderId);
        console.info("LPU_PHASE orderPanel.after", cell.locale, cell.device);
        await expect(panel).toContainText("30");
        await expect(page.getByTestId("order-points-deduct-open")).toBeVisible();

        // LP19: the dialog is prefilled with this order's award, and the note is mandatory.
        console.info("LPU_PHASE deductOpen.before", cell.locale, cell.device);
        const deduct = await openDeduct(page);
        console.info("LPU_PHASE deductOpen.after", cell.locale, cell.device);
        let adjustWrites = 0;
        page.on("request", (request) => {
          if (request.method() === "POST" && new URL(request.url()).pathname === adjustPath(memberId)) adjustWrites++;
        });
        await page.getByTestId("order-points-deduct-submit").click();
        await expect(deduct.dialog).toHaveAttribute("data-order-points-role", "EmptyNote");
        expect(adjustWrites, "no adjust is sent with an empty note").toBe(0);
        await expect(deduct.amount).toHaveValue("30");

        await deduct.note.fill(`Manual deduct ${canary} ${orderId}`);
        console.info("LPU_PHASE deductSubmit.before", cell.locale, cell.device);
        const adjustment = await submitDeduct(page, memberId);
        console.info("LPU_PHASE deductSubmit.after", cell.locale, cell.device);
        expect(adjustment.status, "deduct").toBe(201);
        expect(adjustment.body).toEqual({ delta: -30, order_id: orderId, note: `Manual deduct ${canary} ${orderId}` });
        expect(adjustment.answer!.entry.delta).toBe(-30);
        expect(adjustment.answer!.entry.reason).toBe("manual_adjust");
        expect(adjustment.answer!.entry.order_id).toBe(orderId);
        expect(adjustment.answer!.balance).toBe(0); // the authoritative balance after the deduct
        await expect(page.getByTestId("order-points-deduct-dialog")).toHaveCount(0);

        // Persisted linked ledger: a reload re-reads the same two entries of this order.
        await page.reload();
        await expect(page.getByTestId("order-points-panel")).toHaveAttribute("data-order-points-role", "Ready");
        await expect(page.getByTestId("order-points-panel")).toContainText("-30");
        const settled = await page.getByTestId("order-points-panel").innerText();

        // A second, award-sized deduct is refused with no business change: the amount stays editable, the balance stays 0.
        const second = await openDeduct(page);
        await second.amount.fill("31");
        await expect(second.amount).toHaveValue("31");
        await second.amount.fill("30");
        await second.note.fill(`Second deduct ${canary} ${orderId}`);
        console.info("LPU_PHASE deductRefused.before", cell.locale, cell.device);
        const refused = await submitDeduct(page, memberId);
        console.info("LPU_PHASE deductRefused.after", cell.locale, cell.device);
        expect(refused.status, "an award-sized deduct beyond the balance is refused").toBe(409);
        await expect(second.dialog).toHaveAttribute("data-order-points-role", "Insufficient");
        if (cell.locale === "zh-TW") await expect(second.dialog).toContainText(insufficientHant);
        await page.reload();
        await expect(page.getByTestId("order-points-panel")).toHaveAttribute("data-order-points-role", "Ready");
        expect(await page.getByTestId("order-points-panel").innerText(), "the refusal changes no entry and no balance").toBe(settled);

        // The private search term and member name live in the POST body only.
        expect(await storedPrivateTerms(page)).not.toContain(canary);
        watched.assertClean([memberName, `Manual deduct`]);

        realOrders.push({
          order_id: orderId,
          member_id: memberId,
          award_entry_id: award!.entry_id,
          deduction_entry_id: adjustment.answer!.entry.id,
          balance_after: adjustment.answer!.balance,
          customer_name: orderName,
          locale: cell.locale,
          device: cell.device,
          viewport: cell.viewport,
          entry: "orders_list",
        });
        cases++;
        await page.screenshot({ path: resolve(evidence, `lpu03-${cell.locale}-${cell.device}.png`), fullPage: true });
      } finally {
        await context.close();
      }
    });
  }

test("LPU03 empty award field omits member_award entirely; typed decimal/0/>1000000 never place an order", async ({ browser }) => {
    test.setTimeout(180_000);
    const context = await openContext(browser, { locale: "en", viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    const watched = watch(page);
    const placements: Array<Record<string, unknown>> = [];
    page.on("request", (request) => {
      if (request.method() === "POST" && new URL(request.url()).pathname === pointsPath) {
        placements.push(JSON.parse(request.postData() ?? "{}") as Record<string, unknown>);
      }
    });
    try {
      await openManualOrder(page, "en");
      await addProduct(page);
      await createMember(page, `Synthetic member ${canary} empty`, `line-${canary}-empty`, "no_award");
      await page.getByTestId("manual-member-points").fill("");
      await expect(page.getByTestId("manual-member-points")).toHaveValue("");
      await chooseHomeDelivery(page);
      await fillRecipient(page, `Synthetic customer ${canary} empty`, "0912000001");
      for (const invalid of ["3.5", "0", "1000001"]) {
        await page.getByTestId("manual-member-points").fill(invalid);
        // Implicit form submission from the field: it never opens the review and never writes (a disabled submit is also fine).
        await page.getByTestId("manual-member-points").press("Enter");
        await expect(page.getByTestId("manual-member-error"), `invalid points ${invalid} are refused in the form`).toBeVisible();
        await expect(page.getByTestId("manual-order-submit"), `invalid points ${invalid} never reach review`).toHaveCount(0);
      }
      expect(placements, "no placement while the typed points are invalid").toHaveLength(0);
      await page.getByTestId("manual-member-points").fill("");
      await expect(page.getByTestId("manual-member-error")).toBeHidden();
      const placed = await reviewAndPlace(page);
      expect(placed.status).toBe(201);
      expect("member_award" in placed.body, "an empty award field omits the key entirely").toBe(false);
      expect("member_award" in placements.at(-1)!).toBe(false);
      expect(placements).toHaveLength(1);
      await expect(page.getByTestId("manual-order-result")).toBeVisible();
      await expect(page.getByTestId("manual-member-award-result")).toHaveCount(0);
      watched.assertClean([`Synthetic customer ${canary} empty`]);
      realOrders.push({ order_id: placed.answer.order_id, member_id: null, member_award: null, locale: "en", device: "desktop", customer_name: `Synthetic customer ${canary} empty` });
      cases++;
    } finally {
      await context.close();
    }
  });

for (const device of ["desktop", "mobile"] as const) {
  test(`M10 original award recovery uses the real receipt and ledger (${device})`, async ({ browser }) => {
    test.setTimeout(180_000);
    const locale = "zh-TW";
    const context = await openContext(browser, { locale, viewport: device === "desktop" ? { width: 1440, height: 900 } : { width: 375, height: 812 } });
    const page = await context.newPage();
    const watched = watch(page);
    const recipient = `Synthetic customer ${canary} recovery ${device}`;
    const conversation = device === "mobile" ? required("LC_POINTS_CONVERSATION") : "";
    let placed: Placed | undefined, placementKey = "";
    let credit: { order_id: string; member_id: string; points: number; entry_id: string; state: string } | undefined;
    const writes: Array<{path: string; key?: string; body?: unknown}> = [];
    page.on("request", request => {
      const path = new URL(request.url()).pathname;
      if (request.method() === "POST" && (path === pointsPath || path.endsWith("/member-award-recovery") || path.endsWith("/adjustments")))
        writes.push({path,key:request.headers()["idempotency-key"],body:request.postDataJSON()});
    });
    try {
      if (conversation) {
        await page.goto(abs(`/${locale}/messages?store=${store}`));
        await page.getByTestId(`conversation-${conversation}`).click();
        await page.getByTestId("inbox-manual-order").click();
      } else {
        await page.goto(abs(`/${locale}?store=${store}`));
        await page.getByTestId("action-create-order").click();
      }
      const memberName = `Synthetic member ${canary} recovery ${device}`;
      const {memberId} = await createMember(page, memberName, `line-${canary}-recovery-${device}`, "recovered");
      if (!conversation) {
        await page.getByTestId("manual-order-form").locator(`a[href="/${locale}/orders?store=${store}"]`).click();
        await expect(page.getByTestId("manual-order-form")).toHaveCount(0);
        expect(writes).toHaveLength(0);
        await page.goto(abs(`/${locale}?store=${store}`));
        await page.getByTestId("action-create-order").click();
        await pickMemberBySearch(page, memberName, memberId);
      }
      await addProduct(page);
      await chooseHomeDelivery(page);
      await fillRecipient(page, recipient, "0912000003");
      await page.getByTestId("manual-member-points").fill("30");
      const arm = await fetch(`${control}/points-miss-award`, {method:"POST",headers:{"X-Gate-Key":controlKey}});
      expect(arm.status).toBe(204);
      if (device === "desktop") {
        await page.route(abs(pointsPath), async route => {
          const reply = await route.fetch({maxRetries:0});
          expect(reply.status()).toBe(201);
          placed = await reply.json();
          placementKey = route.request().headers()["idempotency-key"];
          await route.abort();
        }, {times:1});
        await page.getByTestId("manual-order-form").locator('button[type="submit"]').click();
        await page.getByTestId("manual-order-submit").click();
        await expect(page.getByTestId("manual-order-check-original")).toBeVisible();
        const stored = await storedPrivateTerms(page);
        for (const value of [recipient, "0912000003", "1 Order Points Road"]) expect(stored).not.toContain(value);
        await page.reload();
        const readback = page.waitForResponse(reply => new URL(reply.url()).pathname === pointsPath + "/receipt");
        await page.getByTestId("manual-order-check-original").click();
        const reply = await readback;
        expect(reply.request().headers()["idempotency-key"]).toBe(placementKey);
        const saved = await reply.json();
        expect(saved.state).toBe("committed");
        expect(saved.receipt).toMatchObject({order_id:placed?.order_id,payment_mode:"bank_transfer",total_minor:1250});
        expect(saved.receipt.member_award).toEqual(placed?.member_award);
      } else {
        const reply = await reviewAndPlace(page);
        expect(reply.status).toBe(201);
        expect(reply.body.source_context).toMatchObject({conversation_id:conversation,bundle_ids:[]});
        const back = page.getByTestId("manual-order-return-conversation");
        await expect(back).toHaveAttribute("href", `/${locale}/messages?store=${store}&conversation_id=${conversation}`);
        placed = reply.answer;
        placementKey = writes[0].key!;
      }
      expect(placed?.member_award).toMatchObject({member_id:memberId,points:30,state:"not_awarded",reason:"forbidden"});
      const order = placed!.order_id;
      const path = recoveryPath(order);
      const ledger = (reply: import("@playwright/test").Response) => new URL(reply.url()).pathname === entryPath && new URL(reply.url()).searchParams.get("order_id") === order;
      await expect(page.getByTestId("manual-member-award-result")).toHaveAttribute("data-award-state","not_awarded");
      await page.screenshot({path:resolve(evidence,`m10-missed-${device}.png`),fullPage:true});
      const result = page.getByTestId("manual-order-result");
      await expect(result.getByRole("link", {name:"查看原單",exact:true})).toHaveAttribute("href", `/${locale}/orders/${order}?store=${store}`);
      await expect(result).toContainText(order.slice(0, 8));
      await expect(result).toContainText("NT$12.50");
      if (device === "desktop") {
        await expect(result.getByText("收件人",{exact:true})).toHaveCount(0);
        await expect(result.getByText("聯絡電話",{exact:true})).toHaveCount(0);
        await expect(result.getByText("配送方式",{exact:true})).toHaveCount(0);
        await expect(result.getByText("取貨門市",{exact:true})).toHaveCount(0);
        await expect(result.getByText("請選擇…",{exact:true})).toHaveCount(0);
        await expect(result).not.toContainText("· ·");
        await expect(result.locator("ul")).toHaveCount(0);
      } else {
        for (const value of [recipient, "0912000003", "Taipei 1 Order Points Road"]) {
          await expect(result.getByText(value,{exact:true})).toBeVisible();
        }
      }
      await expect(result.getByText("銀行轉帳（顧客須在期限內轉帳）",{exact:true})).toBeVisible();
      await setLevel("full");
      const check = page.waitForResponse(reply => new URL(reply.url()).pathname === path && reply.request().method() === "GET");
      const before = page.waitForResponse(ledger);
      await page.getByTestId("manual-award-check-original").click();
      expect(await (await check).json()).toMatchObject({order_id:order,member_id:memberId,points:30,state:"eligible"});
      expect((await (await before).json()).entries).toEqual([]);
      await page.getByTestId("order-award-recovery-open").click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText(memberName);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);
      expect(writes).toHaveLength(1);
      await page.screenshot({path:resolve(evidence,`m10-confirm-${device}.png`)});
      if (device === "mobile") {
        const {left,right,top,bottom} = await dialog.evaluate(element => {
          const rect = element.getBoundingClientRect();
          return {left:rect.left,right:window.innerWidth-rect.right,top:rect.top,bottom:window.innerHeight-rect.bottom};
        });
        expect(left).toBeGreaterThanOrEqual(15);
        expect(Math.abs(left-right)).toBeLessThan(2);
        expect(top).toBeGreaterThan(0);
        expect(Math.abs(top-bottom)).toBeLessThan(2);
        const cancel = await dialog.getByRole("button",{name:"取消",exact:true}).boundingBox();
        const confirm = await dialog.getByTestId("order-award-recovery-submit").boundingBox();
        expect(cancel && confirm && Math.abs(cancel.y - confirm.y)).toBeLessThan(2);
        expect(cancel && confirm && Math.abs(cancel.width - confirm.width)).toBeLessThan(2);
        expect(cancel && confirm && cancel.x + cancel.width <= confirm.x).toBe(true);
      }
      let checked;
      if (device === "mobile") {
        await page.route(abs(path), async route => {
          const reply = await route.fetch({maxRetries:0});
          expect(reply.status()).toBe(201);
          credit = await reply.json();
          await route.abort();
        }, {times:1});
        await page.getByTestId("order-award-recovery-submit").click();
        await expect(page.getByTestId("order-award-recovery-check")).toBeVisible();
        checked = page.waitForResponse(ledger);
        await page.getByTestId("order-award-recovery-check").click();
      } else {
        const committed = page.waitForResponse(reply => new URL(reply.url()).pathname === path && reply.request().method() === "POST");
        checked = page.waitForResponse(ledger);
        await page.getByTestId("order-award-recovery-submit").click();
        const reply = await committed;
        expect(reply.status()).toBe(201);
        credit = await reply.json();
      }
      expect(credit).toMatchObject({order_id:order,member_id:memberId,points:30,state:"awarded"});
      const after = (await (await checked).json()).entries;
      expect(after).toEqual([expect.objectContaining({id:credit!.entry_id,member_id:memberId,delta:30,reason:"manual_order_award"})]);
      await expect(page.getByTestId("order-award-recorded")).toContainText("+30");
      await expect(page.getByTestId("order-award-recovery-open")).toHaveCount(0);
      await page.screenshot({path:resolve(evidence,`m10-recovered-${device}.png`),fullPage:true});
      expect(writes).toEqual([
        {path:pointsPath,key:placementKey,body:expect.objectContaining({member_award:{member_id:memberId,points:30}})},
        {path,key:expect.any(String),body:{}},
      ]);
      watched.assertClean([recipient]);
      realOrders.push({order_id:order,member_id:memberId,customer_name:recipient,recovery:true,award_entry_id:credit!.entry_id,
        placement_key:placementKey,recovery_key:writes[1].key,conversation_id:conversation,receipt:placed?.member_award,recovery_receipt:credit,
        ledger:after.map(({id,member_id,delta,reason}: {id:string;member_id:string;delta:number;reason:string})=>({id,member_id,delta,reason}))});
      cases++;
    } finally {
      await context.close();
    }
  });
}

// LPU07 is explicitly MOCK at the BFF boundary only: no Go/PG order is written and the Go helper pins that no REAL row appears.
for (const locale of ["zh-TW", "zh-CN", "en"] as Locale[]) {
    test(`LPU07 MOCK not_awarded DTO renders the placed order and the localized reason once, no resend (${locale})`, async ({ browser }) => {
      test.setTimeout(180_000);
      const context = await openContext(browser, { locale, viewport: { width: 1440, height: 900 } });
      const page = await context.newPage();
      const watched = watch(page);
      const submissions: string[] = [];
      let answer: Record<string, unknown> = {};
      const memberName = `Synthetic member ${canary} mock`;
      await openManualOrder(page, locale);
      const { memberId } = await createMember(page, memberName, `line-${canary}-mock`, "mock");
      await page.route(`**${pointsPath}`, async (route) => {
        submissions.push(route.request().postData() ?? "");
        await route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify(answer) });
      });
      try {
        for (const reason of ["member_erased", "member_unavailable", "forbidden", "limit_reached"] as const) {
          const orderId = uuid();
          answer = {
            order_id: orderId,
            commercial_state: "AWAITING_TRANSFER",
            payment_mode: "bank_transfer",
            total_minor: 1250,
            currency: "TWD",
            expires_at: new Date(Date.now() + 72 * 3600_000).toISOString(),
            buyer_link: null,
            link_state: "storefront_unavailable",
            source: "merchant_manual",
            member_award: { member_id: uuid(), points: 30, state: "not_awarded", reason },
          };
          await openManualOrder(page, locale);
          await addProduct(page);
          await pickMemberBySearch(page, memberName, memberId);
          await page.getByTestId("manual-member-points").fill("30");
          await chooseHomeDelivery(page);
          await fillRecipient(page, `Synthetic customer ${canary} mock`, "0912000002");
          const before = submissions.length;
          const placed = await reviewAndPlace(page);
          expect(placed.status).toBe(201);
          await expect(page.getByTestId("manual-order-result")).toContainText(orderId.slice(0, 8)); // the panel prints the order id prefix
          const badge = page.getByTestId("manual-member-award-result");
          await expect(badge).toHaveCount(1);
          await expect(badge).toHaveAttribute("data-award-state", "not_awarded");
          await expect(badge).toHaveAttribute("data-award-reason", reason);
          await expect(badge).not.toBeEmpty();
          if (locale === "zh-TW") await expect(badge).toContainText(notAwardedHant);
          expect(submissions.length - before, `the ${reason} answer is not re-sent`).toBe(1);
          mockAwards.push({ locale, reason, order_id: orderId });
          cases++;
        }
        watched.assertClean([]);
      } finally {
        await page.unroute(`**${pointsPath}`);
        await context.close();
      }
    });
  }

test("session/logout boundary clears the private search intent and late handlers cannot reopen it", async ({ browser }) => {
    test.setTimeout(120_000);
    const context = await openContext(browser, { locale: "en", viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    const watched = watch(page);
    try {
      await openManualOrder(page, "en");
      const name = `Synthetic member ${canary} logout`;
      await createMember(page, name, `line-${canary}-logout`, "session_boundary");
      // Start a search and sign out while it is in flight: the boundary handler must ignore the late answer.
      await page.getByTestId("manual-member-change").click();
      await page.getByTestId("manual-member-query").fill(name);
      await page.locator("summary").filter({ hasText: "Account" }).click();
      await expect(page.getByTestId("workspace-sign-out")).toBeVisible();
      await page.getByTestId("workspace-sign-out").click();
      await expect(page.getByRole("button", { name: "Sign in with identity service", exact: true })).toBeVisible();
      await expect(page.getByTestId("manual-member-results")).toHaveCount(0);
      expect(await storedPrivateTerms(page)).not.toContain(canary);
      expect(page.url()).not.toContain(canary);
      // A fresh sign-in lands on the dashboard with no resurrected picker/selected member.
      await signIn(page);
      await openManualOrder(page, "en");
      await expect(page.getByTestId("manual-member-selected")).toHaveCount(0);
      const query = page.getByTestId("manual-member-query");
      if (await query.count()) await expect(query, "the cleared boundary must not resurrect the typed search term").toHaveValue("");
      watched.assertClean([name]);
      cases++;
    } finally {
      await context.close();
    }
  });

  test("LPU04 customers:read without customers:points: no picker/points, no deduct, order panel readable, direct API refused", async ({ browser }) => {
    test.setTimeout(180_000);
    await setLevel("readonly");
    const context = await openContext(browser, { locale: "en", viewport: { width: 1440, height: 900 } }, await freshSession(browser));
    const page = await context.newPage();
    const watched = watch(page);
    try {
      await openManualOrder(page, "en");
      await expect(page.getByTestId("manual-member-fields")).toHaveCount(0);
      await expect(page.getByTestId("manual-member-points")).toHaveCount(0);
      await expect(page.getByTestId("manual-member-create-open")).toHaveCount(0);
      await addProduct(page);
      await chooseHomeDelivery(page);
      await fillRecipient(page, `Synthetic customer ${canary} readonly`, "0912000003");
      const placed = await reviewAndPlace(page);
      expect(placed.status).toBe(201);
      expect("member_award" in placed.body).toBe(false);
      const orderId = placed.answer.order_id;
      // No entries on this order: the frozen seam's Empty state, or Ready with an empty list (both mean "the read panel is available").
      const panel = await openOrderPanel(page, "en", orderId, /^(Ready|Empty)$/);
      await expect(panel).toBeVisible();
      await expect(page.getByTestId("order-points-deduct-open")).toHaveCount(0);

      // A direct adjust call with this staff session is refused by the server and changes nothing.
      const csrf = (await context.cookies(origin)).filter(cookie => cookie.name === "__Host-commerce_csrf");
      expect(csrf.length, "the signed-in context has one CSRF cookie").toBe(1);
      expect(/^[A-Za-z0-9_-]{43}$/.test(csrf[0]?.value ?? ""), "the fixture CSRF cookie is valid").toBe(true);
      const refused = await page.request.post(abs(adjustPath(uuid())), {
        headers: { Origin: origin, "Content-Type": "application/json", "Idempotency-Key": randomKey(), "X-CSRF-Token": csrf[0].value },
        data: { delta: -1, note: `Synthetic refusal ${canary}` },
      });
      expect(refused.status(), "direct adjust without customers:points").toBe(403);
      expect(await refused.text()).toContain("forbidden");
      cases++;
      watched.assertClean([`Synthetic customer ${canary} readonly`]);
      permissionEvidence.readonly = { order_id: orderId, direct_adjust_status: refused.status() };
      realOrders.push({ order_id: orderId, member_id: null, member_award: null, locale: "en", device: "desktop", permission: "customers:read only" });
    } finally {
      await context.close();
    }
  });

test("LPU04 no customers:read: picker, points, entries panel and member reads are absent or refused", async ({ browser }) => {
    test.setTimeout(180_000);
    await setLevel("noread");
    const context = await openContext(browser, { locale: "en", viewport: { width: 1440, height: 900 } }, await freshSession(browser));
    const page = await context.newPage();
    const watched = watch(page);
    const reads: Array<{ url: string; status: number }> = [];
    page.on("response", (response) => {
      const { pathname } = new URL(response.url());
      if (pathname === searchPath || pathname === entryPath || pathname === memberCreatePath) reads.push({ url: pathname, status: response.status() });
    });
    try {
      await openManualOrder(page, "en");
      await expect(page.getByTestId("manual-order-form")).toBeVisible();
      await expect(page.getByTestId("manual-member-fields")).toHaveCount(0);
      await expect(page.getByTestId("manual-member-query")).toHaveCount(0);
      await addProduct(page);
      await chooseHomeDelivery(page);
      await fillRecipient(page, `Synthetic customer ${canary} noread`, "0912000004");
      const placed = await reviewAndPlace(page);
      expect(placed.status).toBe(201);
      const orderId = placed.answer.order_id;
      await page.goto(abs(`/en/orders/${orderId}?store=${store}`));
      await expect(page.getByTestId("order-points-panel")).toHaveCount(0);
      await expect(page.getByTestId("order-points-deduct-open")).toHaveCount(0);

      // The members page must not silently read member data without customers:read (absent or refused, never a success).
      await page.goto(abs(`/en/members?store=${store}`));
      await page.waitForLoadState("networkidle", { timeout: 15_000 }).catch(() => undefined); // only tolerates a never-idle page; reads are asserted below
      expect(reads.filter((read) => read.status < 400), "no successful member read without customers:read").toEqual([]);
      await expect(page.locator("[data-member-id]")).toHaveCount(0);
      cases++;
      watched.assertClean([`Synthetic customer ${canary} noread`]);
      permissionEvidence.noread = { order_id: orderId, member_reads: reads };
      realOrders.push({ order_id: orderId, member_id: null, member_award: null, locale: "en", device: "desktop", permission: "no customers:read" });
    } finally {
      await context.close();
    }
  });

test("M11 list-only staff cannot open a manual order", async ({ browser }) => {
  await setLevel("orders-readonly");
  const context = await openContext(browser, { locale: "en", viewport: { width: 1440, height: 900 } }, await freshSession(browser));
  const page = await context.newPage();
  try {
    await page.goto(abs(`/en/orders?store=${store}`));
    await expect(page.getByTestId("orders-list")).toBeVisible();
    const heading = page.getByTestId("merchant-orders").locator("header.signed-orders-heading");
    await expect(heading.getByRole("link", { name: createOrderLabel.en, exact: true })).toHaveCount(0);
    const options = page.waitForResponse(reply => new URL(reply.url()).pathname === pointsPath + "/options");
    await page.goto(abs(`/en/orders/new?store=${store}`));
    expect((await options).status()).toBe(403);
    await expect(page.getByTestId("manual-order-form")).toHaveCount(0);
    cases++;
  } finally {
    await context.close();
  }
});
