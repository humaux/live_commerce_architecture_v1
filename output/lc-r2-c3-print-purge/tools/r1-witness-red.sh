#!/usr/bin/env bash
# r1-witness-red.sh — LC-R2 round 1 (K3 data-safety P2): reproducible record of the witness rounds that prove
# every new C3x writerPolicyBehaviour case is real. Each round temporarily widens ONE control in
# migrations/0169_lc_r2_c3x_purge.sql (working tree only; reverted immediately after the run — the committed 0169
# never changes), runs the focused REAL_PG subtest TestClaimsRetentionCRP02Schema/writer-policy-behaviour on a
# disposable fixture container, and asserts the expected red (or, for round a-delete-only, expected GREEN).
#
# NOTE (how round 1 actually ran): this sandbox denied executing scripts under output/, so the rounds below were
# orchestrated by hand (file edit -> scripts/dev/test-focused.sh -> revert -> `git diff` clean check), one round
# at a time. This script is the byte-faithful reproduction kept as evidence tooling (precedent:
# tools/inbox_revoke_audit.py).
#
# Rounds (widened control -> expected outcome -> log):
#   a-delete-only   command_print_retention_delete USING -> (true)                 -> GREEN: another operation's receipt is STILL
#                 undeletable, because PostgreSQL also applies the SELECT policy to a DELETE's scan and the scoped
#                 read policy keeps the row invisible. Recorded as defense-in-depth (r1-green-a-delete-only-widened.log).
#   a-read          command_print_retention_read   USING -> (true)                 -> RED: another operation's receipt becomes readable
#   a-read-delete   read + delete USING -> (true)                                  -> RED: ...and deletable (both guards must fall)
#   a-read-lock     read + lock USING -> (true) (WITH CHECK stays false)           -> RED: ...and FOR UPDATE-visible; a real UPDATE still
#                 fails 42501 on WITH CHECK (false) — the lock-only guard survives the USING widening
#   b-print-lock    comment_print_retention_lock   WITH CHECK -> (true)            -> RED: first_printed_at becomes updatable
#   b-receipt-lock  command_print_retention_lock   WITH CHECK -> (true)            -> RED: command_results.created_at becomes updatable
#   b-peer-lock     bundle_peer_retention_lock     WITH CHECK -> (true)            -> RED: bundle_peers.created_at becomes updatable
#   c-print-grant   live.comment_prints  GRANT SELECT(cols) -> whole table         -> RED: print_count/last_printed_at/last_principal_id readable
#   c-receipt-grant ops.command_results  GRANT SELECT(cols) -> whole table         -> RED: response/request_hash/principal_id readable
#   c-peer-grant    inbox.bundle_peers   GRANT SELECT(cols) -> whole table         -> RED: app_id/object/asset_id/operation_id readable
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"
MIG=migrations/0169_lc_r2_c3x_purge.sql
OUT=output/lc-r2-c3-print-purge
REGEX='^TestClaimsRetentionCRP02Schema$/^writer-policy-behaviour$'

git diff --exit-code -- "$MIG" || { echo "refusing: $MIG already dirty" >&2; exit 1; }

