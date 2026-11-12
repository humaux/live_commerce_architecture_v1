// Purpose: execute actual member form handlers and immutable-intent/session counterexamples in the existing hook host.
// Depends on: inbox-review-host, Node24 and real members-client; only DS presentation/network are MOCK.
// Used by: focused M7 gate; this does not prove native focus, PG persistence or browser acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { environment, node, nodes, textOf } from "./inbox-review-host.test.ts";
registerHooks({
  resolve(s, c, next) {
    if (s === "@live-commerce/ui/ds")
      return {
        shortCircuit: true,
        url: `data:text/javascript,${encodeURIComponent("import {jsx} from 'react/jsx-runtime'; export const Button=p=>jsx('button',{...p,'aria-disabled':!!p.disabledReason});")}`,
      };
    return next(s, c);
  },
});
const { MemberForms } =
  await import("../../apps/admin/components/members/MemberForms.tsx");
const { sessionBoundary } =
  await import("../../apps/admin/lib/settings-client.ts");
const sid = "11111111-1111-4111-8111-111111111111",
  mid = "22222222-2222-4222-8222-222222222222";
const stamp = "2026-11-10T00:00:00Z";
const member = {
  id: mid,
  display_name: "SYNTHETIC_MEMBER",
  contact_note: null,
  notes: null,
  balance: 50,
  version: 1,
  created_at: stamp,
  updated_at: stamp,
  erased_at: null,
};
const store = {
  id: sid,
  name: "Synthetic",
  currency: "TWD",
  permissions: ["customers:read", "customers:points"],
};
const response = (body: unknown, status = 201) =>
  new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "private, no-store",
    },
  });
const deferred = <T>() => {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
};
const by = (h: any, id: string) =>
  node(h, (n) => n.props["data-testid"] === id);
const field = (h: any, name: string, value: string) =>
  node(h, (n) => n.props.name === name).props.onChange({ target: { value } });
const submit = (h: any) =>
  by(h, "member-form").props.onSubmit({ preventDefault() {} });
