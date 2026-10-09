# LC-A3 print idempotency — DELIVERY

- Unit: `unit/lc-a3-print-idempotency` (worktree `.worktrees/lc-a3-print-idempotency`, base `4ff99766`)
- Contract: `contracts/live-console-v1.md` §7.4 — A3 `POST …/live-sessions/{sid}/comments/{comment_ref}/print` is "idempotent per key"
- Evidence label: **REAL_PG** (task-owned disposable PG 18.6 container via `scripts/dev/test-focused.sh`), **MOCK** Graph (loopback)

## Bug

The A3 route always REQUIRED a well-formed `Idempotency-Key` (`claims.go` `claimsRoute` → `keyRequired`), but
`internal/live/stream.go PrintComment` called `SELECT … FROM live.comment_print(…)` directly, with no
`command.Run` receipt. Every retry — even with the same key — incremented `print_count` again (observed
7→8 under REAL_PG). Reproduced RED below: same-key replay returned `print_count=2` and wrote no receipt.

## Fix (Go only; no migration, SQL, OpenAPI or UI change)

1. `internal/httpapi/live_stream.go` — the A3 handler passes `r.Header.Get("Idempotency-Key")` into
   `PrintComment` (route/body/status grammar untouched; key shape already enforced by `claimsRoute`).
