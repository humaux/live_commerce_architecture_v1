// Purpose: #107 — pin that every workflow job that runs pnpm gets it from the cached Corepack home through
//   .github/actions/pnpm (no per-run download from registry.npmjs.org: an ECONNRESET there failed a gate on run 39285412405),
//   and that a cache hit runs Corepack with the network disabled so a download cannot silently come back.
// Depends on: .github/workflows/*.yml, .github/actions/pnpm/action.yml (text only); Node built-ins.
// Used by: scripts/dev/test-node.sh (Foundation "Node unit suites"); no network, no pnpm, no Corepack is executed.
import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import test from "node:test";

const root = new URL("../../", import.meta.url);
const read = (p) => readFileSync(new URL(p, root), "utf8");
const ACTION = ".github/actions/pnpm/action.yml";
const uncommented = (src) => src.replace(/(^|\s)#.*$/gm, "$1");

// Jobs are the 2-space keys under `jobs:`; returns [name, body] with YAML comments removed.
function jobs(src) {
  const body = uncommented(src).split(/^jobs:\s*$/m)[1] ?? "";
  return body.split(/^(?=  [\w-]+:\s*$)/m).filter((j) => j.trim()).map((j) => [j.trim().split(":")[0], j]);
}

function workflowContract(file, src) {
  assert.doesNotMatch(uncommented(src), /\bcorepack\b/, `${file}: Corepack only through ${ACTION} (#107)`);
  for (const [name, job] of jobs(src)) {
    const first = job.search(/\bpnpm\s/);
    if (first < 0) continue;
    const setup = job.search(/uses:\s*\.\/\.github\/actions\/pnpm\s*$/m);
    assert.ok(setup >= 0 && setup < first, `${file} job ${name}: runs pnpm before (or without) ${ACTION} (#107)`);
  }
}

function actionContract(src) {
  const body = uncommented(src);
  assert.match(body, /uses:\s*actions\/cache\/restore@caa296126883cff596d87d8935842f9db880ef25\s*$/m, "restore uses the repo's pinned actions/cache SHA");
  assert.match(body, /uses:\s*actions\/cache\/save@caa296126883cff596d87d8935842f9db880ef25\s*$/m, "save uses the repo's pinned actions/cache SHA");
  assert.equal((body.match(/path:\s*~\/\.cache\/node\/corepack\s*$/gm) ?? []).length, 2, "restore and save the Corepack home");
  assert.match(body, /key:\s*corepack-.*steps\.ver\.outputs\.node.*steps\.ver\.outputs\.pm/, "the key follows the resolved Node version and package.json packageManager (#116)");
  assert.doesNotMatch(body, /hashFiles\('package\.json'\)/, "not the whole package.json (#116)");
  // #147 CI-1: the offline variable must sit in the HIT branch itself, not merely somewhere in the file (the later GITHUB_ENV line also names it).
  const hit = body.match(/if \[\[ "\$HIT" == true \]\]; then\n([\s\S]*?)\n\s*else\b/)?.[1] ?? "";
  assert.match(hit, /COREPACK_ENABLE_NETWORK=0 pnpm/, "a hit runs Corepack offline (inside the HIT branch)");
  // #147 CI-2: Corepack's own version is part of the key, so upgrading Corepack separately from Node cannot reuse a stale home.
  assert.match(body, /corepack --version/, "the ver step resolves Corepack's own version");
  assert.match(body, /key:\s*corepack-.*steps\.ver\.outputs\.corepack/, "the key follows Corepack's version (#147 CI-2)");
  assert.match(body, /echo "COREPACK_ENABLE_NETWORK=0" >> "\$GITHUB_ENV"/, "later pnpm calls in the job stay offline for Corepack");
}

test("every pnpm-running job sets pnpm up through the cached, offline-on-hit Corepack action", () => {
  assert.ok(existsSync(new URL(ACTION, root)), `${ACTION} exists`);
  actionContract(read(ACTION));
  const files = readdirSync(new URL(".github/workflows/", root)).filter((f) => /\.ya?ml$/.test(f));
  assert.ok(files.includes("gates.yml") && files.includes("foundation.yml") && files.includes("deploy-smoke.yml"));
  for (const f of files) workflowContract(f, read(`.github/workflows/${f}`));
});

test("injected bare Corepack or a pnpm step ahead of the action goes RED", () => {
  const gates = read(".github/workflows/gates.yml");
  const use = gates.match(/^      - uses: \.\/\.github\/actions\/pnpm\b.*\n/m)?.[0] ?? "";
  assert.ok(use, "the real gate job uses the action");
  assert.throws(() => workflowContract("gates.yml", gates.replace(use, "")), /runs pnpm before/);
  assert.throws(() => workflowContract("gates.yml", gates.replace(use, "      - run: corepack enable\n")), /Corepack only through/);
  const action = read(ACTION);
  assert.throws(() => actionContract(action.replaceAll("COREPACK_ENABLE_NETWORK=0", "COREPACK_ENABLE_NETWORK=1")), /offline/);
  assert.throws(() => actionContract(action.replace(/actions\/cache\/restore@\w+/, "actions/cache/restore@v5")), /pinned/);
  // #147 CI-1: dropping the variable from the HIT branch only (the GITHUB_ENV line still names it) must go RED.
  assert.ok(action.includes("version=$(COREPACK_ENABLE_NETWORK=0 pnpm"), "the real hit branch is where the contract looks");
  assert.throws(() => actionContract(action.replace("version=$(COREPACK_ENABLE_NETWORK=0 pnpm", "version=$(pnpm")), /HIT branch/);
  // #147 CI-2: a key without Corepack's version goes RED.
  assert.ok(action.includes("-${{ steps.ver.outputs.corepack }}"), "the real key is where the contract looks");
  assert.throws(() => actionContract(action.replace("-${{ steps.ver.outputs.corepack }}", "")), /Corepack's version/);
});

// #147 CI-1: matching the HIT branch is not enough if that branch can fall back to a networked pnpm.
test("a HIT branch cannot fall back to a networked download", () => {
  const action = read(ACTION);
  const offline = "version=$(COREPACK_ENABLE_NETWORK=0 pnpm --version)";
  assert.ok(action.includes(offline), "the real offline invocation is where the test looks");
  const fallback = action.replace(offline, `${offline} || pnpm --version`);
  assert.notEqual(fallback, action);
  assert.throws(() => actionContract(fallback), /offline|networked/, "a hit must not be able to download on an offline failure");
});

// #147 CI-2: the version must be resolved in the `ver` step; moving the echo leaves steps.ver.outputs.corepack empty.
test("the Corepack version is resolved in the ver step", () => {
  const action = read(ACTION);
  const corepackEcho = '        echo "corepack=$(corepack --version)" >> "$GITHUB_OUTPUT"\n';
  assert.ok(action.includes(corepackEcho), "the real ver step is where the test looks");
  const moved = action.replace(corepackEcho, "").replace("    - name: pnpm via Corepack (offline on a cache hit)",
    `    - id: late\n      shell: bash\n      run: |\n${corepackEcho}    - name: pnpm via Corepack (offline on a cache hit)`);
  assert.notEqual(moved, action);
  assert.throws(() => actionContract(moved), /Corepack's version/, "the key must read the version the ver step resolved");
});
