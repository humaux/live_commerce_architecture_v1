// Purpose: Detect claim-mode drift between Go, SQL, both admin read models and the mode picker/prompts.
// Depends on: node:test/assert/fs, internal/claims/claims.go, migration 0115, claims-model/copy and console-model.
// Used by: scripts/dev/test-node.sh; D3 regression before the GitHub real-click gates.
import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import * as claims from "../../apps/admin/lib/claims-model.ts";
import { hostPrompt, hostPromptLanguages } from "../../apps/admin/lib/claims-copy.ts";
import { parseClaimSource } from "../../apps/admin/lib/claim-source-model.ts";

const src = (path: string) => readFile(new URL(`../../${path}`, import.meta.url), "utf8");
const sid = "22222222-2222-4222-8222-222222222222";
test("claims window modes match Go constants and every SQL match_mode CHECK", async () => {
  const go = await src("internal/claims/claims.go");
  const modes = [...go.matchAll(/^\s*Match\w+\s+MatchMode\s*=\s*"([A-Z_]+)"/gm)].map((m) => m[1]);
  assert.equal(modes.length, 3);
  const sql = await src("migrations/0115_claims_contains_mode.sql");
  const checks = [...sql.matchAll(/CHECK\s*\(\s*match_mode\s+IN\s*\(([^)]+)\)\s*\)/gi)];
  assert.equal(checks.length, 3, "window, event and interval constraints must all be checked");
  for (const check of checks)
    assert.deepEqual([...check[1].matchAll(/'([^']+)'/g)].map((m) => m[1]).sort(), [...modes].sort());
  for (const mode of modes) {
    const value = { session_id: sid, state: "CLOSED", match_mode: mode, generation: 0, version: 1, opened_at: null, closed_at: null };
    assert.deepEqual(claims.parseWindow(value, sid), value, `Go supports ${mode}; Claims must read it`);
  }
  assert.deepEqual([...claims.matchModes].sort(), [...modes].sort());
  const picker = await src("apps/admin/components/StudioClaims.tsx");
  const options = [...picker.matchAll(/<option value="(EXACT|KEYWORD_QTY_[A-Z_]+)"/g)].map((m) => m[1]);
  assert.deepEqual(options.sort(), [...modes].sort(), "every backend mode is selectable, without UI-only modes");
  for (const mode of ["CONTAINS", "KEYWORD_ONLY", "", null])
    assert.throws(() => claims.parseWindow({ session_id: sid, state: "CLOSED", match_mode: mode, generation: 0, version: 1, opened_at: null, closed_at: null }, sid));
});

test("contains prompt requires explicit quantity and removes the exact-only instruction", () => {
  assert.equal(hostPrompt("zh-TW", "KEYWORD_QTY_CONTAINS", "A1"), "留言「A1+數量」就能登記，例如「我要A1+2」；只留 A1 不會登記；一則留言只寫一個商品，不要問句；A1+2 = 數量改成 2 件（不是再加 2 件）。之後再留言，以最新數量為準。留言不代表已保留庫存，結帳時才確認。");
  assert.equal(hostPrompt("en", "KEYWORD_QTY_CONTAINS", "A1"), "Comment A1+quantity, e.g. \"A1+2\"; A1 alone is not counted; one item per comment, no questions; A1+2 = set your quantity to 2 (it does not add 2 more). Your latest comment replaces the earlier quantity. Claims don't reserve stock; stock is confirmed at checkout.");
  const ja = hostPrompt("ja", "KEYWORD_QTY_CONTAINS", "A1");
  assert.match(ja, /A1\+2/);
  assert.match(ja, /A1 だけでは登録されません/);
  assert.match(ja, /1コメントにつき1商品/);
  assert.match(ja, /在庫は確保されず/);
  assert.doesNotMatch(ja, /コードだけをコメント/);
});

test("Japanese is copy-only and never widens Meta reply_locale", () => {
  assert.ok(hostPromptLanguages.includes("ja"));
  const wire = { id: sid, platform: "facebook", object: "page", asset_id: "synthetic-page", source_object_id: "123",
    private_reply: true, reply_locale: "ja", active: true, version: 1, verified: false, intake_count: 0, intake_capped: 0, updated_at: "2026-10-06T00:00:00Z" };
  assert.throws(() => parseClaimSource(wire));
  assert.equal(parseClaimSource({ ...wire, reply_locale: "en" }).reply_locale, "en");
});
