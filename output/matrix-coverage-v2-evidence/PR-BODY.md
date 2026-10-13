Canonical delivery records now validate every required native result, indexed artifact and copied byte before returning historical source drift. The frozen full-run receipt corroborates the producer set and source identity, so adding a producer preserves an immutable old record while omitted old producers or tampered bytes remain fatal.

Issue #45 group3. The strict fresh-delivery invocation is documented in GATES.md. Four new Go regression cases are registered in g06 without moving existing tests.

Validation: focused Go red→green mutations for required files, artifact/index tampering, native copies and producer omission; full `bash scripts/dev/test-node.sh` and `bash scripts/dev/check-gates.sh` exit0 at frozen67cfc86f, unchanged source hashes. One existing PR18 child deadline failure is retained alongside isolated and full successful repeat; no assertion/timer changes. Evidence: `output/matrix-coverage-v2-evidence/DELIVERY.md`.

This group does not replace browser delivery evidence or claim new UI acceptance. Independent review and CI remain pending.
