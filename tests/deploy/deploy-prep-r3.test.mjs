// Purpose: test R3 operator dispatch, secret custody and deployment config refusals without real services.
// Depends on: node:test, deploy/scripts/{ops-admin,preflight}.sh, Compose; pinned PG image supplies Bash on macOS only.
// Used by: scripts/dev/test-node.sh and deploy-prep-r3 acceptance. Status: MOCK; no DB or provider calls.
import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const deploy = path.join(root, "deploy");
const scratch = path.join(root, "output/deploy-prep-r3");
mkdirSync(scratch, { recursive: true });
function fixture(fn) {
  const dir = mkdtempSync(path.join(scratch, "fixture-"));
  try {
    for (const name of ["env", "secrets", "backup", "state", "bin"]) mkdirSync(path.join(dir, name));
    for (const service of ["api", "admin", "storefront", "payment-worker", "expiry-worker", "meta-worker", "claims-worker", "ads-worker", "caddy", "postgres"])
      cpSync(path.join(deploy, "env", `${service}.env.example`), path.join(dir, "env", `${service}.env`));
    let env = readFileSync(path.join(deploy, "env/compose.env.example"), "utf8");
    for (const [key, value] of Object.entries({ LC_ENV_DIR: `${dir}/env`, LC_SECRETS_DIR: `${dir}/secrets`, LC_BACKUP_DIR: `${dir}/backup`, LC_STATE_DIR: `${dir}/state`, LC_SECRETS_GID: String(process.getgid?.() ?? 0), LC_ENVIRONMENT: "smoke", LC_COMPANY_CONTACT_EMAIL: "contact@example.invalid", LC_PLATFORM_HOST: "platform.localhost", LC_STORE_BASE_DOMAIN: "example.test" }))
      env = new RegExp(`^${key}=.*$`, "m").test(env) ? env.replace(new RegExp(`^${key}=.*$`, "m"), `${key}=${value}`) : `${env}\n${key}=${value}\n`;
    writeFileSync(path.join(dir, "compose.env"), env);
    return fn(dir);
  } finally { rmSync(dir, { recursive: true, force: true }); }
}
function cleanEnv(dir, extra = {}) {
  return { PATH: process.env.PATH, LC_COMPOSE_ENV: `${dir}/compose.env`, ...extra };
}
function run(cmd, args, env) {
  const r = spawnSync(cmd, args, { env, encoding: "utf8", timeout: 60_000 });
  assert.equal(r.error, undefined, `${cmd} did not finish: ${r.error}`);
  return r;
}
function setKey(file, key, value) {
  const s = readFileSync(file, "utf8"), re = new RegExp(`^${key}=.*$`, "m");
  writeFileSync(file, re.test(s) ? s.replace(re, `${key}=${value}`) : `${s}\n${key}=${value}\n`);
}

function preflightRules(dir) {
  const r = run("bash", [`${deploy}/scripts/preflight.sh`, "--skip-images"], cleanEnv(dir, { LC_PREFLIGHT_VERBOSE: "1" }));
  const lines = (r.stdout + r.stderr).split("\n").filter(l => /^P\d+ (PASS|FAIL|WARN) /.test(l));
  assert.ok(lines.length > 5, "preflight must evaluate rules, not silently exit");
  return lines;
}

