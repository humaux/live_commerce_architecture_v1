# Browser input worker — scoped validation, 2026-09-27

Status: **SQL_EXECUTOR_HTTP_SUBSET18_PASS; BRW05_HTTPS_DELIVERY_PASS; HISTORICAL_FULL709_PASS; PRODUCT_GATES_PENDING**.
The latest bounded HTTP test tree is `53436b6`; the historical full709 source is
`c5160de`, not the new HTTP source. This is not complete BRW/BRI, T08, G06 or SaaS
acceptance. Input routes require an explicitly supplied runtime; production
command wiring remains disabled.

## Implementation and ownership

- Frozen contract: [BRW](../../contracts/live-browser-input-worker-v1.md),
  independently reviewed at `f518c77`.
- Go author: `brw_execution_impl`, integration_worker, gpt-6-sol/high,
  isolated `commerce-meta-inbox-go-20260926`, base `59d149f`;
  `0742fc4` plus `f19cf16`, integrated as `5abcf71` and `ebf8165`.
- Independent tests: `brw_execution_tests`, test_worker, gpt-6-sol/high,
  isolated `commerce-meta-inbox-tests-20260926`, same base. Final tests are
  `9fed698`, integrated at `c25e7d0`. Preexisting `output/` was preserved.
- Later fault-test cohort: `1224d05` / `f814976` / `0bef336`, integrated as
  `361e3ef` / `ea494b6` / `3b7fb56`, test-only and separate from full709.
- Planner fault tests finish at `6815e13`; cleanup result ACK loss and exact
  persisted cutoff at `c4c1354`; independent liability holds at `5611e30`.
  Root's bounded, exact-FK fixture cleanup correction is `b446e49`.
- Root owns forward migrations `0042` / post-River `0011`, exact platform
  authority allowlist, the bounded runner and owner-fixture teardown ordering.
- Independent bounded SQL review: `44fd4cf1-4f2f-409b-9d15-363ae1f6f0e1`.
  Go/platform review found one P1; independent closure after correction:
  `91b09197-79d5-411a-9e4b-a7dd8c8f82fb`. No remaining confirmed P0/P1 in
  those source-review scopes, not a substitute for unrun product gates.

The local input consumer reuses the original River operation/job and existing
LiveKit/Egress helpers. No new dependency, transaction engine, Stop queue or
provider abstraction was introduced. An immutable runtime marker separates
the old custody kernel from this explicitly selected local runtime. Each
provider mutation needs a committed lease-fenced reservation. Input cleanup
and Egress responsibility remain separate; neither a TTL nor an Egress terminal
result can erase unresolved input responsibility. Production entrypoints still
do not construct this consumer or expose input tokens.

## Executed evidence

All paths below are under `/Volumes/data/output/`. Each listed process exited.
The full run covers source `c5160de` and tests through `c25e7d0`; it does not
include the six later top-level test additions. The final 15-test cohort and
old BIC rerun below cover test tree `b446e49`, with product source unchanged.

