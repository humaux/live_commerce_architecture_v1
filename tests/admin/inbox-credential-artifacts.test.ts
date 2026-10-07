// Purpose: independently prove the sensitive worker suppresses installed Playwright ARIA capture and restores diagnostics.
// Depends on: actual installed ArtifactsRecorder._takePageSnapshot body, actual worker fixture callback and Node/TypeScript API.
// Used by: Node privacy gate; fake page/use drive real functions, no browser/server/Go/PG starts here.
// Invariants: I11 credential-bearing DOM never enters a sensitive-worker snapshot; I18 browser artifacts remain NOT_RUN.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url),
  ts = require("typescript-api");
const key = "PLAYWRIGHT_NO_COPY_PROMPT";
const fixturePath = new URL("./inbox-private-evidence.ts", import.meta.url);
const pwPackage = createRequire(require.resolve("@playwright/test")).resolve("playwright/package.json");
const installedPath = resolve(dirname(pwPackage), "lib/index.js");
const installedSource = readFileSync(installedPath, "utf8");
const installedAST = ts.createSourceFile(
  installedPath,
  installedSource,
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.JS,
);
const recorder = installedAST.statements.find(
  (node: any) => ts.isClassDeclaration(node) && node.name?.text === "ArtifactsRecorder",
);
const method = recorder?.members.find((node: any) => node.name?.getText(installedAST) === "_takePageSnapshot");
assert.ok(method?.body, "actual installed Playwright snapshot method must be available for the control");
const takeSnapshot = new Function(
  "debugLogger",
  `return async function(context) ${method.body.getText(installedAST)};`,
)({ log() {} });
function workerDefinition() {
  if (!existsSync(fixturePath)) {
    // Baseline has no private fixture: the actual ordinary recorder receives the sensitive page too.
    return { scope: "absent", auto: false, run: async (_args: object, use: () => Promise<void>) => use() };
  }
  const text = readFileSync(fixturePath, "utf8"),
    ast = ts.createSourceFile("fixture.ts", text, ts.ScriptTarget.Latest, true);
  let declaration: any;
  const visit = (node: any) => {
    if (ts.isCallExpression(node) && node.expression.getText(ast) === "base.extend") declaration = node.arguments[0];
    ts.forEachChild(node, visit);
  };
  visit(ast);
  const registration = declaration?.properties.find(
    (node: any) => node.name?.getText(ast) === "_inboxPrivateEvidence",
  )?.initializer;
  assert.ok(
    registration && ts.isArrayLiteralExpression(registration),
    "actual exported fixture must register the worker guard",
  );
  const [callback, options] = registration.elements;
  const option = (name: string) =>
    options.properties.find((node: any) => node.name?.getText(ast) === name)?.initializer.getText(ast);
  const code = ts.transpileModule(`const run = ${callback.getText(ast)};`, {
    compilerOptions: { target: ts.ScriptTarget.ES2023 },
  }).outputText;
  return { scope: option("scope"), auto: option("auto") === "true", run: new Function(`${code};return run;`)() };
}
function environment(t: any, value?: string) {
  const previous = process.env[key];
  if (value === undefined) delete process.env[key];
  else process.env[key] = value;
  t.after(() => {
    if (previous === undefined) delete process.env[key];
    else process.env[key] = previous;
  });
}
async function snapshot() {
  let reads = 0;
  const state: any = { _testInfo: { errors: [{ message: "MOCK_ASSERTION_FAILURE" }] } };
  const context = {
    pages: () => [
      {
        _wrapApiCall: async (work: () => Promise<void>) => work(),
        ariaSnapshot: async () => {
          reads++;
          return "SYNTHETIC_CREDENTIAL_IN_DOM";
        },
      },
    ],
  };
  await takeSnapshot.call(state, context);
  return {
    reads,
    hasCredential: state._pageSnapshot?.includes("SYNTHETIC_CREDENTIAL_IN_DOM") === true,
    errors: state._testInfo.errors,
  };
}

test("ordinary Playwright diagnostics still capture failed-page ARIA when private guard is absent", async (t) => {
  environment(t);
  const result = await snapshot();
  assert.equal(result.reads, 1);
  assert.equal(result.hasCredential, true);
});
test("sensitive worker lifecycle prevents actual installed snapshot capture without masking the failure", async (t) => {
  environment(t);
  const fixture = workerDefinition();
  await fixture.run({}, async () => {
    const result = await snapshot();
    assert.equal(result.reads, 0, "sensitive worker must not call the credential-bearing ARIA snapshot");
    assert.equal(result.hasCredential, false);
    assert.equal(result.errors.length, 1, "assertion failure remains observable");
  });
  assert.equal(process.env[key], undefined);
  const ordinary = await snapshot();
  assert.equal(ordinary.reads, 1, "normal diagnostics are restored after worker teardown");
});
for (const previous of [undefined, "", "existing-policy"]) {
  test(`worker restores prior environment even when use rejects (${previous === undefined ? "absent" : previous || "empty"})`, async (t) => {
    environment(t, previous);
    const fixture = workerDefinition(),
      failure = new Error("MOCK_UNMASKED_ASSERTION_FAILURE");
    await assert.rejects(
      fixture.run({}, async () => {
        assert.equal(process.env[key], "1", "guard is active only inside worker use");
        throw failure;
      }),
      (cause: unknown) => cause === failure,
      "fixture must propagate the original use failure",
    );
    assert.equal(process.env[key], previous, "restore exact prior value, including empty or absent");
  });
}
test("fixture is worker-auto and module collection does not suppress ordinary diagnostics", async (t) => {
  environment(t);
  const fixture = workerDefinition();
  assert.equal(fixture.scope, '"worker"');
  assert.equal(fixture.auto, true);
  await import("./inbox-private-evidence.ts");
  assert.equal(process.env[key], undefined, "import/collection must not change process environment");
  const ordinary = await snapshot();
  assert.equal(ordinary.reads, 1);
});