test("P02 settlements directory rejects missing/link/owner/group/mode with a valid control", () => {
  const text = readFileSync(`${deploy}/scripts/preflight.sh`, "utf8");
  const code = text.slice(text.indexOf("export_dir ="), text.indexOf('gid = int(E.get("LC_SECRETS_GID"'));
  assert.ok(code.includes('rec("P02"'), "P02 directory rule missing");
  for (const label of ["valid", "missing", "link", "owner", "group", "mode"]) fixture(dir => {
    const target = `${dir}/state/settlements`;
    if (label !== "missing") {
      if (label === "link") {
        mkdirSync(`${dir}/state/real`, { mode: 0o700 });
        symlinkSync(`${dir}/state/real`, target);
      } else mkdirSync(target, { mode: 0o700 });
      if (label === "mode") chmodSync(target, 0o755);
    }
    // Actual existence/link/mode; MOCK only uid/gid because the local test user cannot chown to 65532.
    const script = `import os,stat,json\nfrom types import SimpleNamespace\nE={"LC_STATE_DIR":${JSON.stringify(`${dir}/state`)}}\ntarget=${JSON.stringify(target)}\noriginal_stat=os.stat\ndef scoped_stat(p,*a,**kw):\n s=original_stat(p,*a,**kw)\n if str(p)==target: return SimpleNamespace(st_mode=s.st_mode,st_uid=${label === "owner" ? 123 : 65532},st_gid=${label === "group" ? 123 : 65532})\n return s\nos.stat=scoped_stat\nrows=[]\ndef rec(rule,ok,name): rows.append([rule,ok,name])\n${code}\nprint(json.dumps(rows))\n`;
    const r = run("python3", ["-c", script], cleanEnv(dir));
    assert.equal(r.status, 0, r.stderr);
    const rows = JSON.parse(r.stdout);
    assert.equal(rows.length, 1);
    assert.equal(rows[0][0], "P02");
    assert.equal(rows[0][1], label === "valid", label);
  });
});

test("preflight health permission grammars and API/worker consistency fail closed", () => fixture(dir => {
  const api = `${dir}/env/api.env`, worker = `${dir}/env/claims-worker.env`;
  const findings = () => preflightRules(dir).filter(l => /ADVANCED_ACCESS|capability flags match/.test(l));
  assert.ok(findings().every(l => l.includes(" PASS ")), "empty pre-review defaults must pass");
  for (const bad of ["Pages_read", "pages.read", "pages_read,,pages_write", "pages_read, pages_write", " pages_read"]) {
    setKey(api, "COMMERCE_META_ADVANCED_ACCESS", bad);
    assert.ok(findings().some(l => l.startsWith("P08 FAIL COMMERCE_META_ADVANCED_ACCESS")), "invalid grammar accepted");
  }
  setKey(api, "COMMERCE_META_ADVANCED_ACCESS", "pages_read,pages_write");
  assert.ok(findings().some(l => l.startsWith("P08 FAIL Meta health capability flags match")), "permission mismatch accepted");
  setKey(worker, "COMMERCE_META_ADVANCED_ACCESS", "pages_read,pages_write");
  assert.ok(findings().every(l => l.includes(" PASS ")), "matching valid permissions refused");
  setKey(api, "COMMERCE_META_DM_RECEIVER_CONFIRMED", "1");
  assert.ok(findings().some(l => l.startsWith("P08 FAIL Meta health capability flags match")), "DM mismatch accepted");
}));

test("merchant alert flag is explicit 0|1 and independently requires SMTP", () => fixture(dir => {
  const f = `${dir}/compose.env`;
  for (const [k,v] of Object.entries({ LC_BUYER_MAIL_ENABLED: "0", LC_PASSWORD_LOGIN_ENABLED: "0", LC_ALERT_EMAIL: "", LC_ALERT_WEBHOOK_URL: "", LC_SMTP_HOST: "", LC_SMTP_USERNAME: "", LC_MAIL_FROM: "" })) setKey(f,k,v);
  const rules = () => preflightRules(dir);
  assert.ok(rules().some(l => l.startsWith("P06 PASS LC_MERCHANT_ALERT_MAIL")), "default-off control missing");
  for (const v of ["", "yes", "01", "-1"]) {
    setKey(f,"LC_MERCHANT_ALERT_MAIL",v);
    assert.ok(rules().some(l => l.startsWith("P06 FAIL LC_MERCHANT_ALERT_MAIL")), "invalid opt-in flag accepted");
  }
  setKey(f,"LC_MERCHANT_ALERT_MAIL","1");
  assert.ok(rules().some(l => l.startsWith("P08 FAIL LC_SMTP_HOST")), "merchant-only mail skipped SMTP validation");
  setKey(f,"LC_SMTP_HOST","smtp.example.test"); setKey(f,"LC_SMTP_USERNAME","sender@example.test"); setKey(f,"LC_MAIL_FROM","sender@example.test");
  assert.ok(rules().some(l => l.startsWith("P09 FAIL commerce_smtp_password")), "missing SMTP secret accepted");
  writeFileSync(`${dir}/secrets/commerce_smtp_password`,"fixture-smtp-config-only-password\n");
  assert.ok(rules().some(l => l.startsWith("P09 PASS commerce_smtp_password")), "valid merchant-only SMTP control refused");
}));

