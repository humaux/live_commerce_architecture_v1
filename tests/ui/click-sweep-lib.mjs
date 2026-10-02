// G-UI8 click-sweep library (owner rule 2026-10-03, docs/engineering/ui-architecture.md 10.6b): every check here is a REAL Playwright
// interaction (locator.click / selectOption / check, no force, no dispatchEvent). page.evaluate is used ONLY to read/measure: it lists the
// controls, reads their state and counts DOM mutations; it never changes a value, clicks or fires an event.
//   - enumerate(): visible + enabled interactive controls inside a scope (button, a[href], role=button|tab|menuitem|switch|checkbox|radio|link,
//     select, summary, checkbox/radio/submit inputs)
//   - sweepControl(): one real click (selectOption for a <select>) + judgement: a visible effect within 3 s, no new console error / page error / 5xx,
//     hittable (Playwright actionability: no overlap), destructive/irreversible controls only up to their confirmation then cancelled
//   - Ledger: JSON + Markdown click ledger; matchKnown(): the explicit known-defect list (no silent allowlist: unmatched entries fail too)
import { writeFile } from "node:fs/promises";

// ---- classification (pure; unit tested in click-sweep-lib.test.mjs) ----------------------------------------------------------------------------------
// Destructive or irreversible: exercised only up to the confirmation step, then cancelled (owner list: delete, archive, void, disconnect, refund,
// cancel order, publish, sign out; extended with the obvious siblings revoke / remove / unpublish / deactivate / erase). Matching is on the accessible name,
// test id, title and aria-label of the control.
const DESTRUCTIVE_EN = /\b(delete|remove|archive|void|disconnect|refund|cancel (the )?order|revoke|unpublish|publish|deactivate|erase|discard|suspend|detach|unbind|unlink|terminate)\b/i;
const DESTRUCTIVE_ZH = /刪除|删除|封存|作廢|作废|斷開|断开|中斷連接|中断连接|退款|取消訂單|取消订单|撤銷|撤销|下架|上架|發布|发布|發佈|取消發佈|取消发布|停用|移除|清除|抹除|丟棄|丢弃|解除|終止|终止|停止/;
export const isDestructive = (text) => DESTRUCTIVE_EN.test(text) || DESTRUCTIVE_ZH.test(text);
// Sign out ends the session the sweep itself runs in: it is exercised once, last, in a throwaway context (journey J5), never inside the sweep.
const SIGN_OUT = /\b(sign ?out|log ?out)\b|登出|退出登录|退出登錄/i;
export const isSignOut = (text) => SIGN_OUT.test(text);
// Irreversible but not "destructive": placing an order / paying / sending. Clicked with every mutating request blocked at the network edge, so a click on an
// incomplete form proves the validation and a click on a complete one can never create a record.
const IRREVERSIBLE = /place order|submit order|pay now|pay (with|by)|send (message|reply)|confirm (payment|order)|送出訂單|送出订单|提交訂單|提交订单|立即付款|確認付款|确认付款|送出訊息|送出回覆/i;
export const isIrreversible = (text) => IRREVERSIBLE.test(text);
// "Cancel path" buttons of a confirmation layer.
export const CANCEL_RE = /^(cancel|close|no|back|dismiss|not now|keep|取消|關閉|关闭|否|返回|不要|先不要|保留|留下)/i;
// Class key: the control without ids and numbers, so 24 identical order rows are one class (sampled, never silently dropped: the ledger names class size).
export const classKey = (d) => [d.scope, d.tag, d.role, d.type, d.testid.replace(/[0-9a-f]{8}-[0-9a-f-]{27}|\d+/g, "#"), d.name.replace(/[0-9a-f]{8}-[0-9a-f-]{27}|\d+/g, "#"), d.hrefPath.replace(/[0-9a-f]{8}-[0-9a-f-]{27}|\d+/g, "#")].join("|");

export const CONTROL_CSS = [
  "button", "a[href]", '[role="button"]', '[role="tab"]', '[role="menuitem"]', '[role="switch"]', '[role="checkbox"]', '[role="radio"]', '[role="link"]',
  "select", "summary", 'input[type="checkbox"]', 'input[type="radio"]', 'input[type="submit"]', 'input[type="button"]', 'input[type="reset"]',
].join(", ");
export const LAYER_CSS = '[role="dialog"], [role="alertdialog"], dialog[open], [role="menu"], [role="listbox"], [aria-modal="true"]';

