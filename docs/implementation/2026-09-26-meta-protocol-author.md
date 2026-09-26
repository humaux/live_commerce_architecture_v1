# T07 Meta webhook protocol v1 — author delivery

- Role: `integration_worker`; model: `gpt-6-sol`, reasoning: high.
- Base: `5e4a46212630bb7bd5a4fa3e80240d8fde85371b`; branch: `commerce/meta-protocol-20260926`; worktree: `/Volumes/data/worktrees/commerce-meta-protocol-20260926`.
- Scope: `internal/integrations/meta/*.go` and this note. Contract revision read from main at `e654147`; this branch does not modify the contract or mount a route.
- Implementation: bounded raw-byte HMAC-SHA256 admission, strict token JSON with decoded duplicate-key/depth/UTF-8 checks, exact Page/Instagram unit normalization and canonical identity, fixed redacted formatting, synchronous HTTP challenge and commit-gated ACK.
- Adversarial correction: per-unit quarantine retains only validated bounded entry context; malformed entry time is preserved once in a whole-entry quarantine. Same `message.mid` retains Key across payload changes while PayloadHash changes. No event slicing or partial callback.
- Local synthetic evidence: `GOTOOLCHAIN=go1.27.1 go test -race ./internal/integrations/meta` exit 0; `GOTOOLCHAIN=go1.27.1 go vet ./internal/integrations/meta` exit 0; `git diff --check` exit 0. Test cases cover signature byte changes, strict JSON, depth, 1000/1001 event bound, linear quarantine payload size, stable MID key, redaction, zero verifier, HTTP gates and commit failure.
- Gate scope: local MWP01–MWP05 implementation checks only; independent reviewer/test author and root rerun remain required. PostgreSQL receipt/job transaction, encryption and permanent uniqueness, public route, rate limits, Meta signed sample/OAuth/subscriptions and LIVE behavior are NOT_RUN.
- Durability boundary: `NewHandler` trusts the supplied callback to return nil only after the complete batch has committed atomically. A synthetic test callback is not production durability. Same-MID changed-payload conflict must be quarantined by the later durable admission layer.
