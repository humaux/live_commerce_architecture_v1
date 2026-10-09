# W3-U2 sold-out settings HTTP delivery

- task_id: `662ad08e-b312-4b1f-bbf8-f2ffdb25d14d-sub-http` (delegated parent `662ad08e-b312-4b1f-bbf8-f2ffdb25d14d`; no parent claim)
- base_commit: `894990131cd7e13fe907693e295f64d99087df15`
- branch: `unit/w3-u2-live-settings-http`
- worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u2-live-settings-http`
- backend_commit: `97abfa97ac7e456527b8dbf35482408785afb54c`
- OpenAPI_commit: `e3b3c574807762818c4eaec2f8be371db664d89a` (separate for integrator merge)
- role: scoped Go HTTP implementer, explicit owner override of old Codex UI role default; model/effort inherited from parent, actual runtime identifier UNKNOWN.

## Changes

- `internal/httpapi/sold_out_settings.go`: strict Claims-enabled GET/PUT adapter; auth-derived scope; published merchant template resolution; fixed refusal; transactional command receipt around existing SQL setting/audit.
- `internal/httpapi/claims.go`: single registration call inside existing non-nil label guard, three header labels.
- `internal/claims/sold_out_reply.go`: stale Used by header only.
- `internal/httpapi/sold_out_settings_test.go`: DB-free full router and 16 strict transport subcases, including Claims-disabled surface absence.
- `tests/foundation/live_settings_http_test.go`: full real handler + PG scope/permission/CAS/receipt/template rules, eight subcases.
- `contracts/live-settings-openapi.json`: additive endpoint and exact schemas/error semantics.

No SQL, migration, ACL, go.mod, lockfile, Node dependency, push or production action.

## Frozen wire and exact outcomes

`GET /v1/admin/stores/{store_id}/live-settings/sold-out-reply` requires `live:read`. No body/query/Idempotency-Key. Returns exactly `{enabled,template_id,template_version,version}`; never-saved fixed default is read-only.

`PUT` requires `live:manage`, exactly one canonical `Idempotency-Key`, and exact `{enabled:boolean,template_id:string,template_version:positive integer,expected_version:nonnegative integer}`. Response same four fields. New writes must resolve a published merchant template version and reject `Resolved.Fixed`; SQL still owns kind/body/placeholder rules. Setting, audit and receipt commit together; same body/key replays original successful result even after its CAS is stale. Different body/key reuse and stale CAS are `409 conflict`.

- 401 `unauthorized`: missing/invalid bearer.
- 403 `forbidden`: live permission missing.
- 404 `not_found`: authenticated scope cannot access store, or template version unpublished.
- 405 `method_not_allowed`: other methods, including HEAD.
- 400 `invalid_json`: malformed/duplicate/unknown JSON fields or any field null.
- 415 `json_required`: wrong PUT media type.
- 422 `invalid_request`: missing field, top-level null, invalid integer/key/query/GET body, fixed or unusable template.
- 503 `unavailable` / `retry_later`: bounded scoped/database failures.

Every response `Cache-Control: private, no-store`. Errors use existing `{code,message,request_id,retryable,details}` envelope. No request/template body, note, buyer identity or bearer is logged.

## Commands, exits and evidence

All source tests ran against the final Go file contents represented by backend commit97abfa97. `hashes.txt` pins the files; postcommit `hash-verify.log` confirms all hashes unchanged. The red runs used base backend89499013 with new uncommitted test files.

| Actual command | Exit | Evidence |
| --- | --- | --- |
| First queued `bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` | 143, canceled own waiting shell before compile typo correction; no PG started | diagnostic only, not RED |
| `GOTOOLCHAIN=go1.27.1 go test -run '^$' ./tests/foundation` | 0, compile only; zero selected runtime tests | compile.log |
| `GOTOOLCHAIN=go1.27.1 go test -count=1 -v ./internal/httpapi -run TestLiveSettingsHTTP` before adapter | 1 | transport-red.log |
| `bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` initial RED | 1, PASS0 FAIL2 SKIP0; first helper aborted later subcases | red.log |
| Same focused command after helper accepted subtest t + independent stale case | 1, PASS0 FAIL2 SKIP0; 403/404/409 independently red | red-all.log |
| `GOTOOLCHAIN=go1.27.1 go test -count=1 -v ./internal/httpapi -run TestLiveSettingsHTTP` initial green attempt | 1, top-level null guessed400 but existing helper422 | transport-green.log |
| Same transport command after explicit parent ruling preserved422 | 0; one top-level test,16 subcases | transport-green-final.log |
| `bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation` | 0; PASS1 FAIL0 SKIP0,8 subcases, PG18.6/race | pg-green.log |
| `GOTOOLCHAIN=go1.27.1 go vet ./internal/httpapi` | 0 | vet.log |
| `bash scripts/dev/check-gates.sh` | 1; missing Node deps typescript-api/@live-commerce/i18n in backend worktree | check-gates.log |
| `bash scripts/dev/check-headers.sh 894990131cd7e13fe907693e295f64d99087df15` | 0 | headers.log |
| `node scripts/dev/shard-plan.mjs --check` | 0;1241 top-level tests, each exactly one shard | shard-plan.log |
| `git diff --check` | 0 | command output retained in task tools |
| `python3 -m json.tool contracts/live-settings-openapi.json > /dev/null` | 0 | command output retained in task tools |
| `shasum -a 256 -c /Volumes/data/live_commerce_architecture_v1/output/w3-u2-live-settings-ui/http/hashes.txt` | 0 | hash-verify.log |
| backend commit, OpenAPI commit; `git status --porcelain` | 0; clean worktree | commit metadata above |

Evidence class: REAL_PG (HTTP via httptest; no external send); automated red→green tied to final source hashes. Author's evidence is not independent acceptance.

## Unresolved / NOT_RUN

- check-gates remains FAIL in this dependency-free HTTP worktree. Parent/integrator must rerun in prepared UI/integration worktree. No Node dependencies were changed or installed.
- Independent review and independent rerun NOT_RUN here; parent handles acceptance.
- Browser modes, full foundation, CI, LIVE and production deploy NOT_RUN per task scope/local resource policy.
- Existing helper's top-level null422 vs field-null400 explicitly ruled by parent; no shared parser behavior changed.
- All task-owned PG containers/processes ended and cleaned by focused harness traps. No browser/Node fixture started here. Evidence retained under canonical main output directory.
- Humaux scoped fix record accepted before returning; parent owns canvas.

Humaux memory pointer: `02c1b0b5-2068-4086-88f0-9cbbcb5c69e8` (scoped fix title/body retrieved after queued store). Code indexing queued for three new Go files; linkage attempted for registerSoldOutSettingsRoutes. Initial canceled-wait diagnostic retained in `queued-cancelled.log`. Parent owns canvas and independent acceptance.
