# PR2 MOU03 product causality review — E1

task_id: `0cb19bd4-sub-mou-product-review`; parent: `0cb19bd4-2062-42dc-99f9-322a1bfb828c`.

base_commit: `44bb155e1ad1ef912485a409833c3dffeeb5350e`; reviewed head: `fbdf8ebb0d1e091230fae7a646ce5405b84b2c50`.

Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u4-parcel-merge-ui`. Role: independent read-only source reviewer; dispatched configuration gpt-6.1-sol/high, exact executing model identifier UNKNOWN. Own write path: this primary-output report only. No source, MOU03 waits/assertions, commits, pushes or gates changed/run.

**Conclusion: no new product causal root is supported by the scoped source delta.** In particular, the new `useGuardedRead` signed-out propagation does not expose a self-triggering logout/read/navigation loop. This does not prove that the browser close lifecycle is defect-free. The cause of the unresolved `otherTab.close()` completion remains **UNKNOWN**; there is no evidence-backed product patch to recommend from this delta.

## Exact causal chain

1. `apps/admin/lib/customers-client.ts:201–209` adds `signalLogout()` only after the current load fails its authority check and still passes `live()`. The emitter marks itself blocked first. `refresh()` at lines 288–302 likewise rejects hidden/blocked/stale/aborted work before its new emission. An aborted request does not continue emitting indefinitely.
2. `customers-client.ts:222–245` handles local, BroadcastChannel and storage logout solely through `clear("signed-out", true)`: increment generation, abort controller, clear session fence/data, and latch blocked. No receiver calls `signalLogout()`, changes the reload tick, or navigates. `reveal()` at lines 233–236 will not reload a blocked hook. Only explicit `reload()` at lines 282–285 unblocks/increments the tick. `setView` does not change `load`'s dependency key. Therefore this receiver/emitter graph contains no feedback edge that can sustain read or logout reentrancy.
3. Unchanged `apps/admin/lib/session-events.ts:3–12` dispatches one synchronous same-tab event, posts one logout message through a short-lived channel, closes that channel, and writes one storage marker. There is no timer, network request, recursive emission or navigation in the function.
4. The actual clicked sign-out path already emits this signal **before** its POST: unchanged `apps/admin/src/shell/api.ts:23–36` calls `signalLogout()` at line 24, performs POST logout, then signals again at line 36. `Dashboard.tsx:40` owns the relevant `useGuardedRead`. The first synchronous signal invalidates/aborts that hook's pending read before POST completion; a subsequent rejected pending load fails `live()` at `customers-client.ts:201` and cannot reach the new line 207. This undercuts the proposed causal link between the newly added read-failure emission and the observed post-204 close hang.
5. Unchanged `WorkspaceFrame.tsx:86–103` responds by setting `expired=true/current=false`, aborting its read and clearing private shell data. Its ordinary focus refresh at lines 137–139 is guarded by `current`; the effect at line 83 refuses to revive an expired shell. Removing a store query uses `history.replaceState`, not a document navigation. The only full replacement in this sign-out path remains unchanged `signOut()` at lines 178–185: await `logoutWorkspace()`, then `window.location.replace()`.
6. Unchanged `MerchantOrders.tsx:176–203,452–458` clears order/detail/selection/group/action state and blocks on logout. Those listeners do not re-emit logout or request navigation. Existing `flushSync` teardown predates this delta; it is not a newly added feedback edge.

No new unbounded loop, unload/beforeunload blocker, automatic retry, periodic timer or close handler was found in this scoped propagation path. A finite notification may still overlap a browser navigation, but source reading cannot identify the browser's internal close/commit ordering or turn that overlap into a confirmed product root cause.

## Differential and source binding

Independent Python `git show` byte comparisons exited **0**: these 11 paths are unchanged across `44bb155e..fbdf8ebb` and their current checkout bytes equal the reviewed commit:

`MerchantOrders.tsx`, `ParcelGroup.tsx`, `WorkspaceFrame.tsx`, `Dashboard.tsx`, `lib/session-events.ts`, `src/shell/api.ts`, `tests/admin/orders-ui.spec.ts`, `playwright.config.ts`, root `package.json`, `apps/admin/package.json`, `pnpm-lock.yaml` (component/library names under `apps/admin`).

- Current changed `apps/admin/lib/customers-client.ts` SHA256: `f488f304926dc52c23ac34edbf3cd7caebf230d576a60b0a38b269292c936f68`; equals `git show fbdf8ebb:...`.
- `WorkspaceFrame.tsx`: `6e3d520e2017731e85c53b1c4b2e502f42653e83e48d35e456a13a665f017f06`.
- `session-events.ts`: `4496cbc10b813dee320b9645db9adb24ada96de03f1640cfb26d0900619a3d9e`.
- `src/shell/api.ts`: `52322912d67e306b919b79024c1f1e2e00c82a0482882a61d86c9ec0c1f29e98`.
- MOU spec: `b3af238da8235f82a421f9b53596b9194f34c0436c70881bb90a410d418543a0`.

The worktree has unrelated generated screenshot/ledger changes from the parent's gate run; this reviewer did not edit them. The named product/spec/config paths above were checked individually rather than claiming the whole worktree was clean.

## Evidence attribution and commands

Read-only commands actually executed: `git rev-parse`, `git status --short`, scoped `git diff 44bb155e..fbdf8ebb -- apps/admin/lib/customers-client.ts`, `rg`, targeted `sed`/`nl`, and Python subprocess/hashlib comparisons. The final 11-file comparison and customers-client hash command exited **0**:

```python
import subprocess, hashlib, pathlib
files = ['apps/admin/components/MerchantOrders.tsx', 'apps/admin/components/ParcelGroup.tsx',
 'apps/admin/components/WorkspaceFrame.tsx', 'apps/admin/components/Dashboard.tsx',
 'apps/admin/lib/session-events.ts', 'apps/admin/src/shell/api.ts',
 'tests/admin/orders-ui.spec.ts', 'playwright.config.ts', 'package.json',
 'apps/admin/package.json', 'pnpm-lock.yaml']
