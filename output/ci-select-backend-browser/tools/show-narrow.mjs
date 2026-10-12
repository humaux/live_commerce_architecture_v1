#!/usr/bin/env node
// Round-2 review helper: per-mode narrow covers sizes (descending) + a reverse index package -> modes.
import { readFileSync } from "node:fs";
const n = JSON.parse(readFileSync(new URL("./covers-narrow.json", import.meta.url), "utf8"));
const rows = Object.entries(n).map(([m, l]) => [m, l.length]).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
for (const [m, c] of rows) console.log(String(c).padStart(3), m);
console.log("modes:", rows.length, "| sum:", rows.reduce((a, r) => a + r[1], 0), "| mean:", (rows.reduce((a, r) => a + r[1], 0) / rows.length).toFixed(1));
const rev = new Map();
for (const [m, l] of Object.entries(n)) for (const p of l) { if (!rev.has(p)) rev.set(p, []); rev.get(p).push(m); }
console.log("\nselection size per changed package (modes a PR touching only it would select):");
for (const [p, ms] of [...rev].sort((a, b) => b[1].length - a[1].length)) console.log(String(ms.length).padStart(3), p);
for (const probe of ["internal/live", "internal/integrations/metareply", "internal/claims", "internal/inbox", "internal/catalog", "internal/orders"]) {
  console.log(`\nPROBE ${probe}: ${(rev.get(probe) ?? []).join(" ") || "(none — no mode covers it directly)"}`);
}
