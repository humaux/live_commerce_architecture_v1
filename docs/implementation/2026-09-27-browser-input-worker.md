# Browser input worker — scoped validation, 2026-09-27

Status: **SQL_EXECUTOR_SUBSET11_PASS; SOURCE_FULL709_PASS; PRODUCT_GATES_PENDING**.
Source is fixed at `c5160de`. This is not complete BRW/BRI, T08, G06 or SaaS
acceptance. HTTP token delivery and production command wiring remain disabled.

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
include the two later test-only additions at `361e3ef`.

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

Earlier failing logs (`brw-runtime-root-first`, `brw-runtime-root-repair1`,
`brw-runtime-root-repair2`, all suffixed `-20260927.log`) remain available.
No failing assertion or evidence was deleted to obtain a pass.

## Stop lines and next gates

The eleven top-level tests cover a **subset** of BRW01–04: real-role isolation,
marker/replay mapping, dual-track final admission, exactly one Start, eight
bounded cleanup slots, pending-reservation loss and original-job continuation.
They are not blanket acceptance of all concurrency, process-crash or ACK-loss
permutations in those gates. The later `361e3ef` test-only cohort adds actual
final-reservation COMMIT-ACK loss and concurrent Stop/revoke cases. Its first
run passed ACK loss but timed out in the revoke case. The failure is preserved;
the corrected final `3b7fb56` cohort passes, including exact causal denials and
committed-ACK-loss recovery on the original job without another Start. These
later tests are not silently included in the historical full709 result.
Plan transaction faults, per-cleanup-step ACK/process loss and other missing
BRW01–04 permutations remain separate work. BRW05 HTTPS BFF/post-commit token delivery, BRW06
actual product browser/SFU decoded A/V, BRW07 new-input-queue 90-second recovery,
and BRW08 final integrated regression/Studio evidence remain pending. The
source-level full709 regression passed, but that alone does not close BRW08.
Existing MRR
90-second proof does not automatically cover this queue. The old LMR05 wait is
the separately owner-approved 90 seconds; its safety conditions were not changed.

All external calls above use disposable local TLS fixtures. No customer
stream, payment, provider credential, Cloud or production configuration changed.
