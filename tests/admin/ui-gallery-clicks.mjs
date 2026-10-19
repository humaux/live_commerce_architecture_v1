// Purpose: DS-1 regression journey plus the DS-2 six-case component/template matrix; real input, visible outcomes and persistence.
// Depends on: Playwright, axe, visual-lint, gallery copy/navigation, node:assert/fs/crypto. evaluate reads except labelled synthetic fixture resets.
// Used by: tests/admin/ui-gallery-runner.mjs (--browser-ui-gallery).
import assert from "node:assert/strict";
import { localeNames, locales } from "../../packages/i18n/src/index.ts";
import { galleryNav } from "../../apps/admin/src/ui-gallery-nav.ts";

/** Runs the whole journey once for one locale and viewport; `step(control, action, expected, fn)` records a ledger row. */
export async function clickJourney({ page, step, copy: c, locale, mobile, ds2Nested = false }) {
  const sec = (id) => page.locator(`#${id}`);
  const seen = (loc) => loc.filter({ visible: true }).first().waitFor({ state: "visible" });
  const status = (scope, text) => scope.locator('[role="status"]').filter({ hasText: text });
  const btn = (scope, name) => scope.getByRole("button", { name, exact: true }).filter({ visible: true }).first();
  const attr = async (loc, name) => loc.first().getAttribute(name); // [READ] attribute only
  // Playwright's actionability check treats aria-disabled="true" as "not enabled" and waits forever, but a user can click such a button (it explains itself).
  // So a blocked control gets a real mouse click at its centre; the pointer still hits whatever is on top, so an overlay would still fail it.
  const clickBlocked = async (loc) => {
    await loc.scrollIntoViewIfNeeded();
    const box = await loc.boundingBox(); // [MEASURE] position only
    assert.ok(box, "blocked control has a box");
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  };
  const nav = galleryNav(locale);
  const group = (id) => nav.find((g) => g.id === id);
  const drawer = page.getByRole("navigation", { name: c.navLabel });
  const openDrawer = async () => {
    if (!mobile || (await drawer.isVisible())) return;
    await page.getByRole("button", { name: c.menu, exact: true }).click();
    await drawer.waitFor({ state: "visible" });
  };

  /* ---- A. controls that act in place ---- */
  const buttons = sec("sec-buttons");
  for (const [label, name] of [["primary", c.btnPrimary], ["secondary", c.btnSecondary], ["danger", c.btnDanger], ["link", c.btnLink], ["small", c.btnSmall], ["large", c.btnLarge]])
    await step(`Button ${label}`, "click", `status line shows "${name}"`, async () => {
      await btn(buttons, name).click();
      await seen(buttons.locator('p[role="status"]').filter({ hasText: name }));
    });
  await step("Button disabled with reason", "click", "reason becomes visible, aria-disabled stays true, no other effect", async () => {
    const b = btn(buttons, c.btnDisabled);
    assert.equal(await attr(b, "aria-disabled"), "true");
    const contrast = await b.evaluate(n => { // [READ] actual computed disabled label/background, never style injection
      const style = getComputedStyle(n);
      const luminance = color => {
        const [r, g, b] = color.match(/[\d.]+/g).slice(0, 3).map(v => {
          const channel = Number(v) / 255;
          return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
        });
        return 0.2126 * r + 0.7152 * g + 0.0722 * b;
      };
      const [hi, lo] = [luminance(style.color), luminance(style.backgroundColor)].sort((a, b) => b - a);
      return { ratio: (hi + 0.05) / (lo + 0.05), opacity: Number(style.opacity) };
    });
    assert.ok(contrast.ratio >= 3, `disabled label contrast ${contrast.ratio}`);
    assert.equal(contrast.opacity, 1);
    await clickBlocked(b);
    await seen(status(buttons, c.btnDisabledReason));
  });
  await step("Button disabled with reason (keyboard)", "focus, Enter, Space", "still aria-disabled, reason visible; Playwright's enabled-wait is bypassed by real key presses", async () => {
    const b = btn(buttons, c.btnDisabled);
    await b.focus();
    for (const key of ["Enter", "Space"]) {
      await page.keyboard.press(key);
      assert.equal(await attr(b, "aria-disabled"), "true");
      await seen(status(buttons, c.btnDisabledReason));
    }
  });
  await step("Button loading", "read state", "aria-busy true (a busy button swallows clicks by design)", async () => {
    assert.equal(await attr(buttons.getByRole("button", { name: c.btnLoading }), "aria-busy"), "true");
  });

  const alerts = sec("sec-alerts");
  await step("Alert action (warning)", "click", `status shows "${c.clicked(c.alertAction)}"`, async () => {
    await btn(alerts, c.alertAction).click();
    await seen(status(alerts, c.clicked(c.alertAction)));
  });
  await step("Alert dismiss (info)", "click", "the dismissed alert disappears", async () => {
    await btn(alerts, c.alertDismiss).click();
    await alerts.getByRole("button", { name: c.alertDismiss }).waitFor({ state: "detached" });
  });

  const states = sec("sec-states");
  await step("ErrorState retry", "click", "retry result text visible", async () => {
    await btn(states, c.retry).click();
    await seen(status(states, c.retryDone));
  });
  await step("EmptyState action", "click", `status shows "${c.clicked(c.emptyAction)}"`, async () => {
    await btn(states, c.emptyAction).click();
    await seen(status(states, c.clicked(c.emptyAction)));
  });

  const drop = sec("sec-dropzone");
  await step("Dropzone hint", "read", "format and size hint is on screen before any upload", async () => seen(drop.getByText(c.dzHint)));
  const choose = async (file) => {
    const [chooser] = await Promise.all([page.waitForEvent("filechooser"), btn(drop, c.dzBrowse).click()]);
    await chooser.setFiles(file);
  };
  await step("Dropzone add images (wrong format)", "click, choose bad.gif", `alert "bad.gif: ${c.dzErrType}"`, async () => {
    await choose({ name: "bad.gif", mimeType: "image/gif", buffer: Buffer.from("x") });
    await seen(drop.getByRole("alert").filter({ hasText: `bad.gif: ${c.dzErrType}` }));
  });
  await step("Dropzone add images (too large)", "click, choose 5 MB + 1 file", `alert "big.png: ${c.dzErrSize}"`, async () => {
    await choose({ name: "big.png", mimeType: "image/png", buffer: Buffer.alloc(5 * 1024 * 1024 + 1) });
    await seen(drop.getByRole("alert").filter({ hasText: `big.png: ${c.dzErrSize}` }));
  });
  await step("Dropzone add images (accepted)", "click, choose ok.png", `status "${c.dzAccepted("ok.png")}"`, async () => {
    await choose({ name: "ok.png", mimeType: "image/png", buffer: Buffer.from("x") });
    await seen(status(drop, c.dzAccepted("ok.png")));
  });
  await step("Dropzone choose from library", "click", "library status visible", async () => {
    await btn(drop, c.dzLibrary).click();
    await seen(status(drop, c.dzLibraryClicked));
  });

  const form = sec("sec-form");
  await step("FormSection help ?", "click", "help text opens, aria-expanded true", async () => {
    const help = btn(form, c.formHelpLabel);
    await help.click();
    assert.equal(await attr(help, "aria-expanded"), "true");
    await seen(form.getByText(c.formHelp));
  });
  await step("Save bar Update (nothing to save)", "click", "reason visible, nothing saved", async () => {
    await clickBlocked(btn(form, c.save));
    await seen(status(form, c.saveReason));
  });
  await step("Product name field", "fill 'abc'", `dirty note and counter ${c.formCounter(3)}`, async () => {
    await form.getByLabel(c.nameLabel).fill("abc");
    await seen(form.getByText(c.formDirty));
    await seen(form.getByText(c.formCounter(3)));
  });
  await step("Save bar Update", "click", `"${c.saved}" visible, dirty note gone`, async () => {
    await btn(form, c.save).click();
    await seen(status(form, c.saved));
    assert.equal(await form.getByText(c.formDirty).filter({ visible: true }).count(), 0);
  });
  await step("Save bar Cancel", "fill 'abcd', click Cancel", "field returns to the saved value", async () => {
    const input = form.getByLabel(c.nameLabel);
    await input.fill("abcd");
    await btn(form, c.cancel).click();
    assert.equal(await input.inputValue(), "abc");
  });
  await step("Save bars placement", "read", mobile ? "only the bottom bar is visible (sticky)" : "top and bottom bars both visible", async () => {
    // getByRole leaves out display:none elements, so on a phone it would never see the hidden top bar: select by attribute to count both
    const bars = form.locator(`[role="group"][aria-label="${c.barLabel}"]`);
    assert.equal(await bars.count(), 2);
    assert.equal(await bars.first().isVisible(), !mobile);
    assert.equal(await bars.last().isVisible(), true);
  });

  const dialogDemo = sec("sec-dialog");
  const dialog = page.getByRole("dialog");
  await step("ConfirmDialog cancel", "open, click Cancel", `"${c.dlgCancelled}"`, async () => {
    await btn(dialogDemo, c.dlgOpenPlain).click();
    await seen(dialog.getByText(c.dlgTitle));
    await btn(dialog, c.cancel).click();
    await seen(status(dialogDemo, c.dlgCancelled));
  });
  await step("ConfirmDialog typed confirm", "open, click Delete (blocked), type word, click Delete", `blocked with reason, then "${c.dlgDone}"`, async () => {
    await btn(dialogDemo, c.dlgOpenTyped).click();
    await clickBlocked(btn(dialog, c.dlgConfirm));
    await seen(status(dialog, c.dlgMismatch));
    await dialog.getByRole("textbox").fill(c.dlgPhrase);
    await btn(dialog, c.dlgConfirm).click();
    await seen(status(dialogDemo, c.dlgDone));
  });

  await step("ConfirmDialog busy without busyLabel", "open, click Cancel, Enter, Space, Escape", "dialog stays open (no cancel result) while busy; Cancel works once the request settles", async () => {
    await btn(dialogDemo, c.dlgOpenBusy).click();
    await seen(dialog.getByText(c.dlgTitle));
    const cancel = btn(dialog, c.cancel);
    await cancel.click(); // busy without busyLabel is not aria-disabled, so a plain click is the real user path
    for (const key of ["Enter", "Space", "Escape"]) {
      if (key !== "Escape") await cancel.focus();
      await page.keyboard.press(key);
    }
    assert.equal(await dialog.isVisible(), true);
    assert.equal(await status(dialogDemo, c.dlgCancelled).filter({ visible: true }).count(), 0);
    // after the 5 s busy window the same Cancel closes the dialog
    for (let i = 0; i < 30 && (await dialog.isVisible()); i++) {
      await cancel.click();
      await page.waitForTimeout(300);
    }
    await seen(status(dialogDemo, c.dlgCancelled));
  });

  /* ---- B. the list: filter, selection scope, bulk, row actions, states ---- */
  const table = sec("sec-table");
  if (!mobile)
    await step("Table fits the desktop card", "read", "no horizontal scroll; the actions column is fully visible", async () => {
      // [READ/MEASURE] layout only: a desktop list must show every column, actions included, without scrolling the table frame.
      const fit = await table.locator("table").evaluate((t) => {
        const frame = t.parentElement;
        const head = t.querySelector("th[data-actions]");
        const f = frame.getBoundingClientRect();
        const h = head ? head.getBoundingClientRect() : null;
        return { overflow: frame.scrollWidth - frame.clientWidth, actionsInside: !!h && h.right <= f.right + 1 && h.left >= f.left - 1 };
      });
      assert.ok(fit.overflow <= 1, `table frame scrolls by ${fit.overflow}px at desktop width`);
      assert.equal(fit.actionsInside, true, "actions column header is inside the visible frame");
    });
  const row = (n) => table.getByRole("row").filter({ hasText: c.productName(n) });
  const note = (text) => seen(table.locator('p[role="status"]').filter({ hasText: text }));
  await step("Bulk menu with nothing selected", "click", "disabled with reason (aria-disabled, reason visible)", async () => {
    const bulk = btn(table, c.bulk);
    assert.equal(await attr(bulk, "aria-disabled"), "true");
    await clickBlocked(bulk);
    await seen(status(table, c.bulkNone));
  });
  await step("Header checkbox", "check", `${c.selected(5)} and the all-results offer ${c.selectAll(47)}`, async () => {
    await table.getByRole("checkbox", { name: c.selectPage }).check();
    await seen(table.getByText(c.selected(5), { exact: true }));
    await seen(btn(table, c.selectAll(47)));
  });
  await step("Select all results", "click", `${c.selected(47)} / ${c.allResults(47)}`, async () => {
    await btn(table, c.selectAll(47)).click();
    await seen(table.getByText(c.allResults(47), { exact: true }));
    await seen(table.getByText(c.selected(47), { exact: true }));
  });
  await step("Bulk delete over all results", "menu, Delete selected, Delete (blocked), type word, Delete", `typed confirm; "${c.bulkDone(47, c.scopeFilter)}"`, async () => {
    await btn(table, c.bulk).click();
    await btn(table, c.bulkDelete).click();
    await seen(dialog.getByText(c.dlgTypedTitle));
    await clickBlocked(btn(dialog, c.dlgConfirm));
    await seen(status(dialog, c.dlgMismatch));
    await dialog.getByRole("textbox").fill(c.dlgPhrase);
    await btn(dialog, c.dlgConfirm).click();
    await seen(table.getByText(c.bulkDone(47, c.scopeFilter)));
    assert.equal(await table.getByText(c.selected(47), { exact: true }).count(), 0);
  });
  await step("Row checkbox + bulk unpublish", "check row 2, menu, Unpublish selected", `note "${c.lastAction(c.bulkUnpublish, c.selected(1))}"`, async () => {
    await table.getByRole("checkbox", { name: c.selectRow(2) }).check();
    await seen(table.getByText(c.selected(1), { exact: true }));
    await btn(table, c.bulk).click();
    await btn(table, c.bulkUnpublish).click();
    await note(c.lastAction(c.bulkUnpublish, c.selected(1)));
  });
  await step("Clear selection", "click", "selection banner gone", async () => {
    await btn(table, c.clear).click();
    await table.getByText(c.selected(1), { exact: true }).waitFor({ state: "detached" });
  });
  await step("Row primary action (draft row 3)", "click Publish", `note "${c.lastAction(c.publish, c.productName(3))}"`, async () => {
    await btn(row(3), c.publish).click();
    await note(c.lastAction(c.publish, c.productName(3)));
  });
  await step("Row secondary action (published row 1)", "click Unpublish", `note "${c.lastAction(c.unpublish, c.productName(1))}"`, async () => {
    await btn(row(1), c.unpublish).click();
    await note(c.lastAction(c.unpublish, c.productName(1)));
  });
  await step("Max one primary per row", "read", "no row has more than one primary button", async () => {
    const counts = await table.locator("tbody tr").evaluateAll((rows) => rows.map((r) => r.querySelectorAll('[data-variant="primary"]').length)); // [READ/MEASURE]
    assert.ok(counts.length >= 5 && counts.every((n) => n <= 1), JSON.stringify(counts));
  });
  await step("Row More menu: Duplicate", "click More, click Duplicate", `note "${c.lastAction(c.duplicate, c.productName(1))}"`, async () => {
    await btn(row(1), c.more).click();
    await btn(row(1), c.duplicate).click();
    await note(c.lastAction(c.duplicate, c.productName(1)));
  });
  await step("Row More menu: Delete then Escape", "click More, Delete, press Escape", "dialog opens and Escape closes it without deleting", async () => {
    await btn(row(1), c.more).click();
    await btn(row(1), c.del).click();
    await seen(dialog.getByText(c.dlgTitle));
    await page.keyboard.press("Escape");
    await dialog.waitFor({ state: "detached" });
  });
  await step("Row More menu: Delete then confirm", "click More, Delete, Delete", `note "${c.lastAction(c.del, c.productName(1))}"`, async () => {
    await btn(row(1), c.more).click();
    await btn(row(1), c.del).click();
    await btn(dialog, c.dlgConfirm).click();
    await note(c.lastAction(c.del, c.productName(1)));
  });
  await step("Search box + button", "fill '7', click Search", `${c.pagerSummary(1, 1, 5)} and note`, async () => {
    await table.getByRole("searchbox", { name: c.searchLabel }).fill("7");
    await btn(table, c.searchButton).click();
    await note(c.searchResult("7"));
    assert.equal(new URL(page.url()).searchParams.has("q"), false);
    assert.equal(new URL(page.url()).searchParams.has("pq"), false);
    await seen(table.getByText(c.pagerSummary(1, 1, 5)));
  });
  await step("Search box Enter key", "clear, press Enter", c.pagerSummary(1, 10, 47), async () => {
    const box = table.getByRole("searchbox", { name: c.searchLabel });
    await box.fill("");
    await box.press("Enter");
    await seen(table.getByText(c.pagerSummary(1, 10, 47)));
  });
  await step("Filter select (Published)", "selectOption", c.pagerSummary(1, 7, 32), async () => {
    await table.getByLabel(c.filterShow).selectOption("published");
    await seen(table.getByText(c.pagerSummary(1, 7, 32)));
  });
  await step("Filter select (All)", "selectOption", c.pagerSummary(1, 10, 47), async () => {
    await table.getByLabel(c.filterShow).selectOption("all");
    await seen(table.getByText(c.pagerSummary(1, 10, 47)));
  });
  await step("Filter survives paging", "select Published, click Next, reload, click Previous", `${c.pagerSummary(2, 7, 32)} after Next and reload, ${c.pagerSummary(1, 7, 32)} after Previous; URL keeps pf=published`, async () => {
    await table.getByLabel(c.filterShow).selectOption("published");
    await seen(table.getByText(c.pagerSummary(1, 7, 32)));
    await table.getByRole("link", { name: c.next, exact: true }).click();
    await page.waitForURL(/page=2/);
    await seen(table.getByText(c.pagerSummary(2, 7, 32)));
    assert.match(page.url(), /pf=published/);
    await page.reload({ waitUntil: "load" });
    await seen(table.getByText(c.pagerSummary(2, 7, 32)));
    assert.equal(await table.getByLabel(c.filterShow).inputValue(), "published");
    await table.getByRole("link", { name: c.prev, exact: true }).click();
    await seen(table.getByText(c.pagerSummary(1, 7, 32)));
    assert.equal(await table.getByLabel(c.filterShow).inputValue(), "published");
    await table.getByLabel(c.filterShow).selectOption("all");
    await seen(table.getByText(c.pagerSummary(1, 10, 47)));
  });
  await step("Table state: loading", "selectOption, reload", "loading row (status) and the choice survives a reload", async () => {
    await table.getByLabel(c.stateLabel).selectOption("loading");
    await seen(status(table, c.loadingLabel));
    await page.reload({ waitUntil: "load" });
    assert.equal(await table.getByLabel(c.stateLabel).inputValue(), "loading");
    await seen(status(table, c.loadingLabel));
  });
  await step("Table state: error + retry", "selectOption, click Try again", "error alert with retry; retry returns to the rows", async () => {
    await table.getByLabel(c.stateLabel).selectOption("error");
    await seen(table.getByRole("alert").filter({ hasText: c.errorTitle }));
    await btn(table, c.retry).click();
    await seen(table.getByText(c.productName(1)));
  });
  await step("Table state: empty", "selectOption, click empty action", `empty state; note "${c.clicked(c.emptyAction)}"`, async () => {
    await table.getByLabel(c.stateLabel).selectOption("empty");
    await seen(table.getByText(c.emptyTitle));
    await btn(table, c.emptyAction).click();
    await note(c.clicked(c.emptyAction));
    await table.getByLabel(c.stateLabel).selectOption("ready");
    await seen(table.getByText(c.productName(1)));
  });

  /* ---- C. state kept in the URL: switch, tabs ---- */
  const switchBox = sec("sec-switch");
  const sw = page.getByRole("switch", { name: c.swLabel });
  await step("SegmentedSwitch", "click, reload", `${c.swResult(false)} and it persists`, async () => {
    await sw.click();
    await seen(status(switchBox, c.swResult(false)));
    await page.reload({ waitUntil: "load" });
    assert.equal(await attr(sw, "aria-checked"), "false");
    await seen(status(switchBox, c.swResult(false)));
    await sw.click();
    await seen(status(switchBox, c.swResult(true)));
  });
  await step("Tabs (underline)", "click tab, ArrowRight, reload", "panel follows, arrow key moves, choice survives reload", async () => {
    const tab = (label) => switchBox.getByRole("tab", { name: label });
    await tab(new RegExp(`^${c.tabB}`)).click();
    await seen(switchBox.getByText(c.panelText(c.tabB)));
    await page.keyboard.press("ArrowRight");
    await seen(switchBox.getByText(c.panelText(c.tabC)));
    await page.reload({ waitUntil: "load" });
    assert.equal(await attr(tab(c.tabC), "aria-selected"), "true");
  });
  await step("Tabs (section)", "click tab, End key, reload", "panel follows, End selects the last, choice survives reload", async () => {
    const tab = (label) => switchBox.getByRole("tab", { name: label, exact: true });
    await tab(c.stabB).click();
    await seen(switchBox.getByText(c.panelText(c.stabB)));
    await page.keyboard.press("End");
    await seen(switchBox.getByText(c.panelText(c.stabC)));
    await page.reload({ waitUntil: "load" });
    assert.equal(await attr(tab(c.stabC), "aria-selected"), "true");
  });

  /* ---- D. links that navigate (state is in the URL, so every one is checked after a reload) ---- */
  await step("Pagination next / page 2 link", "click page 2, reload", `${c.pagerSummary(2, 10, 47)}, ${c.productName(6)}, aria-current page, survives reload`, async () => {
    await table.getByRole("link", { name: c.pageN(2), exact: true }).click();
    await page.waitForURL(/page=2/);
    await seen(table.getByText(c.productName(6)));
    await page.reload({ waitUntil: "load" });
    await seen(table.getByText(c.pagerSummary(2, 10, 47)));
    assert.equal(await attr(table.getByRole("link", { name: c.pageN(2), exact: true }), "aria-current"), "page");
  });
  await step("Pagination previous / next arrows", "click next, click previous", "page 3 then page 2; previous is disabled on page 1", async () => {
    await table.getByRole("link", { name: c.next, exact: true }).click();
    await page.waitForURL(/page=3/);
    await seen(table.getByText(c.pagerSummary(3, 10, 47)));
    await table.getByRole("link", { name: c.prev, exact: true }).click();
    await page.waitForURL(/page=2/);
    await table.getByRole("link", { name: c.pageN(1), exact: true }).click();
    await page.waitForFunction(() => !location.search.includes("page=")); // [READ] URL only
    // a disabled arrow has no href, so it is not a link role: find it by its label
    assert.equal(await attr(table.getByLabel(c.prev, { exact: true }), "aria-disabled"), "true");
  });
  await step("Row Edit link", "click Edit on row 2", "URL carries ?edit=2", async () => {
    await row(2).getByRole("link", { name: c.edit, exact: true }).click();
    await page.waitForURL(/edit=2/);
  });
  const kpi = sec("sec-kpi");
  await step("KPI card (clickable)", "click, reload", `${c.kpiOpened(c.kpiUnshipped)} and it persists`, async () => {
    await kpi.getByRole("link", { name: new RegExp(c.kpiUnshipped) }).click();
    await page.waitForURL(/list=unshipped/);
    await seen(status(kpi, c.kpiOpened(c.kpiUnshipped)));
    await page.reload({ waitUntil: "load" });
    await seen(status(kpi, c.kpiOpened(c.kpiUnshipped)));
  });
  const settings = sec("sec-settings");
  await step("SettingsLayout sub-nav", "click Shipping, reload", `${c.setBodyText(c.setItemShipping)}, aria-current, survives reload`, async () => {
    await settings.getByRole("link", { name: c.setItemShipping, exact: true }).click();
    await page.waitForURL(/setting=shipping/);
    await page.reload({ waitUntil: "load" });
    await seen(settings.getByText(c.setBodyText(c.setItemShipping)));
    assert.equal(await attr(settings.getByRole("link", { name: c.setItemShipping, exact: true }), "aria-current"), "page");
  });

  /* ---- E. the v2 shell ---- */
  if (mobile) {
    await step("Shell menu button (< 768 px)", "click, Tab x12, Escape", "drawer opens, focus stays inside it, Escape closes it", async () => {
      await openDrawer();
      assert.equal(await attr(page.getByRole("button", { name: c.menu, exact: true }), "aria-expanded"), "true");
      for (let i = 0; i < 12; i++) await page.keyboard.press("Tab");
      assert.equal(await drawer.evaluate((n) => n.contains(document.activeElement)), true); // [READ] focus containment
      await page.keyboard.press("Escape");
      await drawer.waitFor({ state: "hidden" });
    });
    await step("Shell drawer close button", "open, click close", "drawer hidden", async () => {
      await openDrawer();
      await drawer.getByRole("button", { name: c.close, exact: true }).click();
      await drawer.waitFor({ state: "hidden" });
    });
    await step("Shell drawer backdrop", "open, click outside the drawer", "drawer hidden", async () => {
      await openDrawer();
      await page.getByRole("button", { name: c.close, exact: true }).first().click({ position: { x: 380, y: 420 } });
      await drawer.waitFor({ state: "hidden" });
    });
  }
  await step("Shell group (collapsible second level)", "click Orders, click again", "expands then collapses; sub pages appear and disappear", async () => {
    await openDrawer();
    const orders = btn(drawer, group("orders").label);
    assert.equal(await attr(orders, "aria-expanded"), "false");
    await orders.click();
    assert.equal(await attr(orders, "aria-expanded"), "true");
    const chevron = orders.locator('[data-nav-chevron]');
    const chevronSize = await chevron.evaluate(n => [getComputedStyle(n).width, getComputedStyle(n).height, getComputedStyle(n).flexBasis]); // [READ] slot size, unaffected by rotation
    assert.deepEqual(chevronSize, ["16px", "16px", "16px"]);
    assert.equal(await chevron.locator("svg").getAttribute("width"), "16");
    await page.waitForFunction(el => Math.abs(new DOMMatrix(getComputedStyle(el).transform).b - 1) < 0.01, await chevron.elementHandle()); // [READ] wait for rotation
    // [READ] drawn chevron extent (path box after rotation): at least 3/4 of the 16px icon box, never a 9px sliver beside 14px text.
    const ink = await chevron.locator("path").evaluate(n => { const r = n.getBoundingClientRect(); return Math.max(r.width, r.height); });
    assert.ok(ink >= 12 && ink <= 16, `chevron ink ${ink}px outside [12, 16] of the 16px icon box`);
    assert.equal(await orders.evaluate((n) => n === document.activeElement), true); // [READ] re-render must not move focus (drawer effect stays mounted)
    await seen(drawer.getByRole("link", { name: group("orders").items[0].label, exact: true }));
    await orders.click();
    assert.equal(await attr(orders, "aria-expanded"), "false");
    await page.waitForFunction(el => Math.abs(new DOMMatrix(getComputedStyle(el).transform).b) < 0.01, await chevron.elementHandle()); // [READ]
    await drawer.getByRole("link", { name: group("orders").items[0].label, exact: true }).waitFor({ state: "detached" });
  });
  await step("Shell second-level link", "click Orders, click Returns, reload", "URL ?nav=returns; after reload the group is open and the link is current", async () => {
    const orders = btn(drawer, group("orders").label);
    if ((await attr(orders, "aria-expanded")) !== "true") await orders.click();
    const returns = group("orders").items.find((i) => i.id === "returns");
    await drawer.getByRole("link", { name: returns.label, exact: true }).click();
    await page.waitForURL(/nav=returns/);
    await page.reload({ waitUntil: "load" });
    await openDrawer();
    assert.equal(await attr(btn(drawer, group("orders").label), "aria-expanded"), "true");
    assert.equal(await attr(drawer.getByRole("link", { name: returns.label, exact: true }), "aria-current"), "page");
  });
  await step("Shell top-level link (Workbench)", "click", "URL ?nav=home, link is current after navigation", async () => {
    await openDrawer();
    await drawer.getByRole("link", { name: group("home").label, exact: true }).click();
    await page.waitForURL(/nav=home/);
    await openDrawer();
    assert.equal(await attr(drawer.getByRole("link", { name: group("home").label, exact: true }), "aria-current"), "page");
    if (mobile) await page.keyboard.press("Escape");
  });
  if (!mobile) await step("Sidebar last group", "click Settings, collapse", "last group reachable inside viewport; list scrolls independently", async () => {
    const settings = btn(drawer, group("settings").label);
    const pageY = await page.evaluate(() => window.scrollY); // [READ]
    await settings.click();
    const box = await settings.boundingBox();
    assert.ok(box.y >= 0 && box.y + box.height <= page.viewportSize().height);
    const scroll = await settings.evaluate(n => ({ top: n.closest('nav').querySelector('ul').scrollTop, pageY: window.scrollY })); // [READ]
    assert.ok(scroll.top > 0, "the navigation list scrolls to Settings");
    assert.equal(scroll.pageY, pageY, "the page does not scroll with the list");
    if ((await settings.getAttribute("aria-expanded")) === "true") await settings.click();
  });
  await step("Topbar row and scale", "read geometry after real shell navigation", "50px row, equal-height controls; phone store name visible", async () => {
    const top = page.locator('[data-shell-topbar]');
    const box = await top.boundingBox();
    assert.equal(Math.round(box.height), 50);
    const heights = await top.locator('button:visible,a:visible').evaluateAll(nodes => nodes.map(n => Math.round(n.getBoundingClientRect().height))); // [READ] geometry only
    assert.ok(heights.length >= 3);
    assert.ok(heights.every(h => h === (mobile ? 44 : 36)), `topbar heights: ${heights}`);
    if (mobile) {
      await seen(top.getByText(c.storeName, { exact: true }));
      assert.equal(await top.getByRole("button").count(), 3, "menu, notifications, account only");
    }
  });
  if (mobile) await step("Mobile account items keyboard", "ArrowDown through every item, Escape", "profile, 3 locales, help and store are keyboard reachable; focus restored", async () => {
    const top = page.locator('[data-shell-topbar]');
    const account = top.getByRole("button", { name: c.account, exact: true });
    await account.press("ArrowDown");
    for (const label of [c.setItemProfile, ...locales.map(l => localeNames[l]), c.help, c.enterStore]) {
      const item = top.getByRole("link", { name: label, exact: true });
      await seen(item);
      assert.equal(await item.evaluate(n => n === document.activeElement), true); // [READ] focus
      await page.keyboard.press("ArrowDown");
    }
    await page.keyboard.press("Escape");
    assert.equal(await account.getAttribute("aria-expanded"), "false");
    assert.equal(await account.evaluate(n => n === document.activeElement), true); // [READ]
  });
  await step("Topbar notifications menu", "click, click item", "menu opens, item sets #sec-kpi", async () => {
    await btn(page, c.notifications).click();
    await btn(page, c.kpiUnread).click();
    await page.waitForURL(/#sec-kpi$/);
  });
  await step("Topbar help link", "click", "URL hash #sec-alerts", async () => {
    if (mobile) await btn(page, c.account).click();
    await page.getByRole(mobile ? "link" : "button", { name: c.help, exact: true }).click();
    await page.waitForURL(/#sec-alerts$/);
  });
  await step("Breadcrumb link", "click", "URL hash #sec-tokens", async () => {
    await page.getByRole("navigation", { name: c.crumbLabel }).getByRole("link", { name: c.crumbHome, exact: true }).click();
    await page.waitForURL(/#sec-tokens$/);
  });
  await step("Topbar Enter my store", "click", "opens a new tab (target=_blank)", async () => {
    if (mobile) await btn(page, c.account).click();
    const [popup] = await Promise.all([page.context().waitForEvent("page"), page.getByRole("link", { name: c.enterStore, exact: true }).click()]);
    assert.match(popup.url(), /ui-gallery/);
    await popup.close();
  });
  if (!mobile)
    await step("Skip link", "press Tab then Enter on a fresh load", "URL hash #ds-main", async () => {
      await page.goto(page.url().split("#")[0].split("?")[0], { waitUntil: "load" });
      await page.keyboard.press("Tab");
      await page.keyboard.press("Enter");
      await page.waitForURL(/#ds-main$/);
    });
  await step("Topbar account menu", "click, click Store profile", "navigates to ?setting=profile", async () => {
    const account = page.locator('[data-shell-topbar]').getByRole("button", { name: c.account, exact: true });
    assert.equal(await account.count(), 1, "one accessible account trigger at this viewport");
    await account.click();
    const menuId = await account.getAttribute("aria-controls");
    assert.ok(menuId, "account trigger identifies its disclosure");
    // The gallery's SettingsLayout also has a Store profile link; this journey owns the account disclosure.
    const profile = page.locator(`[id="${menuId}"]`).getByRole(mobile ? "link" : "button", { name: c.setItemProfile, exact: true });
    assert.equal(await profile.count(), 1, "one profile action in the open account disclosure");
    await profile.click();
    await page.waitForURL(/setting=profile/);
  });
  const other = locale === "en" ? "zh-TW" : "en";
  await step("Sidebar expansion preference survives reload", "expand, reload, collapse, reload", "Marketing preserves both states independently of the current page", async () => {
    await openDrawer();
    const marketing = btn(drawer, group("marketing").label);
    if ((await attr(marketing, "aria-expanded")) !== "true") await marketing.click();
    await page.reload({ waitUntil: "load" });
    await openDrawer();
    assert.equal(await attr(btn(drawer, group("marketing").label), "aria-expanded"), "true", "expanded Marketing survives reload");
    await btn(drawer, group("marketing").label).click();
    await page.reload({ waitUntil: "load" });
    await openDrawer();
    assert.equal(await attr(btn(drawer, group("marketing").label), "aria-expanded"), "false", "collapsed Marketing survives reload");
    if (mobile) await page.keyboard.press("Escape");
  });
  await step("Dialog keyboard containment and restoration", "open, Shift+Tab, Tab cycles, Escape", "focus remains in dialog, Escape returns focus to opener", async () => {
    const opener = btn(dialogDemo, c.dlgOpenPlain);
    await opener.click();
    for (const key of ["Shift+Tab", "Tab", "Tab", "Tab"]) {
      await page.keyboard.press(key);
      assert.equal(await dialog.evaluate(n => n.contains(document.activeElement)), true); // [READ] focus only
    }
    await page.keyboard.press("Escape");
    await dialog.waitFor({ state: "hidden" });
    assert.equal(await opener.evaluate(n => n === document.activeElement), true); // [READ]
  });
  await step("Topbar language dropdown", `click ${localeNames[other]}`, `URL /${other}/ui-gallery and its own heading`, async () => {
    // The phone keeps language links inside Account; desktop retains its dedicated dropdown.
    const language = mobile ? page.locator('[data-shell-topbar]') : page.getByRole("navigation", { name: c.localeLabel });
    const trigger = language.getByRole("button", { name: mobile ? c.account : localeNames[locale], exact: true });
    await trigger.click();
    assert.equal(await trigger.getAttribute("aria-expanded"), "true");
    await language.getByRole("link", { name: localeNames[other], exact: true }).click();
    await page.waitForURL(new RegExp(`/${other}/ui-gallery`));
    await seen(page.getByRole("heading", { level: 1 }));
  });
  if (!ds2Nested && locale === "zh-TW" && !mobile) await ds2Matrix({ page, step });
}

// DS-2 extends the existing CI entry point here because the task's write_paths exclude ui-gallery-runner.mjs.
// The same Page is retained so the runner's click ledger records the actual current route for every row.
// [READ] Includes offscreen rendered controls (e.g. a scrolling table), not display:none controls.
async function readControlScale(page) {
  const controls = page.locator('button, [role="button"], select, input:not([type="checkbox"]):not([type="radio"]):not([type="file"]):not([type="hidden"]):not([type="range"]):not([type="color"]), a[data-size], nav > ul > li > a, nav > ul > li > [role="link"]').filter({ visible: true });
  const geometry = await controls.evaluateAll(nodes => nodes.map(n => {
    const box = n.getBoundingClientRect();
    return { height: box.height, centre: box.top + box.height / 2 };
  })); // [READ] rendered border boxes only
  const names = await Promise.all(Array.from({ length: geometry.length }, async (_, i) => {
    const snapshot = await controls.nth(i).ariaSnapshot(); // [READ] Playwright's computed accessible name
    return snapshot.split("\n")[0].replace(/^\s*-\s*/, "");
  }));
  const rows = await controls.evaluateAll(nodes => {
    const pairs = [];
    for (let i = 0; i < nodes.length; i++) for (let j = i + 1; j < nodes.length; j++) {
      let container = nodes[i].parentElement;
      while (container && !container.contains(nodes[j])) container = container.parentElement;
      if (!container) continue;
      const style = getComputedStyle(container);
      // A sticky/fixed branch (including a control's ancestors) is not part of this row.
      const branch = node => {
        let child = node;
        for (let n = node; n && n !== container; n = n.parentElement) {
          if (["absolute", "fixed", "sticky"].includes(getComputedStyle(n).position)) return null;
          child = n;
        }
        return child;
      };
      const a = branch(nodes[i]);
      const b = branch(nodes[j]);
      if (!a || !b || a === b) continue;
      if (style.display.includes("grid")) {
        // Computed grid tracks are used pixel sizes, including implicitly placed rows.
        // Both direct grid items must fit one and the same track; spanning items are not a row.
        const tracks = style.gridTemplateRows.replace(/\[[^\]]*\]/g, "").trim().split(/\s+/);
        if (!tracks.every(track => /^\d+(?:\.\d+)?px$/.test(track))) continue;
        const box = container.getBoundingClientRect();
        let top = box.top + parseFloat(style.borderTopWidth) + parseFloat(style.paddingTop);
        const gap = parseFloat(style.rowGap) || 0;
        const ab = a.getBoundingClientRect();
        const bb = b.getBoundingClientRect();
        for (const track of tracks) {
          const bottom = top + parseFloat(track);
          if ([ab, bb].every(rect => rect.top >= top - 0.5 && rect.bottom <= bottom + 0.5)) {
            pairs.push([i, j]);
            break;
          }
          top = bottom + gap;
        }
      } else if ((style.display.includes("flex") && style.flexDirection.startsWith("row")) || style.display === "table-row") pairs.push([i, j]);
    }
    return pairs;
  }); // [READ] container relationships; no DOM or scroll writes
  const measured = geometry.map((box, i) => ({ name: names[i], ...box }));
  return { measured, rows, violations: controlScaleViolations(measured, rows) };
}

function controlScaleViolations(measured, rows) {
  const violations = [];
  if (!measured.length) violations.push("no rendered controls measured");
  for (const control of measured) if (![28, 36, 44].some(height => Math.abs(control.height - height) <= 0.5)) violations.push(`${control.name}: height=${control.height}px`);
  for (const [i, j] of rows) if (Math.abs(measured[i].centre - measured[j].centre) <= 0.5 && Math.abs(measured[i].height - measured[j].height) > 0.5) violations.push(`same row: ${measured[i].name} height=${measured[i].height}px; ${measured[j].name} height=${measured[j].height}px`);
  return violations;
}

// [READ] The frame the screenshot paints: a full-page shot draws sticky/fixed chrome at its viewport rect, so a bar over a
// control here is a bar over that control in the image. Also clipped tab labels and disabled primary contrast (fill vs what is behind it).
function readFrame(page) {
  return page.evaluate(() => {
    const shown = n => { const r = n.getBoundingClientRect(); const s = getComputedStyle(n); return r.width >= 4 && r.height >= 4 && s.visibility !== "hidden" && s.display !== "none"; };
    const name = n => `${n.tagName.toLowerCase()}[${(n.getAttribute("aria-label") || n.textContent || n.getAttribute("name") || "").trim().slice(0, 30)}]`;
    const chrome = [...document.querySelectorAll("body *")].filter(n => ["sticky", "fixed"].includes(getComputedStyle(n).position) && shown(n));
    const controls = [...document.querySelectorAll('input:not([type="hidden"]), select, textarea, button, [role="radio"], [role="switch"], [role="tab"]')].filter(shown);
    const overlap = (a, b) => Math.min(a.right, b.right) - Math.max(a.left, b.left) > 0.5 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 0.5;
    const occluded = [];
    for (const bar of chrome) for (const c of controls) if (!bar.contains(c) && !c.contains(bar) && overlap(bar.getBoundingClientRect(), c.getBoundingClientRect())) occluded.push(`${name(bar)} over ${name(c)}`);
    const clippedTabs = [];
    const lists = [...document.querySelectorAll('[role="tablist"]')].filter(shown);
    for (const list of lists) {
      const box = list.getBoundingClientRect();
      if (list.scrollWidth > list.clientWidth) clippedTabs.push(`${name(list)} scrollWidth ${list.scrollWidth} > clientWidth ${list.clientWidth}`);
      for (const tab of list.querySelectorAll('[role="tab"]')) {
        const r = tab.getBoundingClientRect();
        if (tab.scrollWidth > tab.clientWidth || r.left < box.left - 0.5 || r.right > box.right + 0.5) clippedTabs.push(`${name(tab)} [${r.left}, ${r.right}] outside [${box.left}, ${box.right}] or scrollWidth ${tab.scrollWidth} > ${tab.clientWidth}`);
      }
    }
    const rgb = c => (c.match(/[\d.]+/g) || []).map(Number);
    const lum = c => { const [r, g, b] = rgb(c).slice(0, 3).map(v => { v /= 255; return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
    const ratio = (a, b) => { const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x); return (hi + 0.05) / (lo + 0.05); };
    const behind = n => { for (let p = n.parentElement; p; p = p.parentElement) { const c = getComputedStyle(p).backgroundColor; const v = rgb(c); if (v.length === 3 || v[3] > 0) return c; } return "rgb(255, 255, 255)"; };
    const disabled = [...document.querySelectorAll('button[data-variant="primary"][aria-disabled="true"]:not([aria-busy="true"])')].filter(shown);
    const faint = disabled.flatMap(b => {
      const s = getComputedStyle(b);
      const fill = ratio(s.backgroundColor, behind(b));
      const label = ratio(s.color, s.backgroundColor);
      return fill >= 3 && label >= 3 && Number(s.opacity) === 1 ? [] : [`${name(b)} fill ${fill.toFixed(2)}:1 vs ${behind(b)}, label ${label.toFixed(2)}:1, opacity ${s.opacity}`];
    });
    return { occluded, clippedTabs, faint, chrome: chrome.length, controls: controls.length, tabs: lists.length, disabledPrimary: disabled.length };
  });
}

// [READ] Real phone scrolling: wheel from the top to the page end in 120px notches. Every field control must at some scroll position sit
// wholly in the viewport with elementFromPoint at its centre and edge midpoints landing on it (no sticky bar on top), and the top bar
// must stay pinned at the viewport top. Tabs are excluded here; readFrame owns tab clipping.
async function wheelReach(page, scope) {
  const size = page.viewportSize();
  await page.mouse.move(size.width / 2, size.height / 2);
  await page.mouse.wheel(0, -100000);
  await page.waitForFunction(() => window.scrollY === 0);
  const probe = () => scope.evaluate(root => {
    const controls = [...root.querySelectorAll('input:not([type="hidden"]), select, textarea, button, [role="radio"], [role="switch"]')].filter(n => {
      const r = n.getBoundingClientRect();
      const s = getComputedStyle(n);
      return r.width >= 4 && r.height >= 4 && s.visibility !== "hidden" && !n.closest('[role="tablist"]');
    });
    return {
      scrollY: window.scrollY,
      max: document.scrollingElement.scrollHeight - window.innerHeight,
      topbar: document.querySelector("[data-shell-topbar]").getBoundingClientRect().top,
      controls: controls.map((n, i) => {
        const r = n.getBoundingClientRect();
        const inset = Math.min(2, r.width / 4, r.height / 4);
        const [cx, cy] = [r.left + r.width / 2, r.top + r.height / 2];
        const points = [[cx, cy], [cx, r.top + inset], [cx, r.bottom - inset], [r.left + inset, cy], [r.right - inset, cy]];
        const inView = r.top >= 0 && r.bottom <= window.innerHeight && r.left >= 0 && r.right <= window.innerWidth;
        const clear = inView && points.every(([x, y]) => { const hit = document.elementFromPoint(x, y); return hit === n || n.contains(hit); });
        return [`${i}:${n.tagName.toLowerCase()}[${(n.getAttribute("aria-label") || n.labels?.[0]?.textContent || n.textContent || n.name || "").trim().slice(0, 30)}]`, clear];
      }),
    };
  });
  const reached = new Map();
  let state;
  for (let notch = 0; notch < 80; notch++) {
    state = await probe();
    for (const [key, clear] of state.controls) reached.set(key, reached.get(key) || clear);
    assert.ok(Math.abs(state.topbar) <= 0.5, `top bar left the viewport top at scrollY=${state.scrollY}: top=${state.topbar}`);
    if (state.scrollY >= state.max - 0.5) break;
    await page.mouse.wheel(0, 120);
    await page.waitForFunction(y => window.scrollY > y, state.scrollY);
  }
  assert.ok(state.scrollY >= state.max - 0.5, `wheel did not reach the page end: ${JSON.stringify({ scrollY: state.scrollY, max: state.max })}`);
  const blocked = [...reached].filter(([, clear]) => !clear).map(([key]) => key);
  assert.deepEqual(blocked, [], "controls never fully visible and unobstructed under native wheel scrolling");
  return `${reached.size} controls each reached fully visible and unobstructed; page end scrollY=${state.scrollY}`;
}

async function sidebarViewportGate({ page, step, locale, template }) {
  const rail = page.locator('[data-shell-rail]');
  const list = rail.locator(':scope > ul');
  const settings = list.locator(':scope > li').last().getByRole("button", { name: galleryNav(locale).find(group => group.id === "settings").label, exact: true });
  await settings.waitFor({ state: "visible" });
  await step(`DS-2 ${locale}/1440 ${template} sidebar fit`, "[READ] scrollHeight/clientHeight/overflow-y", "nav list fits or has auto/scroll overflow at 1440×900", async () => {
    const size = await list.evaluate(n => ({ scroll: n.scrollHeight, client: n.clientHeight, overflow: getComputedStyle(n).overflowY })); // [READ]
    assert.ok(size.client > 0, `nav list has a viewport: ${JSON.stringify(size)}`);
    assert.ok(size.scroll <= size.client || ["auto", "scroll"].includes(size.overflow), `nav list clipped: ${JSON.stringify(size)}`);
    return JSON.stringify(size);
  });
  await step(`DS-2 ${locale}/1440 ${template} sidebar last group`, "click Settings (native list scrolling), read boxes, restore expansion", "last group reachable inside list and viewport; page scroll unchanged", async () => {
    const expanded = await settings.getAttribute("aria-expanded");
    const before = await list.evaluate(n => ({ top: n.scrollTop, pageY: window.scrollY, bottom: n.getBoundingClientRect().bottom })); // [READ]
    const prior = await settings.boundingBox();
    try {
      await settings.click();
      const box = await settings.boundingBox();
      const viewport = await list.boundingBox();
      const after = await list.evaluate(n => ({ top: n.scrollTop, pageY: window.scrollY })); // [READ]
      assert.ok(box.y >= viewport.y - 1 && box.y + box.height <= viewport.y + viewport.height + 1 && box.y >= 0 && box.y + box.height <= 900, `Settings outside scroll viewport: ${JSON.stringify({ box, viewport })}`);
      if (prior.y + prior.height > before.bottom + 1) assert.ok(after.top > before.top, `Settings did not scroll into view: ${JSON.stringify({ before, after })}`);
      assert.equal(after.pageY, before.pageY, "only the sidebar list scrolls");
    } finally {
      if (await settings.getAttribute("aria-expanded") !== expanded) await settings.click();
    }
  });
}

async function ds2Matrix({ page, step }) {
  const { mkdir, readFile, writeFile } = await import("node:fs/promises");
  const { createRequire } = await import("node:module");
  const { createHash } = await import("node:crypto");
  const { browserEvidenceDirectory } = await import("../browser-evidence.mjs");
  const { collect, evaluate, describe } = await import("../ui/visual-lint-lib.mjs");
  const { galleryCopy } = await import("../../apps/admin/src/ui-gallery-copy.ts");
  const { templateCopy } = await import("../../apps/admin/src/ui-gallery-template-copy.ts");
  const output = process.env.LC_UI_GALLERY_EVIDENCE || browserEvidenceDirectory("ui-gallery");
  const require = createRequire(import.meta.url);
  const axeSource = await readFile(require.resolve("axe-core/axe.min.js"), "utf8");
  await mkdir(`${output}/shots`, { recursive: true });
  const origin = new URL(page.url()).origin;
  const manifest = [];
  const capture = async (name, locale, width, height) => {
    // [READ] await font metrics only; screenshot/audits never alter product state.
    await page.evaluate(async () => { await document.fonts.ready; });
    await page.locator('[data-shell-rail] > ul > li').first().waitFor({ state: "attached" });
    if (name === "settings") await page.locator('[data-template="settings"] input').first().waitFor({ state: "visible" });
    if (name === "detail") await page.locator('[data-template="detail"] textarea').waitFor({ state: "visible" });
    const scale = await readControlScale(page);
    await step(`DS-2 ${locale}/${width} ${name} control scale`, "[READ] rendered heights and row centres", "every control 28/36/44 ±0.5px; same-row heights match", async () => {
      assert.equal(scale.violations.length, 0, scale.violations.join("; "));
      if (name.startsWith("list")) {
        const filters = page.getByRole("group", { name: templateCopy[locale].filters, exact: true });
        const select = await filters.getByRole("combobox").boundingBox();
        const search = await filters.getByRole("searchbox").boundingBox();
        assert.ok(Math.abs(select.height - search.height) <= 0.5, "filter combobox and searchbox heights match");
        if (width === 1440) {
          const i = scale.measured.findIndex(control => control.name.startsWith("combobox") && control.name.includes(templateCopy[locale].payment));
          const j = scale.measured.findIndex(control => control.name.startsWith("searchbox"));
          assert.ok(i >= 0 && j >= 0 && scale.rows.some(pair => pair.includes(i) && pair.includes(j)), "real filter row remains covered");
          // [FIXTURE] Mutate the measurement only, never the DOM: 44px is on-scale but wrong beside 36px.
          const mismatched = scale.measured.map((control, index) => index === j ? { ...control, height: 44 } : control);
          assert.ok(controlScaleViolations(mismatched, scale.rows).some(violation => violation.startsWith("same row:") && violation.includes("searchbox")), "same-row mismatch still rejected");
        }
      }
      return `${scale.measured.length} controls measured; no scale or row violations`;
    });
    const path = `${output}/shots/ds2-${locale}-${width}x${height}-${name}.png`;
    // Full-page shots paint sticky bars at the current scroll offset (a second topbar mid-page after uploads);
    // start from the top unless the scrolled state is the subject of the shot.
    if (name !== "form-released") await page.evaluate(() => new Promise(r => { window.scrollTo(0, 0); requestAnimationFrame(() => r()); })); // [READ] view position only
    if (width < 768 && /^(form|settings)/.test(name) && name !== "form-released")
      await step(`DS-2 ${locale}/${width} ${name} wheel reach`, "native wheel top to end; elementFromPoint at each control", "every field control fully in view, unobstructed, at some scroll position; top bar pinned", () => wheelReach(page, page.locator("[data-template]")));
    const frame = await readFrame(page);
    await step(`DS-2 ${locale}/${width} ${name} captured frame`, "[READ] rects of the frame about to be captured", "no sticky/fixed chrome over a control; no clipped tab; disabled primary fill and label >= 3:1", async () => {
      assert.deepEqual(frame.occluded, [], "sticky/fixed chrome over controls in the captured frame");
      assert.deepEqual(frame.clippedTabs, [], "tab labels clipped");
      assert.deepEqual(frame.faint, [], "disabled primary too faint");
      return `${frame.chrome} sticky/fixed elements, ${frame.controls} controls, ${frame.tabs} tabs, ${frame.disabledPrimary} disabled primaries`;
    });
    const png = await page.screenshot({ path, fullPage: true, animations: "disabled" });
    await page.addScriptTag({ content: axeSource });
    const axe = await page.evaluate(async () => (await window.axe.run(document, { resultTypes: ["violations"] })).violations.map(v => ({ id: v.id, impact: v.impact, nodes: v.nodes.map(n => n.target) }))); // [READ] all impacts, stricter than legacy runner
    const snapshot = await page.evaluate(collect); // [READ/MEASURE]
    const lint = evaluate(snapshot, { mobile: width < 768 }).filter(v => v.severity === "block");
    const named = lint.length ? await page.evaluate(describe, [...new Set(lint.flatMap(v => v.ids))]) : {};
    const errors = await page.locator('[data-nextjs-dialog],nextjs-portal').count(); // [READ] no Next error overlay
    manifest.push({ name, locale, width, height, route: new URL(page.url()).pathname + new URL(page.url()).search, path, sha256: createHash("sha256").update(png).digest("hex"), scale, axe, lint, named, errors });
    await writeFile(`${output}/ds2-matrix.json`, `${JSON.stringify({ tier: "MOCK", manifest }, null, 2)}\n`);
    assert.deepEqual(axe, [], `${name} ${locale} ${width}: axe violations`);
    assert.deepEqual(lint, [], `${name} ${locale} ${width}: visual-lint blocking ${JSON.stringify(named)}`);
    assert.equal(errors, 0, "no application error overlay");
  };
  try {
    for (const locale of ["zh-TW", "zh-CN", "en"]) for (const [width, height] of [[1440, 900], [390, 844]]) {
      await page.setViewportSize({ width, height });
      await page.goto(`${origin}/${locale}/ui-gallery`, { waitUntil: "load" });
      // [FIXTURE PREPARATION ONLY] Isolate the synthetic demo between matrix cases. Never run during a persistence assertion.
      await page.evaluate(() => { for (const key of Object.keys(sessionStorage)) if (key.startsWith("ds2-")) sessionStorage.removeItem(key); });
      await page.reload({ waitUntil: "load" });
      await page.getByRole("heading", { level: 1, name: galleryCopy[locale].title }).waitFor();
      await step(`DS-2 ${locale}/${width} components visual`, "screenshot + axe + visual-lint", "axe all impacts 0; blocking layout 0", () => capture("components", locale, width, height));
      if (width === 1440) await sidebarViewportGate({ page, step, locale, template: "components" });
      await clickJourney({ page, step, copy: galleryCopy[locale], locale, mobile: width < 768, ds2Nested: true });
      for (const template of ["list", "detail", "form", "settings"]) {
        const response = await page.goto(`${origin}/${locale}/ui-gallery/templates/${template}`, { waitUntil: "load" });
        assert.equal(response.status(), 200);
        await page.locator(`[data-template="${template}"]`).waitFor();
        await step(`DS-2 ${locale}/${width} ${template} visual`, "screenshot + axe + visual-lint", "axe all impacts 0; blocking layout 0", () => capture(template, locale, width, height));
        if (width === 1440) await sidebarViewportGate({ page, step, locale, template });
        await templateJourney({ page, step, t: templateCopy[locale], template, locale, mobile: width < 768, capture: name => capture(name, locale, width, height) });
      }
    }
  } finally {
    await writeFile(`${output}/ds2-matrix.json`, `${JSON.stringify({ tier: "MOCK", manifest }, null, 2)}\n`);
  }
}

async function templateJourney({ page, step, t, template, locale, mobile, capture }) {
  const { galleryCopy } = await import("../../apps/admin/src/ui-gallery-copy.ts");
  const root = `/${locale}/ui-gallery/templates`;
  const area = page.locator(`[data-template="${template}"]`);
  const button = name => area.getByRole("button", { name, exact: true }).filter({ visible: true }).first();
  const seen = loc => loc.filter({ visible: true }).first().waitFor({ state: "visible" });
  const text = value => area.getByText(value, { exact: true });
  const run = (control, action, expected, fn) => step(`${template}: ${control}`, action, expected, fn);
  const containsFocus = loc => loc.evaluate(n => n.contains(document.activeElement)); // [READ]
  const save = async () => {
    const b = button(t.save);
    assert.equal(await b.getAttribute("aria-disabled"), null, "valid dirty data enables Save");
    await b.click();
    await seen(area.locator('[data-state="saving"]'));
    await seen(area.locator('[data-state="saved"]'));
    await seen(text(t.saved));
  };
  // The record hydrates from sessionStorage in an effect after load, so a read right after reload can see the seed.
  // Form: Cancel enables once ready, then the draft re-syncs (one "dirty" render) back to "clean".
  // Settings reads saved directly; its fields stop being read-only once ready.
  const reloadHydrated = async () => {
    await page.reload({ waitUntil: "load" });
    if (template === "settings") {
      await page.waitForFunction(el => !el.readOnly, await area.getByLabel(t.storeName, { exact: true }).elementHandle()); // [READ]
      return;
    }
    await page.waitForFunction(el => !el.hasAttribute("aria-disabled"), await button(t.cancel).elementHandle()); // [READ]
    await seen(area.locator('[data-state="clean"]'));
  };
  await run("Localized breadcrumb", "read trail, click parent, reload, return", "correct IA group and localized names; parent navigation persists", async () => {
    const c = galleryCopy[locale];
    const groupId = template === "settings" ? "settings" : template === "form" ? "catalog" : "orders";
    const group = galleryNav(locale).find(item => item.id === groupId);
    const trail = page.getByRole("navigation", { name: c.crumbLabel, exact: true });
    const expected = [group.label, ...(template === "detail" ? [t.list] : []), t[template]];
    assert.deepEqual(await trail.locator("li").allTextContents(), expected);
    assert.equal(await trail.locator('[aria-current="page"]').textContent(), t[template]);
    const parent = trail.getByRole("link").last();
    const href = await parent.getAttribute("href");
    await parent.click();
    await page.waitForURL(url => `${url.pathname}${url.search}` === href);
    await page.reload({ waitUntil: "load" });
    assert.equal(`${new URL(page.url()).pathname}${new URL(page.url()).search}`, href);
    await page.goto(`${new URL(page.url()).origin}${root}/${template}`, { waitUntil: "load" });
  });
  if (template === "list") {
    await run("List control placement", "read filter/preview layout", "filters/search only; demo selector outside card; bulk menu appears only after selection", async () => {
      const filters = area.getByRole("group", { name: t.filters, exact: true });
      assert.equal(await filters.getByLabel(t.payment, { exact: true }).count(), 1);
      assert.equal(await filters.getByRole("searchbox", { name: t.search }).count(), 1);
      assert.equal(await filters.getByLabel(t.state, { exact: true }).count(), 0);
      assert.equal(await button(t.action).count(), 0);
      const preview = area.getByRole("group", { name: t.previewControls, exact: true });
      assert.equal(await preview.getByLabel(t.state, { exact: true }).count(), 1);
      assert.equal(await preview.evaluate(n => n.closest("section")), null); // [READ] outside the order card
    });
    if (mobile) await run("Table horizontal scroll affordance", "read cue, ArrowRight, reload", "localized cue and native focusable scroller; table moves without page overflow", async () => {
      const cue = area.locator('[data-table-scroll-hint]');
      await seen(cue.getByText(t.tableScroll, { exact: true }));
      const scroller = area.locator('[data-table-scroll]');
      assert.equal(await scroller.getAttribute("tabindex"), "0");
      const before = await scroller.evaluate(n => n.scrollLeft); // [READ]
      await scroller.press("ArrowRight");
      await scroller.press("ArrowRight");
      await page.waitForFunction(el => el.scrollLeft > 0, await scroller.elementHandle()); // [READ]
      assert.ok(await scroller.evaluate(n => n.scrollLeft) > before);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true); // [READ]
      await page.reload({ waitUntil: "load" });
      await seen(cue.getByText(t.tableScroll, { exact: true }));
    });
    await run("Export menu keyboard", "focus, ArrowDown, End, Enter", "focus enters menu, command updates visible demo result", async () => {
      await button(t.export).focus();
      await page.keyboard.press("ArrowDown");
      const item = area.locator('[role="group"] > div > button').filter({ hasText: t.export }).last();
      assert.equal(await item.evaluate(n => n === document.activeElement), true); // [READ]
      await page.keyboard.press("End");
      await page.keyboard.press("Enter");
      await seen(text(t.exportDone));
    });
    await run("Filter URL and persistence", "select Unpaid, reload", "payment=unpaid, 8 matching rows and filter persist", async () => {
      await area.getByLabel(t.payment, { exact: true }).selectOption("unpaid");
      assert.equal(new URL(page.url()).searchParams.get("payment"), "unpaid");
      await page.reload({ waitUntil: "load" });
      assert.equal(await area.getByLabel(t.payment, { exact: true }).inputValue(), "unpaid");
      assert.equal(await area.locator('tbody [data-status="unpaid"]').count(), 5);
    });
    await run("Select page vs all results", "check page, select all, bulk menu, confirm", "5 selected then 8 selected; all-results operation requires confirmation", async () => {
      await area.getByRole("checkbox", { name: t.selectPage, exact: true }).check();
      await seen(text(`5 ${t.selected}`));
      assert.equal(await area.locator('[data-selection-bar]').getByRole("button", { name: t.action, exact: true }).count(), 1);
      await button(`${t.selectAll} (8)`).click();
      await seen(text(`8 ${t.selected}`));
      await seen(text(`${t.allResults} (8)`));
      await button(t.action).click();
      await button(t.tag).click();
      const dialog = page.getByRole("dialog");
      await seen(dialog.getByText(t.bulkConfirm, { exact: true }));
      await page.keyboard.press("Shift+Tab");
      assert.equal(await containsFocus(dialog), true);
      await page.keyboard.press("Escape");
      await dialog.waitFor({ state: "hidden" });
      await seen(text(`8 ${t.selected}`));
      await button(t.action).click();
      await button(t.tag).click();
      await dialog.getByRole("button", { name: t.confirm, exact: true }).click();
      await seen(text(t.bulkDone));
      assert.equal(await area.getByText(`8 ${t.selected}`, { exact: true }).count(), 0);
      await page.reload({ waitUntil: "load" });
      await seen(text(t.bulkDone));
    });
    await run("Row select and clear", "check row 1, clear", "only one row selected; clearing removes banner", async () => {
      await area.getByRole("checkbox", { name: `${t.row} 1`, exact: true }).check();
      await seen(text(`1 ${t.selected}`));
      await button(t.clear).click();
      assert.equal(await area.locator('tbody tr[data-selected="true"]').count(), 0);
      assert.equal(await button(t.action).count(), 0);
    });
    await run("Search privacy", "fill DEMO-1001, click Search", "one matching order; neither q nor pq nor the term enters any URL", async () => {
      await area.getByRole("searchbox", { name: t.search }).fill("DEMO-1001");
      await button(t.searchButton).click();
      assert.equal(await area.locator("tbody tr").count(), 1);
      assert.equal(await area.locator('tbody tr [data-status="unpaid"]').count(), 1);
      const urls = await area.locator("a[href]").evaluateAll(nodes => nodes.map(n => n.getAttribute("href"))); // [READ]
      for (const href of [page.url(), ...urls]) {
        const u = new URL(href, page.url());
        assert.equal(u.searchParams.has("q"), false);
        assert.equal(u.searchParams.has("pq"), false);
        assert.equal(u.search.includes("DEMO-1001"), false);
      }
      await area.getByRole("searchbox", { name: t.search }).fill("");
      await area.getByRole("searchbox", { name: t.search }).press("Enter");
    });
    await run("Pagination", "click next, reload, previous", "page=2 survives reload; previous returns to page 1", async () => {
      await area.getByRole("link", { name: t.next, exact: true }).click();
      assert.equal(new URL(page.url()).searchParams.get("page"), "2");
      await page.reload({ waitUntil: "load" });
      await seen(area.getByRole("link", { name: `${t.page} 2`, exact: true }));
      assert.equal(await area.getByRole("link", { name: `${t.page} 2`, exact: true }).getAttribute("aria-current"), "page");
      await area.getByRole("link", { name: t.previous, exact: true }).click();
      assert.equal(new URL(page.url()).searchParams.has("page"), false);
    });
    await run("List row actions", "more, tag, reload", "tag result persists", async () => {
      const row = area.locator("tbody tr").first();
      const more = row.getByRole("button", { name: t.more, exact: true });
      assert.equal(await more.getAttribute("aria-label"), t.more);
      assert.equal(await more.getAttribute("data-icon-only"), "true");
      assert.equal(await more.locator("svg").count(), 1, "more icon alone, no chevron");
      // On mobile the scroll frame is moved by Playwright actionability, never by DOM mutation.
      await row.getByRole("button", { name: t.more, exact: true }).click();
      await row.getByRole("button", { name: t.tag, exact: true }).click();
      await seen(text(t.tagged));
      await page.reload({ waitUntil: "load" });
      await seen(text(t.tagged));
    });
    for (const state of ["loading", "error", "empty"]) await run(`List ${state}`, "select state, reload, screenshot, recover", "state persists; retry/reset recovers visibly", async () => {
      await area.getByLabel(t.state, { exact: true }).selectOption(state);
      await page.reload({ waitUntil: "load" });
      assert.equal(await area.getByLabel(t.state, { exact: true }).inputValue(), state);
      await capture(`list-${state}`);
      // Loading/error carry no trustworthy count and empty has nothing to page: no pager (was "0 results · 1 / 2").
      assert.equal(await area.getByRole("navigation", { name: t.paging, exact: true }).count(), 0, `no pager while ${state}`);
      if (state === "loading") { assert.equal(await area.locator('table[aria-busy="true"]').count(), 1); await area.getByLabel(t.state, { exact: true }).selectOption("ready"); }
      // [READ] The state must be readable where it renders, not only after Playwright's click auto-scroll: on 390px the
      // 760px-min table once centred it off-screen (run 38765960368: only "試試"/"此處預" visible at the right edge).
      const inView = async (title, action) => {
        const box = await area.locator("[data-table-scroll]").evaluate((scroll, names) => {
          const view = scroll.getBoundingClientRect();
          const find = name => [...scroll.querySelectorAll("*")].filter(n => n.textContent.trim() === name).pop()?.getBoundingClientRect(); // deepest match
          const parts = [find(names[0]), find(names[1])];
          return { scrollLeft: scroll.scrollLeft, view: [view.left, view.right], parts: parts.map(r => r && [r.left, r.right]) };
        }, [title, action]);
        assert.equal(box.scrollLeft, 0, `${state}: table scroller moved`);
        for (const part of box.parts) assert.ok(part && part[0] >= box.view[0] - 1 && part[1] <= box.view[1] + 1, `${state} state off-screen: ${JSON.stringify(box)}`);
      };
      if (state === "error") { await seen(text(t.errorTitle)); await inView(t.errorTitle, t.retry); await button(t.retry).click(); }
      if (state === "empty") { await seen(text(t.emptyTitle)); await inView(t.emptyTitle, t.reset); await button(t.reset).click(); }
      assert.equal(await area.locator("tbody tr[data-selected]").count(), 5);
      await seen(area.getByRole("navigation", { name: t.paging, exact: true }));
    });
    await run("Order links", "click row View, back, click order number", "both links reach the detail template", async () => {
      await area.getByRole("link", { name: t.view, exact: true }).first().click();
      await page.waitForURL(new RegExp(`${root}/detail`));
      await page.getByRole("link", { name: t.back, exact: true }).click();
      await area.getByRole("link", { name: "DEMO-1001", exact: true }).click();
      await page.waitForURL(new RegExp(`${root}/detail`));
    });
    await page.goto(`${new URL(page.url()).origin}${root}/list`, { waitUntil: "load" });
    await run("Title primary", "click Create demo draft, confirm, reload", "new synthetic row and result persist", async () => {
      await area.getByRole("button", { name: t.create, exact: true }).click();
      const dialog = page.getByRole("dialog");
      await seen(dialog.getByText(t.create, { exact: true }));
      await dialog.getByRole("button", { name: t.confirm, exact: true }).click();
      await seen(text(t.created));
      await seen(area.getByRole("link", { name: "DEMO-1025", exact: true }));
      await page.reload({ waitUntil: "load" });
      await seen(area.getByRole("link", { name: "DEMO-1025", exact: true }));
    });
  }
  if (template === "detail") {
    await run("Detail status title row", "read header layout", "both statuses are inside the h1 row", async () => {
      const row = area.getByRole("heading", { level: 1 }).locator("..");
      await seen(row.getByText(t.paid, { exact: true }));
      await seen(row.getByText(t.pending, { exact: true }));
      if (!mobile) {
        const title = await row.getByRole("heading", { level: 1 }).boundingBox();
        const badge = await row.getByText(t.paid, { exact: true }).boundingBox();
        assert.ok(badge.y < title.y + title.height && badge.y + badge.height > title.y);
      }
    });
    await run("Internal note", "fill, save, reload", "note and result persist; timeline ISO times remain", async () => {
      // Wait for the visible restored editor, then count every match (including hidden duplicates).
      await seen(area.getByRole("textbox", { name: t.noteContent, exact: true }));
      assert.equal(await area.getByLabel(t.noteContent, { exact: true }).count(), 1, "one note editor after restoration");
      await area.getByLabel(t.noteContent, { exact: true }).fill("Synthetic note A");
      await button(t.addNote).click();
      await seen(text(t.noteSaved));
      await page.reload({ waitUntil: "load" });
      assert.equal(await area.getByLabel(t.noteContent, { exact: true }).inputValue(), "Synthetic note A", "saved note is exposed immediately when the editor returns");
      assert.equal(await area.locator("time[datetime]").count(), 3, "three timeline ISO times survive reload");
    });
    for (const [item, result] of [[t.receipt, t.receiptDone], [t.archive, t.archived]]) await run(item, "open More, choose action, reload", "visible demo result persists", async () => {
      await button(t.more).click(); await button(item).click(); await seen(text(result));
      await page.reload({ waitUntil: "load" }); await seen(text(result));
    });
    await run("Back to list", "click", "orders template opens", async () => {
      await area.getByRole("link", { name: t.back, exact: true }).click();
      await page.waitForURL(new RegExp(`${root}/list`));
    });
  }
  if (template === "form") {
    const input = label => area.getByLabel(label, { exact: true });
    const tab = label => area.getByRole("tab", { name: label, exact: true });
    await run("Save disabled→enabled; fields", "edit every basic control, save, reload", "dirty/saving/saved; all field values persist", async () => {
      assert.equal(await button(t.save).getAttribute("aria-disabled"), "true");
      assert.equal(await input(t.name).count(), 1, "required name retains its exact visible label");
      await input(t.name).fill("Synthetic product A");
      await seen(text(t.dirty));
      await input(t.category).selectOption("accessories");
      await input(t.description).fill("Synthetic material and care.");
      await area.getByRole("checkbox", { name: t.visible, exact: true }).uncheck();
      await area.getByRole("radio", { name: t.live, exact: true }).check();
      await area.getByRole("switch", { name: t.enabled, exact: true }).click();
      await save();
      await reloadHydrated();
      assert.equal(await input(t.name).inputValue(), "Synthetic product A");
      assert.equal(await input(t.category).inputValue(), "accessories");
      assert.equal(await input(t.description).inputValue(), "Synthetic material and care.");
      assert.equal(await area.getByRole("checkbox", { name: t.visible, exact: true }).isChecked(), false);
      assert.equal(await area.getByRole("radio", { name: t.live, exact: true }).isChecked(), true);
      assert.equal(await area.getByRole("switch", { name: t.enabled, exact: true }).getAttribute("aria-checked"), "false");
    });
    await run("Validation and cancel confirmation", "clear required name, cancel, Escape, cancel, discard", "error linked to input, invalid save blocked; discard restores saved value", async () => {
      await input(t.name).fill("");
      assert.equal(await input(t.name).getAttribute("aria-invalid"), "true");
      await seen(text(t.required));
      assert.equal(await button(t.save).getAttribute("aria-disabled"), "true");
      await button(t.cancel).click();
      const dialog = page.getByRole("dialog");
      await seen(dialog.getByText(t.cancelConfirm));
      await page.keyboard.press("Escape");
      assert.equal(await input(t.name).inputValue(), "");
      await button(t.cancel).click();
      await dialog.getByRole("button", { name: t.discard, exact: true }).click();
      assert.equal(await input(t.name).inputValue(), "Synthetic product A");
    });
    await run("Section keyboard and number fields", "click Basics, End, edit price/stock, save, reload", "Pricing active, numbers/currency persist", async () => {
      await tab(t.general).click();
      await page.keyboard.press("End");
      // getAttribute does not wait for the re-render that End triggers; wait for the selected tab, then assert.
      await area.locator('[role="tab"][aria-selected="true"]').filter({ hasText: t.pricing }).waitFor().catch(() => {});
      assert.equal(await tab(t.pricing).getAttribute("aria-selected"), "true", "End selects the last (Pricing) tab");
      await input(t.price).fill("480"); await input(t.inventory).fill("24");
      await save();
      await reloadHydrated();
      assert.equal(await input(t.price).inputValue(), "480", "price persists after reload");
      assert.equal(await input(t.inventory).inputValue(), "24", "inventory persists after reload");
      await capture("form-pricing");
      await page.keyboard.press("Tab");
    });
    await run("Upload add/error/retry/remove", "select Images, toggle failure, choose image, retry, save/reload, remove/save/reload", "real file chooser, per-item error/progress/completion; removal persists", async () => {
      await tab(t.media).click();
      await area.getByRole("checkbox", { name: t.transferFail, exact: true }).check();
      const chooserPromise = page.waitForEvent("filechooser");
      await button(t.browse).click();
      const chooser = await chooserPromise;
      // [FIXTURE] One synthetic valid PNG, passed through the native picker reached by a real Browse click.
      await chooser.setFiles({ name: "demo.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9xkAAAAASUVORK5CYII=", "base64") });
      await seen(text(t.transferError));
      await button(`${t.retry} demo.png`).click();
      await seen(area.getByRole("progressbar"));
      await seen(text(t.uploaded));
      await area.getByRole("checkbox", { name: t.transferFail, exact: true }).uncheck();
      await save();
      await reloadHydrated();
      await seen(text("demo.png"));
      await capture("form-media");
      await button(`${t.remove} demo.png`).click();
      await save();
      await reloadHydrated();
      assert.equal(await text("demo.png").count(), 0);
    });
    await run("Save bar placement", "read computed styles", "mobile: bottom only; desktop: top + sticky bottom", async () => {
      const bars = area.locator('[data-state][role="group"]');
      assert.equal(await bars.count(), 2);
      assert.equal(await bars.first().isVisible(), !mobile);
      assert.equal(await bars.last().evaluate(n => getComputedStyle(n).position), "sticky"); // [READ]
    });
    await run("Form flow and sticky release", "click Basics, native Home/PageDown scrolling", "page scrolls, no inner scroller; footer docks only while form bottom is offscreen", async () => {
      await tab(t.general).click();
      const main = page.locator("#ds-main");
      // [READ] Phones have no Home/PageDown key; under touch emulation Chromium intermittently drops the first
      // keyboard scroll (CI 38764110001: PageDown at scrollY=0, max 322/345, focus #ds-main, no movement in 8s).
      // A wheel at the viewport centre is still native compositor scrolling, and an inner scroller under the
      // pointer would swallow it, so the no-inner-scroller assertion keeps its teeth.
      const nativeScroll = async direction => {
        if (!mobile) return main.press(direction < 0 ? "Control+Home" : "PageDown");
        const size = page.viewportSize();
        await page.mouse.move(size.width / 2, size.height / 2);
        await page.mouse.wheel(0, direction < 0 ? -100000 : Math.round(size.height * 0.8));
      };
      await nativeScroll(-1);
      // [READ] On timeout, report which native scroll stalled and the page's scroll state (CI 38758700306 zh-TW 390px).
      const scrollWait = (label, fn, arg) => page.waitForFunction(fn, arg).catch(async error => {
        const state = await page.evaluate(() => ({ scrollY: window.scrollY, max: document.scrollingElement.scrollHeight - window.innerHeight, active: document.activeElement?.id || document.activeElement?.tagName }));
        throw new Error(`${label} stalled: ${JSON.stringify(state)}; ${error.message}`);
      });
      await scrollWait(mobile ? "wheel to top" : "Control+Home", () => window.scrollY === 0); // [READ] native Home must finish before measuring
      const frame = area.locator('[data-form-frame]');
      const templateCard = await area.getByRole("region", { name: t.templates, exact: true }).elementHandle();
      // [READ] One synchronous layout snapshot: separate boundingBox awaits can straddle native scrolling.
      const measure = () => frame.evaluate((n, next) => ({
        surface: n.querySelector('[data-form-surface]').getBoundingClientRect().toJSON(),
        bar: n.querySelector(':scope > [data-state][role="group"]').getBoundingClientRect().toJSON(),
        frame: n.getBoundingClientRect().toJSON(),
        next: next.getBoundingClientRect().toJSON(),
        gap: parseFloat(getComputedStyle(n).getPropertyValue("--ds-space-6")),
        scrollY: window.scrollY,
        height: window.innerHeight,
        released: n.dataset.footerReleased,
      }), templateCard);
      const content = await area.locator('[data-form-content]').evaluate(n => ({ overflow: getComputedStyle(n).overflowY, height: n.clientHeight, scroll: n.scrollHeight })); // [READ]
      assert.equal(content.overflow, "visible");
      assert.ok(content.scroll <= content.height + 1, "content has natural height");
      const panel = await area.getByRole("tabpanel").evaluate(n => ({ overflow: getComputedStyle(n).overflowY, height: n.clientHeight, scroll: n.scrollHeight })); // [READ]
      assert.equal(panel.overflow, "visible");
      assert.ok(panel.scroll <= panel.height + 1, "panel has no clipped content");
      const panelEdges = await area.getByRole("tabpanel").evaluate(n => {
        const style = getComputedStyle(n);
        const content = n.closest('[data-form-content]').getBoundingClientRect();
        const box = n.getBoundingClientRect();
        return { border: [style.borderTopWidth, style.borderRightWidth, style.borderBottomWidth, style.borderLeftWidth].map(parseFloat), inset: [box.left - content.left, content.right - box.right], padding: parseFloat(getComputedStyle(n.closest('[data-form-content]')).paddingLeft) };
      }); // [READ] no nested panel outline; one card-owned inset
      assert.ok(panelEdges.border.every(width => width === 0), `nested panel border: ${JSON.stringify(panelEdges)}`);
      assert.ok(panelEdges.inset.every(inset => Math.abs(inset - panelEdges.padding) <= 1), `panel is flush with the card content: ${JSON.stringify(panelEdges)}`);
      let boxes = await measure();
      const height = boxes.height;
      const geometry = () => `scrollY=${boxes.scrollY} surface=${boxes.surface.bottom} bar=${boxes.bar.bottom} frame=${boxes.frame.bottom} next=${boxes.next.top} released=${boxes.released}`;
      const checkGap = () => {
        const { next, gap } = boxes;
        const edge = boxes.surface.y + boxes.surface.height > height ? boxes.surface.y + boxes.surface.height : boxes.bar.y + boxes.bar.height;
        assert.ok(Math.abs(next.y - edge - gap) <= 1, `form-to-template gap=${next.y - edge}px, expected=${gap}px; ${geometry()}`);
        assert.ok(Math.abs(boxes.frame.bottom - edge) <= 1, `no displaced footer band; ${geometry()}`);
      };
      checkGap();
      const initialGeometry = geometry();
      if (boxes.surface.y + boxes.surface.height > height) {
        assert.ok(Math.abs(boxes.bar.y + boxes.bar.height - height) <= 1, "footer docks at viewport bottom");
      }
      for (let i = 0; i < 4 && boxes.surface.y + boxes.surface.height > height; i++) {
        const before = await page.evaluate(() => window.scrollY); // [READ]
        await nativeScroll(1);
        await scrollWait(`${mobile ? "wheel" : "PageDown"} #${i + 1} from ${before}`, y => window.scrollY > y, before); // [READ] wait for native scrolling
        boxes = await measure();
      }
      assert.ok(boxes.surface.y + boxes.surface.height <= height, "form bottom reached by page scrolling");
      assert.ok(boxes.bar.y >= boxes.surface.y + boxes.surface.height - 1, `footer releases below the surface: ${JSON.stringify(boxes)}`);
      checkGap();
      const releasedGeometry = geometry();
      await capture("form-released");
      const toggle = area.getByRole("switch", { name: t.enabled, exact: true });
      const original = await toggle.getAttribute("aria-checked");
      await toggle.click();
      assert.notEqual(await toggle.getAttribute("aria-checked"), original);
      await toggle.click();
      assert.equal(await toggle.getAttribute("aria-checked"), original);
      await page.reload({ waitUntil: "load" });
      assert.equal(await area.getByRole("tab", { name: t.general, exact: true }).getAttribute("aria-selected"), "true");
      return `initial: ${initialGeometry}; released: ${releasedGeometry}`;
    });
  }
  if (template === "settings") {
    await run("Profile fields and save", "edit name/currency/timezone, save, reload", "all selections persist without locale altering values", async () => {
      assert.equal(await area.getByLabel(t.storeName, { exact: true }).count(), 1, "required store name retains its exact visible label");
      const widths = [];
      for (const label of [t.storeName, t.currency, t.timezone]) {
        const control = area.getByLabel(label, { exact: true });
        assert.equal(await control.evaluate(n => n.closest('[data-width]').dataset.width), "full"); // [READ]
        widths.push((await control.boundingBox()).width);
      }
      assert.ok(Math.abs(widths[1] - widths[2]) <= 1, "same-row controls have equal widths");
      if (mobile) assert.ok(Math.abs(widths[0] - widths[1]) <= 1, "phone rows all use full width");
      await area.getByLabel(t.storeName, { exact: true }).fill("Synthetic store A");
      await area.getByLabel(t.currency, { exact: true }).selectOption("USD");
      await area.getByLabel(t.timezone, { exact: true }).selectOption("UTC");
      await save();
      await reloadHydrated();
      assert.equal(await area.getByLabel(t.storeName, { exact: true }).inputValue(), "Synthetic store A");
      assert.equal(await area.getByLabel(t.currency, { exact: true }).inputValue(), "USD");
      assert.equal(await area.getByLabel(t.timezone, { exact: true }).inputValue(), "UTC");
    });
    await run("Cancel", "edit, click Cancel", "restores saved name", async () => {
      await area.getByLabel(t.storeName, { exact: true }).fill("Discard me");
      await button(t.cancel).click();
      assert.equal(await area.getByLabel(t.storeName, { exact: true }).inputValue(), "Synthetic store A");
    });
    await run("Settings sub-nav + notification", "click Notifications, checkbox, save, reload", "subsection and preference persist", async () => {
      await area.getByRole("navigation", { name: t.nav }).getByRole("link", { name: t.notices, exact: true }).click();
      assert.equal(new URL(page.url()).searchParams.get("setting"), "notices", "notices URL is selected");
      await area.getByRole("checkbox", { name: t.emailNotices, exact: true }).check();
      await save(); await page.reload({ waitUntil: "load" });
      assert.equal(await area.getByRole("checkbox", { name: t.emailNotices, exact: true }).isChecked(), true, "email notice preference survives reload");
      await capture("settings-notices");
      await area.getByRole("navigation", { name: t.nav }).getByRole("link", { name: t.storeProfile, exact: true }).click();
      assert.equal(await area.getByLabel(t.storeName, { exact: true }).inputValue(), "Synthetic store A", "returning to profile restores the saved store name");
    });
  }
  await page.goto(`${new URL(page.url()).origin}${root}/${template}`, { waitUntil: "load" });
  for (const name of ["list", "detail", "form", "settings"]) await run(`Template link ${name}`, "click, reload, return", "correct template route and heading survive reload", async () => {
    const links = area.getByRole("region", { name: t.templates, exact: true });
    await links.getByRole("link", { name: t[name], exact: true }).click();
    await page.waitForURL(new RegExp(`${root}/${name}`));
    await page.reload({ waitUntil: "load" });
    await seen(page.locator(`[data-template="${name}"]`));
    await page.goto(`${new URL(page.url()).origin}${root}/${template}`, { waitUntil: "load" });
  });
  if (template === "form" || template === "settings") await run("Return to component gallery", "click", "component gallery opens", async () => {
    await area.getByRole("link", { name: t.returnGallery, exact: true }).click();
    await page.waitForURL(new RegExp(`/${locale}/ui-gallery$`));
    await seen(page.locator("#sec-buttons"));
  });

}
