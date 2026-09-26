# Meta Inbox Go author record — 2026-09-26

Status: **IMPLEMENTED / LOCAL_GO_GATE_PASS / REAL_PG_GATE_NOT_RUN**.

Base `80064b3`, branch `commerce/meta-inbox-go-20260926`; shared
`platform.ValidateMetaIngressPool` helper `a9be7be` was cherry-picked as
`e4c5448` solely for compilation. This author's paths are
`internal/integrations/meta/inbox.go`, `inbox_test.go`, and this record.
Role: integration worker. Model and reasoning configuration are not exposed to
this child runtime; the parent can fill those from its orchestration record.

The private Inbox accepts only the verifier's raw-aware handler callback.
Constructor checks the dedicated ingress pool and initializes a producer-only
River client. Admission uses a single 10-second bounded transaction and local
statement/lock/idle timeouts; rollback has an independent 2-second context.
The frozen batch receipt is checked first. Replay commits without route lookup,
encryption or job insert. Fresh event ordinals are admitted in Key, PayloadHash,
original ordinal order. New quarantines are encrypted without jobs; new routed
events are encrypted with their frozen tenant/store/route epoch and get exactly
one `meta_inbox_v1` River `InsertTx` on `meta_inbox`. Exact verified raw bytes
are encrypted and finalized with the full batch. Any error returns a fixed safe
storage error and leaves no acknowledgment.

Go 1.27.1, darwin/arm64 author gate:

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test -race ./internal/integrations/meta` | 0 | `ok livecommerce/internal/integrations/meta 1.885s` |
| `go vet ./internal/integrations/meta` | 0 | no diagnostics |

Author tests pin sorted lock order with original ordinals, fixed River kind,
queue and args JSON, constructor rejection and formatting/error redaction.
These local tests do not instantiate PostgreSQL or prove SQL/River atomicity.
Real PG role, dedupe, routing, rollback and cross-tenant gates remain **NOT_RUN**
until migration 0028 and independent acceptance are integrated. Public route,
worker processing, retention and provider activation remain outside this unit.