mutate() { # $1 = round key
  python3 - "$1" <<'PY'
import pathlib, sys
key = sys.argv[1]
p = pathlib.Path('migrations/0169_lc_r2_c3x_purge.sql')
s = p.read_text()
SCOPED = "USING (operation='live.comment.print')"
rd = "CREATE POLICY command_print_retention_delete ON ops.command_results FOR DELETE TO commerce_retention_writer " + SCOPED + ";"
rd_w = "CREATE POLICY command_print_retention_delete ON ops.command_results FOR DELETE TO commerce_retention_writer USING (true);"
rr = "CREATE POLICY command_print_retention_read ON ops.command_results FOR SELECT TO commerce_retention_writer " + SCOPED + ";"
rr_w = "CREATE POLICY command_print_retention_read ON ops.command_results FOR SELECT TO commerce_retention_writer USING (true);"
rl = "CREATE POLICY command_print_retention_lock ON ops.command_results FOR UPDATE TO commerce_retention_writer " + SCOPED + " WITH CHECK (false);"
rl_u = "CREATE POLICY command_print_retention_lock ON ops.command_results FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (false);"
rl_c = "CREATE POLICY command_print_retention_lock ON ops.command_results FOR UPDATE TO commerce_retention_writer " + SCOPED + " WITH CHECK (true);"
pl = "CREATE POLICY comment_print_retention_lock ON live.comment_prints FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (false);"
pl_c = "CREATE POLICY comment_print_retention_lock ON live.comment_prints FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (true);"
bl = "CREATE POLICY bundle_peer_retention_lock ON inbox.bundle_peers FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (false);"
bl_c = "CREATE POLICY bundle_peer_retention_lock ON inbox.bundle_peers FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (true);"
gp = "GRANT SELECT(tenant_id,store_id,session_id,comment_ref,first_printed_at) ON live.comment_prints TO commerce_retention_writer;"
gp_w = "GRANT SELECT ON live.comment_prints TO commerce_retention_writer;"
gr = "GRANT SELECT(tenant_id,store_id,operation,idempotency_key,created_at) ON ops.command_results TO commerce_retention_writer;"
gr_w = "GRANT SELECT ON ops.command_results TO commerce_retention_writer;"
gb = "GRANT SELECT(tenant_id,store_id,bundle_id,peer_key,created_at) ON inbox.bundle_peers TO commerce_retention_writer;"
gb_w = "GRANT SELECT ON inbox.bundle_peers TO commerce_retention_writer;"
subs = {
    'a-delete-only':  [(rd, rd_w)],
    'a-read':         [(rr, rr_w)],
    'a-read-delete':  [(rr, rr_w), (rd, rd_w)],
    'a-read-lock':    [(rr, rr_w), (rl, rl_u)],
    'b-print-lock':   [(pl, pl_c)],
    'b-receipt-lock': [(rl, rl_c)],
    'b-peer-lock':    [(bl, bl_c)],
    'c-print-grant':  [(gp, gp_w)],
    'c-receipt-grant':[(gr, gr_w)],
    'c-peer-grant':   [(gb, gb_w)],
}[key]
for old, new in subs:
    assert s.count(old) == 1, (key, old, s.count(old))
    s = s.replace(old, new)
p.write_text(s)
print('mutated:', key)
PY
}

round() { # $1 key, $2 log file, $3 want-red (yes|no), $4 expected failure fragment ("" when want-red=no)
  local key="$1" log="$OUT/$2" want_red="$3" expect="$4" rc
  echo "=== ROUND $key -> $2 (want_red=$want_red)"
  mutate "$key" || { echo "MUTATE FAILED $key" >&2; exit 1; }
  bash scripts/dev/test-focused.sh "$REGEX" > "$log" 2>&1
  rc=$?
  git restore "$MIG"
  git diff --exit-code -- "$MIG" || { echo "RESTORE FAILED $key" >&2; exit 1; }
  if [[ "$want_red" == yes ]]; then
    [[ $rc -ne 0 ]] || { echo "ROUND $key DID NOT GO RED (exit 0)" >&2; exit 1; }
    grep -qF -- "$expect" "$log" || { echo "ROUND $key red without the expected fragment: $expect" >&2; exit 1; }
    grep -F -- "$expect" "$log" | head -3
  else
    [[ $rc -eq 0 ]] || { echo "ROUND $key expected GREEN, got exit $rc" >&2; exit 1; }
    echo "round $key green as expected (second guard holds): $(grep -F 'top-level' "$log")"
  fi
  echo "round $key: exit=$rc"
}

round a-delete-only   r1-green-a-delete-only-widened.log no  ""
round a-read          r1-red-a-read.log                  yes "another operation's aged receipt unreadable (USING operation): 1 rows, want 0"
round a-read-delete   r1-red-a-read-delete.log           yes "another operation's aged receipt undeletable: 1 rows affected, want 0"
round a-read-lock     r1-red-a-read-lock.log             yes "another operation's aged receipt unlockable-invisible (FOR UPDATE): 1 rows, want 0"
round b-print-lock    r1-red-b-print-lock.log            yes "comment_prints: lock-only UPDATE cannot change first_printed_at"
round b-receipt-lock  r1-red-b-receipt-lock.log          yes "command_results: lock-only UPDATE cannot change created_at"
round b-peer-lock     r1-red-b-peer-lock.log             yes "bundle_peers: lock-only UPDATE cannot change created_at"
round c-print-grant   r1-red-c-print-grant.log           yes "comment_prints: print_count unreadable"
round c-receipt-grant r1-red-c-receipt-grant.log         yes "command_results: response unreadable"
round c-peer-grant    r1-red-c-peer-grant.log            yes "bundle_peers: app_id unreadable"

echo "ALL ROUNDS MATCHED EXPECTATIONS; $MIG restored:"
git status --porcelain -- "$MIG"
echo done
