## SETTINGS-V2-IMPL — tests-only RED stage

Adds one batched real-click acceptance helper for the owner-signed simplified settings design. The UI remains unchanged so the first new overview assertion must fail before implementation. The helper shares the existing signed MOCK IdP / isolated-PG session and covers three locales, profile save and refresh, review/back, definite and uncertain failure, three-step setup, product draft save/refresh, explicit dismissal and all five mapped delivery switches. Delivery receipts/faults are explicitly MOCK; profile/product saves use the existing CI backend.

Authority: signed ux-settings DELIVERY.md latest polish/simplify sections; current merchant-settings-wizard-v1 profile/delivery/marker amendment. Base integration e324e01a93219c099d6d2a9d35ed1f112595fe90 (includes #287 and #254). No UI/API/contract implementation in this RED commit. Existing safety assertions remain in place; implementation must reconcile their old interaction path without losing coverage.

Validation: node syntax check, tsc on new spec, bash -n and node --check exit 0 (E1). Static browser-ownership generation ran after integration fast-forward. No local tests, builds or servers. CI RED pending; GREEN/E4 NOT_RUN. See output/settings-v2-impl/PLAN.md and DELIVERY.md. No merge or auto-merge authorization.

Scope/accounting: settings-real.spec.ts imports/calls the helper because the Go fixture independently requires exactly one session/store. scripts/dev/test-local.sh moves profile.go and quickstart into --browser-identity lc_covers; scripts/dev/pr-modes.mjs removes their obsolete BACKEND_ONLY entries. Generated browser-ownership.json is included. These are the card's required CI registration/generated exceptions; no other app/backend/contract/migration/lockfile changes. All paths are declared in PLAN.md. No files outside that plan changed.

RELAY 1: Foundation run 40827493014 / job 123293774850 on head 4d4fdc657186337df39ec337b7d80c84975bcd32 failed at check-gates because settings-v2.spec.ts was absent from the selected suite inventory. This is registration failure, not causal RED. Targeted fix attempt 1 registers both the existing calling spec and its imported helper in the settings-real suite; no tests, assertions or gates are weakened. Browser ownership is regenerated. Additional path outside PLAN.md: playwright.config.ts, limited to settings suite registration and its header invariant, for integrator approval.

The replacement RED workflow run/job IDs and causal assertion failure will be recorded after CI executes. Implementation is intentionally pending causal RED, not product completion. Author judgment E0; production/provider/sandbox deployment acceptance NOT_RUN. Humaux tools unavailable, so local handoff only.

🤖 Generated with Codex (dispatched by Claude Code integrator).
