// Purpose: #147 TH-13 unit coverage for the emitted-bundle gate: marker extraction, client-JS-only scan, and the fail-closed CLI used by
//   scripts/dev/test-local.sh for both the ordinary (absent) and gallery-enabled (present) admin builds.
// Depends on: node:test/assert/fs/os/child_process and scripts/dev/check-gallery-bundle.mjs; no Next build, browser or provider.
// Used by: scripts/dev/test-node.sh (Foundation CI).
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { findMarkers, scanClientJs, SEAM_MARKERS } from "../../scripts/dev/check-gallery-bundle.mjs";

const script = fileURLToPath(new URL("../../scripts/dev/check-gallery-bundle.mjs", import.meta.url));

function fixture(files) {
  const dir = mkdtempSync(path.join(tmpdir(), "gallery-bundle-"));
  for (const [name, body] of Object.entries(files)) {
    const full = path.join(dir, name);
    mkdirSync(path.dirname(full), { recursive: true });
    writeFileSync(full, body);
  }
  return dir;
}

function run(args) {
  return spawnSync(process.execPath, [script, ...args], { encoding: "utf8" });
}

const seam = 'if(0){}else{window.__lcHoldUpload=!0,window.addEventListener("lc-gallery-upload-release",f)}';

test("findMarkers reports the seam shapes (global property + event listener) and misses ordinary code", () => {
  assert.deepEqual(findMarkers(seam).map(({ marker }) => marker).sort(), [...SEAM_MARKERS].sort());
  assert.deepEqual(findMarkers("const enabled = false;"), []);
  // #186 review P2: a bare fragment in a string or identifier is not a seam. Only the two shapes the simulated
  // gallery seam uses count: a global property access and an addEventListener/dispatchEvent event name.
  assert.deepEqual(findMarkers(`a\nb\n__lcHoldUpload`), [], "bare fragment text is not a seam shape");
  assert.deepEqual(findMarkers(`const o={className:"gallery-upload",label:"HoldUpload"};`), [], "ordinary UI strings are not seams");
  assert.deepEqual(findMarkers(`a\nb\nwindow.__lcHoldUpload`).map(({ marker, line }) => [marker, line]), [["__lcHoldUpload", 3]]);
});

test("scanClientJs walks nested chunks and scans only executable client JS", () => {
  const dir = fixture({
    "chunks/app.js": seam,
    "chunks/nested/deeper.mjs": "window.__lcHoldUpload",
    "chunks/app.js.map": "lc-gallery-upload-release",
    "chunks/app.css": "lc-gallery-upload-release",
    "chunks/notes.txt": "lc-gallery-upload-release",
  });
  try {
    const { files, hits } = scanClientJs(path.join(dir, "chunks"));
    assert.equal(files, 2);
    assert.deepEqual(hits.map(({ file, marker }) => [file.replaceAll(path.sep, "/"), marker]).sort(), [
      ["app.js", "__lcHoldUpload"],
      ["app.js", "lc-gallery-upload-release"],
      [path.join("nested", "deeper.mjs").replaceAll(path.sep, "/"), "__lcHoldUpload"],
    ].sort());
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("the CLI fails an ordinary build that still emits the seam and passes a folded one", () => {
  const dirty = fixture({ "chunks/app.js": seam });
  const clean = fixture({ "chunks/app.js": "const enabled=!1;" });
  try {
    const failed = run(["--expect=absent", dirty]);
    assert.equal(failed.status, 1, failed.stderr);
    assert.match(failed.stderr, /FAIL 2 gallery seam marker\(s\)/);
    assert.match(failed.stderr, /app\.js:\d+ __lcHoldUpload/);
    assert.match(failed.stderr, /app\.js:\d+ lc-gallery-upload-release/);
    const passed = run(["--expect=absent", clean]);
    assert.equal(passed.status, 0, passed.stderr);
    assert.match(passed.stdout, /OK expect=absent files=1 markers=0/);
  } finally {
    rmSync(dirty, { recursive: true, force: true });
    rmSync(clean, { recursive: true, force: true });
  }
});

test("the CLI requires the seam in a flag-enabled gallery build (positive control)", () => {
  const gallery = fixture({ "chunks/app.js": seam });
  const folded = fixture({ "chunks/app.js": "const enabled=!1;" });
  try {
    assert.equal(run(["--expect=present", gallery]).status, 0);
    const missing = run(["--expect=present", folded]);
    assert.equal(missing.status, 1, missing.stderr);
    assert.match(missing.stderr, /emitted no gallery seam marker/);
  } finally {
    rmSync(gallery, { recursive: true, force: true });
    rmSync(folded, { recursive: true, force: true });
  }
});

// #171 + #186 review P2: a fragment-built name emits the fragments, not the joined marker, so the
// artifact scan must resolve the fragments inside the seam shapes while the positive control still
// accepts the flag-enabled gallery build. Two unique seams, not four raw fragment occurrences.
test("the scan rejects fragment-built seam names and keeps the gallery positive control passing", () => {
  const fragments = fixture({
    "chunks/app.js": 'const w=window;w[["__lc","HoldUpload"].join("")]=!0,w.addEventListener(["lc-","gallery-upload","-release"].join(""),f)',
  });
  try {
    const absent = run(["--expect=absent", fragments]);
    assert.equal(absent.status, 1, absent.stderr);
    assert.match(absent.stderr, /FAIL 2 gallery seam marker\(s\)/);
    assert.match(absent.stderr, /__lcHoldUpload/);
    assert.match(absent.stderr, /lc-gallery-upload-release/);
    const present = run(["--expect=present", fragments]);
    assert.equal(present.status, 0, present.stderr);
  } finally { rmSync(fragments, { recursive: true, force: true }); }
});

// #186 review P2 RED witness: an ordinary UI string that merely contains a fragment must not be a seam.
// Before the fix this failed --expect=absent (false FAIL) and satisfied --expect=present (false PASS).
test("ordinary UI strings containing a bare fragment are not gallery seams", () => {
  const clean = fixture({ "chunks/app.js": 'const o={className:"gallery-upload",label:"HoldUpload"};' });
  try {
    const absent = run(["--expect=absent", clean]);
    assert.equal(absent.status, 0, absent.stderr);
    assert.match(absent.stdout, /OK expect=absent files=1 markers=0/);
    const present = run(["--expect=present", clean]);
    assert.equal(present.status, 1, "an ordinary string cannot satisfy the positive control");
    assert.match(present.stderr, /emitted no gallery seam marker/);
  } finally { rmSync(clean, { recursive: true, force: true }); }
});

test("the CLI fails closed on a missing build output, empty output or a bad option", () => {
  const empty = fixture({});
  try {
    const missing = run(["--expect=absent", path.join(empty, "does-not-exist")]);
    assert.equal(missing.status, 1, missing.stderr);
    assert.match(missing.stderr, /missing build output/);
    const noJs = run(["--expect=absent", empty]);
    assert.equal(noJs.status, 1, noJs.stderr);
    assert.match(noJs.stderr, /no client JS/);
    assert.equal(run(["--expect=sometimes", empty]).status, 2);
    assert.equal(run(["--expect=absent", empty, "extra"]).status, 2);
  } finally { rmSync(empty, { recursive: true, force: true }); }
});
