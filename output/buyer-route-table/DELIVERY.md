READY_FOR_CI / INDEPENDENT_REVIEW — Round 10. E3 **STATIC built-CLI acceptance** at `0b9fa08cdc314119c823bc32e90329bff854a18d`; both required scenarios exit0/errors=0/no TOUCHED. Seven reproduced bypasses reject; 12 additional sink variants reject; all 19 mutation sources compile. Build/vet/gofmt/prepush exit0. Hash spec and pins unchanged. Go tests, CI and independent review NOT_RUN. No push.

# BUYER-ROUTE-TABLE — Round 10 (2026-10-15)

Last targeted attempt (attempt 2 on r9 Part B). No fail-open case remains in the exercised scope; ready for integrator CI and independent review, **not unit/runtime/release acceptance**.

Worktree `.worktrees/buyer-route-table`; branch `unit/buyer-route-table`; existing PR #71. Start `d8f2511319b7384be9dcc2f0873b88a40dfd7973`; accepted source `0b9fa08cdc314119c823bc32e90329bff854a18d`; base `e4dad98cc322d35709b475de25063efc92e5d866`. Later delivery changes affect only evidence/docs. `round10/source-hashes.json` binds all 46 scanner/test/pin/gate-document files; verify it at the delivery HEAD. `round10/evidence-hashes.json` binds retained evidence and this delivery.

Executor: Codex (system GPT-6 identity; exact backend ID/effort not independently exposed), integrator scope; no subagents. Coord `57af9b35-a3b8-40a0-ab99-47cc570471af` / `codex-buyer-route-r10`. Ownership: scanner/test files, gate documentation and task output. Main-repository review-r9 was read because it is absent in this worktree; its exact path/hash are in `round10/review-input.json`.

## Required SUMMARY lines

Branch, built production CLI (`round10/branch-receipt.json`):
```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```
Pinned PR #70 overlay (`round10/pay70-receipt.json`):
```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```
Both exit0/errors=0/no TOUCHED, using the same executable SHA256. The overlay is exactly `cmd/api/buyer_payment.go` from `51273344fc12ff4c8062634fdf749d536958bda7`; `overlay-manifest.json` records its hash and sole changed path. Overlay API build exit0.

## Separate commits

| Commit | Change |
| --- | --- |
| `97d977f7b` | Counterexamples FIRST, before guard changes. |
| `d6a6b5583` | Correct fixture literal quoting detected by gofmt in the first commit; tests still precede implementation. |
| `b689baf59` | P2 `mustReplace`, guarded `mustReplaceAll`, and every suffix-named CLI mutation. |
| `690a16e38` | P1 closure reference confinement, immediate-read allowlist and boundary documentation. |
| `0b9fa08cd` | P2 remaining in-process CLI mutations and other source mutation tests; explicit anchor selection replaces no-op fallback attempts. |
| Delivery commit | This report and retained receipts/manifests; no executable source change. |

## P1 root cause, rule and allowlist

The old authority check matched writes through the variable name, leaving pointer/slice values writable through copies and callees. The shared authority function now receives the actual hashed AST closure and checks **every package-var reference** by parser identity. Any reference from outside that closure (function, method, non-frozen initializer, including unreachable code) makes the table UNPROVEN with a positioned freeze ERROR. All `init` functions remain hashed. Exported frozen state remains refused, with only the existing fieldless table-facade exception.

Inside the closure, assignment aliases, returns, field/map/slice/channel/composite stores, method values and goroutine/function-literal captures are refused. Direct writes/address escapes remain refused. Only immediate discard, equality/inequality, unshadowed builtin `len`/`cap` and these stdlib calls are permitted:

