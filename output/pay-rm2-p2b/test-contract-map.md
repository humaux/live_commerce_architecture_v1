# PAY-RM2 phase 2b: test contract map

Purpose: one row per test that drove SQL dropped by `migrations/0197_payuni_phase2b_drop.sql` and
`migrations/post_river/0026_payuni_phase2b_drop.sql`, with its replacement (coordinator ruling 2026-10-19).
Depends on: `tests/foundation/*_test.go` on branch `unit/pay-rm2-p2b`, `output/pay-rm2-p2a/round3-contract-classification.json`.
Used by: PR #85 review, the owner, and the independent reviewer.
Invariants: no assertion is removed without a replacement. Mapping codes:
**a** = a Stripe mock/sandbox test that already asserts the same invariant,
**b** = a refusal/absence assertion,
**c** = historical rows seeded directly (same columns the dropped writer stored), with the original assertions kept.
Evidence level: E1 (static: `go vet`, `go test -c`, `gofmt`, prepush). Every row is NOT_RUN until CI reports.

## Shared replacement helpers (`tests/foundation/payment_sql_contract_helpers_test.go`)

| helper | what it asserts (b) | seeded history (c) |
|---|---|---|
| `p2bRetired` | all 10 `p2bDropped` signatures are absent; the old test-side caller fails with exactly the expected SQLSTATE; `rm2ScopeSnapshot` (tenant-scoped business tables plus hosted pages and Stripe sessions) is unchanged | none |
| `p2bStartRetired` | `checkout.start_payment` caller → 42883; no attempt, operation, receipt, event or job | none |
| `p2bHostedRetired` | begin → 42883, take → 42883, both with no row change; the retained `hosted_payment_view` still projects the seeded page as PENDING/PREPARED | attempt (`rm2HistoricalAttempt`) and prepared page (`p2bSeedHostedPage`) |
| `p2bQueryRetired` | `record_payment_query` caller → 42883 after a real claim; `q.pending` (order AWAITING_PAYMENT, reservation PAYMENT_PENDING, stock 2/0/10) | attempt (`pqSetup` → `rm2HistoryIn`) |
| `p2bCaptureRetired` | everything in p2bQueryRetired, then `payments.apply_capture` → PT409 with no row change, and `q.pending` still holds | full-capture report and reconcile job (`rm2SeedReport`) |

## Per-test map

### payment_start_test.go: all → `p2bStartRetired` (a+b)

| test | a: Stripe test holding the invariant |
|---|---|
| TestBuyerPaymentAtomicReplayAndPendingStock | TestStripeSP07Start/atomic_rows_and_zero_provider_calls, /replay_and_frozen_input_conflicts |
| TestBuyerPaymentConcurrentKeys | TestStripeConcurrentKeys; TestStripeSP07Start/concurrent_payuni_vs_stripe_one_winner |
| TestBuyerPaymentAdmissionDenials | TestStripeSP07Start (admission clauses of contract §14); TestStripeSP19LiveRefusal |
| TestBuyerPaymentFaultRollback | TestStripeStartFaultRollback |
| TestBuyerPaymentFinalWaitGate | TestStripeSP07Start/mixed_provider_lock_workload_no_deadlock |
| TestBuyerPaymentAuthorityAndDispatcherFence | TestStripeAuthorityIngressMatrix / RegistrarMatrix; P2B01 (start_payment absent, ACL gone with it) |
| TestBuyerPaymentQualificationExpiresDuringFinalWrite | TestStripeSP07Start (qualification clauses); TestStripeSL03Registrar/qualify_and_method_gates |
| TestBuyerPaymentFreshSessionAndQualificationAuthority | TestStripeSP07Start; TestStripeSP21RegistrarBindsStoredCredential |
| TestBuyerPaymentExpiredHoldRace | TestStripeSP10Deadline |

### hosted_payment_test.go, hosted_payment_authority_test.go, hosted_payment_commit_test.go: all → `p2bHostedRetired` (a+b+c)

