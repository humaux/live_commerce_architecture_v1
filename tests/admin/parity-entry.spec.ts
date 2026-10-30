// Purpose: Assert the signed PARITY-ENTRY auth frames on real localized routes, with click receipts and parity images.
// Depends on: Playwright; the password-auth gate's packaged Next server; MOCK password response fixtures only.
// Used by: playwright.config.ts password-auth suite / TestBrowserPasswordAuth.
// Invariants: no DOM mutation, forced clicks, live mail, real customer data, or implementation-derived expected copy.
import { expect, test, type Page } from "@playwright/test";
import { appendFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN;
const evidence = process.env.LC_BROWSER_EVIDENCE_DIR;
if (!origin || !evidence) throw new Error("password-auth gate origin/evidence required");
test.use({ baseURL: origin, trace: "off" });
mkdirSync(join(evidence, "parity"), { recursive: true });
const email = "merchant@example.invalid";
const password = "Synthetic-entry-parity-482!";
const views = [
  { width: 1440, height: 900, index: 0 },
  { width: 390, height: 844, index: 1 },
] as const;
type Mode = "signin" | "signup" | "reset";
type Frame = { ids: [string, string]; mode: Mode; state: "form" | "sent" | "code"; error?: string; reason?: string; message?: string };
const frames: Frame[] = [
  { ids: ["20:443", "20:545"], mode: "signin", state: "form" },
  { ids: ["20:464", "20:566"], mode: "signup", state: "form" },
  { ids: ["20:485", "20:587"], mode: "signin", state: "code" },
  { ids: ["20:501", "20:603"], mode: "reset", state: "form" },
  { ids: ["20:513", "20:615"], mode: "reset", state: "code" },
  { ids: ["82:1760", "82:1777"], mode: "reset", state: "sent" },
  { ids: ["1198:14260", "1198:14280"], mode: "signin", state: "form", error: "invalid_credentials", message: "電子郵件或密碼不正確。" },
  { ids: ["1198:14300", "1198:14322"], mode: "signin", state: "form", error: "throttled", message: "嘗試次數過多，請 15 分鐘後再試。" },
  { ids: ["1198:14345", "1198:14365"], mode: "signin", state: "form", error: "mail_unavailable", message: "目前無法寄送登入確認碼。請等待一段時間後再試，現在重試也不會成功。" },
  { ids: ["1198:14385", "1198:14403"], mode: "signin", state: "code", error: "invalid_code", message: "確認碼不正確或已過期，請檢查或重新寄送。" },
  { ids: ["1198:14421", "1198:14439"], mode: "signin", state: "code", error: "invalid_request", message: "本次確認已過期，請重新開始以取得新的確認碼。" },
  { ids: ["1198:14458", "1198:14475"], mode: "reset", state: "code", error: "invalid_code", message: "確認碼不正確或已過期，請檢查或重新寄送。" },
  { ids: ["1198:14492", "1198:14509"], mode: "reset", state: "code", error: "password_policy", reason: "breached", message: "該密碼出現在已知的資料外洩中，請換一個。" },
  { ids: ["1207:7978", "1207:7997"], mode: "reset", state: "code", error: "password_policy", reason: "too_short", message: "密碼太短，至少需要 12 個字元。" },
  { ids: ["1207:8017", "1207:8036"], mode: "reset", state: "code", error: "password_policy", reason: "too_long", message: "密碼太長，最多 128 個字元。" },
  { ids: ["1253:18438", "1253:18452"], mode: "reset", state: "code", error: "invalid_request", message: "本次確認已過期，請重新開始以取得新的確認碼。" },
  { ids: ["1253:18467", "1253:18481"], mode: "reset", state: "form", error: "throttled", message: "嘗試次數過多，請 15 分鐘後再試。" },
  { ids: ["1253:18496", "1253:18519"], mode: "reset", state: "code", error: "throttled", message: "嘗試次數過多，請 15 分鐘後再試。" },
  { ids: ["1253:18557", "1253:18577"], mode: "signup", state: "code" },
];
const title = { signin: "登入商家工作區", signup: "建立商家帳號", reset: "重設密碼" };
const submit = { signin: "繼續", signup: "建立帳號", reset: "寄送確認碼" };
const verify = { signin: "登入", signup: "建立帳號", reset: "設定密碼並登入" };
const path = (mode: Mode, locale = "zh-TW") => `/${locale}/${mode === "signin" ? "" : mode}`;
const panel = (page: Page) => page.getByRole("region", { name: /登入商家工作區|建立商家帳號|重設密碼|輸入確認碼|設定新密碼|請查看信箱/ });
function ledger(frame: string, control: string, expected: string) {
  appendFileSync(join(evidence!, "parity-clicks.jsonl"), JSON.stringify({ frame, route: "real auth route", control, operation: "click/fill", expected, actual: "assertion passed", environment: "MOCK" }) + "\n");
}
async function credentials(page: Page, mode: Mode) {
  await expect(page.locator('form[data-hydrated="true"]')).toBeVisible();
  await page.getByRole("textbox", { name: "電子郵件", exact: true }).fill(email);
  if (mode !== "reset") await page.getByLabel("密碼", { exact: true }).fill(password);
  if (mode === "signup") await page.getByLabel("確認密碼", { exact: true }).fill(password);
}
async function geometry(page: Page, mobile: boolean) {
  const box = await panel(page).boundingBox();
  expect(box).not.toBeNull();
  expect(box!.width).toBeCloseTo(mobile ? 358 : 448, 0);
  expect(box!.x).toBeCloseTo(mobile ? 16 : 496, 0);
  expect(box!.y).toBeCloseTo(mobile ? 104 : 140, 0);
  // READ/MEASURE only: reads overflow; never writes DOM or submits an action.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0);
}

for (const view of views) for (const frame of frames) {
  const id = frame.ids[view.index];
  test(`PARITY-ENTRY signed auth ${id} ${frame.mode}/${frame.state}/${frame.error ?? "ready"}`, async ({ page }) => {
    await page.setViewportSize(view);
    const calls: { path: string; body: Record<string, unknown> }[] = [];
    // FIXTURE only: model existing BFF answers, including rare failure states. UI actions remain real clicks.
    await page.route("**/api/auth/password/*", async (route) => {
      const url = new URL(route.request().url());
      calls.push({ path: url.pathname, body: route.request().postDataJSON() });
      const isVerify = url.pathname.endsWith("/verify");
      const fail = frame.error && (frame.state === "form" || isVerify);
      const status = fail ? frame.error === "throttled" ? 429 : frame.error === "mail_unavailable" ? 503 : frame.error === "invalid_credentials" || frame.error === "invalid_code" ? 401 : 422 : 202;
      await route.fulfill({ status, contentType: "application/json", headers: { "retry-after": "900" }, body: JSON.stringify(fail ? { code: frame.error, ...(frame.reason ? { details: { reason: frame.reason } } : {}) } : { step: "code", expires_at: new Date(Date.now() + 600_000).toISOString() }) });
    });
    await page.goto(path(frame.mode));
    await expect(page.getByRole("heading", { name: title[frame.mode], exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: submit[frame.mode], exact: true })).toBeVisible();
    if (frame.mode === "signup") await expect(page.getByLabel("確認密碼", { exact: true })).toBeVisible();
    if (frame.state !== "form" || frame.error) {
      await credentials(page, frame.mode);
      await page.getByRole("button", { name: submit[frame.mode], exact: true }).click();
      if (frame.mode === "reset" && frame.state !== "form") {
        await expect(page.getByRole("heading", { name: "請查看信箱", exact: true })).toBeVisible();
        if (frame.state === "code") await page.getByRole("button", { name: "輸入確認碼", exact: true }).click();
      }
      if (frame.state === "code") {
        await expect(page.getByRole("heading", { name: frame.mode === "reset" ? "設定新密碼" : "輸入確認碼", exact: true })).toBeVisible();
        await expect(page.getByLabel("6 位數確認碼", { exact: true })).toBeVisible();
        if (frame.error) {
          await page.getByLabel("6 位數確認碼", { exact: true }).fill("123456");
          if (frame.mode === "reset") await page.getByLabel("新密碼", { exact: true }).fill(frame.reason === "too_short" ? "short" : frame.reason === "too_long" ? "x".repeat(129) : password);
          await page.getByRole("button", { name: verify[frame.mode], exact: true }).click();
          if (frame.error === "invalid_request") {
            await expect(page.getByRole("heading", { name: title[frame.mode], exact: true })).toBeVisible();
            await expect(page.getByLabel("6 位數確認碼", { exact: true })).toHaveCount(0);
          }
        }
      }
      if (frame.message) await expect(page.getByRole("alert").filter({ hasText: frame.message })).toHaveText(frame.message);
      expect(calls.length).toBe(frame.state === "code" && frame.error ? 2 : 1);
      expect(calls[0].body).toEqual(frame.mode === "reset" ? { email, locale: "zh-TW" } : { email, password, locale: "zh-TW" });
      expect(calls[0].body).not.toHaveProperty("confirm_password");
      ledger(id, frame.error ? "submit failure state" : "submit credentials", frame.message ?? frame.state);
    }
    await expect(page.locator("main")).not.toContainText("驗證碼");
    await expect(page.locator("main")).not.toContainText("原型：");
    await geometry(page, view.index === 1);
    await page.screenshot({ path: join(evidence!, "parity", `${id}.png`) });
    if (frame.state === "code" && !frame.error) {
      const start = page.getByRole("button", { name: "重新開始", exact: true });
      await start.click();
      await expect(page.getByRole("heading", { name: title[frame.mode], exact: true })).toBeVisible();
      ledger(id, "重新開始", title[frame.mode]);
    }
    // Passwords/codes are ephemeral by contract, rather than a draft to recover on refresh.
    await page.reload();
    await expect(page.getByRole("heading", { name: title[frame.mode], exact: true })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "電子郵件", exact: true })).toHaveValue("");
  });
}

