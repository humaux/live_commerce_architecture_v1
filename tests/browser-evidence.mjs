// Purpose: keep browser-run artifacts beneath the harness root or a unique ignored local run.
// Depends on: Node fs/path and LC_BROWSER_EVIDENCE / LC_BROWSER_EVIDENCE_ROOT.
// Used by: standalone browser producers and Playwright config; historical/golden inputs stay read-only.
import { mkdirSync, mkdtempSync } from "node:fs";
import path from "node:path";

/** Create a suite directory under its harness parent, or a unique ignored standalone run. */
export function browserEvidenceDirectory(label) {
  if (typeof label !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(label)) {
    throw new TypeError("Invalid browser evidence label: use one relative directory name.");
  }
  const parent = process.env.LC_BROWSER_EVIDENCE || process.env.LC_BROWSER_EVIDENCE_ROOT;
  if (parent) {
    const directory = path.resolve(parent, label);
    mkdirSync(directory, { recursive: true });
    return directory;
  }
  const parentDirectory = path.resolve("output/playwright");
  mkdirSync(parentDirectory, { recursive: true });
  return mkdtempSync(path.join(parentDirectory, `${label}-`));
}
