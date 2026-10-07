// Purpose: Independent W6-01B real-click catalogue/tag/note acceptance, CAS races, draft preservation and UNKNOWN replay.
// Depends on: Playwright, customer-tags-copy labels only, genuine browser_w6_customers_reports_test.go and LC_W6UI_*.
// Used by: TestBrowserW6Customers / extended --browser-customers-billing. Fixture controls prepare/fault/query only.
// Invariants: I01/I02/I06/I11/I14/I18; no auth-cookie injection, force clicks, DOM changes or API writes as UI evidence.
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { customerTagsCopy } from "../../apps/admin/lib/customer-tags-copy";

const required = (key: string) => { const value = process.env[key]; if (!value) throw new Error(`${key} required`); return value; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE"), store = required("LC_W6UI_STORE");
const customer = required("LC_W6UI_CUSTOMER"), second = required("LC_W6UI_SECOND"), control = required("LC_W6UI_CONTROL_URL"), key = required("LC_W6UI_CONTROL_KEY");
type Row = { page: string; control: string; operation: string; expected: string; actual: string; result: "pass" | "fail" };
const ledger: Row[] = [];
test.describe.configure({ mode: "serial" });
test.afterEach(async () => { await writeFile(path.join(evidence, "w6ui-click-ledger.json"), JSON.stringify(ledger, null, 2)); });
async function step(page: Page, controlName: string, expected: string, action: () => Promise<void>, operation = "click") {
  const row: Row = { page: new URL(page.url()).pathname, control: controlName, operation, expected, actual: "", result: "fail" }; ledger.push(row);
  try { await action(); row.result = "pass"; row.actual = expected; } catch (error) { row.actual = error instanceof Error ? error.message.slice(0, 240) : "assertion failed"; throw error; }
}
async function ctl(resource: string, mode = "") {
  // Test-only preparation or bounded fault. Actual note/tag writes being accepted still originate in clicks below.
  const response = await fetch(`${control}/${resource}`, { method: "POST", headers: { "X-W6UI-Gate-Key": key, "Content-Type": "application/json" }, body: JSON.stringify({ mode }) });
  expect(response.status, `fixture ${resource}`).toBe(204);
}
async function state(): Promise<{ unknown_notes: number; note_posts: number }> {
  // Read-only fixture status proves that an uncertain write has no automatic replay; no business API mutation here.
  const response = await fetch(`${control}/state`, { method: "POST", headers: { "X-W6UI-Gate-Key": key, "Content-Type": "application/json" }, body: JSON.stringify({ mode: "" }) });
  expect(response.status).toBe(200); return response.json() as Promise<{ unknown_notes: number; note_posts: number }>;
}
async function signed(browser: Browser, locale: "en" | "zh-TW", width: number, actor = "owner") {
  await ctl("actor", actor);
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width, height: width === 390 ? 844 : 900 } });
  const page = await context.newPage(); await page.goto(`${origin}/en/`);
  await step(page, "Sign in with identity service", "signed callback opens the merchant workspace", async () => {
    await page.getByRole("button", { name: "Sign in with identity service" }).click(); await page.getByTestId("shell-store-selector").waitFor(); // sign-out sits in the collapsed Account popover (WorkspaceFrame <details>), never visible here
  });
  expect((await context.cookies()).some((c) => c.name.startsWith("__Host-") && c.secure && c.httpOnly)).toBe(true);
  await detail(page, locale); return { page, context };
}
async function detail(page: Page, locale: string) {
  await page.goto(`${origin}/${locale}/customers/${customer}?store=${store}`);
  await expect(page.getByTestId("customer-notes")).toBeVisible();
}
async function fill(page: Page, field: Locator, value: string, name: string) {
  await step(page, name, "typed draft is visible", async () => { await field.fill(value); await expect(field).toHaveValue(value); }, "fill");
}
async function fits(page: Page) {
  // READ/MEASURE only: no DOM or application-state mutation.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(1);
}
const noteRow = (page: Page, body: string) => page.getByTestId("customer-notes").locator("ul > li").filter({ has: page.locator("p.ct-note-body", { hasText: body }) });

