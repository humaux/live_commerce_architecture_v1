// Purpose: native M7 create/adjust acceptance, isolated from the original immutable members read corpus.
// Depends on: TestBrowserW6Customers members-forms production Next/Go/PG subtest, signed IdP, keyed fixture controls.
// Used by: --browser-customers-billing; controls prepare faults/read SQL only, never replace UI mutation actions.
import { test, expect, type Browser, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { memberFormCopy } from "../../apps/admin/components/members/copy";
import { shellCopy } from "../../apps/admin/src/shell-copy";
const required = (key: string) => {
  const value = process.env[key];
  if (!value) throw Error(`${key} required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"),
  evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_W6UI_FORMS_STORE"),
  alt = required("LC_W6UI_FORMS_ALT"),
  foreign = required("LC_W6UI_FOREIGN_STORE");
const control = required("LC_W6UI_CONTROL_URL"),
  gateKey = required("LC_W6UI_CONTROL_KEY");
type Locale = "en" | "zh-TW" | "zh-CN";
type Row = {
  page: string;
  control: string;
  action: string;
  expected: string;
  actual: string;
  result: "pass" | "fail";
};
const ledger: Row[] = [],
  completed: string[] = [];
test.describe.configure({ mode: "serial" });
test.afterEach(async ({}, info) => {
  if (info.status === "passed") completed.push(info.title);
  await writeFile(
    path.join(evidence, "members-forms-click-ledger.json"),
    JSON.stringify(ledger, null, 2),
  );
});
test.afterAll(async () => {
  await writeFile(
    path.join(evidence, "members-forms-completion.json"),
    JSON.stringify(
      {
        cases: completed,
        classification:
          "REAL_PG native controls; fault/fixture setup separately disclosed",
      },
      null,
      2,
    ),
  );
});
async function step(
  page: Page,
  name: string,
  action: string,
  expected: string,
  run: () => Promise<void>,
) {
  const row: Row = {
    page: new URL(page.url()).pathname,
    control: name,
    action,
    expected,
    actual: "assertion pending",
    result: "fail",
  };
  ledger.push(row);
  try {
    await run();
    row.actual = "user-visible assertion passed";
    row.result = "pass";
  } catch (e) {
    row.actual = "assertion failed; retained native trace";
    throw e;
  }
}
async function ctl(resource: string, mode = "") {
  // Synthetic fixture preparation/fault synchronization only, never a successful UI-write substitute.
  const r = await fetch(`${control}/${resource}`, {
    method: "POST",
    headers: { "X-W6UI-Gate-Key": gateKey, "Content-Type": "application/json" },
    body: JSON.stringify({ mode }),
    signal: AbortSignal.timeout(5000),
  });
  expect(r.status).toBe(204);
}
async function state(member = ""): Promise<Record<string, number>> {
  // Independent Go owner-pool SQL readback; contains counts/numbers only, no buyer text or command keys.
  const r = await fetch(`${control}/forms-state`, {
    method: "POST",
    headers: { "X-W6UI-Gate-Key": gateKey, "Content-Type": "application/json" },
    body: JSON.stringify({ mode: member }),
    signal: AbortSignal.timeout(5000),
  });
  expect(r.status).toBe(200);
  return r.json();
}
const list = (locale: Locale, scope = store) =>
  `${origin}/${locale}/members?store=${scope}`;
const dialog = (page: Page) => page.getByTestId("member-form-dialog");
async function signed(
  browser: Browser,
  locale: Locale = "en",
  width = 1440,
  actor = "forms-points",
) {
  await ctl("actor", actor);
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width, height: 900 },
  });
  const page = await context.newPage();
  await page.goto(`${origin}/en/`);
  await step(
    page,
    "identity sign-in",
    "click",
    "native signed callback",
    async () => {
      await page
        .getByRole("button", { name: "Sign in with identity service" })
        .click();
      await expect(page.getByTestId("shell-store-selector")).toBeVisible();
    },
  );
  await page.goto(list(locale));
  await expect(page.getByTestId("members-list")).toBeVisible();
  return { page, context };
}
async function fill(page: Page, name: string, value: string) {
  const field = page.locator(`[name="${name}"]`);
  await step(page, name, "fill", "entered synthetic value", async () => {
    await field.fill(value);
    await expect(field).toHaveValue(value);
  });
}
async function create(
  page: Page,
  locale: Locale,
  marker: string,
  exercise = false,
) {
  const c = memberFormCopy[locale],
    before = await state();
  await step(
    page,
    "create opener",
    "click",
    "modal and initial field focus",
    async () => {
      await page.getByTestId("member-create-open").click();
      await expect(dialog(page)).toBeVisible();
      await expect(page.locator('[name="name"]')).toBeFocused();
    },
  );
  if (exercise) {
    await step(
      page,
      "create validation",
      "click",
      "first invalid field focused",
      async () => {
        await page.getByTestId("member-form-submit").click();
        await expect(page.locator('[name="name"]')).toBeFocused();
        await expect(dialog(page).getByRole("alert")).toContainText(
          c.invalidName,
        );
      },
    );
    await step(
      page,
      "draft cancel",
      "keyboard Escape",
      "session retained and opener focused",
      async () => {
        await page.keyboard.press("Escape");
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("member-create-open")).toBeFocused();
      },
    );
    await page.getByTestId("member-create-open").click();
    await step(
      page,
      "cancel button",
      "click",
      "draft canceled without signing out",
      async () => {
        await page.getByTestId("member-form-cancel").click();
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("shell-store-selector")).toBeVisible();
      },
    );
    await page.getByTestId("member-create-open").click();
  }
  await fill(page, "name", `M7 Synthetic ${marker}`);
  await fill(page, "contact", marker);
  await fill(
    page,
    "notes",
    "Synthetic long English note for the member form layout.",
  );
  await step(
    page,
    "create submit",
    "click",
    "one acknowledged create and modal closed",
    async () => {
      const settled = page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          new URL(r.url()).pathname === `/api/stores/${store}/members`,
      );
      await page.getByTestId("member-form-submit").click();
      expect((await settled).status()).toBe(201);
      await expect(dialog(page)).not.toBeVisible();
    },
  );
  await step(
    page,
    "contact search",
    "fill/click",
    "created member found through actual search",
    async () => {
      await page.getByTestId("members-search-input").fill(marker);
      await page.getByTestId("members-search-submit").click();
      await expect(
        page
          .getByTestId("members-list")
          .getByRole("link", { name: `M7 Synthetic ${marker}`, exact: true }),
      ).toBeVisible();
    },
  );
  const link = page
    .getByTestId("members-list")
    .getByRole("link", { name: `M7 Synthetic ${marker}`, exact: true });
  const href = await link.getAttribute("href"),
    id = href!.split("/members/")[1].split("?")[0];
  await step(
    page,
    "created member",
    "click/reload",
    "persistent detail after browser refresh",
    async () => {
      await link.click();
      await expect(page.getByTestId("member-detail")).toContainText(marker);
      await page.reload();
      await expect(page.getByTestId("member-detail")).toContainText(marker);
    },
  );
  const after = await state(id);
  expect(after.members).toBe(before.members + 1);
  expect(after.commands).toBe(before.commands + 1);
  expect(after.balance).toBe(0);
  return id;
}
async function review(
  page: Page,
  locale: Locale,
  amount: string,
  negative = false,
) {
  await step(
    page,
    "adjust opener",
    "click",
    "member adjustment fields visible",
    async () => {
      await page.getByTestId("member-adjust-open").click();
      await expect(dialog(page)).toBeVisible();
    },
  );
  await step(
    page,
    "direction",
    "selectOption",
    "correct addition/deduction selected",
    async () => {
      await page.locator('[name="sign"]').selectOption(negative ? "-1" : "1");
      await expect(page.locator('[name="sign"]')).toHaveValue(
        negative ? "-1" : "1",
      );
    },
  );
  await fill(page, "amount", amount);
  await fill(page, "note", "Synthetic adjustment reason");
  await step(
    page,
    "adjust review",
    "click",
    "explicit before/after confirmation",
    async () => {
      await page.getByTestId("member-form-submit").click();
      await expect(dialog(page)).toHaveAttribute("data-step", "confirm");
      await expect(dialog(page)).toContainText(
        memberFormCopy[locale].irreversible,
      );
    },
  );
}
async function confirm(page: Page, balance: number) {
  await step(
    page,
    "adjust confirm",
    "click",
    "authoritative fresh visible balance and ledger",
    async () => {
      await page.getByTestId("member-form-confirm").click();
      await expect(dialog(page)).not.toBeVisible();
      // Calibration seam: removing onSaved/read.reload must fail this assertion BEFORE hard reload.
      await expect(
        page.getByTestId("member-detail").locator("strong"),
      ).toHaveText(String(balance));
      await expect(page.getByTestId("member-ledger")).toContainText(
        "Synthetic adjustment reason",
      );
      await page.reload();
      await expect(
        page.getByTestId("member-detail").locator("strong"),
      ).toHaveText(String(balance));
    },
  );
}
for (const locale of ["en", "zh-TW", "zh-CN"] as const)
  for (const width of [1440, 375])
    test(`REAL_PG forms ${locale}/${width}`, async ({ browser }) => {
      const { page, context } = await signed(browser, locale, width);
      const marker = `M7_SYNTHETIC_${locale}_${width}`;
      const urls: string[] = [],
        messages: string[] = [];
      page.on("request", (r) => urls.push(r.url()));
      page.on("console", (m) => messages.push(m.text()));
      try {
        const id = await create(page, locale, marker, true);
        await review(page, locale, "50");
        await step(
          page,
          "back to edit",
          "click",
          "review returns without mutation",
          async () => {
            await page.getByTestId("member-form-back").click();
            await expect(page.locator('[name="amount"]')).toHaveValue("50");
          },
        );
        await page.getByTestId("member-form-submit").click();
        await confirm(page, 50);
        const first = await state(id);
        expect(first.balance).toBe(50);
        expect(first.sum).toBe(50);
        expect(first.memberEntries).toBe(1);
        await page.getByTestId("member-adjust-open").click();
        await page.locator('[name="sign"]').selectOption("-1");
        await fill(page, "amount", "51");
        await fill(page, "note", "Synthetic adjustment reason");
        await step(
          page,
          "insufficient client amount",
          "click",
          "visible error and no effect",
          async () => {
            await page.getByTestId("member-form-submit").click();
            await expect(dialog(page).getByRole("alert")).toContainText(
              memberFormCopy[locale].insufficient,
            );
            await expect(page.locator('[name="amount"]')).toBeFocused();
          },
        );
        await fill(page, "amount", "20");
        await page.getByTestId("member-form-submit").click();
        await confirm(page, 30);
        const last = await state(id);
        expect(last.balance).toBe(30);
        expect(last.sum).toBe(30);
        expect(last.memberEntries).toBe(2);
        await page.getByTestId("member-adjust-open").click();
        await step(
          page,
          "keyboard containment",
          "keyboard",
          "Tab and ShiftTab stay inside dialog",
          async () => {
            await page.getByTestId("member-form-submit").focus();
            await page.keyboard.press("Tab");
            await expect(page.locator('[name="sign"]')).toBeFocused();
            await page.keyboard.press("Shift+Tab");
            await expect(page.getByTestId("member-form-submit")).toBeFocused();
            await page.keyboard.press("Escape");
            await expect(page.getByTestId("member-adjust-open")).toBeFocused();
          },
        );
        // Read-only measurement/storage inspection; never DOM mutation or UI-action substitution.
        const hygiene = await page.evaluate(() => ({
          width: document.documentElement.scrollWidth,
          viewport: innerWidth,
          storage: [
            ...Object.values(localStorage),
            ...Object.values(sessionStorage),
          ].join("|"),
        }));
        expect(hygiene.width).toBeLessThanOrEqual(hygiene.viewport);
        expect(hygiene.storage.includes(marker)).toBe(false);
        expect(urls.some((u) => u.includes(marker))).toBe(false);
        expect(messages.some((m) => m.includes(marker))).toBe(false);
        await page.screenshot({
          path: path.join(evidence, `members-forms-${locale}-${width}.png`),
          fullPage: true,
        });
      } finally {
        await context.close();
      }
    });
test("REAL_PG forms reader permission and opaque foreign refusal", async ({
  browser,
}) => {
  const { page, context } = await signed(browser, "en", 375, "forms-reader");
  try {
    await expect(page.getByTestId("member-create-open")).toHaveCount(0);
    const before = await state(),
      csrf = (await context.cookies()).find(
        (c) => c.name === "__Host-commerce_csrf",
      )!.value;
    // Supplemental trust-boundary probes, not UI write acceptance: missing UI action cannot be clicked.
    for (const [scope, status] of [
      [store, 403],
      [foreign, 404],
    ] as const) {
      const r = await page.request.post(
        `${origin}/api/stores/${scope}/members`,
        {
          headers: {
            Origin: origin,
            "X-CSRF-Token": csrf,
            "Idempotency-Key": `m7-forbidden-${scope}`,
          },
          data: {
            display_name: "M7 synthetic forbidden",
            contact_note: null,
            notes: null,
          },
        },
      );
      expect(r.status()).toBe(status);
    }
    const after = await state();
    expect(after.members).toBe(before.members);
    expect(after.commands).toBe(before.commands);
    await step(
      page,
      "reader search",
      "click",
      "read UI still works without points actions",
      async () => {
        await page.getByTestId("members-search-submit").click();
        await expect(page.getByTestId("members-list")).toBeVisible();
      },
    );
  } finally {
    await context.close();
  }
});
test("REAL_PG forms create pending double click then committed UNKNOWN never resends", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  const before = await state();
  try {
    await page.getByTestId("member-create-open").click();
    await fill(page, "name", "M7 synthetic dropped create");
    await ctl("fault", "forms-drop");
    await step(
      page,
      "create response lost",
      "click",
      "committed create becomes UNKNOWN",
      async () => {
        await page.getByTestId("member-form-submit").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
      },
    );
    const committed = await state();
    expect(committed.members).toBe(before.members + 1);
    expect(committed.commands).toBe(before.commands + 1);
    await step(
      page,
      "UNKNOWN read only",
      "click/keyboard",
      "refresh and Escape cannot resend or resolve intent",
      async () => {
        await page.getByTestId("member-form-refresh").click();
        await page.keyboard.press("Escape");
        await expect(dialog(page)).toBeVisible();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
        await expect(page.getByTestId("member-form-submit")).toHaveCount(0);
      },
    );
    const after = await state();
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await context.close();
  }
});
test("REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  try {
    const id = await create(page, "en", "M7_UNKNOWN_ADJUST");
    await review(page, "en", "5");
    const before = await state(id);
    await ctl("fault", "forms-drop");
    await page.getByTestId("member-form-confirm").click();
    await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
    const committed = await state(id);
    expect(committed.balance).toBe(5);
    expect(committed.sum).toBe(5);
    expect(committed.memberEntries).toBe(1);
    expect(committed.commands).toBe(before.commands + 1);
    await step(
      page,
      "adjust UNKNOWN refresh",
      "click",
      "read refresh keeps unresolved state and sends no POST",
      async () => {
        await page.getByTestId("member-form-refresh").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
      },
    );
    const after = await state(id);
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await context.close();
  }
});
test("REAL_PG forms held commit suppresses stale store and logout completion", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  const before = await state();
  try {
    await page.getByTestId("member-create-open").click();
    await fill(page, "name", "M7_HELD_PRIVATE");
    await ctl("fault", "forms-hold");
    await step(
      page,
      "pending rapid submit",
      "dblclick/Enter/Escape",
      "one dispatch and inputs/cancel locked",
      async () => {
        await page.getByTestId("member-form-submit").dblclick();
        await expect(dialog(page)).toHaveAttribute("data-step", "pending");
        await page.getByTestId("member-form-submit").press("Enter");
        await page.keyboard.press("Escape");
        await expect(dialog(page)).toHaveAttribute("data-step", "pending");
        await expect(page.locator('[name="name"]')).toBeDisabled();
        await expect(page.getByTestId("member-form-cancel")).toHaveAttribute(
          "aria-disabled",
          "true",
        );
      },
    );
    await expect.poll(async () => (await state()).held).toBe(1);
    const committed = await state();
    expect(committed.members).toBe(before.members + 1);
    expect(committed.writeRequests).toBe(before.writeRequests + 1);
    // Second native tab shares actual cookies; its controls can change scope/logout while pending dialog is modal.
    const other = await context.newPage();
    await other.goto(list("en"));
    await step(
      other,
      "store selector",
      "selectOption",
      "alternate store hides points entry",
      async () => {
        await other.getByTestId("shell-store-selector").selectOption(alt);
        await other.getByTestId("nav-members").click();
        await expect(other.getByTestId("member-create-open")).toHaveCount(0);
      },
    );
    await step(
      other,
      "native logout",
      "click",
      "authenticated session revoked before held response release",
      async () => {
        await other
          .locator("summary")
          .filter({ hasText: shellCopy.en.account })
          .click();
        await other.getByTestId("workspace-sign-out").click();
        await expect(other).toHaveURL(/\/en\/?$/);
      },
    );
    await ctl("forms-release");
    await expect(page.getByTestId("member-form")).toHaveCount(0);
    await expect(
      page.getByText("M7_HELD_PRIVATE", { exact: true }),
    ).toHaveCount(0);
    const after = await state();
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await ctl("forms-release");
    await context.close();
  }
});
test("REAL_PG forms genuine server race refusal and correction controls", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  try {
    const id = await create(page, "en", "M7_RACE");
    await review(page, "en", "50");
    await confirm(page, 50);
    await review(page, "en", "30", true);
    await ctl("forms-race", id);
    const before = await state(id);
    await step(
      page,
      "server concurrent deduction refusal",
      "click",
      "actual409 visible and no second deduction",
      async () => {
        await page.getByTestId("member-form-confirm").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "refused");
        await expect(dialog(page).getByRole("alert")).toContainText(
          memberFormCopy.en.insufficient,
        );
      },
    );
    const after = await state(id);
    expect(after.commands).toBe(before.commands);
    expect(after.memberEntries).toBe(before.memberEntries);
    expect(after.balance).toBe(0);
    await step(
      page,
      "refusal back",
      "click",
      "draft can be corrected",
      async () => {
        await page.getByTestId("member-form-back").click();
        await expect(page.locator('[name="amount"]')).toHaveValue("30");
      },
    );
    await page.getByTestId("member-form-submit").click();
    await page.getByTestId("member-form-confirm").click();
    await expect(dialog(page)).toHaveAttribute("data-step", "refused");
    await step(
      page,
      "refusal close",
      "click",
      "modal closes and opener focused",
      async () => {
        await page.getByTestId("member-form-close").click();
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("member-adjust-open")).toBeFocused();
      },
    );
  } finally {
    await context.close();
  }
});