| test | a: Stripe test holding the invariant |
|---|---|
| TestBuyerPaymentHostedReplayConflictsAndSingleAttempt | TestStripeSP07Start/replay_and_frozen_input_conflicts |
| TestBuyerPaymentHostedConcurrentTakeOneForm | TestStripeHTTPThreeLocalesAndConcurrentHandoff |
| TestBuyerPaymentHostedAdmissionDriftBeforePrepare | TestStripeSP07Start; TestStripeSP14BuyerHTTP/stripe_view_prepare_handoff_enums_and_keys |
| TestBuyerPaymentHostedSQLAtomicPreparationAndOneShot | TestStripeSP07Start/atomic_rows_and_zero_provider_calls; TestStripeSP21PinnedHandoffAfterKeyRotation |
| TestBuyerPaymentHostedSQLAuthorityAndScope | TestStripeAuthorityIngressMatrix; TestStripeSP14BuyerHTTP |
| TestBuyerPaymentHostedForgedOuterFormRejected | TestStripeSP14BuyerHTTP (handoff is a pinned Stripe URL, no buyer-supplied form exists) |
| TestBuyerPaymentHostedStorageFailureRollsBack | TestStripeStartFaultRollback |
| TestBuyerPaymentHostedDriftAndFinancialReviewDenyFirstTake | TestStripeSP14BuyerHTTP; TestStripeUntrackedDriftReview |
| TestBuyerPaymentHostedReplayAfterDisableAndRevokedCapability | TestStripeSP14BuyerHTTP/payuni_refused_history_unchanged_*; TestStripeSL06LiveProcess |
| TestBuyerPaymentHostedQualificationExpiresDuringTakeWait | TestStripeViewFinalDatabaseClock |
| TestBuyerPaymentHostedWriterCannotMutateFormOrClearHandoff | TestStripeSP06Schema (Stripe session immutability); the b-assertion here: the writer functions are gone |
| TestBuyerPaymentHostedQualificationExpiresDuringFinalWrites | TestStripeViewFinalDatabaseClock; TestStripeSP07Start |
| TestBuyerPaymentHostedCapabilityRevokedDuringTakeWait | TestStripeViewFinalDatabaseClock |
| TestBuyerPaymentHostedLocalDeadlineFence | TestStripeSP10Deadline |
| TestBuyerPaymentHostedLostCommitAcknowledgementDoesNotReissue | TestStripeSP21PinnedHandoffAfterKeyRotation (the pinned handoff is re-read, never re-issued) |

### payment_query_test.go

| test | map | replacement |
|---|---|---|
| TestBuyerPaymentQueryReportAtomicDedupAndBinding | a+b | `p2bQueryRetired`; a: TestStripeSP11SQLSignalLoader, TestStripeCaptureDurableObservationAcrossRestart |
| TestBuyerPaymentQueryFaultAndLeaseWaitRollback | a+b | `p2bQueryRetired`; a: TestStripeQueryCancelledIOKeepsRecoverableLease |
| TestBuyerPaymentQueryProviderReferenceCannotFundTwoAttempts | a+b | `p2bQueryRetired`; a: TestStripeSP12MoneyChecks |
| TestBuyerPaymentQueryFinishClockAndReferenceFence | a+b | `p2bQueryRetired`; a: TestStripeSP10Deadline, TestStripeSP12MoneyChecks |
| TestBuyerPaymentQueryCrossAttemptTokenAndMerchantJob | a+b | `p2bQueryRetired`; a: TestStripeSP11SQLSignalLoader, TestStripeSP08HappyMock (no key or scope leakage) |
| TestBuyerPaymentQueryConcurrentReportSingleWinner | a+b | `p2bQueryRetired`; a: TestStripeSP13WebhookHTTP/accept_atomically_and_dedupe_changed_body |
| TestBuyerPaymentQueryHistoricalCredentialAndACL | b (partial) | the worker-only EXECUTE check on load/require/record is replaced by an absence check of the three functions; the ciphertext and observation-write ACL assertions and `q.pending` are unchanged |

### payment_capture_test.go

| test | map | replacement |
|---|---|---|
| TestBuyerPaymentCaptureAtomicReplayAndFreshSession | a+b+c | `p2bCaptureRetired`; a: P2B03 "stripe capture and replay settle", TestStripeSP08HappyMock |
| TestBuyerPaymentCaptureEvidenceAndStickyReview | a+b+c | `p2bCaptureRetired`; a: TestStripeSP12MoneyChecks, TestStripeUntrackedDriftReview |
| TestBuyerPaymentCaptureLaterRefundAndReverseOrder | a+b+c | `p2bCaptureRetired`; a: TestStripeRF07Lifecycle, TestStripeRF05HappyMock |
| TestBuyerPaymentCaptureOldAuthorizationIsNotConflict | a+b+c | `p2bCaptureRetired`; a: TestStripeSP12MoneyChecks |
| TestBuyerPaymentCaptureACLAndObservationBinding | a+b+c | `p2bCaptureRetired`; a: TestStripeSP06Schema, TestStripeSP11SQLSignalLoader |
| TestBuyerPaymentCaptureFaultsAreCausalAndAtomic | a+b+c | `p2bCaptureRetired`; a: TestStripeCaptureDurableObservationAcrossRestart |
| TestBuyerPaymentCaptureReleasedAnomalyKeepsRealMoney | a+b+c | `p2bCaptureRetired`; a: TestStripeSP10Deadline/paid_in_the_last_second_captures_not_closes |
| TestBuyerPaymentCaptureBalanceWaitIsAtomic | a+b+c | `p2bCaptureRetired`; a: TestStripeUntrackedReservationRefusals, TestStripeUntrackedQuantityBoundary |
| TestBuyerPaymentCaptureEmptyReferenceAndMerchantReadScope | a+b+c | `p2bCaptureRetired`; a: TestStripeSP12MoneyChecks; merchant read scope of PAYUNi history: P2B03 (`moRead`) |
| TestBuyerPaymentCaptureMultiSKUSingleCommit | a+b+c | `p2bCaptureRetired`; a: TestStripeUntrackedMixedDrift, TestStripeSP08HappyMock |
| TestBuyerPaymentCaptureQueryJobBindingAndRollback | a+b+c | `p2bCaptureRetired`; a: TestStripeSP11SQLSignalLoader, TestStripeCaptureForgedRiverJobCancelsWithoutMoney |
| TestBuyerPaymentCaptureRealRiverSignedQueryChain | c | `pcRecord` → `rm2SeedReport`; every assertion unchanged |

