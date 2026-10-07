<!-- Purpose: unit brief for the R3 deploy-preparation unit (code-side only; no production host access). Depends on: deploy/
(compose, env examples, scripts/ops-admin.sh, preflight.sh, smoke.sh, deploy.sh), cmd/* operator CLIs, every R3 unit's
output/<unit>/DELIVERY.md "integrator to-do / deploy" notes. Used by: Codex-4 implementer; integrator release gate. -->
# Unit brief — deploy-prep-r3（上线准备，代码侧）

Owner rule: 「等全部做完后再上线」 — this unit prepares, it NEVER deploys. No SSH, no production env, no live keys, no
pushes; test/MOCK only. The pilot host stays on image 8d222999 until the owner approves the release.

## Scope (re-derive from the DELIVERY.md files; this list is the starting point, not exhaustive)
1. **Operator CLI wiring** in `deploy/scripts/ops-admin.sh` (+ compose service if needed) so every R3 operator command runs
   through the one sanctioned wrapper with its allowlist and secret rules:
   - `platform-admin` (OPS-01B 0143 suspend/resume/audit; OPS-02B 0153 support-grant/revoke/list +
     support-principal-add/revoke) — currently NOT wired ("ops-admin.sh has no platform-admin branch on this base").
   - `stripe-admin settlement-sync|close|export|payout` (W4-S2) — verify allowlist + STRIPE_SANDBOX / LIVE-pair gate and the
     `--out` volume mount for export.
   - `stripe-admin platform-*` (W4-S1 allowlist of stores AD-PF2) — verify.
   Tests: the existing ops-admin allowlist tests (grep tests/foundation and deploy/tools for ops-admin) must cover the new
   subcommands (allowed + refused).
2. **Env / compose / preflight**: every env var the R3 units read is present in `deploy/env/*.example` with a safe default
   and a comment; `deploy/scripts/preflight.sh` refuses unsafe combinations (e.g. LIVE stripe without the LIVE pair,
   PAYUNi notify enabled — PAYUNi is cancelled by the owner). Include `COMMERCE_META_PAGE_APP_ID` and any flag listed in
   output/*/DELIVERY.md "deploy" notes. `bash deploy/scripts/check-pins.sh` and the secrets manifest stay consistent.
3. **Smoke**: extend `deploy/scripts/smoke.sh` (static + full modes) with unauthenticated/denied probes for the new admin
   routes (operations ledger, ads unbind/catalog-feed, returns, keyword simulate, settlements) — 401/404 behaviour only,
   no data writes.
4. **Release order + runbooks**: document in `deploy/README.md` (or docs/runbooks/) the release order migrate → api →
   admin + storefront (product-media-v2 note: old admin/api against the migrated DB fails during the swap), and short
   runbooks: weekly settlement (sync/close/export/payout-record, never a bank call), support grants, failed-operations
   ledger triage, ads unbind.
5. **Dependency map**: run `scripts/dev/gen-deps.sh` and commit the regenerated `docs/architecture/DEPENDENCIES.md`;
   `depmap.sh --check` (if present) must pass.
6. **Release-gate dry run list**: write `output/deploy-prep-r3/RELEASE-GATE-PLAN.md` — which CI modes / release-gate.sh
   groups must be green on the release SHA, which are accepted NOT_RUN (SANDBOX keys, LIVE) and why.

## Gates
`bash scripts/dev/check-gates.sh`, `bash deploy/scripts/check-pins.sh`, `shellcheck` on changed scripts (if installed),
the ops-admin / preflight / smoke-static tests, `go vet ./...`. GitHub: `.github/workflows/deploy-smoke.yml` (static +
full) — list it under CI gates; the integrator runs it.
