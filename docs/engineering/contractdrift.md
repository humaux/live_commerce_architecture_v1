# Contractdrift route-table dispatcher freeze

The CI-DRIFT gate in [GATES.md](../delivery/GATES.md#ci-drift--static-contract-drift-ratchet)
can resolve an immutable, fully consumed marked Go route table only while its
reviewed dispatcher fingerprint matches `contracts/route-table-dispatchers.json`.
R5 shape checks remain a first diagnostic layer. The R6 whole-function URL taint
allowlist is removed; it is not a second acceptance path.

The pin file is a JSON array. Its first entry carries a `_comment` header;
each entry has `table`, `dispatcher_sha256`, sorted `functions` and `declarations` lists and
`reviewed`. **Any pin hash, function-list or declaration-list change is a contract change requiring
explicit independent reviewer approval after reading the entire closure.**
`pending independent review` records a candidate, never approval. The scanner
never generates or updates this file as part of passing a gate.

The closure starts at the proven table-consuming HTTP dispatch function(s), both
consumers, the table facade's receiver methods and every package `init` function. From signatures and bodies it
follows every referenced same-package function, const block, var spec and type
spec, recursively following references in those declarations too. Var initializers
can pull functions into the closure; anonymous function source is part of its var
spec. Receiver types that cannot be resolved syntactically conservatively include
every same-name method. Const blocks are frozen whole to preserve iota positions
and inherited expressions. Key expressions in maps are traversed too.

The row table variable is data and is never followed. A handler target referenced
only by that data stays outside the closure. If any closure body also references
the target directly (including fallback calls or stored aliases), its declaration
and dependencies are frozen. The current buyer/feed rows carry kinds/method/path
data, so their reached business helpers remain included.

Before hashing, every non-test file in the table package must be independent of
build selection: `//go:build`, legacy `// +build`, and OS/architecture filename
suffixes fail closed, without evaluating tags. A frozen name declared twice also
fails (method names are receiver-qualified; multiple `init` functions are legal
and all are hashed). Last-wins lookup maps never establish proof.

Every frozen package var must be private, except the marked table's documented
fieldless facade value. Across every non-test package file, **every reference**
must belong to the hashed function/spec closure (including every `init`). An
outside function, method or non-frozen initializer reference withholds every row
with a freeze ERROR naming the variable and reference file:line. Parser-bound
local shadows and selector member names are not package-variable references.
Existing direct-write, address-taking and builtin-mutation checks still run.

Inside the closure, only immediate, proven read contexts are allowed: discard
(`_ = value`), equality/inequality, unshadowed builtin `len`/`cap`, and this entire
stdlib allowlist:

| Use | Required provenance | Read-only basis |
| --- | --- | --- |
| `(*regexp.Regexp).MatchString` | frozen initializer is a direct call to stdlib `regexp.MustCompile`; directly invoked receiver method | [stdlib contract](https://pkg.go.dev/regexp#Regexp.MatchString) returns whether the input matches; [Regexp concurrency contract](https://pkg.go.dev/regexp#Regexp) separates matching from configuration mutators |
| `(*encoding/csv.Writer).Write(record)` | directly invoked on an identifier bound to a concrete `*csv.Writer` parameter, with that file's stdlib import | [stdlib implementation](https://go.dev/src/encoding/csv/writer.go) iterates string fields and copies bytes into its own writer; it does not assign to or retain `record` |

These are the only non-builtin calls receiving frozen values in the real
closures (`parseV2Query`, `matchPattern`, `keyFor`, `orderLink`, `feedPrice`,
`writeFeed`). `Longest` configures matching and is **rejected inside the closure**
as well as outside it. Hashing its source is insufficient. No other regexp
method, strings/slices function, name-only `MatchString`/`Write` imitation or
unknown imported function is implicitly trusted.

Returns, local/global aliases, field/map/slice/channel stores, composite values,
method values and goroutine/function-literal captures fail closed. The syntax
checker deliberately refuses even a direct same-package closure call receiving
a frozen value: it does not attempt parameter-effect/alias analysis. A future
read-only helper can be supported only with a separate proof and negative gate;
it is not automatically safe because its body is hashed. Other unsupported read
syntax also refuses proof. This conservative restriction accepts the current
real tables without changing the hashed source or the pins.

Each declaration is printed, re-tokenized without comments or original whitespace,
and gofmt-normalized. Sort is stable by kind then name, with the full source key
(file order/position, plus the function's file:line:column key) as tiebreak. Function
names are `Recv.Name` with receiver type parameters stripped, or `Name` for plain
functions. A multi-name spec lists its names together; a const block lists sorted
member names while preserving source order inside the hashed block. SHA256 covers,
per declaration, `kind + " " + name + "\n" + normalized source (including synthetic
package header) + "\n"`. The hash, exact function list and exact declaration list
must all match. Comment-only changes do not churn pins; referenced const values,
iota ordering, var initializers and type fields do. Row additions/removals derive
new inventory without changing the dispatcher fingerprint.

A missing file/entry or changed closure withholds all routes for that table,
reports a fatal UNRESOLVED for every row, and asks for dispatcher review and a pin
update. Malformed/duplicate pins and pins for tables the scanner cannot prove also
fail. These are ERRORs even when no source line is TOUCHED and cannot be seeded
into the legacy baseline. Unrelated legacy opaque routers keep their existing
warning/touched behavior.

This is reviewed change detection within the same-package declaration boundary,
not complete static URL analysis or a runtime routing proof. **Cross-package
helpers (e.g. `command.ValidID`) are NOT frozen** and are covered by normal review
and other gates, including transport tests. Var initializer source is frozen;
later runtime values are not a state snapshot. This syntax gate does not model
unsafe/reflection, mutation inside imported implementations. Unknown frozen-value uses are refused,
not treated as harmless aliases. Imported implementation behavior remains a
review and runtime-gate responsibility. Reviewers must check the pin's
closure, row targets and affected package contracts. Tests in
`go_dispatcher_freeze_cli_test.go` anchor fixture hashes and cover declaration
mutations, comments, data-only row targets, direct fallback targets, stored function
values, generic names/determinism, missing/unprovable pins and untouched stale
pins. See the Round 10 delivery for executed evidence and NOT_RUN gates. All CLI
source mutations use `mustReplace` (first occurrence) or `mustReplaceAll` (which
first calls `mustReplace`); missing/no-op anchors fail the fixture immediately.
