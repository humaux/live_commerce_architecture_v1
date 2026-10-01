// meta-connect preflight gate (MCG08): deploy/scripts/preflight.sh rules P06 (dependencies) and P08 (grammars) for the merchant
// Page-connect keys COMMERCE_META_LOGIN_CONFIG_ID / _REDIRECT_URI / _GRAPH_VERSION (contract meta-claims-intake-v1 "Merchant connect (R4)";
// unit brief scope 4). Written by the independent test author from the brief + the amendment, not from preflight.sh internals.
// Label: MOCK (static config files only; no host, no secrets, no network, no Docker). preflight prints rule + status + variable NAMES only,
// so the gate reads those lines; the secrets-dir rules (P02/P03) are expected to FAIL in this bare fixture and are ignored.
// Run: node --test tests/deploy/meta-connect-preflight.test.mjs   (also part of scripts/dev/test-node.sh)
import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const deploy = path.join(root, "deploy");
const services = ["api", "admin", "storefront", "payment-worker", "expiry-worker", "meta-worker", "claims-worker", "ads-worker", "caddy", "postgres"];

const GOOD = {
  webhook: "1", identity: "1", profiles: "db,app,payments-sandbox,meta,claims",
  config: "2952863798433821", redirect: "https://admin.localhost/api/meta/callback", graph: "v26.0",
};

function setKey(file, key, value) {
  const text = readFileSync(file, "utf8");
  const line = `${key}=${value}`;
  const re = new RegExp(`^${key}=.*$`, "m");
  writeFileSync(file, re.test(text) ? text.replace(re, line) : `${text.trimEnd()}\n${line}\n`);
}

