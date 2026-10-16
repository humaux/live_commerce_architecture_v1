// Purpose: guard native compiler-cache reuse without caching gate outcomes or reducing test execution.
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
  assert.match(setup, /^          cache: true$/m, "matrix must reuse Go compiler/module output");
  assert.match(setup, /^          cache-dependency-path: go\.sum$/m, "cache identity must include the verified module manifest");
  assert.match(gate, /GOFLAGS: -mod=readonly/, "module manifests stay read-only");
}

function executionContract(source) {
  const foundation = source.split(/^  foundation\)\s*$/m)[1]?.split(/^    ;;$/m)[0] ?? "";
  const command = foundation.split("\n").find(line => /^  GOTOOLCHAIN=.* go test /.test(line)) ?? "";
  for (const flag of ["-race", "-count=1", "-timeout=4500s"]) assert.ok(command.split(/\s+/).includes(flag), `keep ${flag}`);
  for (const selector of ["LC_FOUNDATION_RUN", "LC_FOUNDATION_SKIP", "LC_FOUNDATION_PKGS"]) assert.ok(command.includes(selector), `keep ${selector}`);
  assert.match(foundation, /go vet \.\/\.\.\./, "whole-repository vet remains enforced by the unit/full run");
}

test("matrix reuses native Go compilation while every selected foundation test still executes", () => {
  cacheContract(workflow);
  executionContract(runner);
});

test("injected disabled cache or unbound manifest goes RED under the real cache contract", () => {
  assert.throws(() => cacheContract(workflow.replace("cache: true", "cache: false")), /matrix must reuse/);
  assert.throws(() => cacheContract(workflow.replace("cache-dependency-path: go.sum", "cache-dependency-path: go.mod")), /verified module manifest/);
});

test("injected result reuse or lost race/selector goes RED, rather than manufacturing cheaper gates", () => {
  for (const [from, to] of [["-count=1", "-count=0"], ["-race", ""], ["LC_FOUNDATION_SKIP", "REMOVED_SKIP"]]) {
    const start = runner.indexOf("  foundation)");
    const changed = runner.slice(0, start) + runner.slice(start).replace(/^  GOTOOLCHAIN=.* go test .*$/m, line => line.replace(from, to));
    assert.throws(() => executionContract(changed), /keep /);
  }
});
