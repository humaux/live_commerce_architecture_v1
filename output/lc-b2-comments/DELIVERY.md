# lc-b2-comments — DELIVERY

Backend unit **LC-B2**: 直播控制台评论只读（comment read-through）— the console's `GET` comments read
(A2) and comment-label print (A3) from `contracts/live-console-v1.md` §2 (poller, leases, caps, bridge
incl. `comment-facts`), §2.3 (cursors, deletion eviction), §2.5 (marks), §7.4 (prints). Backend-only
(Go/SQL/migrations/deploy/scripts); `apps/` and any browser surface are separate units and were not touched.

Author model: DeepSeek V4-Pro (backend worker). Base SHA `3ed634be`. Worktree
`.worktrees/lc-b2-comments`, branch `unit/lc-b2-comments`. Migration number **0121** (assigned placeholder —
the integrator renumbers at merge, per the unit table).

## Tier

DESIGN + MOCK. The domain code, migration 0121 and foundation tests are REAL_PG-verified for the
poller/lease/cap/bridge/marks/print paths, but every Meta Graph interaction is a **loopback httptest MOCK**
— no LIVE Meta call exists here. The console read-through is a T2 subset (`--live-console`); nothing in this
delivery is a product or LIVE pass.

## Implementation

- `migrations/0121_live_console_comments.sql` — two tables + seven SECURITY DEFINER functions, all
  `search_path=pg_catalog`, FORCE RLS:
  - `live.comment_poll_leases` (per active claim source; `generation>0`, `holder_id`, `poll_epoch`,
    `lease_token_hash bytea(32)`, `lease_until`, `demand_until`; FK → `live.claim_sources` ON DELETE CASCADE)
    and `live.comment_prints` (FK → `live.sessions`, no cascade) — neither ever stores comment text/name (I11).
  - `live.acquire_comment_poll_lease` → `(generation,poll_epoch,acquired)`: same-holder-unexpired renewal
    keeps the epoch; expired take-over bumps `generation` (+`poll_epoch` on a fresh buffer).
  - `live.comment_poll_sources()` — active sources with an OPEN window OR live demand (11 cols).
  - `live.console_source(…)` — the bridge's re-check before touching a ring buffer (zero rows → 404).
  - `integration.load_meta_page_token_for_poll(…)` — lease-fenced clone of `load_meta_page_token`.
  - `social.read_comment_events(…)` — IG-live OPEN-4 fallback from the encrypted webhook copy.
  - `live.console_marks(…)` — joins each comment to its claim/print/reply facts.
  - `live.comment_print(…)` — idempotent print-record upsert (`RETURNING print_count,last_printed_at`).
  - Grants only to `commerce_integration_writer` / `commerce_claims_writer` / `commerce_claims_worker` /
    `commerce_runtime` / `commerce_meta_writer` — **no** retired `commerce_worker`.
- `internal/integrations/metareply/comment_poll.go` — `Console`: per-source bounded in-memory ring buffer
  (≤2000 items, 2h age, 10min idle drop), poll leases via `acquire_comment_poll_lease`, Page-token custody
  (AES v1 / HPKE v2) via `load_meta_page_token_for_poll`, the worker-side bridge handlers
  (`POST /internal/v1/comment-page`, `/internal/v1/comment-facts`) with the constant-time base64 bearer check.
- `internal/integrations/metareply/bridge.go` — `BridgeClient` (API side): fixed 32-byte bearer (std-base64
  on the wire), no redirects, fixed safe errors; `CommentPage` / `CommentFacts`; `sealCursor`/`openCursor`
  HMAC-signed `older_cursor` (scope + graph_cursor + expiry).
- `internal/integrations/meta/comment_read.go` — IG-live projection from one decrypted
  `social.comment_events` row; `from.id` collapsed to `IsPage`, never leaves the package.
- `internal/live/stream.go` — `CommentStream` (A2) / `PrintComment` (A3): resolves the one active claim
  source, reads Facebook via bridge or Instagram via the encrypted webhook copy, joins marks via
  `live.console_marks`; fixed sentinels `ErrStreamUnavailable`/`ErrNoSource`/`ErrInvalidCursor`/`ErrInvalidRef`.
- `internal/httpapi/live_stream.go` + `live_stream_test.go` — A2/A3 routes + 405 fallbacks,
  `Cache-Control: no-store` + `Referrer-Policy: no-referrer` (I15); DB-free route/query/classify/transport tests.
