// Purpose: #179 — every waitForResponse/waitForRequest (or historyRead wrapper) assigned to a variable in a browser gate
//   script must be awaited before the next await. A later await of a different expression can throw; the page then closes,
//   the abandoned wait rejects unhandled and node exits on "Target page ... has been closed", hiding the real first failure.
//   Injected counterexamples below prove the lexical scan is not vacuous and that both sanctioned fixes are accepted.
// Depends on: git ls-files (tracked gate scripts only) and ./gate-waits-source.mjs; no browser, no network.
// Used by: scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";
import { offenders } from "./gate-waits-source.mjs";

const root = new URL("../../", import.meta.url);
const gates = execFileSync("git", ["ls-files", "-z", "--", "tests/storefront", "tests/admin"], { cwd: root, encoding: "utf8" })
  .split("\0").filter(Boolean).filter((path) => path.endsWith(".mjs"))
  .map((path) => ({ path, text: readFileSync(new URL(path, root), "utf8") }));

test("browser gate scripts await every started wait before the next await", () => {
  assert.ok(gates.some(({ path }) => path === "tests/storefront/order-gate.mjs"), "scan must see the order gate");
  assert.deepEqual(
    offenders(gates),
    [],
    "a waitForResponse/waitForRequest promise is awaited only after another await; use " +
      "`const [reply] = await Promise.all([pending, trigger()])` so the real first failure surfaces (#179)",
  );
});

const caseOf = (source) => offenders([{ path: "tests/storefront/injected.mjs", text: source }]);

test("the scan rejects the raw idiom", () => {
  assert.equal(caseOf('const pending=p.waitForResponse(r=>r.ok());await p.getByTestId("go").click();const reply=await pending;').length, 1);
  assert.equal(caseOf('const historyUI=historyRead(a);await a.getByTestId("toggle").click();const reply=await historyUI;').length, 1);
  assert.equal(caseOf('const post=card.page().waitForResponse(r=>r.ok());await card.click();const reply=await post;').length, 1);
});

test("the scan is not fooled by an await inside a later callback", () => {
  const source = [
    'const written=page.waitForResponse(r=>r.ok());',
    'const later=Promise.resolve().then(async r=>{const body=await r.json();return body;});',
    'await page.getByTestId("send").click();',
    'const write=await written;',
  ].join("\n");
  assert.equal(caseOf(source).length, 1);
});

test("the scan rejects a wait that is never awaited", () => {
  assert.equal(caseOf('const pending=p.waitForResponse(r=>r.ok());return pending;').length, 1);
});

test("both sanctioned fixes are accepted", () => {
  assert.deepEqual(caseOf('const pending=p.waitForResponse(r=>r.ok());const [reply]=await Promise.all([pending,p.getByTestId("go").click()]);'), []);
  assert.deepEqual(caseOf('const pending=p.waitForResponse(r=>r.ok());const reply=await pending;'), []);
  assert.deepEqual(caseOf('const q=()=>page.waitForResponse(r=>r.ok());const reply=await q();'), []);
});