### Read-path tests: c, all assertions kept

| test | change |
|---|---|
| TestBuyerPaymentViewHandoffStatesAndProfileSwitch | `h.begin` → `p2bSeedHostedPage`; `h.take` → owner sets `handed_out_at` (what take_hosted_page wrote). The profile-switch Take/Begin denials still assert an error (now 42883). Subtest expired-unissued: seeded page |
| TestBuyerPaymentViewSignedFinancialPrecedence | `pqSQLSetup` → `pqSetup`; AUTHORIZED/CAPTURED → `rm2ArchivedFact`; REVIEW_REQUIRED → `rm2SeedReport` plus a REFUND_HISTORY review case |
| TestBuyerPaymentViewFinalDatabaseClock (page stage) | `h.begin` → `p2bSeedHostedPage` |

### Runtime and recovery suites: c (the stored report is seeded instead of written by record_payment_query)

| test | change |
|---|---|
| rm2AssertRemovedWorker (TestBuyerPaymentCaptureForgedRiverJobCancelsWithoutMoney, TestBuyerPaymentWorkerCrashChild, TestBuyerPaymentWorkerLateDefaultInsertPollAndProfileIsolation, TestBuyerPaymentWorkerRealCrashAndRiverLeaseRescue) | `pcRecord` → `rm2SeedReport` |
| TestLegacyRuntimeIsolationAdmissionAndReadiness | SQL start → `rm2HistoryIn` (river_payment, payment_mock_v1); `pcRecord` → `rm2SeedReport` |
| TestLegacyRuntimeIsolationExpiryDoesNotMaintainForeignFamilies | `pcRecord` → `rm2SeedReport` |
| TestLocalRecoveryLogicalRestoreAndColdStart | `q.record` → `rm2SeedReport` |
| TestBuyerPaymentWorkerDeferredRoutingRollback | `q.record` → `rm2SeedReport` |

### Legacy upgrade gates (`legacy_runtime_upgrade_test.go`, `lriApply`)

These fixtures are migrated to the pre-0032 schema, filled by the historical producers (non-terminal PAYUNi operations,
orders AWAITING_PAYMENT), and then upgraded. 0197's guard rightly refuses to drop over that history. Without a change,
the tests that expect `lriApply` to fail (tampered ledger, running old job, and so on) would pass for the wrong reason.
`lriApply` therefore pre-records `0197` and `post_river/0026` with their real checksums, so these gates certify the
upgrade up to the phase-2a head. The drop and its refusal over exactly this history are certified by P2B01, P2B02 and
P2B02b. The old-schema `pcRecord` and `pqRecordIn` calls in these files still run against the historical
`record_payment_query`, which exists in that schema. TestLegacyRuntimeIsolationFamilyCollidingIDs keeps calling
`record_payment_query` after `lriApply`; it therefore certifies the phase-2a family-ID fence. The Stripe equivalent
is TestStripeSP11SQLSignalLoader.

### Phase-2b gates (`payuni_phase2b_test.go`)

| test | role |
|---|---|
| TestPayuniPhase2bSchemaFreshAndUpgrade (P2B01) | RED a95e0b0a2 → GREEN 92721601f |
| TestPayuniPhase2bRefusesUnreconciledPayuni (P2B02) | RED a95e0b0a2 → GREEN 92721601f |
| TestPayuniPhase2bRefusesPayuniOrderAwaitingPayment (P2B02b, new) | guard clause 2 (order AWAITING_PAYMENT, every operation terminal). Added after GREEN, so there is no separate RED run. On a95e0b0a2 it would fail at the missing-migration check |
| TestPayuniPhase2bDispatchStripeCapturesPayuniRefused (P2B03) | RED a95e0b0a2 → GREEN 92721601f |

## Not changed and why

- `home_cod_review_test.go` and `taiwan_cvs_pay_at_pickup_test.go`: they mention `checkout.start_payment` only in
  messages and use the production Go starter, which calls no dropped SQL.
- `payment_queue_migration_test.go` and `legacy_runtime_upgrade_test.go` old-schema calls run on the pre-0032 schema,
  where the functions exist.
- `contracts/payment-*-v1.md`, `contracts/payuni-wire-v1.md`, `contracts/legacy-runtime-isolation-v1.md` and
  `contracts/stripe-psp-v1.md` still describe the dropped functions. They are out of this unit's write scope; a
  follow-up docs unit should update them (UNKNOWN for the owner).
