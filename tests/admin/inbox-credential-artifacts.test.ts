// Purpose: prove private fallback and matcher snapshot suppression without masking installed Playwright failures.
// Depends on: actual installed snapshot/worker serializer/builder bodies, actual fixture callbacks and Node/TypeScript API.
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
function workerDefinition(name = "_inboxPrivateEvidence") {
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
  const registration = declaration?.properties.find((node: any) => node.name?.getText(ast) === name)?.initializer;
  if (!registration && name !== "_inboxPrivateEvidence")
    return {
      scope: "absent",
      auto: false,
      run: async (_args: object, use: () => Promise<void>, _info: object) => use(),
    };
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

// Exercise the installed worker serializer (including its matcher-specific path), not an invented diagnostic DTO.
const workerPath = resolve(dirname(pwPackage), "lib/worker/workerProcessEntry.js");
const workerSource = readFileSync(workerPath, "utf8");
const workerAST = ts.createSourceFile(workerPath, workerSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
const installedFunctions = ["filterStackTrace", "serializeError", "testInfoError"].map((name) => {
  const declaration = workerAST.statements.find(
    (node: any) => ts.isFunctionDeclaration(node) && node.name?.text === name,
  );
  assert.ok(declaration?.body, `actual installed ${name} function required`);
  return declaration.getText(workerAST);
});
const workerRequire = createRequire(workerPath);
const serializeMatcher = new Function(
  "require",
  `
  const {stringifyStackFrames,filteredStackTrace} = require("playwright-core/lib/coreBundle").utils;
  const import_util = {default: require("node:util")};
  ${installedFunctions.join("\n")}
  return testInfoError;
`,
)(workerRequire);
const { buildErrorContext } = require(resolve(dirname(pwPackage), "lib/errorContext.js"));
const matcherSentinel = "SYNTHETIC_NONCREDENTIAL_MATCHER_SNAPSHOT";
function matcherError() {
  const error = new Error("MOCK_LOCATOR_ASSERTION_FAILED");
  (error as any).matcherResult = { ariaSnapshot: `- paragraph: ${matcherSentinel}` };
  return serializeMatcher(error);
}
function matcherContext(errors: object[]) {
  return buildErrorContext({
    titlePath: ["MOCK private diagnostic"],
    location: { file: "/mock/no-test-source.ts", line: 1, column: 1 },
    errors,
  });
}
test("ordinary matcher ARIA context bypasses fallback flag and remains in actual installed diagnostics", (t) => {
  environment(t, "1");
  const error = matcherError();
  assert.equal(error.errorContext.includes(matcherSentinel), true);
  assert.equal(matcherContext([error]).includes(matcherSentinel), true);
});
for (const reject of [false, true]) {
  test(`private test teardown removes only matcher page context while preserving failure (${reject ? "rejected use" : "failed status"})`, async (t) => {
    environment(t);
    const fixture = workerDefinition("_inboxPrivateMatcherEvidence");
    const serialized = matcherError(),
      plain = { message: "MOCK_SECOND_FAILURE", stack: "MOCK_SECOND_STACK" };
    const errors = [serialized, plain],
      before = errors.map(({ errorContext, ...rest }: any) => rest);
    const info = { errors, status: "failed", expectedStatus: "passed" };
    const failure = new Error("MOCK_UNMASKED_USE_FAILURE");
    const work = fixture.run(
      {},
      async () => {
        assert.equal(
          matcherContext(errors).includes(matcherSentinel),
          true,
          "control proves installed serializer carries snapshot before teardown",
        );
        if (reject) throw failure;
      },
      info,
    );
    if (reject) await assert.rejects(work, (cause: unknown) => cause === failure);
    else await work;
    assert.equal(
      matcherContext(errors).includes(matcherSentinel),
      false,
      "private matcher snapshot must not enter error-context.md",
    );
    assert.equal(info.errors, errors);
    assert.equal(errors.length, 2);
    assert.equal(errors[0], serialized);
    assert.equal(errors[1], plain);
    assert.deepEqual(errors, before, "only errorContext is removed; message/stack/cause are unchanged");
    assert.equal(info.status, "failed");
    assert.equal(info.expectedStatus, "passed");
    assert.equal(matcherContext(errors).includes("MOCK_LOCATOR_ASSERTION_FAILED"), true);
    assert.equal(matcherContext(errors).includes("MOCK_SECOND_FAILURE"), true);
  });
}
test("matcher guard is test-auto without replacing the installed artifact auto fixture", () => {
  const fixture = workerDefinition("_inboxPrivateMatcherEvidence");
  assert.equal(fixture.scope, '\"test\"');
  assert.equal(fixture.auto, true);
  const source = readFileSync(fixturePath, "utf8");
  assert.equal(source.includes("_setupArtifacts:"), false, "installed artifact lifecycle remains intact");
});
