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

test("the scan rejects a wait awaited before its trigger (await-wait-then-trigger)", () => {
  // `await wait(...)` in one expression means the click is either still ahead (the await hangs) or already past
  // (the reply was missed); both are the ordering defect this rule exists to stop.
  assert.equal(caseOf('const reply=await p.waitForResponse(r=>r.ok());await p.getByTestId("go").click();').length, 1);
  assert.equal(caseOf('await p.waitForResponse(r=>r.ok());await p.getByTestId("go").click();').length, 1);
  assert.equal(caseOf('const history=await historyRead(p);await p.getByTestId("toggle").click();').length, 1);
});

test("the scan rejects an inline wait created after the trigger inside Promise.all", () => {
  // A wait created inside the array after the click registers its listener only after the action dispatches (#181 P2).
  assert.equal(caseOf('const [reply]=await Promise.all([p.getByTestId("go").click(),p.waitForResponse(r=>r.ok())]);').length, 1);
  // A wait assigned BEFORE the aggregate is already listening; its array position is only lexical, so listing the
  // pre-registered `pending` after the trigger is safe (#181 P2 witness from the review).
  assert.deepEqual(caseOf('const pending=p.waitForResponse(r=>r.ok());const [reply]=await Promise.all([p.getByTestId("go").click(),pending]);'), []);
  // The wait-first form stays green (it is what the gate scripts use).
  assert.deepEqual(caseOf('const pending=p.waitForResponse(r=>r.ok());const [reply]=await Promise.all([pending,p.getByTestId("go").click()]);'), []);
});

test("the scan rejects awaiting a fixture-held reply before releasing the hold", () => {
  // arm(..., {after: true}) withholds the reply until hook.release resolves. A wait declared with that hook and awaited
  // in an aggregate that never drives the hold can only time out, even with the wait listed first.
  const waitFirst = [
    'const historyLoading=arm("orders?limit=20",{method:"GET",after:true}),historyUI=historyRead(a);',
    'const [reply]=await Promise.all([historyUI,a.getByTestId("toggle-order-history").click()]);',
    'historyLoading.release.resolve();',
  ].join("\n");
  assert.equal(caseOf(waitFirst).length, 1);
  // Listing the pre-registered wait after the trigger is safe by array order (#181 P2), but the hold is still never
  // released from the aggregate, so rule 3 alone rejects it.
  const triggerFirst = [
    'const historyLoading=arm("orders?limit=20",{method:"GET",after:true}),historyUI=historyRead(a);',
    'const [reply]=await Promise.all([a.getByTestId("toggle-order-history").click(),historyUI]);',
    'historyLoading.release.resolve();',
  ].join("\n");
  assert.equal(caseOf(triggerFirst).length, 1);
  // #181 P2 witness verbatim from the review: naming only `entered` inside the aggregate does not release; the
  // release runs after the statement, so the await deterministically waits for a reply that is still held.
  const enteredOnly = 'const held=arm("orders?limit=20",{method:"GET",after:true}),pending=historyRead(a); await Promise.all([pending,a.getByTestId("toggle-order-history").click(),held.entered.promise]); held.release.resolve();';
  assert.equal(caseOf(enteredOnly).length, 1);
  // Driving the hold from inside the awaited aggregate (status asserted while held) stays green.
  const released = [
    'const historyLoading=arm("orders?limit=20",{method:"GET",after:true}),historyUI=historyRead(a);',
    'const [reply]=await Promise.all([historyUI,a.getByTestId("toggle-order-history").click(),historyLoading.entered.promise.then(()=>historyLoading.release.resolve())]);',
  ].join("\n");
  assert.deepEqual(caseOf(released), []);
});
