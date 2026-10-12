# browser-evidence-untracked — paused checkpoint

Owner requested pause after the current commit to handle PR16 round4 (2026-10-09).

Base 50086616; worktree/branch browser-evidence-untracked. Fetched origin/r3/integration: already up to date then; PR22 not present. Recheck PR22/trunk before resuming acceptance.

Integrated producer relocation 1225bdd9 (author12fd9078) and harness/CI relocation3debdb4f (author3e3ce8e3). Root adds the static evidence writer ratchet and registration in check-gates/test-node. Product code and committed historical evidence were not changed. The explicit shell --baseline command remains intentional golden maintenance, outside ordinary gate execution.

Checkpoint checks: helper/ratchet Node tests18/18 exit0; whole-repo ratchet exit0 with zero unsafe write destinations. Pre-fix ratchet found72 unsafe destinations (ratchet-before.log). Child Go root unit/vet, aggregate9/9, shell/CI syntax passed; ignoring ROOT mutation was RED then restored GREEN. These are UNIT/static evidence, not full unit acceptance.

NOT_RUN/PENDING:
- Final root independent review of all producers/harnesses/CI consumers; initial inventories are source-only (51 mode strings,49 Go source files plus JS inventory in Humaux/task messages).
- Full test-node, check-gates, admin tsc, root browser-tag go vet.
- Required --browser-product-editor + another formerly dirty mode, on a committed clean tree; preserve raw `git status --short | grep -v '^??'` output WITHOUT restoration.
- Explicit inject→ratchet RED→restore→GREEN proof on the integrated branch, plus adversarial review of lexical alias/quoted-property handling and helper path boundaries.
- Complete readable mode/root/consumer inventory and final DELIVERY; independent K3 review/push remain integrator-owned.

Read-only media-size comparisons now require LC_MEDIA_SIZES_BASELINE for after phase; comparison thresholds remain intact. Review this caller contract and CI sweep/visual aggregation during resume.

Temporary author worktrees .worktrees/evidence-spec-paths and .worktrees/evidence-harness-paths are clean and deliberately retained for resuming. No PG/browser run was started for this unit; no running child process; no push.
