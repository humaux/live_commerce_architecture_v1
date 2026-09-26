# T07 Meta runtime Go author evidence

- Role: `integration_worker`; actual model `gpt-6-sol`, high reasoning.
- Base: `19ac324`; worktree `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch `commerce/meta-runtime-go-20260926`.
- Frozen contract: `contracts/meta-runtime-v1.md` at base, with subsequent deadline clarification by integrator. Local dependency cherry-pick `154d140` is integrator commit `00f3814` for platform/0030, not authored here.
- Authored paths: `internal/integrations/meta/env.go`, `env_test.go`, `runtime.go`, `runtime_test.go`; `cmd/api/meta.go`, `meta_test.go`, `main.go` Meta wiring; `cmd/meta-worker/main.go`, `main_test.go`; this record.

The API defaults Meta off and reads only the flag until enabled. It checks the effective listener for literal loopback before reading Meta DSNs, keys or app credentials. Exact strict JSON loaders reuse `parseStrict`, `NewVerifier` and `NewPayloadKeyring`. API assembly opens a separate ingress pool, proves same-database identity, checks the frozen readiness predicate and mounts a raw-path router outside ServeMux. The separate worker reads only payload keys, opens ordinary worker and Meta consumer pools, proves same-database identity, registers only the `meta_inbox` queue and uses `jobqueue.Run` with a fixed local readiness witness. No provider call or send path was introduced.

The shared ten-second API assembly context now covers opening the existing main pool and constructing existing identity and buyer handlers as well as Meta. This changes the time budget of those existing startup paths; it is canceled before the HTTP server starts, and is not the runtime request context. The worker's ten-second pool/constructor context is likewise canceled before the River run context. Independent real-process identity/buyer startup and session regression remains required.

## Author checks

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test -race -count=1 ./internal/integrations/meta ./cmd/api ./cmd/meta-worker` | 0 | Meta 1.552s; API 2.320s; worker 2.301s |
| `go vet ./internal/integrations/meta ./cmd/api ./cmd/meta-worker` | 0 | no findings |
| `gofmt -w` on authored Go files and `git diff --check` | 0 | clean source formatting |

Unit cases cover default-off zero sensitive reads, listener-before-secret ordering, exact app/key JSON and bounds, worker no app credential reads, canonical concurrency, endpoint duplication/forgery, encoded and cleaned path aliases, non-Meta route preservation and redacted formatting. A source review caught a duplicate-path bug in the draft router; the final code checks map-key presence and has a direct-constructor regression case.

Real PostgreSQL role/readiness/same-database probes and real API plus worker process gates MR02–05 are NOT_RUN by this author; they belong to the independent PG/process author and integrator. This author did not deploy, configure a provider callback, or use production credentials.
