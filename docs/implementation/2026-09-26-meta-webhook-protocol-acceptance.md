# T07 Meta inbound protocol — bounded local acceptance

2026-09-26. **PASS_LOCAL_PROTOCOL_NOT_MOUNTED**, not complete T07, provider
qualification, durable delivery, G06/G07/G14 or full SaaS acceptance.

## Provenance and scope

- Contract: [meta-webhook-protocol-v1](../../contracts/meta-webhook-protocol-v1.md),
  frozen at `5e4a462`, clarified at `e654147`, lossless Unicode requirement at
  `7a74bf3`. [Reuse audit](../discovery/meta/2026-09-26-inbound-reuse.md)
  records the old receiver limits and trusted-route requirement.
- Implementation worker: `integration_worker`, `gpt-6-sol` / `high`, isolated
  `/Volumes/data/worktrees/commerce-meta-protocol-20260926`, base `5e4a462`.
  Source `72f2b76` + fix `89acbbb`, integrated as `263205f` + `56fb141`.
  Owned `internal/integrations/meta/**` and its author report only.
- Independent `test_worker`: isolated
  `/Volumes/data/worktrees/commerce-meta-protocol-tests-20260926`, same base;
  inherited parent model/reasoning without override (runtime identifiers not
  exposed in its report). Test commits `7a93e22` + `b9743a8`, integrated as
  `4084f88` + `62f0f46`; only acceptance tests and its author report.
  [Independent delivery](2026-09-26-meta-protocol-test-author.md) has 11 top-level
  groups / 60 named subtests; test memory `25db4ebc-2cc8-4343-8b82-aba42dc6f491`.
- Read-only security reviewer independently confirmed source `89acbbb` closes
  the Unicode P2 with no open P0/P1/P2 in this slice; memory
  `5acd02d4-2c57-44d1-88c0-58e81e2036b5`. Root reviewed final test diff and reran
  gates on main `62f0f46`; no production calls or customer data were used.

## Root acceptance gates

Runtime: Go `go1.27.1 darwin/arm64`. Commands run at repo root.

| Command / check | Actual result | Stable local evidence |
| --- | --- | --- |
| `GOTOOLCHAIN=go1.27.1 go test -race -count=1 -v ./internal/integrations/... ./tests/integrations/meta` | Exit 0; 52 top-level PASS, 0 FAIL, 0 SKIP | `/Volumes/data/output/meta-webhook-root-final-1.log` |
| `GOTOOLCHAIN=go1.27.1 go vet ./...` | Exit 0; no findings | `/Volumes/data/output/meta-webhook-root-vet-final-1.log` (empty) |
| `git diff --check` | Exit 0 including acceptance documentation | No whitespace errors |
| `scripts/check_packet.py` imported `main()` without its result-file writer | Exit 0; `PASS_PACKET_STRUCTURE_ONLY` | 23 tasks / 15 product gates; structural check, not product acceptance |
| Runtime import scan in `cmd/` and `internal/` | No consumers of `livecommerce/internal/integrations/meta` | Confirms this package remains unmounted; not a live endpoint test |

Race log SHA-256:
`a98c85654dd26ffa454b3b953bfab0ec1301060d59648a7cee4f2b9e6e294ff8`.

MWP01–05 cover exact raw-byte HMAC, secret/config isolation and redaction,
decoded duplicate keys, invalid Unicode, exact depth/body/emitted-unit bounds,
all entry siblings including quarantine, valid Page/Instagram units, message
MID identity versus payload changes, comment transitions, timestamp evidence,
payload ownership, transport rejection, and synchronous commit gating.
Both a response-writer spy and a real `httptest.Server` blocked-callback test
prove no success is written before the callback returns; callback errors and
cancellation fail closed. The callback is synthetic: these tests do not prove
that any database implementation actually commits.

## Actual defect caught, then closed

`encoding/json` replaces unpaired escaped UTF-16 surrogate values with U+FFFD
even when the raw body is valid UTF-8. Distinct signed messages could therefore
produce the same canonical PayloadHash. On original source `72f2b76`, independent
lone-high, lone-low and two-high tests failed (exit 1); the handler returned 200
and invoked the callback three times. A bounded linear scan now rejects these
before decoding, preserving valid pairs, replacement characters and literal
backslash-u text. The same focused test passed after `89acbbb` (exit 0).

| Evidence file under `/Volumes/data/output/` | SHA-256 |
| --- | --- |
| `meta-webhook-utf16-red-handler-72f2b76.log` | `491654e74f75ca1dd91e8705335d9a6fb0ca7c71fe912ec1d8ef15deec9afe0b` |
| `meta-webhook-independent-utf16-green-1.log` | `d8098feebfb0d9c3b15e659810fd0d9bc2197412dd4de99c071c52814a7d5bf7` |
| `meta-webhook-independent-final-1.log` | `4191830742bff560168c318b99f50c8ad1d9972774559281b206b757b96e958b` |

## Remaining release boundary

**NOT_RUN:** actual PG/River atomic inbox/receipt/job writes and ACK-loss recovery;
permanent dedupe and conflict quarantine; trusted app/object/asset routing;
encrypted payload retention, DB-role/RLS and concurrency gates; runtime route
mounting, quotas/operations; real Meta signatures, OAuth, asset authorization,
subscriptions, App Review and comment/message/live-stream behavior.

Existing merchant-submitted `integration.bindings` do not prove Meta ownership.
Do not route private content by matching `entry.id` to those rows. Next delivery
needs a trusted server-verified route registry plus scoped encrypted storage and
restricted quarantine; no tenant guessing, cross-channel identity/consent merge,
or sending from the receiving HTTP request. No customer broadcasts, credentials,
merchant switches or production processes were changed. No new frontend exists
in this increment, so no browser gate is claimed.
