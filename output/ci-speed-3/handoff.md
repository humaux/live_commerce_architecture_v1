# CI-SPEED-3 implementation handoff

Dispatched worker, no recursive delegation. Branch unit/ci-speed-3-ui-select; PR #266 into r3/integration.
Original base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248; RED SHA 04036e51ba4e7ed1236a825e1f96c635f563ec44.
Runtime identifies as GPT-6 Codex; backend model identifier/effort not exposed. Worktree is the assigned ci-speed-3 worktree.
Main checkout untouched. No merge, auto-merge, force-push, commit status, deployment, production action or recursive delegation.
An upstream merge was started without committing, then immediately aborted when rechecking the no-merge rule; no upstream changes remain in the branch.

## Authority and implementation

Authority: owner's CI-speed directive and CI-SELECT narrow-per-domain+nightly, plus integrator's
2026-10-28 ruling and corrections in the current dispatch. The sole amended legacy test is the one
explicitly permitted in the ruling (already in RED). All remaining original tests are unchanged.

Registry/spec/harness/server evidence derives modes' app ownership and exact local-import closure,
including helpers. CLI display compatibility no longer widens ordinary paths. Foundation, backend,
deploy and matrix unions remain. Unknown/missing data/shared infrastructure select the full universe.
Missing browser_* paths retain the full browser universe; observed tags additionally retain non-browser
registry runners, while unknown non-browser-prefixed foundation sources retain tagged runners too.
Foundation's actual scripts/dev/test-node.sh declarations identify separately covered Node-only prep
loaders; browser-driver uncertainty remains full-universe, as do edits to unresolved prep inputs.

Ownership JSON belongs beside existing covers-derivation as required by the card; generator is
scripts/dev/derive-browser-ownership.mjs. CI's existing backend-coverage suite checks drift and its
corruption witness. Source-data snapshot is integration 997480141a7959ad6256a29e43bb1af31b97a404
(read-only Git objects, via --source-ref), because #248 and #237 advanced the integration registry/specs
since this branch forked. In GitHub's merge checkout, the default generator should reproduce it. If the
base advances again and introduces new ownership inputs, treat drift as a new exact-source condition;
never claim the older artifact remains accepted. Do not alter another card's registry/workflow.

## Evidence and limits

E3 RED: Foundation run 40631602073 / job 122629013295 failed the card's own assertions; red.log
and red-assertions.log record them. Gates run 40631602098 succeeded on RED, not card GREEN.
E1 only for implementation: node --check on planner, generator, both CI test files and static impact
measurement script exited 0. python3 scripts/check_packet.py exited 0, PACKET_STRUCTURE_ONLY;
its timestamp output is retained in packet-check.json; its incidental tracked output was restored.
Static artifact generation and the required old/new selection measurement invoked no test/server.
No go tests, Node test suites, browser tests, server, container, formatter or production command ran locally.

Static impact: last 10 merged PR diffs, both policies on identical integration registry, 473 -> 473 modes.
Single known live-settings spec: 53 -> 1 owner; native-device helper: 6 owners.
Admin ownership 47 / storefront 29: the <=15 app target is NOT achieved. Do not remove owners to hit it.
Independent review, GREEN, browser execution and all runtime acceptance remain pending.
Humaux tools unavailable: no memory store, canvas, code index/link or coord writes/locks were possible.
No background process remains. CI run IDs will be supplied in the PR comment and final WAIT_RUN relay.