2. `internal/live/stream.go` — `PrintComment(ctx, tx, scope, token, key, sessionID, commentRef)` now wraps
   the print in `command.Run(ctx, tx, scope, "live.comment.print", key, request, &out, fn)`, exactly the
   `draft.go:63` / `inbox_send.go:155` pattern. The canonical request is `{session_id, comment_ref}` (body
   stays `{}`); it is stored only as the sha256 `request_hash`. `authorize()` (live:manage) stays BEFORE any
   write (and is re-checked after `Run`, matching draft.go's replay/revocation guard); `ErrInvalidRef` → 422
   unchanged. Semantics now:
   - same key replay → stored receipt: byte-identical `{print_count,last_printed_at}`, no new increment;
   - new key → prints again (+1);
   - same key + different `comment_ref` or different `session_id` → 409 `conflict` (command.Run hash check, before any write);
   - same key + different store scope → 404 `not_found` (receipts are store-scoped; the session is foreign in that scope → 23503);
   - no label text anywhere: receipt `response` holds only the two fact fields.
3. Tests:
   - NEW `tests/foundation/live_console_print_idempotency_test.go` — `TestLiveConsoleLCN05PrintIdempotencyKey`
     drives the real HTTP route: same key twice → identical body + `print_count` 1 + DB fact 1; new key → 2;
     same key on another ref → 409 + nothing printed; same key on another session → 409 + nothing printed;
     same key on store A2 → 404, store-A1 fact untouched; post-negative replay still byte-identical;
     `ops.command_results` holds exactly key1+key2 with fact-only responses (no session/ref/text in them);
     `lcFind` full-DB scan: the sentinel label text (served live by A2 in the same test) is in no base table.
   - UPDATED `tests/foundation/live_console_comments_test.go` — `TestLiveConsoleLCN05PrintAndMarks` adapted to
     the keyed signature and extended at service level: two fresh keys → 1 then 2; same-key replay → identical
     fact, no increment; same key + other ref → `command.ErrConflict`; foreign session under a FRESH key →
     `command.ErrNotFound` (a reused key would conflict on the hash first).

## Commands and exit codes

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN05PrintIdempotencyKey'` (BEFORE fix) | 1 | **RED** — `red-run.log` |
| 2 | `bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN05'` (AFTER fix) | 0 | **GREEN** — both LCN05 gates PASS (`green-run.log`) |
| 3 | `bash scripts/dev/test-focused.sh '^TestLiveConsole'` (AFTER fix) | 0 | regression sweep: **PASS=69 FAIL=0 SKIP=0** (`regression-run.log`) |
| 4 | `go test ./internal/httpapi/ ./internal/live/ ./internal/command/` | 0 | ok (DB-free units) |
| 5 | `go build ./...` | 0 | clean |
| 6 | `go vet ./internal/...` | 0 | clean |
| 7 | `go vet ./tests/foundation` and `go vet -tags browser ./tests/foundation` | 0 | clean (both tag sets, = check-gates' last step) |
| 8 | `gofmt -l cmd internal tests migrations` | 0 | no output (clean) |
| 9 | `bash scripts/dev/check-gates.sh` | 1 | **BLOCKED at its Node steps — environmental, see below** (`check-gates.log`) |

## RED → GREEN evidence

RED (pre-fix, `red-run.log` — reproduces the reported bug exactly):

```
--- FAIL: TestLiveConsoleLCN05PrintIdempotencyKey (1.95s)
    same-key replay returned print_count=2, want the stored receipt's 1
    same-key replay changed the response:
    print fact after same-key replay = 2, want 1 (the replay must not print again)
    new-key print returned print_count=3, want 2
    same key + other ref: status=200 body={"print_count":1,…}   (want 409)
    conflicting comment_ref printed anyway: print_count=1
    same key + other session: status=200 body={"print_count":1,…} (want 409)
    conflicting session printed anyway: print_count=1
    post-negative replay: status=200 body={"print_count":4,…}
    receipts = map[], want exactly one row each for key1 and key2
```

GREEN (post-fix, `green-run.log`):

```
--- PASS: TestLiveConsoleLCN05PrintAndMarks (1.53s)
--- PASS: TestLiveConsoleLCN05PrintIdempotencyKey (0.41s)
test-focused: top-level PASS=2 FAIL=0 SKIP=0 exit=0
```

Full live-console REAL_PG sweep post-fix (`regression-run.log`): `PASS=69 FAIL=0 SKIP=0 exit=0`, including
LCN01/02/03/04/05/17, deletion eviction, buyer panel, send/quota/takeover, order-for-buyer and templates gates.

## check-gates.sh — BLOCKED (environmental), with manual equivalents

`check-gates.sh` ran (command 9) and aborted at its first Node steps with `ERR_MODULE_NOT_FOUND`
(`typescript-api`, `@live-commerce/i18n`): this fresh worktree has **no `node_modules`**, and the sandbox
denied the two targeted remediations (`pnpm install --frozen-lockfile`, linking/copying the main checkout's
`node_modules`). Per AGENTS.md (max two targeted fixes, then escalate — no endless retries) the Node steps
are escalated as BLOCKED. They are provably out of this diff's blast radius: `git diff --name-only HEAD` =
3 Go files (+2 new files under `tests/foundation` and `output/`), zero TS/UI files, and the PS1 git-grep
step passed inside the same run before the abort. Manual equivalents for the post-Node steps:

- shard-plan `--check` (static): `shard-plan.json` lists `TestLiveConsoleLCN05PrintAndMarks` exactly once and
  has a `catch_all: true` group → the new `TestLiveConsoleLCN05PrintIdempotencyKey` falls into catch-all; no
  duplicate, no unsharded test.
- check-headers ratchet (manual equivalent; the script itself was sandbox-denied): both modified non-test
  files open with `// Purpose:` doc comments on line 1 (modified-file tier); new files are `tests/**` and
  `output/**` (skipped by the ratchet).
- gate registry / dependency register / migration worker-split / gofmt / go-vet steps: gofmt + both go-vet
  tag sets re-run manually (commands 7–8, clean); the diff adds no mode, no spec file, no migration and no
  go.mod change, so the registry, coverage and register checks have no new input.

## NOT_RUN / BLOCKED

- `check-gates.sh` Node steps (`tests/admin/shell-*.test`, `ui-architecture-gate.mjs`, `shard-plan.mjs --check`,
  `pr-modes.mjs` validation, python gate-registry/dependency-register blocks): **BLOCKED** as above — escalate
  to integrator/owner to re-run once `pnpm install` is possible in this worktree.
- `scripts/dev/check-headers.sh`: sandbox-denied; manual equivalent recorded above.
- `scripts/dev/test-local.sh` full suite (~25 min, all modes): **NOT_RUN** — task prescribed the focused
  runner; `^TestLiveConsole` sweep (69 gates) + DB-free units run instead.
- Browser/UI gates: **NOT_RUN** — no UI change; no UI caller of A3 exists yet (LC-U4 label-print component
  is unbuilt; `grep` for `/print` in `apps/` finds only the BFF request-grammar validator, which is unchanged).
- `scripts/check_packet.py`, `experiments/spec_models.py`: **NOT_RUN** — architecture packet untouched.

## Risks / notes

- A3 now writes one `ops.command_results` receipt per distinct key, like every other keyed command
  (claims/inbox/draft); retention is the existing ops receipt policy — no new storage class, and the stored
  response is fact-only (asserted in the gate).
- Behavior change visible to API clients: reusing one key for a *different* print is now 409 instead of a
  silent increment. No shipped client does this (no A3 UI yet); the route contract (codes, body, key
  requirement) is unchanged.
- Contract wording nit for the owner (no action taken): the §11 route table's receipt column for A3 says
  "— (the row is the record)", while §7.4 requires "idempotent per key"; per-key idempotency is only
  implementable with the command receipt (the row has no key), which is what this fix and the owner's
  ruling prescribe.
- Cleanup: the test container is destroyed by `test-focused.sh` (trap); the gate deletes its own
  `comment_prints` rows (incl. the second session's, which has no ON DELETE CASCADE) before the harness
  purge; no processes left running.
