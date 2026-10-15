# Contractdrift route-table dispatcher freeze

The CI-DRIFT gate in [GATES.md](../delivery/GATES.md#ci-drift--static-contract-drift-ratchet)
can resolve an immutable, fully consumed marked Go route table only while its
reviewed dispatcher fingerprint matches `contracts/route-table-dispatchers.json`.
R5 shape checks remain a first diagnostic layer. The R6 whole-function URL taint
allowlist is removed; it is not a second acceptance path.

The pin file is a JSON array. Its first entry carries a `_comment` header;
each entry has `table`, `dispatcher_sha256`, a sorted `functions` list and
`reviewed`. **Any pin hash or function-list change is a contract change requiring
explicit independent reviewer approval after reading the entire closure.**
`pending independent review` records a candidate, never approval. The scanner
never generates or updates this file as part of passing a gate.

The closure starts at the proven table-consuming HTTP dispatch function(s), both
consumers and the table facade's receiver methods. It follows same-package
function references, calls, stored function values, global aliases and method
values, including helpers in other files. Receiver types that cannot be resolved
syntactically conservatively include every same-name method in the package.
Imported package selectors stop traversal. Row literals are data; named,
anonymous and aliased row handler target bodies are excluded. Other dispatcher
helpers remain included. The current buyer/feed rows contain route kinds or
method/path data, with no explicit function targets, so their reached business
helpers also appear in the pin.

Each function is printed, re-tokenized without comments or original whitespace,
and gofmt-normalized. Global function literals receive stable declaration names.
Declarations sort by receiver-qualified function name. SHA256 covers, per function,
`name + "\n" + normalized source (including synthetic package header) + "\n"`.
Both the hash and exact function list must match. Comment-only changes do not
churn the pin; a reached helper's semantic change does. Table row additions or
removals derive new routes without changing the dispatcher fingerprint.

A missing file/entry or changed closure withholds all routes for that table,
reports a fatal UNRESOLVED for every row, and asks for dispatcher review and a pin
update. Malformed/duplicate pins and pins for tables the scanner cannot prove also
fail. These are ERRORs even when no source line is TOUCHED and cannot be seeded
into the legacy baseline. Unrelated legacy opaque routers keep their existing
warning/touched behavior.

This is reviewed change detection within the declared same-package function
boundary, not complete static URL analysis or a runtime routing proof. Imported
implementations, other mutable/global data and route semantics still require the
existing transport tests and independent review. Reviewers must check the pin's
closure, row targets and any affected package contracts. Tests in
`go_dispatcher_freeze_cli_test.go` anchor fixture fingerprints to literal SHA256
values and cover helper edits, comments, row targets/data, missing and unprovable
pins, stored function values and untouched stale-pin errors. See the current
BUYER-ROUTE-TABLE delivery for executed evidence and NOT_RUN gates.