async function setup(t: any, adjust = false) {
  const env = environment(t);
  let saved = 0,
    refreshed = 0;
  const props = {
    locale: "en" as const,
    store,
    scope: "render|store|member",
    boundary: await sessionBoundary(),
    status: "ready" as const,
    ...(adjust ? { member, memberId: mid } : {}),
    onSaved: () => saved++,
    onRefresh: () => refreshed++,
  };
  const h = env.mount(() => MemberForms(props));
  const open = () =>
    by(h, adjust ? "member-adjust-open" : "member-create-open").props.onClick({
      currentTarget: { isConnected: false },
    });
  open();
  await h.settle();
  return {
    env,
    props,
    h,
    open,
    saved: () => saved,
    refreshed: () => refreshed,
  };
}
test("MOCK create validates before dispatch; cancel preserves authenticated session; success refreshes once", async (t) => {
  const x = await setup(t);
  let calls = 0,
    logout = 0;
  x.env.window.addEventListener("commerce-session-logout", () => logout++);
  globalThis.fetch = async (_u, init) => {
    calls++;
    assert.equal(JSON.parse(String(init?.body)).display_name, "é");
    return response({ member });
  };
  submit(x.h);
  await x.h.settle();
  assert.equal(calls, 0);
  assert.equal(
    node(x.h, (n) => n.props.name === "name").props["aria-invalid"],
    true,
  );
  by(x.h, "member-form-cancel").props.onClick();
  await x.h.settle();
  assert.equal(logout, 0);
  assert.equal(x.env.document.cookie.includes("__Host-commerce_csrf="), true);
  x.open();
  field(x.h, "name", " e\u0301 ");
  submit(x.h);
  submit(x.h);
  await x.h.settle();
  assert.equal(calls, 1);
  assert.equal(x.saved(), 1);
  assert.equal(logout, 0);
});
test("MOCK adjustment validates integer/reason/balance, reviews and returns without POST, then freezes one intent", async (t) => {
  const x = await setup(t, true),
    held = deferred<Response>(),
    entered = deferred<void>();
  assert.equal(node(x.h, (n) => n.props.name === "note").props.rows, 1);
  assert.equal(by(x.h, "member-form-dialog").props["data-validation-invalid"], undefined);
  let calls = 0;
  let request: RequestInit | undefined;
  globalThis.fetch = async (_u, init) => {
    calls++;
    request = init;
    entered.resolve();
    return held.promise;
  };
  field(x.h, "amount", "1.5");
  field(x.h, "note", "synthetic reason");
  submit(x.h);
  await x.h.settle();
  assert.equal(
    node(x.h, (n) => n.props.name === "amount").props["aria-invalid"],
    true,
  );
  field(x.h, "amount", "51");
  field(x.h, "sign", "-1");
  submit(x.h);
  await x.h.settle();
  assert.equal(calls, 0);
  assert.equal(by(x.h, "member-form-dialog").props["data-validation-invalid"], true);
  assert.equal(node(x.h, (n) => n.props.name === "note").props["aria-invalid"], false);
  field(x.h, "amount", "20");
  field(x.h, "note", "");
  submit(x.h);
  await x.h.settle();
  assert.equal(
    node(x.h, (n) => n.props.name === "note").props["aria-invalid"],
    true,
  );
  assert.equal(by(x.h, "member-form-dialog").props["data-validation-invalid"], true);
  field(x.h, "note", "synthetic reason");
  submit(x.h);
  await x.h.settle();
  assert.equal(by(x.h, "member-form-dialog").props["data-validation-invalid"], undefined);
  assert.match(textOf(x.h.output), /50 → 30/);
  assert.equal(calls, 0);
  by(x.h, "member-form-back").props.onClick();
  await x.h.settle();
  submit(x.h);
  await x.h.settle();
  const confirm = by(x.h, "member-form-confirm").props.onClick;
  confirm();
  confirm();
  await entered.promise;
  await x.h.settle();
  assert.equal(calls, 1);
  assert.equal(by(x.h, "member-form-back").props["aria-disabled"], true);
  by(x.h, "member-form-back").props.onClick();
  assert.equal(by(x.h, "member-form-dialog").props["data-step"], "pending");
  assert.deepEqual(JSON.parse(String(request?.body)), {
    delta: -20,
    note: "synthetic reason",
    order_id: null,
  });
  held.resolve(
    response({
      entry: {
        id: sid,
        member_id: mid,
        delta: -20,
        reason: "manual_adjust",
        order_id: null,
        note: "synthetic reason",
        actor_principal_id: sid,
        created_at: stamp,
      },
      balance: 30,
    }),
  );
  await x.h.settle();
  assert.equal(x.saved(), 1);
});
test("MOCK every repeated invalid submit requests field focus without POST", async (t) => {
  const x = await setup(t, true);
  let calls = 0;
  globalThis.fetch = async () => { calls++; return response({ member }); };
  const requests: string[] = [];
  // Observe the real handler/effect query and focus calls only; this is not native DOM focus.
  const dialog = by(x.h, "member-form-dialog");
  dialog.props.ref.current = {
    open: true,
    querySelector: (selector: string) => ({ focus: () => requests.push(selector) }),
  };
  field(x.h, "amount", "20");
  for (const note of ["", "   ", "   "]) {
    field(x.h, "note", note);
    await x.h.settle();
    const before = requests.length;
    submit(x.h);
    await x.h.settle();
    assert.ok(requests.length > before, `focus requested for invalid note ${JSON.stringify(note)}`);
    assert.equal(requests.at(-1), '[name="note"]');
    assert.equal(node(x.h, (n) => n.props.name === "note").props["aria-invalid"], true);
    assert.ok(textOf(node(x.h, (n) => n.props.role === "alert")));
    assert.equal(calls, 0);
  }
  field(x.h, "amount", "0");
  const before = requests.length;
  submit(x.h);
  await x.h.settle();
  assert.ok(requests.length > before);
  assert.equal(requests.at(-1), '[name="amount"]');
  assert.equal(calls, 0);
});
test("MOCK UNKNOWN keeps original command across read refresh/reopen and cannot resend", async (t) => {
  const x = await setup(t);
  let calls = 0;
  let key = "";
  globalThis.fetch = async (_u, init) => {
    calls++;
    key = (init?.headers as Record<string, string>)["Idempotency-Key"];
    throw Error("transport dropped");
  };
  field(x.h, "name", "SYNTHETIC_PRIVATE");
  submit(x.h);
  await x.h.settle();
  assert.equal(by(x.h, "member-form-dialog").props["data-step"], "unknown");
  assert.ok(key);
  assert.equal(x.saved(), 0);
  by(x.h, "member-form-refresh").props.onClick();
  x.open();
  await x.h.settle();
  assert.equal(x.refreshed(), 1);
  assert.equal(calls, 1);
  assert.equal(
    nodes(x.h.output).some(
      (n) => n.props["data-testid"] === "member-form-submit",
    ),
    false,
  );
  by(x.h, "member-form-dialog").props.onCancel({ preventDefault() {} });
  await x.h.settle();
  assert.equal(by(x.h, "member-form-dialog").props["data-step"], "unknown");
  assert.equal(calls, 1);
});
for (const boundary of ["scope", "logout", "cookie"] as const)
  test(`MOCK held create ignores ${boundary} generation success`, async (t) => {
    const x = await setup(t),
      entered = deferred<void>(),
      held = deferred<Response>();
    globalThis.fetch = async () => {
      entered.resolve();
      return held.promise;
    };
    field(x.h, "name", "SYNTHETIC_PRIVATE");
    submit(x.h);
    await entered.promise;
    if (boundary === "scope") {
      x.props.scope = "new-store-render";
      x.h.dirty = true;
    } else if (boundary === "cookie")
      x.env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
    else x.env.window.dispatchEvent(new Event("commerce-session-logout")); // MOCK lifecycle input, never native UI proof.
    await x.h.settle();
    held.resolve(response({ member }));
    await x.h.settle();
    assert.equal(x.saved(), 0);
    assert.equal(textOf(x.h.output).includes("SYNTHETIC_PRIVATE"), false);
  });
