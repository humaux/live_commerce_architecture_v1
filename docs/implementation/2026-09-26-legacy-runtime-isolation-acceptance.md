# Legacy runtime maintenance isolation evidence

Status: **ROOT_CAUSE_REPRODUCED / CONTRACT_FROZEN / CANDIDATE_GATES_PENDING**.
This is a LOCAL unit under T11/T06, not a production or whole-SaaS acceptance.
Contract: [legacy runtime isolation v1](../../contracts/legacy-runtime-isolation-v1.md).

## Causal RED

Product base `c45e8a7`; independent test-only commit
`d6448295da9ea9bb06988d1961f81d172fa304b6`. Test author `meta_consumer_pg`,
role `test_worker`, actual model `gpt-6-sol/high`, worktree
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch
`commerce/legacy-maintenance-tests-20260926`. Write paths were only the new
foundation test and its author evidence note; no product edits.

| Run | Result | Log and SHA256 |
| --- | --- | --- |
| Author | Actual PG18/race, exit 1, foundation 8.682s | `/Volumes/data/output/legacy-runtime-isolation-first-20260926.log`; `86d047fd6f303c9128e47b1dcca0e28f0ec86f03d9c7d3e6572c6eaafb03f430` |
| Root independent | Actual PG18/race, exit 1, foundation 9.027s | `/Volumes/data/output/legacy-runtime-isolation-root-real-red-20260926.log`; `8fc59ec77cb532732e33729d69bc4886c25b71e1ae6a5fab297501db164b8176` |

Both used `GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run
'^TestLegacyRuntimeIsolationExpiryDoesNotMaintainForeignFamilies$' -v
./tests/foundation` with explicitly admitted, task-owned loopback fixtures.
The actual expiry CLI alone discarded linked payment-query and external-operation
stale-running jobs and cleaned a linked terminal payment-reconcile job. Its own
scheduled/retryable/stale/terminal positive controls passed. All compared business
receipts, orders, reservations, observations and facts remained byte-equal.
CLI logs were preserved by the test, including root
`/Volumes/data/output/meta-runtime-process-legacy-expiry-maintenance-1cd697e434f2.log`.
These are real failures, not successful product tests.

Two earlier root launcher logs (`legacy-runtime-isolation-root-red-20260926.log`
and `legacy-runtime-isolation-root-red2-20260926.log`) are **NOT_RUN/SKIP**:
macOS Bash `source <(...)` did not import the fixture environment. A separate
safe probe reproduced the missing shell variable. The corrected launcher used
the inspected fixture-setup segment with explicit required-environment checks;
it did not weaken any test or product fence. Keep all three results distinct.
All task-labelled containers were absent after root completion.

## Independent decision review

Read-only explorer `meta_consumer_go` (`gpt-6-sol/high`) confirmed same payment
kinds/default maintenance policy and the per-profile independent sequence/UNIQUE
cost. Security reviewer `meta_consumer_preflight` (`gpt-6-astra/high`) independently
verified source and author RED hash, then reviewed contract draft hash
`86652a6dd379cbcd4570cc2d9257b92051fb2e94617f33c0f4cbd09b371d8cf7`.
Round one: no concrete P0/P1/P2; bounded freeze approval only. Keep ordinary
`commerce_worker` and state the semantic, not DB-principal, boundary explicitly.

## Implementation candidate, not accepted

Root integration branch `commerce/legacy-family-integration-20260926` contains
SQL `3a03f53`, Go author `69f64d4` integrated as `57802ee`, and focused test selector
`732b10e`. Main remains contract `9772380`; the product candidate is not merged.
Independent source review found no concrete P0/P1/P2 at `57802ee`, including
byte comparisons of the three schema-only static business function copies.
This is source review, not the LRI runtime acceptance.

Root real PG/race `TestMigrationIsIdempotentAndRuntimeRoleIsOrdinary` passed,
actual exit 0, foundation 2.241s, one test and no skips. Log
`/Volumes/data/output/legacy-isolation-root-migration-smoke1-20260926.log`, SHA256
`142501ff17e2e9b70a301995642960b032253b6f01d0894379449da08e6cbe0b`.
Fresh install and repeated Apply are proven by this small check; populated
cutover, full regression and browser acceptance are not yet proven.

## LRI01 causal GREEN, independently accepted LOCAL only

