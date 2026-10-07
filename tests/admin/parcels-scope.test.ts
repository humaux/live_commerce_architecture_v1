// Purpose: independent authoritative-denial/privacy countercases for both ParcelMerge reads.
// Depends on: actual ParcelMerge effect, MerchantOrders clear/prop callbacks and real parcels/orders read clients.
// Used by: W3-U4 Node gate; AST callbacks are driven directly (MOCK lifecycle, not React/browser acceptance).
// Invariants: denial purges all parent private state, aborts sibling first, and rejects late paints; 503 remains retryable.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { createRequire, registerHooks } from "node:module";
import { fileURLToPath } from "node:url";
const ts = createRequire(import.meta.url)("typescript-api");
registerHooks({
  resolve(specifier, context, next) {
    if (specifier.startsWith(".") && context.parentURL) {
      const target = new URL(specifier, context.parentURL);
      if (!/\.[a-z]+$/i.test(target.pathname) && existsSync(fileURLToPath(target) + ".ts"))
        return next(target.href + ".ts", context);
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.endsWith(".ts"))
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(fileURLToPath(url), "utf8"), {
          compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext },
        }).outputText,
      };
    return next(url, context);
  },
});
const clients = await import("../../apps/admin/lib/parcels-client.ts");
const { OrderReadError } = await import("../../apps/admin/lib/orders-client.ts");
const { reconcileGroups } = await import("../../apps/admin/lib/parcels-model.ts");
const { parcelCopy } = await import("../../apps/admin/lib/parcels-copy.ts");
function parse(path: string) {
  return ts.createSourceFile(path, readFileSync(path, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
}
const child = parse("apps/admin/components/ParcelGroup.tsx"),
  parent = parse("apps/admin/components/MerchantOrders.tsx");
const component = (ast: any, name: string) =>
  ast.statements.find((n: any) => ts.isFunctionDeclaration(n) && n.name?.text === name);
const childBody = component(child, "ParcelMerge").body,
  parentBody = component(parent, "MerchantOrders").body;
function variable(name: string) {
  return parentBody.statements
    .find(
      (s: any) =>
        ts.isVariableStatement(s) && s.declarationList.declarations.some((d: any) => d.name.getText(parent) === name),
    )
    ?.declarationList.declarations.find((d: any) => d.name.getText(parent) === name)?.initializer;
}
function prop(name: string) {
  let found: any;
  function visit(n: any) {
    if (ts.isJsxSelfClosingElement(n) && n.tagName.getText(parent) === "ParcelMerge")
      found = n.attributes.properties.find((p: any) => p.name?.text === name)?.initializer?.expression;
    ts.forEachChild(n, visit);
  }
  visit(parentBody);
  return found;
}
function callback(node: any) {
  if (node && ts.isIdentifier(node)) node = variable(node.text);
  return node && ts.isCallExpression(node) && node.expression.getText(parent) === "useCallback"
    ? node.arguments[0]
    : node;
}
function run(node: any, ast: any, env: Record<string, any>) {
  assert.ok(node, "actual lifecycle source required");
  const code = ts.transpileModule(`return (${node.getText(ast)});`, {
    compilerOptions: { target: ts.ScriptTarget.ES2023 },
  }).outputText;
  return new Function(...Object.keys(env), code)(...Object.values(env));
}
const effect = childBody.statements.find(
  (s: any) =>
    ts.isExpressionStatement(s) &&
    ts.isCallExpression(s.expression) &&
    s.expression.expression.getText(child) === "useEffect",
).expression.arguments[0];
const ids = ["10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002"];
const open = {
  items: [
    {
      group_id: "20000000-0000-4000-8000-000000000001",
      version: 1,
      created_at: "2030-01-01T00:00:00Z",
      members: ids.map((order_id) => ({
        order_id,
        order_number: `LC-${order_id.replaceAll("-", "").toUpperCase()}`,
        recipient_masked: "M***",
      })),
    },
  ],
};
const suggestions = { items: [{ recipient_masked: "M***", order_ids: ids }] };
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", "cache-control": "private, no-store" },
  });
