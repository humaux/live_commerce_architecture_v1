// Purpose: regress (1) the tag manager staying mounted through an in-place catalogue refresh after a write and (2) a catalogue
//   forbidden/not_found asking the parent guarded read(s) to re-run, while transient failures stay local (Codex review P2, PR #3).
// Depends on: actual CustomerTags.tsx and Customers.tsx source via the TypeScript API, synthetic React hooks and element tree;
//   only the browser/network edges (readTagData transport, session-event dispatch) and child components are stubbed.
// Used by: scripts/dev/test-node.sh (W6-U1 local MOCK evidence; browser gates remain the click proof).
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { customerTagsCopy } from "../../apps/admin/lib/customer-tags-copy.ts";
import * as model from "../../apps/admin/lib/customer-tags-model.ts";
import * as customersModel from "../../apps/admin/lib/customers-model.ts";

const compile = (file) => ts.transpileModule(readFileSync(new URL(`../../apps/admin/components/${file}`, import.meta.url), "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
const id = "abcdef11-1111-4111-8111-111111111111";

function hooks() {
  const slots = []; const effects = []; let cursor = 0;
  return { reset: () => { cursor = 0; }, flush: () => { for (const e of effects.splice(0)) e(); },
    react: {
      useState: (initial) => { const i = cursor++; if (!(i in slots)) slots[i] = { value: initial };
        return [slots[i].value, (v) => { slots[i].value = typeof v === "function" ? v(slots[i].value) : v; }]; },
      useRef: (initial) => { const i = cursor++; return slots[i] ??= { current: initial }; },
      useEffect: (effect, deps) => { const i = cursor++; const old = slots[i];
        if (!old || !deps || deps.some((v, k) => !Object.is(v, old.deps[k]))) { slots[i] = { deps, cleanup: old?.cleanup };
          effects.push(() => { slots[i].cleanup?.(); slots[i].cleanup = effect(); }); } },
      useId: () => `id-${cursor++}`,
    } };
}
const jsx = (type, props) => ({ type, props });
const walk = (node, out = []) => { if (Array.isArray(node)) node.forEach((n) => walk(n, out)); else if (node && typeof node === "object" && node.props) { out.push(node); walk(node.props.children, out); } return out; };

function tagBody(readTagData, scopeLost) {
  const h = hooks(); const module = { exports: {} }; const logouts = [];
  runInNewContext(compile("CustomerTags.tsx"), { module, exports: module.exports, AbortController, Set, Intl,
    require: (p) => {
      if (p === "react") return h.react; if (p === "react/jsx-runtime") return { jsx, jsxs: jsx };
      if (p === "../lib/customer-tags-copy") return { customerTagsCopy }; if (p === "../lib/customer-tags-model") return model;
      if (p === "../lib/customer-tags-client") return { readTagData };
      if (p === "../lib/session-events") return { signalLogout: () => logouts.push("logout") };
      if (p === "../lib/customer-tags-write") return { useTagWrite: () => ({ locked: false, run() {}, setNotice() {} }) };
      if (p === "./CustomerTagsNotes") return { CustomerNotes: () => null };
      if (p === "./CustomerTagsForms") return { TagFields: () => null, TagWriteStatus: () => null };
      if (/\.css$/.test(p) || p === "@live-commerce/i18n" || p.startsWith("../lib/model")) return {};
      throw new Error(`Unexpected import ${p}`);
    } });
  const props = { locale: "en", store: { id, permissions: ["customers:read", "customers:write"] }, boundary: "b".repeat(64),
    detail: { customer_id: id, active: true, tags: [], tags_revision: "a".repeat(64), notes: [] }, onChanged: async () => true, onScopeLost: scopeLost };
  const element = module.exports.CustomerTags(props); h.reset(); element.type(element.props); h.flush();
  return { logouts };
}

test("a catalogue forbidden or not_found asks the parent to re-run its guarded reads, with no logout", async () => {
  for (const code of ["forbidden", "not_found"]) {
    const lost = []; const { logouts } = tagBody(() => Promise.reject(new Error(code)), () => lost.push(code)); await settle();
    assert.deepEqual(lost, [code]); assert.deepEqual(logouts, [], "session is still valid: no logout");
  }
});
test("transient catalogue failures stay local; unauthorized logs out but is not a scope loss", async () => {
  for (const code of ["unavailable", "retry_later"]) { const lost = []; tagBody(() => Promise.reject(new Error(code)), () => lost.push(code)); await settle(); assert.deepEqual(lost, [], code); }
  const lost = []; const { logouts } = tagBody(() => Promise.reject(new Error("unauthorized")), () => lost.push("x")); await settle();
  assert.deepEqual(logouts, ["logout"]); assert.deepEqual(lost, []);
});

// Customers.tsx wiring: a successful tag write must refresh the catalogue in place. reload() flips the guarded read to loading,
// which would unmount the manager (and its open dialog) before the success notice. The stub mirrors the real hook contract
// (reload -> loading; refresh -> status unchanged), from useGuardedRead in lib/customers-client.ts).
function customers() {
  const h = hooks(); const module = { exports: {} }; const calls = [];
  const state = { catalog: { status: "ready", boundary: "b".repeat(64), data: { items: [] } }, list: { status: "ready", boundary: "b".repeat(64), data: { items: [], next_cursor: "" } } };
  const guarded = (name) => ({ ...state[name], reload: () => { calls.push(`${name}.reload`); state[name].status = "loading"; }, refresh: async () => { calls.push(`${name}.refresh`); return true; } });
  let n = 0;
  runInNewContext(compile("Customers.tsx"), { module, exports: module.exports, URLSearchParams,
    require: (p) => {
      if (p === "react") return h.react; if (p === "react/jsx-runtime") return { jsx, jsxs: jsx };
      if (p === "next/link") return { default: "a" }; if (p === "next/navigation") return { useRouter: () => ({ push() {} }) };
      if (p === "@live-commerce/ui") return { Badge: "span" };
      if (p === "@/lib/client") return { money: () => "" };
      if (p === "@/lib/customers-client") return { readCustomers: async () => ({}), useGuardedRead: () => guarded(n++ % 2 === 0 ? "list" : "catalog") };
      if (p === "@/lib/customers-model") return customersModel;
      if (p === "@/lib/customer-tags-client") return { readTagCatalog: async () => ({}) };
      if (p === "@/lib/customer-tags-copy") return { customerTagsCopy };
      if (p === "./CustomerTags") return { CustomerTagManager: "CustomerTagManager", TagBadges: "TagBadges" };
      if (p === "@/lib/orders-model") return { displayTime: () => "" };
      if (p === "@/lib/customers-copy") return { customersCopy: { en: new Proxy({}, { get: () => "x" }) } };
      if (["./WorkspaceFrame", "./AdminPageHeader", "./Icon"].includes(p)) return { WorkspaceFrame: "div", AdminPageHeader: "div", Icon: "i" };
      if (/\.css$/.test(p) || p === "@live-commerce/i18n" || p === "@/lib/model") return {};
      throw new Error(`Unexpected import ${p}`);
    } });
  const render = () => { h.reset(); n = 0; return module.exports.Customers({ locale: "en", stores: [], store: { id, name: "S", currency: "TWD", permissions: ["customers:read", "customers:write", "orders:read"] }, q: "", after: "", initialError: null, renderKey: "r" }); };
  return { render, calls, state };
}
test("after a tag write the manager stays mounted: the catalogue refreshes in place, never reloads", () => {
  const c = customers(); const manager = () => walk(c.render()).find((e) => e.type === "CustomerTagManager");
  assert.ok(manager(), "manager mounts once the catalogue is ready");
  manager().props.onChanged();
  assert.ok(c.calls.includes("catalog.refresh")); assert.ok(!c.calls.includes("catalog.reload"), "catalog.reload would unmount the open dialog");
  assert.ok(manager(), "manager (and its open dialog) must survive the refresh; a second rename/delete must still be possible");
  manager().props.onChanged(); assert.ok(manager());
});
test("a manager-reported scope loss reloads the customer list and catalogue guarded reads", () => {
  const c = customers(); walk(c.render()).find((e) => e.type === "CustomerTagManager").props.onScopeLost();
  assert.deepEqual([...c.calls].sort(), ["catalog.reload", "list.reload"]);
});