// Keep the original YAML for the export bind: compose-go's older JSON encoder omits false,
// while omitting create_host_path in YAML can default to true. Never infer safety from JSON alone.
function exportBindYAML() {
  const stripe = readFileSync(`${deploy}/compose.yml`, "utf8").match(/^  stripe-admin:\n[\s\S]*?(?=^  [a-z0-9-]+:)/m)?.[0];
  const mount = stripe?.match(/^    volumes:\n((?:^ {6,}.*\n)+)/m)?.[1];
  assert.ok(mount, "stripe-admin export bind missing");
  assert.equal((mount.match(/^\s+target:/gm) ?? []).length, 1, "probe must copy only the export bind");
  assert.match(mount, /^\s+target: \/exports$/m);
  return mount;
}
function assertExportBind(v, yaml) {
  assert.equal(v.type, "bind");
  assert.ok(v.bind, "normalized bind options missing");
  assert.ok(v.bind.create_host_path === false || v.bind.create_host_path === undefined, "automatic bind creation is unsafe");
  assert.match(yaml, /^\s+(?:bind:\s*\{\s*)?create_host_path:\s*false(?:\s*\})?\s*$/m, "source must explicitly disable host-path creation");
}

test("shared loader makes API/worker LIVE pair file-only before Compose interpolation", () => fixture(dir => {
  const file = `${dir}/compose.env`;
  const render = (callerFlag = "1") => {
    const r = run("bash", ["-c", 'source "$DEPLOY/scripts/lib.sh"; lc_load_env "$LC_COMPOSE_ENV"; lc_compose_all config --format json'], cleanEnv(dir, {
      DEPLOY: deploy, LC_STRIPE_LIVE_ENABLED: callerFlag, LC_STRIPE_LIVE_APPROVAL_REF: "MOCK_CALLER_APPROVAL",
      IMAGE_TAG: "caller-tag",
    }));
    assert.equal(r.status, 0, r.stderr);
    return JSON.parse(r.stdout).services;
  };
  const assertPair = (services, flag, reference) => {
    for (const name of ["api", "payment-worker-live"]) {
      assert.equal(services[name].environment.COMMERCE_STRIPE_LIVE_ENABLED, flag, name);
      assert.equal(services[name].environment.COMMERCE_STRIPE_LIVE_APPROVAL_REF, reference, name);
    }
    assert.equal(services.api.image, "lc-go:caller-tag", "ordinary environment override must remain intact");
  };
  setKey(file, "LC_STRIPE_LIVE_ENABLED", "0");
  setKey(file, "LC_STRIPE_LIVE_APPROVAL_REF", "");
  assertPair(render(), "0", "");
  writeFileSync(file, readFileSync(file, "utf8").replace(/^LC_STRIPE_LIVE_(ENABLED|APPROVAL_REF)=.*\n/gm, ""));
  assertPair(render(), "0", "");
  setKey(file, "LC_STRIPE_LIVE_ENABLED", "1");
  setKey(file, "LC_STRIPE_LIVE_APPROVAL_REF", "MOCK_FILE_APPROVAL");
  assertPair(render("0"), "1", "MOCK_FILE_APPROVAL");
  const readonly = run("bash", ["-c", 'source "$DEPLOY/scripts/lib.sh"; readonly LC_STRIPE_LIVE_ENABLED; lc_load_env "$LC_COMPOSE_ENV"'], cleanEnv(dir, {
    DEPLOY: deploy, LC_STRIPE_LIVE_ENABLED: "1",
  }));
  assert.notEqual(readonly.status, 0, "readonly caller state must fail closed");
  assert.ok(readonly.stderr.includes("cannot clear caller Stripe LIVE pair"));
}));

