// Purpose: Detect claim-mode drift between Go, SQL, both admin read models and the mode picker/prompts.
// Depends on: node:test/assert/fs, internal/claims/claims.go, migration 0115, claims-model/copy and console-model.
// Used by: scripts/dev/test-node.sh; D3 regression before the GitHub real-click gates.
import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
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

test("host prompt stays verbatim at the frozen trunk contract; Japanese is absent", async () => {
  assert.deepEqual([...hostPromptLanguages].sort(), ["zh-TW", "zh-CN", "en"].sort());
  const source = await src("apps/admin/lib/claims-copy.ts");
  const block = source.slice(source.indexOf("// FROZEN host prompt copy"), source.indexOf("/** The private message sent"));
  // Integrator 2026-10-07 ruling: pin the origin0813424a frozen block, not new UI-authored prompt text.
  assert.equal(createHash("sha256").update(block).digest("hex"), "178058ae529501b844806c5858dd11e71898b15221455fc301a9d52c488c4e03");
  for (const locale of hostPromptLanguages) assert.equal(hostPrompt(locale, "KEYWORD_QTY_CONTAINS", "A1"), hostPrompt(locale, "KEYWORD_QTY_ONLY", "A1"));
  const wire = { id: sid, platform: "facebook", object: "page", asset_id: "synthetic-page", source_object_id: "123",
    private_reply: true, reply_locale: "ja", active: true, version: 1, verified: false, intake_count: 0, intake_capped: 0, updated_at: "2026-10-06T00:00:00Z" };
  assert.throws(() => parseClaimSource(wire));
  assert.equal(parseClaimSource({ ...wire, reply_locale: "en" }).reply_locale, "en");
});