|Check|Actual result|Log|
|---|---|---|
|BRW SQL and actual original-job worker, final query|Exit 0; 9 top-level PASS, 0 FAIL/SKIP; foundation 25.856s; race and vet|`brw-runtime-null-allowlist-green-20260927.log`|
|Same tests, previous vulnerable query|Exit 1; 8 PASS, 1 FAIL; both old validator/client incorrectly accepted the grant|`brw-runtime-null-allowlist-red-20260927.log`|
|Affected six Go packages, after correction|Exit 0; race and vet|`brw-runtime-go-postfix-20260927.log`|
|Old Stop regression, before final allowlist correction|Exit 0; 53 PASS, 0 FAIL/SKIP; 250.751s|`brw-runtime-stop-regression-20260927.log`|
|Old BIC regression, before final allowlist correction|Exit 0; 6 PASS, 0 FAIL/SKIP; 20.917s|`brw-runtime-bic-regression-20260927.log`|
|Full fixed-source `c5160de` PG/race/vet|Exit 0; 709 top-level PASS, 0 FAIL/SKIP; foundation 1122.264s|`brw-runtime-full-root-20260927.log`|
|New fault tests `361e3ef`, unchanged product source|Exit 1; final-reservation ACK loss PASS 36.05s; concurrent revoke case blocked until the 240s suite limit; not accepted|`brw-fault-root-first-20260927.log`|
|Repaired fault fixture `ea494b6`, same product source|Exit 0; 11 top-level PASS, 0 FAIL/SKIP; foundation 64.602s|`brw-fault-root-repair1-20260927.log`|
|Final fault assertions `3b7fb56`, exact causal SQLSTATE|Exit 0; 11 top-level PASS, 0 FAIL/SKIP; foundation 62.405s; race and vet|`brw-fault-root-exact-rejection-20260927.log`|
|Planner rollback/commit ACK loss, exact pre-ACK receipt and denial|Exit 0; 12 top-level PASS, 0 FAIL/SKIP; foundation 63.266s; race and vet|`brw-plan-fault-root-final-20260927.log`|
|Cleanup result ACK loss, before exact-cutoff strengthening|Exit 0; 13 top-level PASS, 0 FAIL/SKIP; foundation 77.939s; race and vet|`brw-cleanup-fault-root-20260927.log`|
|First dual-hold cohort, faulty shared fixture teardown|Exit 1; foundation 93.858s; FK failure contaminated later cases; not accepted|`brw-cleanup-holds-root-final-20260927.log`|
|Final exact-cutoff/dual-hold cohort, corrected shared teardown|Exit 0; 15 top-level PASS, 0 FAIL/SKIP; foundation 90.968s; race and vet|`brw-cleanup-holds-root-repair1-20260927.log`|
|Old BIC after shared teardown correction|Exit 0; 6 top-level PASS, 0 FAIL/SKIP; foundation 20.030s; race and vet|`brw-bic-teardown-regression-20260927.log`|

Final GREEN SHA-256:
`eb529135b24ed333f3513674dd2febed853199397f2c1917ecc2f6e06d985e80`.
RED SHA-256:
`d02e41504df212ed967de52b8c15e159493282b7c4a301c46b4f878d119aa533`.
Post-fix six-package capture SHA-256:
`29c688de7084d02d6b82045ef43a5958a9a50ba70007a6525f28537d7df1211f`.
Full 709 SHA-256:
`8b57825341033ffb37123b4ee3714db83df80cc3982ad1e6a53d34bb2d1deed3`.
New fault first-run SHA-256:
`aef40f5c71aa57651c0e907589ac98c1025cf613747852d41bb371d0611bb76e`.
Final fault 11 SHA-256:
`7b0d6f863b098a1f37b784035268ecae84092de04164c13243a84ab0159511f4`.
Final planner 12 SHA-256:
`46e605d5f5643fde9f9a7ce7a7d2542900c1a71733b335c57336c481969591d6`.
Initial cleanup 13 SHA-256:
`319143fcec152b9a0e27d6f23e9180e1a601f53eaa69dbe90ea20047d942939c`.
Failed dual-hold cohort SHA-256:
`6a52fc35c0eecade735d385ba238dbb07f353c6b5c7b9d3f67ec95a6d48ef99b`.
Final 15 SHA-256:
`709cf3c95efe3fa1a63d6d669c6d418d4b58823414973ce0048543e712cd0004`.
Old BIC rerun SHA-256:
`7028dbbd977dd7853295f1c210341c56082eb987d822335b51af75c1b16f7399`.

The complete run includes `TestLiveMediaStopLMR05RealCrashAndCommitAckLoss`
PASS in 56.46s with the owner-approved 90s upper bound and unchanged safety
assertions. Existing MRR90 scenarios also pass; they remain separate evidence
from recovery of this new input queue. Each runner removed its own disposable
PG fixture on exit. No customer resource was stopped or changed.