test("export bind accepts omitted JSON false but rejects unsafe source/config mutations", () => {
  const yaml = exportBindYAML();
  for (const bind of [{ create_host_path: false }, {}])
    assert.doesNotThrow(() => assertExportBind({ type: "bind", bind }, yaml));
  assert.throws(() => assertExportBind({ type: "bind", bind: { create_host_path: true } }, yaml));
  assert.throws(() => assertExportBind({ type: "bind", bind: {} }, yaml.replace("create_host_path: false", "create_host_path: true")));
  assert.throws(() => assertExportBind({ type: "bind", bind: {} }, yaml.replace(/^.*create_host_path.*\n/m, "")));
  assert.throws(() => assertExportBind({ type: "bind" }, yaml));
});

test("sanctioned operator allowlist and fail-closed gates (fake docker, no network)", () => fixture((dir) => {
  writeFileSync(`${dir}/bin/docker`, '#!/usr/bin/env bash\nprintf "%s\\n" "$@" >"$STUB_LOG"\nif [[ "${CHECK_FILE_PAIR:-0}" == 1 ]]; then [[ "${COMMERCE_STRIPE_LIVE_ENABLED:-}" == 1 && "${COMMERCE_STRIPE_LIVE_APPROVAL_REF:-}" == MOCK_FILE_APPROVAL_R3 ]] || exit 9; fi\nexit "${STUB_EXIT:-0}"\n', { mode: 0o755 });
  // Extract only fixture construction; never execute full smoke. Container-local storage avoids
  // Docker Desktop's macOS UID mapping, so ownership is checked with actual Linux stat.
  const makeConfig = readFileSync(`${deploy}/scripts/smoke.sh`, "utf8").match(/^make_config\(\) \{[\s\S]*?^\}/m)?.[0];
  assert.ok(makeConfig, "smoke fixture constructor missing");
  writeFileSync(`${dir}/make-config.sh`, makeConfig);
  // One bounded Linux process supplies Bash and UID checks. Docker INSIDE it is always the stub.
  writeFileSync(`${dir}/cases.sh`, `set -u
export PATH="$F/bin:$PATH" LC_COMPOSE_ENV="$F/compose.env" LC_STATE_DIR="$F/state" STUB_LOG="$F/argv"
bad=0
check() {
  local want=$1; shift
  rm -f "$STUB_LOG"
  bash "$REPO/deploy/scripts/ops-admin.sh" "$@" </dev/null >"$F/out" 2>"$F/err"; local got=$?
  if [[ "$got" != "$want" ]]; then echo "FAIL $* expected=$want got=$got"; cat "$F/err"; bad=1; fi
  if [[ "$want" != 0 && "$want" != 7 && -f "$STUB_LOG" ]]; then echo 'FAIL docker ran after refusal'; bad=1; fi
}
for sub in store-suspend store-resume tenant-suspend tenant-resume status audit support-grant support-revoke support-list support-principal-add support-principal-revoke; do
  check 0 platform-admin "$sub"
  invoked=$(tail -n 3 "$STUB_LOG")
  expected=$(printf 'platform-admin\n/app/bin/platform-admin\n%s' "$sub")
  if [[ "$invoked" != "$expected" ]]; then echo "FAIL wrong docker service/binary for $sub"; bad=1; fi
done
for sub in platform-designate platform-open platform-close platform-allow platform-disallow platform-block platform-unblock settlement-close settlement-payout settlement-resolve; do
  check 0 stripe-admin "$sub"
done
for tool in platform-admin stripe-admin store-admin meta-admin unknown-admin; do check 2 "$tool" unknown; done
check 1 stripe-admin settlement-sync --environment SANDBOX
export STRIPE_SANDBOX=1
check 0 stripe-admin settlement-sync --environment SANDBOX
grep -qx STRIPE_SANDBOX "$STUB_LOG" || { echo 'FAIL sandbox opt-in not forwarded'; bad=1; }
for flags in '--environment LIVE' '--environment=LIVE' '-environment=live'; do
  check 1 stripe-admin settlement-sync $flags
  check 1 stripe-admin platform-open $flags
done
# Caller exports never supply approval; missing, zero and malformed file pairs refuse it.
export LC_STRIPE_LIVE_ENABLED=1 LC_STRIPE_LIVE_APPROVAL_REF=MOCK_APPROVAL_R3
check 1 stripe-admin settlement-sync --environment LIVE
check 1 stripe-admin platform-open --environment LIVE
sed -i '/^LC_STRIPE_LIVE_ENABLED=/d; /^LC_STRIPE_LIVE_APPROVAL_REF=/d' "$LC_COMPOSE_ENV"
check 1 stripe-admin settlement-sync --environment LIVE
printf 'LC_STRIPE_LIVE_ENABLED=1\nLC_STRIPE_LIVE_APPROVAL_REF=bad\n' >>"$LC_COMPOSE_ENV"
check 1 stripe-admin settlement-sync --environment LIVE
sed -i 's/^LC_STRIPE_LIVE_APPROVAL_REF=.*/LC_STRIPE_LIVE_APPROVAL_REF=MOCK_FILE_APPROVAL_R3/' "$LC_COMPOSE_ENV"
export LC_STRIPE_LIVE_ENABLED=0 LC_STRIPE_LIVE_APPROVAL_REF=MOCK_CALLER_REF
export COMMERCE_STRIPE_LIVE_ENABLED=0 COMMERCE_STRIPE_LIVE_APPROVAL_REF=MOCK_CALLER_REF
export CHECK_FILE_PAIR=1
check 0 stripe-admin settlement-sync --environment LIVE
check 0 stripe-admin platform-open --environment LIVE
unset CHECK_FILE_PAIR
grep -qx COMMERCE_STRIPE_LIVE_APPROVAL_REF "$STUB_LOG" || { echo 'FAIL file pair not forwarded by name'; bad=1; }
if grep -qE 'MOCK_FILE_APPROVAL_R3|MOCK_CALLER_REF' "$STUB_LOG"; then echo 'FAIL approval value in argv'; bad=1; fi
unset LC_STRIPE_LIVE_ENABLED LC_STRIPE_LIVE_APPROVAL_REF COMMERCE_STRIPE_LIVE_ENABLED COMMERCE_STRIPE_LIVE_APPROVAL_REF
sed -i 's/^LC_STRIPE_LIVE_ENABLED=.*/LC_STRIPE_LIVE_ENABLED=0/; s/^LC_STRIPE_LIVE_APPROVAL_REF=.*/LC_STRIPE_LIVE_APPROVAL_REF=/' "$LC_COMPOSE_ENV"
check 1 stripe-admin settlement-export --statement 00000000-0000-4000-8000-000000000001
for out in /tmp/report.csv /exports/../report.csv /exports/sub/report.csv; do check 1 stripe-admin settlement-export --out "$out"; done
check 0 stripe-admin settlement-export --out /exports/report.csv
check 0 stripe-admin settlement-export --out=/exports/report2.csv
check 1 stripe-admin settlement-export --out /exports/one.csv --out=/exports/two.csv
export STUB_EXIT=7
check 7 platform-admin audit
unset STUB_EXIT
audit_ok() {
  [[ -s "$1" ]] || return 1
  grep -q 'tool=platform-admin sub=audit .*exit=7' "$1" || return 1
  ! grep -qE -- '--out|report[.]csv|00000000' "$1"
}
if ! audit_ok "$F/state/ops-admin.log"; then echo 'FAIL missing/invalid/leaking audit log'; bad=1; fi
if audit_ok "$F/state/no-log"; then echo 'FAIL missing audit log accepted'; bad=1; fi
source "$F/make-config.sh"
export LC_DEPLOY_DIR="$REPO/deploy"
generated=$(mktemp -d)
make_config "$generated/config" full || bad=1
if [[ ! -d "$generated/config/state/settlements" || "$(stat -c '%u:%g:%a' "$generated/config/state/settlements" 2>/dev/null)" != 65532:65532:700 ]]; then
  echo 'FAIL full smoke export directory absent or unsafe'; bad=1
fi
rm -rf "$generated"
exit "$bad"
`);
  const image = readFileSync(`${deploy}/compose.yml`, "utf8").match(/image: (postgres@sha256:[a-f0-9]{64})/)[1];
  const r = run("docker", ["run", "--rm", "--network", "none", "--memory", "64m", "--pids-limit", "64", "-v", `${root}:/repo:ro`, "-v", `${dir}:/fixture`, "-e", "F=/fixture", "-e", "REPO=/repo", "--entrypoint", "bash", image, "/fixture/cases.sh"], cleanEnv(dir));
  assert.equal(r.status, 0, r.stdout + r.stderr);
}));