function harness() {
  const state: any = {
    view: { key: "MOCK_SCOPE", status: "ready", page: { items: ["MOCK_ORDER"] }, detail: "MOCK_DETAIL" },
    bulk: { scope: "MOCK_SCOPE", rows: { [ids[0]]: false } },
    parcels: { scope: "MOCK_SCOPE", groups: ["MOCK_GROUP"] },
    search: { store: "MOCK_STORE", value: "MOCK_PRIVATE_QUERY", cursor: "" },
    actions: { fulfillment_write: true },
  };
  const ref = (current: any) => ({ current });
  const env: any = {
    key: "MOCK_SCOPE",
    bulkScope: "MOCK_SCOPE",
    generation: ref(10),
    controller: ref(new AbortController()),
    cookie: ref("MOCK_COOKIE"),
    session: ref("MOCK_SESSION"),
    hidden: ref(false),
    blocked: ref(false),
    seen: ref({ ids: new Set(ids) }),
    polling: ref(false),
    flushSync: (work: any) => work(),
  };
  for (const name of ["View", "Bulk", "Parcels", "Search", "Actions", "Fresh"]) {
    const key = name.toLowerCase();
    env[`set${name}`] = (value: any) => {
      state[key] = typeof value === "function" ? value(state[key]) : value;
    };
  }
  env.clear = run(callback(variable("clear")), parent, env);
  const scope = prop("onScopeLost"),
    onScopeLost = scope ? run(callback(scope), parent, env) : undefined;
  const onGroups = run(callback(prop("onGroups")), parent, env);
  const local: any = { suggestions: ["MOCK_SUGGESTION"], problem: "" };
  const tasks: { name: string; promise: Promise<any> }[] = [];
  const childEnv: any = {
    store: "MOCK_STORE",
    boundary: "MOCK_SESSION",
    tick: 0,
    refreshGen: 0,
    created: ref(0),
    groupsRef: ref([]),
    c: parcelCopy.en,
    reconcileGroups,
    OrderReadError,
    onGroups,
    onScopeLost: (code: string) => {
      local.denialCode = code;
      local.abortedBeforeCallback = local.signal?.aborted === true;
      onScopeLost?.(code);
    },
    setSuggestions: (v: any) => {
      local.suggestions = typeof v === "function" ? v(local.suggestions) : v;
    },
    setProblem: (v: any) => {
      local.problem = typeof v === "function" ? v(local.problem) : v;
    },
  };
  const commit = childBody.statements.find((s: any) => ts.isFunctionDeclaration(s) && s.name?.text === "commit");
  childEnv.commit = run(commit, child, childEnv);
  for (const name of ["readMergeSuggestions", "readOpenParcelGroups"] as const)
    childEnv[name] = (...args: any[]) => {
      local.signal = args[1];
      const promise = (clients[name] as any)(...args);
      tasks.push({ name, promise });
      return promise;
    };
  return { state, env, local, tasks, onGroups, start: () => run(effect, child, childEnv)() };
}
for (const read of ["readMergeSuggestions", "readOpenParcelGroups"] as const)
  for (const [status, code] of [
    [401, "signed-out"],
    [403, "forbidden"],
    [404, "not-found"],
  ] as const) {
    test(`${read} ${status} immediately purges parent scope and refuses late sibling 200`, async (t) => {
      const old = globalThis.fetch,
        held = Promise.withResolvers<Response>(),
        h = harness();
      t.after(() => {
        held.resolve(json({ items: [] }));
        globalThis.fetch = old;
      });
      globalThis.fetch = async (path) =>
        String(path).includes(read === "readMergeSuggestions" ? "merge-suggestions" : "parcel-groups")
          ? json({}, status)
          : held.promise;
      const cleanup = h.start();
      t.after(cleanup);
      await Promise.allSettled(h.tasks.filter((x) => x.name === read).map((x) => x.promise));
      assert.equal(h.state.view.page, null, "authoritative parcel denial must purge cached parent orders immediately");
      assert.equal(h.state.view.detail, null);
      assert.deepEqual(h.state.bulk.rows, {});
      assert.deepEqual(h.state.parcels.groups, []);
      assert.equal(h.state.actions, null);
      assert.equal(h.env.session.current, "");
      assert.equal(h.env.cookie.current, "");
      assert.equal(h.env.blocked.current, true);
      assert.equal(h.env.controller.current.signal.aborted, true);
      assert.equal(h.local.denialCode, code);
      assert.equal(h.local.abortedBeforeCallback, true, "abort sibling before notifying parent");
      const beforeLateSuggestions = [...h.local.suggestions];
      held.resolve(json(read === "readMergeSuggestions" ? open : suggestions));
      await Promise.allSettled(h.tasks.map((x) => x.promise));
      assert.deepEqual(h.state.parcels.groups, []);
      assert.deepEqual(h.local.suggestions, beforeLateSuggestions, "late suggestions cannot repaint lost scope");
      h.onGroups(["MOCK_LATE_CAPTURED_GROUP"]);
      assert.deepEqual(h.state.parcels.groups, [], "captured parent groups callback also refuses blocked scope");
    });
  }
for (const read of ["readMergeSuggestions", "readOpenParcelGroups"] as const)
  test(`${read} 503 preserves parent and local retry recovers actual DTOs`, async (t) => {
    const old = globalThis.fetch,
      h = harness();
    t.after(() => {
      globalThis.fetch = old;
    });
    globalThis.fetch = async (path) =>
      String(path).includes(read === "readMergeSuggestions" ? "merge-suggestions" : "parcel-groups")
        ? json({}, 503)
        : json(String(path).includes("merge-suggestions") ? suggestions : open);
    const cleanup = h.start();
    await Promise.allSettled(h.tasks.map((x) => x.promise));
    cleanup();
    assert.notEqual(h.state.view.page, null);
    assert.notEqual(h.state.view.detail, null);
    assert.equal(h.env.blocked.current, false);
    assert.equal(h.local.denialCode, undefined);
    assert.equal(
      h.local.problem,
      read === "readMergeSuggestions" ? parcelCopy.en.suggestionsUnavailable : parcelCopy.en.groupsUnavailable,
    );
    globalThis.fetch = async (path) => json(String(path).includes("merge-suggestions") ? suggestions : open);
    const retry = h.start();
    t.after(retry);
    await Promise.allSettled(h.tasks.map((x) => x.promise));
    assert.equal(h.local.problem, "");
    assert.equal(h.local.suggestions.length, 1);
    assert.equal(h.state.parcels.groups.length, 1);
  });
test("captured parent groups callback refuses hidden scope", () => {
  const h = harness();
  h.env.hidden.current = true;
  h.onGroups(["MOCK_LATE_HIDDEN_GROUP"]);
  assert.deepEqual(h.state.parcels.groups, ["MOCK_GROUP"]);
});
