// Purpose: select the browser installation set for one CI matrix mode.
// Depends on: the test-local registry; no browser or network operation.
// Used by: gates.yml and CI installer contract tests.
import { readFileSync, appendFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { modeEntries } from './pr-modes.mjs';
const isBrowser = mode => /^--(?:browser|stripe-browser)/.test(mode);
function declared(entry) {
  const matches = [...entry.body.matchAll(/^\s*lc_browsers=(?:"([a-z ]+)"|([a-z]+))\s*$/gm)];
  if (matches.length !== 1) throw new Error(`${entry.name}: one browser declaration required`);
  const value = matches[0][1] ?? matches[0][2];
  if (!['chromium', 'chromium webkit'].includes(value)) throw new Error(`${entry.name}: invalid browser declaration`);
  return value.split(' ');
}
/** Resolve one matrix leg from the registry and validated calibration environment; no commands execute. */
export function browserRequirements(mode, source, extraEnv = '') {
  let engine = '';
  for (const value of extraEnv.trim().split(/\s+/).filter(Boolean)) {
    if (!/^LC_[A-Z0-9_]+=[A-Za-z0-9._:-]*$/.test(value)) throw new Error('invalid extra_env entry');
    if (value.startsWith('LC_BROWSER_ENGINE=')) engine = value.slice('LC_BROWSER_ENGINE='.length);
  }
  if (!['', 'chromium', 'webkit'].includes(engine)) throw new Error('invalid browser engine');
  if (!isBrowser(mode)) return [];
  const base = mode.replace(/@\d+\/\d+$/, '');
  const entry = modeEntries(source).find(e => e.name === base);
  if (!entry) throw new Error(`unknown registry mode: ${mode}`);
  const browsers = declared(entry);
  if (engine === 'webkit' && !browsers.includes('webkit')) browsers.push('webkit');
  return browsers;
}
/** Assert explicit WebKit commands and selected mixed Go test roots receive WebKit dependencies. */
export function assertWebKitCoverage(source, testSources) {
  // File-level inspection intentionally errs toward requiring WebKit for sibling tests.
  const mixedRoots = testSources.filter(s => /\b\w+\.Setenv\(\s*"LC_BROWSER_ENGINE"\s*,\s*"webkit"\s*\)|\bwebkit\.executablePath\(\)/.test(s))
    .flatMap(s => [...s.matchAll(/^func (Test\w+)\(t \*testing\.T\)/gm)].map(m => m[1]));
  for (const entry of modeEntries(source).filter(e => isBrowser(e.name))) {
    const browsers = declared(entry);
    const code = entry.body.split('\n').filter(l => !l.trimStart().startsWith('#')).join('\n');
    const selectors = [...code.matchAll(/-run\s+["']([^"']+)["']/g)].map(m => m[1].split('/')[0]);
    const selected = mixedRoots.some(name => code.includes(name) || selectors.some(selector => {
      try { return new RegExp(selector).test(name); } catch { return false; }
    }));
    if ((selected || /\bLC_BROWSER_ENGINE=["']?webkit\b/.test(code)) && !browsers.includes('webkit'))
      throw new Error(`${entry.name}: selected WebKit execution requires webkit declaration`);
  }
}
/** CLI writes a safe engine list/cache suffix for GitHub Actions; non-browser legs emit empty requirements. */
export function main(mode = process.argv[2], env = process.env) {
  if (typeof mode !== 'string' || !mode.length) throw new Error('matrix mode required');
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
  const engines = browserRequirements(mode, readFileSync(path.join(root, 'scripts/dev/test-local.sh'), 'utf8'), env.EXTRA_ENV ?? '');
  console.log(`ci-playwright: mode=${mode} browsers=${engines.join(' ') || 'none'}`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `engines=${engines.join(' ')}\ncache=${engines.join('-')}\n`);
  return engines;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main(); } catch (error) { console.error(`ci-playwright: ${error.message}`); process.exit(1); }
}