test("CTUI calibration-sensitive note create uses real mutation response", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = customerTagsCopy.en;
  try {
    if (process.env.LC_W6UI_CALIBRATION === "customers-drop-response") await ctl("fault", "drop-note");
    await fill(page, page.getByTestId("customer-notes").getByRole("textbox"), "w6ui-calibration-note", c.noteAdd);
    await step(page, "Add note", "CTUI-RED-NOTE-CONFIRMED: committed create is visibly confirmed", async () => {
      await page.getByTestId("customer-notes").getByRole("button", { name: c.noteAdd, exact: true }).click();
      await expect(noteRow(page, "w6ui-calibration-note"), "CTUI-RED-NOTE-CONFIRMED").toHaveCount(1);
      await expect(page.getByText(c.noteCreated, { exact: true }), "CTUI-RED-NOTE-CONFIRMED").toBeVisible();
    });
  } finally { await context.close(); }
});

for (const locale of ["en", "zh-TW"] as const) for (const width of [1440, 390]) {
  test(`CTUI catalogue, notes and passive-refresh draft preservation ${locale}/${width}`, async ({ browser }) => {
    const c = customerTagsCopy[locale], { page, context } = await signed(browser, locale, width);
    const created = `UI${locale === "en" ? "E" : "T"}${width}`, text = `w6ui-${locale}-${width}`;
    try {
      await expect(page.getByTestId("note-privacy-hint")).toHaveText(c.notePrivacy);
      if (locale === "zh-TW") await expect(page.getByTestId("note-privacy-hint")).toHaveText("備註會在買家要求刪除資料時一起刪除，買家申請匯出資料時也看得到");
      const editor = page.getByTestId("customer-tags-editor"), notes = page.getByTestId("customer-notes");
      await step(page, "New tag", "detail tag creation form opens", async () => { await editor.getByRole("button", { name: c.newTag, exact: true }).click(); await expect(editor.getByLabel(c.nameLabel, { exact: true })).toBeVisible(); });
      await fill(page, editor.getByLabel(c.nameLabel, { exact: true }), created, c.nameLabel);
      await step(page, "Colour", "new tag uses purple", async () => { await editor.getByLabel(c.colorLabel, { exact: true }).selectOption("purple"); await expect(editor.getByLabel(c.colorLabel, { exact: true })).toHaveValue("purple"); }, "selectOption");
      await step(page, "Create tag", "catalogue contains the created tag", async () => { await editor.getByRole("button", { name: c.create, exact: true }).click(); await expect(editor.getByRole("checkbox", { name: created, exact: true })).toBeVisible(); });
      await step(page, "select tag and Save tags", "whole selection persists after refresh", async () => {
        await editor.getByRole("checkbox", { name: created, exact: true }).check(); await editor.getByRole("button", { name: c.editorSave, exact: true }).click();
        await expect(page.getByText(c.editorSaved, { exact: true })).toBeVisible(); await page.reload();
        await expect(page.getByTestId("customer-tags-editor").getByRole("checkbox", { name: created, exact: true })).toBeChecked();
      });
      await fill(page, notes.getByRole("textbox"), text, c.noteAdd);
      await step(page, "Add note", "new note persists and shows staff/time without raw principal ID", async () => {
        await notes.getByRole("button", { name: c.noteAdd, exact: true }).click(); await expect(noteRow(page, text)).toHaveCount(1);
        await page.reload(); await expect(noteRow(page, text)).toHaveCount(1);
        await expect(noteRow(page, text).locator("time").first()).toHaveAttribute("datetime", /\d{4}-\d\d-\d\dT/);
        await expect(noteRow(page, text)).toContainText(c.noteAuthor);
        expect(await noteRow(page, text).textContent()).not.toContain(required("LC_W6UI_PRINCIPAL"));
      });
      await step(page, "Edit note", "edit form starts with saved body", async () => { await noteRow(page, text).getByRole("button", { name: c.noteEdit, exact: true }).click(); await expect(notes.getByRole("textbox")).toHaveValue(text); });
      const draft = `${text}-draft`;
      await fill(page, notes.getByRole("textbox"), draft, "unsaved edit draft");
      await step(page, "Save tags while note draft is open", "passive detail.notes refresh preserves body and edit intent", async () => {
        await editor.getByRole("checkbox", { name: created, exact: true }).uncheck(); await editor.getByRole("button", { name: c.editorSave, exact: true }).click();
        await expect(page.getByText(c.editorSaved, { exact: true })).toBeVisible();
        await expect(notes.getByRole("textbox"), "CTUI-DRAFT-PRESERVED").toHaveValue(draft);
        await expect(notes.getByRole("button", { name: c.noteSave, exact: true })).toBeVisible();
      });
      await step(page, "Save note", "the existing note changes once; no accidental new note", async () => {
        await notes.getByRole("button", { name: c.noteSave, exact: true }).click(); await expect(noteRow(page, draft)).toHaveCount(1); await expect(noteRow(page, text).filter({ hasText: c.noteEdited })).toHaveCount(1);
        await page.reload(); await expect(noteRow(page, draft)).toHaveCount(1);
      });
      await step(page, "Edit then Cancel", "cancel restores saved body without mutation", async () => {
        await noteRow(page, draft).getByRole("button", { name: c.noteEdit, exact: true }).click(); await notes.getByRole("textbox").fill("discard-me");
        await notes.getByRole("button", { name: c.noteCancel, exact: true }).click(); await expect(noteRow(page, draft)).toHaveCount(1); await expect(notes.getByRole("textbox")).toHaveValue("");
      });
      await step(page, "Delete note cancel/confirm", "cancel keeps note; confirm removes it and reload stays removed", async () => {
        await noteRow(page, draft).getByRole("button", { name: c.noteDelete, exact: true }).click();
        await noteRow(page, draft).getByRole("group").getByRole("button", { name: c.noteCancel, exact: true }).click(); await expect(noteRow(page, draft)).toHaveCount(1);
        await noteRow(page, draft).getByRole("button", { name: c.noteDelete, exact: true }).click();
        await noteRow(page, draft).getByRole("group").getByRole("button", { name: c.noteDelete, exact: true }).click(); await expect(noteRow(page, draft)).toHaveCount(0);
        await page.reload(); await expect(noteRow(page, draft)).toHaveCount(0);
      });

      await page.goto(`${origin}/${locale}/customers?store=${store}`);
      await step(page, "Manage tags", "catalogue modal opens", async () => { await page.getByTestId("customers-tag-manage").click(); await expect(page.getByTestId("tag-manage-dialog")).toBeVisible(); });
      const dialog = page.getByTestId("tag-manage-dialog"), item = () => dialog.locator("li").filter({ has: dialog.locator(".ct-badge", { hasText: created }) });
      await step(page, "Rename/recolour", "saved name and green colour persist in catalogue", async () => {
        await item().getByRole("button", { name: `${c.rename} · ${c.colorLabel}`, exact: true }).click();
        await dialog.getByLabel(c.nameLabel, { exact: true }).fill(`${created}R`);
        for (const colour of ["gray", "red", "orange", "yellow", "teal", "blue", "purple", "green"]) {
          await dialog.getByLabel(c.colorLabel, { exact: true }).selectOption(colour); await expect(dialog.getByLabel(c.colorLabel, { exact: true })).toHaveValue(colour);
        }
        await dialog.getByRole("button", { name: c.save, exact: true }).click(); await expect(dialog.locator(".ct-green", { hasText: `${created}R` })).toBeVisible();
      });
      const renamed = dialog.locator("li").filter({ has: dialog.locator(".ct-badge", { hasText: `${created}R` }) });
      await step(page, "Delete tag cancel/confirm", "cancel retains catalogue tag; confirm removes it", async () => {
        await renamed.getByRole("button", { name: c.remove, exact: true }).click(); await dialog.getByRole("group").getByRole("button", { name: c.cancel, exact: true }).click(); await expect(renamed).toHaveCount(1);
        await renamed.getByRole("button", { name: c.remove, exact: true }).click(); await dialog.getByRole("group").getByRole("button", { name: c.remove, exact: true }).click(); await expect(renamed).toHaveCount(0);
      });
      await step(page, "Close tag manager", "modal closes and invocation remains usable", async () => { await dialog.getByRole("button", { name: c.close, exact: true }).click(); await expect(dialog).toHaveCount(0); await page.getByTestId("customers-tag-manage").click(); await page.keyboard.press("Escape"); await expect(dialog).toHaveCount(0); });
      await fits(page);
    } finally { await context.close(); }
  });
}

