// Purpose: verify PR43's committed receipts, screenshot retention, coverage and tested source hashes without local scratch.
// Depends on: Node standard library, evidence/manifest.json, retained run indexes and the current repository sources.
// Used by: reviewers and CI; --tracked additionally rejects evidence absent from the Git index.
import assert from 'node:assert/strict';
import {readFileSync, existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const evidence = path.join(here, 'evidence');
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const json = file => JSON.parse(readFileSync(file, 'utf8'));
const safe = relative => {
  assert(!path.isAbsolute(relative) && !relative.split('/').includes('..'), `unsafe evidence path: ${relative}`);
  return path.join(evidence, relative);
};
const hashed = function* (value) {
  if (!value || typeof value !== 'object') return;
  if (value.file && value.sha256) yield value;
  for (const item of Object.values(value)) if (typeof item === 'object') yield* hashed(item);
};

try {
  assert(existsSync(path.join(evidence, 'manifest.json')), 'NOT_RUN: committed evidence manifest missing');
  const manifest = json(path.join(evidence, 'manifest.json'));
  let images = 0;
  for (const item of manifest.files) {
    const file = safe(item.path);
    const bytes = readFileSync(file);
    assert.equal(bytes.length, item.bytes, `size mismatch: ${item.path}`);
    assert.equal(sha(bytes), item.sha256, `hash mismatch: ${item.path}`);
    if (item.path.endsWith('.png')) images++;
    else assert(!/\/(?:Volumes|private|Users)\//.test(bytes.toString()), `machine path leaked: ${item.path}`);
  }
  const inventory = new Map(manifest.files.map(f => [f.path, f]));
  assert.equal(inventory.size, manifest.files.length, 'duplicate artifact path');
  assert.equal(new Set(manifest.provenance.map(p => p.path)).size, manifest.provenance.length, 'duplicate provenance path');
  for (const origin of manifest.provenance) {
    assert(/^[a-f0-9]{64}$/.test(origin.raw_sha256), `invalid raw hash: ${origin.path}`);
    assert.equal(origin.view_sha256, inventory.get(origin.path)?.sha256, `published provenance hash mismatch: ${origin.path}`);
  }
  for (const [file, hash] of Object.entries(manifest.source)) {
    assert(!path.isAbsolute(file) && !file.split('/').includes('..'));
    assert.equal(sha(readFileSync(path.join(root, file))), hash, `tested source changed: ${file}`);
  }
  for (const run of manifest.visual_runs) {
    const index = json(safe(`${run.path}/index.json`));
    const lint = json(safe(`${run.path}/lint.json`));
    const retained = json(safe(`${run.path}/screenshots.json`));
    const byFile = new Map(retained.map(s => [s.file, s]));
    assert.equal(byFile.size, 540, 'duplicate screenshot manifest entry');
    assert.equal(index.expected, 348);
    assert.equal(index.captured, 348);
    assert.equal(index.shots.length, 348);
    assert.equal(new Set(index.shots.map(s => `${s.app}:${s.id}:${s.locale}:${s.size}`)).size, 348);
    assert.equal(index.notRun.length + index.missing.length, 0);
    assert.equal(lint.blockingInstances, 0);
    assert.equal(lint.verdict.exit, 0);
    const records = [...hashed(index.shots)];
    assert.equal(records.length, 540);
    assert.equal(retained.length, 540);
    for (const record of records) {
      const entry = byFile.get(record.file);
      assert(entry && entry.sha256 === record.sha256 && entry.bytes === record.bytes, `missing screenshot hash: ${record.file}`);
      assert(['COMMITTED', 'HASH_ONLY'].includes(entry.retention));
      if (entry.retention === 'COMMITTED') assert.equal(sha(readFileSync(safe(`${run.path}/${record.file}`))), record.sha256);
      else assert(!existsSync(safe(`${run.path}/${record.file}`)), 'retention manifest mislabels committed image');
    }
    const supplements = index.shots.flatMap(s => s.supplemental).filter(s => s.size === '1440x900');
    assert.equal(supplements.length, 12);
    assert(supplements.every(s => s.file && s.sha256 && !s.issues.length));
    if (run.require_supplemental_bytes) assert(supplements.every(s => retained.find(r => r.file === s.file)?.retention === 'COMMITTED'));
    assert.equal(retained.filter(s => s.retention === 'COMMITTED').length, run.committed_png);
    assert.equal(retained.filter(s => s.retention === 'HASH_ONLY').length, run.hash_only_png);
    assert.equal(run.committed_png, run.id === 'run.rtxlAF0q' ? 54 : 17);
    if (run.id === 'run.rtxlAF0q') {
      assert(run.require_supplemental_bytes);
      const required = index.shots.filter(s => ['ads', 'products-new', 'products-new-variants', 'products-product'].includes(s.id));
      assert.equal(required.length, 24);
      const files = required.flatMap(s => [s.file, ...s.supplemental.flatMap(p => p.file ? [p.file] : p.frames ? [p.frames.at(-1).file] : [])]);
      assert.equal(new Set(files).size, 54);
      assert(files.every(f => byFile.get(f)?.retention === 'COMMITTED'), 'required affected-page image not committed');
    }
    for (const pane of index.shots.flatMap(s => s.supplemental).filter(s => s.kind === 'inner-scroll frames')) {
      assert(pane.complete && pane.frames.length > 0);
      assert.equal(pane.frames[0].visibleRange.from, 0);
      for (let i = 1; i < pane.frames.length; i++) assert(pane.frames[i].visibleRange.from <= pane.frames[i - 1].visibleRange.to, 'native pane coverage gap');
      // Match the existing capture producer's one CSS-pixel rounding allowance.
      assert(pane.frames.at(-1).visibleRange.to >= pane.scrollHeight - 1, 'native pane end missing');
    }
    console.log(`${run.id}: 348/348, missing=0, blocking=0; 12 supplements; 540 screenshot hashes`);
  }
  for (const [file, directory, total, committed] of [
    ['savebar-91b9a6a8/product/screenshots.json', 'savebar-91b9a6a8/product/shots', 179, 14],
    ['original/before-manifest.json', 'original', 24, 2],
  ]) {
    const records = json(safe(file));
    assert.equal(records.length, total);
    assert.equal(new Set(records.map(r => r.file)).size, total);
    assert.equal(records.filter(r => r.retention === 'COMMITTED').length, committed);
    for (const r of records) {
      assert(['COMMITTED', 'HASH_ONLY'].includes(r.retention));
      assert(/^[a-f0-9]{64}$/.test(r.sha256));
      const image = safe(`${directory}/${r.file}`);
      if (r.retention === 'COMMITTED') {
        const bytes = readFileSync(image);
        assert.equal(sha(bytes), r.sha256);
        assert.equal(bytes.length, r.bytes);
      } else assert(!existsSync(image), `retention mislabels committed image: ${r.file}`);
    }
  }
  const report = readFileSync(path.join(here, 'DELIVERY.md'), 'utf8');
  assert(!/\/(?:Volumes|private|Users)\//.test(report), 'machine path leaked in DELIVERY');
  for (const [, target] of report.matchAll(/\]\(([^)]+)\)/g)) {
    if (/^https:\/\//.test(target)) continue;
    assert(!target.includes('playwright/') && !path.isAbsolute(target), `scratch link: ${target}`);
    assert(existsSync(path.resolve(here, target)), `broken DELIVERY link: ${target}`);
  }
  if (process.argv.includes('--tracked')) {
    const tracked = new Set(execFileSync('git', ['ls-files', '-z'], {cwd: root, encoding: 'utf8'}).split('\0'));
    for (const file of [...manifest.files.map(f => `evidence/${f.path}`), 'evidence/manifest.json', 'DELIVERY.md', 'verify-evidence.mjs'])
      assert(tracked.has(`output/visual-polish-r1/${file}`), `uncommitted evidence: ${file}`);
  }
  console.log(`PASS: ${manifest.files.length} retained artifacts (${images} PNG); source hashes and DELIVERY links verified`);
} catch (error) {
  console.error(`FAIL: ${error.message.replaceAll(root, '.')}`);
  process.exitCode = 1;
}
