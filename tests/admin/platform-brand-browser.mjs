// PS1 brand slice: real navigation on the packaged admin app + existing MOCK identity.
// No form submission, external email, account creation, or provider operation.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import { company, operatedBy, brandedTitle } from "../../apps/admin/lib/company.ts";
import { passwordCopy } from "../../apps/admin/lib/entry-copy.ts";
import { shellCopy } from "../../apps/admin/src/shell-copy.ts";

export async function runBrandGate({ browser, base, output }) {
  await mkdir(output, { recursive: true });
  const context = await browser.newContext();
  const page = await context.newPage();
  const ledger = [];
  try {
    for (const locale of ["zh-TW", "zh-CN", "en"]) {
      for (const width of [390, 1586]) {
        await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
        await page.goto(`${base}/${locale}/`);
        const verify = async (mode, title) => {
          await page.locator("#password-auth-title").waitFor();
          assert.equal(await page.getByTestId("platform-brand").innerText(), company.productName);
          assert.equal(await page.title(), brandedTitle(title));
          assert.equal(await page.locator("title").count(), 1, "single title, not conflicting metadata");
          assert.equal(await page.getByTestId("operator-footer").innerText(), operatedBy);
          // Measurement only; no DOM mutations or injected styles.
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "no horizontal overflow");
          await page.reload();
          await page.locator("#password-auth-title").waitFor();
          assert.equal(await page.title(), brandedTitle(title), "SSR reload preserves title");
          assert.equal(await page.getByTestId("operator-footer").innerText(), operatedBy);
          await page.screenshot({ path: `${output}/brand-${mode}-${locale}-${width}.png`, fullPage: true, animations: "disabled" });
          ledger.push({ locale, width, page: mode, action: "navigate by link; reload", expected: "exact brand/title/operator; no overflow", result: "PASS" });
        };
        await verify("signin", passwordCopy[locale].signinTitle);
        await page.locator(`a[href="/${locale}/signup"]`).click();
        await verify("signup", shellCopy[locale].signup);
        await page.locator(`a[href="/${locale}/"]`).click();
        await page.locator(`a[href="/${locale}/reset"]`).click();
        await verify("reset", shellCopy[locale].reset);
        await page.locator(`a[href="/${locale}/"]`).click();
        await page.locator("#password-auth-title").waitFor();
        // Real language chooser: the same brand survives user-selected locale.
        const nextLocale = locale === "en" ? "zh-TW" : "en";
        await page.locator(".entry-language select").selectOption(nextLocale);
        await page.waitForURL((url) => url.pathname.replace(/\/$/, "") === `/${nextLocale}`);
        assert.equal(await page.getByTestId("platform-brand").innerText(), company.productName);
        ledger.push({ locale, width, page: "signin", action: `select locale ${nextLocale}`, expected: "route changes; untranslated brand preserved", result: "PASS" });
      }
    }
    await writeFile(`${output}/brand-click-ledger.json`, JSON.stringify(ledger, null, 2));
    console.log(`PASS platform-brand: ${ledger.length} real-click/reload cases, 18 screenshots; MOCK not platform-site PS2`);
  } catch (error) {
    await page.screenshot({ path: `${output}/brand-failure.png`, fullPage: true });
    throw error;
  } finally {
    await context.close();
  }
}