- `(*regexp.Regexp).MatchString`, directly invoked on a frozen value initialized directly by stdlib `regexp.MustCompile`. [Go contract](https://pkg.go.dev/regexp#Regexp.MatchString) and [concurrency/configuration boundary](https://pkg.go.dev/regexp#Regexp). `Longest` is not allowlisted and is rejected even inside a hashed function/init.
- `(*encoding/csv.Writer).Write`, directly invoked on a concrete `*csv.Writer` parameter. The [stdlib source](https://go.dev/src/encoding/csv/writer.go) iterates string fields and copies their bytes into its own writer, without storing or modifying the caller's record slice. This real `writeFeed` use is the only additional method needed.

No name-only method match is sufficient. No other regexp method or strings/slices function is allowlisted. Every real use in parseV2Query/matchPattern/keyFor/orderLink/feedPrice/writeFeed passes. Full rule and provenance checks: `docs/engineering/contractdrift.md`.

Conservative limit: even passing a frozen value to a same-package closure function is rejected; parameter-effect analysis is absent. Unsupported safe reads can therefore require a separately reviewed proof extension. This is stricter than allowing all hashed callees, and closes parameter aliases without adding a new alias-analysis subsystem. Current real tables need no such exemption.

## Red map and executed rejection proof

Red means the negative expectation fails because the old CLI silently accepts the unsafe mutation. All seven old-scanner runs exit0/errors0/GO_ROUTE_TABLE4. Final scanner rejects the same source hashes with a positioned freeze ERROR, **before relying on hash mismatch**.

| Reproduction | Old CLI (red) | Final CLI |
| --- | --- | --- |
| pointer-alias | exit0/errors0 | exit1 / 46 freeze ERRORs |
| pointer-helper | exit0/errors0 | exit1 / 46 freeze ERRORs |
| pointer-method | exit0/errors0 | exit1 / 46 freeze ERRORs |
| slice-alias | exit0/errors0 | exit1 / 2 freeze ERRORs |
| slice-helper | exit0/errors0 | exit1 / 2 freeze ERRORs |
| return-escape | exit0/errors0 | exit1 / 46 freeze ERRORs |
| goroutine-capture | exit0/errors0 | exit1 / 46 freeze ERRORs |

`round10/red-map.json` and `green-map.json` link exact mutation source hashes and per-case reports/receipts. Final green target and scanner build are bound to the accepted source SHA. The old binary was built before implementation from d6a6b5583 (only tests differ from r9 scanner).

`inside-map.json` additionally records 12 failures: closure-function return, goroutine capture, Longest, local alias, field/map/slice/channel store, same-package helper argument, non-frozen initializer, init Longest, and an outside method. Each mutated source compiled with `go build ./internal/buyerhttp ./internal/attribution` and then gave exit1 with the variable and position in a freeze ERROR. All CLI/compiler invocations have bounded receipts; no Go test was run.

`harness-notes.json` retains setup corrections: initial malformed test quoting; an exploratory slice mutation incorrectly placed inside hashed FeedHandler already failed pin comparison, so the final bypass control was placed in an outside function as reviewed. No assertion or gate was weakened.

## P2 mutation guard

`mustReplace(t,s,old,new)` fails on absent/empty anchor or unchanged result and preserves first-occurrence replacement. `mustReplaceAll` invokes that guard before preserving all-occurrence behavior. All **117 guarded calls in 15 test files** are listed in `p2-audit.json`; grep over every contractdrift test leaves raw strings.Replace only in the shared helper itself. This includes process CLI and in-process CLI tests whose filenames lack `_cli`, plus remaining parser source-mutation tests. The New setup skips an empty body; alternative JSON anchors are selected before mutation; the aligned registration deletion uses its exact existing source anchor. Existing expectations remain intact. Execution of Go-test assertions is NOT_RUN locally; vet type-checked them.

## Gates, evidence level and commands

| Gate | Command / evidence | Exit |
| --- | --- | --- |
| Production CLI build | `go build -o <task scratch>/new-cli ./scripts/dev/contractdrift`; build-cli receipt | 0 |
| Required branch CLI | `new-cli -root <worktree> -base e4dad98cc`; branch receipt/report | 0 |
| Required #70 overlay CLI | same binary/base on overlay scratch; pay70 receipt/report | 0 |
| Seven bypass red→reject controls | `python3 round10/escapes.py red` / `green`; per-case receipts + maps | expected 0 → 1 |
| Twelve additional boundaries | `python3 round10/escapes.py inside`; per-case receipts + map | expected 1 |
| All 19 mutated sources compile | `go build ./internal/buyerhttp ./internal/attribution`; green/inside *-build receipts | 0 |
| Full package build | `go build ./...`; build receipt | 0 |
| Vet (compiles tests, does not execute them) | `go vet ./...`; vet receipt | 0 |
| Formatting | `gofmt -l <all changed Go files>`; gofmt.json, empty output | 0 |
| Static prepush | integrator `prepush.sh <worktree> e4dad98cc`; prepush receipt | 0 |
| Overlay API build | `go build ./cmd/api`; pay70-build receipt | 0 |

E3 applies only to automated STATIC CLI behavior. Build/vet/gofmt/header/dependency checks are E1. P2 test execution, runtime routing correctness, CI and independent acceptance are not implied. `run.py` records run id, SHA, timeout, terminal exit, output and executable hash; every run finished. To replay deleted scratch, create a fresh task-owned shared clone at the accepted source and rebuild old/new CLIs as indicated by the receipts; never repin mutated source.

## NOT_RUN, risks and preservation

- Local Go tests: NOT_RUN by explicit owner instruction. CI/independent Round10 review: NOT_RUN (no push). REAL_PG, HTTP runtime and browser/click gates: NOT_RUN; this round changes scanner/tests only.
- Known limitation: syntax-only conservative rejection of safe unsupported uses and helper arguments. Cross-package implementations remain outside the hash closure and require their existing review/runtime gates. No claim of unsafe/reflection or whole-program alias verification.
- Pins and hash specification unchanged: closure membership and canonical source stream are unchanged; the new check strengthens proof eligibility. No internal/, cmd/, owning-contract, baseline or dependency changes; preservation-audit.json records this.
- All task-started processes reached terminal exit. Task-owned scratch clone/binaries/overlay were deleted; cleanup.json records the path and reclaimed logical bytes. No servers, fixtures outside scratch, ports or containers were created; no other task output or shared cache was removed.
- 18 changed code files indexed (14 then 4), 221 entities; goDispatcherVarAuthority linked to the Humaux root-cause record. Final acceptance/store/canvas handoff is recorded in Humaux; no local/platform memory used.

No push, PR mutation, merge or production action performed. Integrator must independently review the current source and run CI before accepting the unit.
