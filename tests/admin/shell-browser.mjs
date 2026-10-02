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
  const annotatedTargets = async (name, selector) => {
    const measured = await page.locator(selector).evaluateAll((elements) =>
      elements.map((el, index) => {
        const r = el.getBoundingClientRect();
        const box = document.createElement("div");
        box.dataset.w0Measurement = "true";
        Object.assign(box.style, {
          position: "fixed",
          left: `${r.x}px`,
          top: `${r.y}px`,
          width: `${r.width}px`,
          height: `${r.height}px`,
          outline: "2px solid #b42318",
          zIndex: "9999",
          pointerEvents: "none",
        });
        const label = document.createElement("span");
        label.textContent = `${index + 1}: ${Math.round(r.width)} × ${Math.round(r.height)} px`;
        Object.assign(label.style, {
          position: "absolute",
          left: "0",
          bottom: "0",
          color: "#b42318",
          background: "white",
          font: "bold 10px/12px sans-serif",
        });
        box.append(label);
        document.body.append(box);
        return {
          label: el.getAttribute("aria-label") || el.textContent?.trim(),
          width: r.width,
          height: r.height,
        };
      }),
    );
    assert.ok(measured.length > 0);
    for (const target of measured)
      assert.ok(
        target.width >= 43.9 && target.height >= 43.9,
        "44px measured shell target",
      );
    await shot(name);
    await writeFile(
      `${output}/${name}.json`,
      JSON.stringify(
        {
          annotation:
            "Test-only DOM measurement overlay; CSS pixels, no image rescaling",
          measured,
        },
        null,
        2,
      ) + "\n",
    );
    await page
      .locator("[data-w0-measurement]")
      .evaluateAll((elements) => elements.forEach((el) => el.remove()));
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
      const brand = page.getByTestId("shell-store-name");
      assert.equal(
        await brand.textContent(),
        "A very long English store name for accessible workspace switching without overlap",
      );
      assert.deepEqual(
        await brand.evaluate((el) => ({
          overflow: getComputedStyle(el).overflow,
          textOverflow: getComputedStyle(el).textOverflow,
          whiteSpace: getComputedStyle(el).whiteSpace,
          clipped: el.scrollWidth > el.clientWidth,
        })),
        {
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          clipped: true,
        },
      );
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
        if (width === 390)
          await annotatedTargets(
            `touch-targets-${locale}-390-topbar`,
            "[data-shell-topbar] button:visible,[data-shell-topbar] select:visible,[data-shell-topbar] summary:visible",
          );
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
        if (width === 390)
          await annotatedTargets(
            `touch-targets-${locale}-390-drawer`,
            "[data-shell-rail] button:visible",
          );
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
        if (id === "inventory" && (width === 1586 || width === 390)) {
          await openMenu();
          assert.equal(
            await page
              .getByTestId("nav-group-catalog")
              .getAttribute("aria-expanded"),
            "true",
          );
          assert.equal(
            await page
              .getByTestId("nav-inventory")
              .getAttribute("aria-current"),
            "page",
          );
          await shot(`subroute-expanded-${locale}-${width}`);
          if (width < 1024) await page.keyboard.press("Escape");
        }
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
      if (width === 1586 || width === 390)
        await shot(`account-open-${locale}-${width}`);
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
  for (const locale of ["zh-CN", "zh-TW", "en"]) {
    for (const [width, height] of [
      [1586, 992],
      [390, 844],
    ]) {
      await page.setViewportSize({ width, height });
      await page.goto(`${base}/${locale}/products?store=${storeID}`);
      await page.getByTestId("route-forbidden").waitFor();
      assert.equal(await page.getByTestId("nav-group-catalog").count(), 0);
      assert.equal(await page.getByTestId("products-search").count(), 0);
      assert.equal(await page.title(), shellCopy[locale].products);
      await shot(`forbidden-${locale}-${width}`);
    }
  }
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
  await page.goto(
    `${base}/en/products?store=99999999-9999-4999-8999-999999999999`,
  );
  await page.getByTestId("route-forbidden").waitFor();
  assert.equal(
    await page.getByTestId("shell-store-brand").count(),
    0,
    "invalid explicit store cannot fall back to another store",
  );
  assert.equal(await page.getByTestId("shell-store-selector").count(), 0);
  // A failed identity request must not confuse session expiry with a transient outage.
  await page.route("**/api/stores", (route) =>
    route.fulfill({ status: 401, json: { code: "unauthorized" } }),
  );
  await page.goto(`${base}/en/studio?store=${storeID}`);
  await page.getByTestId("shell-session-expired").waitFor({ timeout: 5000 });
  assert.equal(
    await page.getByTestId("merchant-studio").count(),
    0,
    "expired session cannot mount domain content",
  );
  assert.equal(
    await page.getByTestId("shell-sign-in").getAttribute("href"),
    "/en",
  );
  assert.ok(
    (await page.getByTestId("shell-sign-in").boundingBox()).height >= 44,
  );
  for (const selector of [
    "[data-testid=shell-store-selector]",
    "[data-testid=shell-store-brand]",
    "[data-shell-route]",
    "[data-shell-rail] nav",
  ]) {
    assert.equal(
      await page.locator(selector).count(),
      0,
      "expired session clears scoped chrome: " + selector,
    );
  }
  assert.equal(
    (await page.locator("body").innerText()).includes("Synthetic baseline"),
    false,
  );
  await page.screenshot({ path: `${output}/session-expired-en.png` });
  await page.unroute("**/api/stores");
  // Session notifications clear already-loaded chrome and block delayed reads.
  // Exercise the actual sender primitives, not a forged incoming MessageEvent.
  const noScopedChrome = async () => {
    await page.getByTestId("shell-session-expired").waitFor();
    for (const selector of [
      "[data-testid=shell-store-selector]",
      "[data-testid=shell-store-brand]",
      "[data-shell-route]",
      "[data-testid=merchant-studio]",
      "[data-shell-rail] nav",
    ])
      assert.equal(await page.locator(selector).count(), 0, selector);
  };
  for (const mechanism of ["local", "broadcast", "storage", "focus"]) {
    await page.goto(`${base}/en/studio?store=${storeID}`);
    await ready("/studio");
    const sender = await context.newPage();
    await sender.goto(`${base}/en/reset`);
    if (mechanism === "focus") {
      await page.route("**/api/stores", (route) =>
        route.fulfill({ status: 401, json: { code: "unauthorized" } }),
      );
      await page.evaluate(() => window.dispatchEvent(new Event("focus")));
    } else if (mechanism === "local")
      await page.evaluate(() =>
        window.dispatchEvent(new Event("commerce-session-logout")),
      );
    else
      await sender.evaluate((kind) => {
        if (kind === "storage")
          localStorage.setItem("commerce-session-logout", crypto.randomUUID());
        else {
          const channel = new BroadcastChannel("commerce-session");
          channel.postMessage({ type: "logout" });
          channel.close();
        }
      }, mechanism);
    await noScopedChrome();
    if (mechanism === "focus") await page.unroute("**/api/stores");
    await sender.close();
  }
  let releaseRead, startedRead;
  const pendingRead = new Promise((resolve) => {
    releaseRead = resolve;
  });
  const readStarted = new Promise((resolve) => {
    startedRead = resolve;
  });
  await page.route("**/api/stores", async (route) => {
    const response = await route.fetch();
    startedRead();
    await pendingRead;
    await route.fulfill({ response }).catch(() => {}); // abort after logout is expected
  });
  await page.goto(`${base}/en/studio?store=${storeID}`);
  await readStarted;
  await page.evaluate(() =>
    window.dispatchEvent(new Event("commerce-session-logout")),
  );
  releaseRead();
  await noScopedChrome();
  await page.unroute("**/api/stores");
  await context.clearCookies();
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    for (const [path, label] of [
      ["/reset", "reset"],
      ["/signup", "signup"],
      [`/invite/${"a".repeat(43)}`, "invite"],
    ]) {
      await page.goto(`${base}/${locale}${path}`);
      assert.equal(
        await page.title(),
        shellCopy[locale][label],
        "Public page title from registry",
      );
      if (label === "invite")
        assert.equal(
          await page.locator('meta[name="referrer"]').getAttribute("content"),
          "no-referrer",
          "Invite privacy metadata preserved",
        );
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
        sessionExpiry:
          "401 has dedicated sign-in recovery; domain remains unmounted",
      },
      null,
      2,
    ) + "\n",
  );
  console.log(
    `PASS G-UI2/G-UI4: ${evidence.length} matrix cases, role negative, store switch, axe shell audit`,
  );
}
