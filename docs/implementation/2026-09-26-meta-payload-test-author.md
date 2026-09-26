# T07 Meta inbox payload crypto — independent test delivery

- Role: `test_worker`; model/reasoning inherited from the parent task without an explicit child override. The child runtime did not expose concrete identifiers.
- Base `09d83f3dc9707737459b009dbc81eacc7e4e5697`; branch `commerce/meta-payload-tests-20260926`; worktree `/Volumes/data/worktrees/commerce-meta-payload-tests-20260926`.
- Owned writes only: `internal/integrations/meta/payload_acceptance_test.go` and this report. Implementation commit `3e3dd5c` was cherry-picked locally as `9698868` only to run tests, not authored by this worker. Encryption contract frozen at `183417f`, with exact 13-element AAD JSON array clarified by `1322850`; neither contract nor source was edited here.
- Six independent top-level tests with 45 named subtests cover 1–16 distinct nonzero 32-byte keys, syntax/size bounds, caller-key copying and old-key rotation; raw/event/quarantine round trips and independent nonce/ciphertext; every valid AAD authority field (class, ID, app, object, body/event/payload hash, tenant/store/route/epoch, KeyID); malformed/tampered nonce/cipher/tag and no plaintext on errors; exact 1 MiB raw / 4 MiB event bounds; formatting/JSON redaction including pointers.
- Interoperability is independently checked with Go standard-library `crypto/aes` + `cipher.NewGCM` and an explicit AAD array constructed from the frozen contract, never the implementation's AAD helper. It opens implementation ciphertext and supplies independently sealed ciphertext to `PayloadKeyring.open` for all three classes. A correctly tagged ciphertext with an intentionally wrong plaintext digest proves the *post-open* hash check. All keys and bytes are synthetic fixtures.

## Actual local gates

- `go test -race -count=1 -v ./internal/integrations/meta` — exit **0**; 16 top-level tests in this package, including the six independent groups / 45 named subtests. Full log `/private/tmp/meta-payload-tests-nz9UB6/race-final.log`, SHA-256 `7cefbabcd2b528bb76d5f3420128f378c89288a45f7ed1980ffd1e51710a374e`.
- `go vet ./internal/integrations/meta` — exit **0**. Empty log `/private/tmp/meta-payload-tests-nz9UB6/vet-final.log`, SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- `gofmt` applied and `git diff --check` exit **0**.
- The first race run exited **1** solely because this worker's `bad UUID` fixture uppercased an all-digit UUID, leaving it unchanged. That erroneous fixture was replaced with an explicitly uppercase A–F UUID; no source or contract assertion was weakened. Preserved log `/private/tmp/meta-payload-tests-nz9UB6/first-race.log`, SHA-256 `d2d0fa37f65d33355ead0812cafdf6dcfef2c7875cb00a8b470cf83c981a4f54`.

This is the crypto-only slice of MI02, not a durable inbox acceptance. **NOT_RUN:** PostgreSQL role/RLS and cross-scope ciphertext access, SQL/log/job plaintext inspection, atomic receipts/River jobs, routing/dedupe/retention, HTTP+PG durability, public route, and provider/LIVE behavior. Root independently reviews and reruns before integration.