// ---- page-side helpers (read/measure only), installed on every document ---------------------------------------------------------------------------------
export const INIT_SCRIPT = `(() => {
  if (window.__sweep) return;
  const s = (window.__sweep = { mutations: 0, kinds: {} });
  const note = (k) => { s.mutations++; s.kinds[k] = (s.kinds[k] || 0) + 1; };
  const filter = ["aria-expanded", "aria-selected", "aria-checked", "aria-pressed", "aria-hidden", "aria-current", "aria-invalid", "open", "hidden", "checked", "disabled", "data-state", "class", "value", "data-open", "data-active"];
  const ignored = (n) => { const e = n.nodeType === 1 ? n : n.parentElement; return !e || !!e.closest("script,style,head,next-route-announcer,[data-sweep-ignore]"); };
  new MutationObserver((records) => {
    for (const r of records) {
      if (ignored(r.target)) continue;
      if (r.type === "childList") { if ([...r.addedNodes, ...r.removedNodes].some((n) => !ignored(n))) note("childList"); }
      else if (r.type === "characterData") note("text");
      else note("attr:" + r.attributeName);
    }
  }).observe(document, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: filter });
  const css = ${JSON.stringify(CONTROL_CSS)};
  const visible = (el) => {
    if (!el.getClientRects().length) return false;
    const cs = getComputedStyle(el);
    if (cs.visibility === "hidden" || cs.display === "none" || parseFloat(cs.opacity) === 0) return false;
    if (el.closest('[hidden],[inert],[aria-hidden="true"]')) return false;
    for (let d = el.closest("details"); d; d = d.parentElement && d.parentElement.closest("details")) if (!d.open && !(el.tagName === "SUMMARY" && el.parentElement === d)) return false; // inside a closed disclosure
    const b = el.getBoundingClientRect();
    if (b.width < 1 || b.height < 1) return false;
    if (b.right <= 0 || b.left >= Math.max(document.documentElement.scrollWidth, innerWidth)) return false; // off-canvas (closed drawer), not scrollable into view
    return true;
  };
  const enabled = (el) => !el.disabled && el.getAttribute("aria-disabled") !== "true" && !el.closest("fieldset[disabled]");
  const name = (el) => {
    const al = el.getAttribute("aria-label"); if (al && al.trim()) return al.trim();
    const lb = el.getAttribute("aria-labelledby");
    if (lb) { const t = lb.split(/\\s+/).map((id) => (document.getElementById(id)?.textContent || "").trim()).join(" ").trim(); if (t) return t; }
    if (el.tagName === "INPUT" || el.tagName === "SELECT") {
      let lab = "";
      if (el.labels && el.labels[0]) { const c = el.labels[0].cloneNode(true); c.querySelectorAll("select,option,input,textarea").forEach((n) => n.remove()); lab = (c.textContent || "").replace(/\\s+/g, " ").trim(); } // the label's own words, not the options inside it
      if (lab) return lab;
      if (el.tagName === "INPUT" && (el.type === "submit" || el.type === "button" || el.type === "reset") && el.value) return el.value.trim();
      return (el.name || el.id || "").trim();
    }
    const t = (el.innerText || el.textContent || "").replace(/\\s+/g, " ").trim(); if (t) return t.slice(0, 80);
    const img = el.querySelector("img[alt]"); if (img && img.alt.trim()) return img.alt.trim();
    return (el.getAttribute("title") || "").trim();
  };
  const hrefPath = (el) => { const h = el.getAttribute("href"); if (!h) return ""; try { const u = new URL(h, location.href); return u.origin === location.origin ? u.pathname : u.host; } catch { return h; } };
  // The item the user is already on (current step / selected tab / checked radio / link to this very URL): clicking it again changes nothing by design.
  const isCurrent = (el) => {
    const cur = el.getAttribute("aria-current");
    if ((cur && cur !== "false") || el.getAttribute("aria-selected") === "true") return true;
    if ((el.matches('input[type="radio"]') && el.checked) || (el.getAttribute("role") === "radio" && el.getAttribute("aria-checked") === "true")) return true;
    if (el.tagName === "A") { try { const u = new URL(el.href, location.href); return u.origin === location.origin && u.pathname.replace(/\\/$/, "") === location.pathname.replace(/\\/$/, "") && u.search === location.search && !u.hash; } catch { return false; } }
    return false;
  };
  const describe = (el, index, scope) => ({
    index, scope, tag: el.tagName.toLowerCase(), role: el.getAttribute("role") || "", type: el.getAttribute("type") || "",
    testid: el.getAttribute("data-testid") || "", name: name(el), title: el.getAttribute("title") || "", ariaLabel: el.getAttribute("aria-label") || "",
    href: el.getAttribute("href") || "", hrefPath: hrefPath(el), target: el.getAttribute("target") || "", download: el.hasAttribute("download"),
    expanded: el.getAttribute("aria-expanded"), selected: el.getAttribute("aria-selected"), checked: el.matches("input") ? String(el.checked) : el.getAttribute("aria-checked"),
    pressed: el.getAttribute("aria-pressed"), value: el.matches("select,input") ? el.value : "", inForm: !!el.closest("form"),
    options: el.tagName === "SELECT" ? [...el.options].filter((o) => !o.disabled).map((o) => o.value) : [],
    controls: el.getAttribute("aria-controls") || "", haspopup: el.getAttribute("aria-haspopup") || "", current: isCurrent(el),
  });
  const cssPath = (el) => { const parts = []; for (let n = el; n && n.nodeType === 1 && n !== document.documentElement; n = n.parentElement) { const p = n.parentElement; parts.unshift(n.tagName.toLowerCase() + ":nth-child(" + (p ? [...p.children].indexOf(n) + 1 : 1) + ")"); } return "html > " + parts.join(" > "); };
  window.__sweepHelpers = {
    // unique CSS path of the disclosure a <summary> belongs to (when it is open) / of the topmost open dialog-like layer
    summaryPath(rootSelector, index) { const root = rootSelector === "body" ? document.body : document.querySelector(rootSelector); const el = root && root.querySelectorAll(css)[index]; return el && el.tagName === "SUMMARY" && el.parentElement && el.parentElement.open ? cssPath(el.parentElement) : ""; },
    layerPath() { const layers = [...document.querySelectorAll(${JSON.stringify(LAYER_CSS)})].filter(visible); return layers.length ? cssPath(layers[layers.length - 1]) : ""; },
    enumerate(rootSelector, scope, exclude) {
      const root = rootSelector === "body" ? document.body : document.querySelector(rootSelector);
      if (!root) return [];
      const all = [...root.querySelectorAll(css)];
      const out = [];
      all.forEach((el, index) => {
        if (exclude && el.closest(exclude)) return;
        if (el.parentElement && el.parentElement.closest(css) && !el.matches("input,select")) return; // a control nested in a control: the outer one is the click target
        if (!visible(el) || !enabled(el)) return;
        out.push(describe(el, index, scope));
      });
      return out;
    },
    state() {
      const layers = [...document.querySelectorAll(${JSON.stringify(LAYER_CSS)})].filter(visible);
      const a = document.activeElement;
      return {
        url: location.href, mut: window.__sweep ? window.__sweep.mutations : -1, layers: layers.length, layerNames: layers.map((l) => (l.getAttribute("aria-label") || l.getAttribute("role") || l.tagName).slice(0, 40)),
        active: a && a !== document.body ? (a.getAttribute("data-testid") || a.id || a.tagName) + ":" + (a.getAttribute("name") || "") : "", scrollY: Math.round(scrollY),
        activeInvalid: !!(a && a.matches && a.matches(":invalid:not(form):not(fieldset)")), invalid: document.querySelectorAll(":invalid:not(form):not(fieldset)").length, ariaInvalid: document.querySelectorAll('[aria-invalid="true"]').length,
        alerts: [...document.querySelectorAll('[role="alert"],[role="status"]')].filter(visible).map((e) => (e.textContent || "").trim().slice(0, 60)).join("|"),
      };
    },
    // Messages that say the page itself is degraded on a plain load (nobody has clicked anything): visible alerts, and status lines that name an
    // unavailable / failed / not-enabled state.
    degraded() {
      const bad = /unavailable|not (enabled|configured|available|turned on)|failed|could not|couldn't|error|無法|不可用|尚未開啟|尚未設定|失敗|錯誤|暫時|无法|尚未开启|尚未设置|失败/i;
      return [...document.querySelectorAll('[role="alert"],[role="status"]')]
        .filter((e) => !e.closest("next-route-announcer") && visible(e))
        .map((e) => ({ role: e.getAttribute("role"), text: (e.textContent || "").replace(/\\s+/g, " ").trim().slice(0, 140), testid: e.getAttribute("data-testid") || "" }))
        .filter((m) => m.text && (m.role === "alert" || bad.test(m.text)));
    },
    own(rootSelector, index) {
      const root = rootSelector === "body" ? document.body : document.querySelector(rootSelector);
      const el = root && root.querySelectorAll(css)[index];
      if (!el) return "gone";
      return [el.matches("input,select") ? (el.type === "checkbox" || el.type === "radio" ? String(el.checked) : el.value) : "", el.getAttribute("aria-expanded"), el.getAttribute("aria-selected"), el.getAttribute("aria-checked"), el.getAttribute("aria-pressed"), el.open === undefined ? "" : String(el.open), el.tagName === "SUMMARY" && el.parentElement ? String(el.parentElement.open) : ""].join("|");
    },
    one(rootSelector, index) {
      const root = rootSelector === "body" ? document.body : document.querySelector(rootSelector);
      const el = root && root.querySelectorAll(css)[index];
      return el ? describe(el, index, "") : null;
    },
  };
})();`;