test("MOCK points permission absence hides entry, unrelated customers:write grants do not authorize", async (t) => {
  const x = await setup(t);
  x.props.store = {
    ...store,
    permissions: ["customers:read", "customers:write"],
  };
  x.h.dirty = true;
  await x.h.settle();
  assert.equal(
    nodes(x.h.output).some(
      (n) => n.props["data-testid"] === "member-create-open",
    ),
    false,
  );
  assert.equal(
    nodes(x.h.output).some((n) => n.props["data-testid"] === "member-form"),
    false,
  );
});
for (const code of [
  "points_insufficient",
  "member_erased",
  "idempotency_conflict",
] as const)
  test(`MOCK exact refusal ${code} never becomes success/retry`, async (t) => {
    const x = await setup(t, true);
    let calls = 0;
    globalThis.fetch = async () => {
      calls++;
      return response({ code }, 409);
    };
    field(x.h, "amount", "20");
    field(x.h, "note", "synthetic reason");
    submit(x.h);
    await x.h.settle();
    by(x.h, "member-form-confirm").props.onClick();
    await x.h.settle();
    assert.equal(calls, 1);
    assert.equal(x.saved(), 0);
    assert.equal(
      by(x.h, "member-form-dialog").props["data-step"],
      code === "idempotency_conflict" ? "unknown" : "refused",
    );
  });

for (const direction of [1, -1])
  test(`MOCK adjustment held pending preserves confirmation summary and exclusion (${direction})`, async (t) => {
    const x = await setup(t, true), held = deferred<Response>(), entered = deferred<void>();
    let calls = 0;
    globalThis.fetch = async () => { calls++; entered.resolve(); return held.promise; };
    field(x.h, "sign", String(direction));
    field(x.h, "amount", "20");
    field(x.h, "note", "synthetic pending reason");
    submit(x.h);
    await x.h.settle();
    const summary = textOf(x.h.output);
    const confirmationTitle = textOf(node(x.h, (n) => n.props.id === "member-form-title"));
    const confirm = by(x.h, "member-form-confirm").props.onClick;
    confirm();
    await entered.promise;
    await x.h.settle();
    try {
      assert.equal(by(x.h, "member-form-dialog").props["data-step"], "pending");
      assert.equal(nodes(x.h.flush()).filter((n) => ["input", "textarea", "select"].includes(n.type as string)).length, 0);
      const pending = textOf(x.h.output);
      assert.equal(textOf(node(x.h, (n) => n.props.id === "member-form-title")), confirmationTitle);
      assert.equal(by(x.h, "member-form-dialog").props["aria-describedby"], "member-form-pending");
      assert.equal(node(x.h, (n) => n.props.id === "member-form-pending").props.role, "status");
      assert.equal(nodes(x.h.flush()).some((n) => n.props.id === "member-form-guidance"), false);
      assert.ok(summary.includes("SYNTHETIC_MEMBER"));
      assert.ok(pending.includes("SYNTHETIC_MEMBER"));
      assert.ok(pending.includes("synthetic pending reason"));
      assert.ok(pending.includes(`50 → ${50 + direction * 20}`));
      assert.ok(pending.replace(/\s/g, "").includes(`${direction > 0 ? "+" : "−"}20`));
      assert.ok(textOf(node(x.h, (n) => n.props.role === "status")).length > 0);
      for (const id of ["member-form-back", "member-form-confirm"])
        assert.equal(by(x.h, id).props["aria-disabled"], true);
      by(x.h, "member-form-back").props.onClick();
      by(x.h, "member-form-confirm").props.onClick();
      confirm(); // Stale pre-pending handler must retain synchronous single-intent exclusion.
      by(x.h, "member-form-dialog").props.onCancel({ preventDefault() {} });
      await x.h.settle();
      assert.equal(by(x.h, "member-form-dialog").props["data-step"], "pending");
      assert.equal(calls, 1);
    } finally {
      held.resolve(response({ entry: { id: sid }, balance: 50 + direction * 20 }));
      await x.h.settle();
    }
  });
test("MOCK create held pending retains disabled fields and status without duplicate POST or dismiss", async (t) => {
  const x = await setup(t), held = deferred<Response>(), entered = deferred<void>();
  let calls = 0;
  globalThis.fetch = async () => { calls++; entered.resolve(); return held.promise; };
  field(x.h, "name", "SYNTHETIC_CREATE");
  submit(x.h);
  await entered.promise;
  await x.h.settle();
  try {
    assert.equal(by(x.h, "member-form-dialog").props["data-step"], "pending");
    for (const name of ["name", "contact", "notes"])
      assert.equal(node(x.h, (n) => n.props.name === name).props.disabled, true);
    assert.ok(textOf(node(x.h, (n) => n.props.role === "status")).length > 0);
    for (const id of ["member-form-cancel", "member-form-submit"])
      assert.equal(by(x.h, id).props["aria-disabled"], true);
    by(x.h, "member-form-cancel").props.onClick();
    submit(x.h);
    await x.h.settle();
    assert.equal(by(x.h, "member-form-dialog").props["data-step"], "pending");
    assert.equal(calls, 1);
  } finally { held.resolve(response({ member })); await x.h.settle(); }
});
