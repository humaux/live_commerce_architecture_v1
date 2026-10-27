// Purpose: CI-SPEED build-once guard (scripts/dev/prebuilt.mjs + test-local.sh LC_PREBUILT_DIR / --prebuild): a gate job restores the run's prebuilt Next
//   output only for the same commit — another SHA, a malformed stamp or a missing part is refused (fault: a stamp check that ignores the SHA must go red here).
// Depends on: scripts/dev/prebuilt.mjs, scripts/dev/test-local.sh (text), git and tar --zstd in an isolated temp repository.
// Used by: scripts/dev/test-node.sh (Foundation checks), CI.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { checkStamp, pack, unpack } from "../../scripts/dev/prebuilt.mjs";

const A = "a".repeat(40), B = "b".repeat(40);

test("checkStamp: only the same commit with the requested part is accepted", () => {
  assert.equal(checkStamp({ sha: A, parts: ["admin"] }, A, "admin"), null);
  assert.match(checkStamp({ sha: A, parts: ["admin"] }, B, "admin"), /is for a{40}, HEAD is b{40}/, "SHA mismatch must refuse");
  assert.match(checkStamp({ sha: A, parts: ["admin"] }, A, "storefront"), /no storefront part/);
  for (const bad of [null, {}, { sha: "abc", parts: ["admin"] }, { sha: A.toUpperCase(), parts: ["admin"] }]) assert.match(checkStamp(bad, A, "admin"), /missing or malformed/);
});

test("pack/unpack round-trip in a real repository; a new commit refuses the old artifact", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "prebuilt-")), out = path.join(dir, ".prebuilt");
  try {
    const git = (...a) => execFileSync("git", a, { cwd: dir, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
    const put = (f, t) => { mkdirSync(path.dirname(path.join(dir, f)), { recursive: true }); writeFileSync(path.join(dir, f), t); };
    git("init", "-q"); git("config", "user.name", "t"); git("config", "user.email", "t@example.invalid"); git("config", "commit.gpgsign", "false");
    put(".gitignore", ".next/\n.prebuilt/\n"); put("apps/admin/package.json", "{}"); put("apps/storefront/package.json", "{}");
    git("add", "-A"); git("commit", "-qm", "one");
    put("apps/admin/.next/BUILD_ID", "admin-1"); put("apps/admin/.next/cache/big", "never shipped");
    put("apps/storefront/.next/BUILD_ID", "store-1");
    pack(out, ["admin"], dir); pack(out, ["storefront"], dir);
    assert.deepEqual(JSON.parse(readFileSync(path.join(out, "STAMP.json"), "utf8")), { sha: git("rev-parse", "HEAD"), parts: ["admin", "storefront"] }, "a second pack adds to the same commit's stamp");
    rmSync(path.join(dir, "apps/admin/.next"), { recursive: true }); put("apps/admin/.next/STALE", "local leftover");
    unpack(out, "admin", dir);
    assert.equal(readFileSync(path.join(dir, "apps/admin/.next/BUILD_ID"), "utf8"), "admin-1");
    assert.equal(existsSync(path.join(dir, "apps/admin/.next/STALE")), false, "never mixed with a stale local build");
    assert.equal(existsSync(path.join(dir, "apps/admin/.next/cache")), false, ".next/cache is not shipped");
    put("apps/admin/x.ts", "changed"); git("add", "-A"); git("commit", "-qm", "two");
    assert.throws(() => unpack(out, "admin", dir), /refusing prebuilt admin: prebuilt artifact is for [0-9a-f]{40}, HEAD is [0-9a-f]{40}/);
    assert.throws(() => unpack(path.join(dir, "nowhere"), "admin", dir), /missing or malformed STAMP\.json/);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("test-local.sh: LC_PREBUILT_DIR replaces only the two shared builds; --prebuild packs exactly those", () => {
  const sh = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");
  const fn = (name) => new RegExp(`^${name}\\(\\) \\{\\n([\\s\\S]*?)^\\}`, "m").exec(sh)?.[1] ?? "";
  assert.match(fn("lc_build_storefront"), /if \[\[ -n "\$\{LC_PREBUILT_DIR:-\}" \]\]; then command node scripts\/dev\/prebuilt\.mjs unpack "\$LC_PREBUILT_DIR" storefront\n\s+else COMMERCE_BUYER_WEB_ENABLED=0 pnpm run build:storefront; fi/);
  const admin = fn("lc_build_admin");
  assert.match(admin, /if \[\[ -n "\$\{LC_PREBUILT_DIR:-\}" \]\]; then command node scripts\/dev\/prebuilt\.mjs unpack "\$LC_PREBUILT_DIR" admin\n\s+else COMMERCE_IDENTITY_ENABLED=0 COMMERCE_FIXTURE_ENABLED=0 pnpm run build:admin; fi/);
  assert.match(admin, /fi\n[\s\S]*node scripts\/dev\/check-gallery-bundle\.mjs --expect=absent/, "the restored admin bundle is still checked in every job");
  assert.match(sh, /if \[\[ "\$#" == 2 && "\$1" == --prebuild \]\]; then\n\s+unset LC_PREBUILT_DIR; lc_environment; lc_build_storefront; lc_build_admin\n\s+command node scripts\/dev\/prebuilt\.mjs pack "\$2" admin storefront; exit 0/);
});
