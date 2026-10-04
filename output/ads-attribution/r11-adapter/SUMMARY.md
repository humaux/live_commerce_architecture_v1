# R11 Meta adapter handoff

- task_id: 9d636ee3-f29a-43b1-951a-11037b6c2068
- base_commit: 8f419664f5a2c82085daadce68ab41e31851696b
- branch: unit/ads-attribution-r11-adapter
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r11-adapter
- role: integration implementation; no recursive delegation
- actual model/reasoning: UNKNOWN (runtime does not expose a verified model/settings identifier to this worker)
- evidence class: MOCK unit tests, including race detection; no external Meta calls

## Changed paths

internal/integrations/meta_ads/{breakdowns.go,ops.go,breakdowns_test.go,r11_breakdowns_test.go};
internal/metaconnect/{graph.go,connect_test.go,r11_scope_test.go};
output/ads-attribution/r11-adapter/.

## Implementation

Each optional breakdown dimension is accepted atomically. A refused, uncertain, malformed,
duplicate, unbounded or ambiguous dimension contributes no rows and its enum enters
`unavailable`; successful dimensions remain in the detail. D7's already validated daily
provider reference remains SUCCEEDED even when every optional dimension is unavailable.
Each subsequent scheduled read starts every dimension again; there is no immediate retry.
Pagination continues to use only validated cursors on a pinned path, never paging.next URLs.
All six previously non-null breakdown metrics are now *int64 and serialize to null when
omitted. Explicit zero is preserved. Page-connect keeps its four minimum permissions:
read_insights is optional and, when granted, survives Granted/listPages into the scopes
passed to the SQL pick result. A declined read_insights grant still permits normal Page
connection. The root owns the audience missing-grant/reconnect state.

The root adjudicated that Login for Business's frozen config_id-only OAuth contract must
remain intact. No scope parameter or unused requested-permission list was introduced.
Requesting read_insights requires the owner to change the external Login for Business
configuration. That change is BLOCKED_EXTERNAL, not a locally implemented runtime action.
Source: internal/integrations/meta/oauth/oauth.go:260-272 (Dialog.URL), oauth_test.go:146
(scope empty asserted), internal/metaconnect/service.go:196 (Start returns dialog.URL),
graph.go:128-154 (preserves granted scopes without minimum-only filtering).

The old aggregate-refusal tests were moved to the single-dimension boundary, preserving
their precise failure classification, no-partial-rows assertions and paging bounds. DST
tests retain invalid-hour refusal and now assert daily success plus unavailable hourly.

## Commands and observed exits

All commands ran in this worktree; logs below are relative to this evidence directory.

| Actual command | Exit | Evidence |
|---|---:|---|
| go test ./internal/integrations/meta_ads ./internal/metaconnect -run '^TestR11' -count=1 (baseline before implementation; scope assertion superseded by root optional-capability ruling) | 1, expected RED | red.log |
| go test ./internal/integrations/meta_ads ./internal/metaconnect -run '^TestR11' -count=1 (before optional-scope ruling) | 0 | green-focused.log |
| go test ./internal/integrations/meta_ads ./internal/metaconnect -count=1 (before optional-scope ruling) | 0 | green-unit.log |
| go test -race ./internal/integrations/meta_ads ./internal/metaconnect -count=1 (before optional-scope ruling) | 0 | green-race.log |
| go test ./internal/metaconnect -run '^TestR11PageConnectAllowsDeclinedReadInsights$' -count=1 (RED before restoring minimum-scope eligibility) | 1, expected RED | red-optional-scope.log |
| go test -race ./internal/integrations/meta_ads ./internal/metaconnect -count=1 (final scope ruling + granted/declined readback coverage) | 0 | green-final-race.log |
| go vet ./internal/integrations/meta_ads ./internal/metaconnect | 0 | vet.log (empty) |
| go build ./internal/integrations/meta_ads ./internal/metaconnect | 0 | build.log (empty) |
| git diff --check | 0 | no output |
| gofmt -l internal/integrations/meta_ads/breakdowns.go internal/integrations/meta_ads/breakdowns_test.go internal/integrations/meta_ads/r11_breakdowns_test.go internal/integrations/meta_ads/ops.go internal/metaconnect/connect_test.go internal/metaconnect/r11_scope_test.go internal/metaconnect/graph.go | 0 | no output |

## Unresolved / integration responsibilities

- REAL_PG persistence of unavailable dimensions/nullable values: NOT_RUN by this author;
  root owns migration and integration tests. The detail interface was frozen by root.
- SQL Page-connect minimum permissions and legacy foundation fake grants intentionally
  remain valid. Optional audience-capability status is handled by root's SQL/UI work.
- Independent review and independent test rerun: NOT_RUN by this author.
- SANDBOX/LIVE and real Login for Business configuration/App Review: BLOCKED_EXTERNAL;
  no production credentials requested or used, no remote calls made.
- No deploy, push, DB/container fixture or background service started. httptest servers
  close automatically via test cleanup. Logs including RED are retained.