const copy = {
  "zh-TW": { title: "登入商家工作區", email: "電子郵件", signup: "建立帳號", signupTitle: "建立商家帳號", confirm: "確認密碼", reset: "忘記密碼？", resetTitle: "重設密碼", back: "返回登入" },
  "zh-CN": { title: "登录商家工作区", email: "邮箱", signup: "创建账号", signupTitle: "创建商家账号", confirm: "确认密码", reset: "忘记密码？", resetTitle: "重置密码", back: "返回登录" },
  en: { title: "Sign in to your merchant workspace", email: "Email", signup: "Create an account", signupTitle: "Create your merchant account", confirm: "Confirm password", reset: "Forgot your password?", resetTitle: "Reset your password", back: "Back to sign in" },
} as const;
for (const locale of ["zh-TW", "zh-CN", "en"] as const) test(`PARITY-ENTRY ${locale}: click every auth navigation control and confirm password`, async ({ page }) => {
  const c = copy[locale];
  await page.goto(path("signin", locale));
  await expect(page.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
  await page.getByRole("link", { name: c.signup, exact: true }).click();
  await expect(page.getByRole("heading", { name: c.signupTitle, exact: true })).toBeVisible();
  await expect(page.getByLabel(c.confirm, { exact: true })).toBeVisible();
  await page.getByRole("link", { name: c.back, exact: true }).click();
  await page.getByRole("link", { name: c.reset, exact: true }).click();
  await expect(page.getByRole("heading", { name: c.resetTitle, exact: true })).toBeVisible();
  await expect(page.getByRole("textbox", { name: c.email, exact: true })).toBeVisible();
  await page.getByRole("link", { name: c.back, exact: true }).click();
  await expect(page.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
  ledger(locale, "signup/back/reset/back", "localized real routes");
});

test("PARITY-ENTRY confirmation mismatch sends no signup; resend remains an explicit click", async ({ page }) => {
  let calls = 0;
  // FIXTURE only: count exact existing BFF requests; never communicate with a real mailbox.
  await page.route("**/api/auth/password/signup", async route => {
    calls++;
    expect(route.request().postDataJSON()).toEqual({ email, password, locale: "zh-TW" });
    await route.fulfill({ status: 202, json: { step: "code", expires_at: new Date(Date.now() + 600_000).toISOString() } });
  });
  await page.clock.install();
  await page.goto(path("signup"));
  await credentials(page, "signup");
  await page.getByLabel("確認密碼", { exact: true }).fill("Different-synthetic-password!");
  await page.getByRole("button", { name: "建立帳號", exact: true }).click();
  await expect(page.getByRole("heading", { name: "建立商家帳號", exact: true })).toBeVisible();
  expect(calls).toBe(0);
  await page.getByLabel("確認密碼", { exact: true }).fill(password);
  await page.getByRole("button", { name: "建立帳號", exact: true }).click();
  await expect(page.getByRole("heading", { name: "輸入確認碼", exact: true })).toBeVisible();
  expect(calls).toBe(1);
  await expect(page.getByRole("button", { name: /秒後可重新寄送/ })).toBeDisabled();
  await page.clock.fastForward(61_000);
  await page.getByRole("button", { name: "重新寄送確認碼", exact: true }).click();
  await expect(page.getByRole("button", { name: /秒後可重新寄送/ })).toBeDisabled();
  expect(calls).toBe(2);
  ledger("1253:18557", "confirm mismatch / correct / resend", "zero mismatched writes; one write per explicit action");
});