- `cmd/api/comment_stream.go` — assembles the bridge client + optional payload keyring; **never** imports the
  Page-token private keyring.
- `cmd/claims-worker/main.go` (+ `doc.go`, `main_test.go`) — serves the bridge listener on
  `COMMERCE_CLAIMS_CONSOLE_ADDR` and runs the poller sweep; fails closed without leaking secrets.
- `deploy/compose.yml` + `deploy/secrets.manifest.tsv` — `commerce_claims_bridge_token` /
  `commerce_claims_cursor_key` secrets (b64std32), bridge listener `:8081` on claims-worker, bridge client on
  api; claims-worker stays a single replica (I23).
- `scripts/dev/test-local.sh` — `--live-console` mode (provisions disposable PG, `LC_TEST_DATABASE_ALLOWED=1`,
  `go test -race -run '^TestLiveConsole' ./internal/live ./tests/foundation`).
- `docs/delivery/GATES.md` — `--live-console` row (T2 subset).
- `tests/foundation/live_console_comments_test.go` — the six REAL_PG `TestLiveConsoleLCN01/02/04/05` tests.

## Tests actually run (commands + exit codes)

| # | command | exit | note |
|---|---------|------|------|
| 1 | `bash scripts/dev/test-local.sh --live-console` | 0 | all 6 `TestLiveConsoleLCN*` REAL_PG green (LCN01/02/04/05) |
| 2 | `go build ./...` | 0 | all Go packages compile |
| 3 | `go vet ./...` | 0 | clean |
| 4 | `gofmt -l <all touched .go files>` | 0 | no output = clean |
| 5 | `go test ./internal/integrations/metareply/... ./internal/live/... ./internal/httpapi/... ./cmd/api/... ./cmd/claims-worker/...` | 0 | all `ok` |
| 6 | `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY` (structure only) |

## Red → green evidence

The six foundation tests are the red→green artifact, all REAL_PG (through the real SECURITY DEFINER
lease/token/credential functions; MOCK loopback Graph):

- `TestLiveConsoleLCN01LeaseAndTokenFence` — lease acquire/renew/take-over fences the Page-token load.
- `TestLiveConsoleLCN01CapsAndNoPoll` — tenant/fleet caps and demand; a draft/archived source is never polled.
- `TestLiveConsoleLCN01BufferCapAgeAndCursor` — ring-buffer cap/age/idle-drop and the HMAC `older_cursor`.
- `TestLiveConsoleLCN02BridgeStatusesAndFacts` — bridge status codes + `comment-facts` (ring buffer + Graph read).
- `TestLiveConsoleLCN04NoCommentTextPersisted` — sentinel comment text/author name never lands in PG.
- `TestLiveConsoleLCN05PrintAndMarks` — print idempotency (A3) and marks mapping (A2).

Author fix round (during bring-up, before any integrator run): four migration-0121 defects and one bridge
protocol defect, all found by the REAL_PG gate and fixed in place —
1. `GRANT USAGE ON SCHEMA live TO commerce_claims_worker` (the poller role had no `live` schema USAGE).
2. `comment_poll_lease_read` policy + column SELECT grant on `live.comment_poll_leases` for
   `commerce_claims_writer` (needed by `live.comment_poll_sources`).
3. `source_poll_read` no-GUC SELECT policy on `live.claim_sources` for `commerce_integration_writer`
   (the poller runs outside any merchant transaction, so the 0064 `intake_scope` policy can never match).
4. `live.console_marks`: unconditional `SELECT … INTO v_claim` (the conditional `record := NULL` still raised
   `record "v_claim" is not assigned yet`).
5. `live.comment_print`: qualified `RETURNING live.comment_prints.print_count, …` (RETURNS TABLE output params
   collided with table columns → 42702).
6. Bridge bearer is 32 arbitrary bytes, so it is **std-base64 on the wire** (raw bytes trip Go's header
   validation over real HTTP); `bridge.go` encodes, `comment_poll.go` base64-decodes + constant-time compares,
   and the test `lcnPost` encodes.

## NOT_RUN / BLOCKED

