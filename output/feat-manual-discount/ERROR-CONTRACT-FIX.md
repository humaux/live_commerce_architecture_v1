# FEAT-MANUAL-DISCOUNT error-contract fix 1/2

task_id: `taskfeff99ce-3a23-47aa-bc52-2d8d7640987b` (root-owned).
base_commit: `2c960d7b8024a62cdf8473ff0f30367a43f792b4`.
Worktree: `.worktrees/codex-pr394-offline-diagnostic`.
Branch: `codex/pr394-offline-diagnostic`.
Role: bounded product author; assigned Sol/high; actual model/effort telemetry UNKNOWN.
Date: 2026-11-02 UTC. Local static work only; no delegated agents.

## Result and boundary

Implementation prepared; Go compilation, unit execution and real-PG GREEN are **NOT_RUN** under this card's no-runtime restriction. This is not product acceptance or merge approval. Root owns independent review, pre-push checks, push and CI registration. No server, database, browser, provider, installation or background process was started.

Changed paths:

- `internal/merchanttools/discount.go`
- `internal/merchanttools/manual.go`
- `internal/merchanttools/manual_discount_test.go`
- `output/feat-manual-discount/ERROR-CONTRACT-FIX.md`

The new discount SQL boundary normalizes actual `*pgconn.PgError` PT401/PT403/PT404 into existing platform sentinels and PT409 into typed 409 `conflict`. Mapping happens only after `platform.WithScope` returns, preserving its fresh, opaque store-visibility recheck. Canceled/deadline contexts and errors, unknown SQLSTATEs, existing typed merchant/pricing errors and genuine uncertainty retain the original error; the existing pipeline classifier still controls the public envelope and driver privacy. No global `command.ErrConflict` mapping or receipt error behavior changed.

The existing normalized `ManualInput` authorization/hash comparison now precedes `SetCart`. A same-key request with changed discount plus changed items reaches full-request `idempotency_conflict` before the cart's narrower replay hash can replace that code with ordinary `conflict`. Static validation, option/payment/delivery checks still precede authorization; permission checks, quote/binding, version checks and final receipt code remain unchanged. Earlier authorization can now remain durable if a subsequent cart command fails; this uses the existing immutable partial-pipeline authorization/resume contract, without creating an order or hold.

Small boundary unit cases with no new dependency were added in the existing test file: all four closed codes including wrapped PgError and privacy, unknown codes, wrapped typed errors, ordinary cart conflict, scope opacity, cancellation/deadline and transport uncertainty. These are not a fake database or replacements for real-PG tests. The existing validation test body and entire foundation manual-discount test file remain byte-exact against base.

## Authority and existing RED

Source/contracts read at base: AGENTS.md; architecture sections 0/1/2/26/27; `contracts/tasks.json` T21 (T18/T19/T20 dependencies, G01/G13/G14); `contracts/invariants.json` including I01/I02/I03/I04/I05/I06/I08; `contracts/storefront-v2.md:649/651`; `contracts/cart-quote-v1.md:192/207`; current discount/pipeline/direct HTTP classifier and `platform.WithScope` code. Humaux development detail `7bdb220b-0798-4b1d-aff5-16ee94cae75f` was read. The code map was consulted as navigation only; current file bytes supersede stale map line numbers.

Independent source adjudication: root evidence `output/integrator/tools/logs/ci-evidence-1238/394-a-findings-source-1551.md` sections 1/2; its conclusions were checked against current source and original RED.

Existing real-PG RED: root evidence `output/integrator/tools/logs/ci-evidence-1238/394-2c960-g05-1712.raw.log`, SHA256 `2ed43d1bcc138d3b8a5287fee45086a043a47af1dc9eea7365515924c5f1b485`, job `125416742913`, tested checkout `7689e99e359114c594c17b33188549f9c384c8ec` (parents c6be + base 2c960). Lines 1930–1947 show both real product failures after durable partial-state, refusal-state, original-body replay and fresh-key HTTP201 controls:

- Different legitimate principal, identical body/key: target runtime SQL PT409 and rollback verified, but HTTP503 `retry_later` instead of 409 `conflict`.
- Same principal/key, changed discount plus items: HTTP409 `conflict` instead of 409 `idempotency_conflict`.

This author did not rerun RED and does not claim GREEN. Required acceptance remains these unchanged registered tests plus existing project gates and independent review at the delivered commit. Original permission matrices are not reopened or weakened.

## Actual static commands and receipts

All commands below ran in the assigned worktree; no Go runtime/test command ran.

| Command | Exit | Result |
| --- | --- | --- |
| `git status --short` and `git rev-parse HEAD` on entry | 0 | Clean, exact base above |
| `gofmt -w internal/merchanttools/discount.go internal/merchanttools/manual.go internal/merchanttools/manual_discount_test.go` | 0 | Formatting only |
| `gofmt -l internal/merchanttools/discount.go internal/merchanttools/manual.go internal/merchanttools/manual_discount_test.go` | 0 | Empty output |
| `git diff --check` | 0 | No whitespace errors |
| Inline `python3` using pathlib/hashlib and `git diff --name-only` / `git show BASE:path` | 0 | Changed paths within allowlist; foundation bytes equal base; original validation test body equal base |
| `shasum -a 256 tests/foundation/manual_discount_test.go ROOT_E/394-2c960-g05-1712.raw.log` | 0 | Hashes below and above |

An exploratory combined `rg`/source-read command exited 1 because zsh rejected the unmatched read-only glob `contracts/module*`; no files were changed by that command. No gate was interpreted as passing from it. The task's code and contracts were read directly.

Static integrity output (before report creation):

```text
foundation exact baseline bytes 66747 sha256 289348f1b1b042717cc2ac33120434b1955c80eb5849c2d19fcacfbb05163da9
existing validation test body byte-exact
3b43e5297aabcc84993d06e231c168fb5223636053f4262c248372ab1892bcc4 internal/merchanttools/discount.go
8da32638d2964a4f04f0473a2a7b0439db708d1e939fd622a9934fd8cc47336f internal/merchanttools/manual.go
9538f45ba3f064cdc9ba93897f3300bd05cc9a337cfd1a0edf169c8bb5eec8ee internal/merchanttools/manual_discount_test.go
```

## Remaining work and risks

- NOT_RUN: Go compilation/unit/race, real-PG product GREEN, required CI, independent review, provider SANDBOX/LIVE, deployment. Product acceptance remains pending.
- Existing separate HOLD items (legacy empty-bundle For/Send request hashing and reader concurrent revocation ordering) were not modified or bundled into this two-defect fix.
- No migration, grant, schema, DTO, dependency, foundation test, fixture or gate changed. Inventory/payment/idempotency invariants are retained.
- Rollback anchor is the base commit above; this local commit contains no migration or data change. Do not discard other agents' work.
- No task-created fixture/container/port/process/temp directory requires cleanup. Evidence is intentionally retained. Final commit ID is supplied in the parent handoff; source hashes above bind this report without a self-referential commit hash.
