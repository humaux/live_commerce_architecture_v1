// Purpose: Reject removal or incidental references of the actual matrix gate execution entries.
// Depends on: the real runner/workflow/registry bytes and matrixExecutionIssues; no subprocess, PG or browser.
// Used by: test-node.sh; isolated string mutations cannot alter a shared runner.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { matrixExecutionIssues } from "./product-editor-matrix-execution.mjs";

const source = { node: readFileSync("scripts/dev/test-node.sh", "utf8"),
  foundationWorkflow: readFileSync(".github/workflows/foundation.yml", "utf8"),
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

for (const flag of ["--run", "--skip", "-test.run", "-test.skip"]) {
  for (const filter of [`${flag} '^$'`, `${flag}=^$`, `${flag}='^$'`, `'${flag}=^$'`]) {
    test(`foundation refuses Go filter alias ${filter}`, () => {
      const registry = source.registry.replace(goCommand, () => `${goCommand} ${filter}`);
      assert.ok(registry.includes(`${goCommand} ${filter}`));
      assert.ok(matrixExecutionIssues({ ...source, registry }).some((issue) => issue.includes("forward")));
    });
  }
}
for (const prefix of ["", "FOO=1 ", 'NODE_OPTIONS="--trace-warnings" FOO=1 ']) {
  test(`unfiltered required Node command accepts literal env prefix ${prefix}`, () => {
    const node = source.node.replace(nodeCommand, () => `${prefix}${nodeCommand}`);
    assert.deepEqual(matrixExecutionIssues({ ...source, node }), []);
  });
  for (const flag of ["--test-only", "--test-shard", "--test-concurrency", "--test-name-pattern", "--test-skip-pattern"]) {
    const value = flag === "--test-shard" ? "1/2" : flag === "--test-concurrency" ? "1" : "true";
    for (const filter of [flag, `${flag} ${value}`, `${flag}=${value}`, `'${flag}=${value}'`]) {
      test(`required Node command refuses ${prefix}${filter}`, () => {
        const node = source.node.replace(nodeCommand, () => `${prefix}${nodeCommand} ${filter}`);
        assert.ok(node.includes(`${prefix}${nodeCommand} ${filter}`));
        assert.ok(matrixExecutionIssues({ ...source, node }).some((issue) => issue.includes("filters")));
      });
    }
  }
}
for (const flag of ["-run", "-skip", "--run", "--skip", "-test.run", "-test.skip"]) {
  for (const filter of [`${flag} '^$'`, `${flag}=^$`]) {
    test(`Go selector in literal GOFLAGS cannot suppress the foundation matrix: ${filter}`, () => {
      const prefix = `GOFLAGS='${filter}' `;
      const registry = source.registry.replace(goCommand, () => `${prefix}${goCommand}`);
      assert.ok(registry.includes(`${prefix}${goCommand}`));
      assert.ok(matrixExecutionIssues({ ...source, registry }).some((issue) => issue.includes("forward")));
    });
  }
}
test("ordinary literal GOFLAGS options remain accepted", () => {
  const registry = source.registry.replace(goCommand, () => `GOFLAGS='-mod=readonly' ${goCommand}`);
  assert.deepEqual(matrixExecutionIssues({ ...source, registry }), []);
});
test("foundation workflow rejects a Go selector in its literal GOFLAGS environment", () => {
  const foundationWorkflow = source.foundationWorkflow.replace("      GOFLAGS: -mod=readonly", "      GOFLAGS: '-test.run=^$'");
  assert.ok(matrixExecutionIssues({ ...source, foundationWorkflow }).some((issue) => issue.includes("GOFLAGS")));
});
test("Node matrix command cannot hide failures with a trailing shell OR", () => {
  const node = source.node.replace(nodeCommand, () => `${nodeCommand} || true`);
  assert.ok(matrixExecutionIssues({ ...source, node }).some((issue) => issue.includes("missing")));
});
test("foundation Go command cannot hide failures with a trailing shell OR", () => {
  const registry = source.registry.replace(goCommand, () => `${goCommand} || true`);
  assert.ok(matrixExecutionIssues({ ...source, registry }).some((issue) => issue.includes("forward")));
});
const goCondition = "${{ !(github.event_name == 'pull_request' && github.base_ref == 'r3/integration') }}";
for (const runner of ["test-node", "test-local"]) {
  const line = `        run: bash scripts/dev/${runner}.sh`;
  assert.ok(source.foundationWorkflow.includes(line));
  for (const [kind, replacement] of [
    ["remove", `        name: retained scripts/dev/${runner}.sh`],
    ["comment", `        # run: bash scripts/dev/${runner}.sh`],
    ["wrong runner", `        run: echo scripts/dev/${runner}.sh`],
    ["disabled step", `        if: false\n${line}`],
    ["disabled expression", "        if: ${{ false }}\n" + line],
    ["unreachable block", `        run: |\n          exit 0\n          bash scripts/dev/${runner}.sh`],
  ]) {
    test(`foundation workflow refuses ${kind} ${runner} with filename retained`, () => {
      const foundationWorkflow = source.foundationWorkflow.replace(line, () => replacement)
        .replace(`        if: ${goCondition}\n        if:`, "        if:");
      assert.ok(foundationWorkflow.includes(`scripts/dev/${runner}.sh`));
      assert.ok(matrixExecutionIssues({ ...source, foundationWorkflow }).some((issue) => issue.includes("foundation.yml")));
    });
  }
  test(`foundation workflow accepts executable ${runner} run block`, () => {
    const foundationWorkflow = source.foundationWorkflow.replace(line, () => `        run: |\n          # actual runner\n          bash scripts/dev/${runner}.sh`);
    assert.deepEqual(matrixExecutionIssues({ ...source, foundationWorkflow }), []);
  });
  test(`foundation workflow refuses continue-on-error for ${runner}`, () => {
    const foundationWorkflow = source.foundationWorkflow.replace(line, () => `        continue-on-error: true\n${line}`);
    assert.ok(matrixExecutionIssues({ ...source, foundationWorkflow }).some((issue) => issue.includes("foundation.yml")));
  });
}
test("foundation job disabled despite both runner filenames retained", () => {
  const foundationWorkflow = source.foundationWorkflow.replace("  foundation:\n", "  foundation:\n    if: false\n");
  assert.ok(matrixExecutionIssues({ ...source, foundationWorkflow }).some((issue) => issue.includes("foundation.yml")));
});
test("foundation job cannot mark runner failures continue-on-error", () => {
  const foundationWorkflow = source.foundationWorkflow.replace("    runs-on: ubuntu-24.04\n", "    runs-on: ubuntu-24.04\n    continue-on-error: true\n");
  assert.ok(matrixExecutionIssues({ ...source, foundationWorkflow }).some((issue) => issue.includes("foundation.yml")));
});

test("foundation job condition after steps still disables the runners", () => {
  const foundationWorkflow = source.foundationWorkflow.replace("\n# Local scripts", "\n    if: false\n\n# Local scripts");
  assert.ok(matrixExecutionIssues({ ...source, foundationWorkflow }).some((issue) => issue.includes("foundation.yml")));
});
test("incidental go test text cannot replace the real foundation executable", () => {
  const registry = source.registry.replace(goCommand, () => goCommand.replace("go test ", "echo go test "));
  assert.ok(matrixExecutionIssues({ ...source, registry }).some((issue) => issue.includes("forward")));
});
