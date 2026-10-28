# OC-05 delivery

Scope: server-owned per-store quick-start completion, three derived steps, admin GET/POST, migration 0205 and assigned R2 pin 135. UI OC-06 excluded.

Role: dispatched commerce implementation worker. Runtime identity: Codex / GPT-6; exact model variant and reasoning setting are not exposed to this worker (UNKNOWN). No recursive delegation.
Base: `79f95784c9dee419346fe3119d4d4f56720d4a3c`; branch `unit/oc05-quickstart-marker`; assigned worktree `.worktrees/oc05`.

Allowed writes: `internal/quickstart/**`, `migrations/0205_quickstart_marker.sql`, `tests/quickstart/**`, R2 pin + comment only, merchant wizard route rows + amendment, API mount file, generated dependency map, `output/oc-05/**`.

Authority: OC-05 in `contracts/tasks.json`; AD-03 in `contracts/merchant-settings-wizard-v1.md` and `contracts/owner-choices-20261026-v1.md`; assigned card overrides generic migration placeholder and grants pin 135.

Verification: CI-only. No local tests, servers, database, browser, containers, or model experiments. Local formatting and mandated dependency-map regeneration are explicitly allowed by the card. RED/GREEN receipts pending; no acceptance claim. Author self-assessment is E0.

RED tests use the production admin handler + commerce_runtime role + real PostgreSQL, with explicit NOT_RUN when disposable PG is absent. They cover new-store state, real-data derivation, old-store backfill, idempotent dismissal, cross-tenant/store refusal and strict client-state rejection.

NOT_RUN: independent review; all runtime acceptance until PR CI returns. No production/provider actions.
Rollback: integrator may revert this PR's code; migration is forward-only. Retain the completion table/data; any database rollback requires a separately reviewed migration. No production migration was applied here.