Original RED source `d644829` and both failing logs above remain preserved.
Independent test checkpoint `7d3fdf0`, integrated by root as `e559b10`, rebinds
the exact linked foreign jobs to their family tables. It retains the same
expiry CLI, maintenance parameters, timing bounds and positive controls.

| Run | Result | Log and SHA256 |
| --- | --- | --- |
| Author | Actual PG18/race, exit 0, 1 PASS / 0 FAIL / 0 SKIP, foundation 7.249s | `/Volumes/data/output/legacy-isolation-author-causal-candidate-20260926.log`; `c397bb20670dc0f89bec94fdd4553c8863d443c41d6fb3742617175f6f8f57ee` |
| Root independent at `e559b10` | `bash scripts/dev/test-local.sh --legacy-isolation`, actual exit 0, 1 PASS / 0 FAIL / 0 SKIP, foundation 9.871s | `/Volumes/data/output/legacy-isolation-root-causal-green1-20260926.log`; `0ab273b1debe888f6cd1fd85c2090600ebf99e4ac37a950427f05ee2aaa7c0cf` |

Read-only reviewer `meta_consumer_preflight` independently compared RED and GREEN
source and root evidence. Three foreign rows are nonempty in their correct family
tables before whole-row comparison; eight business-table snapshots stay unchanged.
All four expiry maintenance controls advance and the real CLI exits normally.
No concrete P0/P1/P2 in this bounded LRI01 review. This does not accept LRI02–06
or establish database-principal containment.

## LRI02 reciprocal maintenance sub-gate

Independent checkpoint `9fe4dd3`, root `72bd176`, adds the converse payment-leader
case. It starts the real payment client with default maintenance options and a
paused native queue: four own maintenance controls advance, while expiry,
external and Meta job tables plus eight business tables remain whole-row equal.
The injected MOCK transport records zero calls. Linked IDs are family-qualified.

| Run | Result | Log and SHA256 |
| --- | --- | --- |
| Author | Actual PG18/race, exit 0, 2 PASS / 0 FAIL / 0 SKIP, foundation 14.112s | `/Volumes/data/output/legacy-isolation-author-mutual-candidate2-20260926.log`; `80fc94653c7ec7c9f9237dc7230fe3b030ecdc6e840c656a18896741d62fd5e5` |
| Root independent at `72bd176` | Same focused selector, actual exit 0, 2 PASS / 0 FAIL / 0 SKIP, foundation 22.253s | `/Volumes/data/output/legacy-isolation-root-mutual-green1-20260926.log`; `4e4eb63acf773222f27ed989c841e233ad4a53c59b615768d0b7db18872a73ff` |

Read-only reviewer `meta_consumer_preflight` independently accepted only this
reciprocal maintenance sub-gate, with no concrete P0/P1/P2. Three-profile
cross-fetch and existing converse/restart/shutdown gates still need regression.
The author's earlier `53300` fixture failure exhausted setup connections; only
unused seed pools were closed before retry. No product limits or assertions were
weakened, and the failed log remains retained.

## LRI06 real buyer browser regression

Root `237b880` integrates test checkpoint `3757320`: current browser observations
read payment and expiry facts from their correct family schemas, while no-write
checks include both new tables and the retained external table. Production code
is unchanged from the reviewed candidate. Each command rebuilt the storefront.

| Gate | Root result | Log and SHA256 |
| --- | --- | --- |
| `bash scripts/dev/test-local.sh --browser-order` | exit 0, 23 cases, test 16.47s / package 18.625s | `/Volumes/data/output/legacy-isolation-root-browser-order1-20260926.log`; `148ddac34642f3be952d758333324fe5b45aefbc8a498459db5c60b366a6635f` |
| `bash scripts/dev/test-local.sh --browser-payment` | exit 0, 11 cases, test 8.03s / package 9.437s | `/Volumes/data/output/legacy-isolation-root-browser-payment1-20260926.log`; `39d6d30ecb3b69003064249aeae8b7a94543849a5a5d6e8efe351768e95d7637` |

These use actual Next → Go → isolated PG18 → Chromium, with a local MOCK PSP
only. The payment gate proves two native form posts against persisted digests;
it does not contact a real payment provider or establish settlement.
Run-specific logs/results/screenshots are retained under
`output/playwright/buyer-order-1349535540/` and
`output/playwright/buyer-payment-4074165545/`.