// ---- monitor: console / page errors / 5xx / dialogs / popups / downloads / external navigation, per context ----------------------------------------------------
export function monitor(context, { allowedHosts, onExternal }) {
  const m = { events: [], inflight: new Map(), seen: new Set(), seenAt: new Map() };
  const push = (e) => m.events.push({ ...e, at: Date.now() });
  const attach = (page) => {
    if (m.seen.has(page)) return;
    m.seen.add(page);
    page.on("console", (msg) => {
      if (msg.type() !== "error") return;
      const text = msg.text();
      // The browser's own "Failed to load resource: ... status of 4xx" lines describe a network status, not a script error: statuses are judged by the
      // response listener below (5xx and own-origin asset 404 fail; a 4xx answer to a form POST is the validation working).
      if (/^Failed to load resource/.test(text) && !/status of 5\d\d/.test(text)) return;
      push({ type: "console-error", text: text.slice(0, 300), url: page.url() });
    });
    page.on("pageerror", (e) => push({ type: "pageerror", text: String(e.message || e).slice(0, 300), url: page.url() }));
    page.on("response", (r) => {
      const status = r.status(), req = r.request(), url = r.url();
      if (status >= 500) push({ type: "http5xx", text: `${status} ${req.method()} ${new URL(url).pathname}` });
      else if (status === 404 && req.method() === "GET" && ["script", "stylesheet", "image", "font", "media"].includes(req.resourceType())) push({ type: "http404-asset", text: `404 ${req.resourceType()} ${new URL(url).pathname}` });
    });
    page.on("dialog", async (d) => { push({ type: "dialog", text: `${d.type()}: ${d.message()}`.slice(0, 200) }); await d.dismiss().catch(() => {}); });
    page.on("download", (d) => push({ type: "download", text: d.suggestedFilename() }));
    page.on("request", (r) => {
      m.inflight.set(r, Date.now());
      if (/^(fetch|xhr)$/.test(r.resourceType()) && !r.url().includes("_rsc=")) { const k = r.method() + " " + new URL(r.url()).pathname; (m.seenAt.get(k) ?? m.seenAt.set(k, []).get(k)).push(Date.now()); }
    });
    for (const ev of ["requestfinished", "requestfailed"]) page.on(ev, (r) => m.inflight.delete(r));
  };
  context.on("page", (p) => { attach(p); if (p.opener()) { push({ type: "popup", text: p.url() }); } });
  for (const p of context.pages()) attach(p);
  // Network edge: nothing outside the stack is ever contacted. A navigation to another host is answered with a stub page and recorded as the effect.
  context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (["data:", "blob:", "about:"].includes(url.protocol) || allowedHosts.has(url.host) || /^(127\.0\.0\.1|localhost)(:|$)/.test(url.host)) return route.fallback();
    push({ type: "external", text: `${route.request().method()} ${url.origin}${url.pathname}`.slice(0, 160) });
    onExternal?.(url);
    return route.fulfill({ status: 200, contentType: "text/html", body: "<!doctype html><title>blocked external</title><p>external request blocked by the click sweep</p>" });
  });
  // A request storm: one data request repeated >= limit times inside windowMs (an effect that re-triggers itself). Returns { path, count } or null.
  m.storm = (windowMs = 5000, limit = 40) => { for (const [k, times] of m.seenAt) for (let i = 0; i + limit - 1 < times.length; i++) if (times[i + limit - 1] - times[i] <= windowMs) return { path: k, count: times.length }; return null; };
  m.mark = () => m.events.length;
  m.since = (n) => m.events.slice(n);
  // In flight = a data request (fetch/xhr/document) that started under 8 s ago. Next's <Link> prefetches (?_rsc=) and requests cancelled by a navigation
  // can stay "pending" forever in the event stream; they are not the page still working.
  m.idle = () => { const now = Date.now(); for (const [r, at] of m.inflight) if (now - at < 8000 && ["fetch", "xhr", "document"].includes(r.resourceType()) && !r.url().includes("_rsc=")) return false; return true; };
  return m;
}

