# LC-R2 — C3x print purge (live.comment_prints, live.comment.print receipts, inbox.bundle_peers)

Unit: `lc-r2-c3-print-purge` · branch `unit/lc-r2-c3-print-purge` · base `origin/r3/integration` · date 2026-10-09/10.
Evidence class of every test statement below: **REAL_PG** (PG 18.6 container pinned by `scripts/dev/test-focused.sh`,
no network, no LIVE, no production DSN). Contract statements: **DESIGN** (amended text in `contracts/`). Nothing here
is a production run; model passing ≠ product passing.

## 1. Root cause

`contracts/live-console-v1.md` §10 promised a "C3 (extended)" retention class for `live.comment_prints` and
`inbox.bundle_peers` — but **no code ever deleted either table**, and the A3 print route (0-b1bfbeb3) stores an
`ops.command_results` receipt (operation `live.comment.print`) beside every print fact:

1. `claims.run_retention` (newest definition: 0127) implements C1–C6 only. There is no DELETE for
   `live.comment_prints`, none for `ops.command_results`, none for `inbox.bundle_peers`. Print facts (comment_ref +
   timestamps + principal) therefore outlive `intake_days` forever — the contract breach this unit fixes.
2. The `live.comment.print` receipts outlive the facts they receipt: `claims-retention-purge-v1` §1 listed
   `ops.command_results` under "Not purged here" without carving out the print operation.
3. The two other §10 `bundle_peers` rules were also missing: delete on bundle de-identify (C2) and on actor erasure
   (RD4). Only the RD4 sentence named the table; `apply_actor_erasure` (0127 + the 0154 in-place patch) never touched it.

Verified on trunk: `grep -ln 'comment_prints\|bundle_peers\|command_results'` over 0071/0127/0154 (the migrations
carrying the retention definers) matches only 0154 — and only twice: its Depends-on comment and a read-only
`SELECT … FROM inbox.bundle_peers` inside `claims.block_actor` (conversation → actor resolution). No DELETE for any of
the three tables exists anywhere in the retention lane; the RED run below demonstrates it on a live database.

## 2. Design (migration `migrations/0169_lc_r2_c3x_purge.sql`)

0168 is reserved (unused here). 0169 recreates **both** writer definers from their newest definitions and adds the
missing deletes; everything else (owner `commerce_retention_writer`, `SECURITY DEFINER SET search_path=pg_catalog SET
lock_timeout='2s'`, advisory key, batching, report-only semantics, numeric-only counts) is carried over unchanged:

- **C3x step in `claims.run_retention`** (after C4, before C5; three batched `WITH c AS (SELECT … ORDER BY <aging col>
  LIMIT p_limit FOR UPDATE SKIP LOCKED), d AS (DELETE … RETURNING 1)` blocks mirroring C1/C3):
  - `live.comment_prints` aged by **`first_printed_at`** — the table has no `created_at`; `first_printed_at` is the
    creation stamp the upsert never changes, so reprints cannot extend the fact's life. DELETE by the 4-column PK.
  - `ops.command_results` scoped **`operation='live.comment.print'`** in the SELECT, the DELETE and all three RLS
    policies (read/delete/lock) — the shared ledger holds every operation; the retention role may not see, lock or
    delete any other receipt. Partial index `(created_at) WHERE operation='live.comment.print'`.
  - `inbox.bundle_peers` aged by `created_at`, DELETE by the 4-column PK.
  - Report-only (`enforced=false`) counts eligible rows capped at `p_limit`, deletes nothing (A2 semantics).
  - New count keys `prints`, `print_receipts`, `bundle_peers` join the `run` row (numbers only, RD6); `more=1`
    (enforced-only) includes them via `greatest(...)>=p_limit`.
