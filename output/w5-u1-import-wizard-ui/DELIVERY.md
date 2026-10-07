<!-- Purpose: deliver the import wizard alone on the canonical W6 base, with local evidence and CI handoff.
Depends on: W6 9f7ec2cd, frozen migration-import APIs0152/0156, current scope/source proofs and local checks.
Used by: integrator review and GitHub acceptance after W6 PR3 merges. -->
# W5-U1 import wizard — reconciled delivery

Branch: unit/w5-u1-import-wizard-ui. Base: 9f7ec2cd84e34ff87dfb472e4a475126cb3908d5. Final commit is recorded in the main checkout receipt after FINAL STEP. Configured Codex family; exact deployed runtime model UNKNOWN. Root owns this worktree; read-only reviewer uses no write paths or recursive delegation.

The customer/history CSV wizard, safe row verdicts, UTF-8 refusal guidance, immutable manual replay, receipt confirmation, terminal replay mismatch, failure CSV and read-only customer archive remain intact. Marketing consent is always unknown; historical orders do not enter revenue. PRIVATEALL wraps import responses including real auth/coded errors. Thirty-one import namespace files are byte-identical to rollback8c1183a; no behavior rewrite in this reconciliation.

## Scope reconciliation

The old branch included b20e1bcf duplicates and unrelated settlement work via trunk merges. Latest W6 was fetched and merged first; rollback codex/w5-u1-before-reconcile-20261007=8c1183a8678ee6d7ed3c82b89a7c36b55d30a848 retains all history. The unpublished W5 branch is reconstructed from W6 with only import paths. All47 unrelated paths (14 source/migration/deploy/tests,33 settlement evidence) are excluded from its diff. They are retained in the rollback history, without a revert of trunk settlement code.

W6 tag/report/privacy/readiness fixes and tests are the canonical base; b20 is absent from the reconstructed history. Shared GATES keeps the exact W6 seam row. test-node adds only the import block. test-local retains every W6 flag/order/heartbeat lock and adds migration-import once to validation, usage and admin-build, plus its own preflight/run blocks. Mode count79→80; no thresholds or tests weakened. No backend/runtime/schema/lockfile changes belong to this unit.

## Local checks and evidence

Candidate tree3195778e70893d1da860179ee58d229755758e56 was tested with:

- bash scripts/dev/test-node.sh: exit0; optional R04 binary NOT_RUN.
- pnpm --filter @live-commerce/admin typecheck: exit0.
- Strict import/customer-tags/reports spec tsc: exit0; exact args in reconcile/specs.status.json.
- bash scripts/dev/check-gates.sh: exit0.
- Native scope audit: pre-reconcile47 foreign paths RED1; candidate0 foreign paths GREEN0. This is tree-scope evidence, not browser calibration.
- git diff --check and source-byte comparison: exit0. Final source/commit receipt is in main output.

Read-only review accepts scope:144 original candidate paths (43 source/gate/harness plus101 W5 evidence);31 import files unchanged; W6 files/Node suites/flag order retained. Source-only same family, no runtime review claim. Final delivery/evidence additions remain inside the unit's output path.

Setup failures are retained as failures: two normalization probes guessed lowercase Usage and stopped before test-local writes; exact uppercase source was then used. The initial parallel wrappers contended on git write-tree; three tests did not start. One serial tree binding fixed the wrappers; fresh Node/types/specs0 and the already-passed gates0 bind the same candidate. None of these is credited as behavioral RED.

## CI gates

Integrator pushes the final branch after the source handoff. W5 gets its own PR only after W6 PR3 merges; no stacked PR and no force push. Before each commit, fetch and merge the branch's own origin ref if it exists. Current origin/unit/w5-u1-import-wizard-ui is absent; its absence is checked and logged rather than claimed as a merge.

- bash scripts/dev/test-local.sh --browser-migration-import: normal GREEN.
- LC_MIUI_CALIBRATION=drop-customer-commit bash scripts/dev/test-local.sh --browser-migration-import: RED for MIUI-RED-COMMIT-RECEIPT after readiness.
- LC_MIUI_CALIBRATION=truncate-preview bash scripts/dev/test-local.sh --browser-migration-import: RED for MIUI-RED-PREVIEW-DATA after readiness, then normal GREEN.
- Affected regressions: --browser-customers-billing, --browser-admin-shell, click-sweep shards/aggregate, visual-lint shards/aggregate, admin build.

## NOT_RUN and cleanup

Browser/Next build/real import PG/full foundation/CI and both import fault calibrations NOT_RUN locally. Provider sandbox/live, production, money and real PII NOT_RUN. E3 applies only to local Node/types/structure and source scope. No PR/push/deploy performed. All owned local checks finish before commit; no PG containers/ports started. Old source/evidence is historical in the rollback branch; new final SHA/current source receipt supersedes the old W5 handoff.
