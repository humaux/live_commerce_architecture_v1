# store-domains (R5) — third fix summary (unit/store-domains-fix, fix3)

Implements the three backend follow-ups the UI author flagged in `UI-SUMMARY-codex.md` (协议例外与后端 P0, items
2 and 5). Backend only (Go, SQL, tests); nothing under `apps/` changed. On top of `f43019d`.

## P0 — merchant domain writes are idempotent through command.Run (I02)
- `internal/storefrontdomains/service.go`: `Request`, `Suspend`, `Detach` now wrap the definer call in
  `command.Run(ctx, tx, scope, operation, Idempotency-Key, request, result, fn)` exactly like the other merchant
  commands (ads/catalog pattern). Operation names `storefront.domain.request|suspend|detach`; the request struct
  carries `principal_id` (+ hostname/base_domain or origin) so the receipt is actor-scoped by digest.
  - Same key + same body → `command.Run` replays the first saved result: the TXT token, version and
    `merchant.domain_requested` audit row are **not** rotated/re-added.
  - Same key + different body → `ErrConflict` (receipt hash mismatch), nothing written.
- `internal/httpapi/storefront.go`: the three write routes now require `Idempotency-Key` exactly once
  (`storefrontDomainKeyed`, the same `claimsKey` grammar `command.Run` re-checks) and reject it before any
  transaction opens; the read route forbids it (`storefrontDomainUnkeyed`).
- `mapError` gained a `nil` short-circuit and passes through the package's own sentinels — required now that the
  already-mapped `fn()` error returns through `command.Run` before reaching the outer `mapError`.

## DTO `kind` field (authoritative, never inferred from the suffix)
- `migrations/0106_store_domains.sql`: `read_store_domains` now emits
  `'kind', CASE WHEN d.evidence_ref='platform-subdomain' THEN 'platform' ELSE 'custom' END` per row.
- `internal/storefrontdomains/service.go`: `DomainRow` gains `Kind string \`json:"kind"\``; `Read` validates it is
  exactly `platform` or `custom` (fails closed otherwise).

## Apex DNS instructions carry the verifier's edge A/AAAA set
- `DNSInstructions` gains `EdgeAddresses []string \`json:"edge_addresses,omitempty"\``.
- The caller resolves the same edge source the verifier accepts (`stores.<base>`) outside the transaction and passes
  it into `Request`; the service attaches it **only for an apex host** (a CNAME host keeps just the CNAME target).
  Edge is excluded from the `command.Run` digest so a client retry still replays.

## Gate tests added (real PG, `tests/foundation/store_domains_test.go`)
- `TestStoreDomainsIdempotency` — first request → replay (token/version/audit unchanged) → same-key-different-body
  `ErrConflict` with no audit row.
- `TestStoreDomainsMerchantViewKind` — platform row reads `kind:"platform"`, merchant row `kind:"custom"`.
- `TestStoreDomainsApexEdgeInstructions` — apex host returns the edge set; CNAME host returns CNAME only.
- The `request`/`suspend`/`detach` helpers now use fresh keys per call so lifecycle re-requests still rotate the
  token (a fixed key would replay the first result).

## UI author field names
- DTO row: `kind` (`"platform" | "custom"`)
- DNS instructions: `edge_addresses` (string array, apex only)

## Gates (all green)
- `go build ./...` / `go vet ./...` — exit 0.
- `gofmt -l` on changed Go files — clean.
- `bash scripts/dev/check-gates.sh` — ok (56 modes).
- `bash scripts/dev/depmap.sh --check` — up to date.
- `bash scripts/dev/test-focused.sh '^(TestStoreDomains|TestStorefrontPublish|TestT06WorkerAuthorityAndFunctionACL)'`
  — PASS=31 FAIL=0 SKIP=0.
- `go test ./internal/storefrontdomains ./internal/httpapi` — PASS.

## Not run / expected follow-ups
- Browser gates stay RED at the `apps/` halves (domain-card split, storefront 301) — out of backend scope.