1. **`bash scripts/dev/check-gates.sh`** → BLOCKED before reaching the Go/python checks: this worktree has no
   `node_modules`, so the UI/shell architecture gates fail `Cannot find module 'typescript-api'` (unrelated to
   this change). The migration-specific gate it enforces passes: 0121 has no `GRANT … TO commerce_worker`.
2. **`bash scripts/dev/check-headers.sh`** and **`python3 output/lc-b2-comments/check_registry.py`** → NOT_RUN
   this session (approval-gated). The tri-line Purpose/Depends on/Used by headers are present on every new file
   (verified by reading), and the `--live-console` GATES row + `test-local.sh` usage entry are both present and
   consistent (the `check_registry.py` cross-check passes by inspection).
3. **`python3 experiments/spec_models.py --out experiments/results`** → NOT_RUN (approval-gated). Unchanged by
   this unit.
4. **Browser / UI gates** → out of scope: backend-only unit; no UI surface in `apps/` was touched and no UI
   test authored here. (The AGENTS.md "real click" acceptance rule applies to UI units.)

## Risks

- **Migration numbering.** `0121_live_console_comments.sql` is the assigned **placeholder**; the integrator must
  renumber it to the next free sequential number at merge (forward-only/checksummed). I used 0121 verbatim as
  assigned and never renumber migrations I was not told to.
- **Single-replica bridge.** The bridge is stateful (per-source leases in PG + in-memory ring buffer), so
  claims-worker MUST stay a single replica (I23). `deploy/compose.yml` reflects this; any future horizontal
  scaling of claims-worker would silently break poll-lease exclusivity and buffer consistency.
- **Graph paths are MOCK.** `facebookPage` reads are exercised against a loopback httptest server only; no LIVE
  Meta app was run. §13.3 (LIVE) is outside this T2 unit.
- **Cursor/buffer eviction semantics.** Ring-buffer eviction (cap/age/idle-drop) and `older_cursor` expiry are
  REAL_PG/loopback-verified, but the deletion-eviction path (§2.3) depends on the retention amendment adding the
  `comment_prints` DELETE row (§10); that amendment is a separate unit.
- **Secret rotation coupling.** `commerce_claims_bridge_token` is shared by api and claims-worker; rotating it
  requires restarting both together (documented in `secrets.manifest.tsv`).

## Integrator items

- **Migration number**: renumber `0121_live_console_comments.sql` to the next sequential free number at merge.
- **Secrets**: `deploy/compose.yml` + `deploy/secrets.manifest.tsv` are integrator-merged write paths; the
  `secrets-init.sh` generated-file count was bumped 56→58 in the manifest comment.
- **Dependency map**: `docs/engineering/dependency-map.md` may be stale after the new `internal/live` /
  `internal/integrations/metareply` / `internal/integrations/meta` imports; regenerate with
  `bash scripts/dev/depmap.sh` post-merge (not done here).
- **Non-author acceptance**: required — the author is the only reviewer of this change; the integrator/test_worker
  must independently re-run `bash scripts/dev/test-local.sh --live-console` and the `security_reviewer` should
  review the bearer/cursor-key custody and the `no text persisted` invariant (I11).

## Write paths used (all within allowance)

`migrations/0121_live_console_comments.sql`, `internal/integrations/metareply/comment_poll.go`,
`internal/integrations/metareply/bridge.go`, `internal/integrations/meta/comment_read.go`,
`internal/live/stream.go`, `internal/httpapi/live_stream.go`, `internal/httpapi/live_stream_test.go`,
`internal/httpapi/handler.go`, `cmd/api/comment_stream.go`, `cmd/api/main.go`, `cmd/claims-worker/main.go`,
`cmd/claims-worker/doc.go`, `cmd/claims-worker/main_test.go`, `deploy/compose.yml`,
`deploy/secrets.manifest.tsv`, `scripts/dev/test-local.sh`, `docs/delivery/GATES.md`,
`tests/foundation/live_console_comments_test.go`, `output/lc-b2-comments/DELIVERY.md`. No `apps/`/UI, no
go.mod/go.sum/OpenAPI-shared-schema/pnpm-lock changes. `experiments/results/packet-check.json` was regenerated
locally by `check_packet.py` (timestamp bump only) and left uncommitted (integrator pass).

---

## Integrator completion (Sonnet)

Author text above is DeepSeek's and stays as written; wherever it says migration **0121**, read **0123** (renumbered
here: 0121 is `0121_msg_templates`, 0122 is reserved by LC-B1). Evidence class: **REAL_PG + MOCK Graph** (no LIVE Meta).

