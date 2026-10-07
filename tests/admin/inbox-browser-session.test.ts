// Purpose: keep INU02 authority probes inside the authenticated browser session on HTTP loopback.
// Depends on: actual installed Playwright cookie matcher and actual INU02/helper source via TypeScript AST.
// Used by: Node red/green gate; page/request/fetch are transport drivers, browser/Go/PG remain NOT_RUN.
// Invariants: preserve exact 404/403 and DM-absence assertions; no Cookie/header injection or auth bypass.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
const require = createRequire(import.meta.url),
  ts = require("typescript-api");
const pwRequire = createRequire(createRequire(require.resolve("@playwright/test")).resolve("playwright/package.json"));
const corePath = resolve(dirname(pwRequire.resolve("playwright-core/package.json")), "lib/coreBundle.js");
const core = ts.createSourceFile(
  corePath,
  readFileSync(corePath, "utf8"),
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.JS,
);
const functions = new Map<string, string>();
let matcher: any;
function visit(node: any) {
  if (
    ts.isFunctionDeclaration(node) &&
    ["isLocalHostname", "domainMatches", "pathMatches", "filterCookies"].includes(node.name?.text)
  )
    functions.set(node.name.text, node.getText(core));
  if (ts.isClassExpression(node) && node.members.some((m: any) => m.name?.getText(core) === "_networkCookie"))
    matcher = node.members.find((m: any) => m.name?.getText(core) === "matches");
  ts.forEachChild(node, visit);
}
visit(core);
assert.equal(functions.size, 4);
assert.ok(matcher?.body, "actual installed Cookie.matches required");
const installed = new Function(
  `${[...functions.values()].join("\n")}\nreturn {filterCookies,matches:function(url3) ${matcher.body.getText(core)}};`,
)();
const cookie = { name: "__Host-commerce_session", value: "MOCK_ONLY", domain: "127.0.0.1", path: "/", secure: true };
test("installed APIRequestContext cookie rules exclude Secure cookie from HTTP127 but retain HTTPS/localhost", () => {
  assert.equal(installed.matches.call({ _raw: cookie }, new URL("http://127.0.0.1:1234/api/stores/mock/inbox")), false);
  assert.equal(installed.filterCookies([cookie], ["http://127.0.0.1:1234/api/stores/mock/inbox"]).length, 0);
  assert.equal(installed.matches.call({ _raw: cookie }, new URL("https://127.0.0.1:1234/api/stores/mock/inbox")), true);
  assert.equal(
    installed.matches.call(
      { _raw: { ...cookie, domain: "localhost" } },
      new URL("http://localhost:1234/api/stores/mock/inbox"),
    ),
    true,
  );
});
const file = "tests/admin/inbox-ui.spec.ts",
  source = readFileSync(file, "utf8");
const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
const registration = ast.statements.find(
  (node: any) =>
    ts.isExpressionStatement(node) &&
    ts.isCallExpression(node.expression) &&
    node.expression.arguments[0]?.text?.startsWith("INU02 "),
);
assert.ok(registration, "execute the real INU02 read and assertion statements");
const callback = registration.expression.arguments[1];
const helpers = ast.statements.filter(
  (node: any) => ts.isFunctionDeclaration(node) && node.name?.text === "browserInboxRead",
);
function probe(name: "cross" | "denied") {
  const statements = callback.body.statements;
  const index = statements.findIndex(
    (node: any) =>
      ts.isVariableStatement(node) && node.declarationList.declarations.some((d: any) => d.name.getText(ast) === name),
  );
  assert.ok(index >= 0);
  const code = ts.transpileModule(
    `${helpers.map((node: any) => node.getText(ast)).join("\n")}\nreturn async()=>{${statements
      .slice(index, index + 3)
      .map((node: any) => node.getText(ast))
      .join("\n")}};`,
    { compilerOptions: { target: ts.ScriptTarget.ES2023 } },
  ).outputText;
  return new Function("page", "store", "ids", "dm", "expect", code);
}
const dm = "MOCK_DM_PRIVATE";
const expect = (actual: unknown) => ({
  toBe: (expected: unknown) => assert.equal(actual, expected),
  not: { toContain: (value: string) => assert.equal(String(actual).includes(value), false) },
});
for (const [name, status] of [
  ["cross", 404],
  ["denied", 403],
] as const) {
  async function execute(t: any, browserStatus: number, body = "{}") {
    const previous = globalThis.fetch,
      reads: { path: string; init?: RequestInit }[] = [];
    t.after(() => {
      globalThis.fetch = previous;
    });
    globalThis.fetch = async (path, init) => {
      reads.push({ path: String(path), init });
      return new Response(body, { status: browserStatus });
    };
    const page = {
      request: {
        get: async (path: string) => {
          const carriesCookie = installed.matches.call({ _raw: cookie }, new URL(path, "http://127.0.0.1:1234"));
          return { status: () => (carriesCookie ? browserStatus : 401), text: async () => body };
        },
      },
      evaluate: async (fn: any, path: string) => fn(path),
    };
    return { work: probe(name)(page, "MOCK_STORE", { foreign: "MOCK_FOREIGN" }, dm, expect)(), reads };
  }
  test(`actual INU02 ${name} uses browser auth and retains exact ${status}/DM absence`, async (t) => {
    const { work, reads } = await execute(t, status);
    await work;
    assert.equal(reads.length, 1);
    assert.equal(
      reads[0].path,
      name === "cross"
        ? "/api/stores/MOCK_STORE/inbox/conversations/MOCK_FOREIGN/messages"
        : "/api/stores/MOCK_STORE/inbox/conversations",
    );
    assert.equal(reads[0].init?.method, "GET");
    assert.equal(reads[0].init?.credentials, "same-origin");
    assert.equal(reads[0].init?.cache, "no-store");
    assert.equal(reads[0].init?.headers, undefined);
  });
  for (const wrong of [401, 200])
    test(`actual INU02 ${name} still rejects wrong authority ${wrong}`, async (t) => {
      const { work } = await execute(t, wrong);
      await assert.rejects(work);
    });
  test(`actual INU02 ${name} still rejects DM in denied response`, async (t) => {
    const { work } = await execute(t, status, dm);
    await assert.rejects(work);
  });
}