for f in files:
    a = subprocess.check_output(['git', 'show', '44bb155e:' + f])
    b = subprocess.check_output(['git', 'show', 'fbdf8ebb:' + f])
    assert a == b
    assert pathlib.Path(f).read_bytes() == b
f = 'apps/admin/lib/customers-client.ts'
b = pathlib.Path(f).read_bytes()
assert b == subprocess.check_output(['git', 'show', 'fbdf8ebb:' + f])
print(hashlib.sha256(b).hexdigest())
```

The parent's retained primary `mou-round/baseline.log` was read directly (tail/hash plus referenced Playwright summary, exit **0**). It contains `TestBrowserMerchantOrdersUIRealChain` PASS and the successful isolated-mode footer; referenced `.../merchant-orders-c-browser/20261008T093555.272733000/playwright.log:17` records **10 passed**. Baseline log SHA256: `e69c5a0fcf85c9175b7f03b1a8ca4b62b3507a0f74a29a90c20f1200b8c4f668`. This is the parent's run, not a reviewer runtime rerun. An initial lookup of this primary log under the worktree returned a path error (exit 2); the correct primary path was subsequently read successfully. No missing-path output is treated as evidence of absence.

Trace timings and the fact that all assertions completed before `orders-ui.spec.ts:493` are attributed to the separate `mou-round/trace-review.md`/parent evidence; this reviewer did not duplicate trace extraction. That record places replacement-body completion shortly before close and static asset starts after close. A response body finishing is not proof of document commit/load. The existing replacement lifecycle may overlap close; it does not establish that the new propagation caused starvation.

Browser/Go/PG/build/full tests/production **NOT_RUN by this reviewer**. No scoped P0/P1 is confirmed by this causal review. No speculative product fix or MOU03 wait/assertion change is proposed. Root cause remains UNKNOWN pending evidence that specifically distinguishes browser close/navigation internals from an actual product execution defect; a single unchanged local pass does not close the intermittent failure.
