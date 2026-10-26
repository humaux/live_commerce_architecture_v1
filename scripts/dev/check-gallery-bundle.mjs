#!/usr/bin/env node
// Purpose: #147 TH-13 production-artifact check. Fails an ordinary (flag-absent) admin build whose EMITTED client JS still carries the
//   simulated gallery hold/release seam, and confirms a flag-enabled gallery build still carries it so the absence check cannot pass vacuously.
// Depends on: node:fs/node:path and the `next build` output under apps/admin/.next/static; no Next runtime, browser, network or provider.
// Used by: scripts/dev/test-local.sh (`lc_build_admin` -> --expect=absent for every production-shaped build a gate already runs;
//   `--browser-ui-gallery` -> --expect=present after its flag-enabled build) and tests/ci/gallery-bundle.test.mjs.
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Seam shapes (#171 + #186 review P2): a name built with Array.join/template concatenation emits the fragments, not the
// joined spelling, but a bare fragment is also ordinary UI text (`className:"gallery-upload"`). Match only the two shapes
// the simulated seam actually uses: a property access on a global (`window.__lcHoldUpload`, or the fragment-built
// `w[["__lc","HoldUpload"].join("")]`) and an addEventListener/dispatchEvent event name (direct or fragment-built).
export const SEAM_PROPERTY = "__lcHoldUpload";
export const SEAM_EVENT = "lc-gallery-upload-release";
export const SEAM_MARKERS = [SEAM_PROPERTY, SEAM_EVENT];
const CLIENT_JS = /\.m?js$/;
const root = fileURLToPath(new URL("../../", import.meta.url));
export const DEFAULT_STATIC_DIR = path.join(root, "apps/admin/.next/static");

// One fragment join: an array of string literals concatenated with join("") / join('').
const JOIN_ARRAY = String.raw`\[\s*((?:(?:"[^"]*"|'[^']*')\s*,\s*)*(?:"[^"]*"|'[^']*'))\s*\]\s*\.\s*join\(\s*(?:""|'')\s*\)`;
const PROPERTY_DIRECT = /(?:\bwindow\b|\bglobalThis\b|\bself\b)\s*(?:\.\s*__lcHoldUpload\b|\[\s*["']__lcHoldUpload["']\s*\])/;
const PROPERTY_FRAGMENT = new RegExp(String.raw`\b[A-Za-z_$][\w$]*\s*\[\s*${JOIN_ARRAY}\s*\]`, "g");
const EVENT_DIRECT = /(?:addEventListener|dispatchEvent)\s*\(\s*["'`]lc-gallery-upload-release["'`]/;
const EVENT_FRAGMENT = new RegExp(String.raw`(?:addEventListener|dispatchEvent)\s*\(\s*${JOIN_ARRAY}`, "g");

/** The concatenated literals of a fragment join: ["__lc","HoldUpload"].join("") -> "__lcHoldUpload". */
function joinedName(arrayBody) {
  return (arrayBody.match(/"[^"]*"|'[^']*'/g) ?? []).map((literal) => literal.slice(1, -1)).join("");
}

/** One hit per distinct seam in one chunk (first occurrence, 1-based line); ordinary strings are not seams. */
export function findMarkers(text) {
  const hits = [];
  const add = (marker, index) => {
    if (index < 0 || hits.some((hit) => hit.marker === marker)) return;
    let line = 1;
    for (let at = 0; at < index; at++) if (text.charCodeAt(at) === 10) line++;
    hits.push({ marker, line });
  };
  const property = PROPERTY_DIRECT.exec(text);
  if (property) add(SEAM_PROPERTY, property.index);
  else for (const match of text.matchAll(PROPERTY_FRAGMENT)) {
    if (joinedName(match[1]) === SEAM_PROPERTY) { add(SEAM_PROPERTY, match.index); break; }
  }
  const event = EVENT_DIRECT.exec(text);
  if (event) add(SEAM_EVENT, event.index);
  else for (const match of text.matchAll(EVENT_FRAGMENT)) {
    if (joinedName(match[1]) === SEAM_EVENT) { add(SEAM_EVENT, match.index); break; }
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
  // Count unique seams: the same component emitted into several chunks is one seam, not one per occurrence.
  const seams = new Map();
  for (const hit of hits) if (!seams.has(hit.marker)) seams.set(hit.marker, hit);
  for (const hit of seams.values()) console.error(`check-gallery-bundle: ${hit.file}:${hit.line} ${hit.marker}`);
  if (expect === "absent" && seams.size > 0) {
    console.error(`check-gallery-bundle: FAIL ${seams.size} gallery seam marker(s) in ${files} emitted client JS files under ${dir}`);
    return 1;
  }
  if (expect === "present" && seams.size === 0) {
    console.error(`check-gallery-bundle: FAIL the flag-enabled build emitted no gallery seam marker in ${files} client JS files under ${dir}`);
    return 1;
  }
  console.log(`check-gallery-bundle: OK expect=${expect} files=${files} markers=${seams.size} dir=${dir}`);
  return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2));
}
