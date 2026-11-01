// Purpose: record real-route C6 frame captures and user-action evidence, including failed RED assertions.
// Depends on: Playwright and native filesystem/path; signed FRAME-COPY.md exported in #472.
// Used by: parity-data-ops.spec.ts and parity-customer-import.spec.ts.
// Invariants: screenshots never establish interaction success; no DOM writes, force clicks or private data.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

export const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
const evidence = process.env.LC_BROWSER_EVIDENCE!;
type Row = { page: string; control: string; operation: string; expected: string; actual: string };
const rows = new WeakMap<Page, Row[]>();

export async function action(page: Page, name: string, operation: string, expected: string, run: () => Promise<void>) {
  const row = { page: new URL(page.url()).pathname, control: name, operation, expected, actual: "FAIL / not observed" };
  const ledger = rows.get(page) ?? []; ledger.push(row); rows.set(page, ledger);
  await test.step(`${name} → ${expected}`, async () => {
    await run(); row.actual = "PASS / observed";
  });
}

export async function click(page: Page, name: string, expected: string, verify: () => Promise<void>) {
  await action(page, name, "click", expected, async () => {
    await page.getByRole("button", { name, exact: true }).click(); await verify();
  });
}

export async function capture(page: Page, frame: string) {
  const file = path.join(evidence, "parity", `${frame}.png`);
  await mkdir(path.dirname(file), { recursive: true });
  await page.screenshot({ path: file, animations: "disabled" });
  await test.info().attach(`Figma ${frame}`, { path: file, contentType: "image/png" });
}

export async function heading(page: Page, name: string) {
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  // READ/MEASURE only, after the signed-copy assertion: RED must identify copy drift before layout drift.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}

export async function login(page: Page) {
  await page.goto(`${origin}/en/`);
  await click(page, "Sign in with identity service", "signed store selector is mounted", () => expect(page.getByTestId("shell-store-selector")).toBeAttached());
}

export function recordEvidence() {
  test.afterEach(async ({ page }, info) => {
    const dir = path.join(evidence, "parity"); await mkdir(dir, { recursive: true });
    const file = path.join(dir, `${info.testId.replace(/[^a-z0-9-]/gi, "-")}-clicks.json`);
    await writeFile(file, JSON.stringify({ test: info.title, status: info.status, clicks: rows.get(page) ?? [] }, null, 2));
    await info.attach("C6 click ledger", { path: file, contentType: "application/json" });
  });
}
