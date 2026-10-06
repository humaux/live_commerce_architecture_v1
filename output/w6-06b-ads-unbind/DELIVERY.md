# DELIVERY — W6-06B Meta ad account unbind + catalog feed URL entry (backend, small)

- Branch `unit/w6-06b-ads-unbind`, worktree `.worktrees/w6-06b-ads-unbind`, base trunk `6277731e`.
- Commits: `adf6e8c8` (brief + contract amendment), `6c537d9c` (RED tests + red.log), `be44d6fe` (GREEN implementation), + this DELIVERY commit.
- Implementer model: Aliyun Qwen (per main-agent spawn record); acceptance must be independent (author ≠ acceptor, AGENTS.md).
- Evidence tiers: **REAL_PG** (local PG via `scripts/dev/test-focused.sh`, fake `ConnectFunc` — no Meta call anywhere), **MOCK** nowhere needed; nothing LIVE, nothing SANDBOX. No production secrets, no buyer PII in fixtures (fake tokens are `make([]byte,…)` fills).

## What shipped (≤10 lines)

1. `contracts/meta-ads-v1.md` Amendment W6-06B (:690) — unbind semantics + refusal codes + feed entry, interface-first.
2. `migrations/0160_ads_unbind.sql` — `integration.meta_ads_unbind(bytea,uuid,text)` (:40, OWNER commerce_integration_writer, EXECUTE commerce_runtime :93) and `ads.catalog_feed_url(bytea,uuid)` (:104, OWNER commerce_ads_writer, EXECUTE :123). No table, no GRANT/POLICY delta, §4.4 privilege list untouched.
3. Unbind = detach, never delete: FOR UPDATE lock of every enabled `meta_ads` binding of the asset (serializes with `claim_operation`, 0008), `operations_in_flight` RETURN (DISPATCHING/UNKNOWN/ACKNOWLEDGED listed ≤50, never cancelled — external-operation-v1 rules 4–5), sealed token heads+versions destroyed (0108 pattern), idempotent no-op `{unbound:false,already_unbound:true}`.
4. `internal/ads/unbind.go` — `Service.Unbind` (:66): command receipt `ads.meta.unbind`, per-binding CAS disable (0008:115) in the SAME tx so the frozen 0074 `bindings_ads_disable_guard` can refuse **PT409 binding_in_use** atomically (R2-ADS-PAUSE-1; rollback keeps credentials), audit `ads.account_unbound`; `Service.CatalogFeed` (:125).
5. `internal/ads/errors.go` — frozen codes `operations_in_flight`/`binding_in_use` (:60), `Refusal.Details` + `ErrorDetails()` (:38), targeted PT409 mapping (:104).
6. `internal/httpapi/ads.go` — `POST …/ads/meta/unbind` (:85, keyed, exact body `{ad_account_id}`), `GET …/ads/catalog-feed` (:207, ads:read, no query/key), 405 fallbacks (:53), `adsScope` answers via `respondErrorDetails` (:288; envelope of every other route unchanged).
7. History kept (proved): drafts, remote objects, `ads.connections`, oauth states, operation ledger; READY ops go `STALE_BINDING` at next claim (rule 5); re-bind = new binding via the frozen connect flow, disabled binding never re-used.
8. Feed URL = lexicographically-first ACTIVE origin (C collation) of the published store + `/feeds/meta.csv` (public unsigned feed, `internal/attribution/feed.go`); no secret ⇒ `ads:read` suffices (amendment §B).
9. Pin honestly updated: `tests/foundation/r2_integration_upgrade_test.go` migration count 81→82 (+comment line).
10. Tests red→green: `output/w6-06b-ads-unbind/red.log` (404s + SQLSTATE 42883 before implementation), `green.log` (all runs below, exit 0).

## Contract changes

- `contracts/meta-ads-v1.md` — Amendment W6-06B (§A unbind, §B catalog feed entry, §C tests). Additive; frozen trigger untouched; new frozen refusal codes `operations_in_flight` 409, `binding_in_use` 409.
- No OpenAPI/shared-schema edits (integrator-only files); see To-do below.

