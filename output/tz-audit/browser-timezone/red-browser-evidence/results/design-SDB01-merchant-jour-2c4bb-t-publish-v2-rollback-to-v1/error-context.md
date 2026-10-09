# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: design.spec.ts >> SDB01 merchant journey zh-TW mobile: profile, logo, sections, page, save, guard, publish v1, edit, publish v2, rollback to v1
- Location: tests/admin/design.spec.ts:108:3

# Error details

```
Error: expect(locator).toHaveText(expected) failed

Locator:  getByTestId('design-version-row').first().locator('td').nth(2)
Expected: "2026/10/08 00:04"
Received: "2026/10/07 09:04"
Timeout:  10000ms

Call log:
  - Expect "toHaveText" getByTestId('design-version-row').first().locator('td').nth(2) with timeout 10000ms
  - waiting for getByTestId('design-version-row').first().locator('td').nth(2)
    24 × locator resolved to <td>2026/10/07 09:04</td>
       - unexpected value "2026/10/07 09:04"

```

```yaml
- cell "2026/10/07 09:04"
```

# Test source

```ts
  203 |     await expect(pageBlock.getByRole("alert").first()).toBeVisible();
  204 |     await expect(preview.locator("img")).toHaveCount(0);
  205 |     await expect(preview.locator("a")).toHaveCount(0);
  206 |     expect(await preview.evaluate((el) => el.innerHTML)).not.toContain("<img");
  207 |     // G-UI8 audit [READ/MEASURE]: reads the XSS canary flag the hostile markdown must not set
  208 |     expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();
  209 |     await page.getByTestId("design-save").click();
  210 |     await expect(note).toHaveText(c.needsSave); // slug "Bad Slug" is refused before anything is sent
  211 |     await expect(status).toContainText(c.noDraft);
  212 |     await pageBlock.getByLabel(c.pages.slug).fill("about");
  213 |     await pageBlock.locator("textarea").fill("About **us**\n\n- one\n- two\n\n[our site](https://example.com)");
  214 |     await expect(preview.locator("strong")).toHaveText("us");
  215 |     await expect(preview.locator("li")).toHaveCount(2);
  216 |     await expect(preview.locator("a")).toHaveAttribute("href", "https://example.com");
  217 |     await expect(preview.locator("a")).toHaveAttribute("rel", /noopener/);
  218 |     await shot(page, "pages", j);
  219 |
  220 |     // ---- save draft: one PUT, only {expected_version, document}, keyed, CSRF'd, no HTML in the document ----
  221 |     const [put] = await Promise.all([
  222 |       page.waitForRequest((r) => r.method() === "PUT" && /\/design\/draft$/.test(r.url())),
  223 |       page.getByTestId("design-save").click(),
  224 |     ]);
  225 |     expect(Object.keys(put.postDataJSON()).sort()).toEqual(["document", "expected_version"]);
  226 |     expect(put.postDataJSON().expected_version).toBe(0);
  227 |     expect(put.postDataJSON().document.profile.name).toBe(name);
  228 |     expect(put.headers()["idempotency-key"]).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  229 |     expect(put.headers()["x-csrf-token"]).toMatch(/^[A-Za-z0-9_-]{43}$/);
  230 |     expect(JSON.stringify(put.postDataJSON().document)).not.toContain("<");
  231 |     await expect(note).toHaveText(c.okSaved);
  232 |     await expect(status).toContainText(c.saved);
  233 |     await expect(status).toContainText(fill(c.draftVersion, { n: 1 }));
  234 |     await expect(status).toContainText(c.neverPublished);
  235 |
  236 |     // ---- unsaved-changes guard (beforeunload hook, in-app navigation confirm: stay and leave) ----
  237 |     // at 390 px the rail is an off-canvas drawer behind the menu button; at desktop width it is always visible
  238 |     // This fixture already has integration:read; navigation must exercise the dirty
  239 |     // guard without granting a different domain solely to reach a destination.
  240 |     const goSettings = async () => {
  241 |       if (mobile && (await page.locator('button[aria-controls="workspace-navigation"]').getAttribute("aria-expanded")) !== "true") await page.locator('button[aria-controls="workspace-navigation"]').click();
  242 |       await page.getByTestId("nav-group-settings").click();
  243 |     };
  244 |     // G-UI8 audit [READ/MEASURE]: reads whether the page registered a blocking beforeunload guard (synthetic event only measures defaultPrevented); the real navigation click follows in goSettings()
  245 |     const beforeUnloadPrevented = () => page.evaluate(() => { const e = new Event("beforeunload", { cancelable: true }); window.dispatchEvent(e); return e.defaultPrevented; });
  246 |     expect(await beforeUnloadPrevented(), "clean draft must not warn").toBe(false);
  247 |     await page.getByTestId("design-tab-profile").click();
  248 |     await page.getByLabel(c.profile.tagline, { exact: true }).fill("a tagline nobody saved");
  249 |     await expect(status).toContainText(c.unsaved);
  250 |     expect(await beforeUnloadPrevented(), "dirty draft must warn on unload").toBe(true);
  251 |     dlg.answer("dismiss");
  252 |     await goSettings();
  253 |     await expect.poll(() => dlg.seen.filter((d) => d.message === c.guard).length).toBe(1);
  254 |     await expect(page).toHaveURL(new RegExp(`/${j.locale}/design`));
  255 |     await expect(page.getByLabel(c.profile.tagline, { exact: true })).toHaveValue("a tagline nobody saved");
  256 |     // Canceling navigation leaves the modal drawer open. Return focus to the
  257 |     // editor before editing; its inert content must not receive input behind it.
  258 |     if (mobile) {
  259 |       await page.keyboard.press("Escape");
  260 |       await expect(page.locator('button[aria-controls="workspace-navigation"]')).toHaveAttribute("aria-expanded", "false");
  261 |     }
  262 |     // undoing the edit makes the page clean again: no warning, navigation goes through without a dialog
  263 |     await page.getByLabel(c.profile.tagline, { exact: true }).fill("");
  264 |     await expect(status).toContainText(c.saved);
  265 |     expect(await beforeUnloadPrevented()).toBe(false);
  266 |     const before = dlg.seen.length;
  267 |     await goSettings();
  268 |     await expect(page).toHaveURL(new RegExp(`/${j.locale}/settings`));
  269 |     expect(dlg.seen.length).toBe(before);
  270 |     // dirty again and leave on purpose: the edit is gone and the saved draft is what comes back
  271 |     await page.goto(`/${j.locale}/design?store=${j.id}`);
  272 |     await page.getByLabel(c.profile.tagline, { exact: true }).fill("discard me");
  273 |     dlg.answer("accept");
  274 |     await goSettings();
  275 |     await expect(page).toHaveURL(new RegExp(`/${j.locale}/settings`));
  276 |     expect(dlg.seen.filter((d) => d.message === c.guard).length).toBe(2);
  277 |     await page.goto(`/${j.locale}/design?store=${j.id}`);
  278 |     await expect(status).toContainText(fill(c.draftVersion, { n: 1 }));
  279 |     await expect(page.getByLabel(c.profile.tagline, { exact: true })).toHaveValue("");
  280 |     await expect(page.getByTestId("design-name")).toHaveValue(name);
  281 |
  282 |     // ---- publish v1 ----
  283 |     const publish = page.getByTestId("design-publish");
  284 |     await expect(publish).toBeEnabled();
  285 |     const [pub1, versionsReply] = await Promise.all([
  286 |       page.waitForRequest((r) => r.method() === "POST" && /\/design\/publish$/.test(r.url())),
  287 |       page.waitForResponse((r) => r.request().method() === "GET" && /\/design\/versions$/.test(new URL(r.url()).pathname)),
  288 |       publish.click(),
  289 |     ]);
  290 |     expect(versionsReply.status()).toBe(200);
  291 |     const publishedVersions = await versionsReply.json() as { items: { version: number; published_at: string }[] };
  292 |     const publishedAt = publishedVersions.items.find((v) => v.version === 1)?.published_at;
  293 |     expect(publishedAt).toBeTruthy();
  294 |     expect(pub1.postDataJSON()).toEqual({ expected_draft_version: 1 });
  295 |     await expect(note).toHaveText(fill(c.okPublished, { n: 1 }));
  296 |     expect(dlg.seen.at(-1)?.message).toBe(fill(c.publishConfirm, { n: 1 }));
  297 |     await expect(status).toContainText(fill(c.liveVersion, { n: 1 }));
  298 |     await expect(publish).toBeDisabled(); // nothing new to publish
  299 |     await page.getByTestId("design-tab-versions").click();
  300 |     await expect(rows).toHaveCount(1);
  301 |     await expect(rows.first()).toContainText("v1");
  302 |     await expect(rows.first()).toContainText(c.versions.live);
> 303 |     await expect(rows.first().locator("td").nth(2)).toHaveText(displayTime(j.locale, publishedAt!));
      |                                                     ^ Error: expect(locator).toHaveText(expected) failed
  304 |     if (j.locale === "en" && j.viewport === "desktop") {
  305 |       // The already published store is read in a UTC mobile context; no second fixture or publish.
  306 |       const utcContext = await page.context().browser()!.newContext({
  307 |         baseURL: origin, storageState: await page.context().storageState(), timezoneId: "UTC",
  308 |         viewport: { width: 390, height: 844 },
  309 |       });
  310 |       try {
  311 |         const utcPage = await utcContext.newPage();
  312 |         expect(await utcPage.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone)).toBe("UTC");
  313 |         await utcPage.goto(`/${j.locale}/design?store=${j.id}`);
  314 |         await utcPage.getByTestId("design-tab-versions").click();
  315 |         await expect(utcPage.getByTestId("design-version-row").first().locator("td").nth(2)).toHaveText(displayTime(j.locale, publishedAt!));
  316 |         await utcPage.reload();
  317 |         await utcPage.getByTestId("design-tab-versions").click();
  318 |         await expect(utcPage.getByTestId("design-version-row").first().locator("td").nth(2)).toHaveText(displayTime(j.locale, publishedAt!));
  319 |       } finally {
  320 |         await utcContext.close();
  321 |       }
  322 |     }
  323 |     await expect(rows.first().getByTestId("design-rollback")).toHaveCount(0);
  324 |     await shot(page, "versions-v1", j);
  325 |
  326 |     // ---- edit again and publish v2 (a dirty page is saved first, as draft v2, then published) ----
  327 |     await page.getByTestId("design-tab-profile").click();
  328 |     await page.getByTestId("design-name").fill(`${name} two`);
  329 |     await expect(publish).toBeEnabled();
  330 |     await publish.click();
  331 |     await expect(note).toHaveText(fill(c.okPublished, { n: 2 }));
  332 |     expect(dlg.seen.at(-1)?.message).toBe(fill(c.publishConfirm, { n: 2 }));
  333 |     await expect(status).toContainText(fill(c.draftVersion, { n: 2 }));
  334 |     await expect(status).toContainText(fill(c.liveVersion, { n: 2 }));
  335 |     await page.getByTestId("design-tab-versions").click();
  336 |     await expect(rows).toHaveCount(2);
  337 |     await expect(rows.nth(0)).toContainText("v2");
  338 |     await expect(rows.nth(0)).toContainText(c.versions.live);
  339 |     await expect(rows.nth(0).getByTestId("design-rollback")).toHaveCount(0);
  340 |     await expect(rows.nth(1)).toContainText("v1");
  341 |     await expect(rows.nth(1).getByTestId("design-rollback")).toBeVisible();
  342 |
  343 |     // ---- rollback to v1: a NEW version v3 (kind rollback, from v1); history untouched; the draft is not touched ----
  344 |     const [rb] = await Promise.all([
  345 |       page.waitForRequest((r) => r.method() === "POST" && /\/design\/rollback$/.test(r.url())),
  346 |       rows.nth(1).getByTestId("design-rollback").click(),
  347 |     ]);
  348 |     expect(rb.postDataJSON()).toEqual({ version: 1 });
  349 |     expect(dlg.seen.at(-1)?.message).toBe(fill(c.versions.rollbackConfirm, { n: 1 }));
  350 |     await expect(note).toHaveText(fill(c.okRolledBack, { n: 3 }));
  351 |     await expect(rows).toHaveCount(3);
  352 |     await expect(rows.nth(0)).toContainText("v3");
  353 |     await expect(rows.nth(0)).toContainText(c.versions.kind.rollback);
  354 |     await expect(rows.nth(0)).toContainText(fill(c.versions.source, { n: 1 }));
  355 |     await expect(rows.nth(0)).toContainText(c.versions.live);
  356 |     await expect(rows.nth(0).getByTestId("design-rollback")).toHaveCount(0);
  357 |     await expect(rows.nth(1).getByTestId("design-rollback")).toBeVisible();
  358 |     await expect(rows.nth(2).getByTestId("design-rollback")).toBeVisible();
  359 |     await expect(status).toContainText(fill(c.liveVersion, { n: 3 }));
  360 |     await expect(status).toContainText(fill(c.draftVersion, { n: 2 }));
  361 |     await shot(page, "versions-v3", j);
  362 |
  363 |     // ---- everything survives a reload; the draft still has the v2 edit ----
  364 |     const reloadedVersions = page.waitForResponse((r) => r.request().method() === "GET" && /\/design\/versions$/.test(new URL(r.url()).pathname));
  365 |     await page.reload();
  366 |     const reloadedReply = await reloadedVersions;
  367 |     expect(reloadedReply.status()).toBe(200);
  368 |     const reloadedList = await reloadedReply.json() as { items: { version: number; published_at: string }[] };
  369 |     expect(reloadedList.items.find((v) => v.version === 1)?.published_at).toBe(publishedAt);
  370 |     await expect(status).toContainText(fill(c.draftVersion, { n: 2 }));
  371 |     await expect(status).toContainText(fill(c.liveVersion, { n: 3 }));
  372 |     await expect(page.getByTestId("design-name")).toHaveValue(`${name} two`);
  373 |     await page.getByTestId("design-tab-versions").click();
  374 |     await expect(rows).toHaveCount(3);
  375 |     await expect(rows.nth(2).locator("td").nth(2)).toHaveText(displayTime(j.locale, publishedAt!));
  376 |     expect(dlg.seen.filter((d) => d.type === "beforeunload").length).toBe(0);
  377 |     expect(errors).toEqual([]);
  378 |   });
  379 | }
  380 |
  381 | // ---- BFF probes: what may reach Go, and what must stop at the BFF (the Go-side proxy counts what arrived) ----
  382 | type Call = { method: string; path: string; length: number; leaked: boolean };
  383 | const goCalls = async (): Promise<Call[]> => (await fetch(`${apiOrigin}/__test/design-calls`)).json() as Promise<Call[]>;
  384 | type Probe = { method: string; path: string; body?: string; headers?: Record<string, string>; multipart?: { size: number; bytes?: number[]; type: string }; csrf?: boolean | string; key?: boolean; creds?: RequestCredentials };
  385 | async function probe(page: Page, p: Probe) {
  386 |   // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere) (probe() helper)
  387 |   return page.evaluate(async (q) => {
  388 |     const csrf = document.cookie.split(";").map((s) => s.trim()).find((s) => s.startsWith("__Host-commerce_csrf="))?.split("=")[1] ?? "";
  389 |     const headers: Record<string, string> = { ...(q.headers ?? {}) };
  390 |     if (q.csrf !== false) headers["X-CSRF-Token"] = typeof q.csrf === "string" ? q.csrf : csrf;
  391 |     if (q.key !== false && q.method !== "GET") headers["Idempotency-Key"] = crypto.randomUUID();
  392 |     let body: BodyInit | undefined = q.body;
  393 |     if (q.multipart) {
  394 |       const data = q.multipart.bytes ? new Uint8Array(q.multipart.bytes) : new Uint8Array(q.multipart.size);
  395 |       const form = new FormData();
  396 |       form.append("file", new Blob([data], { type: q.multipart.type }), "f.bin");
  397 |       body = form;
  398 |     } else if (q.body !== undefined) headers["Content-Type"] = "application/json";
  399 |     const r = await fetch(q.path, { method: q.method, credentials: q.creds ?? "same-origin", cache: "no-store", headers, body });
  400 |     const bytes = new Uint8Array(await r.arrayBuffer());
  401 |     return { status: r.status, type: r.headers.get("content-type"), cache: r.headers.get("cache-control"), nosniff: r.headers.get("x-content-type-options"), text: new TextDecoder().decode(bytes.slice(0, 400000)), length: bytes.length, b64: bytes.length < 5000 ? btoa(String.fromCharCode(...bytes)) : "" };
  402 |   }, p);
  403 | }
```
