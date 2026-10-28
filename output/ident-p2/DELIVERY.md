# IDENT-P2 delivery — #215 (OC-02) and #228 (OC-03) P2 follow-ups

Branch `unit/issue-215-228-identity-p2` (base r3/integration `5365b5701`), PR #265 into `r3/integration`.
Body: "Closes humaux/live_commerce_architecture_v1#215, humaux/live_commerce_architecture_v1#228".
Backend only; nothing under `apps/**` is touched. Evidence class: real-PG gates run on GitHub CI only (shard:unit),
MOCK mailer / MOCK IdP. Local: go vet and static checks only.

## Commit layout

- RED `05c641166` — tests only (no production/migration change): added OG04 ordering/interval, OG07, OG08/OG09,
  RA08/RA09/RA10. Expected failing test: `TestGuardOG07OIDCReauthCapsInfiniteAuthenticatedAt`.
- Implementation `e4510f742` — `migrations/0207_recent_auth_followups.sql`, R2 pin 137, contract §16, deploy note,
  DEFINERS.md, package-doc blank lines + regenerated dependency map.
  Its historical unit and Foundation results do not establish GREEN for the fix-2 head.

## Item disposition

| Item | Disposition |
|---|---|
| #215.1 OIDC auth_time must not move authenticated_at back | Already fixed on base: `migrations/0203_recent_auth_guards.sql:75-78`, asserted by `tests/identity/reauth_guard_sql_test.go:333` (OG05). Skipped. |
| #215.2 future/infinity has no upper bound | Already fixed on base: `migrations/0203_recent_auth_guards.sql:71-74`, asserted by `tests/identity/reauth_guard_sql_test.go:362` (OG06). Skipped. |
| #215.3 reauth.go:102/189 limits and PD5 budget untested | `TestReauthRA08ReauthThrottleKeys` (`tests/identity/reauth_test.go:738`) drives both call sites to ThrottleError on the 11th call; `TestReauthRA09PD5BudgetRefusesBeforeCompare` (`:774`) covers the 10-per-ip and 50-total branches (`migrations/0201_recent_auth.sql:279-283`). |
| #215.4 RA02 only asserts PUBLIC | Already fixed on base: `tests/identity/reauth_guard_sql_test.go:259-286` asserts the exact EXECUTE grantee set via `aclexplode`. Skipped. |
| #215.5 0070:351 INSERT unobservable behind DEFAULT | `TestReauthRA10IssuePathWritesAuthenticatedAt` (`tests/identity/reauth_test.go:843`) drops the column DEFAULT inside the transaction and issues a sign-up session through `identity.complete_email_challenge`. |
| #215.6 table-wide backfill lock | Deploy note added to `deploy/README.md` (R3 release section); no data migration. |
| #215.7 output/oc-02 GREEN missing | Already fixed on base: `output/oc-02/DELIVERY.md:42` records run 40400054009 / job 121850396888 (verified: head fed149133, success). Skipped. |
| #215.8 package doc hijacked by file headers | Blank line before `package identity` in `internal/identity/reauth.go:11` and `internal/identity/oidc_reauth.go:11`; `docs/engineering/dependency-map.md:290` regenerated from service.go's package doc. |
| #215.9 remaining Opus P2 | The report file is not in this repo; the enumerated items above cover the ones restated in #215. NOT_RUN for any unlisted report item. |
| #228.1 OG01/OG02 only 2 hours | `TestGuardOG08ElevenMinuteSessionRefusedByEveryDefiner` (`tests/identity/reauth_guard_sql_test.go:404`) and `TestGuardOG09NineMinuteSessionPassesTheGuard` (`:416`). |
| #228.2 OG04 only strings.Contains | `TestGuardOG04...` now asserts the `interval '10 minutes'` call and that it precedes the first write/lock token (`tests/identity/reauth_guard_sql_test.go:242-303`). |
| #228.3 staff trio wording | `output/oc-03/DEFINERS.md` header now states the `staff_owner_ctx` lock exception. `migrations/0203` is checksum-frozen (`migrations/migrate.go:4,253-257`), so its header is not edited; 0207 records that. |
| #228.4 hardcoded 10 min vs COMMERCE_REAUTH_WINDOW_MINUTES | Recorded 10 minutes as the fixed contract value in `contracts/merchant-password-auth-v1.md` §16; the env var only tunes when the UI prompts. |
| #228.5 +1 minute future skew | Documented the 1-minute clock-skew upper bound and the infinity cap in §16. |
| #228.6 infinite authenticated_at survives GREATEST | Fixed by `migrations/0207_recent_auth_followups.sql` (outer `LEAST(..., clock_timestamp())`); RED/GREEN test OG07 (`tests/identity/reauth_guard_sql_test.go:378`). |
| #228.7 set_bank_transfer_settings unlisted | Added to `output/oc-03/DEFINERS.md` "Not guarded, and why" with the owner decision still open (no guard added). |
| #228.8 carrier/pick-list CSV no card | `output/oc-03/DEFINERS.md` now records that a card is still needed; no code change here. |
| #228.9 integrator tasks.json/write_paths | Integrator-owned; not touched by this PR. |
| #228.10 no deployment before OC-15 | Restated in the PR body; no code change. |

