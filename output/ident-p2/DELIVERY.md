# IDENT-P2 delivery — #215 (OC-02) and #228 (OC-03) P2 follow-ups

Branch `unit/issue-215-228-identity-p2` (base r3/integration `5365b5701`), PR #265 into `r3/integration`.
Body: "Closes humaux/live_commerce_architecture_v1#215, humaux/live_commerce_architecture_v1#228".
Backend only; nothing under `apps/**` is touched. Evidence class: real-PG gates run on GitHub CI only (shard:unit),
MOCK mailer / MOCK IdP. Local: go vet and static checks only.

## Commit layout

- RED `05c641166` — tests only (no production/migration change): added OG04 ordering/interval, OG07, OG08/OG09,
  RA08/RA09/RA10. Expected failing test: `TestGuardOG07OIDCReauthCapsInfiniteAuthenticatedAt`.
- GREEN `<green sha>` — `migrations/0207_recent_auth_followups.sql`, R2 pin 137, contract §16, deploy note,
  DEFINERS.md, package-doc blank lines + regenerated dependency map.

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

`0207` is this PR's only migration. `0205` (OC-05) and `0206` (CVS snapshot) are reserved by open PRs that were not
merged when this PR opened; the R2 pin is 137 (134 + 0205 + 0206 + 0207) as instructed. Until those two merge, the
foundation R2 count leg sees 135 files and fails; the unit shard that carries the RED/GREEN assertions is independent.

## NOT_RUN

- Local `go test` / PostgreSQL (repo rule). All test results are the CI runs above.
- The foundation R2 count leg is expected red until 0205/0206 merge (see above).
- Real IdP / real SMTP / deployed environment.
- Item #228.8 implementation (needs a card) and #228.7 guard (owner decision).
