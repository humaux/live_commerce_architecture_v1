// Purpose: Assert all fourteen signed domains frames on the real design route with native user actions.
// Depends on: Playwright, signed store-design login, existing domain DTOs and isolated response fixtures.
// Used by: playwright.config.ts store-design suite; --browser-design GitHub CI.
// Invariants: fixtures supply data only; every command comes from clicks; uncertain writes replay the identical journal.
import { expect, test, type Page, type Locator } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

type Locale = "zh-TW" | "zh-CN" | "en";
type State = "REQUESTED" | "OWNERSHIP_PENDING" | "TLS_PENDING" | "ACTIVE" | "SUSPENDED" | "DETACHED";
type Domain = { origin: string; kind: "platform" | "custom"; state: State; version: number; token: string | null; verify_deadline: string | null; serving: boolean };
const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
const store = process.env.LC_BROWSER_BFF_STORE!;
const evidence = process.env.LC_BROWSER_EVIDENCE!;
if (!origin || !store || !evidence) throw new Error("store-design browser fixture required");
const platform = "https://87654321.example.test";
const first = "https://first.example.test";
const second = "second.example.test";
// Synthetic response data, never a credential or a real verification token.
const proof = "domain-fixture-" + "a".repeat(28);
const copy = {
  "zh-TW": { title: "網域設定", entry: "網店網址", platform: "平台網址 · 系統配置", fullPlatform: "查看完整平台網址", add: "新增網域", addTitle: "新增自訂網域", hostname: "網域（不含 https://）", apply: "申請此網域", cancel: "取消", settings: "查看設定", dns: "這筆自訂網域", records: "查看 DNS 記錄", back: "返回網域設定", home: "回網域設定", refresh: "重新讀取狀態", copyHost: "複製主機", copyValue: "複製值", full: "複製完整值", copied: "已複製完整值", return: "返回剛才頁面", suspend: "暫停此網址", detach: "解除此網址", suspendTitle: "暫停自訂網域", detachTitle: "解除自訂網域", confirmSuspend: "確認暫停", confirmDetach: "確認解除", receipt: "操作結果", pending: "先確認這一次操作", retry: "重試原操作", view: "查看目前網域記錄", reapply: "重新申請此網域", missing: "目前缺少完整 DNS 指示", missingIP: "根網域的 IP 尚未提供", readFailed: "狀態未更新", invalid: "請輸入有效的網域名稱，不含 https:// 或路徑。", states: ["等待 DNS 驗證", "等待網域歸屬驗證", "等待 HTTPS 憑證", "已啟用", "已暫停", "已解除"], notServing: "平台網址仍保留；尚未透過此自訂網址服務。", serving: "正在服務買家" },
  "zh-CN": { title: "域名设置", entry: "网店网址", platform: "平台网址 · 系统分配", fullPlatform: "查看完整平台网址", add: "新增域名", addTitle: "新增自定义域名", hostname: "域名（不含 https://）", apply: "申请此域名", cancel: "取消", settings: "查看设置", dns: "这笔自定义域名", records: "查看 DNS 记录", back: "返回域名设置", home: "回域名设置", refresh: "重新加载状态", copyHost: "复制主机", copyValue: "复制值", full: "复制完整值", copied: "已复制完整值", return: "返回上一页", suspend: "暂停此网址", detach: "解绑此网址", suspendTitle: "暂停自定义域名", detachTitle: "解绑自定义域名", confirmSuspend: "确认暂停", confirmDetach: "确认解绑", receipt: "操作结果", pending: "先确认本次操作", retry: "重试原操作", view: "查看当前域名记录", reapply: "重新申请此域名", missing: "当前缺少完整 DNS 指引", missingIP: "尚未提供根域名 IP", readFailed: "状态未更新", invalid: "请输入有效的域名，不含 https:// 或路径。", states: ["等待 DNS 验证", "等待域名归属验证", "等待 HTTPS 证书", "已启用", "已暂停", "已解绑"], notServing: "平台网址仍保留；尚未通过此自定义网址提供服务。", serving: "正在服务买家" },
  en: { title: "Domain settings", entry: "Domains", platform: "Platform address · System assigned", fullPlatform: "View full platform address", add: "Add domain", addTitle: "Add custom domain", hostname: "Domain (without https://)", apply: "Request this domain", cancel: "Cancel", settings: "View settings", dns: "This custom domain", records: "View DNS records", back: "Back to domain settings", home: "Return to domain settings", refresh: "Reload status", copyHost: "Copy host", copyValue: "Copy value", full: "Copy full value", copied: "Full value copied", return: "Return to previous page", suspend: "Suspend this address", detach: "Detach this address", suspendTitle: "Suspend custom domain", detachTitle: "Detach custom domain", confirmSuspend: "Confirm suspension", confirmDetach: "Confirm detachment", receipt: "Operation result", pending: "Confirm this operation first", retry: "Retry original operation", view: "View current domain records", reapply: "Request this domain again", missing: "Complete DNS instructions unavailable", missingIP: "Root domain IP not provided", readFailed: "Status not updated", invalid: "Enter a valid domain without https:// or a path.", states: ["Waiting for DNS verification", "Waiting for ownership verification", "Waiting for HTTPS certificate", "Active", "Suspended", "Detached"], notServing: "The platform address is retained; this custom address is not serving buyers yet.", serving: "Serving buyers" },
};
type ClickRow = { page: string; control: string; action: string; expected: string; actual: string; result: "PASS" | "FAIL" };
const ledger: ClickRow[] = [];
test.use({ baseURL: origin });
test.beforeEach(() => { ledger.length = 0; });
test.afterEach(async ({}, info) => {
  await info.attach("click-ledger", { body: JSON.stringify(ledger, null, 2), contentType: "application/json" });
});
async function click(page: Page, target: Locator, expected: string, verify: () => Promise<void>) {
  const row: ClickRow = { page: new URL(page.url()).pathname, control: await target.innerText(), action: "click", expected, actual: "pending", result: "FAIL" };
  ledger.push(row);
  await target.click();
  await verify();
  row.actual = expected; row.result = "PASS";
}
const row = (domainOrigin: string, state: State = "REQUESTED", kind: Domain["kind"] = "custom", serving = false): Domain => ({ origin: domainOrigin, kind, state, serving, version: 1, token: null, verify_deadline: null });