## Failures and root-cause repairs

1. Earlier PG tests exposed missing persisted turn selection, an ambiguous
   PL/pgSQL `generation` reference and the absent new executor ABIs in the
   strict Go allowlist. Corrections preserve exact signatures and lease fences.
2. Owner-fixture teardown formerly removed the native job before its business
   ownership. It now deletes only the fixture's operation first under deferred
   constraints; the production unresolved-custody guard is never disabled.
3. A nil result from the shared Egress helper formerly completed the input job.
   The input wrapper now snoozes instead. Actual worker tests prove Egress
   terminal state retains the same unresolved input job and permits cleanup.
4. River 0.40 short snoozes use `available`, not necessarily `scheduled`.
   Tests require persisted `metadata.snoozes`, `attempted_at`, nonterminal state
   and no finalization. They do not merely accept an untouched available job.
   The first projection read uses a left join until the worker creates its
   execution row. Original waits and all no-I/O/no-Start assertions stay fixed.
5. Missing `to_regprocedure` results introduced NULL into an ABI allowlist.
   SQL three-valued `NOT(IN ...)` then hid unexpected EXECUTE grants from the
   legacy client. The independent PG counterexample renames one ABI while
   preserving its OID/grant. Filtering NULL in the two shared membership checks
   fixes the cause; the unchanged test fails before and passes after correction.
6. The new concurrent-revoke fixture held the operation row while awaiting
   revocation. The final-reserve waiter already held authorization SHARE;
   revocation needed its UPDATE lock, creating a test-orchestrated cycle.
   Holding the earlier binding lock instead allows revocation to commit first,
   then the still-real blocked reservation must reject with SQLSTATE `ME409`.
   Stop also has a real observed lock interleaving. Bounded call contexts prevent
   indefinite fixture hangs; deadline/EOF cannot satisfy the causal assertion.
   No product code or original safety/wait threshold changed in this repair.
7. The new dual-hold fixture reached a Stop observation/projection FK cycle.
   Separate autocommit teardown deletes failed, leaving an `EG_brw` fixture
   row that contaminated later cases. `bicSetup` now uses one owner-only,
   10-second transaction, defers only `live.media_stop_observation_fk`, performs
   the same scoped deletes and revalidates at commit. It neither disables
   constraints nor changes product facts. Independent exact-diff review
   `6c858eee-0d72-448e-8912-7d41e91515a0` found no P0/P1; final 15 plus old BIC6
   pass. First-run failure evidence is retained.

Earlier failing logs (`brw-runtime-root-first`, `brw-runtime-root-repair1`,
`brw-runtime-root-repair2`, all suffixed `-20260927.log`) remain available.
No failing assertion or evidence was deleted to obtain a pass.

## Stop lines and next gates

The fifteen top-level tests cover a **subset** of BRW01–04: real-role isolation,
marker/replay mapping, dual-track final admission, exactly one Start, eight
bounded cleanup slots, pending-reservation loss and original-job continuation.
They are not blanket acceptance of all concurrency, process-crash or ACK-loss
permutations in those gates. The later `361e3ef` test-only cohort adds actual
final-reservation COMMIT-ACK loss and concurrent Stop/revoke cases. Its first
run passed ACK loss but timed out in the revoke case. The failure is preserved;
the corrected final `3b7fb56` cohort passes, including exact causal denials and
committed-ACK-loss recovery on the original job without another Start. These
later tests are not silently included in the historical full709 result.
Planner tests now prove full transaction rollback and committed-ACK-loss replay
with exact pre-ACK receipt identity and marker-zero denial. Cleanup tests use
actual local TLS Remove/Delete/GetParticipant/ListRooms endpoints, compare
Remove cutoff to the persisted reservation, and drop a committed cleanup-result
ACK after server completion. The two hold tests explicitly seed legal owner-only
hold states, then prove real worker selection and execution in both directions;
they do **not** prove production transitions naturally enter those holds.
Per-step reservation/process loss, actual 180-second deadline, remaining
logout/access/lifetime permutations and other BRW01–04 cases remain separate
work. BRW05 post-commit Go HTTP and HTTPS browser token delivery have the bounded
evidence below. BRW06
actual product browser/SFU decoded A/V, BRW07 new-input-queue 90-second recovery,
and BRW08 final integrated regression/Studio evidence remain pending. The
source-level full709 regression passed, but that alone does not close BRW08.
Existing MRR
90-second proof does not automatically cover this queue. The old LMR05 wait is
the separately owner-approved 90 seconds; its safety conditions were not changed.

