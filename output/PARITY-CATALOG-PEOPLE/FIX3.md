# PARITY-CATALOG-PEOPLE FIX3 (PR #363, head 82cbe05a6 review RVFIX)

Independent review (Claude Opus, `output/issue-107/review-363-82cbe05a6.md`) returned REQUEST_CHANGES on head `82cbe05a6`.
Integrator ruling (2026-10-31): fix ONLY the two P1 items below; P2s go to an integrator issue.

## CI Runs and Jobs on Head 82cbe05a6

- Foundation checks: Run `41192921118`, Job `124513221914`
- Gates (GitHub runners): Run `41192921258`
  - Shard unit: Job `124514020454`
  - Shard g09: Job `124514020249`
  - Bundle 02: Job `124514020327`
  - Bundle 03: Job `124514020323`
  - Bundle 05: Job `124514020471`
  - Receipts: Job `124519485495`
  - Required: Job `124519671740`

## Review Findings and Actions

| Item | Finding | Class | Resolution |
|---|---|---|---|
| P1-1 | `internal/customers/read.go:88`: N+1 detail reads in `List` | b | Compute `consent_state` in SQL via `migrations/0215_customer_consent_state.sql`. Delete per-row `loadDetail` in `read.go:82-93`. Decode `consent_state` directly from SQL list row in `decodeCustomer`. Cross-check in `decodeDetail` using `consentState(history)`. Bump R2 upgrade pin in `tests/foundation/r2_integration_upgrade_test.go:134` (141 -> 142). |
| P1-2 | `apps/admin/components/ProductList.tsx:100`: "Product tasks" disclosure has no accessible name | b | Add `aria-label={c.operations}` to `<summary data-testid="products-operations">` while retaining `<span aria-hidden="true">{c.operations}</span>`. Add browser accessible-name assertion in `tests/admin/catalog-core.spec.ts:132` and `tests/admin/catalog-v2-simplified.acceptance.ts:176`. |

## Evidence Retention (ADR-20261031)

No raw CI job logs or downloaded artifacts are committed under `output/`.

### Source File SHA256 Manifest

```
1be6944ca6a8f254cc160e4345252b72141c8423963c4d752d240775b2805b7e  migrations/0215_customer_consent_state.sql
4ab7b372931a3e3588748cd1009d4a069ba729d682e29db507930797e6290889  internal/customers/read.go
ebb5dd47ca8d7d2ddf5f7cbc2e92253333378ed81a7e67144abfea05ac353237  internal/customers/types.go
eac9e54eebb105322e4f6542e096bd67639fc734d9463471f8812d84a1f7314a  internal/customers/types_test.go
77abb14edbf9a1c077a02c75292ba3f6fc586ea17151f39f69f5d97a07984ee5  tests/foundation/r2_integration_upgrade_test.go
c1408345a47a42c7e347d2d9f05d6bdd27f3e2611490d78fee2806435aca4df2  apps/admin/components/ProductList.tsx
4375610923f49aa997b7c6e5911c8c1233a7344780f3289e9f0c08716b8c9c97  tests/admin/catalog-core.spec.ts
9891d007a4c8878bc7d73ee85d8ae8a3222fb4462f1cb0d70c48adc884501b77  tests/admin/catalog-v2-simplified.acceptance.ts
88b2c4c1113beb0e0a0b2183434e7763b8ecdd8268d941ede93313fb4c84983b  output/ci-select-backend-browser/browser-ownership.json
```
