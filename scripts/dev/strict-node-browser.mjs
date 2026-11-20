#!/usr/bin/env node
// Consumes the two Node-only browser mode receipts for release-gate.sh. Never runs a test.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { BROWSER_SPEC_MARK, strictBrowserNodeSpec } from "./matrix-receipts.mjs";

const [mode, log, evidenceRoot] = process.argv.slice(2);
try {
  const text = readFileSync(log, "utf8");
  const spec = strictBrowserNodeSpec(text);
  assert.ok(spec, "missing or invalid Node spec reporter summary");
  if (mode === "--browser-picklist") {
    assert.ok(spec.lines.includes(BROWSER_SPEC_MARK) && spec.lines.some(line =>
      /^✔ W3-U1b real clicks: selection, print, CSV, confirmation, unknown and scope \(\d+(?:\.\d+)?ms\)$/.test(line)),
    "missing named Picklist browser PASS");
    console.log(`${spec.count} Node browser tests including W3-U1b real clicks`);
  } else if (mode === "--browser-platform-site") {
    const { platformBrowserCoverage } = await import("../../tests/admin/platform-browser-coverage.mjs");
    const read = file => JSON.parse(readFileSync(join(evidenceRoot, "platform-site", file), "utf8"));
    const { pass, ssrCases, tagCases, clicks, messagingCopyCases, reloads } =
      platformBrowserCoverage(read("ps-browser-result.json"), read("click-ledger.json").rows);
    const summary = `${pass} page cases, ${ssrCases} SSR cases, ${tagCases} tag cases, ${clicks} real clicks, ${messagingCopyCases} messaging-copy cases and ${reloads} reloads`;
    assert.deepEqual(text.split(/\r?\n/).filter(line => line.startsWith("PASS PS1/PS2/PS4:")),
      [`PASS PS1/PS2/PS4: ${summary}`], "platform terminal summary missing or corrupt");
    console.log(`${spec.count} Node tests; ${summary} (MOCK transport)`);
  } else {
    throw new Error(`unsupported browser mode ${mode}`);
  }
} catch (error) {
  console.error(`strict browser evidence: ${error.message}`);
  process.exitCode = 1;
}