Recovery research found River's default one-hour orphan-job rescue and shared
schema-wide maintenance; changing only this consumer's rescue setting would not
prove a 90-second bound. A reuse of the existing fenced read-only recovery
observer is under design review. State checked is not cleanup completed or
native job resumed. No rescue policy, clock, or recovery acceptance gate has
been changed to manufacture a pass.

All external calls above use disposable local TLS fixtures. No customer
stream, payment, provider credential, Cloud or production configuration changed.

## BRW05 post-commit token delivery increment

Go source `690fe34` and BFF source `7050635` reuse the existing scoped transaction,
strict decoder, signed session and store authorization. The token signer runs
only after `WithScope` returns a successful COMMIT acknowledgement. The response
is an exact six-field DTO bounded to 8192 bytes, with private no-store caching;
errors do not echo credentials. `cmd/api` still supplies no browser-input runtime.
No SQL migration, new dependency or general callback framework was added.

Independent HTTP tests `8607063` were integrated as `53436b6`. The author's four
Go files and root's four BFF files received a bounded independent review with no
confirmed P0/P1; this is not a claim of whole-product review.

|Check|Actual result|Log under `/Volumes/data/output/`|
|---|---|---|
|BRW SQL/executor + Go HTTP on `53436b6`|18 top-level PASS, 0 FAIL/SKIP; exit 0; 81.114s|`brw05-go-http-root-first-20260928.log`|
|Existing Studio backend after shared-parser extraction|8 top-level PASS, 0 FAIL/SKIP; exit 0; 13.073s|`brw05-studio-regression-root-20260928.log`|
|Existing signed-login browser BFF on `7050635`, before Go increment|`TestBrowserStudioBFFRealChain` PASS; exit 0; foundation 6.793s|`brw05-bff-legacy-root-20260928.log`|

SHA256 values in table order:

```text
8d295a485a90a8d3fea99893a155800b735368a5b7b1b9aa00121071bd24e41d
e3b06bf9e71ee6fecc782c27c8aa4b3008d160510b8b0043b75a19c82e746ad0
34ba5ad255e4d43262be7f1581c4e1ce7cc83fbd69128a67ba9016a931ee16e6
```

The three new Go HTTP cases cover token commit/replay, strict input and authority,
and actual committed-but-lost-ACK behavior: no token is returned on the failed
acknowledgement, while healthy replay retains the original grant expiry. They
also check exact claims/signature and absence of JWTs from durable receipts.
Root Go unit race tests and vet passed for `internal/httpapi`, `internal/live`
and `internal/integrations/livekit`.

The old browser regression does **not** exercise the new HTTPS input routes.
The separate new HTTPS token-delivery result is recorded below. Actual SFU
decoded media, INPUT recovery and the new-source full regression remain unclosed.

### First HTTPS browser attempt (not accepted)

The local `--browser-input-delivery` run on `93ce8a2` exited 1 after 3.41s
(foundation 5.839s). The Stop phase passed preceding login/token/replay/WSS and
denial assertions, then failed the forged-Origin assertion: expected 403,
received 409. Because that negative ran after Stop, its response alone does not
prove the hostile Origin reached the BFF. Root-cause review and a causal test
repair are pending; the required 403 is unchanged. The revoke phase did not run.

