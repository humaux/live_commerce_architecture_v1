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
function visit(node: any) {
  if (
    ts.isFunctionDeclaration(node) &&
    ["isLocalHostname", "domainMatches", "pathMatches", "cookieMatchesURL", "filterCookies"].includes(node.name?.text)
  )
    functions.set(node.name.text, node.getText(core));
  ts.forEachChild(node, visit);
}
visit(core);
// Playwright v1.65 moves cookie selection to shared network functions; use their actual installed bodies.
assert.equal(functions.size, 5, "actual installed cookie selection functions required");
const installed = new Function(
  `${[...functions.values()].join("\n")}\nreturn {filterCookies,matches:cookieMatchesURL};`,
)();
const cookie = { name: "__Host-commerce_session", value: "MOCK_ONLY", domain: "127.0.0.1", path: "/", secure: true };
test("installed cookie rules trust HTTP loopback but refuse matched ordinary HTTP Secure cookies", () => {
  for (const domain of ["127.0.0.1", "localhost", "[::1]", "192.0.2.1"]) {
    // Documentation-only nonloopback address: this is actual-source MODEL evidence, no network request.
    const scoped = { ...cookie, domain };
    for (const protocol of ["http:", "https:"]) {
      const url = new URL(`${protocol}//${domain}:1234/api/stores/mock/inbox`);
      const expected = protocol === "https:" || domain !== "192.0.2.1";
      assert.equal(installed.matches(scoped, url), expected, "same-domain Secure transport classification");
      assert.equal(installed.filterCookies([scoped], [url.href]).length, expected ? 1 : 0);
    }
  }
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
      reads: { path: string; init?: RequestInit }[] = [],
      browserReads: string[] = [], requestReads: string[] = [];
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
          requestReads.push(path);
          const carriesCookie = installed.matches(cookie, new URL(path, "http://127.0.0.1:1234"));
          return { status: () => (carriesCookie ? browserStatus : 401), text: async () => body };
        },
      },
      evaluate: async (fn: any, path: string) => { browserReads.push(path); return fn(path); },
    };
    return { work: probe(name)(page, "MOCK_STORE", { foreign: "MOCK_FOREIGN" }, dm, expect)(), reads, browserReads, requestReads };
  }
  test(`actual INU02 ${name} uses browser auth and retains exact ${status}/DM absence`, async (t) => {
    const { work, reads, browserReads, requestReads } = await execute(t, status);
    await work;
    // Transport choice stays mandatory even when APIRequestContext also authenticates loopback.
    assert.equal(requestReads.length, 0, "INU02 must not use page.request transport");
    assert.equal(browserReads.length, 1, "INU02 must execute its read in the browser");
    assert.equal(reads.length, 1, "browser helper must invoke real fetch source");
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
