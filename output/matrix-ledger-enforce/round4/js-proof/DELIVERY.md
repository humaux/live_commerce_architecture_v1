task_id: 945871d1-sub-js (parent 945871d1-23a6-431f-b460-be41577377ad)
base_commit: c81533c26a067422141dcb9d3a0f775d09078027
role: test_worker isolated JS implementation; model gpt-6.1-sol; reasoning medium (corrected by parent from the actual task card)
worktree: .worktrees/pr28-r4-js-proof detached
write_paths: tests/ci/product-editor-matrix-claims.mjs; tests/ci/product-editor-matrix-ledger.test.mjs; task evidence output/matrix-ledger-enforce/round4-js

Implementation: existing structural signature now includes SyntaxKind of PrefixUnaryExpression/PostfixUnaryExpression.operator and operand. Existing canonicalClaims signature API tested with !x/~x/+x, ++x/--x, x++/x-- and harmless parentheses. CI presence and either hook presence use Object.hasOwn, so empty strings fail too. Guard executes before spec read, mutation/snapshot and test collection registration. Real child node test module exercised in 8 env combinations, NODE_TEST_CONTEXT and inherited CI/hooks cleared. Nonempty unmatched test-name sentinel plus no process isolation prevent recursive tests; no browser starts. Existing assertions/mutations retained.

Commands (cwd this worktree):
node --test --test-name-pattern='canonical unary signature|CI hook refusal' tests/ci/product-editor-matrix-ledger.test.mjs
RED before implementation fixes, corrected child harness: exit 1, tests12/pass0/fail12; red.log and red.exit.
Same command GREEN after fixes: exit 0, tests12/pass12/fail0; green.log and green.exit.
git diff --check: exit 0.
git diff -- tests/ci/product-editor-matrix-claims.mjs tests/ci/product-editor-matrix-ledger.test.mjs > output/matrix-ledger-enforce/round4-js/changes.patch: exit 0.
shasum -a 256 tests/ci/product-editor-matrix-claims.mjs tests/ci/product-editor-matrix-ledger.test.mjs output/matrix-ledger-enforce/round4-js/changes.patch > output/matrix-ledger-enforce/round4-js/hashes.txt: exit 0.

Initial harness failures retained: initial-harness-red.log exit1 (recursive node:test run skipped due inherited NODE_TEST_CONTEXT); initial-harness-green.log exit1 (4 unary passed,8 failed because inherited NODE_TEST_CONTEXT skipped). After clearing NODE_TEST_CONTEXT, empty-name pattern ^$ triggered root test recursion; recursive-harness-red.log retained and only our child PIDs stopped. Changed only child test invocation to nonempty sentinel/isolation none, then obtained genuine red and green above. No failed samples deleted. Saved source backups are inert .source.txt and never test runners.

Evidence level: focused automated source/child-gate tests tied to hashes (author proof); independent acceptance NOT_RUN here, parent must apply/retest. Full Node runner/check-gates/browser/Go/PG/provider/CI NOT_RUN. No commit/push/provider changes. No acceptance/spec/fixture/wait changes. Unresolved: parent independent acceptance and global gates. This subtask does not claim PR complete.