### Changes
1. **Merged `r3/integration` (eb5746f8).** Conflicts, both sides kept: `cmd/api/main.go` (commentStream + inbox service),
   `cmd/claims-worker/{main.go,doc.go}` (bridge listener + resubscriber), `scripts/dev/test-local.sh` (`--live-console`
   plus `--inbox`, `--msg-templates` in both the guard and the usage string and the dispatch).
2. **Migration `0121_live_console_comments.sql` -> `0123_live_console_comments.sql`.** References updated in the migration
   header, `metareply/{bridge,comment_poll}.go`, `live/stream.go`, `tests/foundation/live_console_comments_test.go`,
   `contracts/live-console-v1.md` (two `(0121,` mentions + the LC-B2 unit row). The LC-B5 `0121` mentions stay (that is msg_templates).
3. **Amendment 1 A1.2 (LC-B2 delta).**
   - `comment_poll.go`: bridge `comment-facts` is platform-branched (`factsFields`, `parseCommentFacts`): FB
     `created_time,from{id},parent{id}`, IG `timestamp,from{id},parent_id`; no message/name is read for facts; a comment
     whose author id is missing answers `{found:false}` (no guessed `is_page=false`). The single read is parsed as a BARE
     object (the old FB code parsed a `{data:[]}` page, which real Graph does not return for `GET /{comment_id}`; the fake
     Graph in `live_console_comments_test.go` was corrected to the real shape).
   - `0123`: `social.read_comment_facts(p_session,p_comment_ref)` (STABLE DEFINER, owner `commerce_meta_writer`, EXECUTE
     `commerce_runtime`, live:read via `identity.principal_holds`; newest copy; comment_key recomputed in SQL as
     sha256(json["meta-social-comment/v1",app,object,asset,ref])). Same migration grants `commerce_meta_writer` USAGE on
     schemas `identity`,`live` and column `live.claim_sources.asset_id`: the existing `social.read_comment_events` definer
     was also missing the schema USAGE (a latent defect: it had never been exercised, the A2 IG read now is).
   - `internal/live/stream_facts.go` (new): `CommentStream.Facts` (a: bridge, b: IG webhook copy decrypted API-side with the
     payload keyring, c: `ErrFactsUnavailable`), `PrivateReplyMarks` and `ConsoleMarks.WithFactsUnavailable`
     (`private_reply_unavailable_reason: "facts_unavailable"`; `used`/`auto_pending` keep precedence). FB found:false stays
     `Found=false` (A4 `comment_unknown`); IG with neither source is `ErrFactsUnavailable`; a bridge outage with no IG copy is
     the retryable `ErrStreamUnavailable`. `live.console_marks` keeps its frozen signature; the reason is applied API-side.
   - Poller: Instagram sources are no longer forward-polled (they were polled with the FB comment field set; the file header
     already said they are not). IG sources are only acquired on demand for comment-facts.
4. **Wiring.** Already built into `cmd/api` (`buildCommentStream`) and `cmd/claims-worker` (bridge + sweep) by the author;
   this unit adds a DB-free full-router test `TestLiveStreamRoutesCoexistWithInboxAndTemplates` (A2/A3 + inbox + templates
   mounted together). The new `Facts`/`PrivateReplyMarks` have no HTTP route yet: their consumer is the LC-B4 A4 planner
   (`409 comment_facts_unavailable`) and the A8/A13 marks (LC-B3/B4).
5. **Exact-privilege rows added to `tests/foundation/live_claims_schema_test.go` (KC03), only LC-B2's own objects:**
   column SELECT `claims.events.reason`, `integration.operations.{semantic_key,result_code,created_at}`,
   `live.comment_poll_leases.{tenant_id,store_id,source_id,demand_until}`; table SELECT/INSERT/UPDATE `live.comment_prints`
   (+ its eight columns); EXECUTE `live.{comment_poll_sources,console_source,console_marks,comment_print}`; owned-object
   count 23 -> 27 (those four definers).
6. **Test timing (harness, not a gate):** `LCN01 idle-drop-reset` used `IdleDrop=20ms`, which flaked when the shared Docker
   host was loaded (page1 items=0 before the first poll); now `IdleDrop=3s`, sleep 3.5s. The asserted behaviour is unchanged.
