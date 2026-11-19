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
