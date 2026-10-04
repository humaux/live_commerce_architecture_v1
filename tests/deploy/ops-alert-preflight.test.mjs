// ops-disk-guard D4 preflight gate: deploy/scripts/preflight.sh rule P08 for LC_ALERT_EMAIL / LC_ALERT_WEBHOOK_URL (docs/delivery/units/ops-disk-guard.md).
// Without an alert channel a failing watchdog reaches nobody (cron MAILTO=root is a local mailbox): the 2026-10-03 disk-full outage failed W1-W4
// for 29.5 h unseen. Production must configure one (FAIL), elsewhere it is a WARN; an e-mail target also needs the SMTP relay it is sent through.
// Label: MOCK (static config files only; no host, no network, no Docker). preflight prints rule + status + variable NAMES only; the secrets-dir
// rules (P02/P03) are expected to FAIL in this bare fixture and are ignored (only lines naming LC_ALERT are read).
// Run: node --test tests/deploy/ops-alert-preflight.test.mjs   (also part of scripts/dev/test-node.sh)
import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const deploy = path.join(root, "deploy");
const services = ["api", "admin", "storefront", "payment-worker", "expiry-worker", "meta-worker", "claims-worker", "ads-worker", "caddy", "postgres"];

// preflight against a temp config built from the shipped *.example files; o = LC_ENVIRONMENT, alert vars, relay vars, relay secret.
function preflight(o = {}) {
  const cfg = { env: "smoke", email: "", hook: "", smtpHost: "smtp.fake.invalid", smtpUser: "sender@fake.invalid", smtpSecret: "x".repeat(24), ...o };
  const dir = mkdtempSync(path.join(tmpdir(), "lc-od-pf-"));
  try {
    for (const d of ["env", "secrets", "backup", "state"]) mkdirSync(path.join(dir, d));
    for (const s of services) cpSync(path.join(deploy, "env", `${s}.env.example`), path.join(dir, "env", `${s}.env`));
    if (cfg.smtpSecret !== null) writeFileSync(path.join(dir, "secrets", "commerce_smtp_password"), `${cfg.smtpSecret}\n`);
    const gid = String(process.getgid?.() ?? 0);
    writeFileSync(path.join(dir, "compose.env"), [
      "COMPOSE_PROJECT_NAME=lc-od-pf", "COMPOSE_PROFILES=db,app,payments-sandbox", "IMAGE_TAG=smoke", "LC_IMAGE_PREFIX=lc", `LC_ENVIRONMENT=${cfg.env}`,
      "LC_BIND_ADDR=127.0.0.1", "LC_HTTP_PORT=80", "LC_HTTPS_PORT=443", "LC_ADMIN_HOST=admin.localhost", "LC_STORE_HOST=shop.localhost",
      "LC_API_HOST=api.localhost", "LC_HOOKS_HOST=hooks.localhost", "LC_PUBLIC_IP=", `LC_ENV_DIR=${dir}/env`, `LC_SECRETS_DIR=${dir}/secrets`,
      `LC_SECRETS_GID=${gid}`, `LC_BACKUP_DIR=${dir}/backup`, `LC_STATE_DIR=${dir}/state`, "LC_PG_HOST=postgres", "LC_PG_SSLMODE=disable",
      "LC_IDENTITY_ENABLED=1", "LC_OIDC_ISSUER=https://accounts.google.com", "LC_ONBOARDING_ENABLED=0", "LC_ONBOARDING_CURRENCIES=",
      "LC_BUYER_ENABLED=1", "LC_BUYER_SESSION_TTL_SECONDS=3600", "LC_STRIPE_ENABLED=1", "LC_REQUIRE_MEDIA_GATE=0", "LC_REQUIRE_RETENTION_ENFORCED=0",
      `LC_SMTP_HOST=${cfg.smtpHost}`, `LC_SMTP_USERNAME=${cfg.smtpUser}`, `LC_ALERT_EMAIL=${cfg.email}`, `LC_ALERT_WEBHOOK_URL=${cfg.hook}`, "",
    ].join("\n"));
    // A clean environment: preflight lets the shell win over compose.env, so nothing of the caller's may leak in.
    const env = { PATH: process.env.PATH, HOME: process.env.HOME ?? dir, LC_COMPOSE_ENV: path.join(dir, "compose.env") };
    const run = spawnSync("bash", [path.join(deploy, "scripts", "preflight.sh"), "--skip-images"], { env, encoding: "utf8", timeout: 120_000 });
    assert.ok(run.status !== null, `preflight did not finish: ${run.error ?? ""}`);
    const lines = `${run.stdout}\n${run.stderr}`.split("\n").filter((l) => /^P\d\d /.test(l));
    assert.ok(lines.length > 5, "preflight produced no rule lines (a zero-output run is never a pass)");
    return { lines, all: `${run.stdout}${run.stderr}` };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const alertLines = (r) => r.lines.filter((l) => l.includes("LC_ALERT") && !l.includes("(rule summary)"));
const status = (l) => l.split(" ")[1];

test("production without any alert channel is a FAIL (nobody would hear the next outage)", () => {
  const l = alertLines(preflight({ env: "production" }));
  assert.equal(l.length, 1, l.join("\n"));
  assert.match(l[0], /^P08 FAIL LC_ALERT_EMAIL or LC_ALERT_WEBHOOK_URL/);
});

test("outside production the same gap is only a WARN", () => {
  const l = alertLines(preflight({ env: "smoke" }));
  assert.equal(l.length, 1, l.join("\n"));
  assert.equal(status(l[0]), "WARN");
});

test("a webhook alone satisfies the rule", () => {
  assert.deepEqual(alertLines(preflight({ env: "production", hook: "https://hooks.fake.invalid/x" })), []);
});

test("an e-mail target with a complete relay (host, username, secret) satisfies the rule", () => {
  assert.deepEqual(alertLines(preflight({ env: "production", email: "owner@fake.invalid" })), []);
});

test("an e-mail target without the relay secret cannot be sent: FAIL in production, WARN elsewhere", () => {
  const prod = alertLines(preflight({ env: "production", email: "owner@fake.invalid", smtpSecret: null }));
  assert.ok(prod.some((l) => status(l) === "FAIL" && l.includes("needs LC_SMTP_HOST")), prod.join("\n"));
  const dev = alertLines(preflight({ env: "smoke", email: "owner@fake.invalid", smtpSecret: "__UNSET__" }));
  assert.ok(dev.some((l) => status(l) === "WARN" && l.includes("needs LC_SMTP_HOST")), dev.join("\n"));
});

test("the example's CHANGE_ME_SENDER placeholder is not a relay: the e-mail target cannot be sent", () => {
  const l = alertLines(preflight({ env: "production", email: "owner@fake.invalid", smtpUser: "CHANGE_ME_SENDER@qq.com" }));
  assert.ok(l.some((x) => status(x) === "FAIL" && x.includes("needs LC_SMTP_HOST")), l.join("\n"));
});

for (const bad of ["not-an-address", "two@fake.invalid,three@fake.invalid", "a b@fake.invalid", "<owner@fake.invalid>", "owner@fake"]) {
  test(`LC_ALERT_EMAIL must be one plain address (${JSON.stringify(bad)})`, () => {
    const r = preflight({ env: "smoke", email: bad });
    assert.ok(alertLines(r).some((l) => status(l) === "FAIL" && l.includes("one plain address")), alertLines(r).join("\n"));
    assert.ok(!r.all.includes(bad.trim()), "preflight must print variable names, never values");
  });
}