test("CTUI catalogue duplicate/length, tag filter clear and persistence, notes pagination and cap", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = customerTagsCopy.en;
  try {
    const notes = page.getByTestId("customer-notes");
    for (const fault of ["tag-read-503", "notes-read-503"]) {
      await ctl("fault", fault);
      await step(page, `${fault} Retry`, "failed genuine read is visible and Retry restores controls/notes", async () => {
        await page.reload(); const region = fault === "tag-read-503" ? page.getByTestId("customer-tags-editor") : notes;
        await expect(region).toContainText(c.unavailable); await region.getByRole("button", { name: c.retry, exact: true }).click();
        if (fault === "tag-read-503") await expect(region.getByRole("checkbox", { name: "Seed01", exact: true })).toBeVisible();
        else await expect(region.locator("ul > li")).toHaveCount(50);
      });
    }
    await step(page, "Load more notes", "older notes append without duplicates", async () => {
      await notes.getByRole("button", { name: c.notesMore, exact: true }).click(); await expect(notes.locator("ul > li")).toHaveCount(53);
      const bodies = await notes.locator("p.ct-note-body").allTextContents(); expect(new Set(bodies).size).toBe(bodies.length);
    });
    const editor = page.getByTestId("customer-tags-editor");
    await step(page, "20-tag checkbox cap", "exactly 20 can be selected, remaining unchecked boxes are disabled", async () => {
      for (const checkbox of await editor.getByRole("checkbox").all()) if (await checkbox.isChecked()) await checkbox.uncheck();
      for (let i = 1; i <= 20; i++) await editor.getByRole("checkbox", { name: `Seed${String(i).padStart(2, "0")}`, exact: true }).check();
      await expect(editor.getByRole("checkbox", { name: "Seed21", exact: true })).toBeDisabled();
      await editor.getByRole("button", { name: c.editorSave, exact: true }).click(); await page.reload();
      await expect(page.getByTestId("customer-tags-editor").locator('input[type="checkbox"]:checked')).toHaveCount(20);
    });
    await fill(page, notes.getByRole("textbox"), "x".repeat(1001), "1001-character note");
    await expect(notes.getByRole("button", { name: c.noteAdd, exact: true })).toBeDisabled();
    await fill(page, notes.getByRole("textbox"), "x".repeat(1000), "1000-character note"); await expect(notes.getByRole("button", { name: c.noteAdd, exact: true })).toBeEnabled();
    await fill(page, notes.getByRole("textbox"), "", "clear draft");
    await page.goto(`${origin}/en/customers?store=${store}`);
    const filter = page.getByTestId("customers-tag-filter");
    await step(page, "Filter by tag", "matching first customer only and URL persists", async () => {
      await filter.selectOption({ label: "Seed01" }); await expect(page).toHaveURL(/tag=/);
      await expect(page.getByTestId(`customer-row-${customer}`)).toBeVisible(); await expect(page.getByTestId(`customer-row-${second}`)).toHaveCount(0);
      await page.reload(); await expect(filter.locator("option:checked")).toHaveText("Seed01");
      await filter.selectOption(""); await expect(page.getByTestId(`customer-row-${second}`)).toBeVisible(); await expect(page).not.toHaveURL(/tag=/);
    }, "selectOption");
    await step(page, "customer detail link and browser Back", "real customer link opens detail and Back restores list", async () => {
      await page.getByTestId(`customer-open-${customer}`).click(); await expect(page.getByTestId("customer-notes")).toBeVisible(); await page.goBack(); await expect(page.getByTestId("customers-tag-filter")).toBeVisible();
    });
    await page.getByTestId("customers-tag-manage").click(); const dialog = page.getByTestId("tag-manage-dialog");
    await fill(page, dialog.getByLabel(c.nameLabel, { exact: true }), "z".repeat(21), "21-character name"); await expect(dialog.getByRole("button", { name: c.create, exact: true })).toBeDisabled();
    await fill(page, dialog.getByLabel(c.nameLabel, { exact: true }), "seed01", "case-normalized duplicate");
    await step(page, "Create duplicate", "actual backend 409 tag_exists is visible without duplicate", async () => { await dialog.getByRole("button", { name: c.create, exact: true }).click(); await expect(dialog).toContainText(c.errors.tag_exists); });
    await dialog.getByRole("button", { name: c.close, exact: true }).click();
  } finally { await context.close(); }
});