// run preflight against a temp config built from the shipped *.example files with the given overrides.
function preflight(over = {}) {
  const o = { ...GOOD, ...over };
  const dir = mkdtempSync(path.join(tmpdir(), "lc-mc-pf-"));
  try {
    for (const d of ["env", "secrets", "backup", "state"]) mkdirSync(path.join(dir, d));
    for (const s of services) cpSync(path.join(deploy, "env", `${s}.env.example`), path.join(dir, "env", `${s}.env`));
    setKey(path.join(dir, "env", "claims-worker.env"), "COMMERCE_META_GRAPH_VERSION", "v22.0"); // must parse (unrelated P08)
    setKey(path.join(dir, "env", "api.env"), "COMMERCE_OIDC_CLIENT_ID", "gate-client");
    setKey(path.join(dir, "env", "api.env"), "COMMERCE_META_WEBHOOK_ENABLED", o.webhook);
    setKey(path.join(dir, "env", "api.env"), "COMMERCE_META_LOGIN_CONFIG_ID", o.config);
    setKey(path.join(dir, "env", "api.env"), "COMMERCE_META_LOGIN_REDIRECT_URI", o.redirect);
    setKey(path.join(dir, "env", "api.env"), "COMMERCE_META_LOGIN_GRAPH_VERSION", o.graph);
    const gid = String(process.getgid?.() ?? 0);
    writeFileSync(path.join(dir, "compose.env"), [
      "COMPOSE_PROJECT_NAME=lc-mc-pf", `COMPOSE_PROFILES=${o.profiles}`, "IMAGE_TAG=smoke", "LC_IMAGE_PREFIX=lc", "LC_ENVIRONMENT=smoke",
      "LC_BIND_ADDR=127.0.0.1", "LC_HTTP_PORT=80", "LC_HTTPS_PORT=443", "LC_ADMIN_HOST=admin.localhost", "LC_STORE_HOST=shop.localhost",
      "LC_API_HOST=api.localhost", "LC_HOOKS_HOST=hooks.localhost", "LC_PUBLIC_IP=", `LC_ENV_DIR=${dir}/env`, `LC_SECRETS_DIR=${dir}/secrets`,
      `LC_SECRETS_GID=${gid}`, `LC_BACKUP_DIR=${dir}/backup`, `LC_STATE_DIR=${dir}/state`, "LC_PG_HOST=postgres", "LC_PG_SSLMODE=disable",
      `LC_IDENTITY_ENABLED=${o.identity}`, "LC_OIDC_ISSUER=https://accounts.google.com", "LC_ONBOARDING_ENABLED=0", "LC_ONBOARDING_CURRENCIES=",
      "LC_BUYER_ENABLED=1", "LC_BUYER_SESSION_TTL_SECONDS=3600", "LC_STRIPE_ENABLED=1", "LC_REQUIRE_MEDIA_GATE=0", "LC_REQUIRE_RETENTION_ENFORCED=0",
      "LC_ALERT_WEBHOOK_URL=", "",
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

const loginLines = (r, status) => r.lines.filter((l) => l.includes("META_LOGIN") && l.startsWith(status === undefined ? "" : "") && (status ? / (FAIL|WARN) /.test(l) && l.split(" ")[1] === status : true));
const has = (r, rule, status, needle) => r.lines.some((l) => l.startsWith(`${rule} ${status} `) && l.includes(needle));

test("positive control: a complete, valid configuration raises no META_LOGIN finding", () => {
  const r = preflight();
  assert.deepEqual(r.lines.filter((l) => l.includes("META_LOGIN")), [], `unexpected findings: ${r.lines.filter((l) => l.includes("META_LOGIN"))}`);
});

test("feature off (COMMERCE_META_LOGIN_CONFIG_ID empty): no rule applies even with the other keys empty", () => {
  const r = preflight({ config: "", redirect: "", graph: "" });
  assert.deepEqual(r.lines.filter((l) => l.includes("META_LOGIN")), []);
});

test("P06: the Page route needs the webhook receiver", () => {
  const r = preflight({ webhook: "0" });
  assert.ok(has(r, "P06", "FAIL", "COMMERCE_META_LOGIN_CONFIG_ID"), r.lines.filter((l) => l.startsWith("P06")).join("\n"));
});

test("P06: sessions come from identity", () => {
  const r = preflight({ identity: "0" });
  assert.ok(has(r, "P06", "FAIL", "COMMERCE_META_LOGIN_CONFIG_ID") && r.lines.some((l) => l.includes("LC_IDENTITY_ENABLED")), r.lines.filter((l) => l.startsWith("P06")).join("\n"));
});

test("P06: without the claims profile nobody sends the private replies (a WARN, not a FAIL)", () => {
  const r = preflight({ profiles: "db,app,payments-sandbox,meta" });
  assert.ok(has(r, "P06", "WARN", "COMMERCE_META_LOGIN_CONFIG_ID"), r.lines.filter((l) => l.startsWith("P06")).join("\n"));
  assert.ok(!has(r, "P06", "FAIL", "COMMERCE_META_LOGIN_CONFIG_ID"));
});

for (const bad of ["abc", "12 34", "1".repeat(41) + "x", "0x10", "-1"]) {
  test(`P08: COMMERCE_META_LOGIN_CONFIG_ID must be numeric (${JSON.stringify(bad.slice(0, 12))})`, () => {
    const r = preflight({ config: bad });
    assert.ok(has(r, "P08", "FAIL", "COMMERCE_META_LOGIN_CONFIG_ID"), r.lines.filter((l) => l.startsWith("P08")).join("\n"));
    assert.ok(!r.all.includes(bad.trim()) || bad.length < 4, "preflight must print variable names, never values");
  });
}

for (const [label, redirect] of [
  ["wrong host", "https://evil.example/api/meta/callback"],
  ["http scheme", "http://admin.localhost/api/meta/callback"],
  ["the ads callback path", "https://admin.localhost/api/ads/meta/callback"],
  ["trailing slash", "https://admin.localhost/api/meta/callback/"],
  ["query string", "https://admin.localhost/api/meta/callback?next=https://evil.example"],
  ["empty", ""],
]) {
  test(`P08: COMMERCE_META_LOGIN_REDIRECT_URI must be exactly https://<LC_ADMIN_HOST>/api/meta/callback (${label})`, () => {
    const r = preflight({ redirect });
    assert.ok(has(r, "P08", "FAIL", "COMMERCE_META_LOGIN_REDIRECT_URI"), r.lines.filter((l) => l.startsWith("P08")).join("\n"));
    assert.ok(!r.all.includes("evil.example"), "preflight must not print the value");
  });
}

for (const bad of ["26.0", "v26", "v26.0.1", "latest", "", "v1000.0"]) {
  test(`P08: COMMERCE_META_LOGIN_GRAPH_VERSION grammar vNN.N (${JSON.stringify(bad)})`, () => {
    const r = preflight({ graph: bad });
    assert.ok(has(r, "P08", "FAIL", "COMMERCE_META_LOGIN_GRAPH_VERSION"), r.lines.filter((l) => l.startsWith("P08")).join("\n"));
  });
}

for (const good of ["v26.0", "v22.0", "v99.9"]) {
  test(`P08: COMMERCE_META_LOGIN_GRAPH_VERSION accepts ${good}`, () => {
    const r = preflight({ graph: good });
    assert.ok(!has(r, "P08", "FAIL", "COMMERCE_META_LOGIN_GRAPH_VERSION"));
  });
}
