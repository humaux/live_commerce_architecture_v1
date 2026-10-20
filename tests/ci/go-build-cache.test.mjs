// Purpose: guard native compiler-cache reuse without caching gate outcomes or reducing test execution: only the shard:gNN legs
//   (one identical ./tests/foundation -race build graph) may restore/save the immutable setup-go key, and the foundation command keeps
//   -race, -count=1 and the exact -run/-skip selector semantics (review-735cb2ce2 P2-1, P2-3).
// Depends on: pinned gates.yml setup-go configuration and the foundation mode's real invocation.
//   Also pins that every cache-owning shard leg runs LC_FOUNDATION_PKGS=./tests/foundation exactly once (gates.yml gate step, every shard:
//   case arm at any indentation; #86, PR #81 review P2-2; #107).
// Used by: test-node.sh on GitHub; injected regressions exercise the same assertions as the real files.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const workflow = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
const runner = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");

function cacheContract(source) {
  // Only the matrix job owns this optimization; the lightweight plan job cannot seed a partial cache.
  const gate = source.split(/^  gate:\s*$/m)[1]?.split(/^  aggregate:\s*$/m)[0] ?? "";
  const setup = gate.match(/      - uses: actions\/setup-go@[^\n]+\n([\s\S]*?)(?=      - (?:name|uses):)/)?.[1] ?? "";
  assert.match(gate, /actions\/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16\b/, "keep the existing pinned action");
  assert.match(setup, /^          go-version-file: go\.mod$/m, "exact Go version stays in go.mod");
  // A non-Go or different-graph writer (ui-gallery, shard:unit) would seed the shared immutable key with a partial cache.
  assert.match(setup, /^          cache: \$\{\{ startsWith\(matrix\.mode, 'shard:g'\) \}\}$/m, "only the shard:gNN legs may own the Go compiler cache");
  const deps = setup.match(/^          cache-dependency-path: \|\n((?:            \S+\n)+)/m)?.[1].trim().split(/\s+/) ?? [];
  assert.deepEqual(deps, ["go.mod", "go.sum"], "cache identity must include the verified module manifest");
  assert.match(gate, /GOFLAGS: -mod=readonly/, "module manifests stay read-only");
}

