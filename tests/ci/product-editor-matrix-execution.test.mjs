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


// A duplicate across the two planned lanes is intentional; a duplicate inside a
// lane or a valid-looking command outside its actual Gate run is never evidence.
const laneNames = ["foundation_gate", "gate"];
function changeLane(name, change) {
  const header = `  ${name}:\n`, start = source.workflow.indexOf(header);
  assert.ok(start >= 0, `${name} exists`);
  const offset = source.workflow.slice(start + header.length).search(/^  [A-Za-z_][\w-]*:[ \t]*$/m);
  const end = offset < 0 ? source.workflow.length : start + header.length + offset;
  return source.workflow.slice(0, start) + change(source.workflow.slice(start, end)) + source.workflow.slice(end);
}
function rejectsDispatch(workflow) {
  assert.ok(matrixExecutionIssues({ ...source, workflow }).some((issue) => issue.includes("dispatch")));
}
for (const name of laneNames) {
  for (const selector of ["run", "skip"]) {
    test(`${name} independently rejects a changed planner ${selector} call`, () => {
      rejectsDispatch(changeLane(name, (body) => body.replace(`shard-plan.mjs ${selector} `, `shard-plan.mjs wrong-${selector} `)));
    });
  }
  for (const mutation of ["missing", "comment", "duplicate", "extra filter", "masked failure"]) {
    test(`${name} rejects ${mutation} dispatch even when the other lane is intact`, () => {
      rejectsDispatch(changeLane(name, (body) => {
        const line = body.split("\n").find((s) => s.trim().startsWith("shard:g[0-9][0-9]) "));
        assert.ok(line);
        const replacement = mutation === "missing" ? "" : mutation === "comment" ? `# ${line.trim()}`
          : mutation === "duplicate" ? `${line}\n${line}`
          : mutation === "extra filter" ? line.replace('2>&1 | tee', '-skip TestProductEditorMatrixLedgerEvidence 2>&1 | tee')
          : line.replace(' ;;', ' || true ;;');
        return body.replace(line, () => replacement);
      }));
    });
  }
  test(`${name} cannot move the dispatch into an unrelated step`, () => {
    rejectsDispatch(changeLane(name, (body) => {
      const line = body.split("\n").find((s) => s.trim().startsWith("shard:g[0-9][0-9]) "));
      return body.replace(line, "") + `      - name: Unrelated command\n        run: |\n${line}\n`;
    }));
  });
  test(`${name} must bind its own matrix output and runtime MODE`, () => {
    const output = name === "foundation_gate" ? "foundation_jobs" : "gate_jobs";
    rejectsDispatch(changeLane(name, (body) => body.replace(`fromJSON(needs.plan.outputs.${output})`, "fromJSON(needs.plan.outputs.jobs)")));
    rejectsDispatch(changeLane(name, (body) => body.replace("MODE: ${{ matrix.mode }}", "MODE: shard:g01")));
    rejectsDispatch(changeLane(name, (body) => body.replace("        id: gate\n", "        id: unused_gate\n")));
  });
  test(`${name} cannot disappear or be duplicated while global dispatch text remains`, () => {
    rejectsDispatch(changeLane(name, (body) => body.replace(`  ${name}:`, `  renamed_${name}:`)));
    rejectsDispatch(changeLane(name, (body) => body + body));
  });
}
test("an unplanned third lane cannot supply another valid-looking dispatch", () => {
  const line = source.workflow.split("\n").find((s) => s.trim().startsWith("shard:g[0-9][0-9]) "));
  rejectsDispatch(source.workflow + `\n  unplanned_gate:\n    steps:\n      - run: |\n${line}\n`);
});
for (const name of laneNames) {
  for (const position of ["before run", "after run"]) {
    test(`${name} rejects a step MODE override ${position} that would mislabel its receipt`, () => {
      rejectsDispatch(changeLane(name, (body) => {
        const override = "        env:\n          MODE: shard:g01\n";
        if (position === "before run") return body.replace("        id: gate\n", `        id: gate\n${override}`);
        return body.replace("      # MATRIX-RECEIPT", `${override}      # MATRIX-RECEIPT`);
      }));
    });
  }
  test(`${name} cannot replace job MODE with an incidental binding inside the Gate script`, () => {
    rejectsDispatch(changeLane(name, (body) => body.replace("      MODE: ${{ matrix.mode }}\n", "")
      .replace("        id: gate\n        run: |\n", "        id: gate\n        run: |\n          MODE: ${{ matrix.mode }}\n")));
  });
  test(`${name} rejects unreviewed step conditions and failure masking`, () => {
    for (const setting of ["if: false", "continue-on-error: true", "env: ${{ fromJSON('{}') }}"])
      rejectsDispatch(changeLane(name, (body) => body.replace("        id: gate\n", `        id: gate\n        ${setting}\n`)));
  });
}
