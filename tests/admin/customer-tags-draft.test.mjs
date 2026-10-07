// Purpose: controlled-hook regression of actual CustomerNotes callbacks/effects when parent reads refresh.
// Depends on: CustomerTagsNotes source, TypeScript API, synthetic React hooks and frozen model/copy modules.
// Used by: W6-U1 local draft-loss red/green evidence; does not replace real browser clicks.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { customerTagsCopy } from "../../apps/admin/lib/customer-tags-copy.ts";
import * as model from "../../apps/admin/lib/customer-tags-model.ts";
import { displayTime } from "../../packages/format/src/index.ts";

const id = "abcdef11-1111-4111-8111-111111111111";
const note = { id, author_id: id, body: "old synthetic note", version: 2, created_at: "2026-10-07T00:00:00Z", edited_at: null, own: true };
const source = ts.transpileModule(readFileSync(new URL("../../apps/admin/components/CustomerTagsNotes.tsx", import.meta.url), "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText;

function harness(readTagData = () => new Promise(() => {}), logouts = [], override = {}) {
  const slots = []; let cursor = 0; const effects = []; const writes = [];
  const react = {
    useId: () => { const index = cursor++; return slots[index] ??= `synthetic-${index}`; },
    useState: (initial) => {
      const index = cursor++; if (!(index in slots)) slots[index] = { value: initial };
      return [slots[index].value, (value) => { slots[index].value = typeof value === "function" ? value(slots[index].value) : value; }];
    },
    useRef: (initial) => { const index = cursor++; return slots[index] ??= { current: initial }; },
    useEffect: (effect, deps) => {
      const index = cursor++; const old = slots[index];
      if (!old || !deps || deps.some((v, i) => !Object.is(v, old.deps[i]))) {
        slots[index] = { deps, cleanup: old?.cleanup };
        effects.push(() => { slots[index].cleanup?.(); slots[index].cleanup = effect(); });
      }
    },
  };
  const jsx = (type, props) => ({ type, props }); const module = { exports: {} };
  runInNewContext(source, { module, exports: module.exports, AbortController, Intl, Set,
    require: (path) => {
      if (path === "react") return react;
      if (path === "react/jsx-runtime") return { jsx, jsxs: jsx };
      if (path === "../lib/customer-tags-copy") return { customerTagsCopy };
      if (path === "../lib/customer-tags-model") return model;
      if (path === "../lib/orders-model") return { displayTime };
      // Pending read fixture deliberately never returns: only parent prop/effect/callback behavior is under test.
      if (path === "../lib/customer-tags-client") return { readTagData };
      if (path === "../lib/session-events") return { signalLogout: () => logouts.push("logout") }; // browser-event edge only
      throw new Error(`Unexpected fixture import ${path}`);
    },
  });
  const props = { locale: "en", store: { id, permissions: ["customers:read", "customers:write", "customers:privacy"] },
    detail: { customer_id: id, active: true, notes: [note] }, boundary: "a".repeat(64), refresh: async () => {},
    write: { locked: false, run: (...args) => writes.push(args), setNotice: () => {} }, ...override };
  const render = (current = props) => { cursor = 0; return module.exports.CustomerNotes(current); };
  const flush = () => { for (const effect of effects.splice(0)) effect(); };
  const nodes = (tree) => {
    const all = []; const walk = (n) => {
      if (Array.isArray(n)) { for (const child of n) walk(child); }
      else if (n && typeof n === "object" && n.props) { all.push(n); walk(n.props.children); }
    }; walk(tree); return all;
  };
  const textarea = (tree) => nodes(tree).find((n) => n.type === "textarea");
  const button = (tree, label) => nodes(tree).find((n) => n.type === "button" && n.props.children === label);
  const form = (tree) => nodes(tree).find((n) => n.type === "form");
  render(); flush();
  return { props, render, flush, nodes, textarea, button, form, writes };
}

test("passive tag/detail refresh preserves an unsaved new-note draft", () => {
  const h = harness(); let tree = h.render();
  h.textarea(tree).props.onChange({ target: { value: "unsaved synthetic add" } });
  const next = { ...h.props, detail: { ...h.props.detail, notes: [{ ...note }] } };
  h.render(next); h.flush(); tree = h.render(next);
  assert.equal(h.textarea(tree).props.value, "unsaved synthetic add");
  h.form(tree).props.onSubmit({ preventDefault() {} });
  assert.equal(h.writes[0][0], "POST"); assert.equal(h.writes[0][2].body, "unsaved synthetic add");
});
test("passive refresh updates visible notes but preserves edit draft and original CAS version", () => {
  const h = harness(); let tree = h.render();
  h.button(tree, "Edit").props.onClick(); tree = h.render();
  h.textarea(tree).props.onChange({ target: { value: "unsaved synthetic edit" } });
  const newer = { ...note, body: "new server synthetic note", version: 3 };
  const next = { ...h.props, detail: { ...h.props.detail, notes: [newer] } };
  h.render(next); h.flush(); tree = h.render(next);
  assert.equal(h.textarea(tree).props.value, "unsaved synthetic edit");
  assert.ok(h.nodes(tree).some((n) => n.type === "p" && n.props.children === newer.body));
  h.form(tree).props.onSubmit({ preventDefault() {} });
  assert.equal(h.writes[0][0], "PATCH"); assert.equal(h.writes[0][1], `customers/${id}/notes/${id}`);
  assert.equal(h.writes[0][2].body, "unsaved synthetic edit"); assert.equal(h.writes[0][2].version, 2);
});
test("explicit refresh signal clears old edit draft while passive array changes do not", () => {
  const h = harness(); let tree = h.render(); h.button(tree, "Edit").props.onClick(); tree = h.render();
  h.textarea(tree).props.onChange({ target: { value: "unsaved synthetic edit" } });
  const next = { ...h.props, reloadVersion: 1 };
  h.render(next); h.flush(); tree = h.render(next);
  assert.equal(h.textarea(tree).props.value, ""); assert.equal(h.button(tree, "Cancel"), undefined);
});

// Codex review P2 (PR #3): a note read that loses authorization must not leave private note bodies or a draft on screen.
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
test("an unauthorized note read clears private note bodies and the draft", async () => {
  const h = harness(() => Promise.reject(new Error("unauthorized"))); let tree = h.render();
  h.textarea(tree).props.onChange({ target: { value: "unsaved synthetic draft" } });
  await settle(); tree = h.render();
  assert.ok(!h.nodes(tree).some((n) => n.type === "p" && n.props.children === note.body), "note body still rendered after unauthorized");
  assert.equal(h.textarea(tree)?.props.value ?? "", "");
});
test("a transient note read failure keeps the notes already shown", async () => {
  const h = harness(() => Promise.reject(new Error("retry_later"))); await settle(); const tree = h.render();
  assert.ok(h.nodes(tree).some((n) => n.type === "p" && n.props.children === note.body));
});

// Codex review P2 (PR #3): an unauthorized read must end the session for the whole page (global logout lifecycle), not only this widget.
test("an unauthorized note read signals the global logout once; a transient failure does not", async () => {
  const lost = []; harness(() => Promise.reject(new Error("unauthorized")), lost); await settle();
  assert.deepEqual(lost, ["logout"]);
  const kept = []; harness(() => Promise.reject(new Error("retry_later")), kept); await settle();
  assert.deepEqual(kept, []);
});

// Codex review P1 (PR #3): a writer WITHOUT customers:privacy keeps Edit/Delete on notes the server marks own after a
// fresh mount (reload); another author's note gets neither. Authorship comes from the server flag, not session memory.
test("non-privacy writer: own notes keep Edit and Delete on a fresh mount, others' notes do not", () => {
  const other = { ...note, id: "abcdef22-2222-4222-8222-222222222222", body: "another author", own: false };
  const h = harness(undefined, [], { store: { id, permissions: ["customers:read", "customers:write"] },
    detail: { customer_id: id, active: true, notes: [note, other] } });
  const tree = h.render();
  const rows = h.nodes(tree).filter((n) => n.type === "li" && n.props.className === undefined);
  const labels = (row) => h.nodes(row.props.children).filter((n) => n.type === "button").map((n) => n.props.children);
  assert.deepEqual(labels(rows[0]), ["Edit", "Delete"]);
  assert.deepEqual(labels(rows[1]), []);
});