// ---- reading helpers -----------------------------------------------------------------------------------------------------------------------------------------
const safe = async (fn, fallback = null) => { try { return await fn(); } catch { return fallback; } };
export const degradedMessages = (page) => safe(() => page.evaluate(() => window.__sweepHelpers.degraded()), []);
export const pageState = (page) => safe(() => page.evaluate(() => window.__sweepHelpers.state()), { url: page.url(), mut: -2, layers: 0, layerNames: [], active: "", scrollY: 0, invalid: 0, ariaInvalid: 0, alerts: "", navigated: true });
export const listControls = (page, rootSelector, scope, exclude = "") => safe(() => page.evaluate(([r, s, x]) => window.__sweepHelpers.enumerate(r, s, x), [rootSelector, scope, exclude]), []);

// Wait until the page is quiet: no request in flight and no DOM mutation for `quiet` ms (bounded by `max`: a page that never settles is "noisy").
export async function settle(page, mon, { quiet = 350, max = 6000 } = {}) {
  const deadline = Date.now() + max;
  let last = -9, since = Date.now();
  while (Date.now() < deadline) {
    const st = await pageState(page);
    if (st.mut !== last || !mon.idle()) { last = st.mut; since = Date.now(); }
    else if (Date.now() - since >= quiet) return true;
    await page.waitForTimeout(80);
  }
  if (process.env.LC_SWEEP_DEBUG) console.log(`  dbg settle timeout url=${page.url().slice(-50)} mut=${last} inflight=${mon.idle() ? "idle" : "busy"}`);
  return false;
}

