// Purpose: Reject removal or incidental references of the actual matrix gate execution entries.
// Depends on: the real runner/workflow/registry bytes and matrixExecutionIssues; no subprocess, PG or browser.
// Used by: test-node.sh; isolated string mutations cannot alter a shared runner.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { matrixExecutionIssues } from "./product-editor-matrix-execution.mjs";

const source = { node: readFileSync("scripts/dev/test-node.sh", "utf8"),
  workflow: readFileSync(".github/workflows/gates.yml", "utf8"), registry: readFileSync("scripts/dev/test-local.sh", "utf8") };
// The parent gate must first register this test; fixtures then use actual argv and registry contents.
test("matrix gates are executed through the actual current commands", () => assert.deepEqual(matrixExecutionIssues(source), []));
for (const name of ["product-editor-matrix-ledger.test.mjs", "product-editor-matrix-execution.test.mjs"]) {
  test(`incidental comment cannot replace the active Node ${name} argv`, () => {
    const token = ` tests/ci/${name}`;
    assert.ok(source.node.includes(token), "real active mutation anchor");
    const node = source.node.replace(token, "") + `\n# node --test tests/ci/${name}\n`;
    assert.ok(matrixExecutionIssues({ ...source, node }).some((x) => x.includes(name)));
  });
}
test("CI cannot append an evidence-test skip to the actual balanced dispatch", () => {
  const token = 'LC_FOUNDATION_SKIP="$(node scripts/dev/shard-plan.mjs skip "${MODE#shard:}")"';
  assert.ok(source.workflow.includes(token));
  const workflow = source.workflow.replace(token, token.slice(0, -1) + '|^TestProductEditorMatrixLedgerEvidence$"');
  assert.ok(matrixExecutionIssues({ ...source, workflow }).some((x) => x.includes("dispatch")));
});
for (const selector of ["RUN", "SKIP"]) {
  test(`foundation go test must forward the actual ${selector} selector`, () => {
    const token = '${LC_FOUNDATION_' + selector + ':+-' + selector.toLowerCase() + ' "$LC_FOUNDATION_' + selector + '"}';
    assert.ok(source.registry.includes(token));
    const registry = source.registry.replace(token, "");
    assert.ok(matrixExecutionIssues({ ...source, registry }).some((x) => x.includes("forward")));
  });
}

const nodeCommand = source.node.split("\n").find((line) => line.startsWith("node --test ") && line.includes("tests/ci/product-editor-matrix-ledger.test.mjs"));
assert.ok(nodeCommand, "real Node command mutation anchor");
for (const flag of ["--test-name-pattern", "--test-skip-pattern"]) {
  for (const filter of [`${flag} '^$'`, `${flag}='.*'`, `'${flag}=.*'`]) {
    test(`required Node suites cannot retain filenames but add ${filter}`, () => {
      const node = source.node.replace(nodeCommand, () => `${nodeCommand} ${filter}`);
      assert.ok(node.includes(`${nodeCommand} ${filter}`), "exact filter appended to real command");
      for (const name of ["ledger", "execution", "controls"])
        assert.ok(node.includes(`tests/ci/product-editor-matrix-${name}.test.mjs`), "suite filename retained");
      assert.ok(matrixExecutionIssues({ ...source, node }).some((issue) => issue.includes("filters")));
    });
  }
}
test("filters on a different Node command do not replace or filter the required suites", () => {
  const node = source.node + "\nnode --test --test-name-pattern='^$' tests/ci/shard-plan.test.mjs\n";
  assert.deepEqual(matrixExecutionIssues({ ...source, node }), []);
});

const goCommand = source.registry.split("\n").find((line) => line.includes("go test ") && line.includes('${LC_FOUNDATION_PKGS:-./...}'));
assert.ok(goCommand, "real foundation command mutation anchor");
for (const [flag, pattern] of [["-run", "^$"], ["-skip", "^TestProductEditorMatrix"]]) {
  for (const filter of [`${flag} '${pattern}'`, `${flag}=${pattern}`, `${flag}='${pattern}'`, `'${flag}=${pattern}'`]) {
    test(`foundation command cannot retain forwarding but add ${filter}`, () => {
      const registry = source.registry.replace(goCommand, () => `${goCommand} ${filter}`);
      assert.ok(registry.includes(`${goCommand} ${filter}`), "exact filter appended to real command");
      assert.ok(matrixExecutionIssues({ ...source, registry }).some((issue) => issue.includes("forward")));
    });
  }
}