async function open(page: Page, locale: Locale, mobile: boolean) {
  await page.setViewportSize(mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 });
  const state = { domains: [row(platform, "ACTIVE", "platform", true), row(first)], readsFail: false, uncertain: false, apex: false, failStatus: 0, addresses: ["192.0.2.20", "2001:db8::20"], writes: [] as { key: string; body: string; action: string }[], receipts: new Map<string, unknown>() };
  // Server-shaped MOCK responses only. Real signed login/store scope and product event handlers remain active.
  await page.route(`**/api/stores/${store}/storefront/domains**`, async route => {
    const request = route.request();
    const action = new URL(request.url()).pathname.split("/domains")[1];
    if (request.method() === "GET") {
      if (state.readsFail) return route.fulfill({ status: 503, json: { code: "unavailable" } });
      return route.fulfill({ headers: { "Cache-Control": "private, no-store" }, json: { domains: state.domains } });
    }
    expect(request.method()).toBe("POST");
    const key = request.headers()["idempotency-key"];
    expect(key).toMatch(/^storefront-domain-/);
    const body = request.postData()!;
    state.writes.push({ key, body, action });
    // Definitive server rejection (4xx): no receipt is stored and the outcome is not uncertain.
    if (state.failStatus) return route.fulfill({ status: state.failStatus, json: { code: "rejected" } });
    let result = state.receipts.get(key);
    if (!result) {
      const value = request.postDataJSON();
      if (action === "") {
        expect(Object.keys(value)).toEqual(["hostname"]);
        const address = `https://${value.hostname}`;
        const existing = state.domains.find(d => d.origin === address);
        if (existing) { existing.state = "REQUESTED"; existing.version++; existing.serving = false; }
        else state.domains.push(row(address));
        result = { domain_id: "11111111-1111-4111-8111-111111111111", origin: address, version: existing?.version ?? 1, state: "REQUESTED", dns: { txt_name: `_lc-verify.${value.hostname}`, txt_value: proof, cname_target: "stores.example.test", apex: state.apex, ...(state.apex ? { edge_addresses: state.addresses } : {}) } };
      } else {
        expect(Object.keys(value)).toEqual(["origin"]);
        expect(["/suspend", "/detach"]).toContain(action);
        const domain = state.domains.find(d => d.origin === value.origin)!;
        expect(domain.kind).toBe("custom");
        domain.state = action === "/detach" ? "DETACHED" : "SUSPENDED";
        domain.serving = false; domain.version++;
        result = { ok: true };
      }
      state.receipts.set(key, result);
    } else expect(state.writes.find(w => w.key === key)).toEqual({ key, body, action });
    // Commit then lose response: replay must keep the exact key/body and must not rotate TXT.
    if (state.uncertain) return route.abort("connectionreset");
    await route.fulfill({ json: result });
  });
  await page.goto("/en/");
  await page.getByRole("button", { name: "Sign in with identity service", exact: true }).click();
  await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
  await page.goto(`/${locale}/design?store=${store}`);
  await expect(page.getByTestId("design-page")).toBeVisible();
  await page.getByRole("button", { name: copy[locale].entry, exact: true }).click();
  // Causal RED checkpoint: existing route and response render before checking the signed frame.
  await expect(page.getByTestId("design-domains")).toContainText(first);
  await expect(page.getByRole("heading", { name: copy[locale].title, exact: true }).last()).toBeVisible();
  return state;
}