test("CTUI real CAS conflicts, refresh discards only explicit user intent", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = customerTagsCopy.en;
  try {
    const editor = page.getByTestId("customer-tags-editor"), notes = page.getByTestId("customer-notes");
    await editor.getByRole("checkbox", { name: "Seed01", exact: true }).uncheck();
    await ctl("concurrent-tags");
    await step(page, "stale Save tags", "actual 409 keeps winning tag set and offers explicit Refresh", async () => {
      await editor.getByRole("button", { name: c.editorSave, exact: true }).click(); await expect(page.getByRole("alert")).toContainText(c.errors.version_changed);
      await page.getByRole("button", { name: c.refresh, exact: true }).click(); await expect(editor.getByRole("checkbox", { name: "Seed02", exact: true })).toBeChecked(); await expect(editor.getByRole("checkbox", { name: "Seed01", exact: true })).not.toBeChecked();
    });
    const seed = noteRow(page, "w6ui-seed-note-51");
    await seed.getByRole("button", { name: c.noteEdit, exact: true }).click(); await fill(page, notes.getByRole("textbox"), "w6ui-stale-draft", "old note version draft");
    await ctl("concurrent-note");
    // Saving tags refreshes detail.notes but MUST NOT rebase the old edit's version onto the concurrent version.
    await editor.getByRole("checkbox", { name: "Seed03", exact: true }).check();
    await editor.getByRole("button", { name: c.editorSave, exact: true }).click(); await expect(page.getByText(c.editorSaved, { exact: true })).toBeVisible();
    await expect(notes.getByRole("textbox"), "CTUI-DRAFT-VERSION-FENCE").toHaveValue("w6ui-stale-draft");
    await step(page, "stale Save note", "409 refuses silent version rebase; draft survives until explicit Refresh", async () => {
      await notes.getByRole("button", { name: c.noteSave, exact: true }).click(); await expect(page.getByRole("alert")).toContainText(c.errors.version_changed);
      await expect(notes.getByRole("textbox")).toHaveValue("w6ui-stale-draft"); await page.getByRole("button", { name: c.refresh, exact: true }).click();
      await expect(notes.getByRole("textbox")).toHaveValue(""); await expect(noteRow(page, "w6ui-concurrent-version")).toHaveCount(1);
    });
  } finally { await context.close(); }
});

