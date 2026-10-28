// Purpose: guard native compiler-cache reuse without caching gate outcomes or reducing test execution: only the shard:gNN legs
//   (one identical ./tests/foundation -race build graph) may restore/save the immutable setup-go key, and the foundation command keeps
//   -race, -count=1 and the exact -run/-skip selector semantics (review-735cb2ce2 P2-1, P2-3).
// Depends on: pinned gates.yml setup-go configuration and the foundation mode's real invocation.
//   Also pins that every cache-owning shard leg runs LC_FOUNDATION_PKGS=./tests/foundation exactly once (gates.yml gate step, every shard:
//   case arm at any indentation, spaced alternatives, no += append, no broader glob ahead of them; #86, PR #81 review P2-2; #107, #116).
//   The shadow check runs every earlier non-shard glob (with [!x] mapped to [^x] and a bash [^x] kept as a negation; #147) against every real shard mode from shard-plan.json via ci-plan expand,
//   each gNN mode must reach the shard:g[0-9][0-9] arm first, and an arm with $ = ; & or a backtick in an alternative fails closed (#116 residual of #137);
//   only a line without "|" or "shard:" before its first ")" that starts with VAR= is skipped as a plain command, and the enumerated mode set must contain a shard:gNN.
// Used by: test-node.sh on GitHub; injected regressions exercise the same assertions as the real files.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { expand } from "../../scripts/dev/ci-plan.mjs";