export function controlLocator(page, rootSelector, desc) {
  return page.locator(rootSelector).first().locator(CONTROL_CSS).nth(desc.index);
}

const label = (d) => d.name || d.ariaLabel || d.title || d.testid || `<${d.tag}${d.role ? ` role=${d.role}` : ""}>`;
export const controlLabel = label;
const hay = (d) => [d.name, d.testid, d.title, d.ariaLabel].join(" ");

// ---- one control ------------------------------------------------------------------------------------------------------------------------------------------------
// ctx: { page, mon, unit: {route, viewport, locale}, rootSelector, shots: dir, fingerprint(): Promise<string|null>, window: ms }
// Returns a ledger row. The caller restores the page afterwards (restore()).
export async function sweepControl(ctx, desc, classSize, opts = {}) {
  const { page, mon, unit } = ctx;
  const t0 = Date.now();
  const dbg = (what) => { if (process.env.LC_SWEEP_DEBUG) console.log(`  dbg +${Date.now() - t0}ms ${what}`); };
  const row = {
    app: unit.app ?? "", page: unit.route, viewport: unit.viewport, locale: unit.locale, scope: desc.scope, control: label(desc), testid: desc.testid, kind: `${desc.tag}${desc.role ? `[${desc.role}]` : ""}`,
    action: desc.tag === "select" ? "selectOption" : (desc.type === "checkbox" || desc.role === "switch" || desc.role === "checkbox") ? "click (toggle)" : "click",
    expected: "", actual: "", result: "pass", failure: "", note: "", classSize,
  };
  // Safety net for every caller (top level, layers, disclosures): sign out revokes the session the whole sweep runs in. It is exercised once, last (J5).
  if (isSignOut(hay(desc))) { row.result = "skip"; row.expected = "sign out is exercised once, last, in a throwaway context (journey J5)"; row.actual = "not clicked here: it would revoke the sweep's own session"; return row; }
  const destructive = isDestructive(hay(desc)), commit = !!opts.commit && !destructive, irreversible = !destructive && (commit || isIrreversible(hay(desc)));
  const guard = destructive || irreversible;
  row.expected = destructive ? "confirmation layer opens; cancel; nothing changed" : commit ? "commit button of an open layer: its request fires (blocked by the sweep) or validation shows" : irreversible ? "validation / confirmation only (mutating requests blocked)" : desc.tag === "a" && !desc.target ? "navigation or in-page change" : "visible change";
  // The element must still be the control we enumerated (state changed under us: skip, never guess). The page is quiet first: lists re-render while they load.
  await settle(page, mon);
  const now = await safe(() => page.evaluate(([r, i]) => window.__sweepHelpers.one(r, i), [ctx.rootSelector, desc.index]));
  const sameAs = (n) => n && n.tag === desc.tag && n.testid === desc.testid && n.hrefPath === desc.hrefPath && ((desc.testid || desc.hrefPath) ? true : label(n) === label(desc)); // names of cards change while images load: identity = tag + test id + link target
  let same = sameAs(now);
  if (!same) { // the list re-rendered (skeleton -> cards): find the same control at its new position before giving up
    const moved = (await listControls(page, ctx.rootSelector, desc.scope)).find((n) => sameAs(n) && label(n) === label(desc)) ?? (await listControls(page, ctx.rootSelector, desc.scope)).find(sameAs);
    if (moved) { desc.index = moved.index; same = true; }
  }
  if (!same) { row.result = "skip"; row.actual = "control no longer present at this position (page state changed by an earlier click)"; return row; }
  dbg("located");
  const loc = controlLocator(page, ctx.rootSelector, desc);
  const before = await pageState(page);
  const fpBefore = guard ? await ctx.fingerprint?.() : null;
  let blocked = [];
  const guardRoute = async (route) => {
    const method = route.request().method();
    if (!["GET", "HEAD", "OPTIONS"].includes(method) && /\/api\//.test(route.request().url())) { blocked.push(`${method} ${new URL(route.request().url()).pathname}`); return route.abort("blockedbyclient"); }
    return route.fallback();
  };
  if (guard) await page.route("**/*", guardRoute);
  const mark = mon.mark();
  const ownBefore = await safe(() => page.evaluate(([r, i]) => window.__sweepHelpers.own(r, i), [ctx.rootSelector, desc.index]));
  dbg("before click");
  let clickError = null;
  try {
    if (desc.tag === "select") {
      await loc.click({ trial: true, timeout: 4000 }); // hittability: Playwright actionability (visible, stable, receives events)
      const other = desc.options.find((o) => o !== desc.value);
      if (other === undefined) { row.result = "skip"; row.actual = "select has a single option"; return row; }
      await loc.selectOption(other, { timeout: 4000 });
      row.action = `selectOption(${other})`;
    } else {
      await loc.hover({ timeout: 4000 }).catch(() => {}); // a hover-only popup is not the click's effect: the baseline is taken after it
      await settle(page, mon, { quiet: 200, max: 1500 });
      const afterHover = await pageState(page);
      before.mut = afterHover.mut; before.layers = afterHover.layers;
      await loc.click({ timeout: 4000 });
    }
  } catch (e) { clickError = e; }
  if (clickError && desc.tag === "a" && desc.href.startsWith("#")) {
    // A visually hidden in-page link (skip to content) is a keyboard control: a real user reaches it with Tab and presses Enter.
    try {
      await page.reload({ waitUntil: "domcontentloaded" }); // the Tab order starts at the top of a fresh document (the failed click moved the focus start point)
      await settle(page, mon);
      await page.bringToFront(); // a fresh page has no keyboard focus until it is activated
      let focused = false;
      for (let i = 0; i < 12 && !focused; i++) {
        await page.keyboard.press("Tab");
        focused = await loc.evaluate((el) => el === document.activeElement).catch((e) => { dbg(`focus check error ${String(e.message).slice(0, 100)}`); return false; });
        if (process.env.LC_SWEEP_DEBUG) dbg(`tab ${i} active=${await page.evaluate(() => { const a = document.activeElement; return a ? a.tagName + ":" + (a.getAttribute("href") || a.textContent || "").slice(0, 30) : "none"; })}`);
      }
      dbg(`keyboard path focused=${focused}`);
      if (focused) { await page.keyboard.press("Enter"); row.action = "keyboard: Tab to the link, Enter"; clickError = null; }
    } catch (e) { dbg(`keyboard path error ${String(e.message).slice(0, 120)}`); }
  }
  if (clickError) {
    const msg = String(clickError.message || clickError);
    const intercept = /intercepts pointer events/.test(msg);
    row.result = "fail";
    row.failure = intercept ? "unhittable" : /not visible|not enabled|detached|outside of the viewport|not stable/.test(msg) ? "unhittable" : "click-error";
    row.actual = (intercept ? "overlapped: " : "click failed: ") + msg.replace(/\s+/g, " ").slice(0, 260);
    if (guard) await page.unroute("**/*", guardRoute).catch(() => {});
    return row;
  }
  dbg("clicked");
  // effect within 3 s
  const effects = [];
  const deadline = Date.now() + (ctx.window ?? 3000);
  let after = before;
  while (Date.now() < deadline) {
    after = await pageState(page);
    const ev = mon.since(mark);
    effects.length = 0;
    if (after.navigated || after.url !== before.url) effects.push(after.url !== before.url ? `url ${before.url.replace(/^https?:\/\/[^/]+/, "")} -> ${after.url.replace(/^https?:\/\/[^/]+/, "")}` : "page reloaded");
    for (const e of ev) if (["dialog", "popup", "download", "external"].includes(e.type)) effects.push(`${e.type}: ${e.text}`);
    if (after.layers !== before.layers) effects.push(after.layers > before.layers ? `layer opened (${after.layerNames.join(",")})` : "layer closed");
    if (after.mut > before.mut) effects.push(`DOM changed (${after.mut - before.mut} mutations)`);
    if (after.invalid > before.invalid || after.ariaInvalid > before.ariaInvalid || (after.activeInvalid && !before.activeInvalid)) effects.push("validation shown");
    const ownAfter = await safe(() => page.evaluate(([r, i]) => window.__sweepHelpers.own(r, i), [ctx.rootSelector, desc.index]));
    if (ownAfter && ownBefore && ownAfter !== ownBefore && !after.navigated && after.url === before.url) effects.push(`the control's own state changed (${ownBefore.replace(/\|+$/, "")} -> ${ownAfter.replace(/\|+$/, "")})`.slice(0, 120));
    if (Math.abs(after.scrollY - before.scrollY) > 8) effects.push("scrolled");
    if (effects.length) break;
    await page.waitForTimeout(100);
  }
  if (guard) await page.unroute("**/*", guardRoute).catch(() => {});
  const events = mon.since(mark);
  const bad = events.filter((e) => ["console-error", "pageerror", "http5xx", "http404-asset"].includes(e.type));
  row.actual = effects.length ? effects.join("; ") : "no visible change within 3 s";
  row.layer = after.layers > before.layers;
  row.urlChanged = after.url !== before.url;
  row.dialogText = events.find((e) => e.type === "dialog")?.text ?? "";
  row.external = events.find((e) => e.type === "external")?.text ?? "";
  if (bad.length) { row.result = "fail"; row.failure = bad[0].type; row.actual += ` | ${bad.map((e) => `${e.type}: ${e.text}`).join(" ; ").slice(0, 300)}`; }
  else if (irreversible && blocked.length) { row.note = `request ${blocked.join(", ")} blocked by the sweep (not executed)`; if (!effects.length) row.actual = row.note; }
  else if (destructive && blocked.length && !row.layer && !row.dialogText) { row.result = "fail"; row.failure = "destructive-without-confirmation"; row.actual = `fired ${blocked.join(", ")} with no confirmation step (request blocked by the sweep)`; }
  else if (!effects.length && desc.current) { row.note = "already the current item: no change expected"; row.actual = "no change (the current item)"; }
  else if (!effects.length && !(irreversible && blocked.length)) { row.result = "fail"; row.failure = "no-effect"; }
  if (guard && !blocked.length && effects.length && row.result === "pass") row.note = destructive ? (row.layer || row.dialogText ? "confirmation shown, then cancelled" : "inline change only; no request fired") : "";
  if (guard) {
    const fpAfter = await ctx.fingerprint?.();
    if (fpBefore && fpAfter && fpBefore !== fpAfter) { row.result = "fail"; row.failure = "destructive-changed-state"; row.actual += " | server state changed after the destructive control"; }
  }
  row.ms = Date.now() - t0;
  row.after = after;
  return row;
}

// Close whatever the control opened with its own cancel path (a real click on cancel/close, else Escape). Returns true when the layer is gone.
export async function cancelLayer(page, mon) {
  const st = await pageState(page);
  if (!st.layers) return true;
  const buttons = await listControls(page, LAYER_CSS, "layer");
  const cancel = buttons.find((b) => CANCEL_RE.test(b.name) || CANCEL_RE.test(b.ariaLabel));
  if (cancel) await controlLocator(page, LAYER_CSS, cancel).click({ timeout: 3000 }).catch(() => {});
  else await page.keyboard.press("Escape");
  await settle(page, mon, { quiet: 200, max: 2000 });
  if (!(await pageState(page)).layers) return true;
  await page.keyboard.press("Escape");
  await settle(page, mon, { quiet: 200, max: 2000 });
  return !(await pageState(page)).layers;
}

// Back to the page state before the control: leave a layer by its cancel path, otherwise reload the unit URL ("reload if needed").
export async function restore(page, mon, url, row) {
  if (row.layer && !row.urlChanged) { if (await cancelLayer(page, mon)) { const st = await pageState(page); if (st.url === url) return "layer closed"; } }
  await page.goto(url, { waitUntil: "domcontentloaded" }).catch(() => {});
  await settle(page, mon);
  return "reloaded";
}

// ---- ledger ------------------------------------------------------------------------------------------------------------------------------------------------------------------
export class Ledger {
  constructor() { this.rows = []; this.n = 0; }
  add(row) { row.id = `r${String(++this.n).padStart(5, "0")}`; this.rows.push(row); return row; }
  // pages = distinct page/viewport/locale units opened; loads = page-load rows; controls = real control clicks (not page loads, not journey steps)
  counts() {
    const c = { pages: new Set(), loads: { total: 0, fail: 0 }, controls: { total: 0, pass: 0, fail: 0, skip: 0 }, journeys: { total: 0, pass: 0, fail: 0, skip: 0 } };
    for (const r of this.rows) {
      if (r.control === "(page load)") { c.pages.add(`${r.app}|${r.page}|${r.viewport}|${r.locale}`); c.loads.total++; if (r.result === "fail") c.loads.fail++; }
      else if (r.kind === "journey") { c.journeys.total++; c.journeys[r.result]++; }
      else if (r.kind !== "page") { c.controls.total++; c.controls[r.result]++; }
    }
    return { ...c, pages: c.pages.size };
  }
  failures() { return this.rows.filter((r) => r.result === "fail"); }
}

const cell = (s) => String(s ?? "").replace(/\|/g, "\\|").replace(/\s+/g, " ").slice(0, 160);
export function renderMarkdown(rows, summary) {
  const head = ["# G-UI8 click ledger", "", `Generated ${summary.generated}. ${summary.line}`, "",
    "Every row is one real Playwright interaction (click / selectOption). Destructive and irreversible controls stop at their confirmation and are cancelled; `skip` rows name why.", "",
    "| # | App | Page | Viewport | Locale | Scope | Control | Action | Expected | Actual | Result |", "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"];
  const body = rows.map((r) => `| ${r.id} | ${r.app ?? ""} | ${cell(r.page)} | ${r.viewport} | ${r.locale} | ${cell(r.scope)} | ${cell(r.control)}${r.testid ? ` (\`${cell(r.testid)}\`)` : ""} | ${cell(r.action)} | ${cell(r.expected)} | ${cell(r.actual)}${r.note ? ` (${cell(r.note)})` : ""}${r.classSize > 1 ? ` [1 of ${r.classSize} alike]` : ""} | ${r.result === "pass" ? "PASS" : r.result === "skip" ? "SKIP" : `FAIL ${r.failure}`} |`);
  return [...head, ...body, ""].join("\n");
}
export async function writeLedger(dir, rows, summary) {
  const slim = rows.map(({ after, ...r }) => r);
  await writeFile(`${dir}/ledger.json`, JSON.stringify({ summary, rows: slim }, null, 1));
  await writeFile(`${dir}/ledger.md`, renderMarkdown(rows, summary));
}

// ---- known-defect list: explicit, every entry carries an owner unit; entries that match nothing fail the run too (no silent allowlist) ------------------------------
// entry: { id, app?, route, kind, control?, viewport?, locale?, severity, owner_unit, summary, flaky? }; route, kind and control may each be a string or an array (any of).
// control is matched as a substring of "<control name> <test id>". flaky: true = the defect reproduces on some runs only, so an unmatched entry is not stale.
const anyOf = (spec, test) => (Array.isArray(spec) ? spec : [spec]).some(test);
export function matchKnown(failures, known) {
  const used = new Set(), result = [];
  for (const f of failures) {
    const hit = known.find((k) => (!k.app || k.app === f.app) && anyOf(k.route, (r) => r === f.page) && anyOf(k.kind, (x) => x === f.failure) && (!k.control || anyOf(k.control, (c) => `${f.control} ${f.testid}`.includes(c))) && (!k.viewport || k.viewport === f.viewport) && (!k.locale || k.locale === f.locale));
    if (hit) used.add(hit.id);
    result.push({ row: f, known: hit?.id ?? null });
  }
  return { classified: result, stale: known.filter((k) => !used.has(k.id) && !k.flaky) };
}
