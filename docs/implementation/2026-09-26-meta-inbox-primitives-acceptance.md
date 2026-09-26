# T07 inbox prerequisites — raw handoff and payload encryption

2026-09-26. **PASS_LOCAL_PRIMITIVES_ONLY**. This is the crypto part of MI02,
not a durable inbox, PostgreSQL isolation, retention, Meta or full-SaaS gate.

## Delivered and independently reviewed

- Root `69cb0de`: private raw-aware synchronous callback preserves exact signed
  whitespace/escapes and mixed-asset envelopes after full verification. Public
  `NewHandler` stays compatible; invalid input never reaches the callback;
  persistence failure remains fixed HTTP 503, never a leaked storage error.
- Implementation `3e3dd5c` integrated as `6db9e8f`: `PayloadKeyring`, independent
  of payment credentials, owns copied 1..16 nonzero distinct 32-byte keys;
  AES-256-GCM uses random 12-byte nonces and fixed 13-element JSON AAD. Class,
  exact context, size, digest and envelope checks fail closed. Old-key rotation
  remains possible without changing frozen event context.
- Independent tests `6a2de59` integrated as `1738a88`: six test groups / 45 named
  subtests; independent standard-library AEAD seals/opens against explicit
  contract AAD, including a valid tag with incorrect plaintext digest.
- Read-only source/design reviewer found no remaining P0/P1/P2 in these
  primitives or the database design. Review memory
  `4427af12-ea0c-4323-87ce-2eb4c13d5a55`; author memory
  `cf19e810-f272-401f-bfae-33e84464a3b2`; independent test memory
  `21639d02-3a79-413c-8594-b7167ee52e34`. Database design frozen at `11da18e`.

Author (`integration_worker`, gpt-6-sol/high) and independent `test_worker`
(parent model/reasoning inherited, concrete runtime identifiers not exposed)
used separate branches/worktrees from `09d83f3`, with disjoint allowed paths.
See [author record](2026-09-26-meta-payload-author.md) and
[independent record](2026-09-26-meta-payload-test-author.md). Root reviewed source
and test changes and ran the final merged tree at `1738a88` on Go1.27.1 darwin/arm64.

## Actual root gates

| Command / evidence | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.27.1 go test -race -count=1 -v ./internal/integrations/... ./tests/integrations/meta` | Exit 0; 64 top-level PASS, 0 FAIL, 0 SKIP |
| `/Volumes/data/output/meta-inbox-primitives-root-final-1.log` | SHA-256 `eb4df332164cef46d4b7fbce5dcc6cbcf27501b0f74c9decafe38de24dacb2ac` |
| `GOTOOLCHAIN=go1.27.1 go vet ./...` | Exit 0; empty `/Volumes/data/output/meta-inbox-primitives-vet-final-1.log` |
| Independent final log, copied to `/Volumes/data/output/meta-payload-independent-final-1.log` | SHA-256 `7cefbabcd2b528bb76d5f3420128f378c89288a45f7ed1980ffd1e51710a374e` |

Independent first-run failure was a **test fixture error**, not a source defect:
uppercasing an all-digit UUID changed nothing. Replaced it with an explicit
uppercase A–F UUID; no source or assertion was weakened. Retained root copy
`/Volumes/data/output/meta-payload-independent-fixture-error-1.log`, SHA-256
`d2d0fa37f65d33355ead0812cafdf6dcfef2c7875cb00a8b470cf83c981a4f54`.

## Design corrections frozen for database implementation

[Inbox contract](../../contracts/meta-inbox-v1.md) requires trusted immutable
asset ownership rather than merchant self-assertion, historical receipt lookup
before mutable routing, restricted whole-batch encryption plus scoped event
bodies, permanent dedupe independent of River pruning, and exact atomic jobs.
The review closed age-only purge data loss: pending/unreviewed payload remains
recoverable; raw batches wait for every member and linked job to be terminal.
Registrar cannot terminalize data; curator cannot claim business processing;
both remain unassigned outside test fixtures until actual authorized workflows.
River admission must guard UPDATE as well as INSERT against reserved-job mutation.

**NOT_IMPLEMENTED / NOT_RUN:** migration0028/post-River guards, Inbox PG service,
encrypted DB persistence and ACL/RLS, atomic jobs/ACK-loss/replay, trusted OAuth
proof issuance, actual routing, retention execution, consumer processing,
production key loading/rotation, public route/limits/operations, provider/LIVE.
No migration, production process, customer data, broadcast or merchant switch was
touched. This increment changes no UI and claims no new browser acceptance.