## Tests & exit codes (all on this machine, `scripts/dev/test-focused.sh` = REAL_PG)

| Command | Result |
| --- | --- |
| `bash scripts/dev/test-focused.sh 'TestAdsUnbind\|TestAdsCatalogFeed' ./internal/ads` | PASS=2 FAIL=0 exit 0 |
| `bash scripts/dev/test-focused.sh 'TestAds' ./internal/ads` (regression incl. core connect/bind + drafts) | PASS=5 FAIL=0 exit 0 |
| `go test ./internal/httpapi` (full package, DB-free) | ok exit 0 |
| `bash scripts/dev/test-focused.sh 'TestR2IntegrationUpgradeFromReleaseHead\|TestMetaAdsMA02Schema'` | PASS=2 FAIL=0 exit 0 (upgrade≡fresh catalog incl. 0160 objects) |
| `bash scripts/dev/check-gates.sh` | `check-gates: ok (71 modes)`, `check-headers: OK` |
| `go vet ./...` ; `gofmt -l internal cmd tests` | clean, exit 0 |

New tests: `TestAdsUnbindRealPG` (feed scoping/permission, in-flight 409 list + nothing destroyed, counting-draft PT409 atomicity, detach + token destruction + history kept + `binding_disabled` on new plans, no-op replay, cross-store no-op, permission matrix incl. ads:manage-without-integration:manage, AD422 input shape) and `TestAdsUnbindServiceRealPG` (service 409 details + no receipt on refusal, receipt/replay/no-op/audit, re-bind through the frozen connect flow with connections=2, `Service.CatalogFeed` + WithScope refusal), plus 11 DB-free transport cases in `TestAdsTransportRules`.

## CI gates for the integrator (RAM-heavy, run on GitHub not the dev Mac)

- Full `tests/foundation` suite (incl. complete MA01–MA12, ACL/policy pins, G07).
- **Full G07 is mandatory for this unit** (PROCESS §2.6: it touches `migrations/`).
- `go test -race ./...` and the full DB integration lane.

## Integrator to-do (merge-time, integrator-only files)

- `contracts/ads-openapi.json`: add `POST /v1/admin/stores/{store_id}/ads/meta/unbind` (200 `{unbound,ad_account_id,binding_ids,already_unbound}`; 409 `operations_in_flight` with `details.operations[]≤50/operations_total` and 409 `binding_in_use`; Idempotency-Key required) and `GET …/ads/catalog-feed` (200 `{feed_url|null,domains[],path}`).
- Admin BFF/UI unit (Codex, later): unbind button with the R2-ADS-PAUSE-1 copy («先暂停、后断开») on `binding_in_use`, the in-flight list on `operations_in_flight`, and the feed-URL copy/paste card; note the merchant must revoke the app grant in their own Facebook settings (no Meta-side revocation here).

## Risks / known limits (documented, not blocking)

- **No Meta-side revocation:** unbind destroys the local sealed token copy only; the grant stays revocable by the merchant in Facebook settings (amendment §A non-goal; UI copy belongs to the UI unit).
- **Feed serving caveat:** `catalog_feed_url` lists ACTIVE domains but cannot check `valid_until` (column absent from the 0074 GAP-2 grant, deliberately not added); actual fetch-time resolution stays governed by `buyer.resolve_published_store` (amendment §B).
- `meta_dataset` bindings and CAPI untouched by design (CAPI kill switch remains `PUT capi {enabled:false}`); Page/IG disconnect stays 0108.
- Multiple enabled same-asset bindings (normally impossible via `bindOne` reuse) are all locked/destroyed/CAS-disabled in one call; a version race answers 409 `conflict`.

## NOT_RUN / BLOCKED

- NOT_RUN (local, by constraint): full foundation suite, `go test -race ./...`, G07 full, browser/UI tests (no UI in this unit), any Meta LIVE/SANDBOX call (never in scope).
- BLOCKED: none.

## Cleanup

- No long-lived processes started by this unit remain (test-focused.sh manages its own containers/locks); fixture rows live only in the throwaway test databases created by the harness. No ports held, no shared caches touched, no other task directories modified.
