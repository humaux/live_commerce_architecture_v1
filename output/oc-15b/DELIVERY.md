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

## Current implementation (Fix 2, integrator FINAL)

- Restore `cmd/api/platform_billing.go`, `cmd/api/platform_billing_test.go`, `internal/billing/webhook.go` and `internal/billing/errors.go` byte-for-byte to `origin/r3/integration` (`5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248`). All earlier mount refactors and the added mount regression test are removed as explicitly instructed. Existing mount/webhook tests are unchanged from that ref.
- New `internal/billing/admin_errors.go` maps allowlisted PT403 messages in the `pin_customer` and `record_checkout_session` SQL callbacks, BEFORE the unchanged shared `run`/`mapError` can collapse them. Other errors follow the existing path. Typed errors still wrap ErrForbidden, so platform.WithScope retains its store-visibility recheck and transaction rollback. No SQL, scope, permission, Stripe request or webhook behavior changes.
- `internal/billing/service.go` changes only its required header and those two admin-only SQL callback returns. The scanner's webhook dependency spans remain untouched. Portal has no guarded writer and is unchanged.
- Contract §16 lists the admin write mapper; RH13 billing-checkout assertion remains intact (only its implementation-path comment changes in this fix).
- First placement attempt: static scanner exit 0, errors=0. No second placement attempt was needed. Earlier failure at `ff23d27c5`, Foundation run `40620706981` / job `122593760499`, named `cmd/api/platform_billing.go:27` and `:77` for the opaque EXACT webhook guard; current mount equals the integration ref.

## Static checks and scope

- `go run ./scripts/dev/contractdrift`: exit 0. Summary and source hashes in `fix2-static.log`; full raw local output in untracked `fix2-scanner.log` (not committed because its raw warning formatting has trailing whitespace).
- `go build ./...`: exit 0; `go vet ./internal/billing ./cmd/api ./tests/identityhttp`: exit 0. These are authorized static checks, E1 only. No local tests, servers, PG or containers.
- `gofmt -w internal/billing/admin_errors.go internal/billing/service.go` and `git diff --check`: exit 0.
- No Go package or import dependency change (billing already imports pgconn and platform). Dependency map unchanged; regenerate with scripts/dev/depmap.sh only if required by CI.
- Outside card write paths: NONE. No scanner, baseline, dispatcher pin, migration, R2 pin, gate-adapter assertion, lockfile, other worktree or branch changes. Existing untracked handoff/monitor files from previous sessions are preserved.
- Humaux, coord, code-index and canvas tools are unavailable in this runtime. No Humaux store/update claim; root cause, change and handoff are recorded here. No recursive delegation or separate claim.

## Fix 1 of 2 (2026-10-28; superseded by Fix 2)

Foundation run `40619161251`, job `122588674187`, failed at head `52ed2c77a986b111af06399732e7e90a3e59fd61`: `TestGoRepositoryInventory` requires `EXACT /v1/platform/stripe/webhook` in `inv.Unresolved`, while the explicit method route belongs in `inv.Routes`. Putting registration inside the mount classifies it as a route registrar and skips its opaque EXACT guard (`go_extract.go`), so the original refactor removed that guard. Raw filtered failure: `fix1.log`.

Historical Fix 1 moved registration to construction and retained the concrete handler return type, but Foundation run `40620706981` / job `122593760499` still failed contractdrift at `ff23d27c5`. The integrator rejected further mount refactoring. Fix 2 above replaces that approach entirely; no earlier mount/runtime claim is current acceptance.

## Acceptance and handoff

Required: this PR's CI has zero new contract-drift findings; RH13 billing case GREEN after the recorded assertion RED; existing billing/webhook tests green unchanged; independent integrator review after GREEN. At submission all implementation runtime checks are PENDING.

NOT_RUN: local tests/servers by owner rule; implementation runtime GREEN pending CI; independent review; provider sandbox/live; production acceptance. Fake Stripe is not provider sandbox evidence. Any unrelated gate blocker is a STOP condition; at most two targeted attempts on the same in-scope blocker.

Rollback: integrator may revert the PR commits; no schema or production action. No local runtime process, fixture, container or port was started. Historical failed-monitor evidence remains under this card's output directory. This worker must not merge.

Final implementation head and CI run/job ids are recorded in the PR body/comment and handoff (a commit cannot contain its own SHA). Do not push an evidence-only commit after GREEN.
