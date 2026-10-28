// Purpose: OC-11 / OC-Q10 admin gate: an uncertain publish shows 「發布結果待確認」, blocks re-publish, survives a reload and rechecks by READ only.
// Depends on: @playwright/test, design-copy; the same harness env as tests/admin/design.spec.ts (LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_BFF_STORE).
// Used by: tests/foundation/browser_store_design_test.go (same playwright invocation as design.spec.ts; playwright.config.ts store-design testMatch).
// Publish answers are cut by interception only (route.fetch then abort = committed but unheard; plain abort = never reached Go); every user step is a
// real click/fill. Tests share one store and run in file order (workers: 1).
import { expect, test, type Page, type Route } from "@playwright/test";
import { designCopy, fill } from "../../../apps/admin/lib/design-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
// The BFF-probe store: its own merchant, and design.spec.ts's PG-fact journeys (exact version counts) stay untouched.
const store = required("LC_BROWSER_BFF_STORE");
const c = designCopy.en;
const okPublished = new RegExp(c.okPublished.replace("{n}", "\\d+"));
test.use({ baseURL: origin });

type Cut = "none" | "after-commit" | "before-commit" | "held-after-commit";
async function open(page: Page) {
  const publishes: { key: string | undefined; body: string | null }[] = [];
  const writes: string[] = [];
  const state = { cut: "none" as Cut, release: () => {} };
  await page.route("**/design/publish", async (route: Route) => {
    const req = route.request();
    publishes.push({ key: req.headers()["idempotency-key"], body: req.postData() });
    if (state.cut === "after-commit") {
      await route.fetch(); // Go commits...
      await route.abort("connectionreset"); // ...but the browser never hears
    } else if (state.cut === "held-after-commit") {
      await route.fetch(); // Go commits, then the answer is held back while the test reloads the page
      await new Promise<void>((resolve) => (state.release = resolve));
      await route.abort("connectionreset").catch(() => {}); // the page that asked is gone
    } else if (state.cut === "before-commit") await route.abort("connectionreset"); // never reached Go
    else await route.continue();
  });
  page.on("request", (r) => {
    if (r.method() !== "GET" && !r.url().includes("/design/publish")) writes.push(`${r.method()} ${r.url()}`);
  });
  page.on("dialog", (d) => void d.accept());
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
  await page.goto(`/en/design?store=${store}`);
  return { publishes, writes, state };
}
const edit = (page: Page, tag: string) => page.getByTestId("design-name").fill(`oc11 ${tag} ${Date.now()}`);

test("OC11 publish lost after commit: pending, no second publish, recheck finds it", async ({ page }) => {
  expect(designCopy["zh-TW"].publishPending).toBe("發布結果待確認");
  expect(designCopy["zh-CN"].publishPending).toBe("发布结果待确认");
  const { publishes, writes, state } = await open(page);
  state.cut = "after-commit";
  await edit(page, "a");
  await page.getByTestId("design-publish").click();

  const pending = page.getByTestId("design-publish-pending");
  await expect(pending).toContainText(c.publishPending);
  await expect(page.getByTestId("design-note")).toHaveCount(0); // never a success note
  await expect(page.getByTestId("design-publish")).toBeDisabled(); // no second publish possible
  expect(publishes, "OC11-A1 exactly one publish before recheck").toHaveLength(1);
  writes.length = 0; // the save before publish is legitimate; from here on nothing may be written

  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await expect(pending).toHaveCount(0);
  expect(publishes, "OC11-A2 recheck is a read: still exactly one publish request, one key").toHaveLength(1);
  expect(writes, "OC11-A3 no other write was sent").toEqual([]);
});

test("OC11 publish never reached Go: survives reload, failed read is unconfirmed, empty read is not-yet, never success", async ({ page }) => {
  const { publishes, state } = await open(page);
  state.cut = "before-commit";
  await edit(page, "b");
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  expect(publishes).toHaveLength(1);

  await page.reload(); // OC11-A5: UNKNOWN is still UNKNOWN after a refresh
  await expect(page.getByTestId("design-publish-pending"), "OC11-A5 pending survives a reload").toContainText(c.publishPending);
  await expect(page.getByTestId("design-publish"), "OC11-A5 Publish stays disabled after reload").toBeDisabled();

  await page.route("**/design/versions", (route) => route.abort("connectionreset")); // the recheck read itself fails
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-unconfirmed"), "OC11-A6 a failed read is 'could not confirm', not 'no record'").toContainText(c.publishUnconfirmed);
  await expect(page.getByTestId("design-publish-notyet")).toHaveCount(0);
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  await page.unroute("**/design/versions");

  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A7 empty read: not recorded yet").toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  expect(publishes, "OC11-A8 still one publish request").toHaveLength(1);
});

test("OC11 old publish row of the same draft version never confirms a new lost publish (publish, rollback, publish again)", async ({ page }) => {
  const { state } = await open(page);
  await edit(page, "c");
  await page.getByTestId("design-publish").click(); // real publish of this draft version
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await page.getByTestId("design-tab-versions").click();
  await page.getByTestId("design-rollback").last().click(); // rollback: the newest row is now a rollback, the draft is unchanged
  await expect(page.getByTestId("design-note")).toContainText(/Rolled back/);
  await expect(page.getByTestId("design-publish")).toBeEnabled();

  state.cut = "before-commit"; // publish this same draft version again; the answer is lost and nothing committed
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A4 the old publish row (source_version equal) must not confirm").toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  await expect(page.getByTestId("design-note"), "no success note of any kind").toHaveCount(0);
});

test("OC11 reload while the publish request is in flight: pending is armed before the request, no second publish", async ({ page }) => {
  const { publishes, state } = await open(page);
  state.cut = "held-after-commit";
  await edit(page, "d");
  await page.getByTestId("design-publish").click();
  await expect.poll(() => publishes.length, { message: "the publish request left the page" }).toBe(1);
  await page.reload(); // Go has committed, the answer has not come back: this is still UNKNOWN
  await expect(page.getByTestId("design-publish-pending"), "OC11-A9 pending armed before the request").toContainText(c.publishPending);
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  state.release();
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  expect(publishes, "OC11-A10 the publish POST count stays 1").toHaveLength(1);
});

test("OC11 stale versions list: confirmed publish, failed refresh, second publish aborted before commit never shows success", async ({ page }) => {
  const { state } = await open(page);
  await page.route("**/design/versions", (route) => route.abort("connectionreset")); // every versions read fails from here on
  await edit(page, "e");
  await page.getByTestId("design-publish").click(); // real publish; its answer arrives, the list refresh after it fails
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  state.cut = "before-commit"; // the same draft again; nothing commits
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await page.unroute("**/design/versions");
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A11 the first (confirmed) publish row must not confirm the second").toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-note")).toHaveCount(0);
});