test("platform operator custody, export mount and binary are wired", () => fixture((dir) => {
  const r = run("docker", ["compose", "--project-directory", deploy, "--env-file", `${dir}/compose.env`, "-f", `${deploy}/compose.yml`, "--profile", "ops", "--profile", "app", "config", "--format", "json"], cleanEnv(dir));
  assert.equal(r.status, 0, r.stderr);
  const services = JSON.parse(r.stdout).services;
  assert.equal(services["expiry-worker"].environment.COMMERCE_MERCHANT_ALERT_MAIL, "0");
  setKey(`${dir}/compose.env`, "LC_MERCHANT_ALERT_MAIL", "1");
  const on = run("docker", ["compose", "--project-directory", deploy, "--env-file", `${dir}/compose.env`, "-f", `${deploy}/compose.yml`, "--profile", "ops", "--profile", "app", "config", "--format", "json"], cleanEnv(dir));
  assert.equal(on.status, 0, on.stderr);
  assert.equal(JSON.parse(on.stdout).services["expiry-worker"].environment.COMMERCE_MERCHANT_ALERT_MAIL, "1");
  setKey(`${dir}/compose.env`, "LC_MERCHANT_ALERT_MAIL", "0");
  const op = services["platform-admin"];
  assert.ok(op, "missing platform-admin service");
  assert.deepEqual(op.profiles, ["ops"]);
  assert.equal(op.restart, "no");
  assert.deepEqual(Object.keys(op.networks), ["backend"]);
  assert.equal(op.environment.COMMERCE_PLATFORM_OPERATOR_DATABASE_URL_FILE, "/run/secrets/dsn_lc_platform_operator");
  assert.deepEqual(op.secrets.map(s => s.source), ["dsn_lc_platform_operator"]);
  for (const [name, s] of Object.entries(services)) if (name !== "platform-admin")
    assert.ok(!(s.secrets ?? []).some(x => x.source === "dsn_lc_platform_operator"), `${name} has operator authority`);
  const v = services["stripe-admin"].volumes.find(v => v.target === "/exports");
  assert.equal(v.source, `${dir}/state/settlements`);
  assert.equal(v.read_only ?? false, false);
  assertExportBind(v, exportBindYAML());
  for (const [file, needle] of [
    ["docker/go.Dockerfile", /ARG GO_CMDS="[^"]*\bplatform-admin\b/],
    ["postgres/logins.tsv", /lc_platform_operator\tcommerce_platform_operator\tinherit_noset\tcore\tplatform-admin:COMMERCE_PLATFORM_OPERATOR_DATABASE_URL/],
    ["secrets.manifest.tsv", /dsn_lc_platform_operator\tdsn_tcp\tderive\tplatform-admin\t/],
    ["scripts/lib.sh", /LC_ONESHOT_SERVICES=\([^)]*\bplatform-admin\b/],
    ["scripts/host-setup.sh", /mkdirp 0700 65532 65532 "\$state\/settlements"/],
    ["scripts/smoke.sh", /chown 65532:65532 "\$dir\/state\/settlements"/],
  ]) assert.match(readFileSync(`${deploy}/${file}`, "utf8"), needle);
}));