// #86 (PR #81 review P2-2): the cache above is only safe while every cache-owning leg builds the same ./tests/foundation graph.
function shardScopeContract(source) {
  const gate = source.split(/^  gate:\s*$/m)[1]?.split(/^  aggregate:\s*$/m)[0] ?? "";
  // #107: every case arm with a shard: alternative, at any indentation, quoted or in "(pattern)" form; its body runs to ";;".
  const lines = gate.split("\n");
  const arms = [];
  lines.forEach((line, i) => {
    const m = line.trimStart().match(/^\(?([^#)\s][^)\s]*)\)(.*)$/);
    const alternatives = m?.[1].replace(/["']/g, "").split("|") ?? [];
    if (!alternatives.some(a => a.startsWith("shard:"))) return;
    let body = m[2];
    for (let j = i + 1; !body.includes(";;") && j < lines.length; j++) body += `\n${lines[j]}`;
    arms.push([alternatives, body.split(";;")[0].trim()]);
  });
  assert.ok(arms.some(([alternatives]) => alternatives.includes("shard:g[0-9][0-9]")), "the shard:gNN arm exists");
  for (const [alternatives, body] of arms) {
    if (alternatives.every(a => a === "shard:unit")) continue; // 'shard:unit' never satisfies startsWith(matrix.mode, 'shard:g')
    const message = `${alternatives.join("|")} legs share the cache and must build only ./tests/foundation`;
    assert.match(body, /^LC_FOUNDATION_PKGS=\.\/tests\/foundation (?!\S*\.\/)/, message);
    // The shell keeps the last assignment, so a second one anywhere in the arm would override the first (#107).
    assert.equal(body.split("LC_FOUNDATION_PKGS=").length - 1, 1, message);
  }
}

function executionContract(source) {
  const foundation = source.split(/^  foundation\)\s*$/m)[1]?.split(/^    ;;$/m)[0] ?? "";
  const command = foundation.split("\n").find(line => /^  GOTOOLCHAIN=.* go test /.test(line)) ?? "";
  for (const flag of ["-race", "-count=1", "-timeout=4500s"]) assert.ok(command.split(/\s+/).includes(flag), `keep ${flag}`);
  // Exact expansions, not names: a swapped -run/-skip or a dropped flag silently changes which tests a shard executes.
  for (const selector of ['${LC_FOUNDATION_RUN:+-run "$LC_FOUNDATION_RUN"}', '${LC_FOUNDATION_SKIP:+-skip "$LC_FOUNDATION_SKIP"}', "${LC_FOUNDATION_PKGS:-./...}"]) {
    assert.ok(command.includes(selector), `keep ${selector}`);
  }
  assert.match(foundation, /go vet \.\/\.\.\./, "whole-repository vet remains enforced by the unit/full run");
}

test("matrix reuses native Go compilation while every selected foundation test still executes", () => {
  cacheContract(workflow);
  shardScopeContract(workflow);
  executionContract(runner);
});

test("injected gNN leg outside ./tests/foundation goes RED under the shard scope contract", () => {
  const gnn = "shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation ";
  assert.ok(workflow.includes(gnn), "the real gNN arm is where the contract looks");
  for (const to of ["shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./... ", "shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation ./cmd/... ",
    "shard:g[0-9][0-9]) "]) {
    assert.throws(() => shardScopeContract(workflow.replace(gnn, to)), /must build only \.\/tests\/foundation/, to);
  }
  assert.throws(() => shardScopeContract(workflow.replace(gnn, "shard:gx) LC_FOUNDATION_PKGS=./tests/foundation ")), /shard:gNN arm/);
  // A catch-all shard:* arm also receives modes like shard:gFoo, which satisfy startsWith(matrix.mode, 'shard:g').
  const any = 'shard:*)    LC_FOUNDATION_PKGS=./tests/foundation ';
  assert.ok(workflow.includes(any));
  assert.throws(() => shardScopeContract(workflow.replace(any, "shard:*)    LC_FOUNDATION_PKGS=./... ")), /must build only/);
});

// #107 (PR #101 review P2): the arm finder must not depend on one indentation or an unquoted pattern, and a later assignment on
// the same arm (the shell keeps the last one) must not hide behind a correct first one.
test("injected shadowing assignment, re-indented or quoted arm goes RED under the shard scope contract", () => {
  const gnn = "            shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation ";
  const any = "            shard:*)    LC_FOUNDATION_PKGS=./tests/foundation ";
  assert.ok(workflow.includes(gnn) && workflow.includes(any), "the real arms are where the contract looks");
  for (const [from, to] of [
    [gnn, `${gnn}LC_FOUNDATION_PKGS=./... `],
    [any, `${any}LC_FOUNDATION_RUN=x LC_FOUNDATION_PKGS=./... `],
    [gnn, `${gnn}\\\n              LC_FOUNDATION_PKGS=./... `],
    [any, "          shard:*)    LC_FOUNDATION_PKGS=./... "],
    [any, "\t\t\tshard:*)    LC_FOUNDATION_PKGS=./... "],
    [any, '            "shard:"*)    LC_FOUNDATION_PKGS=./... '],
    [any, "            'shard:*')    LC_FOUNDATION_PKGS=./... "],
    [any, "            (shard:*)    LC_FOUNDATION_PKGS=./... "],
    [any, "            shard:x|shard:*)    LC_FOUNDATION_PKGS=./... "],
  ]) {
    assert.throws(() => shardScopeContract(workflow.replace(from, to)), /must build only \.\/tests\/foundation/, JSON.stringify(to));
  }
});

// #116 (PR #108 review P2-1..P2-3): spaces around "|", "+=" appends and a broader non-shard glob placed before the shard arms.
test("injected spaced alternatives, += append or shadowing glob goes RED under the shard scope contract", () => {
  const gnn = "            shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation ";
  const any = "            shard:*)    LC_FOUNDATION_PKGS=./tests/foundation ";
  assert.ok(workflow.includes(gnn) && workflow.includes(any), "the real arms are where the contract looks");
  for (const [from, to, expected] of [
    [any, "            shard:x | shard:*)    LC_FOUNDATION_PKGS=./... ", /must build only/],
    [any, "            shard:x |shard:*)    LC_FOUNDATION_PKGS=./... ", /must build only/],
    [any, "            ( shard:x  |  shard:*)    LC_FOUNDATION_PKGS=./... ", /must build only/],
    [any, `${any}LC_FOUNDATION_PKGS+=" ./..." `, /must build only/],
    [gnn, `${gnn}LC_FOUNDATION_PKGS+=./... `, /must build only/],
    [gnn, "            *) echo shadowed ;;\n" + gnn, /shadow/],
    [gnn, "            shard*) echo shadowed ;;\n" + gnn, /shadow/],
    [gnn, "            s*|x) echo shadowed ;;\n" + gnn, /shadow/],
    [gnn, "            *:g*) echo shadowed ;;\n" + gnn, /shadow/],
    [any, "            *)  echo shadowed ;;\n" + any, /shadow/],
  ]) {
    assert.throws(() => shardScopeContract(workflow.replace(from, to)), expected, JSON.stringify(to));
  }
});

test("injected disabled cache or unbound manifest goes RED under the real cache contract", () => {
  const cacheLine = "cache: ${{ startsWith(matrix.mode, 'shard:g') }}";
  assert.ok(workflow.includes(cacheLine));
  for (const to of ["cache: true", "cache: false", "cache: ${{ startsWith(matrix.mode, 'shard:') }}"]) {
    assert.throws(() => cacheContract(workflow.replace(cacheLine, to)), /only the shard:gNN legs/);
  }
  assert.throws(() => cacheContract(workflow.replace("            go.mod\n            go.sum\n", "            go.mod\n")), /verified module manifest/);
  assert.throws(() => cacheContract(workflow.replace("            go.mod\n            go.sum\n", "            go.sum\n")), /verified module manifest/);
});

test("injected result reuse or lost race/selector goes RED, rather than manufacturing cheaper gates", () => {
  for (const [from, to] of [["-count=1", "-count=0"], ["-race", ""], ["LC_FOUNDATION_SKIP", "REMOVED_SKIP"],
    ['-skip "$LC_FOUNDATION_SKIP"', '-run "$LC_FOUNDATION_SKIP"'], [':+-skip ', ':+'], [':+-run ', ':+-skip '], ["${LC_FOUNDATION_PKGS:-./...}", "./..."]]) {
    const start = runner.indexOf("  foundation)");
    // Shell selectors occur in both the conditional expansion and its value; remove both.
    const changed = runner.slice(0, start) + runner.slice(start).replace(/^  GOTOOLCHAIN=.* go test .*$/m, line => line.replaceAll(from, to));
    assert.notEqual(changed, runner, `inject ${from} into the real command`);
    assert.throws(() => executionContract(changed), /keep /);
  }
});
