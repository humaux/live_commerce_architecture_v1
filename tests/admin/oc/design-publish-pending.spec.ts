// Purpose: OC-11 / OC-Q10 admin gate: an uncertain publish shows 「發布結果待確認」, blocks re-publish and rechecks by READ only.
// Depends on: @playwright/test, design-copy; the same harness env as tests/admin/design.spec.ts (LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_DESIGN_STORES).
// Used by: NOT_RUN until the integrator registers it next to design.spec.ts in tests/foundation/browser_store_design_test.go.
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
type Journey = { id: string; locale: "en" | "zh-TW"; viewport: "desktop" | "mobile" };
const journeys = JSON.parse(required("LC_BROWSER_DESIGN_STORES")) as Journey[];
test.use({ baseURL: origin });

for (const j of journeys) {
  test(`OC11 publish pending ${j.locale} ${j.viewport}`, async ({ page }) => {
    const c = designCopy[j.locale];
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
    await page.goto(`/${j.locale}/design?store=${j.id}`);
    await page.getByTestId("design-name").fill(`oc11 ${Date.now()}`);
    await page.getByTestId("design-publish").click();

    const pending = page.getByTestId("design-publish-pending");
    await expect(pending).toContainText(c.publishPending);
    await expect(page.getByTestId("design-note")).toHaveCount(0); // never a success note
    await expect(page.getByTestId("design-publish")).toBeDisabled(); // no second publish possible
    expect(publishes).toHaveLength(1);
    posts.length = 0; // the save before publish is legitimate; from here on nothing may be written

    const versionReads: string[] = [];
    page.on("request", (r) => r.method() === "GET" && r.url().includes("/design/versions") && versionReads.push(r.url()));
    await page.getByTestId("design-publish-recheck").click();
    await expect(page.getByTestId("design-note")).toContainText(fill(c.okPublished, { n: 1 }));
    await expect(pending).toHaveCount(0);
    expect(versionReads.length).toBeGreaterThan(0);
    expect(publishes, "recheck is a read: still exactly one publish request, one key").toHaveLength(1);
    expect(posts, "no other write was sent").toEqual([]);
  });
}
