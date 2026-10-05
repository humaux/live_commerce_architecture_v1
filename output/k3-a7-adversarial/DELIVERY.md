# k3-a7-adversarial delivery
- Branch/commit: see git log (unit/k3-a7-adversarial)   Base: r3/integration   Model: Kimi K3
- Summary: independent adversarial tests for kwc-v1. 160-case corpus
  `tests/claims/kwc-v1-adversarial.json` + runner `tests/claims/kwc_adversarial_test.go`;
  boundary/property/fuzz tests `internal/claims/grammar/contains_adversarial_test.go`.
  146/146 frozen expectations pass; 14 safe-outcome findings fail by design (they document
  bugs; see output/k3-a7-adversarial/REPORT.md): F1 question-table gaps (請問/如何 → order),
  F2 2-char table entries evadable by interposed chars (取 消 → order),
  F3 lookalike-letter carve (А01+2 → keyword "01" ×2, wrong product).
- Contract/interface changes: none (test-only unit; tables frozen — findings would require kwc-v2).
- Tests: `go test ./internal/claims/... ./tests/claims/... -count=1` → exit 1
  (red evidence: output/k3-a7-adversarial/red.log; the 14 failing cases ARE the findings;
  all frozen cases and the entire pre-existing claims suite pass).
  `go test -run='^$' -fuzz='^FuzzParseContainsAdversarial$' -fuzztime=60s ./internal/claims/grammar`
  → exit 0, 2,614,284 execs, corpus 136 (output/k3-a7-adversarial/fuzz.log).
- Gates run: `gofmt -l internal/claims/grammar tests/claims` → clean;
  `go vet ./internal/claims/grammar/ ./tests/claims/` → exit 0;
  `bash scripts/dev/check-gates.sh` → exit 1 but ONLY at pre-existing env failure
  `ui-architecture-gate.mjs: Cannot find module 'typescript-api'` (node deps not installed
  in this fresh worktree; unrelated to this unit — output/k3-a7-adversarial/gates.log).
- Evidence class: UNIT (pure grammar, DB-free). Meta/webhook/REAL_PG ingest: NOT_RUN.
- Risks: the 14 failing safe-outcome assertions are intentional (findings for the
  integrator); if integrator accepts the tables as frozen, these cases should be moved to a
  documented known-gap list or flipped only by a kwc-v2 brief — not silently weakened.
- NOT_RUN / BLOCKED: REAL_PG three-mode ingest of the corpus (KCC02, author unit);
  Meta consumer path (KCC03); repeated-comment dedup (DB-level); `check-headers.sh`
  (permission-blocked in this session; both new Go files carry the required header block);
  `scripts/dev/check-gates.sh` blocked by missing node module (pre-existing env issue).
- Integrator to-do: decide F1/F2/F3 disposition (kwc-v2 table/segmentation brief vs
  documented known gap) BEFORE enabling KEYWORD_QTY_CONTAINS for a real live; no migration,
  schema or privilege changes from this unit.