Root inspected desktop/mobile orders and addresses, mobile history, desktop
payment-ready, touch-mobile Traditional Chinese history read-only, and neutral
mobile return. No clipping/overflow was observed in these views; test mode,
currency, uncertain-outcome and return-page warnings remain visible. The
order-only fixture intentionally lacks payment setup; the separate payment
fixture proves ready and post-handoff read-only states. No design change.
Seven overwritten tracked review PNGs were first byte-compared to retained
run-specific copies, then restored to their previous committed baselines.
This is responsive/Chromium mobile emulation, not physical iOS/Safari acceptance.

## LRI03 / LRI04 integration review, not yet accepted

Root `88bd408` admission run exited 0: 3 PASS / 0 FAIL / 0 SKIP,
foundation 17.456s, including the two maintenance cases. Log
`/Volumes/data/output/legacy-isolation-root-admission1-20260926.log`, SHA256
`3fff052699d0a7c065dcb827809b2c64742f8788bde6283110e20f3410efd20b`.
The independent review still requires both destination job-table snapshots
around wrong-schema failures; expected-lane counts alone are insufficient.

Author upgrade checkpoint `a7f4520` passed seven cases on its older helper base.
Root integration `1711ce6` did **not** pass: 4 PASS / 4 FAIL / 0 SKIP,
exit 1, foundation 24.322s. Log
`/Volumes/data/output/legacy-isolation-root-upgrade1-20260926.log`, SHA256
`f25fc8b94345ac114d5118abae8b6cb584b908ed24d083a3d97cad8dba6a4e05`.
The historical fixtures omitted the explicit `river` argument after the shared
helper default became `river_payment`. The author result cannot transfer across
that helper change. A test-only repair must retain current-family defaults.

Read-only reviewer also requires discriminating same-ID reconciliation success,
explicit retained `available` state, independently dominant sequence maxima for
both families, and consistently admitted historical profile/environment/account
data. Merely changing a SANDBOX fixture's profile label to LIVE proves routing
label preservation, not valid historical LIVE admission. These are test-proof
gaps, not established product defects. No real-provider calls are permitted.

## LRI05 first broad diagnostic, failed

Root `9e67eaa`, `bash scripts/dev/test-local.sh`, exited 1 with 541 top-level
PASS / 13 FAIL / 0 SKIP; foundation 413.584s. Vet was not reached. Log
`/Volumes/data/output/legacy-isolation-root-full-candidate1-20260926.log`, SHA256
`5ea6f6c0f885ecbb6e18fef1389f57d7c2f8cfe8aa95aa64600ae200f63006aa`.
This diagnostic deliberately preceded the pending Meta fixture integration and
is not full acceptance. Remaining failures exposed old custom-queue fixtures,
two missing private-trigger entries in an exact ACL allowlist, the new ledger
count, and an external-dispatcher assertion reading the payment family table.
Repairs preserve production guards, original negative assertions, and the
external dispatcher's actual `river` route; forged-row fixtures remain narrowly
owner-only and restore admission guards before runtime validation.

The author's first Meta grouped run separately hit an eight-second process
readiness failure with zero-byte child logs. An exact isolated rerun passed;
the cause remains unproven. It is not the root diagnostic's earlier wrong-table
failure and does not justify relaxing the startup deadline. Keep both records.
Candidate repair commits `b899650` and `667376e` are integrated as `491128c` and
`77cd52f`. Root `--meta-runtime` exited 0 with 11 PASS / 0 FAIL / 0 SKIP,
foundation 57.494s. Log
`/Volumes/data/output/legacy-isolation-root-meta-adapt2-20260926.log`, SHA256
`7fe9cac9eab92d2e29a63d7546bf66b710056c929ec1a1a7aacb39e910513a80`.
Root `--payment` exited 1 with 68 PASS / 1 FAIL / 0 SKIP, foundation 149.082s.
Log `/Volumes/data/output/legacy-isolation-root-payment-adapt2-20260926.log`, SHA256
`f176077b72e75e7564d2e65ea6361b81029aa2695d05435d5c3870524b938712`.
The remaining CrossAttempt failure agrees with the review's shared-fixture
isolation finding. Review also identified competing query-only/capture-only
test clients on the same fixed queue, even though capture passed this run.
Both require test repairs before acceptance; green timing is not a race proof.

## Pending

- Remaining profile/runtime regression, admission/readiness and populated-upgrade
  portions of LRI02–04.
- Root independent full PG18/race/vet regression (browser gates above are local).
- Independent exact-source final signoff before merging the product increment.
- Production backup/impact/approval, real-provider qualification and full SaaS
  acceptance are outside this unit and remain unproven.
