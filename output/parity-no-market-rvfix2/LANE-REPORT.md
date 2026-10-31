# RVFIX2-403 lane report (push 1: RED tests only)

| finding | class | change |
|---|---|---|
| P1 duplicate tw POST after policy PUT failure | a (RED test) | tests/admin/settings-v2.spec.ts `duplicateMarketAcceptance` ("denied" 403, "lost" reply), called after `noMarketAcceptance` |
| P2-2 false comment (stale line number) | fix | tests/admin/settings-real.spec.ts:264 |

Review file output/issue-107/review-403-2045a03ad.md was absent in the worktree; card text used. Case (b) clicks the settings-pending retry button (retryPending) rather than reloading. Implementation (SettingsWizard.tsx POST callback + retryPending markets branch) is left to the relay.

STATUS: DONE "RED pushed"
