// Purpose: The DS-1 real-click journey over every control of the design-system gallery (owner 2026-10-03: real Playwright clicks, visible result, persistence after reload where the state lives in the URL); each step is one click-ledger row.
// Depends on: Playwright Page (click/check/fill/selectOption/keyboard only; page.evaluate only reads), apps/admin/src/ui-gallery-nav.ts, packages/i18n/src/index.ts (locale names), node:assert.
// Used by: tests/admin/ui-gallery-runner.mjs (--browser-ui-gallery).
import assert from "node:assert/strict";
import { localeNames } from "../../packages/i18n/src/index.ts";
import { galleryNav } from "../../apps/admin/src/ui-gallery-nav.ts";

/** Runs the whole journey once for one locale and viewport; `step(control, action, expected, fn)` records a ledger row. */
export async function clickJourney({ page, step, copy: c, locale, mobile }) {
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
    await clickBlocked(b);
    await seen(status(buttons, c.btnDisabledReason));
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

  /* ---- B. the list: filter, selection scope, bulk, row actions, states ---- */
  const table = sec("sec-table");
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
    assert.equal(await orders.evaluate((n) => n === document.activeElement), true); // [READ] re-render must not move focus (drawer effect stays mounted)
    await seen(drawer.getByRole("link", { name: group("orders").items[0].label, exact: true }));
    await orders.click();
    assert.equal(await attr(orders, "aria-expanded"), "false");
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
  await step("Topbar notifications menu", "click, click item", "menu opens, item sets #sec-kpi", async () => {
    await btn(page, c.notifications).click();
    await btn(page, c.kpiUnread).click();
    await page.waitForURL(/#sec-kpi$/);
  });
  await step("Topbar help link", "click", "URL hash #sec-alerts", async () => {
    await page.getByRole("link", { name: c.help, exact: true }).click();
    await page.waitForURL(/#sec-alerts$/);
  });
  await step("Breadcrumb link", "click", "URL hash #sec-tokens", async () => {
    await page.getByRole("navigation", { name: c.crumbLabel }).getByRole("link", { name: c.crumbHome, exact: true }).click();
    await page.waitForURL(/#sec-tokens$/);
  });
  await step("Topbar Enter my store", "click", "opens a new tab (target=_blank)", async () => {
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
    await btn(page, c.account).click();
    await btn(page, c.setItemProfile).click();
    await page.waitForURL(/setting=profile/);
  });
  const other = locale === "en" ? "zh-TW" : "en";
  await step("Topbar language links", `click ${localeNames[other]}`, `URL /${other}/ui-gallery and its own heading`, async () => {
    await page.getByRole("navigation", { name: c.localeLabel }).getByRole("link", { name: localeNames[other], exact: true }).click();
    await page.waitForURL(new RegExp(`/${other}/ui-gallery`));
    await seen(page.getByRole("heading", { level: 1 }));
  });
}