const workflow = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
// Every shard mode a real run enables (unit + each group of shard-plan.json), the same expansion the plan job uses for the matrix.
const shardPlan = JSON.parse(readFileSync(new URL("../../scripts/dev/shard-plan.json", import.meta.url), "utf8"));
const shardModes = expand(["foundation-shards"], { groupIds: shardPlan.groups.map(g => g.id) }).matrix.map(e => e.mode);
assert.ok(shardModes.length > 0 && shardModes.some(m => /^shard:g\d\d$/.test(m)), "shard-plan.json must enumerate at least one shard:gNN mode");
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
  // #116: spaces around "|" are fine; an arm needs whitespace-free alternatives, so command lines containing ")" are not arms.
  // #147 CI-3: exactly the second `case "$MODE" in` (the dispatch holding the shard arms) is scanned; a harmless `*) ;;` in the
  // evidence-root case before it, or a later case block, is not a shadow.
  const dispatch = gate.split(/case "\$MODE" in/)[2]?.split(/^\s*esac\b/m)[0] ?? "";
  const lines = dispatch.split("\n");
  const arms = [];
  lines.forEach((line, i) => {
    const m = line.trimStart().match(/^\(?\s*([^#)\s][^)]*)\)(.*)$/);
    const alternatives = m?.[1].replace(/["']/g, "").split("|").map(a => a.trim()) ?? [];
    // Whitespace inside an alternative or a leading VAR= means a command line, not an arm. Anything else with $ = ; & ` cannot be
    // evaluated as a glob here, so it fails closed instead of being skipped (review-137 P2).
    // A line is a plain command only when its text before the first ")" has no "|" and no "shard:" (e.g. LC_X=1 cmd); otherwise it is an arm.
    if (!m || !alternatives.every(a => /^\S+$/.test(a)) || (/^[A-Za-z_]\w*\+?=/.test(alternatives[0]) && !/[|]|shard:/.test(m[1]))) return;
    assert.ok(alternatives.every(a => /^[^=$;&`]+$/.test(a)), `cannot evaluate case arm "${line.trim()}": keep alternatives plain globs`);
    let body = m[2];
    for (let j = i + 1; !body.includes(";;") && j < lines.length; j++) body += `\n${lines[j]}`;
    arms.push({ alternatives, body: body.split(";;")[0].trim(), shard: alternatives.some(a => a.startsWith("shard:")) });
  });
  assert.ok(arms.some(a => a.alternatives.includes("shard:g[0-9][0-9]")), "the shard:gNN arm exists");
  // #116: a broader non-shard glob ahead of the last shard arm (*, shard*, s*, *:g*) would capture shard modes first.
  // #147 CI-4: a "^" right after "[" is bash's negation (like "[!"), so only a "^" elsewhere is escaped.
  const toRegExp = glob => new RegExp(`^${glob.replace(/[.+${}()|\\]/g, "\\$&").replace(/(?<!\[)\^/g, "\\^").replace(/\*/g, ".*").replace(/\?/g, ".").replace(/\[!/g, "[^")}$`);
  for (const { alternatives } of arms.slice(0, arms.findLastIndex(a => a.shard)).filter(a => !a.shard)) {
    for (const mode of ["shard:g01", "shard:gFoo", "shard:x", ...shardModes]) {
      assert.ok(!alternatives.some(a => toRegExp(a).test(mode)), `${alternatives.join("|")} would shadow the shard arms for ${mode}`);
    }
  }
  // Every real gNN mode must reach the gNN arm first: a shard arm ahead of it (shard:*) would hand it the wrong -run/-skip.
  for (const mode of shardModes.filter(m => /^shard:g\d\d$/.test(m))) {
    const first = arms.find(a => a.alternatives.some(x => toRegExp(x).test(mode)));
    assert.ok(first?.alternatives.includes("shard:g[0-9][0-9]"), `${first?.alternatives.join("|")} would shadow the shard:gNN arm for ${mode}`);
  }
  for (const { alternatives, body } of arms.filter(a => a.shard)) {
    if (alternatives.every(a => a === "shard:unit")) continue; // 'shard:unit' never satisfies startsWith(matrix.mode, 'shard:g')
    const message = `${alternatives.join("|")} legs share the cache and must build only ./tests/foundation`;
    assert.match(body, /^LC_FOUNDATION_PKGS=\.\/tests\/foundation (?!\S*\.\/)/, message);
    // The shell keeps the last assignment, so a second one anywhere in the arm would override the first (#107); "+=" appends count (#116).
    assert.equal(body.match(/LC_FOUNDATION_PKGS\+?=/g)?.length, 1, message);
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
    [gnn, "            [^a]*) echo shadowed ;;\n" + gnn, /shadow/], // #147 CI-4: bash [^x] is a negation, like [!x]
    [any, "            *)  echo shadowed ;;\n" + any, /shadow/],
  ]) {
    assert.throws(() => shardScopeContract(workflow.replace(from, to)), expected, JSON.stringify(to));
  }
});

// #116 residual of #137 (review-137 P2s): the shadow check enumerates the real shard modes, maps [!x], and fails closed on an arm it cannot evaluate.
test("injected per-mode shadow, gNN-last arm, [!x] class or unevaluable alternative goes RED under the shard scope contract", () => {
  const lines = workflow.split("\n");
  const gnnLine = lines.find(l => l.trimStart().startsWith("shard:g[0-9][0-9])"));
  const anyLine = lines.find(l => l.trimStart().startsWith("shard:*)"));
  assert.ok(gnnLine && anyLine, "the real arms are where the contract looks");
  const swap = (from, to) => workflow.replace(from, () => to);
  for (const [mutated, expected, label] of [
    [swap(gnnLine, `            *:g02) echo shadowed ;;\n${gnnLine}`), /shadow/, "*:g02 before the gNN arm"],
    [swap(gnnLine, `            *:g0[0-9]|x) echo shadowed ;;\n${gnnLine}`), /shadow/, "*:g0[0-9] before the gNN arm"],
    [swap(gnnLine, `            [!a]*) echo shadowed ;;\n${gnnLine}`), /shadow/, "[!a]* before the gNN arm"],
    [swap(`${gnnLine}\n`, "").replace(anyLine, () => `${anyLine}\n${gnnLine}`), /shadow/, "gNN arm after shard:*"],
    [swap(anyLine, '            shard:*|"$X")    LC_FOUNDATION_PKGS=./... '), /cannot evaluate/, 'shard:*|"$X")'],
    [swap(anyLine, "            a;b|shard:*)    LC_FOUNDATION_PKGS=./... "), /cannot evaluate/, "a;b|shard:*)"],
    [swap(gnnLine, `            x=1|shard:g0[0-9]) LC_FOUNDATION_PKGS=./... ;;\n${gnnLine}`), /cannot evaluate/, "x=1|shard:g0[0-9]) before the gNN arm"],
  ]) {
    assert.throws(() => shardScopeContract(mutated), expected, label);
  }
});

// #147 CI-3: only the second `case "$MODE"` (the dispatch with the shard arms) is scanned; a harmless catch-all in the first (the evidence-root case) is not a shadow.
test("a harmless arm in the first case MODE statement is not a shadow of the shard arms", () => {
  const first = "            --browser*|--stripe-browser*|--ops-disk-guard)";
  assert.ok(workflow.includes(first), "the real first-case arm is where the test looks");
  assert.doesNotThrow(() => shardScopeContract(workflow.replace(first, `            *) ;;\n${first}`)));
});

// #147 CI-3: the dispatch is exactly the second `case "$MODE" in`; a later case block that mentions "shard:" must not replace it.
test("only the second case MODE statement is scanned for the shard arms", () => {
  // CI-SPEED-2: the dispatch now closes the per-mode gate_one() function.
  const anchor = "            esac\n          }\n";
  assert.ok(workflow.includes(anchor), "the dispatch esac is where the test looks");
  const third = "            esac\n          }\n          case \"$MODE\" in\n            *) echo \"no shard: arms here\" ;;\n          esac\n";
  assert.doesNotThrow(() => shardScopeContract(workflow.replace(anchor, third)), "a later case block is not the dispatch");
});

// #147 CI-4: `[^s]` is a negation (every shard: mode starts with "s"), not the positive class {^, s} a regexp escaping "^" would build.
test("a bash [^x] glob is a negation, not a positive class", () => {
  const gnn = "            shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation ";
  assert.ok(workflow.includes(gnn), "the real gNN arm is where the test looks");
  assert.doesNotThrow(() => shardScopeContract(workflow.replace(gnn, `            [^s]*) echo not-a-shadow ;;\n${gnn}`)),
    "[^s] cannot shadow a mode that starts with s");
  assert.throws(() => shardScopeContract(workflow.replace(gnn, `            [!x]*) echo shadowed ;;\n${gnn}`)), /shadow/,
    "[!x] is the other bash negation spelling and still shadows modes starting with s");
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