test("actual export bind refuses a missing directory; enabling auto-create is detected", () => fixture((dir) => {
  const image = readFileSync(`${deploy}/compose.yml`, "utf8").match(/image: (postgres@sha256:[a-f0-9]{64})/)[1];
  const source = `${dir}/state/settlements`;
  const project = `lc-r3-bind-${path.basename(dir).toLowerCase()}`;
  const file = `${dir}/bind-probe.yml`;
  const args = ["compose", "--project-name", project, "--env-file", `${dir}/compose.env`, "-f", file];
  const yaml = exportBindYAML();
  // Only Docker create: no PostgreSQL process, build, pull, secret, network or deployed service.
  for (const [label, mount] of [["source", yaml], ["unsafe mutation", yaml.replace("create_host_path: false", "create_host_path: true")]]) {
    writeFileSync(file, `services:\n  export-probe:\n    image: ${image}\n    network_mode: none\n    mem_limit: 64m\n    pids_limit: 64\n    volumes:\n${mount}`);
    assert.equal(existsSync(source), false, "probe source must be absent before create");
    try {
      const r = run("docker", [...args, "create", "--pull", "never", "--no-build"], cleanEnv(dir));
      const refusesMissingSource = r.status !== 0 && /bind source path does not exist|bind source path .*does not exist/.test(r.stdout + r.stderr);
      if (label === "source") {
        assert.ok(refusesMissingSource, "explicit false must refuse the missing source, not fail for an unrelated reason");
        assert.equal(existsSync(source), false, "safe config must not create the source");
      } else {
        assert.equal(r.status, 0, r.stdout + r.stderr);
        assert.equal(refusesMissingSource, false, "auto-create mutation must fail the safety predicate");
      }
    } finally {
      const cleanup = run("docker", [...args, "down", "--volumes", "--remove-orphans"], cleanEnv(dir));
      assert.equal(cleanup.status, 0, cleanup.stderr);
      // Only this fixture's path; Docker Desktop may not expose a daemon-created path on the host.
      rmSync(source, { recursive: true, force: true });
    }
  }
}));

