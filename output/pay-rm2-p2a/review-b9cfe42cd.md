**Verdict: REQUEST CHANGES**

Review of commit b9cfe42cd (PR #70, worktree .worktrees/pay-rm2-p2a, HEAD b9cfe42cd3b90e5e274a2b7ae1cfa0355621d86f). This was a read-only review. I did not run any Go tests or browser tests, which is policy. Evidence level: E1 (static trace, gofmt/vet, and the CI log from run 38755929156). Runtime gates: NOT_RUN by me.

## Findings

### P1: auth-real.spec.ts still has one whole-body equality on a per-request error envelope
`tests/admin/auth-real.spec.ts:431-433`
```ts
expect(
  (await browserJSON(page, `${detailPath}/rotate`, rotationOptions)).body,
).toEqual(rotated.body);
```
`rotated` is a 422 `invalid_request` envelope (asserted at :428-429). `internal/httperror/error.go:13-38` mints a fresh random `request_id` for every request, so this comparison fails in the same way as the three that were fixed. The CI log shows the failure: `output/pay-rm2-p2a/ci-38755929156/.../playwright.log:18-19,25` reports a differing `request_id` at the first replay site (old line 395). Playwright stops at the first failure, so the account-create replay masked the rotate replay. After this commit CI will move on and fail at :433.
Fix: use the same pattern as :396-398. Assert `status` is 422, then compare `{...body, request_id: ""}`.

### P2: misleading claim that the second process reloads the key, and the decrypt check is tautological
`tests/foundation/account_process_test.go:217-219` has the comment "Both sealed versions still decrypt with the reloaded key configuration". This is not true:
- `maAssertPersistedSecret` (`tests/foundation/merchant_accounts_test.go:179-216`) decrypts with the test's own `m.keys`, not with the restarted process.
- The ciphertexts were sealed by `maHistorical` (`tests/foundation/payuni_history_helpers_test.go:18-79`) using the same `m.keys` and an identical AAD struct. The check is the test decrypting its own encryption. It proves nothing across the restart.
- Production `accounts.Service.Get` (`internal/integrations/accounts/service.go:78-102`) only reads `key_id`, and Create and Rotate return `ErrInvalid`. The API process never calls `Keyring.open` or `Keyring.seal` for PAYUNi. Its only remaining link to the keyring is parse and validate at startup (`cmd/api/accounts.go:48-54`). Readiness of the second process therefore proves only that the key config still parses.

Before this commit, the second process sealed the rotated secret with its reloaded keyring and the test decrypted it independently. That was a real key-reload proof. It is now lost.
Fix: reword the comment to "sealed history is intact and readable with fixture keys; process key reload is limited to startup parse". Name the generic equivalent. `tests/foundation/stripe_process_test.go:43` `TestStripeSP15Process` uses the same `accounts.Keyring` (`internal/integrations/accounts/stripe_crypto.go:164-272`): a wrong custody keyring cannot open, the correct one opens, and the built `cmd/payment-worker` binary reaches readiness with that keyring env. That run is in-process for decrypt and binary-level for readiness only.

### P2: lost guarantees are not named (project rule: name equivalents for lost guarantees)
Guarantees of the original test that are gone, with my classification:
| Lost guarantee | Kind | Equivalent today |
|---|---|---|
| Create persists exactly one account+binding+credential through the real binary | retired-feature-only | none needed (write retired) |
| Rotation after restart keeps id/binding/state, adds exactly one revision, no provider work | retired-feature-only | none needed |
| Process-held `COMMERCE_ACCOUNT_REPLAY_KEY` fingerprint replay across restart | retired-feature-only (credential fingerprint) | none needed |
| Process 2 seals/decrypts with reloaded keyring | generic (keyring reload) | partial: `TestStripeSP15Process` (stripe_process_test.go:43), not via cmd/api restart |
| Idempotent replay of a *successful* write after a real API process restart returns the stored receipt with no durable change | generic (command receipts in PG) | **not found**: `TestStudioBackendSTU03RealAPIWorkerRestart` (studio_process_test.go:238) replays within the same API process. `TestK3LcB1IdempotencyKeyReuse` (k3_lc_b1_adversarial_test.go:325) runs in-process. |
| PG recovery of persisted metadata across restart (GET/list) | generic | kept (account_process_test.go:200-207) |
| PID change, clean SIGTERM, logs free of secrets | generic | kept |

This commit and `output/pay-rm2-p2a/DELIVERY.md` record none of this. The lost generic guarantee "successful idempotent write replayed after an API restart" needs either a named equivalent or an explicit NOT_COVERED/BLOCKED entry. Restoring the feature is not required.

### P3: refusal durability is checked with counts, but a full-row snapshot already exists
`account_process_test.go:191,214` use `m.facts` (counts only). In-place rewrites such as `merchant_accounts.credential_version` or binding updates would go unnoticed, and the account is not re-read after the refused rotation. `maHistorySnapshot` (`payuni_history_helpers_test.go:83-97`, comment "counts alone cannot") is in the same package. Suggest `maHistorySnapshot` before and after, or a GET re-check after :213. Note that `facts[6]` counts `river.river_job` globally, which is inherited, not new.

## Verified OK

1. **pmHistorical NULL binding** (`payuni_history_helpers_test.go:110-111`). This matches pre-removal production exactly. `main:internal/payments/methods.go:138-146` `SetMethod` inserted `nullif($9::text,'')::uuid, nullif($10::bigint,0)`, so an unlink stored NULL/NULL. Readers coalesce back to 0 (`internal/payments/methods.go:228`, `discovery.go:33`). `diagnose` (methods.go:206-211) emits CONNECTION_MISSING and skips CREDENTIALS_UNVERIFIED when `ConnectionID==""`, with `BindingVersion`/`CredentialVersion` 0 for the check at `payment_methods_test.go:221-229`. No masking: a linked fixture with binding 0 would produce connection-set plus NULL binding, which violates `method_connection_pair` (`migrations/0015_payment_methods.sql:28-29`) and fails `t.Fatal`, as production would. Non-test callers (`merchant_settings_http_test.go`, `merchant_settings_discovery_test.go`) pass linked or zero pairs consistent with `validMethodInput` (methods.go:166-167).
2. **maHistorical is compatible with the production reader.** The AAD struct is field-for-field identical to `credentialAAD` (`internal/integrations/accounts/crypto.go`). `Get` joins `account_credentials` on `version=credential_version` (=2), which exists. `first` comes from the in-process `m.get` and is DeepEqual-compared with the binary's GET and list. `expected_version: 2` in the rotation body is correct: it matches the current version, so a 422 proves retirement and not a version conflict (409).
3. **Blanking request_id is the minimal relaxation** (:396-398, :526-528, :550-552). `code`, `message`, `retryable` and `details` are still compared, and an explicit status assertion was added, which is stronger than before. The other `toEqual` calls in the file compare success bodies (no envelope) and are fine. The only remaining envelope compare is the one in the P1 finding.
4. gofmt and vet are clean.

## Commands run
| Command | Exit |
|---|---|
| `git show b9cfe42cd` | 0 |
| `gofmt -l tests/foundation/account_process_test.go tests/foundation/payuni_history_helpers_test.go` (no output) | 0 |
| `go vet ./tests/foundation/` | 0 |
| `go vet -tags browser ./tests/foundation/` | 0 |
| `git show main:internal/payments/methods.go` (pre-removal SetMethod) | 0 |
| grep/sed static reads (spec, helpers, httperror, accounts, CI logs) | 0 |

Go tests and browser tests: NOT_RUN (forbidden locally).
