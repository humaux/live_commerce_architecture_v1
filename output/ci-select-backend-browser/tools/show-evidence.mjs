#!/usr/bin/env node
// Round-2 review helper: print the evidence rows of one or more modes (optionally filtered to packages),
// deduplicated by (package, via, path) so the derivation stays auditable.
// usage: node --test show-evidence.mjs   (edit MODES/PKGS below)
import { readFileSync } from "node:fs";
const d = JSON.parse(readFileSync(new URL("../covers-derivation.json", import.meta.url), "utf8"));
const MODES = process.env.LC_EV_MODES ? process.env.LC_EV_MODES.split(",") : ["--browser-password-auth", "--browser-tracking-backfill"];
const PKGS = process.env.LC_EV_PKGS ? process.env.LC_EV_PKGS.split(",") : null;
for (const mode of MODES) {
  console.log("=====", mode);
  const seen = new Set();
  for (const ev of d[mode] ?? []) {
    if (PKGS && !PKGS.includes(ev.package)) continue;
    const k = `${ev.package}|${ev.via.split(" @")[0]}|${ev.path}`;
    if (seen.has(k)) continue;
    seen.add(k);
    console.log(`  ${ev.package} | ${ev.via} | ${ev.path} | ${ev.handler}`);
  }
}
