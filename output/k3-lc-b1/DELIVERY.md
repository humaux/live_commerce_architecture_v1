# k3-lc-b1 delivery
- Branch/commit: see git log (unit/k3-lc-b1)   Base: r3/integration 1a6a9177   Model: Kimi K3
- Summary: independent adversarial gate for LC-B1 (A7 lifecycle + ≤5 OPEN windows/store).
  `tests/foundation/k3_lc_b1_adversarial_test.go` — 13 test groups: same-session start/end/archive CAS races,
  per-store cap under cross-store and 6th-slot concurrency, idempotency-key reuse with different bodies/sessions,
  stale-version zero-side-effect, end-vs-ingest race, claims after end/archive, tenant isolation (I01/RLS),
  end-without-window, reopen-vs-end loop. 11/13 pass; 2 fail by design = 4 findings (output/k3-lc-b1/REPORT.md):
  F-M2-DRAFT (M2 opens window on draft session, dead window eats a cap slot), F-ARCH-OFFER-UPDATE,
  F-ARCH-OFFER-CREATE, F-ARCH-TIMELINE (archived session still writable). Both were author-disclosed risk areas.
- Contract/interface changes: none (test-only unit).
- Tests: `bash scripts/dev/test-focused.sh 'K3LcB1|LiveLifecycle'` → exit 1 (red: output/k3-lc-b1/red.log;
  second identical run: green.log). 17 PASS incl. all 6 author LiveLifecycle tests; the 2 failing K3 groups ARE
  the findings; ran with `-race -count=1`.
- Gates run: `gofmt -l` clean; `go vet ./tests/foundation/` exit 0; `bash scripts/dev/check-gates.sh` → exit 1
  ONLY at the pre-existing env failure `ui-architecture-gate.mjs: Cannot find module 'typescript-api'`
  (node deps not installed in this fresh worktree; same blocker as unit k3-a7-adversarial; gates.log).
  check-headers ratchet skips `tests/**` (file carries a doc header regardless).
- Evidence class: REAL_PG, MOCK manual ingress. Meta consumer/poller paths: NOT_RUN (LC-B2 scope).
- Risks: the 2 failing groups are intentional (findings for the integrator); do not weaken the assertions —
  flip only with an integrator ruling (fix unit or contract amendment).
- NOT_RUN / BLOCKED: full `go test -race ./...` (r3/integration carries ~26 unrelated pre-existing foundation
  failures per the brief); browser/UI (no UI in LC-B1); `check-gates.sh` tail blocked by missing node module
  (pre-existing env issue).
- Integrator to-do: rule on F-M2-DRAFT (suggest: `applyWindow` requires `lifecycle='live'` for opens) and
  F-ARCH-OFFER-UPDATE/CREATE/TIMELINE (suggest: lifecycle guard in CreateOffer/UpdateOffer/RecordOfferFeatured,
  or a §9 amendment narrowing "read-only"); no migration, schema or privilege changes from this unit.
