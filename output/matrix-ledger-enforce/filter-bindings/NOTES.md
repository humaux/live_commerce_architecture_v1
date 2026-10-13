# PR28 final execution-filter proof

Only threads 4251056896 and 4251056902 are changed. Remote base f1f404a15d15f08d01269e1ec06a07df9de8d492 was fetched and merged with exit 0 (already up to date). Source checkpoint e0fb76ce4034721a5429f701469a3e9fd4b756e7 modifies only the execution module and its real-registry negative controls.

The exact real Node command retains all three required suite filenames. Six negatives append each of `--test-name-pattern` and `--test-skip-pattern` in space, equals and whole-quoted forms. Eight Go negatives retain RUN/SKIP/package forwarding, appending `-run` or `-skip` in space, bare equals, quoted equals and whole-quoted equals forms. Six existing positive/negative tests remain; an unrelated filtered Node command is a positive control.

`node --test tests/ci/product-editor-matrix-execution.test.mjs`: unchanged old module RED exit 1, 12 failed / 9 passed; final module GREEN exit 0, 21 passed / zero failed. `red.json` and `green.json` bind exact bytes. The original command remains positive. The parser reads literal shell words without evaluation and rejects only exact selector argv keys; known dynamic Go forwarding tokens are checked independently and removed before checking for extra selectors.

Initial preparation used a JavaScript replacement string whose literal `$` suffix was interpreted as substitution. Exact-append assertions exposed that fixture error. Mutations now use replacement callbacks; the genuine RED was rerun on unchanged old module, with try/finally restoration, then GREEN on final module. Initial logs are preserved separately and do not substitute for the genuine RED/GREEN receipts.

Scoped independent read-only source review at e0fb76ce is E1 PASS, not new CI/K3 or full independent acceptance. Index submission included both changed MJS files; the server did not resolve matrixExecutionIssues for the requested code-memory edge. The exact path/hash/reason remains in Humaux fix 124f31ca-afd0-489f-b279-52d8ce81a28d.

New native browser evidence is required because both changed executable sources are hard-bound by the canonical evidence gate. Historical round5 bytes and tested commit are retained. The reused bounded runner snapshots all fourteen executable source hashes before and after each gate. No waits, browser assertions, suite filters, calibration, product scope or dependencies change. Shared source hashes are informational.
