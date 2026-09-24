# Merchant-owned account browser intake v1

2026-09-24, draft for independent preflight. Extends merchant-accounts-v1 and
merchant-settings-http-v1; no migration, new provider, collection or shipment.
User selected a wizard: platform → own account → methods → status. Saved account
metadata is CONFIGURED_UNVERIFIED, never evidence of provider admission.

## Frozen transport and composition

- Base `/v1/admin/stores/{store_id}/provider-accounts`.
- GET collection: existing pagination `limit/cursor`, scoped UUID keyset, default
  50/max 100, `items/next_cursor`. No credentials or encryption key IDs.
- POST collection: `{provider,environment,account_id,credentials:{hash_key,hash_iv}}`.
- GET `/{connection_id}`: safe metadata.
- POST `/{connection_id}/rotate`: `{expected_version,credentials:{hash_key,hash_iv}}`;
  connection ID comes only from the path. No environment/account rebinding.
- Writes require `integration:manage`, Idempotency-Key, CAS for rotate; reads
  require `integration:read`. Reuse accounts.Create/Rotate/Get and command receipts,
  their after-wait authorization recheck, pgx scoped transaction and safe errors.
- Exact-resource routes reject all queries, including trailing `?`; collection GET
  accepts only the existing pagination parser. 64 KiB strict JSON/unknown/trailing
  rejection, no-store/request ID, 5s shared transaction deadline remain unchanged.
- Dedicated secret wire structs have redacted JSON/String/GoString representations;
  JSON input can decode only named secret fields. Do not weaken Credentials logging
  redaction to serialize a transport request. Public metadata DTO explicitly omits
  key_id. Public API has no decrypt/export/verify/delete operation.
- HTTP composition owns `httpapi.Options { SessionStoreList bool; Accounts *accounts.Service }`.
  Existing no-option callers remain valid. Convert the few composition callers;
  platform.Options stays provider-neutral. Nil Accounts fails authenticated account
  routes with 503, not a fake connection or fixture fallback.
- Credential writes have a per-authenticated-tenant/store admission budget: 60
  per fixed window of one minute, counted after scope authorization before domain
  execution (including replay/invalid semantic requests). Stdlib mutex/monotonic
  time; at most 4096 active scope windows. Prune expired windows when admitting
  a new scope at capacity; if still full, fail the new scope with 503, leaving
  existing scopes usable. No raw tokens/account IDs, timers or new dependency.
  One scope's exhaustion is 429 `rate_limited`, Retry-After 60, zero durable effect;
  it cannot consume another scope's allowance. This is bounded process protection,
  not a distributed quota; edge anti-abuse remains a deployment requirement.
  Reads/unrelated commerce routes do not consume the budget. The initial global
  60/min proposal was rejected by independent preflight for cross-tenant starvation.

## Deployment secret boundary

`COMMERCE_ACCOUNTS_ENABLED` uses existing strict flag parsing, default disabled.
Disabled mode reads no account secrets and creates no River client/account service.
Enabled requires identity enabled and literal loopback API listener; public BFF
origin already requires HTTPS except explicit loopback tests. Load:

- `COMMERCE_ACCOUNT_ACTIVE_KEY_ID`
- `COMMERCE_ACCOUNT_KEYS_JSON`: array of `{id,key_base64}` entries, 1..16 unique
  IDs, each canonical standard-base64 32-byte key, strict JSON/no extra value.
- `COMMERCE_ACCOUNT_REPLAY_KEY`: canonical standard-base64 32 bytes, independent
  from every encryption key. It must be kept stable for permanent replay.

No fallback/generated production key. Fixed safe errors never include config.
Reuse accounts.NewKeyring and core.New with an insert-only River client on the
ordinary runtime pool; do not start workers or perform provider discovery/calls.
Old encryption keys must remain available for historical operations. Environment
configuration is not KMS, backup or a verified live deployment; document those gates.

## Browser boundary

Extend existing store BFF allowlist only for the four account routes and the five
already-built method settings routes (GET/PUT/inspect). Validate canonical IDs,
country, known resource and code syntax before proxying; exact paths reject queries.
Reuse authenticatedStores, HttpOnly session and origin+CSRF. Account resources
require actual identity mode; legacy dev bearer fixture cannot intake secrets.
Inspect alone is read-only POST without Idempotency-Key; mutations still require it.
Never forward incoming tenant, bearer, cookie or arbitrary headers to Go.
No credential storage in URL, browser storage, SSR props, telemetry or retry journal.
UI clears input secrets after submit; network-unknown recovery reads metadata and
requires explicit re-entry with retained nonsecret command identity, never autofills.

## Acceptance gates

- AC01: actual HTTP → scoped runtime PG saves, paginates, reads, rotates, restarts;
  safe metadata only, encrypted DB content and no provider job/payment/method enable.
- AC02: identical replay, changed-secret conflict, rotate CAS, historical metadata
  and disabled binding stay correct; revoked permission and foreign-store reject.
- AC03: invalid paths/queries/body/secret format/config, expired/missing session,
  nil service, rate exhaustion and malformed pagination fail safely, no input echo.
  Limiter tests prove independent scope budgets, reset, pruning/capacity and races.
- AC04: real Chromium → packaged Next BFF → Go → isolated PG with signed MOCK IdP:
  save/read/rotate/list, CSRF/origin rejection, secrets absent from responses/storage,
  settings PUT allowed and inspect read-only. Synthetic keys only; no live PSP.
- AC05: full PG/race/vet, TypeScript/build and independent review. Author cannot
  certify alone. UI visual/browser and real provider gates remain separate.

No payment/logistics enable gate is removed. No credentials taken from client
production. Root owns this contract, integration and independent PG/browser gates;
isolated author owns account list/HTTP/options/config and local unit tests only.