async function shot(page: Page, frame: string, locale: Locale) {
  if (locale !== "zh-TW") return;
  const directory = path.join(evidence, "parity");
  await mkdir(directory, { recursive: true });
  await page.screenshot({ path: path.join(directory, `${frame.replace(":", "-")}.png`), animations: "disabled" });
  // Read-only measurements, never DOM mutation or a replacement for an action.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}

for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const mobile of [false, true]) {
  test(`PARITY-DOMAINS ${locale}/${mobile ? "mobile" : "desktop"}: all signed states, DNS and persisted commands`, async ({ page }) => {
    test.setTimeout(90000);
    const c = copy[locale], state = await open(page, locale, mobile);
    const panel = page.getByTestId("design-domains");
    const button = (name: string) => panel.getByRole("button", { name, exact: true });
    const heading = (name: string) => panel.getByRole("heading", { name, exact: true });
    await expect(heading(c.platform)).toBeVisible();
    // The old read-only list and settings detour are replaced, not retained alongside the signed screen.
    await expect(page.getByTestId("design-domain-settings")).toHaveCount(0);
    await expect(page.getByTestId("design-domain-platform").getByRole("heading", { name: /^(Store address|系统网址|系統網址)$/ })).toHaveCount(0);
    await expect(panel.getByRole("heading", { name: /^(Custom domain|自有域名)$/ })).toHaveCount(0);
    await shot(page, mobile ? "5:16621" : "5:16526", locale);
    await click(page, button(c.fullPlatform), "full authoritative platform origin", async () => { await expect(panel).toContainText(platform); await expect(heading(c.full)).toBeVisible(); });
    await click(page, button(c.return), "main domains", async () => { await expect(heading(c.title)).toBeVisible(); });
    const count = state.writes.length;
    await click(page, button(c.refresh), "refresh does not write", async () => { await expect(panel).toContainText(first); expect(state.writes.length).toBe(count); });
    state.readsFail = true;
    await click(page, button(c.refresh), "failed read retains last known rows", async () => { await expect(panel).toContainText(c.readFailed); await expect(panel).toContainText(first); });
    state.readsFail = false;
    await click(page, button(c.refresh), "read recovery", async () => { await expect(panel.getByText(c.readFailed, { exact: true })).toHaveCount(0); });
    await click(page, button(c.add), "request form", async () => { await expect(heading(c.addTitle)).toBeVisible(); });
    await shot(page, mobile ? "1076:7029" : "1076:7028", locale);
    await page.getByLabel(c.hostname, { exact: true }).fill("https://invalid.example.test/path");
    await click(page, button(c.apply), "invalid host refuses before POST", async () => { await expect(panel).toContainText(c.invalid); expect(state.writes.length).toBe(count); });
    await click(page, button(c.cancel), "cancel does not change bindings", async () => { await expect(heading(c.title)).toBeVisible(); expect(state.writes.length).toBe(count); });
    await button(c.add).click();
    await page.getByLabel(c.hostname, { exact: true }).fill(second);
    await click(page, button(c.apply), "server request receipt", async () => { await expect(heading(c.receipt)).toBeVisible(); await expect(panel).toContainText(`https://${second}`); });
    await shot(page, mobile ? "1076:7037" : "1076:7036", locale);
    await click(page, button(c.records), "DNS from this request", async () => { await expect(heading(c.dns)).toBeVisible(); await expect(panel).toContainText(proof); });
    await expect(panel).toContainText(`_lc-verify.${second}`);
    await expect(panel).toContainText("stores.example.test");
    await expect(panel.getByTestId("domain-dns-CNAME")).toHaveCount(1);
    await expect(panel.getByTestId("domain-dns-A")).toHaveCount(0);
    await shot(page, mobile ? "1076:7031" : "1076:7030", locale);
    if (process.env.LC_BROWSER_ENGINE !== "webkit") await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
    for (const [type, host, value] of [["CNAME", second, "stores.example.test"], ["TXT", `_lc-verify.${second}`, proof]]) {
      for (const [name, expected] of [[c.copyHost, host], [c.copyValue, value]]) {
        await click(page, panel.getByTestId(`domain-dns-${type}`).getByRole("button", { name, exact: true }), "complete selected value", async () => {
          await expect(heading(c.full)).toBeVisible(); await expect(panel.getByTestId("domain-full-value")).toHaveText(expected);
          if (process.env.LC_BROWSER_ENGINE !== "webkit") { await expect(panel).toContainText(c.copied); expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(expected); }
        });
        await shot(page, mobile ? "1076:7033" : "1076:7032", locale);
        await click(page, button(c.return), "return to selected DNS record", async () => { await expect(heading(c.dns)).toBeVisible(); await expect(panel).toContainText(`https://${second}`); });
      }
    }
    await button(c.back).click();
    await expect(panel).toContainText(first);
    await expect(panel).toContainText(`https://${second}`);
    await page.reload();
    await expect(heading(c.title)).toBeVisible();
    await expect(panel).toContainText(`https://${second}`);
    const secondCard = panel.getByTestId("domain-custom-row").filter({ hasText: `https://${second}` });
    await secondCard.getByRole("button", { name: c.settings, exact: true }).click();
    // A GET is not a complete DNS receipt; no guessed CNAME/IP or stale cross-domain TXT.
    await expect(panel).toContainText(c.missing);
    await expect(panel.getByTestId("domain-dns-CNAME")).toHaveCount(0);
    await expect(panel).not.toContainText(proof);
    await button(c.back).click();
    const target = panel.getByTestId("domain-custom-row").filter({ hasText: first });
    for (const [phase, expected] of (["REQUESTED", "OWNERSHIP_PENDING", "TLS_PENDING", "ACTIVE", "SUSPENDED", "DETACHED"] as const).map((s, i) => [s, c.states[i]] as const)) {
      state.domains[1].state = phase; state.domains[1].serving = false;
      await button(c.refresh).click();
      await expect(target).toContainText(expected);
      await expect(target).not.toContainText(c.serving);
    }
    state.domains[1].state = "ACTIVE"; state.domains[1].serving = true;
    await button(c.refresh).click();
    await expect(target).toContainText(c.serving);
    await target.getByRole("button", { name: c.settings, exact: true }).click();
    for (const action of ["suspend", "detach"] as const) {
      const initialWrites = state.writes.length;
      await button(c[action]).click();
      await expect(heading(action === "detach" ? c.detachTitle : c.suspendTitle)).toBeVisible();
      await expect(panel).toContainText(first);
      if (action === "detach") await shot(page, mobile ? "1076:7035" : "1076:7034", locale);
      await click(page, button(c.cancel), "cancel preserves exact target", async () => { await expect(heading(c.dns)).toBeVisible(); expect(state.writes.length).toBe(initialWrites); });
      await button(c[action]).click();
      await click(page, button(action === "detach" ? c.confirmDetach : c.confirmSuspend), "operation receipt", async () => { await expect(heading(c.receipt)).toBeVisible(); });
      expect(state.domains.find(d => d.origin === `https://${second}`)?.state).toBe("REQUESTED");
      expect(state.domains[0].state).toBe("ACTIVE");
      await button(c.home).click();
      await page.reload();
      await expect(target).toContainText(action === "detach" ? c.states[5] : c.states[4]);
      await target.getByRole("button", { name: c.settings, exact: true }).click();
    }
    await expect(button(c.reapply)).toBeVisible();
    state.uncertain = true;
    await click(page, button(c.reapply), "original operation awaits confirmation", async () => { await expect(heading(c.pending)).toBeVisible(); });
    await shot(page, mobile ? "1076:7039" : "1076:7038", locale);
    await expect(button(c.add)).toHaveCount(0);
    const frozen = state.writes.at(-1)!;
    await page.reload();
    await expect(heading(c.pending)).toBeVisible();
    await expect(panel).toContainText(first);
    const receipts = state.receipts.size;
    state.uncertain = false;
    await click(page, button(c.retry), "same command receipt recovered", async () => { await expect(heading(c.receipt)).toBeVisible(); });
    expect(state.writes.at(-1)).toEqual(frozen);
    expect(state.receipts.size).toBe(receipts);
    await button(c.records).click();
    await expect(panel).toContainText(proof);
    await expect(panel).toContainText(`_lc-verify.${first.slice(8)}`);
    await button(c.back).click();
    // Apex record family and missing-address state, driven only by current response data.
    state.apex = true;
    for (const [host, addresses] of [["apex.example.test", ["192.0.2.20", "2001:db8::20"]], ["missing.example.test", []]] as const) {
      state.addresses = [...addresses];
      await button(c.add).click();
      await page.getByLabel(c.hostname, { exact: true }).fill(host);
      await button(c.apply).click();
      await button(c.records).click();
      await expect(panel.getByTestId("domain-dns-CNAME")).toHaveCount(0);
      if (addresses.length) { await expect(panel.getByTestId("domain-dns-A")).toContainText(addresses[0]); await expect(panel.getByTestId("domain-dns-AAAA")).toContainText(addresses[1]); }
      else { await expect(panel).toContainText(c.missingIP); await expect(panel.getByTestId("domain-dns-A")).toHaveCount(0); }
      await button(c.back).click();
    }
    await expect(page.getByTestId("design-domain-platform").getByRole("button", { name: c.detach, exact: true })).toHaveCount(0);
    await expect(panel).not.toContainText("互動原型");
  });
}

