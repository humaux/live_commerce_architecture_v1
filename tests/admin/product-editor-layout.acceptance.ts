// Purpose: Verify real editor navigation, visible field focus and reserved savebar geometry without synthetic DOM changes.
// Depends on: Playwright Page/expect and the production ProductDocumentForm markup; caller-owned browser/PG fixture.
// Used by: product-editor.acceptance.ts under --browser-product-editor; reads geometry and performs real nav clicks.
import { expect, type Page } from "@playwright/test";
import { presentationCopy } from "../../apps/admin/lib/presentation-copy";

// Run the additional interaction sequence once per locale on the caller's page;
// the existing reserved-layout checks still run on every create/edit fixture.
const hardenedLocales = new WeakMap<Page, Set<string>>();

/** Exercises UXH01–05 with native input and read-only browser measurements. */
async function assertProductEditorHardening(page: Page) {
  const locale = new URL(page.url()).pathname.split("/")[1] as "zh-TW" | "zh-CN" | "en";
  const labels = {
    "zh-TW": { nav: "商品編輯導覽", group: "商品編輯區段", toggle: "查看完成度", heading: "完成度" },
    "zh-CN": { nav: "商品编辑导航", group: "商品编辑区段", toggle: "查看完成度", heading: "完成度" },
    en: { nav: "Product editor navigation", group: "Product editor sections", toggle: "Show readiness checklist", heading: "Readiness" },
  }[locale];
  expect(labels, "UXH01 supported editor locale").toBeTruthy();
  const seen = hardenedLocales.get(page) ?? new Set<string>();
  if (seen.has(locale)) return;
  const originalViewport = page.viewportSize()!;
  const form = page.locator("form.pe-document");
  const nav = form.locator(".pe-index nav");
  const tabs = form.getByTestId("product-section-tabs");
  const toggle = form.locator(".pe-readiness-toggle");
  const checklist = form.locator("#pe-readiness-section");
  const name = form.getByTestId("product-name");
  const originalName = await name.inputValue();
  const originalExpanded = originalViewport.width <= 900 && await toggle.getAttribute("aria-expanded") === "true";
  const verify = expect.configure({ soft: true, timeout: 1000 });
  const resize = async (width: number) => {
    await page.setViewportSize({ width, height: originalViewport.height });
    if (width <= 900) await expect(toggle).toBeVisible();
    else await expect(toggle).toBeHidden();
    // READ/MEASURE: let native resize/React layout and observer measurements settle.
    await nav.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  };
  const setExpanded = async (expanded: boolean) => {
    if ((await toggle.getAttribute("aria-expanded") === "true") !== expanded) await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", String(expanded));
  };
  // Keyboard traversal acquires real button focus; clicking a section deliberately
  // focuses its field, so locator.focus() would not represent this user flow.
  const focusLastSection = async () => {
    const last = nav.locator("button[aria-controls]").last();
    for (let i = 0; i < 160; i++) {
      if (await last.evaluate(el => document.activeElement === el)) return last;
      await page.keyboard.press("Shift+Tab");
    }
    throw new Error("UXH02 keyboard could not reach the last section button");
  };
  try {
    await resize(900);
    await setExpanded(true);
    console.log("UXH01", JSON.stringify({ locale, expected: labels }));
    await verify(nav, "UXH01 navigation has its distinct localized name").toHaveAccessibleName(labels!.nav);
    await verify(tabs, "UXH01 TabStrip group has its distinct localized name").toHaveAccessibleName(labels!.group);
    await verify(tabs, "UXH01 named strip exposes the group role").toHaveAttribute("role", "group");
    await verify(toggle, "UXH01 readiness action has its distinct localized name").toHaveAccessibleName(labels!.toggle);
    await verify(checklist.getByRole("heading", { level: 2 }), "UXH01 readiness h2 retains the checklist name").toHaveAccessibleName(labels!.heading);
    await setExpanded(false);

    // The readiness button is immediately after the strip. A real click followed
    // by Shift+Tab focuses the last section without invoking its field-focus action.
    await toggle.click();
    await toggle.click();
    let focused = await focusLastSection();
    const focusedSection = await focused.getAttribute("aria-controls");
    await expect(focused).toBeFocused();
    await resize(901);
    focused = nav.locator(`button[aria-controls="${focusedSection}"]`);
    console.log("UXH02", JSON.stringify({ locale, direction: "900→901", focusedSection,
      retained: await focused.evaluate(el => document.activeElement === el) }));
    await verify(focused, "UXH02 section button retains keyboard focus when leaving narrow layout").toBeFocused();
    // Reacquire focus after a RED loss so the reverse transition is independently tested.
    focused = await focusLastSection();
    await expect(focused).toBeFocused();
    await resize(900);
    focused = nav.locator(`button[aria-controls="${focusedSection}"]`);
    console.log("UXH02", JSON.stringify({ locale, direction: "901→900", focusedSection,
      retained: await focused.evaluate(el => document.activeElement === el) }));
    await verify(focused, "UXH02 section button retains keyboard focus when entering narrow layout").toBeFocused();

    await resize(390);
    await nav.locator('button[aria-controls="basics"]').click();
    await name.click();
    await expect(name).toBeFocused();
    const scrollToEnd = async () => {
      const maximum = await tabs.evaluate(el => el.scrollWidth - el.clientWidth); // READ/MEASURE
      expect(maximum, "UXH03 native strip really overflows").toBeGreaterThan(1);
      await tabs.hover();
      await page.mouse.wheel(maximum + 100, 0);
      await expect.poll(() => tabs.evaluate(el => el.scrollWidth - el.clientWidth - el.scrollLeft),
        { message: "UXH03 native horizontal wheel reaches the strip end" }).toBeLessThanOrEqual(1);
      return tabs.evaluate(el => el.scrollLeft); // READ/MEASURE
    };
    const before = await scrollToEnd();
    await expect(name).toBeFocused();
    await page.keyboard.press("End");
    await page.keyboard.insertText(" UXH03");
    await expect(name).toHaveValue(`${originalName} UXH03`);
    await tabs.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))); // READ/MEASURE
    const after = await tabs.evaluate(el => el.scrollLeft); // READ/MEASURE
    console.log("UXH03", JSON.stringify({ locale, operation: "unrelated draft edit", before, after }));
    verify(Math.abs(after - before), "UXH03 unrelated draft render preserves the user's strip position").toBeLessThanOrEqual(1);
    await name.fill(originalName);
    await scrollToEnd();
    await setExpanded(true);
    await checklist.locator('.pe-readiness[aria-controls="media"]').first().click();
    const selected = nav.locator('button[aria-controls="media"]');
    await verify(selected, "UXH03 actual readiness click changes the active section").toHaveAttribute("aria-current", "location");
    await verify.poll(() => selected.evaluate(el => {
      const strip = el.closest('[data-testid="product-section-tabs"]')!;
      const r = el.getBoundingClientRect(), frame = strip.getBoundingClientRect();
      return r.left >= frame.left - 1 && r.right <= frame.right + 1;
    }), { message: "UXH03 active-section change reveals the selected tab" }).toBe(true);
    console.log("UXH03", JSON.stringify({ locale, operation: "readiness click reveals media", scrollLeft: await tabs.evaluate(el => el.scrollLeft) }));
    await setExpanded(false);

    const hint = nav.getByText(`${presentationCopy[locale as keyof typeof presentationCopy].previous} · ${presentationCopy[locale as keyof typeof presentationCopy].next}`, { exact: true });
    await expect(hint).toBeVisible();
    const measureHintRow = () => tabs.evaluate(el => {
      const strip = el.getBoundingClientRect(), frame = el.parentElement!.getBoundingClientRect();
      return { overflow: el.scrollWidth - el.clientWidth, reserved: frame.bottom - strip.bottom };
    }); // READ/MEASURE: isolate the hint row from unrelated responsive field wrapping.
    const narrow = await measureHintRow();
    await resize(890);
    const wide = await measureHintRow();
    await verify(hint, "UXH04 hint is hidden when there is no overflow").toBeHidden();
    verify(narrow.overflow, "UXH04 narrow viewport has real overflow").toBeGreaterThan(1);
    verify(wide.overflow, "UXH04 wider viewport removes real overflow").toBeLessThanOrEqual(1);
    verify(narrow.reserved, "UXH04 overflow hint has a measurable reserved row").toBeGreaterThan(0);
    verify(Math.abs(wide.reserved - narrow.reserved), "UXH04 hint row stays reserved when overflow disappears").toBeLessThanOrEqual(1);
    await resize(390);
    const narrowAgain = await measureHintRow();
    await verify(hint, "UXH04 hint returns with overflow").toBeVisible();
    verify(Math.abs(narrowAgain.reserved - wide.reserved), "UXH04 hint row stays reserved when overflow returns").toBeLessThanOrEqual(1);
    console.log("UXH04", JSON.stringify({ locale, narrow, wide, narrowAgain }));

    // Select on desktop before the responsive layout introduces horizontal
    // overflow. No mobile tab click or keyboard focus may scroll it into view.
    await resize(901);
    const seo = nav.locator('button[aria-controls="seo"]');
    await seo.click();
    await expect(seo, "UXH05 desktop click selects SEO before resize").toHaveAttribute("aria-current", "location");
    await resize(390);
    const measureSelected = () => seo.evaluate(el => {
      const strip = el.closest('[data-testid="product-section-tabs"]')!;
      const tab = el.getBoundingClientRect(), frame = strip.getBoundingClientRect();
      return { tabLeft: tab.left, tabRight: tab.right, tabWidth: tab.width,
        frameLeft: frame.left, frameRight: frame.right, frameWidth: frame.width,
        overflow: strip.scrollWidth - strip.clientWidth, scrollLeft: strip.scrollLeft,
        fullyVisible: tab.width > 0 && tab.left >= frame.left - 1 && tab.right <= frame.right + 1 };
    }); // READ/MEASURE only: native desktop selection and viewport resize drive the flow.
    const resizedSelected = await measureSelected();
    expect(resizedSelected.overflow, "UXH05 mobile strip has real horizontal overflow").toBeGreaterThan(1);
    expect(resizedSelected.tabWidth, "UXH05 selected tab fits inside the mobile strip").toBeLessThanOrEqual(resizedSelected.frameWidth);
    await verify(seo, "UXH05 resize preserves the selected SEO section").toHaveAttribute("aria-current", "location");
    await verify.poll(async () => (await measureSelected()).fullyVisible,
      { message: "UXH05 desktop-selected SEO is fully visible after 901→390 resize" }).toBe(true);
    console.log("UXH05", JSON.stringify({ locale, direction: "901→390", ...await measureSelected() }));
    seen.add(locale);
    hardenedLocales.set(page, seen);
  } finally {
    if (await name.inputValue() !== originalName) await name.fill(originalName);
    await resize(900);
    await setExpanded(originalExpanded);
    await resize(originalViewport.width);
    await expect(name).toHaveValue(originalName);
  }
}

