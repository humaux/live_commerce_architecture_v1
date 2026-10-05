// Purpose: Aggregate exact-SHA gate results without changing any verdict or log.
// Depends on: Node fs/path and local gates-*/results.tsv files; no external services.
// Used by: admin-visual evidence handoff; missing rows remain NOT_RUN.
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

const sourceArgument = process.argv.indexOf("--source");
const source = sourceArgument < 0 ? "" : process.argv[sourceArgument + 1];
if (!/^[a-f0-9]{40}$/.test(source ?? "")) {
  throw new Error("Pass --source with the full pinned source SHA; never inherit an old checkpoint.");
}
const root = new URL("./", import.meta.url).pathname;
const expected = [
  "product-editor", "cvs", "admin-shell", "identity", "password-auth",
  "admin-legacy", "merchant-buyer", "manual-order", "merchant-orders-bff",
  "merchant-orders-ui", "input-delivery", "studio-bff", "studio-ui",
  "live-claims", "refund-fulfilment", "customers-billing", "storefront-publish",
  "design", "store-domains", "meta-ads", "ads-attribution", "meta-connect",
  "catalog-media", "ops-polish", "promotions", "catalog-core",
  "checkout-offline", "home-cod", "e2e", "webkit", "platform-site",
  "click-sweep", "visual-lint",
].map((name) => `--browser-${name}`);
const history = new Map();
for (const directory of readdirSync(root).filter((name) => name.startsWith("gates-")).sort()) {
  const file = join(root, directory, "results.tsv");
  let lines;
  try { lines = readFileSync(file, "utf8").trimEnd().split("\n").slice(1); }
  catch (error) { if (error.code === "ENOENT") continue; throw error; }
  for (const line of lines) {
    const [sha, mode, exit, seconds, log] = line.split("\t");
    if (!sha || !mode || !/^\d+$/.test(exit) || !/^\d+$/.test(seconds) || !log)
      throw new Error(`Malformed ledger row in ${file}`);
    const record = { source: sha, mode, exit: Number(exit), seconds: Number(seconds), log };
    const previous = history.get(mode) ?? [];
    history.set(mode, [...previous, record]);
  }
}
const rows = expected.map((mode) => {
  const attempts = history.get(mode) ?? [];
  const exact = attempts.filter((record) => record.source === source).at(-1);
  const latest = attempts.at(-1);
  return { mode, status: exact ? (exact.exit === 0 ? "PASS" : "FAIL") : "NOT_RUN_FINAL_SOURCE", exact, latest };
});
const counts = {};
for (const row of rows) counts[row.status] = (counts[row.status] ?? 0) + 1;
if (process.argv.includes("--json")) {
  console.log(JSON.stringify({ source, counts, rows }, null, 2));
} else {
  console.log(`# Browser command ledger\n\nSource: \`${source}\`. Aggregated from actual TSV rows; no row means NOT_RUN, never PASS. Earlier-source passes are shown only as historical evidence.\n`);
  console.log(`Counts: ${JSON.stringify(counts)}.\n`);
  console.log("| Mode | Final-source status | Exit | Seconds | Evidence log |\n| --- | --- | --- | --- | --- |");
  for (const row of rows) {
    const record = row.exact;
    console.log(`| \`${row.mode}\` | ${row.status} | ${record?.exit ?? "—"} | ${record?.seconds ?? "—"} | ${record ? `[log](${record.log.replace("output/admin-visual/", "")})` : row.latest ? `Historical ${row.latest.source.slice(0, 8)} exit ${row.latest.exit}` : "NOT_RUN"} |`);
  }
}