test("CTUI UNKNOWN explicit same-key retry commits once, revoked session cannot replay", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = customerTagsCopy.en;
  try {
    const notes = page.getByTestId("customer-notes"), sent: string[] = [];
    page.on("request", (request) => { if (request.method() === "POST" && new URL(request.url()).pathname.endsWith(`/customers/${customer}/notes`)) sent.push(request.headers()["idempotency-key"] ?? ""); });
    const before = await state(); await ctl("fault", "drop-note"); await fill(page, notes.getByRole("textbox"), "w6ui-unknown-once", c.noteAdd);
    await step(page, "unknown Add note / Retry", "unknown locks controls, no automatic retry, explicit retry keeps exact key", async () => {
      await notes.getByRole("button", { name: c.noteAdd, exact: true }).click(); await expect(page.getByRole("alert")).toContainText(c.unknown);
      await expect(notes.getByRole("textbox")).toBeDisabled();
      await expect.poll(async () => (await state()).unknown_notes).toBe(1); const committed = await state(); expect(committed.note_posts - before.note_posts).toBe(1);
      await page.waitForTimeout(500); expect((await state()).note_posts).toBe(committed.note_posts); expect(sent).toHaveLength(1);
      await page.getByRole("button", { name: c.retry, exact: true }).click(); await expect(noteRow(page, "w6ui-unknown-once")).toHaveCount(1);
      expect(sent).toHaveLength(2); expect(sent[0]).not.toBe(""); expect(sent[1]).toBe(sent[0]); await page.reload(); await expect(noteRow(page, "w6ui-unknown-once")).toHaveCount(1);
    });
    await ctl("fault", "drop-note"); await fill(page, notes.getByRole("textbox"), "w6ui-revoked-unknown", c.noteAdd); await notes.getByRole("button", { name: c.noteAdd, exact: true }).click(); await expect(page.getByRole("alert")).toContainText(c.unknown);
    const pending = await state(); await ctl("revoke");
    await step(page, "Retry after session revocation", "expired boundary cannot submit saved command again", async () => {
      await page.getByRole("button", { name: c.retry, exact: true }).click(); await expect(page.getByRole("alert")).toContainText(c.errors.unauthorized);
      expect((await state()).note_posts).toBe(pending.note_posts);
    });
  } finally { await context.close(); }
});