- Log: `/Volumes/data/output/brw05-https-browser-root-first-20260928.log`.
- Browser evidence: `output/playwright/input-delivery-20260927T163548.588591000/`.
- Disposable test resources were removed; the three preexisting containers were
  not touched. No test failure is counted as HTTPS acceptance.

### Bounded HTTPS repairs (overall gate still failed)

Repair 1 (`39f398d`) moved the Origin probe before closure, used the genuine
browser-context cookie jar, and counted actual edge Origin/status plus upstream
calls. The Stop phase passed. Revoke's positive control returned 409: both
harnesses use package-singleton `fixture(t)`, hence the same tenant/store, and
the reused command key collided with the other attempt's request digest.
This was a correct product conflict, not a reason to relax the expected 200.

Repair 2 (`9cb5283`) scopes probe keys by phase and verifies their persisted
attempt IDs. Both Node browser phases pass (Stop 2.082s; revoke 0.483s), including
exact Origin 200/403 and post-close 409. The enclosing Go test still exits 1
(4.20s; foundation 5.192s): its final revoke-state query requires a row in
`media_execution_state`, which is absent before any worker claim/Stop. The
registrar revoke function itself only writes the revocation record. Independent
adjudication of this oracle is pending; there is no third unreviewed repair/run.

|Run|Log under `/Volumes/data/output/`|SHA256|
|---|---|---|
|Repair 1, exit 1|`brw05-https-browser-root-repair1-20260928.log`|`d1ac1f40b175efbcc494242cba9926be7a366d21d10130cb16ad7b0813e80fd1`|
|Repair 2, exit 1|`brw05-https-browser-root-repair2-20260928.log`|`581d18e60d518c81f8546fce4cde3985ebcacf09ece11203f5368edcb87bbeaa`|

Final browser evidence: `output/playwright/input-delivery-20260927T164756.521055000/`.
Passing browser subchecks do not override the failed enclosing gate.

### Escalated adjudication and accepted scoped follow-up

Independent review confirmed the prewire revoke execution row is absent by
design: planning/reservation creates custody, registrar revoke writes only a
revocation, and claim/Stop creates the execution projection. After the two-repair
stop, root explicitly authorized one exact-oracle correction, independently
reviewed before running. `6bc61d5` changes only the final test query; it requires
the exact scoped revocation, **no** execution row, issued RESERVED/open-admission
liability, and the original READY/generation-zero operation and available,
unfinalized native River job. It does not fabricate a projection, mark input
closed, relax any browser assertion or change product code.

`--browser-input-delivery` then exited **0**: Go test 4.07s, foundation 5.837s;
the two genuinely signed HTTPS browser phases pass (Stop 2.010s, revoke 0.466s).
Packaged Next build and three Node boundary tests also pass. Edge counters prove
two real hostile-Origin requests receive 403 and neither reaches Go; positive
controls receive 200. Fixed grants/receipts, post-close 409, and retained original
responsibility all pass. Earlier failed evidence remains above.

- Log: `/Volumes/data/output/brw05-https-browser-root-adjudicated-20260928.log`.
- SHA256: `404f87d19943269928098c783c0307b80d5ef4a054d046b14a522437c06ce924`.
- Browser evidence: `output/playwright/input-delivery-20260927T165827.706359000/`.
- Scope: signed browser/HTTPS BFF/Go/actual PG token transport and WSS handshake,
  not decoded SFU media, merchant input controls, INPUT recovery or production.

## Merchant input read integration (2026-09-28)

Frozen source `32d74c0` adds exact GET `/input` and `/input/prepared` under the
existing merchant session route. Migration 0043 grants only runtime EXECUTE;
the prepared SQL envelope carries fixed deployment pins for Go verification,
which are stripped from the public five-field DTO. Input status has seven
nonsecret fields. Both reads use current authority, HTTPS BFF, private no-store,
bounded response parsing and no fixture authentication. The old Studio detail
accepts marked input attempts but never offers input authorizations as legacy
rehearsal candidates. Input `can_stop` preserves joint input/Egress liability.

