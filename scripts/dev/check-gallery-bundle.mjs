#!/usr/bin/env node
// Purpose: #147 TH-13 production-artifact check. Fails an ordinary (flag-absent) admin build whose EMITTED client JS still carries the
//   simulated gallery hold/release seam, and confirms a flag-enabled gallery build still carries it so the absence check cannot pass vacuously.
// Depends on: node:fs/node:path and the `next build` output under apps/admin/.next/static; no Next runtime, browser, network or provider.
// Used by: scripts/dev/test-local.sh (`lc_build_admin` -> --expect=absent for every production-shaped build a gate already runs;
//   `--browser-ui-gallery` -> --expect=present after its flag-enabled build) and tests/ci/gallery-bundle.test.mjs.
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Fragment markers (#171): a name built with Array.join/template concatenation emits the
// fragments, not the joined spelling, so the emitted-bundle gate matches both.
export const SEAM_MARKERS = ["__lcHoldUpload", "lc-gallery-upload-release", "HoldUpload", "gallery-upload"];
const CLIENT_JS = /\.m?js$/;
const root = fileURLToPath(new URL("../../", import.meta.url));
export const DEFAULT_STATIC_DIR = path.join(root, "apps/admin/.next/static");

/** Every marker occurrence in one emitted chunk, with its 1-based line. */
export function findMarkers(text, markers = SEAM_MARKERS) {
  const hits = [];
  for (const marker of markers) {
    let from = 0;
    while (true) {
      const index = text.indexOf(marker, from);
      if (index < 0) break;
      let line = 1;
      for (let at = 0; at < index; at++) if (text.charCodeAt(at) === 10) line++;
      hits.push({ marker, line });
      from = index + marker.length;
    }
  }
  return hits;
}

/**
 * Scan executable client JS under `dir`. Source maps and non-JS assets are not code the browser runs, so they are skipped; the caller
 * asserts the seam is absent from what the browser actually executes.
 */
export function scanClientJs(dir) {
  let files = 0;
  const hits = [];
  const walk = (current) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) { walk(full); continue; }
      if (!CLIENT_JS.test(entry.name)) continue;
      files++;
      for (const hit of findMarkers(readFileSync(full, "utf8"))) hits.push({ file: path.relative(dir, full), ...hit });
    }
  };
  walk(dir);
  return { files, hits };
}

function usage() {
  return "usage: node scripts/dev/check-gallery-bundle.mjs [--expect=absent|present] [static-dir]";
}

function parseArgs(argv) {
  let expect = "absent";
  let dir = null;
  for (const arg of argv) {
    if (arg.startsWith("--expect=")) expect = arg.slice("--expect=".length);
    else if (arg.startsWith("--")) throw new Error(`unknown option ${arg}`);
    else if (dir) throw new Error(`unexpected argument ${arg}`);
    else dir = arg;
  }
  if (expect !== "absent" && expect !== "present") throw new Error(`--expect must be absent or present, got ${expect}`);
  return { expect, dir: dir ? path.resolve(dir) : DEFAULT_STATIC_DIR };
}

function main(argv) {
  let options;
  try { options = parseArgs(argv); } catch (error) { console.error(`check-gallery-bundle: ${error.message}\n${usage()}`); return 2; }
  const { expect, dir } = options;
  if (!existsSync(dir)) { console.error(`check-gallery-bundle: missing build output ${dir}`); return 1; }
  const { files, hits } = scanClientJs(dir);
  if (files === 0) { console.error(`check-gallery-bundle: no client JS under ${dir}`); return 1; }
  for (const hit of hits) console.error(`check-gallery-bundle: ${hit.file}:${hit.line} ${hit.marker}`);
  if (expect === "absent" && hits.length > 0) {
    console.error(`check-gallery-bundle: FAIL ${hits.length} gallery seam marker(s) in ${files} emitted client JS files under ${dir}`);
    return 1;
  }
  if (expect === "present" && hits.length === 0) {
    console.error(`check-gallery-bundle: FAIL the flag-enabled build emitted no gallery seam marker in ${files} client JS files under ${dir}`);
    return 1;
  }
  console.log(`check-gallery-bundle: OK expect=${expect} files=${files} markers=${hits.length} dir=${dir}`);
  return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2));
}
