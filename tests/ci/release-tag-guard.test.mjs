// Purpose: CW-3 (#147, from #116) — the test-only Go build tag `claimsconsoletest` (cmd/claims-worker/console_testhook.go closes the
//   claims bridge listener after readiness) must never reach a release build. Every git-tracked file outside prose (docs/, output/, *.md;
//   deploy/** and docs/runbooks/** are scanned even as .md) that names the tag must be one of the three test-lane files below; a
//   Dockerfile, compose file, deploy script, scripts/, Makefile or workflow YAML carrying it is red. The allowlist is the hook, the one
//   process test that builds with it, and this guard. cmd/claims-worker/main.go is a release source and must not name the tag at all.
//   SELECTORS pins the only two build selectors by file:line, so moving one is a reviewable edit rather than a silent drift.
// Depends on: git ls-files (tracked files only).
// Used by: scripts/dev/test-node.sh; injected regressions below exercise the same scan as the real tree.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";

const TAG = "claimsconsoletest";
// Only test-lane files may name the tag. None is built into a release: the hook carries its own build constraint, the process test
// runs on the Foundation/Gates runners, and this guard must name the tag to scan for it.
const ALLOWED = new Set([
  "cmd/claims-worker/console_testhook.go",
  "tests/foundation/claims_worker_console_process_test.go",
  "tests/ci/release-tag-guard.test.mjs",
]);
// The only two places a build selects the tag (this commit's file:line). A moved selector must update this list.
const SELECTORS = [
  { path: "cmd/claims-worker/console_testhook.go", line: 1, text: "//go:build claimsconsoletest" },
  { path: "tests/foundation/claims_worker_console_process_test.go", line: 50, text: '"-tags", "claimsconsoletest"' },
];
// Prose is exempt (evidence logs, design docs, GATES.md name the tag), EXCEPT what operators copy-paste into a release:
// deploy/** and docs/runbooks/** (review-148 P2-b), including their .md files.
const OPERATOR = /^(deploy|docs\/runbooks)\//;
const EXEMPT = path => !OPERATOR.test(path) && (/^(docs|output)\//.test(path) || path.endsWith(".md"));
const root = new URL("../../", import.meta.url);

// files: [{path, text}]. Returns the paths that name the tag but are not allowed.
function offenders(files) {
  return files
    .filter(({ path }) => !ALLOWED.has(path) && !EXEMPT(path))
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

test("the allowlist pins the only build selectors at file:line", () => {
  for (const { path, line, text } of SELECTORS) {
    const source = files.find(f => f.path === path)?.text.split("\n") ?? [];
    assert.ok(
      (source[line - 1] ?? "").includes(text),
      `${path}:${line} no longer contains ${text}; update SELECTORS (the tag may not move into a release path silently)`,
    );
  }
});

test("claimsconsoletest is the only build constraint in cmd/claims-worker", () => {
  const constraints = files
    .filter(f => f.path.startsWith("cmd/claims-worker/") && f.path.endsWith(".go"))
    .flatMap(f => [...f.text.matchAll(/^\/\/go:build (.+)$/gm)].map(m => `${f.path}: ${m[1]}`));
  assert.deepEqual(constraints, ["cmd/claims-worker/console_testhook.go: claimsconsoletest"]);
});

test("injected release-path occurrences are red", () => {
  const bad = [
    "deploy/docker/go.Dockerfile",
    "deploy/compose.yml",
    "scripts/dev/build.sh",
    "Makefile",
    ".github/workflows/gates.yml",
    "deploy/README.md",
    "docs/runbooks/deploy.md",
  ].map(path => ({ path, text: `RUN go build -tags=netgo,${TAG} ./cmd/claims-worker\n` }));
  assert.deepEqual(offenders(bad), bad.map(b => b.path));
  // A release source (the worker's own main.go) must not carry the tag either.
  assert.deepEqual(offenders([{ path: "cmd/claims-worker/main.go", text: `go build -tags ${TAG}` }]), ["cmd/claims-worker/main.go"]);
  assert.deepEqual(offenders([{ path: "docs/delivery/GATES.md", text: TAG }, { path: "output/x.log", text: TAG }]), []);
  assert.deepEqual(offenders([{ path: "tests/foundation/claims_worker_console_process_test.go", text: TAG }]), []);
});
