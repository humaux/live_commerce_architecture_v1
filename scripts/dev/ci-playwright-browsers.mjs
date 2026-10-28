// Purpose: select the browser installation set for one CI matrix job (one mode, or the union over a CI-SPEED-2 bundle's members).
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
/** Reviewed exception to the WebKit coverage scan, keyed by repo-relative POSIX path. Each entry promises that the
 * file's LC_BROWSER_ENGINE=webkit path only selects browserFront's TLS boundary and never resolves or launches
 * Playwright's WebKit binary; the reason names that reviewed code path. Any other file carrying
 * `// lc:webkit=front-only` fails closed. An allowlisted file that also calls webkit.executablePath() contradicts its
 * own declaration: that call is positive evidence the WebKit executable is resolved, so it fails closed too.
 * Adding an entry here is a coverage decision, never a way to silence this scan (PR #188 review P1). */
const FRONT_ONLY_SOURCES = new Map([
  ['tests/foundation/browser_admin_relay_test.go', 'browserFront TLS front under a variable LC_BROWSER_ENGINE; no WebKit executable is launched'],
]);
const canonicalPath = file => typeof file === 'string' && file.trim()
  ? path.posix.normalize(file.replace(/\\/g, '/').replace(/^\.\//, '')) : '';
/** A Go test file launches WebKit when it calls webkit.executablePath(), sets the engine to the literal "webkit", or drives
 * LC_BROWSER_ENGINE from a variable. An explicit marker is the reviewed contract and beats every inference:
 * `// lc:webkit=browser` (a browser binary may launch) or `// lc:webkit=front-only` (only browserFront's TLS branch,
 * allowed only for a file in FRONT_ONLY_SOURCES); a variable-driven file without a marker throws (TH-7, #147). */
function usesWebKit(source, file = '') {
  const marker = source.match(/^\/\/ lc:webkit=(front-only|browser)\b/m)?.[1];
  if (marker === 'browser') return true;
  if (marker === 'front-only') {
    const reviewed = canonicalPath(file);
    if (!FRONT_ONLY_SOURCES.has(reviewed))
      throw new Error(`${reviewed || '<unknown file>'}: "// lc:webkit=front-only" is a reviewed per-file exception, not a self-serve opt-out; add the file and its reason to FRONT_ONLY_SOURCES (allowed: ${[...FRONT_ONLY_SOURCES.keys()].join(', ')})`);
    if (/\bwebkit\.executablePath\(\)/.test(source))
      throw new Error(`${reviewed}: "// lc:webkit=front-only" contradicts executable WebKit launch evidence (webkit.executablePath())`);
    return false;
  }
  if (/\bwebkit\.executablePath\(\)/.test(source)) return true;
  let driven = false;
  for (const m of source.matchAll(/\b\w+\.Setenv\(\s*"LC_BROWSER_ENGINE"\s*,\s*([^)]*?)\s*\)/g)) {
    if (m[1] === '"webkit"') return true;
    if (!/^"[a-z]+"$/.test(m[1])) driven = true;
  }
  if (driven) throw new Error('variable-driven LC_BROWSER_ENGINE needs a "// lc:webkit=front-only|browser" marker: the WebKit coverage scan cannot see which engine it launches');
  return false;
}
/** Assert explicit WebKit commands and selected mixed Go test roots receive WebKit dependencies.
 * `testFiles` is parallel to `testSources` (repo-relative paths) and supplies the provenance the front-only
 * allowlist needs; a marker without provenance fails closed rather than being trusted. */
export function assertWebKitCoverage(source, testSources, testFiles = []) {
  if (testFiles.length && testFiles.length !== testSources.length)
    throw new Error(`testFiles must be parallel to testSources: ${testFiles.length} != ${testSources.length}`);
  // File-level inspection intentionally errs toward requiring WebKit for sibling tests.
  const mixedRoots = testSources.map((body, i) => ({ body, file: testFiles[i] ?? '' }))
    .filter(({ body, file }) => usesWebKit(body, file))
    .flatMap(({ body }) => [...body.matchAll(/^func (Test\w+)\(t \*testing\.T\)/gm)].map(m => m[1]));
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
/** CLI writes a safe engine list/cache suffix for GitHub Actions; non-browser legs emit empty requirements.
 * CI-SPEED-2: a bundle job passes all its member modes and gets the union, in registry order (chromium before webkit), so the
 * cache suffix is one a solo job of the same engine set also uses. */
export function main(modes = process.argv.slice(2), env = process.env) {
  if (typeof modes === 'string') modes = [modes];
  if (!Array.isArray(modes) || !modes.length || modes.some((m) => typeof m !== 'string' || !m.length)) throw new Error('matrix mode required');
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
  const source = readFileSync(path.join(root, 'scripts/dev/test-local.sh'), 'utf8');
  const wanted = new Set(modes.flatMap((mode) => browserRequirements(mode, source, env.EXTRA_ENV ?? '')));
  const engines = ['chromium', 'webkit'].filter((e) => wanted.has(e));
  console.log(`ci-playwright: mode=${modes.join(' ')} browsers=${engines.join(' ') || 'none'}`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `engines=${engines.join(' ')}\ncache=${engines.join('-')}\n`);
  return engines;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main(); } catch (error) { console.error(`ci-playwright: ${error.message}`); process.exit(1); }
}
