# OA-1 delivery — in progress

- Branch: `unit/oa-1-order-attempt-core`; base: `55e98ba0434557e94e3b7c9e31ff0e98352623b1`.
- Implementer: Codex-3; reasoning: high. Allowed paths and boundaries:
  `docs/delivery/units/oa-1-order-attempt-core.md`.
- Contract: frozen order-attempt-v1 plus reviewed A1/A1.1 at
  `cb4256f37586aad5139f1216fc92d6226fa70003`, merged in `0f1060a72`.
  Retired scopes reject fresh CAS, but preserve authorized exact replay during
  retention. Backend separate from paused PR #32.
- Current artifact: storage, private admission helpers, C1 SQL coordinator and
  Go orchestration; C1 is under CI review. C2 execution remains incomplete;
  the new placement path is not enabled.

## Evidence

| Revision | CI run | Gate | Result |
| --- | --- | --- | --- |
| `b58b834d4` | [38422920898](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38422920898) | `focused:^TestOrderAttemptSchema` | RED: `PASS=0 FAIL=3 SKIP=0 exit=1`; missing identities/tables/execution entries after successful REAL_PG setup; job `115385256709`, `schema-red-ci.log` |
| `0232733e2` | [38424566780](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38424566780) | `focused:^TestOrderAttempt(Provenance\|Model)` | REAL_PG provenance RED (5); MODEL_ONLY seeds/oracle PASS (2), `PASS=2 FAIL=5 SKIP=0 exit=1`; job `115391256051`, `provenance-model-red-ci.log` |
| `efcdd3998` | [38426328576](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38426328576) | `focused:^TestOrderAttempt(Schema\|Provenance\|Model)` | REAL_PG strengthened baseline RED; `PASS=2 FAIL=11 SKIP=0 exit=1`, job `115396767591`, `strengthened-red-ci.log` |
| `1e616d320` | [38426867350](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38426867350) | storage identity/authority/incomplete-admission + model | `PASS=4 FAIL=1 SKIP=0 exit=1`; incomplete-admission fixture stopped before commit with SQLSTATE 42P08 (UUID/text parameter inference). Not behavioral proof; fixed explicit UUID cast, assertions unchanged. Job `115399906105`, `storage-first-ci.log` |
| `68ba72cc9` | [38428406310](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38428406310) | storage/provenance/state/model/authority | `PASS=16 FAIL=4 SKIP=0 exit=1`; actual storage commit and state tests PASS; three authority tests RED on absent C1 entries, one provenance RED on missing direct ledger/session FK. Job `115404898781`, `storage-provenance-authority-ci.log`. Exact FK added; assertions unchanged. |
| `14065204e` | [38429245975](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38429245975) | `focused:^TestOrderAttemptFacts` | RED: `PASS=0 FAIL=6 SKIP=0 exit=1`; successful REAL_PG setup, coordinator has no checkout access and facts function is absent. Job `115406897065`, `facts-red-ci.log`. |
| `2dc37d8cc` | [38430215747](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38430215747) | OA focused seams | `PASS=7 FAIL=25 SKIP=0 exit=1`; two model and five codec tests passed. PG setup failed on 0173 PL/pgSQL CASE parsing (42601), so PG behavior was NOT_RUN. Job `115410251105`, `admission-seams-first-ci.log`. Parenthesized IF-contained CASE conditions in `fa6a57dd7`; assertions unchanged. |
| `b54e4ee0b` | [38430742365](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38430742365) | real C1 preparation | `PASS=0 FAIL=2 SKIP=0 exit=1`; same migration setup failure, not causal C1 proof. Job `115412090363`, `admission-first-ci.log`. |
| `ce14e8892` | [38432101298](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38432101298) | `focused:^TestOrderAttempt` | `PASS=32 FAIL=18 SKIP=0 exit=1`; storage/provenance/authority/facts and codec/model pass; absent C1, subject and job helpers are causal RED. One subject isolation fixture has a wrong tenant/store pair (23503) before its assertion; corrected separately, not counted as isolation proof. Job `115416759643`, `c1-seams-red-ci.log`. |
| `4f1d17643` | [38432782403](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38432782403) | A1 retirement/schema baseline | RED `PASS=1 FAIL=5 SKIP=0 exit=1`: missing partial unique predicate, reactivation refused, pending retirement accepted, principal lock absent. Job `115419129700`, `scope-retirement-red-ci.log`. |
| `21cf485b3` | [38433099080](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38433099080) | claim/job implementations plus native fixtures and replay RED | `PASS=27 FAIL=2 SKIP=0 exit=1`; claims, job guards, state, facts and provenance pass. Two exact-replay tests causally RED because prepare is absent (42883). Job `115421727992`, `c1-claims-jobs-ci.log`. |
| `ad755a9e9` | [38434570109](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38434570109) | retired scope, subjects and C1 SQL; catalog quantity boundary RED | `PASS=26 FAIL=1 SKIP=0 exit=1`; retirement, retained exact replay and other admission tests pass. Quantity 1000 causally refused with PT400; fixed in `66f0167a0` without changing claim ledger allowances. Job `115427121077`, `admission-boundaries-ci.log`. |
| `11bb42f68` | [38434841389](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38434841389) | Go C1/calculation and replay authorization during scope wait | `PASS=3 FAIL=2 SKIP=0 exit=1`; revoke orders:read/session while waiting returned a receipt instead of PT403/PT401. Catalog-only revocation replay passed. Fixed final current-read check in `999f4282d`. Service atomic positive readback used nonexistent `orders.buyer_owner_id`; corrected to actual `owner_id` in `a9f1b3b74`, assertions unchanged. Job `115427367242`, `c1-service-replay-red-ci.log`. |
| `598aa6b44` | [38436791173](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38436791173) | C1 repairs plus execution-fence/context tests FIRST | `PASS=20 FAIL=7 SKIP=0 exit=1`: C1 repairs, exact retired replay, post-wait read revocation and atomic admission PASS. Seven execution-fence cases causally RED on absent entry/ACL. Job `115433959896`, `execution-fence-red-ci.log`. |
| `8cdb9f8a4` | [38438426265](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38438426265) | frozen origin/CVS source facts FIRST | RED `PASS=0 FAIL=5 SKIP=0 exit=1`: missing frozen origins/pickup facts, zero origins and invalid/disabled CVS codes accepted. Pay-at-pickup positive also returned unavailable and requires separate diagnosis. Job `115439047589`, `materialization-facts-red-ci.log`. |
| `ad9a0efa9` | [38439304096](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38439304096) | CI-only formatting receipt before focused fence/quote tests | Formatting emitted `oa1-gofmt.patch` then exited 1; Gate step did not run: **NOT_RUN**, not behavioral RED. Applied the CI-produced formatting diff without executing a local formatter, and removed the temporary workflow step. Job `115441543152`, `execution-fence-first-ci.log`; artifact `gate-focused_TestOrderAttempt_ExecutionFence_QuoteMaterialization`. |

CI is the only execution environment. Local tests/builds/gates/server/PG:
**NOT_RUN(owner CI-only directive)**. Static source review does not prove C2.
CI workflow gate timeout is 150 minutes; the parent tracks the run and reads
its terminal result/log. No acceptance claim until assertions execute.

## Contract-clause → test mapping

- §1/§2/§6k → `TestOrderAttemptSchemaDurableIdentities`,
  `TestOrderAttemptSchemaTableAuthority`, `TestOrderAttemptSchemaExecutionDefinerACL`.
- Remaining §6 matrix and mutation proofs: NOT_RUN / implementation pending;
  planned mapping is in the unit brief. No schema-only or MODEL_ONLY result
  is described as REAL_PG placement acceptance.

## CI gates and open work

Focused OA model/fuzz and REAL_PG matrix; all affected foundation shards;
full migration upgrade/count/ACL pins; contract drift and repository structural
gates; process wiring and affected browser modes; independent money/privacy
review. Exact migration numbers will be rechecked against trunk before PR.
Local notification planning and erasure lock prefix are prerequisites of C2;
provider dispatch and public cutover remain dependent units.
