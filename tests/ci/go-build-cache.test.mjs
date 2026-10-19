// Purpose: guard native compiler-cache reuse without caching gate outcomes or reducing test execution: only the shard:gNN legs
//   (one identical ./tests/foundation -race build graph) may restore/save the immutable setup-go key, and the foundation command keeps
//   -race, -count=1 and the exact -run/-skip selector semantics (review-735cb2ce2 P2-1, P2-3).
// Depends on: pinned gates.yml setup-go configuration and the foundation mode's real invocation.
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
  executionContract(runner);
});

test("injected disabled cache or unbound manifest goes RED under the real cache contract", () => {
  const cacheLine = "cache: ${{ startsWith(matrix.mode, 'shard:g') }}";
  for (const to of ["cache: true", "cache: false", "cache: ${{ startsWith(matrix.mode, 'shard:') }}"]) {
    assert.ok(workflow.includes(cacheLine));
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
