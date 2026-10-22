// Purpose: CW-3 (#147, from #116) — the test-only Go build tag `claimsconsoletest` (cmd/claims-worker/console_testhook.go closes the
//   claims bridge listener 1s after bind) must never reach a release build. Every git-tracked file outside prose (docs/, output/, *.md)
//   that names the tag must be on the four-file allowlist below; a Dockerfile, compose file, deploy script, scripts/, Makefile or
//   workflow YAML carrying it is red. The allowlist is the hook, the one main.go comment, the one process test (mrBuild) and this guard.
// Depends on: git ls-files (tracked files only).
// Used by: scripts/dev/test-node.sh; injected regressions below exercise the same scan as the real tree.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";

const TAG = "claimsconsoletest";
const ALLOWED = new Set([
  "cmd/claims-worker/console_testhook.go",
  "cmd/claims-worker/main.go",
  "tests/foundation/claims_worker_console_process_test.go",
  "tests/ci/release-tag-guard.test.mjs",
]);
const root = new URL("../../", import.meta.url);

// files: [{path, text}]. Returns the paths that name the tag but are not allowed.
function offenders(files) {
  return files
    .filter(({ path }) => !ALLOWED.has(path) && !/^(docs|output)\//.test(path) && !path.endsWith(".md"))
    .filter(({ text }) => text.includes(TAG))
    .map(({ path }) => path);
}

const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: root, encoding: "utf8" }).split("\0").filter(Boolean);
const files = tracked.flatMap(path => {
  try {
    return [{ path, text: readFileSync(new URL(path, root), "utf8") }];
  } catch {
    return []; // deleted in the working tree or a submodule path
  }
});

test("claimsconsoletest appears only in the test harness", () => {
  assert.ok(files.some(f => f.path === "cmd/claims-worker/console_testhook.go"), "scan must see the hook file");
  assert.deepEqual(offenders(files), [], "test-only build tag leaked outside its allowlist (a release build would close the claims bridge)");
});

test("the allowlisted harness still builds with the tag (guard not stale)", () => {
  const harness = files.find(f => f.path === "tests/foundation/claims_worker_console_process_test.go");
  assert.match(harness?.text ?? "", /"-tags", "claimsconsoletest"/);
});

test("injected release-path occurrences are red", () => {
  const bad = [
    "deploy/docker/go.Dockerfile",
    "deploy/compose.yml",
    "scripts/dev/build.sh",
    "Makefile",
    ".github/workflows/gates.yml",
  ].map(path => ({ path, text: `RUN go build -tags=netgo,${TAG} ./cmd/claims-worker\n` }));
  assert.deepEqual(offenders(bad), bad.map(b => b.path));
  assert.deepEqual(offenders([{ path: "tests/foundation/claims_worker_console_process_test.go", text: TAG }]), []);
});
