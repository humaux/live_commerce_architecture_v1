// Purpose: independently run real Playwright inbox discovery before expensive browser/Go acceptance.
// Depends on: installed @playwright/test CLI, real playwright.config.ts and real inbox spec files.
// Used by: Node gate; --list collects actual fixtures/tests without browsers, servers, Go or PostgreSQL.
// Invariants: I18 collection success cannot be substituted with source grep or browser-runtime PASS.
import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { basename, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const cli = createRequire(import.meta.url).resolve("@playwright/test/cli");
const uuid = (n: number) => `10000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
type Row = { file: string; title: string; project: string };
function rows(suites: any[]): Row[] {
  return suites.flatMap((suite) => [
    ...(suite.specs ?? []).flatMap((spec: any) =>
      (spec.tests ?? []).map((entry: any) => ({
        file: basename(spec.file ?? suite.file),
        title: spec.title,
        project: entry.projectName ?? "",
      })),
    ),
    ...rows(suite.suites ?? []),
  ]);
}

test("real Playwright --list collects each original INU case and isolated INU09 exactly once", () => {
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    LC_BROWSER_SUITE: "inbox",
    LC_BROWSER_ENGINE: "chromium",
    LC_BROWSER_PUBLIC_ORIGIN: "http://127.0.0.1:3100",
    LC_BROWSER_API_ORIGIN: "http://127.0.0.1:3199",
    LC_BROWSER_EVIDENCE: resolve(root, "output/lc-u2b-inbox-page/protocol-discovery-fixture"),
    LC_BROWSER_INBOX_STORE: uuid(1),
    LC_BROWSER_INBOX_OTHER_STORE: uuid(2),
    LC_BROWSER_INBOX_BUYER_ORIGIN: "https://mock-published.example.test",
    LC_BROWSER_INBOX_IDS: JSON.stringify({ open: uuid(3), closed: uuid(4), instagram: uuid(5), bundle: uuid(6) }),
    LC_BROWSER_INBOX_READER_TOKEN: "MOCK_DISCOVERY_READER",
    LC_BROWSER_INBOX_VIEWER_TOKEN: "MOCK_DISCOVERY_VIEWER",
  };
  // Force JSON to stdout, never the configured runtime artifact or any inherited reporter destination.
  for (const key of ["PLAYWRIGHT_JSON_OUTPUT_FILE", "PLAYWRIGHT_JSON_OUTPUT_NAME", "PLAYWRIGHT_JSON_OUTPUT_DIR"])
    delete env[key];
  const result = spawnSync(
    process.execPath,
    [cli, "test", ...runnerSelectors(), "--list", "--config", resolve(root, "playwright.config.ts"), "--reporter=json"],
    {
      cwd: root,
      env,
      encoding: "utf8",
      timeout: 30_000,
      maxBuffer: 2 * 1024 * 1024,
    },
  );
  assert.equal(result.error, undefined, "real discovery CLI must finish within its 30s bound");
  assert.equal(result.signal, null, "discovery must not be terminated");
  let report: any;
  assert.doesNotThrow(() => {
    report = JSON.parse(result.stdout);
  }, "real CLI JSON report must be readable");
  const diagnostics = (report.errors ?? [])
    .map((error: any) => error.message ?? error.value ?? "collection error")
    .join("\n");
  assert.equal(result.status, 0, `actual Playwright collection failed: ${diagnostics || result.stderr}`);
  assert.deepEqual(report.errors ?? [], [], "actual collector reports no worker-fixture scope errors");
  const discovered = rows(report.suites ?? []);
  // Current real matrix: INU01-05/08 once, INU06/07 in TW/CN/en, plus INU09 once: 12+1.
  assert.equal(discovered.length, 13, "all existing locale scenarios plus protocol scenario are collected");
  const counts = Object.fromEntries(
    Array.from({ length: 9 }, (_value, n) => [`INU${String(n + 1).padStart(2, "0")}`, 0]),
  );
  for (const row of discovered) {
    const id = row.title.match(/^INU\d{2}\b/)?.[0];
    assert.ok(id && Object.hasOwn(counts, id), "every discovered test belongs to the intended INU suite");
    counts[id!]++;
  }
  assert.deepEqual(counts, {
    INU01: 1,
    INU02: 1,
    INU03: 1,
    INU04: 1,
    INU05: 1,
    INU06: 3,
    INU07: 3,
    INU08: 1,
    INU09: 1,
  });
  for (const id of ["INU06", "INU07"])
    for (const locale of ["zh-TW", "zh-CN", "en"]) {
      assert.equal(discovered.filter((row) => row.title.startsWith(`${id} ${locale} `)).length, 1);
    }
  assert.deepEqual([...new Set(discovered.map((row) => row.file))].sort(), [
    "inbox-bundle-ui.spec.ts",
    "inbox-ui.spec.ts",
  ]);
  assert.equal(
    discovered.find((row) => row.title.startsWith("INU09 "))?.file,
    "inbox-bundle-ui.spec.ts",
    "sensitive worker options belong in their own spec file",
  );
  assert.equal(
    new Set(discovered.map((row) => `${row.project}:${row.file}:${row.title}`)).size,
    13,
    "no duplicate collection from shared helpers or config registration",
  );
});

function runnerSelectors(): string[] {
  // Execute the real Go runner's literal selectors too: config alone cannot detect an accidentally filtered-out spec.
  const harness = readFileSync(resolve(root, "tests/foundation/browser_inbox_ui_test.go"), "utf8");
  const call = harness.match(/browser := exec\.CommandContext\(ctx, "pnpm", "exec", "playwright", "test", (.*?)"--reporter=list"/);
  assert.ok(call, "actual browser runner arguments are observable");
  assert.equal(call[1].replace(/"[^"\n]*"|[\s,]/g, ""), "", "only literal runner selectors before reporter");
  return [...call[1].matchAll(/"([^"\n]*)"/g)].map((match) => match[1]);
}