const journalKey = `storefront-domains-cmd:${store}`;
// Read-only measurement of the sessionStorage journal, never a replacement for an action.
const journal = (page: Page) => page.evaluate(k => sessionStorage.getItem(k), journalKey);

test("PARITY-DOMAINS definitive 409 on add clears journal and fence", async ({ page }) => {
  test.setTimeout(90000);
  const c = copy.en, state = await open(page, "en", false);
  const panel = page.getByTestId("design-domains");
  state.failStatus = 409;
  await panel.getByRole("button", { name: c.add, exact: true }).click();
  await page.getByLabel(c.hostname, { exact: true }).fill(second);
  await panel.getByRole("button", { name: c.apply, exact: true }).click();
  await expect(panel.getByRole("alert")).toBeVisible();
  // A conflict is not a validation reject: the invalid-hostname copy must not be used.
  await expect(panel).not.toContainText(c.invalid);
  await page.reload();
  await expect(panel.getByRole("heading", { name: c.title, exact: true }).last()).toBeVisible();
  await expect(panel.getByRole("heading", { name: c.pending, exact: true })).toHaveCount(0);
  expect(await journal(page)).toBeNull();
});

test("PARITY-DOMAINS definitive 403 on suspend clears journal and fence", async ({ page }) => {
  test.setTimeout(90000);
  const c = copy.en, state = await open(page, "en", false);
  const panel = page.getByTestId("design-domains");
  await panel.getByTestId("domain-custom-row").filter({ hasText: first }).getByRole("button", { name: c.settings, exact: true }).click();
  await panel.getByRole("button", { name: c.suspend, exact: true }).click();
  state.failStatus = 403;
  await panel.getByRole("button", { name: c.confirmSuspend, exact: true }).click();
  await expect(panel.getByRole("alert")).toBeVisible();
  await page.reload();
  await expect(panel.getByRole("heading", { name: c.title, exact: true }).last()).toBeVisible();
  await expect(panel.getByRole("heading", { name: c.pending, exact: true })).toHaveCount(0);
  expect(await journal(page)).toBeNull();
});

