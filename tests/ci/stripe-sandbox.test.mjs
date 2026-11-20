// Exercise the release gate's event classifier with synthetic events, without a provider or key.
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const gate = readFileSync("scripts/dev/release-gate.sh", "utf8");
const judge = gate.match(/^judge_gotest\(\) \{[\s\S]*?^\}$/m)[0];
const required = ["TestStripeSP16Sandbox", "TestStripeRF10Sandbox",
  "TestStripeSP21Registrar/sandbox_probe_against_real_stripe"];
const call = gate.match(/judge_gotest "\$LOG" "\$OUT\/G07z.skipped" [^\n]*?(?=\)")/)[0];

test("G07z requires a PASS for each named sandbox probe", () => {
  const dir = mkdtempSync(join(tmpdir(), "g07z-"));
  try {
    const classify = actions => {
      const events = ["TestStripeSP21Registrar", ...required].flatMap((name, i) => {
        const action = i === 0 ? "pass" : actions[i - 1];
        return action ? ["=== RUN   " + name, "--- " + action.toUpperCase() + ": " + name + " (0.00s)"] : [];
      });
      writeFileSync(join(dir, "log"), [...events, "PASS", ""].join("\n"));
      const result = spawnSync("bash", ["-c", judge + "\nLOG=log OUT=.\n" + call], { cwd: dir, encoding: "utf8" });
      assert.equal(result.status, 0, result.stderr);
      return Number(result.stdout.trim().split(" ")[1]);
    };
    assert.equal(classify(required.map(() => "pass")), 0);
    for (const [i, name] of required.entries()) {
      for (const action of [null, "skip"]) {
        assert.ok(classify(required.map((_, j) => j === i ? action : "pass")) > 0, name + " " + action);
      }
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("full strict rejects a fake Stripe key or credential path before starting a suite", () => {
  const dir = mkdtempSync(join(tmpdir(), "strict-isolation-"));
  try {
    const secretPath = join(dir, "protected-fake.env");
    const fake = ["sk", "test", "syntheticisolationcanary"].join("_");
    writeFileSync(secretPath, "STRIPE_SECRET_KEY=" + fake, { mode: 0o600 });
    const options = { PATH: process.env.PATH, HOME: dir, LC_RELEASE_GATE_FULL_STRICT: "1",
      LC_RELEASE_GATE_OUT: join(dir, "ordinary") };
    for (const secret of [{ STRIPE_SECRET_KEY: fake }, { LC_SECRETS_FILE: secretPath }]) {
      const result = spawnSync("bash", ["scripts/dev/release-gate.sh", "--strict"], {
        encoding: "utf8", env: { ...options, ...secret }, timeout: 10000,
      });
      assert.equal(result.status, 2, result.stdout + result.stderr);
      assert.match(result.stderr, /Stripe-credential-free job/);
      assert.ok(!result.stderr.includes(fake));
      assert.throws(() => readFileSync(join(dir, "ordinary", "results.tsv")), { code: "ENOENT" });
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("strict G06s requires the complete package, test and passing package terminal", () => {
  const dir = mkdtempSync(join(tmpdir(), "strict-g06-"));
  try {
    const w1 = "livecommerce/bench/l01/neutral/go";
    const stripe = "livecommerce/internal/integrations/psp/stripe";
    const event = (Package, Action, Test) => ({ Package, Action, Test });
    const emit = (name, events) => writeFileSync(join(dir, name), events.map(e => JSON.stringify(e)).join("\n") + "\n");
    emit("G06.jsonl", [event(w1, "skip", "TestHTTPPostgres"), event(stripe, "skip", required[0])]);
    for (const [pkg, terminal, cover] of [[w1, "pass", true], ["unrelated/go", "pass", false],
        [w1, "fail", false], [w1, "skip", false], [w1, null, false]]) {
      emit("G07.jsonl", [event(pkg, "pass", "TestHTTPPostgres"), ...(terminal ? [event(pkg, terminal)] : []),
        event(stripe, "pass", required[0]), event(stripe, "pass")]);
      const result = spawnSync("python3", ["scripts/dev/strict-g06-coverage.py", dir, "native"], { encoding: "utf8" });
      assert.equal(result.status, 0, result.stderr);
      assert.equal(readFileSync(join(dir, "G06s.covered"), "utf8"), cover ? w1 + ".TestHTTPPostgres" : "");
      assert.equal(readFileSync(join(dir, "G06s.skipped"), "utf8"), (cover ? "" : w1 + ".TestHTTPPostgres\n") + stripe + "." + required[0]);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("strict evidence rejects missing, duplicate, wrong-attempt and failed child jobs, and missing SP18 leaves", () => {
  const script = String.raw`
import copy, runpy
v = runpy.run_path("scripts/dev/strict-release-evidence.py")
sha = "a" * 40
ctx = {"sha": sha, "run_id": 99, "attempt": 2}
run = dict(path=v["GATES"], head_sha=sha, id=99, run_attempt=2, head_branch="r3/integration", event="workflow_dispatch")
def job(i, name, steps):
    return dict(id=i, name=name, run_id=99, run_attempt=2, head_sha=sha, status="completed",
                conclusion="success", steps=[dict(name=s, conclusion="success") for s in steps])
jobs = [job(i, name, [step, "Sign strict child receipt", "Upload strict child receipt"])
        for i, (name, _, step) in enumerate(v["KINDS"].values(), start=100)]
jobs.append(job(200, "receipts", ["Strict canonical G07 intake", "Canonical finalization deadline"]))
assert len(v["attempt_binding"](run, jobs, v["KINDS"], ctx)) == 4
def reject(call):
    try:
        call()
    except ValueError:
        return
    raise AssertionError("invalid native evidence accepted")
reject(lambda: v["attempt_binding"](run, jobs[:-1], v["KINDS"], ctx))
reject(lambda: v["attempt_binding"](run, jobs + jobs[:1], v["KINDS"], ctx))
for key, value in [("run_attempt", 1), ("head_sha", "b" * 40), ("conclusion", "failure")]:
    bad = copy.deepcopy(jobs)
    bad[0][key] = value
    reject(lambda: v["attempt_binding"](run, bad, v["KINDS"], ctx))
bad = copy.deepcopy(jobs)
bad[-1]["steps"][-1]["conclusion"] = "skipped"
reject(lambda: v["attempt_binding"](run, bad, v["KINDS"], ctx))
log = "".join(f"{name}: pass={count+1} fail=0 skip=0 leaf_pass={count} leaf_fail=0 leaf_skip=0 exit=0 verdict=0 duration=1s\n"
              for name, count in v["BROWSER_STEPS"].items())
log += "PASS: stripe-browser steps [ node-env sp18 su07 su09 ]\nSANDBOX checkout.stripe.com test mode; no live charge; SP17 webhook NOT_RUN\n"
assert v["browser_projection"](log)["passed"]["sp18"] == 6
reject(lambda: v["browser_projection"](log.replace("leaf_pass=6", "leaf_pass=5")))
reject(lambda: v["browser_projection"](log.replace("exit=0 verdict=0", "exit=1 verdict=1")))
`;
  const result = spawnSync("python3", ["-c", script], { encoding: "utf8", timeout: 10000 });
  assert.equal(result.status, 0, result.stderr);
});
