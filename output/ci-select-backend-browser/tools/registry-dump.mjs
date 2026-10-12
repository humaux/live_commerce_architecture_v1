#!/usr/bin/env node
// Round-2 exploration helper: dump each registry mode's build/fixture/browsers, the Go test names and
// file paths its lc_prepare/lc_run reference. Feeds the design of derive-narrow-covers.mjs.
import { readFileSync } from "node:fs";

const src = readFileSync("scripts/dev/test-local.sh", "utf8");
const b = src.indexOf("# BEGIN MODE REGISTRY"), e = src.indexOf("# END MODE REGISTRY");
const reg = src.slice(b, e);
const re = /^ {2}(--[a-z0-9-]+|foundation)\)\n([\s\S]*?)(?=^ {2}(?:--[a-z0-9-]+|foundation|\*)\)|^esac$)/gm;
let m; const rows = [];
while ((m = re.exec(reg))) {
  const body = m[2];
  const build = (body.match(/^ {4}lc_build=(\S+)$/m) || [])[1];
  const fixture = (body.match(/^ {4}lc_fixture=(\S+)$/m) || [])[1];
  const browsers = (body.match(/^ {4}lc_browsers=(.+)$/m) || [])[1];
  const coversLine = body.match(/^ {4}lc_covers="([^"]*)"$/m);
  const ncov = coversLine ? (coversLine[1] ? coversLine[1].split(/\s+/).length : 0) : null;
  const run = (body.match(/lc_run\(\) \{\n([\s\S]*?)\n {4}\}/) || [, ""])[1];
  const prep = (body.match(/lc_prepare\(\) \{\n([\s\S]*?)\n {4}\}/) || [, ""])[1];
  const files = [...new Set([...(prep + "\n" + run).matchAll(/(?:tests|apps|scripts|packages)\/[A-Za-z0-9_.@/-]+/g)].map((x) => x[0]))];
  const runs = [...new Set([...run.matchAll(/-run '\^?\(?\^?([A-Za-z0-9_|^$()]+)\$?\)?'/g)].map((x) => x[1]))];
  rows.push({ mode: m[1], build, fixture, browsers, ncov, runs, files });
}
console.log(`${rows.length} registry arms`);
for (const r of rows) {
  console.log(`\n${r.mode}  build=${r.build} fixture=${r.fixture} browsers=${r.browsers} covers=${r.ncov}`);
  if (r.runs.length) console.log(`  -run: ${r.runs.join(" , ")}`);
  if (r.files.length) console.log(`  files: ${r.files.join(" ")}`);
}
