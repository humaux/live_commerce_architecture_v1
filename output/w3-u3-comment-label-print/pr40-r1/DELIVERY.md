<!-- Purpose: factual PR40 round-one four-thread fix handoff, red/green evidence and execution blockers.
Depends on: base6c67c217, CommentLabelPrint and existing Node/browser harnesses.
Used by: integrator to rerun unrestricted gates, commit with Implemented-by and leave review threads under integrator control. -->
# PR40 round1 — BLOCKED_ENV, not READY

- Worktree/branch: .worktrees/w3-u3-comment-label-print / unit/w3-u3-comment-label-print.
- Base/HEAD remains **6c67c217e73322c38eb3502374a6db8fa71a6fc9**; no new commit exists.
- Author Codex-1 (runtime model/effort unavailable); read-only explorers pr40_test_plan and pr40_strictmode_probe, gpt-6.1-sol/medium, E1 only.
- Exact coordination task: 24d9636d-ca88-434b-83d3-96704794aea5, codex-w3-u3.

## Four assigned threads

1. **4237009798**: printable uses ACCEPTED status, then the existing keyword/positive quantity checks.
   REJECTED controls/labels are absent; a mark refresh withdraws a previously previewed label, and a retained old print handler cannot emit A3.
2. **4237009801**: close calls native dialog.close() before clearing preview/focusing the opener.
   That synchronously removes the modal's inert boundary. No delayed timer/RAF or shared shell change is needed.
   Both entry buttons pass their event.currentTarget; focus does not depend on whether clicking made the button active.
   Cover close button, Escape and explicit close after printing. Printing itself still keeps the preview open,
   preserving existing paper/retry behavior and frozen browser assertions; no new auto-close policy.
3. **4237013589**: every mount-effect setup creates a fresh controller; its cleanup captures and aborts only that controller.
   Actual-source effect ownership tests cover setup→cleanup→setup and a cleanup that must not abort a later setup.
   A new browser case mounts the actual component with installed development React/DOM19.3.0 under root StrictMode,
   verifies two setups/one cleanup, then uses real clicks and a controlled MOCK transport acknowledgement to assert one
   write/print and released buttons. This real browser case is authored/typechecked but **NOT_RUN**.
4. **4237013594**: afterEach persists the independently observed ledger first, then asserts that no row is FAIL.
   The actual A4 criterion and actual teardown run in an isolated Node VM: a two-column observation rejects only
   after its FAIL evidence is saved; the normal three-column observation stays green. The native CSS fault injection
   is separately available with W3U3_A4_CALIBRATION=two-columns and is **NOT_RUN**, not substituted by the Node proof.

Changed product file: apps/admin/components/CommentLabelPrint.tsx only.
Tests: comment-label-print.test.ts adds production-handler/effect/ledger regressions with a complete typed synthetic DTO;
the local native-dialog stub refuses outside focus while modal, and close is a no-op if already closed.
No shared hook-host changes. Existing denial/recorder tests retain their intent.
comment-label-print.spec.ts adds real native focus/Escape assertions and independently observed ledger fields
to existing zh-TW/zh-CN/en ×1586/390 cases; no old assertion, threshold or timeout removed/relaxed.
comment-label-strict-fixture.ts builds test-only in-memory assets from actual installed development React/DOM/scheduler
and transpiles the current production component. No fake React hook runtime in that browser fixture; only transport,
format display and CSS import are mocked. Its route is Playwright route.fulfill, not a product endpoint.
No Go/SQL, product route, DTO, dependency or lockfile change. The existing spec is already registered in the labels
part of --browser-live-console; it now has ten cases (nine existing plus the MOCK_REACT_DEV case).

## Commands and observed exits

