// Purpose: real-click W3-U1b acceptance against isolated contract-shaped MOCK HTTP, run only in CI.
// Depends on: packaged Next admin (started through tests/helpers/next-startup.mjs), Playwright Chromium/API requests, picklist-fixture TLS facade and node:test.
// Used by: test-local.sh --browser-picklist; no evaluate writes, provider effects or PG.
import { browserEvidenceDirectory } from "../browser-evidence.mjs";
import test from "node:test";
import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createWriteStream, type WriteStream } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { pickFixture, picklistTLS, storeID, sessionID } from "./picklist-fixture.mjs";
import { picklistCopy } from "../../apps/admin/lib/picklist-copy.ts";
import { nextAttemptLog, startNextWithPortRetry } from "../helpers/next-startup.mjs";

test(
  "W3-U1b real clicks: selection, print, CSV, confirmation, unknown and scope",
  { timeout: 900000 },
  async () => {
    // #147 CI-7 round 2: matrix-receipts.mjs accepts this suite's node:test summary as browser evidence only because this
    // marker says the suite drives Playwright; lc_prepare's unit/model suites never print it.
    console.log("## matrix-receipts browser-spec");
    const output = browserEvidenceDirectory("picklist");
    await mkdir(output, { recursive: true });
    const fixture = await pickFixture();
    let log: WriteStream | undefined;
    const ledger: { case: string; result: string }[] = [];
    let browser: Awaited<ReturnType<typeof chromium.launch>> | undefined;
    let edge: Awaited<ReturnType<typeof picklistTLS>> | undefined;
    let server: ReturnType<typeof spawn> | undefined;
    try {
      // #116: no reserve-then-free port; the shared helper picks it and relaunches only an automatic-port EADDRINUSE exit
      // (proof: the attempt's log). The TLS edge targets one upstream port, so each attempt gets its own edge.
      const base = await startNextWithPortRetry({}, async ({ port, attempt, track }) => {
        const upstreamBase = `http://127.0.0.1:${port}`;
        log = createWriteStream(nextAttemptLog(`${output}/server.log`, attempt), { flags: "w", mode: 0o600 });
        await once(log, "open");
        await edge?.close();
        // Both browser and APIRequestContext use HTTPS; Secure __Host cookies remain automatic in every probe.
        edge = await picklistTLS(upstreamBase);
        const base = edge.origin;
        const child = (server = spawn(
          process.execPath,
          ["apps/admin/.next/standalone/apps/admin/server.js"],
          {
            stdio: ["ignore", log, log],
            env: {
              ...process.env,
              COMMERCE_UI_PORT: String(port),
              PORT: String(port),
              HOSTNAME: "127.0.0.1",
              COMMERCE_IDENTITY_ENABLED: "1",
              COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1",
              COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
              COMMERCE_PUBLIC_ORIGIN: base,
              COMMERCE_API_ORIGIN: fixture.origin,
              COMMERCE_BFF_KEY: fixture.key,
            },
          },
        ));
        track(child, log);
        for (let n = 0; ; n++) {
          if (child.exitCode !== null || child.signalCode !== null) throw new Error("Next exited before readiness");
          try {
            // Readiness checks the owned internal HTTP listener, without relaxing Node TLS verification globally.
            if ((await fetch(`${upstreamBase}/en/reset`)).ok) return base;
          } catch {}
          if (n === 119) throw new Error("Next readiness timeout");
          await new Promise((r) => setTimeout(r, 500));
        }
      });
      browser = await chromium.launch();
      for (const locale of ["zh-TW", "zh-CN", "en"] as const)
        for (const width of [1440, 390]) {
          const c = picklistCopy[locale],
            context = await browser.newContext({
              viewport: { width, height: 900 },
              acceptDownloads: true,
              ignoreHTTPSErrors: true, // Test-only ephemeral certificate; public origin itself remains HTTPS.
            });
          await context.addCookies(
            ["session", "csrf"].map((kind) => ({
              name: `__Host-commerce_${kind}`,
              value: fixture.token,
              domain: "127.0.0.1",
              path: "/",
              secure: true,
              httpOnly: kind === "session",
              sameSite: "Lax" as const,
            })),
          );
          const page = await context.newPage();
          await page.goto(
            `${base}/${locale}/orders?store=${storeID}&session_id=${sessionID}`,
          );
          const toolbar = page.getByTestId("pick-toolbar");
          await expect(toolbar).toBeVisible();
          const rowCheck = page
            .getByRole("checkbox", { name: new RegExp(`^${c.select}:`) })
            .first();
          await rowCheck.check();
          await expect(page.getByTestId("pick-count")).toHaveText("1");
          await rowCheck.uncheck();
          await expect(page.getByTestId("pick-count")).toHaveText("0");
          await page
            .getByRole("checkbox", { name: c.page, exact: true })
            .check();
          await expect(page.getByTestId("pick-count")).toHaveText("10");
          await page.getByTestId("orders-next").click();
          await expect(
            page.getByRole("checkbox", { name: c.page, exact: true }),
          ).not.toBeChecked();
          await expect(page.getByTestId("pick-count")).toHaveText("10");
          await page
            .getByRole("checkbox", { name: c.page, exact: true })
            .check();
          await expect(page.getByTestId("pick-count")).toHaveText("20");
          await toolbar
            .getByRole("button", { name: c.pick, exact: true })
            .click();
          const dialog = page.getByRole("dialog");
          await expect(dialog).toBeVisible();
          await expect(
            dialog.getByRole("heading", { name: c.summary, exact: true }),
          ).toBeVisible();
          await expect(dialog.locator(".pick-order")).toHaveCount(19);
          await expect(
            dialog.locator(".pick-lines").first().locator("tbody td").last(),
          ).toHaveText("38");
          await expect(
            dialog.getByText(c.notPickable, { exact: false }),
          ).toBeVisible();
          await page.emulateMedia({ media: "print" });
          await expect(page.locator("nav").first()).not.toBeVisible();
          assert.equal(
            await dialog
              .locator("tbody tr")
              .first()
              .evaluate((node) => getComputedStyle(node).breakInside),
            "avoid",
          );
          await page.screenshot({
            path: `${output}/${locale}-${width}-print.png`,
            fullPage: true,
          });
          await page.emulateMedia({ media: "screen" });
          await dialog
            .getByRole("button", { name: c.print, exact: true })
            .click();
          await dialog
            .getByRole("button", { name: c.close, exact: true })
            .click();
          await toolbar
            .getByRole("button", { name: c.export, exact: true })
            .click();
          await expect(toolbar.getByLabel(c.template, {exact:true})).toBeVisible();
          for (const template of [
            "black_cat",
            "hsinchu",
            "chunghwa_post",
            "generic",
          ]) {
            await toolbar
              .getByLabel(c.template, { exact: true })
              .selectOption(template);
            const download = page.waitForEvent("download");
            await toolbar
              .getByRole("button", { name: c.download, exact: true })
              .click();
            const file = await download;
            assert.equal(
              file.suggestedFilename(),
              `${template}-11111111-202610060600.csv`,
            );
            const bytes = await readFile((await file.path())!);
            assert.equal(bytes.subarray(0, 3).toString("hex"), "efbbbf");
            assert.equal(
              bytes.toString("utf8"),
              fixture.state.exports.at(-1)!.csv,
            );
          }
          await page
            .getByRole("dialog")
            .getByRole("button", { name: c.close, exact: true })
            .click();
          const before = fixture.state.batches.length;
          await toolbar
            .getByRole("button", { name: `${c.cvs} (10)`, exact: true })
            .click();
          await toolbar
            .getByRole("button", { name: c.cancel, exact: true })
            .click();
          assert.equal(fixture.state.batches.length, before);
          await toolbar
            .getByRole("button", { name: `${c.cvs} (10)`, exact: true })
            .click();
          await toolbar
            .getByRole("button", { name: c.confirm, exact: true })
            .click();
          await expect(
            toolbar.getByRole("heading", { name: c.done }),
          ).toBeVisible();
          assert.equal(fixture.state.batches.length, before + 1);
          assert.equal(fixture.state.batches.at(-1)!.ids.length, 10);
          await toolbar
            .getByLabel(c.scope, { exact: true })
            .selectOption("session");
          await toolbar
            .getByRole("button", { name: c.pick, exact: true })
            .click();
          await expect(page.getByRole("dialog")).toBeVisible();
          assert.deepEqual(
            fixture.state.requests
              .filter((r) => r.route === "orders/pick-list")
              .at(-1)!.body,
            { session_id: sessionID },
          );
          await page
            .getByRole("dialog")
            .getByRole("button", { name: c.close, exact: true })
            .click();
          await page.screenshot({
            path: `${output}/${locale}-${width}-results.png`,
            fullPage: true,
          });
          assert(
            await page.evaluate(
              () => document.documentElement.scrollWidth <= innerWidth + 1,
            ),
          );
          await toolbar
            .getByRole("button", { name: c.clear, exact: true })
            .click();
          await expect(page.getByTestId("pick-count")).toHaveText("0");
          ledger.push({
            case: `${locale}/${width}: row select/page select/cross-page/print/4 CSV/cancel/confirm/session/clear`,
            result: "PASS",
          });
          await context.close();
        }
      const context = await browser.newContext({ ignoreHTTPSErrors: true });
      await context.addCookies(
        ["session", "csrf"].map((kind) => ({
          name: `__Host-commerce_${kind}`,
          value: fixture.token,
          domain: "127.0.0.1",
          path: "/",
          secure: true,
          httpOnly: kind === "session",
          sameSite: "Lax" as const,
        })),
      );
      const page = await context.newPage(),
        c = picklistCopy.en;
      const deniedBefore = fixture.state.requests.length;
      for (const [path, headers, expected] of [
        [`orders/pick-list`, {}, 403],
        [
          `orders/pick-list`,
          { "X-CSRF-Token": fixture.token, "Idempotency-Key": "forbidden-key" },
          422,
        ],
        [
          `orders/export?template=generic&extra=1`,
          { "X-CSRF-Token": fixture.token },
          422,
        ],
      ] as const) {
        const response = await context.request.post(
          `${base}/api/stores/${storeID}/${path}`,
          {
            headers: { Origin: base, ...headers },
            data: { order_ids: ["33333333-3333-4333-8333-000000000001"] },
          },
        );
        assert.equal(response.status(), expected);
      }
      assert.equal(fixture.state.requests.length, deniedBefore);
      ledger.push({
        case: "BFF invalid key/query/CSRF probes: no backend dispatch",
        result: "PASS",
      });
      fixture.state.count = 510;
      await page.goto(`${base}/en/orders?store=${storeID}`);
      for (let n = 1; n <= 50; n++) {
        await page.getByRole("checkbox", { name: c.page, exact: true }).check();
        await expect(page.getByTestId("pick-count")).toHaveText(String(n * 10));
        if (n < 50) {
          await page.getByTestId("orders-next").click();
          await expect(
            page.getByRole("checkbox", { name: c.page, exact: true }),
          ).not.toBeChecked();
        }
      }
      await expect(
        page.getByRole("button", { name: `${c.cvs} (250)`, exact: true }),
      ).toBeDisabled();
      await page.getByTestId("orders-next").click();
      await expect(
        page.getByRole("checkbox", { name: c.page, exact: true }),
      ).toBeDisabled();
      await expect(page.getByTestId("pick-count")).toHaveText("500");
      ledger.push({
        case: "500 cross-page cap and CVS100 refusal",
        result: "PASS",
      });
      await page.getByRole("button", { name: c.clear, exact: true }).click();
      await page.getByRole("checkbox", { name: c.page, exact: true }).check();
      fixture.state.unknown = true;
      const count = fixture.state.batches.length;
      await page
        .getByRole("button", { name: `${c.cvs} (5)`, exact: true })
        .click();
      await page.getByRole("button", { name: c.confirm, exact: true }).click();
      await expect(page.getByText(c.unknown, { exact: true })).toBeVisible();
      await page.reload();
      await expect(page.getByText(c.unknown, { exact: true })).toBeVisible();
      assert.equal(fixture.state.batches.length, count + 1);
      await expect(
        page.getByRole("button", { name: /Convenience-store labels \(/ }),
      ).toBeDisabled();
      ledger.push({
        case: "HTTP unknown persists across reload; zero automatic resend",
        result: "PASS",
      });
      fixture.state.recovery = "unknown";
      await page
        .getByRole("button", { name: c.checkStatus, exact: true })
        .click();
      await expect(
        page.getByText(`${c.checked}: 0 / 5`, { exact: true }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: /Convenience-store labels \(/ }),
      ).toBeDisabled();
      fixture.state.recovery = "created";
      await page
        .getByRole("button", { name: c.checkStatus, exact: true })
        .click();
      await expect(
        page.getByRole("heading", { name: c.done, exact: true }),
      ).toBeVisible();
      await expect(page.getByText(c.unknown, { exact: true })).toHaveCount(0);
      assert.equal(fixture.state.batches.length, count + 1);
      await page
        .getByRole("button", { name: new RegExp(`^${c.check} ·`) })
        .first()
        .click();
      await expect(page.getByTestId("order-detail")).toBeVisible();
      await expect(page.getByTestId("order-detail")).toContainText(
        "Synthetic frozen item",
      );
      ledger.push({
        case: "recovery GET: UNKNOWN remains blocked, CREATED clears; cross-page order opens via scoped search",
        result: "PASS",
      });
      fixture.state.unknown = false;
      fixture.state.canExport = false;
      fixture.state.canShip = false;
      await page.reload();
      await expect(page.getByTestId("pick-toolbar")).toBeVisible();
      await expect(
        page
          .getByTestId("pick-toolbar")
          .getByRole("button", { name: c.export, exact: true }),
      ).toHaveCount(0);
      await expect(
        page.getByRole("button", { name: /Convenience-store labels \(/ }),
      ).toHaveCount(0);
      ledger.push({
        case: "permission reduction removes export/CVS controls",
        result: "PASS",
      });
      await context.clearCookies();
      await page.reload();
      await expect(page.getByTestId("pick-toolbar")).toHaveCount(0);
      await context.close();
      assert(
        fixture.state.requests
          .filter((r) => r.route !== "shipments/cvs-batch")
          .every((r) => r.key === null),
      );
      assert.equal(
        new Set(fixture.state.batches.map((r) => r.key)).size,
        fixture.state.batches.length,
      );
    } finally {
      await writeFile(
        `${output}/click-ledger.json`,
        JSON.stringify({ tier: "MOCK", ledger }, null, 2),
      );
      await browser?.close();
      await edge?.close();
      if (server) {
        const owned = server;
        owned.kill("SIGTERM");
        await Promise.race([
          new Promise((r) => owned.once("exit", r)),
          new Promise((r) => setTimeout(r, 5000)),
        ]);
        if (owned.exitCode === null && owned.signalCode === null)
          owned.kill("SIGKILL");
      }
      log?.end();
      await fixture.close();
    }
  },
);
