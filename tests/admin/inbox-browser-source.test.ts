// Purpose: execute the actual inbox login helper against a pending auth/store navigation counterexample.
// Depends on: Node test/assert/fs, typescript-api; reads the real inbox-ui.spec.ts without running a browser.
// Used by: LC-U2b Node/static gate; MOCK Page covers navigation ordering only, browser acceptance remains CI.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
const ts = createRequire(import.meta.url)("typescript-api");
const spec = readFileSync("tests/admin/inbox-ui.spec.ts", "utf8");
const source = ts.createSourceFile("inbox-ui.spec.ts", spec, ts.ScriptTarget.Latest, true);
const origin = "https://mock-admin.invalid";
const fixture = "10000000-0000-4000-8000-000000000001";
const other = "20000000-0000-4000-8000-000000000002";

test("actual login settles authentication and the real fixture-store navigation before Messages", async () => {
  const helpers = source.statements.filter((n: any) => ts.isFunctionDeclaration(n) &&
    ["login", "selectFixtureStore"].includes(n.name?.text)).map((n: any) => n.getText(source)).join("\n");
  assert.ok(helpers.includes("async function login"), "exercise the real login declaration");
  let url = `${origin}/en/`, selected = other, authPending = false, switching: string | null = null;
  const events: string[] = [];
  const matches = (value: string | RegExp | ((url: URL) => boolean)) =>
    typeof value === "string" ? url === value : value instanceof RegExp ? value.test(url) : value(new URL(url));
  const page: any = {
    async goto(value: string) { url = value; events.push("goto"); },
    url: () => url,
    async waitForURL(value: any) {
      if (authPending) { authPending = false; url = `${origin}/en?store=${other}`; selected = other; events.push("auth settled"); }
      else if (switching) { selected = switching; switching = null; url = `${origin}/en?store=${selected}`; events.push("store settled"); }
      assert.equal(matches(value), true, "actual navigation destination must match the requested wait");
    },
    getByRole: () => ({ async click() { authPending = true; events.push("sign in"); } }),
    getByTestId(id: string) {
      return {
        id,
        async inputValue() { return selected; },
        async selectOption(value: string) { switching = value; events.push("real selectOption"); },
        async click() {
          if (id === "nav-group-messages") {
            assert.equal(authPending, false, "do not use a pre-authentication navigation snapshot");
            assert.equal(switching, null, "await the full store navigation before entering Messages");
            assert.equal(selected, fixture, "enter Messages only in the exact fixture store");
            url = `${origin}/en/messages?store=${selected}`; events.push("Messages");
          }
        },
      };
    },
  };
  const expect = (target: any) => ({
    async toBeVisible() {},
    async toContainText() {},
    async toHaveCount() {},
    async toHaveValue(value: string) { assert.equal(selected, value, "selector must retain the fixture store"); },
    async toHaveURL(value: any) {
      if (target === page) await page.waitForURL(value);
      else throw new Error("MOCK expected a page URL assertion");
    },
  });
  const code = ts.transpileModule(`${helpers}\nreturn login;`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText;
  const login = new Function("origin", "store", "ids", "sentinels", "expect", code)(
    origin, fixture, { open: "30000000-0000-4000-8000-000000000003" }, ["MOCK_DM", "MOCK_NAME"], expect,
  );
  await login(page);
  assert.equal(events.filter((event) => event === "real selectOption").length, 1);
  assert.ok(events.indexOf("auth settled") < events.indexOf("real selectOption"));
  assert.ok(events.indexOf("store settled") < events.indexOf("Messages"));
  assert.equal(url, `${origin}/en/messages?store=${fixture}`);
});
