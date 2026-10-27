// Purpose: OC-11 / OC-Q10 admin gate: an uncertain publish shows 「發布結果待確認」, blocks re-publish and rechecks by READ only.
// Depends on: @playwright/test, design-copy; the same harness env as tests/admin/design.spec.ts (LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_DESIGN_STORES).
// Used by: tests/foundation/browser_store_design_test.go (same playwright invocation as design.spec.ts; playwright.config.ts store-design testMatch).
// The publish answer is lost AFTER Go committed (route.fetch then abort), the worst case for a blind retry. Interception only records
// and cuts the answer; every user step is a real click/fill.
import { expect, test } from "@playwright/test";
import { designCopy, fill } from "../../../apps/admin/lib/design-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
// The BFF-probe store: its own merchant, and design.spec.ts's PG-fact journeys (exact version counts) stay untouched.
const store = required("LC_BROWSER_BFF_STORE");
test.use({ baseURL: origin });

{
  test("OC11 publish pending (en, three-locale wording)", async ({ page }) => {
    const c = designCopy.en;
    expect(designCopy["zh-TW"].publishPending).toBe("發布結果待確認");
    expect(designCopy["zh-CN"].publishPending).toBe("发布结果待确认");
    const publishes: { key: string | undefined; body: string | null }[] = [];
    const posts: string[] = [];
    await page.route("**/design/publish", async (route) => {
      const req = route.request();
      publishes.push({ key: req.headers()["idempotency-key"], body: req.postData() });
      if (publishes.length === 1) {
        await route.fetch(); // Go commits...
        await route.abort("connectionreset"); // ...but the browser never hears
      } else await route.continue();
    });
    page.on("request", (r) => {
      if (r.method() !== "GET" && !r.url().includes("/design/publish")) posts.push(`${r.method()} ${r.url()}`);
    });
    page.on("dialog", (d) => void d.accept());
    await page.goto(new URL("/en/", origin).toString());
    await page.getByRole("button", { name: "Sign in with identity service" }).click();
    await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
    await page.goto(`/en/design?store=${store}`);
    await page.getByTestId("design-name").fill(`oc11 ${Date.now()}`);
    await page.getByTestId("design-publish").click();

    const pending = page.getByTestId("design-publish-pending");
    await expect(pending).toContainText(c.publishPending);
    await expect(page.getByTestId("design-note")).toHaveCount(0); // never a success note
    await expect(page.getByTestId("design-publish")).toBeDisabled(); // no second publish possible
    expect(publishes, "OC11-A1 exactly one publish before recheck").toHaveLength(1);
    posts.length = 0; // the save before publish is legitimate; from here on nothing may be written

    const versionReads: string[] = [];
    page.on("request", (r) => {
      if (r.method() === "GET" && r.url().includes("/design/versions")) versionReads.push(r.url());
    });
    await page.getByTestId("design-publish-recheck").click();
    await expect(page.getByTestId("design-note")).toContainText(new RegExp(c.okPublished.replace("{n}", "\\d+")));
    await expect(pending).toHaveCount(0);
    expect(versionReads.length).toBeGreaterThan(0);
    expect(publishes, "OC11-A2 recheck is a read: still exactly one publish request, one key").toHaveLength(1);
    expect(posts, "OC11-A3 no other write was sent").toEqual([]);
  });
}
