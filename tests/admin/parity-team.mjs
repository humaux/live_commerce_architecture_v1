// Purpose: Assert all 164 signed staff frames on real routes and capture honest parity/click receipts.
// Depends on: node:test, Playwright, frozen FRAME-COPY oracle and admin-parity's existing MOCK session/server.
// Used by: parity-shell-browser.mjs / --browser-admin-parity; independent reviewers consume parity images.
// Invariants: fixture responses are MOCK, not backend acceptance; no gallery, DOM writes, forced clicks or live mail.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { expect as baseExpect } from "@playwright/test";
import oracle from "./parity-team-frames.json" with { type: "json" };

const expect = baseExpect.configure({ timeout: 1500 });
const viewports = [{ name: "desktop", width: 1440, height: 900 }, { name: "mobile", width: 390, height: 844 }];
const roles = { owner: "店主", admin: "管理員", live: "直播人員", fulfilment: "出貨人員", viewer: "僅供檢視" };
const apiRoles = { owner: "owner", admin: "admin", live: "live_operator", fulfilment: "fulfilment", viewer: "viewer" };
const roleLines = ["店主 · 管理全部功能、訂閱帳單與員工", "管理員 · 管理業務，不含訂閱帳單與員工", "直播人員 · 管理直播，查看商品、訂單與庫存", "出貨人員 · 處理出貨、匯出訂單及庫存", "僅供檢視 · 查看資料，不能變更"];
const syntheticID = (n) => `87000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
// Synthetic bearer for a MOCK acceptance response only; never log it, persist it, or use a real invitation.
const inviteToken = Buffer.alloc(32, 7).toString("base64url");
const authStates = new Set(["link", "signin", "signup", "verify-signin", "verify-signup", "accept", "accept-checking", "joined", "expired", "revoked", "used", "wrong-account", "invitee-member", "invitee-off"]);
const inviteErrors = { "invalid-email": [422, "invalid_email"], "already-member": [409, "already_member"], "invite-limit": [429, "too_many_invitations"], "owner-disabled": [403, "owner_role_disabled"], reauth: [403, "reauth_required"] };
const unknownStates = new Set(["invite-checking", "role-checking", "remove-checking", "revoke-checking", "checking-unresolved", "role-unresolved", "remove-unresolved", "revoke-unresolved"]);
const translate = (text, locale) => {
  assert.ok(oracle.locales[text], `signed translation missing for ${text}`);
  return oracle.locales[text][locale];
};
const suffixRole = (state, fallback) => Object.keys(roles).find(role => state.endsWith(`-${role}`)) ?? fallback;

function fixtureState(state, store) {
  const member = { principal_id: syntheticID(2), email: "ari@example.test", role: "admin", joined_at: "2026-10-12T06:00:00Z", is_me: false };
  const owner = { principal_id: syntheticID(1), email: state === "list-owner-no-email" ? null : "owner@example.test", role: "owner", joined_at: "2026-10-01T06:00:00Z", is_me: true };
  const invitation = { id: syntheticID(3), email: "mei@example.test", role: apiRoles[suffixRole(state, "admin")], created_at: "2026-10-30T06:00:00Z", expires_at: "2026-11-02T06:00:00Z", expired: false, mail_state: "SENT" };
  const team = { my_role: state === "non-owner" ? "viewer" : "owner", members: state === "non-owner" ? [] : state === "empty" ? [owner] : [owner, member], invitations: state !== "pending-empty" && (state.startsWith("pending") || state.startsWith("revoke")) ? [invitation] : [] };
  const writes = [];
  let refreshed = false;
  return {
    team, writes,
    async install(page) {
      const fixtureCookies = await page.context().cookies();
      // FIXTURE SETUP: responses follow the existing strict BFF DTOs. All writes are triggered by real UI controls.
      // These transport fixtures are UI-only; the untouched staff-team Go/PG/SMTP chain owns authorization acceptance.
      await page.route("**/api/team/*", async route => {
        const action = new URL(route.request().url()).pathname.split("/").pop();
        const body = route.request().postDataJSON();
        const json = (status, value) => route.fulfill({ status, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify(value) });
        if (action === "list") {
          assert.deepEqual(body, { store_id: store }, "team scope comes from the authorized store fixture");
          if (state === "load-error" && !refreshed) { refreshed = true; return json(503, { code: "unavailable" }); }
          if (state === "password-off") return json(404, { code: "not_found" });
          return json(200, team);
        }
        writes.push({ action, role: body.role }); // Do not persist token, credentials or invitation body.
        if (action === "accept") {
          assert.deepEqual(body, { token: inviteToken });
          if (["expired", "revoked", "used", "wrong-account"].includes(state)) return json(404, { code: "invite_invalid" });
          if (state === "invitee-member") return json(409, { code: "already_member" });
          if (state === "invitee-off") return json(404, { code: "not_found" });
          if (state === "accept-checking") return json(503, { code: "retry_later" });
          return json(200, { store_id: store, role: "admin" });
        }
        assert.equal(body.store_id, store);
        if (unknownStates.has(state)) return json(503, { code: "retry_later" });
        if (state.startsWith("reauth")) return json(403, { code: "reauth_required" });
        if (action === "invite") {
          assert.deepEqual(body, { store_id: store, email: "mei@example.test", role: invitation.role, locale: "zh-TW" });
          if (inviteErrors[state]) return json(inviteErrors[state][0], { code: inviteErrors[state][1] });
          invitation.mail_state = state === "mail-failed" ? "FAILED" : "SENT";
          team.invitations = [invitation];
          return json(201, { id: invitation.id, expires_at: invitation.expires_at, mail_state: invitation.mail_state });
        }
        if (action === "set-role") {
          assert.deepEqual(body, { store_id: store, principal_id: member.principal_id, role: apiRoles[suffixRole(state, "viewer")] });
          member.role = body.role;
        } else if (action === "remove") {
          assert.deepEqual(body, { store_id: store, principal_id: member.principal_id });
          if (state === "last-owner") return json(409, { code: "last_owner" });
          team.members = [owner];
        } else if (action === "revoke-invite") {
          assert.deepEqual(body, { store_id: store, invite_id: invitation.id });
          team.invitations = [];
        } else assert.fail(`unsupported existing team action: ${action}`);
        return route.fulfill({ status: 204 });
      });
      await page.route("**/api/auth/password/*", async route => {
        const action = new URL(route.request().url()).pathname.split("/").pop();
        // MOCK auth setup; never send code mail, record passwords, or manufacture a production identity.
        assert.ok(["login", "signup", "verify"].includes(action));
        // FIXTURE only: simulate the MOCK verify response establishing the existing synthetic session.
        // No DOM is changed; the form submission and subsequent document navigation remain real.
        if (action === "verify") await page.context().addCookies(fixtureCookies);
        return route.fulfill({ status: action === "verify" ? 200 : 202, contentType: "application/json", body: JSON.stringify(action === "verify" ? { redirect: `/zh-TW/team?store=${store}` } : { step: "code", expires_at: new Date(Date.now() + 600000).toISOString() }) });
      });
    },
  };
}

export function registerTeamParity({ session, origin, store, output, sha }) {
  const captures = [], clicks = [];
  const button = (page, text) => page.locator("main").getByRole("button", { name: text, exact: true });
  async function click(page, text) {
    const row = { page: new URL(page.url()).pathname.replace(/\/invite\/[^/]+/, "/invite/[token]"), control: text, operation: "click", expected: "signed control is visible, enabled and responds", actual: "NOT_REACHED", result: "fail" };
    clicks.push(row);
    try {
      const target = button(page, text);
      await expect(target, `signed visible control: ${text}`).toBeVisible();
      await expect(target).toBeEnabled();
      await target.click();
      row.actual = "clicked; destination is asserted by the next state check"; row.result = "clicked";
    } catch (error) { row.actual = String(error.message).slice(0, 350); throw error; }
  }
  async function selectRole(page, role) {
    const text = roleLines[Object.keys(roles).indexOf(role)];
    const target = page.getByRole("radio", { name: text, exact: true });
    await expect(target, "signed five-role radio control").toBeVisible();
    await target.check();
    await expect(target).toBeChecked();
    clicks.push({ page: "/zh-TW/team", control: text, operation: "check", expected: "selected role is checked", actual: "checked", result: "pass" });
  }
  async function pending(page) { await click(page, "待接受邀請"); }
  async function invite(page, state) {
    await click(page, "邀請員工");
    await page.getByRole("textbox", { name: "電子郵件", exact: true }).fill("mei@example.test");
    await selectRole(page, suffixRole(state, "admin"));
  }
  async function edit(page, state) {
    await click(page, "變更角色");
    await selectRole(page, suffixRole(state, "viewer"));
  }
  async function authForm(page, email = "mei@example.test") {
    await page.getByRole("textbox", { name: "電子郵件", exact: true }).fill(email);
    await page.getByLabel("密碼", { exact: true }).fill("Synthetic-team-parity-password!");
    await click(page, "傳送驗證碼");
    await expect(page.getByRole("heading", { name: "查看信箱驗證碼", exact: true })).toBeVisible();
  }
  async function enter(page, context, frame, f, locale) {
    const state = frame.state;
    if (authStates.has(state)) {
      if (["link", "signin", "signup", "verify-signin", "verify-signup"].includes(state)) await context.clearCookies(); // SETUP: signed-out fixture only.
      await page.goto(`${origin()}/${locale}/invite/${inviteToken}`);
      await expect(page.getByRole("heading", { name: translate("加入團隊", locale), exact: true }), "signed invitation heading").toBeVisible();
      if (state === "link") return;
      if (["signin", "signup", "verify-signin", "verify-signup"].includes(state)) {
        const signup = state.endsWith("signup");
        const link = page.getByRole("link", { name: translate(signup ? "建立帳號" : "登入", locale), exact: true });
        await expect(link).toBeVisible();
        await link.click();
        // Observe only the pathname: an invitation's return fragment must never enter an assertion log.
        await expect.poll(() => new URL(page.url()).pathname.replace(/\/$/, "")).toBe(`/${locale}${signup ? "/signup" : ""}`);
        await page.getByRole("textbox", { name: "電子郵件", exact: true }).fill("mei@example.test");
        if (state.startsWith("verify")) await authForm(page);
        return;
      }
      if (state !== "accept") {
        await click(page, "接受邀請");
        await expect.poll(() => f.writes.length, { message: "accept must issue exactly one write" }).toBe(1);
      }
      return;
    }
    await page.goto(`${origin()}/${locale}/team?store=${store}`);
    if (state === "nav") {
      // The shared shell's owner-approved mobile drawer supersedes module-specific shell variations.
      if (page.viewportSize().width < 1024) await page.getByRole("button", { name: "開啟導覽", exact: true }).click();
      const group = page.locator("[data-shell-rail]").getByRole("button", { name: translate("設定", locale), exact: true });
      if (await group.getAttribute("aria-expanded") === "false") await group.click();
      return;
    }
    if (["non-owner", "load-error", "password-off"].includes(state)) return;
    await expect(page.locator("main").getByRole("heading", { name: translate("員工", locale), exact: true }), "signed staff heading replaces legacy Team").toBeVisible();
    if (state === "empty") return;
    await expect(button(page, translate("員工名單", locale)), "signed members tab replaces legacy always-open sections").toBeVisible();
    if (["list", "list-owner-no-email"].includes(state)) return;
    if (state.startsWith("pending")) { await pending(page); return; }
    if (state.startsWith("revoke")) {
      await pending(page); await click(page, "撤回");
      if (!state.startsWith("revoke-confirm")) await click(page, "確認撤回");
    } else if (state.startsWith("remove") || state === "last-owner") {
      await click(page, "移除");
      if (state !== "remove-confirm") await click(page, "確認移除");
    } else if (state.startsWith("role-") || state.startsWith("list-updated")) {
      const noChange = state.endsWith("-admin");
      await edit(page, state);
      if (noChange) {
        await expect(button(page, "繼續"), "signed unchanged-role control is disabled").toBeDisabled();
        assert.equal(f.writes.length, 0, "unchanged role never sends set-role");
        // No forced click: role-confirm-admin/role-done-admin have no approved reachable path.
        // Their signed content assertions remain a visible blocker for the integrator guard, not fake coverage.
        if (state === "list-updated-admin") { await click(page, "取消"); await page.reload(); }
      } else {
        if (!state.startsWith("role-edit")) await click(page, "繼續");
        if (!state.startsWith("role-edit") && !state.startsWith("role-confirm")) await click(page, "確認變更");
        if (state.startsWith("list-updated")) { await click(page, "回到員工"); await page.reload(); }
      }
    } else {
      await invite(page, state);
      if (!/^invite-(owner|admin|live|fulfilment|viewer)$/.test(state)) await click(page, "送出邀請");
      if (state.startsWith("reauth-")) {
        await click(page, "重新登入");
        await page.getByRole("textbox", { name: "電子郵件", exact: true }).fill("owner@example.test");
        if (state === "reauth-verify") await authForm(page, "owner@example.test");
      }
    }
    if (state.endsWith("unresolved")) await click(page, "查看最新名單");
  }

  async function content(page, frame, locale) {
    const scope = frame.state === "nav" ? page.locator("[data-ui-shell]") : page.locator("main");
    for (const text of frame.text) {
      const value = translate(text, locale);
      if (["mei@example.test", "owner@example.test"].includes(text) && ["signin", "signup", "verify-signin", "verify-signup", "reauth-signin", "reauth-verify"].includes(frame.state) || text === "mei@example.test" && /^invite-(owner|admin|live|fulfilment|viewer)$/.test(frame.state)) {
        if (!frame.state.includes("verify")) { await expect(page.getByRole("textbox", { name: translate("電子郵件", locale), exact: true })).toHaveValue(value); continue; }
      }
      if (["輸入密碼", "輸入 6 位數字"].includes(text)) { await expect(page.getByPlaceholder(value, { exact: true })).toBeVisible(); continue; }
      await expect(scope.getByText(value, { exact: true }).first(), `signed ${frame.state}: ${value}`).toBeVisible();
    }
    // Exact names and roles are part of the oracle. Text alone cannot pass when controls are inert divs.
    const names = new Set(frame.text);
    for (const text of ["員工名單", "待接受邀請", "邀請員工", "變更角色", "移除", "撤回", "取消", "送出邀請", "繼續", "確認變更", "確認移除", "確認撤回", "查看最新名單", "接受邀請"]) {
      if (!names.has(text)) continue;
      const target = scope.getByRole("button", { name: translate(text, locale), exact: true });
      await expect(target).toBeVisible();
      if (["password-off", "mail-failed"].includes(frame.state) && text === "邀請員工" || frame.state === "invitee-off" && text === "接受邀請") await expect(target).toBeDisabled();
      else if (frame.state === "role-edit-admin" && text === "繼續") await expect(target).toBeDisabled();
      else await expect(target).toBeEnabled();
    }
    if (frame.state.startsWith("invite-") && Object.keys(roles).some(role => frame.state === `invite-${role}`) || frame.state.startsWith("role-edit")) {
      await expect(page.getByRole("radio")).toHaveCount(5);
      for (const text of roleLines) await expect(page.getByRole("radio", { name: translate(text, locale), exact: true })).toBeVisible();
    }
    // Legacy controls require an explicit design replacement; no test exemption or fixture is weakened.
    await expect(scope.getByRole("combobox", { name: /^(Role|角色)$/ })).toHaveCount(0);
    await expect(scope.getByRole("button", { name: /^(Send again|再次寄送|重新寄送)$/ })).toHaveCount(0);
    await expect(scope.getByText(/原型示範|預填示範|示範回應|Prototype switch/)).toHaveCount(0);
    await expect(scope.getByText(/未納入產品|未纳入产品|Not part of product/i)).toHaveCount(0);
    await expect(scope).not.toContainText(inviteToken);
    assert.ok(!new URL(page.url()).pathname.includes("ui-gallery"), "parity must use the real route");
    // READ/MEASURE only: no CSS injection or DOM mutation to conceal overflow.
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "signed viewport has no horizontal overflow");
  }

  async function recovery(page, frame, f) {
    const state = frame.state;
    if (unknownStates.has(state) || state === "accept-checking") {
      assert.equal(f.writes.length, 1, "UNKNOWN never repeats the mutation");
      for (const text of ["送出邀請", "確認變更", "確認移除", "確認撤回", "接受邀請"]) await expect(button(page, text)).toHaveCount(0);
      if (!state.endsWith("unresolved") && state !== "accept-checking") {
        await click(page, "查看最新名單");
        const target = oracle.frames.find(frame => frame.state === (state === "invite-checking" ? "checking-unresolved" : state.replace("-checking", "-unresolved")));
        await content(page, target, "zh-TW");
        assert.equal(f.writes.length, 1, "reconciliation is a list read only");
      }
      await page.reload();
      assert.equal(f.writes.length, 1, "refresh never retries an uncertain write");
    } else if (["list", "list-owner-no-email", "empty", "non-owner"].includes(state) || state.startsWith("list-updated")) {
      await page.reload(); await content(page, frame, "zh-TW");
      if (state !== "non-owner") {
        await click(page, "邀請員工"); await expect(page.getByRole("radio")).toHaveCount(5);
        await click(page, "取消"); await expect(page.locator("main").getByRole("heading", { name: "員工", exact: true })).toBeVisible();
      }
    } else if (state === "load-error") {
      await click(page, "查看最新名單");
      await expect(button(page, "員工名單")).toBeVisible();
      assert.equal(f.writes.length, 0);
    } else if (state === "mail-failed") {
      await click(page, "回到員工"); await page.reload(); await pending(page);
      await expect(page.locator("main").getByText("mei@example.test", { exact: true })).toBeVisible();
      assert.equal(f.writes.length, 1, "mail failure retains invitation history without sending again");
    } else if (state.startsWith("invite-sent") || ["role-done", "role-done-owner", "role-done-live", "role-done-fulfilment", "remove-done", "revoke-done"].includes(state)) {
      await click(page, "回到員工");
      const next = state.startsWith("invite-sent") ? state.replace("invite-sent", "pending") : state === "revoke-done" ? "pending-empty" : state === "remove-done" ? "empty" : state.replace("role-done", "list-updated");
      await content(page, oracle.frames.find(frame => frame.state === next), "zh-TW");
      await page.reload();
      if (state.startsWith("invite-sent") || state === "revoke-done") await pending(page);
      if (state.startsWith("invite-sent")) await expect(page.locator("main").getByText("mei@example.test", { exact: true })).toBeVisible();
      if (state === "remove-done") await expect(page.locator("main").getByText("Ari · ari@example.test", { exact: true })).toHaveCount(0);
      if (state === "revoke-done") await expect(page.locator("main").getByText("mei@example.test", { exact: true })).toHaveCount(0);
      if (state.startsWith("role-done")) await expect(page.locator("main").getByText(`${roles[suffixRole(state, "viewer")]} · 加入 10/12`, { exact: true })).toBeVisible();
      assert.equal(f.writes.length, 1, "saved result remains after refresh without a repeated write");
    } else if (frame.text.includes("取消")) {
      await click(page, "取消");
      if (state.startsWith("role-confirm")) await expect(page.getByRole("radio")).toHaveCount(5);
      else if (state.startsWith("revoke-confirm")) await expect(button(page, "撤回")).toBeVisible();
      else await expect(page.locator("main").getByRole("heading", { name: "員工", exact: true })).toBeVisible();
      assert.equal(f.writes.length, 0, "cancel is read-only");
    } else if (Object.hasOwn(inviteErrors, state) && state !== "reauth") {
      await click(page, "返回"); await expect(page.getByRole("radio")).toHaveCount(5);
      assert.equal(f.writes.length, 1, "back does not retry the refused invitation");
    } else if (["joined", "invitee-member"].includes(state)) {
      const link = page.getByRole("link", { name: "進入店舖", exact: true });
      await expect(link).toBeVisible(); await link.click();
      await expect(page.locator("[data-shell-route='/']")).toBeAttached();
      await page.reload(); await expect(page.locator("[data-shell-route='/']")).toBeAttached();
      assert.equal(f.writes.length, 1);
    } else if (["accept", "expired", "revoked", "used", "wrong-account"].includes(state)) {
      const link = page.getByRole("link", { name: "重新登入", exact: true });
      await expect(link).toBeVisible(); await link.click();
      await expect(page.getByRole("textbox", { name: "電子郵件", exact: true })).toBeVisible();
    } else if (state.startsWith("pending")) {
      if (state === "pending-empty") await click(page, "回到員工");
      else await click(page, "員工名單");
      await expect(button(page, "員工名單")).toBeVisible();
      assert.equal(f.writes.length, 0);
    } else if (state === "reauth-verify") {
      await page.getByRole("textbox", { name: "驗證碼", exact: true }).fill("123456");
      await click(page, "驗證並繼續");
      await expect(button(page, "員工名單")).toBeVisible();
      assert.equal(f.writes.length, 1, "fresh auth returns to a list read without automatically repeating the write");
    } else if (["verify-signin", "verify-signup"].includes(state)) {
      await page.getByRole("textbox", { name: "驗證碼", exact: true }).fill("123456");
      await click(page, "驗證並繼續");
      await expect(button(page, "接受邀請")).toBeVisible();
      await page.reload(); await expect(button(page, "接受邀請")).toBeVisible();
      assert.equal(f.writes.length, 0, "verification never accepts an invitation automatically");
    } else if (state === "last-owner") {
      await click(page, "回到員工"); await expect(button(page, "員工名單")).toBeVisible();
      assert.equal(f.team.members.filter(member => member.role === "owner").length, 1);
      assert.equal(f.writes.length, 1, "owner-floor refusal is not retried");
    } else if (state === "reauth") {
      await click(page, "重新登入");
      await expect(page.getByRole("textbox", { name: "電子郵件", exact: true })).toBeVisible();
      assert.equal(f.writes.length, 1);
    }
  }

  // Register in the existing node:test browser mode; its matrix-receipts browser-spec mark is retained verbatim.
  for (const viewport of viewports) for (const frame of oracle.frames) test(`PARITY-TEAM signed ${frame.ids[viewport.name]} ${frame.state}`, { timeout: 20000 }, async () => {
    const { context, page } = await session({ width: viewport.width, height: viewport.height });
    const f = fixtureState(frame.state, store);
    const id = frame.ids[viewport.name];
    const firstClick = clicks.length;
    let journeyCompleted = false, matched = false, failure = "", density = null;
    page.setDefaultTimeout(2000);
    try {
      await f.install(page);
      await enter(page, context, frame, f, "zh-TW"); journeyCompleted = true;
      await content(page, frame, "zh-TW"); matched = true;
      // READ/MEASURE only, mirroring density-probe's visible text counts; no layout or state changes.
      density = await page.locator("main").evaluate(main => {
        const blocks = [...main.querySelectorAll("h1,h2,p,label,button,a,input")].filter(node => {
          const box = node.getBoundingClientRect();
          return box.width > 0 && box.height > 0 && box.bottom > 0 && box.top < innerHeight && getComputedStyle(node).visibility !== "hidden";
        }).map(node => node instanceof HTMLInputElement ? node.value || node.placeholder : node.textContent?.trim() ?? "").filter(Boolean);
        return { blocks: blocks.length, characters: blocks.join("").length };
      });
      // Capture the requested state BEFORE recovery actions move to a different frame.
      await mkdir(`${output}/parity`, { recursive: true });
      await page.screenshot({ path: `${output}/parity/${id.replace(":", "-")}.png`, animations: "disabled" });
      await recovery(page, frame, f);
    } catch (error) { failure = String(error.message).slice(0, 500); throw error; }
    finally {
      await mkdir(`${output}/parity`, { recursive: true });
      if (!matched) await page.screenshot({ path: `${output}/parity/${id.replace(":", "-")}.png`, animations: "disabled" }).catch(() => {});
      for (const row of clicks.slice(firstClick)) if (row.result === "clicked") row.result = matched && !failure ? "pass" : "fail";
      captures.push({ frame: id, state: frame.state, viewport, route: authStates.has(frame.state) ? "/zh-TW/invite/[token] (auth links may navigate)" : "/zh-TW/team", screenshot: `parity/${id.replace(":", "-")}.png`, deliveryPNG: `ux-team/${frame.exports[viewport.name]}`, journeyCompleted, matched, failure, density, writes: f.writes });
      await writeFile(`${output}/parity/team-manifest.json`, JSON.stringify({ tier: "MOCK", sha, source: oracle.source, captures }, null, 2) + "\n");
      await writeFile(`${output}/parity/team-click-ledger.json`, JSON.stringify({ tier: "MOCK", sha, clicks }, null, 2) + "\n");
      await context.close();
    }
  });

  for (const locale of ["zh-CN", "en"]) for (const viewport of viewports) test(`PARITY-TEAM main screen ${locale} ${viewport.name}`, { timeout: 15000 }, async () => {
    const { context, page } = await session({ width: viewport.width, height: viewport.height });
    try {
      const frame = oracle.frames.find(frame => frame.state === "list");
      const f = fixtureState("list", store); await f.install(page);
      await enter(page, context, frame, f, locale);
      await content(page, frame, locale);
      await page.reload(); await content(page, frame, locale);
    } finally { await context.close(); }
  });

  test("PARITY-TEAM frozen oracle is the signed export, not application copy", () => {
    const source = readFileSync(new URL("../../output/parity-team/FRAME-COPY.md", import.meta.url));
    assert.equal(createHash("sha256").update(source).digest("hex"), oracle.source_sha256);
    assert.equal(oracle.frames.length, 82);
    assert.equal(new Set(oracle.frames.flatMap(frame => Object.values(frame.ids))).size, 164);
  });
}
