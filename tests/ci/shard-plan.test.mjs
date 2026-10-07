// Unit tests of the foundation shard plan (scripts/dev/shard-plan.mjs): the source parser agrees with Go's own test discovery rules, the packing is deterministic and
// balanced, and the coverage proof is red when a top-level test would run in no shard or in two. Run by scripts/dev/test-node.sh (no Go, no PG).
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { UNPLANNED_LIMIT, check, pack, parseDurations, runRegex, selectedBy, skipRegex, topLevelTests } from "../../scripts/dev/shard-plan.mjs";

const plan = JSON.parse(readFileSync(new URL("../../scripts/dev/shard-plan.json", import.meta.url), "utf8"));
const names = topLevelTests();

test("parser: only compiled, correctly-shaped Test functions of untagged _test.go files", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "shard-parse-"));
  writeFileSync(path.join(dir, "a_test.go"), 'package foundation_test\nfunc TestAlpha(t *testing.T) {}\nfunc Test_under(t *testing.T) {}\nfunc Testlower(t *testing.T) {}\nfunc TestMain(m *testing.M) {}\nfunc TestHelper() string { return "" }\nfunc BenchmarkX(b *testing.B) {}\nfunc Test(t *testing.T) {}\n');
  writeFileSync(path.join(dir, "b_test.go"), '//go:build browser\n\npackage foundation_test\nfunc TestBrowserOnly(t *testing.T) {}\n');
  writeFileSync(path.join(dir, "c_test.go"), '//go:build !browser\n\npackage foundation_test\nfunc TestNotBrowser(tt *testing.T) {}\n');
  writeFileSync(path.join(dir, "d.go"), "package foundation\nfunc TestNotATestFile(t *testing.T) {}\n");
  assert.deepEqual(topLevelTests(dir), ["Test", "TestAlpha", "TestNotBrowser", "Test_under"].sort());
});

test("the real plan: every top-level test of tests/foundation runs in exactly one group", () => {
  assert.ok(names.length > 1000, `parsed ${names.length} tests`);
  const found = check(plan, names);
  assert.deepEqual(found.problems, []);
  for (const n of names) assert.equal(selectedBy(plan, n).length, 1, n);
});

test("a new test (in no list) still runs, exactly once, in the catch-all group; the same name with no catch-all is red", () => {
  const fake = "TestBrandNewGateNobodyPlanned";
  const catchAll = plan.groups.find((g) => g.catch_all).id;
  assert.deepEqual(selectedBy(plan, fake), [catchAll]);
  assert.deepEqual(check(plan, [...names, fake]).problems, []);
  // red: break the invariant that makes this true (no catch-all) and the same fake name falls outside every shard
  const noCatchAll = { ...plan, groups: plan.groups.map((g) => ({ ...g, catch_all: false })) };
  assert.deepEqual(selectedBy(noCatchAll, fake), []);
  assert.ok(check(noCatchAll, [...names, fake]).problems.some((p) => p.includes(`${fake} is selected by 0 shard(s)`)));
  // red: two catch-alls would run every unplanned test twice
  const two = { ...plan, groups: plan.groups.map((g, i) => ({ ...g, catch_all: i < 2 })) };
  assert.ok(check(two, [...names, fake]).problems.some((p) => /exactly one group must be catch_all/.test(p)));
});

test("a test listed in two groups is red; drift beyond the limit is red", () => {
  const dup = structuredClone(plan), victim = dup.groups[0].tests[0];
  dup.groups[1].tests.push(victim);
  assert.ok(check(dup, names).problems.some((p) => p.includes(`${victim} is listed in both`)));
  const many = Array.from({ length: UNPLANNED_LIMIT + 1 }, (_, i) => `TestUnplanned${i}`);
  assert.ok(check(plan, [...names, ...many]).problems.some((p) => /catch-all shard has drifted/.test(p)));
  assert.deepEqual(check(plan, [...names, ...many.slice(0, 3)]).problems, [], "a few unplanned tests are fine: they run in the catch-all");
});

test("regexes: anchored explicit lists; the catch-all skips exactly everyone else's tests", () => {
  for (const g of plan.groups) {
    if (g.catch_all) { assert.equal(runRegex(g), ""); assert.ok(skipRegex(plan, g).startsWith("^(") && skipRegex(plan, g).endsWith(")$")); }
    else { assert.ok(runRegex(g).startsWith("^(Test") && runRegex(g).endsWith(")$")); assert.equal(skipRegex(plan, g), ""); }
  }
  const skip = skipRegex(plan, plan.groups.find((g) => g.catch_all));
  assert.ok(skip.length < 120000, `catch-all -skip is ${skip.length} bytes; a single argument over ~128 KB fails exec`);
});

test("pack: deterministic, balanced to within the longest test, every name once", () => {
  const seconds = Object.fromEntries(names.map((n, i) => [n, (i * 7919) % 97 / 10]));
  const a = pack(names, seconds, 7), b = pack([...names].reverse(), seconds, 7);
  assert.deepEqual(a, b);
  assert.equal(a.groups.flatMap((g) => g.tests).length, names.length);
  const est = a.groups.map((g) => g.est_seconds), max = Math.max(...Object.values(seconds));
  assert.ok(Math.max(...est) - Math.min(...est) <= max + 1, `est ${est.join(",")}`);
});

test("parseDurations: top-level lines only", () => {
  const log = "=== RUN   TestA\n    --- PASS: TestA/sub (9.00s)\n--- PASS: TestA (12.34s)\n--- SKIP: TestB (0.00s)\n--- FAIL: TestC (1.50s)\nPASS\n";
  assert.deepEqual(parseDurations(log), { TestA: 12.34, TestB: 0, TestC: 1.5 });
});