test("R3 preflight rejects cancelled PAYUNi, validates Page app and keeps LIVE pair mandatory", () => fixture((dir) => {
  const api = `${dir}/env/api.env`;
  const preflight = () => {
    const r = run("bash", [`${deploy}/scripts/preflight.sh`, "--skip-images"], cleanEnv(dir));
    assert.notEqual(r.status, null);
    const findings = (r.stdout + r.stderr).split("\n").filter(l => /^(P06|P08) (FAIL|WARN) /.test(l));
    assert.ok((r.stdout + r.stderr).includes("P06"), "preflight must produce rule output");
    return { r, findings };
  };
  assert.match(readFileSync(api, "utf8"), /^COMMERCE_PAYUNI_NOTIFY_ENABLED=0$/m);
  const claims = `${dir}/env/claims-worker.env`;
  assert.match(readFileSync(claims, "utf8"), /^COMMERCE_META_PAGE_APP_ID=$/m);
  const baseline = preflight();
  assert.ok(!baseline.findings.some(l => /PAYUNI|PAGE_APP_ID/.test(l)), baseline.findings.join("\n"));
  setKey(api, "COMMERCE_PAYUNI_NOTIFY_ENABLED", "1");
  let r = preflight();
  assert.notEqual(r.r.status, 0);
  assert.ok(r.findings.some(l => l.startsWith("P06 FAIL") && l.includes("COMMERCE_PAYUNI_NOTIFY_ENABLED")));
  setKey(api, "COMMERCE_PAYUNI_NOTIFY_ENABLED", "0");
  setKey(claims, "COMMERCE_META_PAGE_APP_ID", "bad-test-value");
  r = preflight();
  assert.ok(r.findings.some(l => l.startsWith("P08 FAIL") && l.includes("COMMERCE_META_PAGE_APP_ID")));
  assert.ok(!(r.r.stdout + r.r.stderr).includes("bad-test-value"), "config value leaked");
  setKey(claims, "COMMERCE_META_PAGE_APP_ID", "123456789");
  r = preflight();
  assert.ok(!r.findings.some(l => l.includes("COMMERCE_META_PAGE_APP_ID")), "valid optional worker app id must be admitted");
  setKey(claims, "COMMERCE_META_PAGE_APP_ID", "");
  setKey(api, "COMMERCE_PAYMENT_PROFILE", "LIVE");
  setKey(`${dir}/compose.env`, "LC_STRIPE_ENABLED", "1");
  r = preflight();
  assert.ok(r.findings.some(l => l.startsWith("P06 FAIL") && l.includes("LC_STRIPE_LIVE_ENABLED")));
}));