// Runtime acceptance, not a style/threshold test. evaluate only reads geometry
// and focus; section opening and scrolling are driven by actual nav clicks.
// Nested scroll panes require a viewport capture per section: a full-page image
// cannot reveal content outside the pane's current scroll position.
/** Clicks every available section and checks that focus remains inside its visible scroll pane. */
export async function assertProductEditorReservedLayout(
  page: Page,
  capture?: (section: string) => Promise<void>,
) {
  const form = page.locator("form.pe-document");
  const pane = form.getByTestId("product-fields");
  const footer = form.locator(".pe-savebar");
  const buttons = form.locator(".pe-index nav button[aria-controls]");
  const readiness = form.locator(".pe-readiness");
  const statusBefore = await readiness.locator("[data-ready]").evaluateAll((nodes) => nodes.map((node) => node.getAttribute("data-ready")));
  await expect(pane).toBeVisible();
  expect(await buttons.count()).toBeGreaterThanOrEqual(6);
  const assertReserved = async () => {
    await expect
      .poll(
        async () =>
          pane.evaluate((element) => {
            const save = element.closest("form")!.querySelector(".pe-savebar")!;
            const area = element.getBoundingClientRect();
            const bar = save.getBoundingClientRect();
            const viewport = window.visualViewport;
            const top = viewport?.offsetTop ?? 0;
            const bottom = top + (viewport?.height ?? innerHeight);
            return (
              element.clientHeight > 0 &&
              element.scrollHeight > element.clientHeight &&
              area.bottom <= bar.top &&
              bar.top >= top &&
              bar.bottom <= bottom + 1
            );
          }),
        "scrollable fields and visible savebar occupy separate reserved rows",
      )
      .toBe(true);
    await expect(footer.getByTestId("product-publish")).toBeVisible();
    const viewport = page.viewportSize()!;
    if (viewport.width <= 900) {
      const nav = await form.locator(".pe-index").boundingBox();
      const area = await pane.boundingBox();
      expect(nav!.y + nav!.height).toBeLessThanOrEqual(area!.y);
    }
  };
  await assertReserved();
  await assertProductEditorHardening(page);
  if (page.viewportSize()!.width <= 390) {
    const locale = new URL(page.url()).pathname.split("/")[1] as keyof typeof presentationCopy;
    const tabs = form.getByTestId("product-section-tabs");
    await expect(tabs).toBeVisible();
    const hint = tabs.locator("..").getByText(`${presentationCopy[locale].previous} · ${presentationCopy[locale].next}`, { exact: true });
    await expect(hint).toBeVisible();
    // READ/MEASURE: overflow has a visible hint and the hint is outside the horizontal clip.
    expect(await hint.evaluate(el => {
      const r = el.getBoundingClientRect();
      const parent = el.parentElement!.getBoundingClientRect();
      return r.left >= parent.left && r.right <= parent.right && r.width > 0;
    })).toBe(true);
    const status = footer.locator(":scope > span");
    if ((await status.innerText()).trim()) {
      // READ/MEASURE: a localized status row cannot share the actions' baseline or a 90px cap.
      expect(await status.evaluate(el => {
        const label = el.getBoundingClientRect();
        const actions = el.nextElementSibling!.getBoundingClientRect();
        return label.bottom <= actions.top && label.width >= actions.width;
      })).toBe(true);
    }
  }
  // Diagnostic companion, NOT a replacement for frozen R6/R9. Read the actual
  // clip and native hit-test result; retain any frozen raw-rectangle failure.
  const clippedBounds = await form.evaluate((element) => {
    const nav = element.querySelector(".pe-index nav")!;
    const fields = element.querySelector(".pe-fields")!;
    const bar = element.querySelector(".pe-savebar")!.getBoundingClientRect();
    const n = nav.getBoundingClientRect(), p = fields.getBoundingClientRect();
    const navTargets = [...nav.querySelectorAll("button")].map((button) => {
      const r = button.getBoundingClientRect();
      return { label: button.textContent, rawLeft: r.left, rawRight: r.right,
        visibleLeft: Math.max(n.left, r.left), visibleRight: Math.min(n.right, r.right) };
    }).filter((r) => r.visibleRight > r.visibleLeft);
    const rawTails = [...fields.querySelectorAll("button,input,select,textarea")].flatMap((control) => {
      const r = control.getBoundingClientRect();
      const top = Math.max(r.top, bar.top), bottom = Math.min(r.bottom, bar.bottom);
      if (r.top >= p.bottom || top >= bottom || r.width === 0) return [];
      const hit = document.elementFromPoint(Math.max(p.left, r.left) + 1, (top + bottom) / 2);
      return [{ id: control.id || control.getAttribute("data-testid"), rawBottom: r.bottom,
        paneBottom: p.bottom, barTop: bar.top, clippedTailHittable: !!hit && control.contains(hit) }];
    });
    return { locale: document.documentElement.lang, width: innerWidth,
      navLeft: n.left, navRight: n.right, navTargets, rawTails };
  });
  for (const tail of clippedBounds.rawTails) expect(tail.clippedTailHittable).toBe(false);
  if (clippedBounds.width <= 390) for (const target of clippedBounds.navTargets) {
    expect(target.visibleLeft).toBeGreaterThanOrEqual(8);
    expect(target.visibleRight).toBeLessThanOrEqual(clippedBounds.width - 8);
  }
  console.log("EDITOR_VISIBLE_CLIP_DIAGNOSTIC", JSON.stringify(clippedBounds));
  // Iterate the controls that actually exist (single-SKU has a pricing section,
  // matrix mode does not). No fixture DOM mutation or synthetic event dispatch.
  const ids = await buttons.evaluateAll((nodes) =>
    nodes.map((n) => n.getAttribute("aria-controls")!),
  );
  for (const id of ids) {
    const button = form.locator(`.pe-index nav button[aria-controls="${id}"]`);
    await button.click();
    await expect(button).toHaveAttribute("aria-current", "location");
    for (const item of await readiness.all()) {
      const targetID = await item.getAttribute("aria-controls");
      expect(targetID).toBeTruthy();
      if (targetID === id) await expect(item).toHaveAttribute("aria-current", "location");
      else await expect(item).not.toHaveAttribute("aria-current");
    }
    const section = pane.locator(`[id="${id}"]`);
    await expect
      .poll(
        () =>
          section.evaluate((element) => {
            const root = element.closest(".pe-fields")!;
            const area = root.getBoundingClientRect();
            const target = element.getBoundingClientRect();
            const focused = document.activeElement;
            if (!(focused instanceof HTMLElement) || !element.contains(focused))
              return false;
            const field = focused.getBoundingClientRect();
            return (
              target.bottom > area.top &&
              target.top < area.bottom &&
              field.top >= area.top &&
              field.bottom <= area.bottom
            );
          }),
        `${id} actual focused control is inside the scroll pane, not under footer`,
      )
      .toBe(true);
    await assertReserved();
    if (capture) await capture(id);
  }
  if (page.viewportSize()!.width > 900) {
    const media = form.locator('.pe-readiness[aria-controls="media"]').first();
    await media.click();
    await expect(media).toHaveAttribute("aria-current", "location");
    await expect.poll(() => pane.locator("#media").evaluate((element) => element.contains(document.activeElement))).toBe(true);
    await pane.hover();
    const scrollDistance = await pane.locator("#media").evaluate((element) => Math.ceil(element.getBoundingClientRect().height + innerHeight));
    await page.mouse.wheel(0, scrollDistance);
    await expect(form.locator('.pe-index nav button[aria-current="location"]')).toHaveCount(1);
    await expect.poll(() => form.locator('.pe-index nav button[aria-current="location"]').getAttribute("aria-controls")).not.toBe("media");
    const currentSection = await form.locator('.pe-index nav button[aria-current="location"]').getAttribute("aria-controls");
    expect(currentSection).not.toBe("media");
    for (const item of await readiness.all()) {
      if (await item.getAttribute("aria-controls") === currentSection) await expect(item).toHaveAttribute("aria-current", "location");
      else await expect(item).not.toHaveAttribute("aria-current");
    }
    await media.click();
    await expect(media).toHaveAttribute("aria-current", "location");
    // A second click in the current section remains a real, idempotent focus jump.
    await media.click();
    await expect(media).toHaveAttribute("aria-current", "location");
    await expect.poll(() => pane.locator("#media").evaluate((element) => element.contains(document.activeElement))).toBe(true);
  }
  await pane.hover();
  const endDistance = await pane.evaluate(el => el.scrollHeight + el.clientHeight); // READ/MEASURE
  await page.mouse.wheel(0, endDistance);
  await expect.poll(() => pane.evaluate(el => el.scrollHeight - el.clientHeight - el.scrollTop)).toBeLessThanOrEqual(2);
  await expect(pane.locator("#seo").locator("input,textarea").last()).toBeInViewport();
  if (capture) await capture("scroll-end");
  expect(await readiness.locator("[data-ready]").evaluateAll((nodes) => nodes.map((node) => node.getAttribute("data-ready")))).toEqual(statusBefore);
}
