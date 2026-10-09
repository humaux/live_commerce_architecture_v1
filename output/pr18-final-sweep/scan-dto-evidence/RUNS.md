# PR18 IG scan DTO runs

- task_id: 088736c6; delegated unit: pr18-ig-scan-dto.
- base_commit: 680aa30477747e32f494ca91d515f97c925d085c.
- branch/worktree: unit/pr18-ig-scan-dto / .worktrees/pr18-ig-scan-dto.
- Scope: internal/live/stream.go, focused Go tests, contracts/live-console-v1.md §2.6 only; own output evidence.
- Role/model/effort: integration implementation / Codex declared GPT-6 identity; exact configured model and inherited effort UNKNOWN.
- Directed Humaux reads: seq approvals; 8/8 rejected enumeration; development flow d937b9b6 read in full. No new claim or parent canvas mutation.
- Red unit: GOTOOLCHAIN=go1.27.2 go test -race -count=1 -run '^TestConsoleStreamPageScanExhaustedWire$' -v ./internal/live.
- Red PG: LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN02ScanExhausted$'.
- Logs: red-unit.log, red-pg.log. Only the PG runner uses a disposable fixture and existing machine heartbeat lock. No browser/full-suite process starts.
- Initial unit RED exit 1: omitted scan_exhausted. Initial PG RED exit 1: all-filtered next.seq stalls at 0 and omitted field; 100-row case hit harness SQLSTATE 53300 because existing ig.webhook creates a new consumer pool on each call.
- Harness correction only: 100-row case reuses one existing real commerce_meta_consumer pool; all 100 signed webhook/consumer writes and assertions retained. Production source remains unchanged; rerun RED in red-pg-actual.log.
- Corrected PG RED exec72935 → exit1 PASS0 FAIL1 SKIP0; all three subcases fail actual field assertions, all-filtered next.seq is incorrectly0, 100→99 and empty-after99 assertions reached. Initial PG exec15485 exit1 preserved with harness-error evidence.
- Unit RED exec58829 exit1. Offline dependencies exec34786 exit0, no lockfile/manifest change.