7. Docs: `docs/delivery/GATES.md` (`--live-console` row), `scripts/dev/test-local.sh` (`--live-console` now runs
   `^TestLiveConsoleLCN` only, 300s; Inbox/Templates keep their own modes), `contracts/live-console-v1.md` unit row/A1.6 note.

### New tests
- `tests/foundation/live_console_comments_facts_test.go`: `TestLiveConsoleLCN02IGFactsBridgeGraphBranch` (IG page/buyer/reply/
  missing-author/unknown, FB/IG field sets and bearer-in-header asserted on the MOCK Graph) and
  `TestLiveConsoleLCN02IGFactsFallbackAndUnavailable` (a Graph facts, b real signed IG webhook -> inbox -> consumer ->
  `social.read_comment_facts` decrypted API-side for page/reply/plain, media-mismatch copy not this source's, c
  `facts_unavailable` + marks reason, no-keyring api, foreign session, bad ref, FB unknown = Found:false, bridge-down 503 vs
  copy-still-answers, A2 IG page through `social.read_comment_events`, IG author id never persisted in clear).
- `internal/integrations/metareply/comment_facts_test.go` (DB-free table test), `internal/httpapi/live_stream_test.go` (router).

### Commands and exit codes (logs in `output/lc-b2-comments/`)
| command | exit |
|---|---|
| `bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN'` (8 tests: LCN01 x3, LCN02 x3 incl. 2 IG, LCN04, LCN05) -> `green-lcn.log` | 0 |
| `bash scripts/dev/test-focused.sh '^(TestLiveClaims\|TestLiveTools\|TestLiveSessionFlow\|TestMetaClaimsMCI0\|TestLiveClaimsKC03\|TestLiveConsoleInbox\|TestLiveConsoleTemplates)'` -> `green-regression.log` | 1 first run: 75 PASS, 1 FAIL `TestMetaClaimsMCI07SendAndOutcomes` (River job timing under a loaded shared Docker host) |
| `bash scripts/dev/test-focused.sh '^(TestLiveConsoleLCN01BufferCapAgeAndCursor\|TestMetaClaimsMCI07)'` -> `rerun.log` | 0 |
| `bash scripts/dev/test-focused.sh '^TestLiveClaimsKC03'` -> `kc03.log` | 0 (after the rows in item 5) |
| `bash scripts/dev/test-focused.sh '^(TestMetaConsumer\|TestMetaInbox\|TestMetaClaimsIntake\|TestExternalOperation\|TestLegacyRuntime\|TestMetaRuntime\|TestClaimsRetention)'` (guards the new meta_writer grants) -> `green-wide.log` | 0 (69 PASS) |
| `go build ./... && go vet ./internal/... ./cmd/... ./tests/foundation`; `gofmt -l internal cmd tests` | 0; no output |
| `go test ./internal/live/... ./internal/httpapi/... ./internal/integrations/metareply/... ./internal/integrations/meta/... ./cmd/api/... ./cmd/claims-worker/...` | 0 |
| `bash scripts/dev/check-headers.sh` | 0 |
| `bash scripts/dev/check-gates.sh` (node_modules symlinked from r3-integration, link removed afterwards) -> `check-gates.log` | 0 (67 modes documented) |

### NOT_RUN / limits
- `bash scripts/dev/test-local.sh --live-console` itself was not run: the same tests ran through `test-focused.sh` (identical pinned PG image
  and fixture guard) because the shared PG slot is queue-locked; the mode's dispatch only differs by `-race`. **Not run with `-race`.**
- A first wider selection (adding `TestLiveMedia|TestLivePlanning|TestMetaAds`) hit the 900 s focused timeout and showed an unrelated
  `TestLiveMediaRecoveryMRR02TimelyReadbackWitnessCommitsAfterNinety` failure; it was not pursued (media code untouched).
- IG Graph field set is MOCK until probe R3 / LC-U12; no LIVE Meta call; `created_at` for the IG webhook copy is delivery time (U7).
- Poller token load still requires attested scope `pages_read_engagement` for IG sources too (author behaviour); LC-U12 decides if IG needs
  `instagram_manage_comments` instead.
- No HTTP route exposes `Facts`/`PrivateReplyMarks` (LC-B4 A4 / LC-B3 A8,A13 consume them).
