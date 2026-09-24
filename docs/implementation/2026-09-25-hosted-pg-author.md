# Hosted payment actual PostgreSQL gate — test worker evidence

Date: 2026-09-25. Role: `test_worker`. Delegated runtime did not expose its actual
model identifier or reasoning setting, so neither is inferred here. Base:
`d6720aa`; isolated branch/worktree:
`codex/commerce-hosted-pg-20260925` at
`/Volumes/data/worktrees/commerce-hosted-pg-20260925`. This task owns only
`tests/foundation/hosted_payment*_test.go` and this record. Test commits:
`7d3107b`, `f0b284a` and `aa0bec5`. Dependency commits pulled from integrator/Go author:
`f938e6f`, `4450831`, `12571fb`, `3645071`; no dependency files authored here.
Frozen contract: `contracts/payment-hosted-v1.md`. Humaux coordination task:
`57c89d96-5533-436b-9064-e6817341e0e0`.

## Actual gate and evidence

Command (exit 0): `bash scripts/dev/test-local.sh --payment`, against a
task-owned, temporary PostgreSQL 18 database with migration `0025`, Go race
instrumentation, and the existing payment start/query/capture cases. The final
run passed 16 hosted top-level tests and all existing payment cases in 56.911s.
Final stronger log: `/Volumes/data/output/hosted-pg-author-payment-4.log`;
SHA-256: `4bd360af52dc3054dfe0d6021e0dd2807b07f7116a8356f73a0521da2438dc7c`.
Compile-only `go test ./tests/foundation -run '^$' -count=1` also exited 0.
The root integrator owns the independent full `./...`/race/vet and source
review gate. This author run does not substitute for it.

| Gate | Actual PostgreSQL assertions |
| --- | --- |
| HP01 | One frozen attempt, query operation, original River query job with exact operation ID and version, command receipt, event, page, awaiting order and pending stock commit together. Missing local AES key and injected page INSERT error roll every fact and job back. |
| HP02 | Same key and full hosted input replay the exact receipt; stored form and prepared/deadline timestamps remain byte/time identical. Locale, method version, order, endpoint config and profile changes conflict; another key cannot add an attempt. All three locales persist and independently decrypt to the expected wire language. |
| HP03 | Two concurrent takes produce one `ISSUED` form and one `ALREADY_ISSUED` no-form response. A lost-response style repeat, including after binding disable, never reissues. The root adds an independent COMMIT-ack-loss network fault test in its own file. |
| HP04 | Actual role/function grants, fixed definer path, non-writer table ownership, no direct form/ciphertext table access, no `SET ROLE` writer escalation, exact column UPDATE grants and one-way RLS were checked. Ordinary checkout SQL cannot load/save/take even copied valid form. Cross-owner/store/order, forged GUC/capability, revoked capability and malformed outer action, merchant, version, hex, hash, array field were denied. |
| HP05 | Before preparation and first handoff, binding disable, hidden method, credential head rotation and revoked qualification fail closed. MOCK proof does not qualify SANDBOX/LIVE. Qualification expiry during order lock, form INSERT trigger, and handoff UPDATE trigger rolls back. Nontransactional sequence counters prove each delayed trigger was actually entered before the expected conflict. `pg_blocking_pids` proves the exact test transaction blocked each take before qualification expiry or capability revocation. A test-only owner shift of immutable attempt time exercises the local 60-second database deadline comparison without a 60-second suite delay. |
| HP06 | Independent AES-GCM verifier, separate from PAYUNi client/keyring code, authenticates/decrypts the synthetic provider wire. It checks exact MerID, trade, amount in TWD, original attempt Unix timestamp, fixed return/notify URLs, credit selector, product description, page timeout and locale. AUTHORiZED financial fact or review case blocks first release. No form or credentials are printed in test output. |
| HP07 | Author's payment subset passed on actual PG and race. Root's independent full regression, vet and source verdict are recorded separately by integrator; this record claims only the author-run scope. |

The earlier pass at `/Volumes/data/output/hosted-pg-author-payment-3.log`
(SHA-256 `4921b2fec1764dcfc8197a00254a1578606d3465d904ec77ff7222531e2b2743`)
covered the same cases but used sleep-only wait timing and had no durable trigger
entry marker. It is retained as a weaker baseline, not the final causal gate.
The first author attempt was stopped after a test-side pgx mistake: `Query`
deferred a permission error until row consumption, leaving rows open during
pool cleanup. It was corrected to `QueryRow.Scan`, and the subsequent two
isolated actual-PG payment runs passed. The failed diagnostic log remains at
`/Volumes/data/output/hosted-pg-author-payment.log`; the earlier corrected
pass remains at `/Volumes/data/output/hosted-pg-author-payment-2.log`.

## Boundaries

All merchant IDs, keys, buyer capabilities, payment facts, URLs and reports are
synthetic inside temporary local PG. No provider transport, production data,
customer payment, browser flow or deployment was executed. The privileged
hosted signer is the frozen trust boundary: the SQL shape checks do not prove
they can authenticate a forged inner `EncryptInfo` from a compromised signer.
The manual attempt-time shift tests the local deadline branch, not elapsed real
time or a provider's undocumented timestamp acceptance window. Payment success,
real merchant qualification, sandbox charge and public UI remain outside this
internal gate.
