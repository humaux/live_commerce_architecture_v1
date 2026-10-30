# RVFIX5 #294 — local ACK plus an older unresolved staff send

Tests-only RED stage; implementation has not changed. Author E0; local syntax/whitespace E1 only. New causal RED, GREEN and independent acceptance PENDING. No local tests, servers, browser, PG, containers or LIVE.

## Authority and ownership

Integrator's current RVFIX instruction and designated `cxf-rv-294.log` final REQUEST_CHANGES at `b9cc7e0c350f7353b1cfcbe211e28cd7987114e9`: P1 local ACK A completion bypasses observing older queued/blocked B below newer sent C; B seen while A is pending can disappear before A completes. Read `AGENTS.md`, architecture §§0/1/2/26/27, live-console-v1 §3.6 and current InboxThread/load/send plus direct consumers. Fetch/merge origin/r3/integration exit0, already current at `17328372dea91220050e9583259312e2db2e5d97`. Main checkout read-only. Assigned branch/worktree retained, no delegation or duplicate claim. Role: dispatched UI worker; model/effort exact runtime identifier UNKNOWN.

Rollback reference for this additive test batch: `b9cc7e0c350f7353b1cfcbe211e28cd7987114e9`. No destructive action, owned process or downloaded fixture. Pre-existing dirty rvfix3 DELIVERY and untracked handoffs are preserved and excluded from this commit.

## Causal counterexamples and gate

24 added Node cases = three locales × queued/blocked B × B appears with A completion / B observed during pending A then omitted × own SUCCEEDED/FAILED_FINAL. Start with actual A12 ACK, not a projection-only mount. At each refresh assert folded composer, disabled Send, localized delivery result, captured/current callback refusal and exactly one POST. Omission/reappearance preserves B. Explicit matching B settlement alone permits the next draft; rejected A still restores A's original text.

12 added native-click cases = three locales × queued/blocked B × both observation timings, using qualified signed synthetic principal. Fill/click Send, native thread Refresh, observe own success plus older B below newer sent C, omit/reintroduce B, reload/reopen, then explicitly settle B and fill. Assert visible result, hidden composer, disabled controls and one POST. Transport changes prepare facts only; no DOM writes, forced clicks, dispatchEvent or direct API substitute. Existing fixture defaults remain unchanged. Add strict discovery for all 33 V2 scenarios; all 21 prior V2 and 13 legacy scenarios remain required.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Thread composer|fill / Send|Actual queued A ACK; one POST|CI PENDING|
|Thread Refresh|click while B is visible / omitted / reappears|Lock retained beneath sent C, also after A succeeds|CI PENDING|
|Inbox / conversation|reload / click row|Visible persisted B still locks sends|CI PENDING|
|Thread Refresh / draft|click explicit B settlement / fill|Next composer enabled, no resend|CI PENDING|

CI must first reach these new behavioral assertions for causal RED, not fail collection/harness. Record run/job IDs in red.log and raw filtered failed logs after integrator relay. Only then push one root implementation batch. GREEN must be that implementation head's own Foundation/Gates and independent review. No baseline/exemption/threshold/retry/timeout weakening; no next-startup or gate-adapter changes.

## BOURDIEU

Q1 novice/expert means recent experience with reply/status tasks, never class/device. Q2 preserve choose buyer → reply/check status. Q3 a completed staff reply cannot erase another pending obligation. Q4 identity/delivery comes from server evidence. Q5 identical three-locale safety and seller copy. Q6 aesthetics separate; no new verdict or visual change.

## PROJECT UNDERSTANDING

Architecture §§0/1/2 distinguish MOCK from LIVE and keep authenticated server facts authoritative; §§26/27 require isolated ownership and independent acceptance. Live-console-v1 §3.6 explicitly permits concurrent staff sends, so A succeeding does not settle B. Existing load/send guards and A9 pagination are the implementation evidence for this regression. Existing operation detail identity and original rejected text must survive repair. No new setting, copy, API, permission or backend change.

## MOBBIN → EAGLE

No new visual pattern, asset or reference required by this state-safety repair. Existing design handoff provenance remains in `../DELIVERY.md`; no fresh Mobbin/Eagle read or import claimed. Needed new query: none.

## FIGMA OFFICIAL GUIDANCE

No canvas/layout/component/variable/prototype write or design hand-in. Existing design guide provenance remains in `../DELIVERY.md`; no fresh guide read or design verification claimed. Figma copy lint, three-column new-copy table and new desktop/mobile fact audit are not applicable to this tests-only behavior regression; no seller string changed.

## Scope and handoff

In-card tests: inbox-v2-ui.spec.ts and inbox-discovery.test.ts. Outside initial card paths: `tests/admin/inbox-unknown.test.ts` (same-module direct consumer, scope pre-approved; integrator disposition) and generated `output/ci-select-backend-browser/browser-ownership.json`. Regenerate ownership last with the mandated script. No source import/dependency/Go graph change, so no depmap change. No migration/contract/OpenAPI/pin/lockfile edit. Prior cumulative exceptions remain in the PR body.

Humaux search/get, rejected enumeration (3/9 representative coverage), repo map and development details read. Plan stored under `[decision] PR294 RVFIX5 tests-only causal ACK-plus-older-obligation RED`; own child canvas updated under acquired lock, parent untouched. Code index/link will use actual changed file contents. P2 timestamp ordering remains existing dedicated 'P2 for issue' comment 6489297640; not duplicated or repaired here.

Push once, register native workflow run IDs, hand off within ten minutes. Integrator completion relay is the wakeup; no silent local polling process. STOP for unrelated gates or after two targeted attempts on the same blocker. No merge/automerge/status/deploy. Final receipts belong in a PR comment, no evidence-only GREEN commit.