| Command | Exit | Evidence |
|---|---:|---|
| node --test --experimental-strip-types tests/admin/comment-label-print.test.ts, unchanged product baseline |1|red-final.log:11PASS/6FAIL; additional trigger-capture red in red-trigger.log |
| Same command, old mount effect with new tests |1|red-effect.log:20PASS/2FAIL (fresh-controller/cleanup ownership) |
| Same command, ledger without enforcement |1|red-ledger.log:18PASS/1FAIL (two-column FAIL saved but not rejected) |
| Same command, current four-fix source |0|green-addendum.log:22/22,0fail/skip/cancel; local hook-host/VM MODEL_ONLY, not real browser |
| pnpm --filter admin exec tsc --noEmit |0|tsc-addendum.log |
| Supplemental strict typecheck of browser spec + Strict fixture |0|spec-tsc-addendum.log; exact command below |
| Actual generated Strict fixture parsed by native node:vm Script, without the hook Host |0|strict-fixture-parse.log:1290210-byte generated script; syntax only, runtime NOT_RUN |
| bash scripts/dev/check-gates.sh |0|check-gates-addendum.log:82 documented modes,1263 inventory,headers OK; untracked helper header authored but not staged |
| bash scripts/dev/test-node.sh |1|test-node-addendum.log: first glob455 tests,452PASS/3FAIL; full script stops, later suites NOT_RUN |
| Impeccable detector on the earlier two-fix component |0|detector-final.json=[]; historical source check, not current four-fix/visual/browser acceptance |
| git diff --check |0|No whitespace error in current source diff |
| git add -- the four source/test files |128|Git index.lock creation denied by sandbox; staging/commit cannot proceed |

Supplemental typecheck (from this worktree):
```sh
pnpm --filter admin exec tsc --ignoreConfig --noEmit --strict --target ES2023 \
  --module ESNext --moduleResolution Bundler --esModuleInterop --types node \
  --typeRoots ./node_modules/@types ../../tests/admin/comment-label-print.spec.ts \
  ../../tests/admin/comment-label-strict-fixture.ts
```
The first attempt without --ignoreConfig returned TS5112 (explicit input files conflict with ambient tsconfig).
The corrected command above performs strict checking rather than skipping source checks.

The full Node failures are environment refusals, not dismissed test results:
- existing tests/admin/team-bff.test.ts: listen EPERM on127.0.0.1;
- two existing tests/deploy/deploy-prep-r3.test.mjs cases: Docker socket permission denied.
No test changed/skipped/retried to hide these failures. Target22-case Node suite ran separately and passed.
Final red replay temporarily used the exact HEAD version of this product file, then restored the exact fixed bytes;
current source hashes below bind the restored fix. No server or PG run was started.

Git refused /Volumes/data/live_commerce_architecture_v1/.git/worktrees/w3-u3-comment-label-print/index.lock
with Operation not permitted. This environment marks common Git metadata read-only; no bypass/copy of .git,
alternative branch, forced push or fabricated SHA was used. Commit itself is NOT_RUN because staging failed.
Intended trailer: **Implemented-by: Codex**. No push; no thread resolution.

## CI gates / remaining work

**--browser-live-console: NOT_RUN.** The current preamble requires heavy browser modes on GitHub.
Oct10's permission was explicitly for that round only; it is not reused for this new round.
The label component is reused by the console, so the affected original full mode is required after integration:
LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console (no grep/timeouts changed).
This must exercise all prior REAL_PG cases plus the new real development StrictMode MOCK component mount.
The actual two-column A4 calibration must also run, then the same normal mode must return green:
```sh
W3U3_A4_CALIBRATION=two-columns LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console
LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console
```
Calibration expectation: exit1 specifically from the paper-selector FAIL observation/afterEach guard in the six
locale/width cases, not another error; normal expectation exit0. Product print.css is never changed.
The flag is test-only and outside the reserved LC_/COMMERCE_ namespace so the existing browser environment filter
can pass it without any Go harness modification. It carries no authority, credentials or product configuration.
Rerun the full test-node command in an environment allowing loopback/Docker, and commit on this same branch
once Git metadata is writable. Required PR CI/independent review and hardware/LIVE printing are NOT_RUN.

All four assigned changes are present, but this is not a READY/all-gates-green handoff. Real StrictMode mounting,
native CSS calibration, full browser acceptance, full Node, commit, PR CI and independent runtime review remain
NOT_RUN or BLOCKED as stated. No authentication/configuration was mutated.

## Current source SHA256

- CommentLabelPrint.tsx: ec7d225c86df01d1a800a68757e5e1eda2764bf99dd426aadc53170caf80a126
- comment-label-print.test.ts: 2470418a81b9a40308a339ce2e835a022b7e877d41b2fadec41cfeefb546655b
- comment-label-print.spec.ts: daf089ec93a495f25697230778add4d299f66efb90babf4e483004e390cb9e24
- comment-label-strict-fixture.ts: 5da974b95e514b6f9853bbc4879e5001665aa9593a1b7467a933998bf1495f62

Foreign output/ext-agents/ is untouched. Old acceptance artifacts are historical, not current browser proof.
Uncommitted source and all red/failure evidence remain in this assigned worktree for continuation; fix.patch includes
the tracked changes and the new untracked fixture. No shared host/shell/BFF/Go/SQL code was changed.
