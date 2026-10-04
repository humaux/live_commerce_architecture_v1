import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const read = file => readFileSync(new URL(file, import.meta.url), 'utf8');
const log = read('g07-final/G07.log');
const samples = [];
let at;
for (const line of read('g07-machine-load.log').split('\n')) {
  if (/^\d{4}-.*Z$/.test(line)) at = line;
  const match = /load averages?:\s*([\d.]+)[, ]+([\d.]+)[, ]+([\d.]+)/.exec(line);
  if (match) samples.push({ at, one: +match[1], five: +match[2], fifteen: +match[3] });
}
assert.ok(samples.length > 0);
const preflight = read('g07-quiet-preflight.log').trim().split('\n').map(line => {
  const [at, one, five, fifteen, cpus] = line.split(/\s+/);
  return { at, one: +one, five: +five, fifteen: +fifteen, cpus: +cpus };
});
const cpus = preflight.at(-1).cpus;
const stats = key => {
  const a = samples.map(row => row[key]).sort((a, b) => a - b);
  const mid = Math.floor(a.length / 2);
  return {
    median: a.length % 2 ? a[mid] : (a[mid - 1] + a[mid]) / 2,
    max: a.at(-1),
    atOrAboveLogicalCPUs: a.filter(v => v >= cpus).length,
  };
};
console.log(JSON.stringify({
  source: '6c6c3fb397dd8c6ae8c291f3d751c1eedbc6ac89',
  topLevelPass: (log.match(/^--- PASS:/gm) ?? []).length,
  allPassRecords: (log.match(/^[\t ]*--- PASS:/gm) ?? []).length,
  allFailRecords: (log.match(/^[\t ]*--- FAIL:/gm) ?? []).length,
  skips: (log.match(/^[\t ]*--- SKIP:.*$/gm) ?? []),
  preflight, logicalCPUs: cpus, sampleCount: samples.length,
  firstSample: samples[0].at, lastSample: samples.at(-1).at,
  oneMinute: stats('one'), fiveMinute: stats('five'),
  continuouslyBelowLogicalCPUs: samples.every(row => row.one < cpus && row.five < cpus),
}, null, 2));
