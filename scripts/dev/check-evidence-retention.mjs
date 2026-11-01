// Purpose: evidence retention gate (docs/adr/ADR-20261031-evidence-retention.md): a PR may not add raw CI artifact copies,
//   a single file over 1 MiB, or more than 4 MiB in total under output/. Raw artifacts stay in GitHub Actions (90 days) and in
//   ignored local paths; DELIVERY.md cites run id, job, artifact name and sha256 instead.
// Depends on: git (merge-base, diff, ls-tree); GITHUB_BASE_REF in pull_request CI, or --base <ref> when run by hand.
// Used by: scripts/dev/check-gates.sh (foundation workflow) and tests/ci/evidence-retention.test.mjs.
// Invariants: only files this PR adds or modifies under output/ are judged, so history already on the base is never re-judged;
//   with no PR base (push to main) the gate reports NOT_RUN and exits 0 rather than inventing a base.
import {execFileSync} from 'node:child_process';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

// Raw CI artifact copies: downloaded artifact trees, Playwright run roots and results, archives and videos.
export const RAW = /(^|\/)[^/]*-artifacts?\/|\/playwright\/run\.[^/]+\/|\/test-results\/|\/results\/[^/]+\/(trace\.zip|video\.webm|test-failed-\d+\.png|error-context\.md)$|\.(zip|gz|tgz|tar|webm)$/i;
export const MAX_FILE = 1 << 20;
export const MAX_TOTAL = 4 << 20;

/** Judge the output/ files a PR adds or modifies ([{path, size}]); returns one message per violation. */
export function judge(entries) {
  const errors = [];
  let total = 0;
  for (const {path: file, size} of entries) {
    total += size;
    if (RAW.test(file)) errors.push(`${file}: raw CI artifact copy; cite run id/job/artifact name + sha256 in DELIVERY.md instead`);
    else if (size > MAX_FILE) errors.push(`${file}: ${size} bytes > ${MAX_FILE} per-file limit`);
  }
  if (total > MAX_TOTAL) errors.push(`output/ additions total ${total} bytes > ${MAX_TOTAL} per-PR budget`);
  return errors;
}

/** Files under output/ added or modified on HEAD since its merge-base with base, with their blob sizes. */
export function changedOutput(root, base) {
  const git = (...args) => execFileSync('git', args, {cwd: root, encoding: 'utf8', maxBuffer: 1 << 28});
  const mergeBase = git('merge-base', base, 'HEAD').trim();
  const changed = new Set(git('diff', '--name-only', '--no-renames', '--diff-filter=AM', '-z', mergeBase, 'HEAD', '--', 'output').split('\0').filter(Boolean));
  if (!changed.size) return [];
  const sizes = new Map(git('ls-tree', '-r', '-l', '-z', 'HEAD', '--', 'output').split('\0').filter(Boolean).map(row => {
    const tab = row.indexOf('\t');
    return [row.slice(tab + 1), Number(row.slice(0, tab).trim().split(/\s+/)[3])];
  }));
  return [...changed].map(file => ({path: file, size: sizes.get(file) ?? 0}));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const at = process.argv.indexOf('--base');
  const base = at > 0 ? process.argv[at + 1] : process.env.GITHUB_BASE_REF ? `origin/${process.env.GITHUB_BASE_REF}` : '';
  if (!base) {
    console.log('evidence-retention: NOT_RUN (no PR base: set GITHUB_BASE_REF or pass --base <ref>)');
  } else {
    const entries = changedOutput(process.cwd(), base);
    const errors = judge(entries);
    for (const e of errors) console.error(`evidence-retention: ${e}`);
    const bytes = entries.reduce((n, e) => n + e.size, 0);
    console.log(`evidence-retention: ${errors.length ? 'FAIL' : 'PASS'} (${entries.length} output/ files, ${bytes} bytes vs ${base}; ADR-20261031)`);
    process.exitCode = errors.length ? 1 : 0;
  }
}
