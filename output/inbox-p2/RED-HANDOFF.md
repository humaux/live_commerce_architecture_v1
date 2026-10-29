# #242 remaining RED batch — do not merge mutation head

Authority: issue #242 and its 2026-10-28 incremental review comment; owner relay
authorizes at most two further pushes (one combined RED, then one restoration).
Starting head: 7f6ea56586ad9c5c90319b05c86b2821de193dd1 (clean worktree).
Prior head GREEN independently read: Foundation 40650271764 / 122688477621;
Gates 40650271800 / required 122818633209, receipts 122794721961.
Both workflows report success and headSha exactly equal to the starting head.
`gh run view <id> --log-failed` is read for each; neither has failed jobs.
These receipts do not accept the mutation head or its later restoration.

## One combined direct-test mutation push

1. `tests/admin/inbox-ui.spec.ts`, INU01, after successful mark-read and before
   reload: a temporary `page.route` obtains the real BFF response with filter=all
   for the browser's filter=unread request. No DOM injection or mocked success.
   This simulates the ignored-filter defect identified by item 2. Real controls
   still select unread. The added precise GET200 + aria-busy=false + visible
   untouched Instagram row barriers prevent the empty loading state from passing
   either absence assertion. Expected failure: **completed unread reload excludes
   the distinct read conversation**, expected count 0, received 1.
2. `tests/foundation/inbox_unread_trigger_test.go`, UR02: temporarily replace only
   the second postDM with the former direct last_inbound_seq increment. No trigger
   or constraint is disabled. Keep the existing messageSeq(2) assertions intact.
   Expected failure: **committed inbound rows=1 ... want rows=2 and matching
   positive sequence**. CI location: foundation catch-all shard g09.

Observed new RED: PENDING until original CI logs/artifacts confirm both assertions.
No infrastructure failure, compile error, or downstream receipt failure counts.

## Restore after observed RED (one push remains)

- Delete only the temporary page.route block (its two mutation comments through
  its closing `});`). Keep all reload completion barriers and exclusion assertions.
- Restore the Go header invariant to `no direct last_inbound_seq writes or disabled
  triggers; all original UR02 permissions remain asserted.`
- Replace the temporary owner.Exec block and its comments with the original:
  `if next := e.postDM(t, peer, "SYNTHETIC-UR02-NEW-INBOUND", time.Now()); next != conv {`
  followed by `t.Fatal("new inbound created a different conversation")` and `}`.
- Append exact RED head/run/job IDs and filtered assertion excerpts to red.log.
  Update DELIVERY.md, PR_BODY.md and the existing PR body before the final push.
- Current tests must all remain; do not weaken assertions or touch frozen files.
- Commit with the required Codex trailer; push only unit/issue-242-inbox-p2.
- Record final Gates/Foundation run IDs in a PR comment; do not push an evidence
  only commit after GREEN. Integrator relays conclusions into a fresh session.
- If either mutation did not reach the intended assertion, report the blocker;
  do not consume additional pushes beyond the owner budget.

## Boundaries / NOT_RUN

No production changes in this batch. No migrations, pins, imports, dependency-map
changes, frozen files, other branches/worktrees, merges or auto-merge. Existing
gate-adapter assertions untouched. Test interception is temporary mutation evidence,
not browser acceptance; final GREEN must use the real unmodified transport.
Local tests/servers/browser/PG: NOT_RUN (policy). LIVE Meta: NOT_RUN.
Independent acceptance: NOT_RUN. New exact-head CI: PENDING.
No local background service or test fixture was created.

Humaux tools are now available. Decision stored and own canvas updated under an
acquired/released lock; no additional coord claim (assigned child lane).
