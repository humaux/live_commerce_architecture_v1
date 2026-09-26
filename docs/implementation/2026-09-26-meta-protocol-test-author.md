# T07 Meta webhook protocol v1 — independent test delivery

- Role: `test_worker`. Model and reasoning were inherited from the parent task with no child override; this child runtime did not expose their concrete identifiers.
- Base: `5e4a46212630bb7bd5a4fa3e80240d8fde85371b`; branch: `commerce/meta-protocol-tests-20260926`; worktree: `/Volumes/data/worktrees/commerce-meta-protocol-tests-20260926`.
- Owned writes only: `tests/integrations/meta/protocol_test.go` and this report. The implementation commits `72f2b7676c19f084614afe9190db83cc6c213c35` and `89acbbbf5fd71dc5c239dadcc3474e56e3d7a893` were cherry-picked locally solely to run the tests; they are **not** this worker's test deliverables. The frozen contract at `e654147` was read, not edited here.
- The independent suite has 11 top-level MWP01–MWP05 tests and 60 named subtests. It uses Go standard-library HMAC, `httptest`, and synthetic IDs/secrets only. It covers exact raw-byte signature and hash; unsafe config, zero verifier and redaction; decoded duplicate JSON keys, surrogate escapes, UTF-8, depth, body/event limits; every known/unknown sibling and linear quarantine payload; Page/Instagram admission and invalid timestamps; stable message MID Key versus changed PayloadHash, comment transition identity and owned payload bytes; GET/POST transport; writer-spy and real HTTP blocked-callback ACK barriers, callback failure/cancellation.

## Red-to-green evidence

- Against original source `72f2b76` (local cherry-pick `6082cff`), the new isolated-surrogate test failed as intended: `go test -race -count=1 -run '^TestMWP02StrictJSONAndPrecision$' -v ./tests/integrations/meta`, exit **1**. Lone high, lone low and two-high escapes were admitted by `Verifier`, returned HTTP 200 through `NewHandler`, and invoked the callback three times. Log: `/private/tmp/meta-protocol-tests-Qx61Fh/utf16-red-handler-72f2b76.log` (SHA-256 `491654e74f75ca1dd91e8705335d9a6fb0ca7c71fe912ec1d8ef15deec9afe0b`). Root also copied this log to `/Volumes/data/output/meta-webhook-utf16-red-handler-72f2b76.log`.
- After source fix `89acbbb` (local cherry-pick `55a18bb`), the **same** focused command exited **0**. Valid surrogate pair, literal/escaped replacement character and literal backslash-u controls remained accepted. Log: `/private/tmp/meta-protocol-tests-Qx61Fh/utf16-green-89acbbb.log` (SHA-256 `d8098feebfb0d9c3b15e659810fd0d9bc2197412dd4de99c071c52814a7d5bf7`).

## Final local gates

- `go test -race -count=1 -v ./internal/integrations/meta ./tests/integrations/meta` — exit **0**; 16 top-level tests across both packages, including this worker's 11 groups / 60 named subtests. Log: `/private/tmp/meta-protocol-tests-Qx61Fh/race-post-utf16-verbose.log` (SHA-256 `4191830742bff560168c318b99f50c8ad1d9972774559281b206b757b96e958b`).
- `go vet ./internal/integrations/meta ./tests/integrations/meta` — exit **0**. Log: `/private/tmp/meta-protocol-tests-Qx61Fh/vet-post-utf16.log` (empty; SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`).
- `gofmt` applied to the owned Go test file; `git diff --check` must remain clean at final handoff. Root independently reruns and reviews before integration.

These are local Go protocol tests using synthetic events, not provider or durability acceptance. **NOT_RUN:** PostgreSQL/River atomic receipts/jobs, persistent uniqueness and same-MID conflict quarantine, encryption/tenant asset routing, public route/rate limits, Meta signed sample, OAuth/subscriptions, live comment/message behavior, and full T07 deployment acceptance.