test("CTUI signed reader and non-privacy writer show conservative note permissions", async ({ browser }) => {
  const c = customerTagsCopy.en;
  for (const actor of ["reader", "writer"]) {
    const { page, context } = await signed(browser, "en", 390, actor);
    try {
      const notes = page.getByTestId("customer-notes"), editor = page.getByTestId("customer-tags-editor");
      if (actor === "reader") {
        await expect(notes).toContainText(c.readOnly); await expect(notes.getByRole("textbox")).toHaveCount(0); await expect(editor.getByRole("checkbox")).toHaveCount(0);
      } else {
        await expect(notes).toContainText(c.ownNotesOnly); await expect(noteRow(page, "w6ui-concurrent-version").getByRole("button", { name: c.noteEdit, exact: true })).toHaveCount(0);
        await fill(page, notes.getByRole("textbox"), "w6ui-writer-own", c.noteAdd);
        await step(page, "writer creates/edits own note", "write-only staff may edit proven current-session own note", async () => {
          await notes.getByRole("button", { name: c.noteAdd, exact: true }).click(); await expect(noteRow(page, "w6ui-writer-own")).toHaveCount(1);
          await noteRow(page, "w6ui-writer-own").getByRole("button", { name: c.noteEdit, exact: true }).click(); await notes.getByRole("textbox").fill("w6ui-writer-edited");
          await notes.getByRole("button", { name: c.noteSave, exact: true }).click(); await expect(noteRow(page, "w6ui-writer-edited")).toHaveCount(1);
        });
        await page.reload(); await expect(noteRow(page, "w6ui-writer-edited").getByRole("button", { name: c.noteEdit, exact: true })).toHaveCount(0);
      }
    } finally { await context.close(); }
  }
});
