# k3-w4-01b — K3 adversarial acceptance of W4-01B PAYUNi NotifyURL receiver

- Reviewer: Kimi K3 (independent; NOT the author — author was DeepSeek V4-Pro, finisher Sonnet).
- Base SHA: `56d7e899` (unit/k3-w4-01b = W4-01B `8ab3a6a9` + r3/integration `df6be6ce`).
- Scope: `internal/payments/payuninotify/`, `internal/integrations/psp/payuni` (NewNotify/AuthenticateNotification),
  `migrations/0136_payuni_notify.sql`, `migrations/migrate.go` (UPDATE(scheduled_at) grant), `cmd/api/payuni_notify.go`.
- New tests only: `tests/foundation/k3_w4_01b_adversarial_test.go` (10 × `TestK3W401B*`, REAL_PG + MOCK signatures).
  No product code changed.

## Verdict

**No P0, no P1 findings.** Every attack class in the unit brief was repelled by the delivered code;
all 10 adversarial tests pass against the real 0136 schema and the real `commerce_payuni_ingress` role.

## Tests → results (REAL_PG, MOCK signed bodies)

Command: `bash scripts/dev/test-focused.sh '^TestK3W401B'` → **exit 0, PASS=10 FAIL=0** (`output/k3-w4-01b/green.log`;
iteration logs `run1.log`/`run2.log` — the two early failures were bugs in my own fixture code, fixed in the test file;
`run3.log` died to a transient machine-wide `fork: Resource temporarily unavailable` while queued on the PG lock).

| Test | Attack | Result |
|---|---|---|
| ForgedBodiesLeaveNoRows | attacker-key signature; re-signed genuine EncryptInfo with attacker HashInfo; outer MerID rebound; inner MerID rebound (validly signed); outer/inner Status pair broken; 26-char MerTradeNo; unsigned form; duplicate outer field; Version downgrade 1.0; garbage | all 400, 0 receipts, 0 review cases, job not woken, no facts |
| ConcurrentReplayExactlyOneReceipt | 8 goroutines, identical signed body | 8×200, exactly 1 receipt, redelivery_count=7, job count constant, job woken, no facts |
| RotationGraceBoundary | rotate v2→v3→v4; body signed with two-behind v2 key | 400, zero rows; v3 (grace) and v4 (current) both 200 QUEUED |
| CrossStoreSameCredentialReplayScoped | store B re-registered with the SAME provider account and SAME HashKey; A's genuine signed delivery replayed at B's endpoint | 200, receipt UNKNOWN_TRADE on B's connection, A's job not woken, 0 receipts on A, 0 review cases |
| DisabledStoreStillRecordsAndWakes | `control.stores.active=false` then valid notify | 200 QUEUED + wake (in-flight money not lost) |
| EndpointDisableRotateAndLiveRefusal | disabled endpoint vs unknown token; token rotation; LIVE at definer; LIVE/""/"PRODUCTION" at NewInbox | disabled ≡ unknown (identical 404 body); old token dies, new token works; definer 22023; NewInbox ErrConfig |
| WakeOnlyMatchingJobNeverInserts | orphan payment_query_v1 decoy insert; second tenant's real query job on the same table; expiry-lane probe | orphan insert refused 22023 by guard (positive control); wake rewound only the matching job; other tenant's job untouched; river_expiry untouched; no INSERT |
| ProfileMismatchReviewedNotWoken | SANDBOX endpoint on same connection, notify for PROVIDER_MOCK attempt ×2 (distinct payloads) | 2 MISMATCH receipts, exactly 1 NOTIFY_MISMATCH review case (PK dedup), no wake, no facts |
| IngressAuthorityCeiling | ingress login: direct read/write receipts, endpoints, merchant_accounts, orders, tenants, review insert, river DELETE/INSERT, pg_authid, registrar definer | all 42501 |
| BodyAndHeaderBoundaries | 8192 B (admitted to verify → 400) / 8193 B (413); charset=UTF-8 accepted; doubled Content-Type 415; missing Content-Type 415 | all as expected, 0 rows |

Neighbor regression: author's gate `^TestPayuniNotify|^TestPayuniNotifyAuthority` untouched (no shared helpers changed);
`go vet ./tests/foundation` exit 0; `gofmt -l` clean; `bash scripts/dev/check-gates.sh` exit 0 (incl. check-headers).

## P2 observations (not defects; no test failure)

1. **QUEUED receipt can overstate the wake** — `migrations/0136_payuni_notify.sql:240-244`: if the attempt's job is
   `running`/`completed` when the notify lands, the wake UPDATE matches 0 rows yet the receipt still records
   QUEUED with job_id. Benign (the query already ran or is running; notify is trigger-only and never writes money
   facts), but the disposition name does not distinguish "woken" from "already in flight".
2. **redelivery_count cap** — `payuni_notify_receipts.redelivery_count CHECK(...BETWEEN 0 AND 100000)`: after 100 000
   redeliveries of one payload the bump violates the CHECK → 23514 → handler 503 → PAYUNi (if it retries) loops
   forever on that payload. Requires the merchant's own valid signature, so self-inflicted only; noting for the
   integrator's operations docs.
3. **ACK evidence gap** (already declared by the author): 200 empty body, PAYUNi retry semantics NOT_VERIFIED
   (`contracts/payuni-wire-v1.md` :19-21). Confirmed nothing in the code depends on an ACK body.

## Positive security properties confirmed (beyond author's tests)

- Outer MerID, inner MerID and the status pair are all bound to the connection before any DB write.
- Rotation grace is exactly one credential version wide; two-behind signatures die with 400 and zero rows.
- MerTradeNo mapping is connection-scoped even when a second store shares the same provider account and HashKey.
- `integration.guard_payment_job_family` refuses even owner-level orphan `payment_query_v1` inserts (22023), so the
  wake's `WHERE id=... AND args=...` pinning cannot be circumvented by planting a decoy row.
- The ingress role holds exactly EXECUTE on the two 0136 definers — no table, sequence, river or catalog privilege.
- LIVE is refused at the registrar definer (22023), at `NewInbox` (ErrConfig) and at `cmd/api` config (author's test).

## Evidence class

REAL_PG (disposable PG 18.6 container, real roles/RLS/definers) + MOCK (synthetic AES-GCM signed bodies; no provider
contact). NOT_RUN: real PAYUNi callbacks, LIVE, full-repo `go test -race ./...` (AGENTS.md T02 gate).

## Processes/fixtures

No stray processes or containers started by this unit remain: every `test-focused.sh` run removes its own
`lc-focused-*` container and releases the machine-wide lock. Logs under `output/k3-w4-01b/`.
