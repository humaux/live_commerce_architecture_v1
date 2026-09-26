# Meta payload encryption author record — 2026-09-26

Status: **IMPLEMENTED / LOCAL_GO_GATE_PASS / INDEPENDENT_ACCEPTANCE_PENDING**.

Scope: only `internal/integrations/meta/payload.go` and its author tests. Base
`09d83f3`; branch `commerce/meta-payload-20260926`. This implements the frozen
Encryption sub-contract in `contracts/meta-inbox-v1.md`, including its exact
13-element compact JSON AAD array. It has no SQL, handler, provider, or PSP
keyring integration.

Design: constructor copies 1–16 distinct nonzero AES-256 keys and validates
their IDs. Seal and open validate the class-specific context matrix and bounded
body/envelope, authenticate every context field and the envelope key ID via
AES-256-GCM AAD, use a fresh 12-byte random nonce, and check the expected
SHA-256 plaintext digest on both sides. All three sensitive types redact
string, Go formatting, and JSON. Public errors are fixed strings.

Local gate on Go 1.27.1, darwin/arm64:

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test -race ./internal/integrations/meta` | 0 | `ok livecommerce/internal/integrations/meta 1.552s` |
| `go vet ./internal/integrations/meta` | 0 | no diagnostics |

Author tests cover keyring rejection/ownership/rotation, all three classes,
AAD tamper, malformed contexts/envelopes, bounds, digest mismatch, and
redaction. The first local race run failed because an uppercase UUID test
fixture contained only digits; the fixture was corrected and the gate reran
green. No test threshold was relaxed.

Independent verifier should run its separately authored acceptance suite and
cross-check the frozen AAD vector. Database, production secret handling,
retention, and public webhook admission gates are **NOT_RUN** by this change.
