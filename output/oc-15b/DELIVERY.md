# OC-15b — implementation submitted for CI; GREEN pending

- Role: dispatched Codex implementation worker; no recursive delegation. Runtime model ID/reasoning are not exposed (UNKNOWN).
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/oc15b`; branch `unit/oc15b-billing-reauth`; base `79f95784c9dee419346fe3119d4d4f56720d4a3c`.
- PR: https://github.com/humaux/live_commerce_architecture_v1/pull/252 into `r3/integration`. No merge, auto-merge, force-push or commit-status writes.
- Authority: OC-15b dispatch and current relay; issue #244; contract §16/§16.1; migration 0203; contractdrift.md; scanner source read only.
- Own assessment E0. RED assertion evidence E3, bound to the tests-only head below. Implementation GREEN and independent acceptance PENDING.

## RED (confirmed)

Tests-only head `d55d9282c392f33d0e6b436c36a2a5f78920113b`, Gates run `40612243880`, job `122575742449`, attempt 1. Completed failure. CI checked out synthetic merge `b1934fb5e5deca4f59a766be957466a2c7e0a41b` of that head into base `79f95784c9dee419346fe3119d4d4f56720d4a3c`.

At 2026-10-28T02:01:56Z, `TestReauthRH13GuardedDomainsEndToEnd` failed its own assertion: `billing checkout: HTTP 403 code "forbidden", want 403 "reauth_required"`. The package compiled and reached the HTTP/PG assertion with fake Stripe. Exact filtered raw log: `red.log`; tests-only diff: `tests-only.patch`.

`gh run view --job 122575742449 --log-failed` initially refused logs while the containing run was in progress. `gh api repos/humaux/live_commerce_architecture_v1/actions/jobs/122575742449/logs` succeeded; its checkout and assertion lines are preserved in red.log. That transport limitation was not treated as a test result.

## Implementation

1. `cmd/api/platform_billing.go`: register literal `POST /v1/platform/stripe/webhook` on a stdlib ServeMux. Preserve the nil-webhook fallback identity, exact decoded-path admission, non-POST delegation, and alias fallthrough. For POST selection only, clear RawPath on a request/URL copy so encoded separators retain the old decoded-path behavior. Invoke the selected handler with the ORIGINAL request; signed body, headers, URL, method and request URI are untouched.
2. `internal/billing/webhook.go`: narrow WebhookHandler's return type from http.Handler to the concrete http.HandlerFunc it already returns on every path. This is the scanner's existing explicit-registration leaf-handler form (go_opaque.go); the same webhook function and method rejection execute. No scanner/exemption changes.
3. `internal/billing/errors.go`: PT403 maps via `platform.PT403(pgErr.Message, platform.ErrForbidden)`.
4. Contract §16 mapper list now includes billing instead of the OC-15b pending note.
5. Add `TestMountPlatformBillingPreservesRequestAndDispatch`: 7 methods × 10 paths/query forms, including escapes and aliases, with handler/status/body and original request assertions. Existing billing/webhook tests and RH13 assertions are unchanged. RH13 itself was restored in the tests-only commit.

## Commands and scope

- `gofmt -w` on the four changed implementation/test Go files: exit 0 (authorized static formatting).
- No local tests, servers, PG, containers, builds or gates run. Runtime acceptance is exclusively this PR's GitHub CI.
- No new imports, packages or package docs; dependency graph unchanged. Dependency map left unchanged; if CI requires a change it must be regenerated with scripts/dev/depmap.sh, never hand-edited.
- Source files retain their existing headers, with Purpose/Depends on/Used by/Invariants added to the three modified production files.
- Outside card write paths: NONE. No changes to baseline.json, route-table-dispatchers.json, the scanner, migrations, R2 pin, gate-adapter assertions, lockfiles or other worktrees.
- Humaux tools (search/store/canvas/coord) are unavailable in this runtime; durable card-local receipts substitute for unavailable shared-store actions. Existing integrator assignment is retained; no separate claim or delegation.

## Acceptance and handoff

Required: this PR's CI has zero new contract-drift findings; RH13 billing case GREEN after the recorded assertion RED; existing billing/webhook tests green unchanged; independent integrator review after GREEN. At submission all implementation runtime checks are PENDING.

NOT_RUN: local tests/servers by owner rule; implementation runtime GREEN pending CI; independent review; provider sandbox/live; production acceptance. Fake Stripe is not provider sandbox evidence. Any unrelated gate blocker is a STOP condition; at most two targeted attempts on the same in-scope blocker.

Rollback: integrator may revert the PR commits; no schema or production action. No local runtime process, fixture, container or port was started. Historical failed-monitor evidence remains under this card's output directory. This worker must not merge.

Final implementation head and CI run/job ids are recorded in the PR body/comment and handoff (a commit cannot contain its own SHA). Do not push an evidence-only commit after GREEN.