## Migration numbering

`0207` is this PR's only migration. The integrator confirmed the serial order #254 (0205), #262 (0206),
then #265 (0207). Fix 2 merges `origin/r3/integration` at `2bc6d482606a2b0583af15789086fdad727b27ec`
after GitHub confirmed #262 merged at 2026-10-28T23:01:15Z. The base's exact R2 count is 136;
this PR retains 137 (base + 1). No migration was renamed or changed.

## Fix 2 evidence and handoff

- Authority: integrator's fix-2 card; worktree `ident-p2`, branch above, base `2bc6d4826`.
  Executor: Codex; runtime does not expose a verifiable model/effort identifier. No delegated agents.
- Historical causal RED, verified from raw CI log: run `40655692256`, job `122727348722`,
  SHA `05c641166990fcc04470da95b63497e5acd5e1c8`. OG07 asserts finite/capped auth time and fails with
  `infinity=true capped=false, want false/true`. Filtered original lines: `red.log`.
- Prior-head failure: run `40634706241`, job `122681056072` (`shard:g04`),
  `TestR2IntegrationUpgradeFromReleaseHead` found 135 files, expected 137.
  Filtered raw evidence: `fix2/g04-prior.log`. Merge brings the two missing files; pin stays exact.
- Read all failure jobs on the prior head: the same run's receipts `122717034017`, aggregate `122717034150`,
  required `122740388437`; cancelled dispatch `40634706260` receipts `122654268303` and required `122680396541`;
  cancelled dispatch `40664915076` required `122757083424`. Missing receipts/aggregate slices and cancelled
  upstream jobs explain those dependent failures; none is acceptance for the new head.
- Also inspected cancelled jobs: g10 `122681056834` exposed an actual fixture dependency error before cancellation:
  `stripe_live_schema_test.go:628` ran 0207 without the 0203 function shape (`unexpected shape ... 0`).
  Fix: defer 0207 with 0203 in the pre-0077 fixture, assert it is absent before upgrade, and require exactly ten
  migrations on the real Apply (previously nine). No production patch anchors or privilege assertions change.
  Filtered raw evidence: `fix2/g10-prior.log`.
  g07 `122681057273` and CVS `122681057149` logs end in cancellation; meta-health `122681057293` is cancelled
  and GitHub returns no job log, so its test outcome is UNKNOWN. These require new-head CI.
- Static checks: `gofmt -d tests/foundation/{r2_integration_upgrade,stripe_live_schema}_test.go` exit 0, no diff;
  `bash scripts/dev/depmap.sh` exit 0, generated map unchanged from the merge.
  `GOTOOLCHAIN=go1.27.3 go vet ./tests/foundation` exit 0. No local tests were run.
  Imported integration whitespace findings were left untouched; card-owned diff is checked separately.
- New-head Gates and Foundation verification: PENDING. Run IDs will be recorded in the PR comment and handoff,
  without an evidence-only push. Integrator relays completion; no local watcher/server is left running.
- Scope beyond OC-02/OC-03 write paths: `tests/foundation/stripe_live_schema_test.go` fixes this card's migration
  dependency in the upgrade fixture; integrator approval required. Existing contract/deploy edits and
  `output/ident-p2/**` are also listed in the PR.
  Merge-only changes belong to the integration base and introduce no additional authored module edits.
- Evidence: E3 for historical raw CI failures only; author assessment E0. Current-head completion is not claimed.

## NOT_RUN

- Local `go test` / PostgreSQL (repo rule). All test results are the CI runs above.
- New-head GREEN and independent review (await CI and integrator).
- Real IdP / real SMTP / deployed environment.
- Item #228.8 implementation (needs a card) and #228.7 guard (owner decision).
