Matrix execution bindings now refuse the deferred Go/Node filter aliases and correctly parse assignment-prefixed commands. They also verify that the real `foundation.yml` job and steps execute the required commands, rejecting disabled or unreachable steps, suppressed failures and selecting `GOFLAGS` values.

Issue #45, group 2 of 4. This changes the static test binding only; the existing CI commands are unchanged.

Validation on frozen source `c59e43e0`:

- Focused mutation tests: 143/143; initial and follow-up bypass injections were red before repair.
- `bash scripts/dev/test-node.sh`: exit 0, 1812/1812, zero skips.
- `bash scripts/dev/check-gates.sh`: exit 0; all 83 modes documented and tracked tests executed.
- Before/after source hashes match. Raw failures, final logs and receipts are in `output/matrix-coverage-v2-execution/DELIVERY.md`.

Browser, REAL_PG and remote CI were not run by this author for this static-only group. Integrator independent review is pending.
