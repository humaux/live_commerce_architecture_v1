// staff-team: strict parsers for the BFF answers (apps/admin/lib/team-model.ts), the BFF request grammar and copy parity of
// the three admin locales (lib/team-copy.ts). Synthetic fixtures only. Run: node --test --experimental-strip-types tests/admin/team-model.test.ts
import assert from "node:assert/strict";
import { test } from "node:test";
import { parseInviteResult, parseJoined, parseTeam, roles, teamActions, teamKeys, validTeamBody } from "../../apps/admin/lib/team-model.ts";
import { teamCopy } from "../../apps/admin/lib/team-copy.ts";

const id = "11111111-1111-4111-8111-111111111111";
const team = () => ({
  my_role: "owner",
  members: [{ principal_id: id, email: "owner@example.test", role: "owner", joined_at: "2026-10-01T00:00:00.123456Z", is_me: true }],
  invitations: [{ id, email: "new@example.test", role: "viewer", created_at: "2026-10-01T00:00:00Z", expires_at: "2026-10-04T00:00:00Z", expired: false, mail_state: "SENT" }],
});

test("team answer: accepts the frozen shape, null role and OIDC member without email", () => {
  assert.equal(parseTeam(team()).members[0].role, "owner");
  const other = { ...team(), my_role: null, members: [], invitations: [] };
  assert.equal(parseTeam(other).my_role, null);
  const oidc = team();
  (oidc.members[0] as { email: string | null }).email = null;
  assert.equal(parseTeam(oidc).members[0].email, null);
});

test("team answer: unknown key, unknown role, bad id, bad mail state are rejected", () => {
  for (const mutate of [
    (t: any) => { t.extra = 1; },
    (t: any) => { t.my_role = "root"; },
    (t: any) => { t.members[0].role = "superadmin"; },
    (t: any) => { t.members[0].principal_id = "x"; },
    (t: any) => { t.members[0].token_hash = "x"; },
    (t: any) => { t.invitations[0].mail_state = "QUEUED"; },
    (t: any) => { t.invitations[0].expires_at = "soon"; },
    (t: any) => { delete t.invitations; },
  ]) {
    const t = team();
    mutate(t);
    assert.throws(() => parseTeam(t), /unavailable/);
  }
});

test("invite and accept answers are strict", () => {
  assert.equal(parseInviteResult({ id, expires_at: "2026-10-04T00:00:00Z", mail_state: "FAILED" }).mail_state, "FAILED");
  assert.throws(() => parseInviteResult({ id, expires_at: "2026-10-04T00:00:00Z", mail_state: "PENDING" }), /unavailable/);
  assert.throws(() => parseInviteResult({ id, expires_at: "2026-10-04T00:00:00Z", mail_state: "SENT", token: "x" }), /unavailable/);
  assert.deepEqual(parseJoined({ store_id: id, role: "viewer" }), { store_id: id, role: "viewer" });
  assert.throws(() => parseJoined({ store_id: id, role: "x" }), /unavailable/);
});

test("request grammar: every action has an exact key set and rejects junk values", () => {
  const token = "A".repeat(43);
  assert.deepEqual([...teamActions], ["list", "invite", "revoke-invite", "set-role", "remove", "accept"]);
  assert.ok(validTeamBody("list", { store_id: id }));
  assert.ok(!validTeamBody("list", { store_id: "../x" }));
  assert.ok(validTeamBody("invite", { store_id: id, email: "a@example.test", role: "fulfilment", locale: "zh-TW" }));
  assert.ok(!validTeamBody("invite", { store_id: id, email: "a b@example.test", role: "viewer", locale: "en" }));
  assert.ok(!validTeamBody("invite", { store_id: id, email: "a@example.test", role: "root", locale: "en" }));
  assert.ok(!validTeamBody("invite", { store_id: id, email: "a@example.test", role: "viewer", locale: "fr" }));
  assert.ok(validTeamBody("set-role", { store_id: id, principal_id: id, role: "admin" }));
  assert.ok(!validTeamBody("set-role", { store_id: id, principal_id: id, role: "" }));
  assert.ok(validTeamBody("remove", { store_id: id, principal_id: id }));
  assert.ok(validTeamBody("revoke-invite", { store_id: id, invite_id: id }));
  assert.ok(validTeamBody("accept", { token }));
  assert.ok(!validTeamBody("accept", { token: "short" }));
  assert.ok(!validTeamBody("accept", { token: token + "=" }));
  // No action accepts a tenant or principal chosen by the client beyond the ids the server re-verifies.
  for (const action of teamActions) assert.ok(!teamKeys[action].includes("tenant_id"));
});

test("copy: three locales carry the same keys and a label for every role", () => {
  const shape = (value: unknown): unknown =>
    value && typeof value === "object" ? Object.fromEntries(Object.entries(value).sort().map(([k, v]) => [k, shape(v)])) : typeof value;
  const en = shape(teamCopy.en);
  assert.deepEqual(shape(teamCopy["zh-CN"]), en);
  assert.deepEqual(shape(teamCopy["zh-TW"]), en);
  for (const locale of Object.keys(teamCopy) as (keyof typeof teamCopy)[])
    for (const role of roles) {
      assert.ok(teamCopy[locale].roleNames[role]);
      assert.ok(teamCopy[locale].roleHelp[role]);
    }
  // Role labels are duplicated in internal/identity/staffmail.go (the invitation mail): pin the spelling here.
  assert.equal(teamCopy["zh-CN"].roleNames.viewer, "只读成员");
  assert.equal(teamCopy["zh-TW"].roleNames.live_operator, "直播營運");
  assert.equal(teamCopy.en.roleNames.fulfilment, "Fulfilment");
});
