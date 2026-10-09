// Purpose: independent W3-U2 negative grammar, locale and automatic PR-mode coverage checks.
// Depends on: actual live-settings model/copy and the real CLI mode planner.
// Used by: test-node.sh and --browser-live-settings's preflight; no backend stand-ins.
import assert from "node:assert/strict";
import { test } from "node:test";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import {
  settingsBody,
  settingsData,
  settingsQuery,
  settingsResource,
  validSoldOutText,
} from "../../apps/admin/lib/live-settings-model.ts";
import { liveSettingsCopy } from "../../apps/admin/lib/live-settings-copy.ts";
import { modeEntries, planPr } from "../../scripts/dev/pr-modes.mjs";
const sid = "a1111111-1111-4111-8111-111111111111";
test("only frozen session-scoped manual operations are available", () => {
  assert.deepEqual(settingsResource(`live-sessions/${sid}/reminders`), {
    kind: "reminders",
    methods: ["GET", "POST"],
  });
  for (const p of [
    "live-settings/reminder",
    `live-sessions/${sid}/claims/blocklist/actor-key`,
    `live-sessions/${sid}/reminders/automatic`,
  ])
    assert.equal(settingsResource(p), null);
  assert.equal(
    settingsQuery("blocklist", "GET", new URL("https://x/?note=PRIVATE")),
    false,
  );
  assert.equal(
    settingsQuery(
      "check",
      "GET",
      new URL(`https://x/?bundle_id=${sid}&bundle_id=${sid}`),
    ),
    false,
  );
  assert.equal(
    settingsBody(
      "settings",
      JSON.stringify({
        enabled: true,
        template_id: "sold-out-reply/v1",
        template_version: 1,
        expected_version: 0,
      }),
    ),
    false,
  );
});
test("private notes and sold-out drafts respect rune limits and closed placeholders", () => {
  assert.equal(
    settingsBody(
      "blocklist",
      JSON.stringify({ bundle_id: sid, note: "漢".repeat(200) }),
    ),
    true,
  );
  for (const note of ["漢".repeat(201), "note\ncontrol"])
    assert.equal(
      settingsBody("blocklist", JSON.stringify({ bundle_id: sid, note })),
      false,
    );
  assert.equal(
    settingsBody(
      "blocklist",
      JSON.stringify({ bundle_id: sid, actor_key: "forged" }),
    ),
    false,
  );
  assert.equal(validSoldOutText("{{product.name}} 已售完"), true);
  for (const body of [
    "{{buyer.name}}",
    "{{product.name}} {{product.name}}",
    "x\ny",
    "字".repeat(281),
  ])
    assert.equal(validSoldOutText(body), false);
});
test("restricted rows can never advertise copying, and checkout links cannot carry bearer data", () => {
  const report = {
    sent: [],
    queued: 0,
    failed: [],
    followup: [
      {
        bundle_id: sid,
        display_name: null,
        reason: "restricted",
        reminder_state: "claimed",
        link_copy_allowed: true,
      },
    ],
    link: "https://shop.example/zh-TW/checkout",
  };
  assert.equal(
    (
      settingsData("reminders", "GET", report) as {
        followup: { link_copy_allowed: boolean }[];
      }
    ).followup[0].link_copy_allowed,
    false,
  );
  for (const link of [
    "javascript:alert(1)",
    "https://shop.example/checkout#t=secret",
    "https://shop.example/checkout?name=buyer",
  ])
    assert.throws(() => settingsData("reminders", "GET", { ...report, link }));
});
test("three admin locales preserve the mandatory policy copy and real PR planner includes this new mode", () => {
  assert.equal(
    liveSettingsCopy("zh-TW").rule,
    "只有在 24 小時內傳過訊息給粉專的買家會收到提醒；其他買家會列在待跟進清單",
  );
  assert.equal(
    liveSettingsCopy("zh-TW").quota,
    "會用掉這則留言唯一一次私訊回覆",
  );
  for (const locale of ["zh-TW", "zh-CN", "en"])
    assert.ok(liveSettingsCopy(locale).unknown.length > 20);
  assert.ok(
    planPr(["apps/admin/components/LiveSettings.tsx"]).modes.includes(
      "--browser-live-settings",
    ),
  );
});

test("live-settings is one inert registry entry with the existing admin/PG command", () => {
  const source = readFileSync("scripts/dev/test-local.sh", "utf8");
  assert.equal(modeEntries(source).filter(e => e.name === "--browser-live-settings").length, 1);
  const result = spawnSync("/bin/bash", ["scripts/dev/test-local.sh", "--dry-run", "--browser-live-settings"], {
    encoding: "utf8", env: {PATH: "/usr/bin:/bin"},
  });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /mode=--browser-live-settings build=admin fixture=pg/);
  assert.match(result.stdout, /LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE=1 GOTOOLCHAIN=go1\.27\.2 go test -race -tags browser -count=1 -timeout=780s -run '\^TestBrowserLiveSettingsUIRealChain\$'/);
  assert.match(result.stdout, /pnpm run build:admin/);
  assert.doesNotMatch(result.stdout, /pnpm run build:storefront/);
  assert.match(result.stdout, /live-settings-bff\.test\.ts tests\/admin\/live-settings-model\.test\.ts/);
});
