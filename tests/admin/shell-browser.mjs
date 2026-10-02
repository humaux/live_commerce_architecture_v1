import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { otherID } from "./shell-fixture.mjs";
import { shellCopy } from "../../apps/admin/src/shell-copy.ts";
const require = createRequire(import.meta.url);
const sizes = [
  [1366, 768],
  [1586, 992],
  [2000, 1100],
  [1024, 768],
  [768, 1024],
  [390, 844],
  [375, 812],
  [360, 740],
];
export async function runShellGate({
  page,
  context,
  f,
  base,
  output,
  storeID,
}) {
  const evidence = [];
  const shot = async (name) => {
    await page.screenshot({
      path: `${output}/${name}.png`,
      animations: "disabled",
    });
  };
  const ready = async (path = "/") => {
    await page
      .locator(`[data-shell-route="${path}"]`)
      .waitFor({ state: "attached" });
    await page
      .locator('[data-testid="nav-group-overview"]')
      .waitFor({ state: "attached" });
  };
  const openMenu = async () => {
    if (
      await page.locator('[aria-controls="workspace-navigation"]').isVisible()
    )
      await page.locator('[aria-controls="workspace-navigation"]').click();
  };
  const geometry = async () => {
    const result = await page.evaluate(() => {
      const top = document.querySelector("[data-shell-topbar]"),
        rail = document.querySelector("[data-shell-rail]"),
        main = document.querySelector("#main");
      const tr = top.getBoundingClientRect(),
        rr = rail.getBoundingClientRect(),
        mr = main.getBoundingClientRect();
      const hit = (el, r) =>
        el.contains(
          document.elementFromPoint(
            Math.max(1, r.x + Math.min(20, r.width / 2)),
            Math.max(1, r.y + Math.min(20, r.height / 2)),
          ),
        );
      return {
        overflow: document.documentElement.scrollWidth > innerWidth,
        topHit: hit(top, tr),
        mainHit: hit(main, mr),
        railHit: innerWidth < 1024 || hit(rail, rr),
        overlap:
          mr.top < tr.bottom - 1 ||
          (innerWidth >= 1024 && tr.left < rr.right - 1),
      };
    });
    assert.deepEqual(result, {
      overflow: false,
      topHit: true,
      mainHit: true,
      railHit: true,
      overlap: false,
    });
  };
  for (const locale of ["zh-CN", "zh-TW", "en"])
    for (const [width, height] of sizes) {
      await page.setViewportSize({ width, height });
      await page.goto(`${base}/${locale}/?store=${otherID}`);
      await ready();
      await geometry();
      await shot(`shell-${locale}-${width}x${height}`);
      // Browser-level audit of shell only. Old page-body findings belong to later domain units.
      await page.addScriptTag({
        content: await readFile(require.resolve("axe-core/axe.min.js"), "utf8"),
      });
      const axe = await page.evaluate(async () => {
        const result = await window.axe.run({
          include: ["[data-shell-rail]", "[data-shell-topbar]"],
        });
        return result.violations
          .filter((v) => v.impact === "serious" || v.impact === "critical")
          .map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }));
      });
      assert.deepEqual(axe, [], `${locale} ${width}: axe`);
      if (width >= 1024) {
        await page.locator("[data-shell-rail] button:visible").first().focus();
        const order = [];
        for (let step = 0; step < 32 && order.at(-1) !== "main"; step++) {
          const region = await page.evaluate(() => {
            const active = document.activeElement;
            return active?.closest("[data-shell-rail]")
              ? "rail"
              : active?.closest("[data-shell-topbar]")
                ? "topbar"
                : active?.closest("main")
                  ? "main"
                  : "outside";
          });
          if (region !== order.at(-1)) order.push(region);
          await page.keyboard.press("Tab");
        }
        assert.deepEqual(
          order,
          ["rail", "topbar", "main"],
          "G-UI4 keyboard focus order",
        );
      }
      if (width < 1024) {
        const menu = page.locator('[aria-controls="workspace-navigation"]');
        await menu.click();
        await page.locator("[data-shell-rail]").waitFor({ state: "visible" });
        await page.keyboard.press("Shift+Tab");
        assert.ok(
          await page.evaluate(() =>
            document
              .querySelector("[data-shell-rail]")
              .contains(document.activeElement),
          ),
        );
        await page.keyboard.press("Tab");
        await page.keyboard.press("Escape");
        assert.equal(
          await menu.evaluate((e) => e === document.activeElement),
          true,
        );
        await menu.click();
        await shot(`drawer-${locale}-${width}x${height}`);
        const small = await page
          .locator(
            "[data-shell-rail] button:visible,[data-shell-topbar] button:visible,[data-shell-topbar] select:visible,[data-shell-topbar] summary:visible",
          )
          .evaluateAll((elements) =>
            elements
              .filter((e) => {
                const r = e.getBoundingClientRect();
                return r.width < 43.9 || r.height < 43.9;
              })
              .map((e) => e.textContent),
          );
        assert.deepEqual(small, [], "44px shell targets");
        await page.keyboard.press("Escape");
      }
      // Each rendered destination is actually clicked, not inferred from the registry.
      const navPaths = [
        "studio",
        "orders",
        "products",
        "collections",
        "inventory",
        "customers",
        "promotions",
        "ads",
        "design",
        "finance",
        "settings",
        "team",
        "billing",
      ];
      for (const id of navPaths) {
        await openMenu();
        const target = page.getByTestId(
          id === "studio"
            ? "nav-group-live"
            : id === "customers"
              ? "nav-group-customers"
              : id === "design"
                ? "nav-group-storefront"
                : id === "finance"
                  ? "nav-group-finance"
                  : `nav-${id}`,
        );
        const group = ["products", "collections", "inventory"].includes(id)
          ? "catalog"
          : ["promotions", "ads"].includes(id)
            ? "marketing"
            : ["settings", "team", "billing"].includes(id)
              ? "settings"
              : null;
        if (
          group &&
          (await page
            .getByTestId(`nav-group-${group}`)
            .getAttribute("aria-expanded")) !== "true"
        )
          await page.getByTestId(`nav-group-${group}`).click();
        await target.click();
        await page.waitForURL(
          (url) =>
            url.pathname === `/${locale}/${id === "studio" ? "studio" : id}`,
        );
        await ready(`/${id}`);
        assert.equal(
          await page.getByTestId("route-forbidden").count(),
          0,
          "owner may open " + id,
        );
      }
      await page.goto(`${base}/${locale}/?store=${storeID}`);
      await ready();
      const account = page.locator("[data-shell-topbar] details").last();
      await account.locator("summary").click();
      await page.getByTestId("workspace-sign-out").scrollIntoViewIfNeeded();
      assert.ok(await page.getByTestId("workspace-sign-out").isVisible());
      evidence.push({
        locale,
        width,
        height,
        axe: "no serious/critical",
        navigation: "13 destinations clicked",
        geometry: "pass",
        touch: width < 1024 ? "44px" : "not required",
      });
    }
  // UX authorization is independently tested against a changed MOCK /stores response. Go auth remains in existing real gates.
  // These hidden routes have shell-only coverage here; no claim of business-form acceptance.
  for (const path of ["/orders/new", "/products/import"]) {
    await page.goto(`${base}/en${path}?store=${storeID}`);
    await ready(path);
    assert.equal(await page.getByTestId("route-forbidden").count(), 0);
    assert.equal(await page.locator("[data-shell-rail]").count(), 1);
  }
  f.state.role = "staff";
  f.state.permissions = ["orders:read"];
  await page.goto(`${base}/en/products?store=${storeID}`);
  await page.getByTestId("route-forbidden").waitFor();
  assert.equal(await page.getByTestId("nav-group-catalog").count(), 0);
  assert.equal(await page.getByTestId("products-search").count(), 0);
  assert.match(await page.title(), /Products/);
  f.state.role = "owner";
  await page.goto(`${base}/en/products?store=${storeID}`);
  await ready("/products");
  await page.getByTestId("shell-store-selector").selectOption(otherID);
  await page.waitForURL(
    (url) =>
      (url.pathname === "/en" && url.searchParams.get("store") === otherID) ||
      (url.pathname === "/en/" && url.searchParams.get("store") === otherID),
  );
  await ready();
  assert.equal(
    await page.getByTestId("shell-store-selector").inputValue(),
    otherID,
  );
  assert.equal(
    await page.getByTestId("products-search").count(),
    0,
    "old page unmounted on store switch",
  );
  assert.equal(
    await context
      .cookies()
      .then((c) => c.some((x) => x.name === "__Host-commerce_session")),
    true,
  );
  await context.clearCookies();
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    for (const [path, label] of [["/reset", "reset"], ["/signup", "signup"], [`/invite/${"a".repeat(43)}`, "invite"]]) {
      await page.goto(`${base}/${locale}${path}`);
      assert.equal(await page.title(), shellCopy[locale][label], "Public page title from registry");
      if (label === "invite") assert.equal(await page.locator('meta[name="referrer"]').getAttribute("content"), "no-referrer", "Invite privacy metadata preserved");
    }
  }
  await writeFile(
    `${output}/browser-results.json`,
    JSON.stringify(
      {
        tier: "MOCK",
        matrix: evidence,
        role: "pass UI403",
        storeSwitch: "hard-navigation clears old page",
        backendAuthorization: "NOT_PROVEN_BY_THIS_MOCK",
        publicTitles: "9 registry title checks passed",
      },
      null,
      2,
    ) + "\n",
  );
  console.log(
    `PASS G-UI2/G-UI4: ${evidence.length} matrix cases, role negative, store switch, axe shell audit`,
  );
}