test("R3 smoke exercises exact denied probes, accepts 404 only for disabled mounts", () => {
  const r = run("bash", ["-c", `
source "$REPO/deploy/scripts/smoke-r3.sh" || exit 1
lc_r3_request() {
  case "$1:$2" in
    POST:*/claims/simulate) [[ "$3" == '{"comment":"A1"}' && -z "$4" ]] || return 1 ;;
    POST:*/ads/meta/unbind) [[ "$3" == '{"ad_account_id":"9001"}' && "$4" == smoke-r3-unbind ]] || return 1 ;;
    GET:*) [[ -z "$3" && -z "$4" ]] || return 1 ;;
    *) return 1 ;;
  esac
  if [[ "$1:$2" == "$FAIL_PROBE" ]]; then echo "$FAIL_STATUS"; return; fi
  case "$2" in */ads/*) echo "$ADS_STATUS" ;; */claims/simulate) echo "$CLAIMS_STATUS" ;; *) echo 401 ;; esac
}
bad=0
FAIL_PROBE= FAIL_STATUS= ADS_STATUS=404 CLAIMS_STATUS=401
out=$(lc_r3_smoke 1 0) || exit 1
[[ $(echo "$out" | wc -l | tr -d ' ') == 6 ]] || exit 1
ADS_STATUS=401; lc_r3_smoke 1 1 >/dev/null || exit 1
CLAIMS_STATUS=404; lc_r3_smoke 0 1 >/dev/null || exit 1
CLAIMS_STATUS=401
for route in operations returns settlements; do
  FAIL_PROBE=GET:/v1/admin/stores/00000000-0000-4000-8000-000000000001/$route
  for FAIL_STATUS in 200 404 403 422 ''; do
    if lc_r3_smoke 1 1 >/dev/null; then echo "false pass $route $FAIL_STATUS"; bad=1; fi
  done
done
FAIL_PROBE=; ADS_STATUS=404
if lc_r3_smoke 1 1 >/dev/null; then echo 'false pass missing ads mount'; bad=1; fi
ADS_STATUS=401; CLAIMS_STATUS=404
if lc_r3_smoke 1 1 >/dev/null; then echo 'false pass missing simulate mount'; bad=1; fi
exit "$bad"
`], { PATH: process.env.PATH, REPO: root });
  assert.equal(r.status, 0, r.stdout + r.stderr);
});