Independent review found a strict-decoding P1: Go zero values accepted null
booleans and an open/closed contradiction. Repairs `39d46cd` and `b5a960f` add
raw boolean/string checks, state/admission consistency and null/contradiction
negative tests; valid CLOSED input with Egress cleanup still permits Stop.
Independent reviewer approved the final source, not runtime acceptance.

Root results on `32d74c0`:

- Six Node boundary tests, Admin typecheck and packaged production build pass.
- `go test -race ./internal/live ./internal/httpapi` and matching `go vet` exit
  0 (1.411s/1.744s for the packages).
- First `--studio-backend` exits 1 (foundation 17.237s): the new test reused a
  POST-oriented helper that supplied a non-nil empty body for GET, correctly
  rejected with 422. It also applied the registered Studio private cache rule
  to an intentionally unregistered nil-runtime 404. The direct SQL freshness
  and existing Studio tests pass. Correction belongs in the test fixture;
  production body checks must not be relaxed. Focused PG correction is below.
- Actual `--browser-input-delivery` exits 0: Go 5.37s, foundation 7.113s;
  two signed HTTPS browser phases pass (2.854s/0.883s). Real IDs traverse
  browser → Next → Go → PG for nullable prepared selection, RESERVED status,
  consumed candidate, legacy detail forwarding and Stop/revoke liability.
  Wrong query/key/method/store fail; synthetic upstream IDs separately test
  malformed/oversized BFF sanitation. Existing token/replay/Origin gates pass.

|Run|Log under `/Volumes/data/output/`|SHA256|
|---|---|---|
|PG first, exit 1|`studio-input-read-pg-root-first-20260928.log`|`10c0a3344f0aefb8c02212538baeeaad8c0cee142d8803bcd21eb694127b51fc`|
|HTTPS, exit 0|`studio-input-read-https-root-first-20260928.log`|`299763fdce68d128adb79d1cfc181fda610d1b5e5de65478f52548a23ee68aed`|

Browser evidence: `output/playwright/input-delivery-20260927T173109.531042000/`.
Owned PG/Next fixtures are removed after each runner. No customer, production
runtime, provider credentials or external streams were changed. These are
read/transport gates, **not** BRW06 decoded SFU, BRI07 visible input controls,
BRW07 INPUT recovery, current-source full regression or deployability acceptance.

### Scoped GET fixture correction and accepted PG read gate

`a74be82` fixes only the new independent test helper: an empty GET uses a nil
body; the nonempty `{}` GET still must receive 422. The nil-runtime 404 retains
the global `no-store` assertion in `4b45135`, while registered routes must
return `private, no-store`. No production guard or shared old test helper changed.

On frozen `4b45135`, `bash scripts/dev/test-local.sh --studio-backend` exits **0**:
**12 top-level PASS, zero FAIL/SKIP**, foundation 18.926s. This includes the four
new actual-PG input tests and existing Studio regression. Covered: runtime-only
function ACL/no raw table access, private mapping stripping, current candidate
eligibility, kernel-only and legacy separation, independent input/Egress Stop
responsibility, held/revoked input liability, HTTP negatives and real-clock
session expiry. The source under test is unchanged from the HTTPS pass above.

- Log: `/Volumes/data/output/studio-input-read-pg-root-repair1-20260928.log`.
- SHA256: `da2bdc19a9e0e841a524ac600dc0e3933160b4678b4fea566bd1da126c729be1`.
- Fixture cleanup confirmed: only protected `lc-meta-upgrade-9d14f59e966f`,
  `humaux-thread-qdrant`, `humaux-thread-pg` remain. Their state was not changed.
- This closes the **read projection** slice, not camera capture/SFU decoded
  media, input recovery, current full-suite or production rollout gates.
