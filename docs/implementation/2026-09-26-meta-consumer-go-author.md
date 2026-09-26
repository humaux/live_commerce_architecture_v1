# T07 Meta social consumer Go author evidence

- Role: `integration_worker`; actual model `gpt-6-sol`, high reasoning.
- Base: `23a37bb`; isolated branch/worktree: `commerce/meta-consumer-go-20260926` at `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`.
- Contract: frozen `contracts/meta-consumer-v1.md` at base. Local dependency cherry-pick `6855636` is the integrator's `f71f7eb` SQL/platform commit; it is not authored by this worker.
- Authored paths: `internal/integrations/meta/consumer.go`, `projection.go`, `consumer_test.go`, `projection_test.go`, and this record.

The private projection replays one canonical decrypted unit through the existing strict parser and Meta classifier, then checks exact event asset, kind, key, hash and bytes. Message peers and comment subjects use the contract's scoped tuple hashes. The worker borrows only the validated consumer pool, uses the fixed River kind/queue and exact event/job/attempt SQL interface, and commits READY or read-only terminal outcomes in a bounded READ COMMITTED transaction. Errors and formatting are fixed/redacted.

## Author checks

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test -race -count=1 ./internal/integrations/meta` | 0 | `ok livecommerce/internal/integrations/meta 2.395s` |
| `go vet ./internal/integrations/meta` | 0 | no findings |
| `gofmt -w` on four authored Go files | 0 | formatted source |

Go tests cover all seven frozen kinds, scoped identities, Unicode/attachment retention, wrong evidence and quarantine rejection, invalid River jobs, SQLSTATE mapping and formatting/JSON redaction. These are source/unit checks. Actual River plus PostgreSQL MC03–06, authority MC01, migration MC07 and independent review are NOT_RUN by this author; the separately assigned PG author/integrator own them. No provider calls, production calls or network side effects were made.
