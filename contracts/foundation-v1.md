# Foundation contract v1 — frozen for T02/T03 local slice

2026-09-20. Integrator owns this contract, migrations and dependency locks. This is a foundation slice, NOT checkout/payment completion, not production authentication, and not global gate acceptance.

## Go interfaces

Module `livecommerce`, pgx/v5.11.0, River/riverpgxv5 v0.40.0; project Go 1.27.1 and x/text v0.39.0 pinned after vulnerability scanning. Use net/http and slog. No live provider requests.

Package `internal/platform`:

```go
var ErrUnauthorized error
type Scope struct { TenantID, StoreID, PrincipalID string; Revision int64 }
func OpenPool(context.Context, string) (*pgxpool.Pool, error)
func WithScope(ctx context.Context, pool *pgxpool.Pool, token, storeID, permission string, fn func(pgx.Tx, Scope) error) error
func NewHandler(pool *pgxpool.Pool) http.Handler
```

`WithScope` begins READ COMMITTED transaction; hashes opaque token SHA256, rejects token <32 or >512 bytes and invalid canonical UUID store before SQL; calls `identity.resolve_scope($hash,$storeUUID,$permission)` INSIDE transaction. No row -> ErrUnauthorized, no existence leak. Function returns tenant_id, principal_id, authz_revision from stored grants. It then calls parameterized set_config for `app.tenant_id`, `app.store_id`, `app.principal_id` with is_local=true. Callback success commits; error/cancel/panic rolls back; cleanup context bounded and independent of cancelled request. Permission is a server constant, never HTTP-supplied. Scope/DSN/tokens are not logged. Do not expose pool to domain repositories.

OpenPool: max 8 connections, caller-respecting 2s startup budget. Require commerce_runtime membership; reject superuser/BYPASSRLS/schema owner, membership in commerce_auth, or ability to SET ROLE to a privileged/schema-owner role. No startup migrations. Owner credentials are only used by a separate migration/test path. HTTP business queries and scope transactions have a 5s context budget plus transaction-local statement/idle timeouts of 5s and lock timeout of 1s.

## HTTP

- GET /healthz: {"status":"ok"}; no database dependency.
- GET /readyz: ping with <=2s context, 503 generic error if unavailable.
- GET /v1/admin/stores/{store_id}: Bearer token only, permission `store:read`, same generic 401 for absent/revoked/unauthorized scope; returns {id,name,currency} queried from control.stores through scoped transaction.
- GET /v1/admin/stores/{store_id}/audit-events: permission `audit:read`, latest <=50 {id,action,created_at}; no arbitrary pagination or PII.
- All responses Cache-Control: no-store; no CORS by default, no cookies accepted, no tenant from body/header/Host. Forwarded headers ignored. Fixed JSON error codes (unauthorized, unavailable, not_found, internal), no raw PG error/DSN. Unsupported route/method handled by stdlib.
- API binds 127.0.0.1:8080 default. DB URL required from DATABASE_URL; address configurable by LISTEN_ADDR. ReadHeaderTimeout 5s, ReadTimeout 10s, WriteTimeout 15s, IdleTimeout 60s, MaxHeaderBytes 16KiB; signal-driven <=10s shutdown. No migration or local role bypass in API.

## SQL contract

Migration owner creates tables and `commerce_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS`. Deployment grants this group to a distinct login. Runtime has no control over auth tables/grants and cannot create roles/schemas. All direct table grants explicitly enumerated.

control.tenants(id,name,active); control.stores(tenant_id,id,name,currency,active), composite PK; identity.principals(id,active); identity.memberships(tenant_id,principal_id,active,authz_revision); identity.store_grants(tenant_id,store_id,principal_id,permission), composite FKs; identity.sessions(id,token_hash,principal_id,audience,expires_at,revoked_at).

`identity.resolve_scope(bytea,uuid,text)` SECURITY DEFINER with fixed pg_catalog search_path and fully qualified table references; EXECUTE revoked from PUBLIC, only commerce_runtime receives it. Owner is a dedicated `commerce_auth NOLOGIN NOSUPERUSER NOBYPASSRLS` role with SELECT only on auth and store metadata, never the runtime or superuser. An explicit SELECT policy for commerce_auth permits store metadata lookup before tenant resolution; runtime has no membership in this role. It checks merchant audience, live session/principal/member/tenant/store and explicit grant. Auth tables have no runtime SELECT/WRITE. FORCE RLS on direct business tables is additional query safety, not authentication. Store UUID has global UNIQUE as well as scoped composite PK, so one requested store cannot resolve ambiguously across tenants.

`ops.audit_events(tenant_id,store_id,id,principal_id,action,created_at)` scoped composite FK and append-only runtime INSERT/SELECT; no body/PII. Store/audit policies compare BOTH tenant and store; missing local context fails closed.

River migrations use upstream rivermigrate, NEVER hand-written job schema. A dedicated connection holds a session advisory lock across the business migration and River's per-step transaction boundaries. Do not wrap all River migrations in one transaction: upstream step 006 adds an enum value that later steps need after commit. Checksums and upstream migration ledger allow rerunning after partial migration; the API is not started until Apply succeeds. Test-only atomic-enqueue fixture writes an audit row and a `foundation_probe` River job inside one scope transaction; failed transaction must leave neither. No external sending worker is enabled in this slice; trusted worker credential/routing is separate future T06 work.

## Required negative checks

Real PG18: non-owner/no-BYPASS runtime; no-scope reads empty; cross-tenant and same-tenant cross-store denied; composite FK rejects foreign tenant; invalid/expired/revoked token and revoked grant rejected; failed callback and panic leave no audit/job; successful callback commits both; scope does not leak after pool reuse; cancelled context returns boundedly; runtime cannot read auth tables or mutate grant/audit history. HTTP: no bearer cannot exploit cookie/Host/X-Tenant/permission query; authorized store JSON; no token or SQL details in errors.

River's InsertTx upsert needs SELECT, INSERT and UPDATE(kind), plus sequence USAGE; runtime gets no UPDATE(state/args/attempts) or DELETE. No tenant job-list endpoint exists. This synthetic enqueue/rollback check is not proof of full queue tenant isolation; tenant-bound job envelopes and worker validation are a future T06 gate.

Four conversation domains remain identity/webchat/social/support, no universal messages table. Taiwan identifiers remain strings and selection requires server-bound nonce/revision/provider checks; those features are NOT implemented by this foundation migration. Checkout schema/payment/stock contracts are next T01 sub-slices, not silently implied by this API.