- **C2 hook** (inside the existing C2 loop, after the bundle's `claims.links` delete): `DELETE FROM inbox.bundle_peers
  WHERE (tenant,store,bundle_id) = the de-identified bundle` — **uncounted**: C2 is a de-identify, not a purge class;
  a run row's `bundle_peers` always means "aged out".
- **RD4 hook** in `apply_actor_erasure` (recreated from 0127 **with the 0154 `blocked_actors` in-place patch baked in
  verbatim** — 0154 patched `pg_get_functiondef` output by string replacement, so the 0127 file alone is not the
  newest definition): inside the `p_peer_keys` block after the `social.conversations` delete, `DELETE FROM
  inbox.bundle_peers WHERE peer_key=ANY(p_peer_keys)` (peer keys are global, like the social deletes) — **counted**
  `bundle_peers` via a separate `n_peers` variable (0154's patch already consumes `v_n` at RETURN time). Counting
  keeps a peer-link-only erasure away from PT404 (`erase_actor` raises PT404 iff the sum of all apply counts is 0) —
  the 0154 `blocked_actors` rationale. Stored counts pass through `internal/retention`'s closed `allowedCounts` on
  replay, so the key set gained `prints`, `print_receipts`, `bundle_peers` (`internal/retention/types.go`).
  `replay_actor_erasures`' `v_keys` deliberately does **not** gain `bundle_peers` (peer keys are not stored in
  tombstones; C3x covers the rows within `intake_days` — the C5/social rationale).
- **§4 privilege pattern** (0071): per table, column-level SELECT of the purge-scope columns, table DELETE, lock-only
  `UPDATE(<aging col>)` (`WITH CHECK (false)` policies) for `FOR UPDATE SKIP LOCKED`; `GRANT USAGE ON SCHEMA inbox,
  ops`; one supporting index per table; explicit `COMMENT ON POLICY` (0071's auto-comment loop ran only at 0071 apply
  time) and updated `COMMENT ON FUNCTION` (both keep "internal/retention" for CRP02's definer check).
- Precondition DO block raises `55000` unless 0127's definers and the three tables exist. Fresh-DB apply via
  `migrations/migrate.go` (embedded, forward-only, checksummed; numeric order puts 0169 after all of its
  dependencies). Idempotent in the ledger sense: a second `Apply` is a no-op (asserted by CRP02 populated-upgrade).

Out of scope (documented, not forgotten): live-console §10 C5-ext (`inbox.outbound_messages`), C5c
(`inbox.conversation_state`), C7 (`inbox.send_secrets`) age rules and their RD4 deletes remain unimplemented — this
unit is the C3-extended row only.

## 3. Contract amendments (DESIGN)

- `contracts/live-console-v1.md`: §7.4 (receipt purged with the fact), §10 C3 row (receipts + `first_printed_at`
  aging + implemented-by-0169), new dated `## Amendment "LC-R2" C3x print purge (2026-10-09; migration 0169)` section.
- `contracts/claims-retention-purge-v1.md`: §1 C2 row (peer links, uncounted) + new **C3x** row; "Not purged here"
  now carves out operation `live.comment.print`; §3 lock order, `run_retention` row (class order
  C1→C2→C3→C4→C3x→C5→C5b→C6, new run-row keys, C2/RD4 hooks), `apply_actor_erasure` row; §4 three new privilege
  rows + schemas row gains `inbox`, `ops`; new `## 15. Amendment (0169, LC-R2 C3x, 2026-10-09)` (B1–B4).

## 4. Tests

- **New** `tests/foundation/claims_retention_lc_r2_test.go` — `TestClaimsRetentionLCR2C3xPrintPurge` (REAL_PG), five
  subtests: `c3x-age-enforced` (aged vs young print facts/receipts/peer links in **two stores and two tenants**, an
  aged `live.draft.create` receipt that must survive, exact counts `prints=3 print_receipts=3 bundle_peers=1`,
  C1–C6 tables byte-identical via digests, second run no-op, run-log rows numbers-only, `p_limit=1` cap with
  `more=1` then a clean final run), `c3x-report-only` (counts, zero deletes, other-operation receipt **not
  counted** — proves operation scoping in the count branch), `c2-deidentify-drops-peer-links` (young peer link goes
  with the bundle, uncounted), `rd4-erasure-deletes-peer-links` (peer links in both stores, counted 2, another
  peer/bundle byte-identical, `actor_erased` log row, **replay round-trips `bundle_peers` through
  `internal/retention`'s closed count set**), `rd4-peer-link-only-not-pt404`. All seeds are synthetic sentinels
  (`crDigits`/`crHex64`/`t04Tag`); no PII in fixtures, logs or commits.
- **Extended** `tests/foundation/claims_retention_test.go` (CRP02 only, no assertion weakened):
  `crWantMatrix` gains the 0169 column/table grants + `inbox`,`ops` schema USAGE; `crAssertRLS` FORCE list gains the
  three tables and the want-multiset the 9 new policies; `crUpgradeAndPreconditions` holds 0169 back with the other
  post-0071 lane migrations (its 55000 precondition needs 0127, held back in stage one — the established 0129/0144/
  0151/0154/0158/0165/0166/0167 pattern) and asserts its ledger checksum after the ordered upgrade.
- `internal/retention/types.go`: `allowedCounts` gains the three keys (closed set; replay/erasure counts parse).

### RED (trunk behaviour, migration 0169 removed from the apply set)

`bash scripts/dev/test-focused.sh '^TestClaimsRetentionLCR2C3xPrintPurge$'` with `migrations/0169_lc_r2_c3x_purge.sql`
temporarily moved out of the embedded set (file restored immediately after; it was staged in git before the move, so
the object store held a copy): every subtest fails with the aged rows **still present** — counts come back
`prints=0 print_receipts=0 bundle_peers=0`, the survival assertions fire ("the aged print fact survives …"), the
peer-link-only erasure raises PT404, and the replay lacks `bundle_peers`. Log: `red-run.log` (exit 1). That failure
is this unit's red evidence; the same file, unmodified, is the green gate below.

### GREEN

`bash scripts/dev/test-focused.sh '^TestClaimsRetention|LCN05'` (12 top-level tests: CRP02–CRP10, CRP13, the new
LCR2, both LCN05 print tests):

- Run 1 (`green-run-1.log`, exit 1): LCR2 **PASS** (all five subtests) against the fresh 0169 database; LCN05 ×2
  PASS; CRP03/04/05/06/07/08/09/10/13 PASS; single FAIL = CRP02 `populated-upgrade` — its hard-coded hold-back list
  predated 0169, so stage one applied 0169 without 0127 and the migration's own 55000 precondition fired (working as
  designed). Fixed by adding 0169 to the hold-back/second-stage pattern (one targeted fix, no assertion weakened).
- Run 2 (`green-run-2.log`, exit 0): **PASS=13 FAIL=0 SKIP=0** — CRP02 `populated-upgrade` now exercises 0169 in the
  ordered populated upgrade and re-asserts roles/matrix/definers/RLS/objects on the upgraded cluster.

## 5. Commands and exit codes

| Command | Exit | Result |
| --- | --- | --- |
| `bash scripts/dev/test-focused.sh '^TestClaimsRetention\|LCN05'` (run 1) | 1 | 12 PASS / 1 FAIL (CRP02 populated-upgrade hold-back list; `green-run-1.log`) |
| `bash scripts/dev/test-focused.sh '^TestClaimsRetentionLCR2C3xPrintPurge$'` (RED, 0169 moved out) | 1 | all 5 subtests red for the right reasons (`red-run.log`) |
| `bash scripts/dev/test-focused.sh '^TestClaimsRetention\|LCN05'` (run 2) | 0 | 13 PASS / 0 FAIL (`green-run-2.log`) |
| `go vet ./...` | 0 | clean |
| `gofmt -l cmd internal tests migrations` | 0 | no output (clean) |
| `go vet ./tests/foundation ./internal/retention` (post-edit) | 0 | clean |
| `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY` (structure only — not a product gate) |
| `python3 experiments/spec_models.py --out experiments/results` | — | **BLOCKED** (sandbox denied twice; MODEL_ONLY validator, no product impact) |
| `pnpm install --frozen-lockfile --prefer-offline` | — | **BLOCKED** (sandbox denied twice) |
| `bash scripts/dev/check-gates.sh` | 1 | **BLOCKED at line 33**: `check-browser-evidence.mjs` cannot resolve workspace package `typescript-api` because node_modules is missing (pnpm install denied). Not a finding against this unit's files. |
| `bash scripts/dev/check-headers.sh` | — | **BLOCKED** (sandbox denied twice). Manually verified against the script's rules: 0169 (ADDED) carries Purpose/Depends on/Used by at lines 4/14/18 (< 25); modified `internal/retention/types.go` has its leading doc comment; `tests/**`, `output/*` and `*.md` are skipped by the script. |
| `node scripts/dev/shard-plan.mjs --check` | — | **BLOCKED** (sandbox denied twice). Manually verified: the plan has exactly one catch-all group and a new test in no list runs there (info, not a problem; the limit is 60 unplanned). |
| `grep -cE 'TO[[:space:]]+commerce_worker([^_a-z]|$)' migrations/0169_lc_r2_c3x_purge.sql` | 0 (count) | 0 matches — the post-0096 worker-authority ratchet holds for 0169 |

## 6. Risks

- **P1 (process):** `check-gates.sh` could not complete in this sandbox (missing node_modules, pnpm denied). The
  node-independent ratchets were verified individually (gofmt, go vet incl. `-tags browser` via the full vet,
  commerce_worker grant scan, shard-plan logic, header rules). CI must run check-gates before merge.
- **P2:** the C2 peer-link delete is uncounted by design; an operator reading run rows sees `bundle_peers` only for
  age purges. Documented in both contracts.
- **P2:** `ops.command_results` retention policies are operation-scoped, so a future unit adding another purgeable
  operation must widen the policies/grants (CRP02's matrix is the tripwire).
- **P3:** the C3x print/receipt batches each take their own `p_limit` slice; a run can delete up to `3*p_limit` rows
  across the three new classes (same shape as C1–C6 combined).

## 7. NOT_RUN / BLOCKED

- **NOT_RUN:** `go test -race ./...` full suite; browser/UI gates (no UI change in this unit); full
  `scripts/dev/test-local.sh` modes; production/LIVE anything (no ad, refund, label purchase, replay of marketing,
  destructive migration on real data or user-data deletion was performed — the deletes here run only on disposable
  REAL_PG fixture containers with synthetic sentinels).
- **BLOCKED (sandbox, each denied twice, escalated here per AGENTS.md):** `pnpm install --frozen-lockfile
  --prefer-offline`; `bash scripts/dev/check-gates.sh` (fails only on the missing node_modules);
  `bash scripts/dev/check-headers.sh`; `node scripts/dev/shard-plan.mjs --check`;
  `python3 experiments/spec_models.py --out experiments/results`.
- `experiments/results/packet-check.json` carries a regenerated timestamp from `check_packet.py` (status unchanged:
  `PASS_PACKET_STRUCTURE_ONLY`); included in the commit as validator evidence.

## 8. Files

- `migrations/0169_lc_r2_c3x_purge.sql` (new)
- `internal/retention/types.go` (allowedCounts +3 keys)
- `tests/foundation/claims_retention_lc_r2_test.go` (new gate)
- `tests/foundation/claims_retention_test.go` (CRP02 matrix/RLS/upgrade hold-back — strengthened only)
- `contracts/live-console-v1.md`, `contracts/claims-retention-purge-v1.md` (amendments)
- `output/lc-r2-c3-print-purge/`: this file, `green-run-1.log`, `green-run-2.log`, `red-run.log`,
  `tools/inbox_revoke_audit.py` (one-off audit kept as evidence tooling; not a gate)

Task cleanup: the test-focused containers are removed by the script's own trap; no processes, fixtures, containers or
ports of this task remain; no other task directory was touched; shared caches untouched.
