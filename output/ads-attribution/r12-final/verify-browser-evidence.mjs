import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { devices } from '@playwright/test';

const base = new URL('./browser-final/', import.meta.url).pathname;
for (const [kind, expected] of [['report', 42], ['checkout', 24]]) {
  const dir = path.join(base, kind);
  const manifest = JSON.parse(readFileSync(path.join(dir, 'screenshots.json')));
  assert.equal(manifest.length, expected);
  const names = new Set();
  for (const row of manifest) {
    const file = row.File ?? row.file;
    assert.equal(path.basename(file), file);
    assert.ok(!names.has(file));
    names.add(file);
    const bytes = readFileSync(path.join(dir, file));
    assert.equal(createHash('sha256').update(bytes).digest('hex'), row.Sha256 ?? row.sha256);
    const viewport = Number(row.Viewport ?? row.viewport);
    assert.ok([390, 1586].includes(viewport));
    // Frozen Chromium checkout driver spreads the Pixel 7 profile, then
    // overrides its CSS viewport to 390. PNG pixels include its 2.625 DPR.
    const scale = kind === 'checkout' && viewport === 390 ? devices['Pixel 7'].deviceScaleFactor : 1;
    assert.equal(bytes.readUInt32BE(16), Math.ceil(viewport * scale));
  }
  console.log(`${kind}: ${manifest.length} unique screenshots, SHA-256 and PNG widths verified`);
}
let actions = 0;
for (const locale of ['zh-TW', 'zh-CN', 'en']) {
  for (const width of [390, 1586]) {
    const ledger = JSON.parse(readFileSync(path.join(base, 'report', `attribution-clicks-${locale}-${width}.json`)));
    assert.equal(ledger.length, 17);
    for (const row of ledger) assert.equal(row.actual, 'PASS');
    for (const control of ['organic claim cohort', 'acknowledged UNKNOWN read', 'fresh audience request receipt', 'linked-draft identifiers']) {
      assert.equal(ledger.filter(row => row.control === control).length, 1);
    }
    actions += ledger.length;
  }
}
console.log(`report: six real-click ledgers, ${actions} PASS records, required R12 actions present`);