for (const [name, addresses, a, aaaa] of [
  ["v6 first", ["2001:db8::20", "192.0.2.20"], "192.0.2.20", "2001:db8::20"],
  ["v4 only", ["192.0.2.20"], "192.0.2.20", null],
] as const) test(`PARITY-DOMAINS apex DNS rows by IP family: ${name}`, async ({ page }) => {
  test.setTimeout(90000);
  const c = copy.en, state = await open(page, "en", false);
  const panel = page.getByTestId("design-domains");
  state.apex = true; state.addresses = [...addresses];
  await panel.getByRole("button", { name: c.add, exact: true }).click();
  await page.getByLabel(c.hostname, { exact: true }).fill("apex.example.test");
  await panel.getByRole("button", { name: c.apply, exact: true }).click();
  await panel.getByRole("button", { name: c.records, exact: true }).click();
  await expect(panel.getByTestId("domain-dns-A")).toHaveCount(1);
  await expect(panel.getByTestId("domain-dns-A")).toContainText(a);
  if (aaaa) {
    await expect(panel.getByTestId("domain-dns-AAAA")).toHaveCount(1);
    await expect(panel.getByTestId("domain-dns-AAAA")).toContainText(aaaa);
    await expect(panel).not.toContainText(c.missingIP);
  } else {
    await expect(panel.getByTestId("domain-dns-AAAA")).toHaveCount(0);
    await expect(panel).toContainText(c.missingIP);
  }
});
