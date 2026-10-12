# W3-U2 independent review

**VERDICT: PASS for the bounded source review at `115bd20251213e8be8fd20e149ff89f2a37754d8`. No unresolved confirmed P0/P1/P2 in the reviewed scope.** All three reported P1s were fixed. This is not independent browser/PG/full-gate acceptance.

- task_id: `662ad08e-sub-review`; parent: `662ad08e-b312-4b1f-bbf8-f2ffdb25d14d`.
- base_commit: `894990131cd7e13fe907693e295f64d99087df15`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u2-live-settings-ui`.
- Reviewer: independent read-only reviewer; assigned gpt-6.1-sol/high from dispatch history; exact executing model identifier UNKNOWN.
- Read the primary `RULINGS.md`, AGENTS/preamble/PROCESS, feature source and dependency/privacy/proxy follow-ups. No source edits, commits, PG/browser/build/full-suite execution or recursive delegation. This report is the only authorized output-file write.
- Reviewed paths: LiveSettings/BuyerPanel/CommentStream, live-settings controller/client/model/proxy and dedicated BFF leaves/page, Next proxy routing, sold-out HTTP adapter/claims wrapper, and relevant acceptance tests. The LC-U2a dependency privacy merge is included in the reviewed lineage.

## Closed findings

1. **P1 nested restriction 404 did not revoke the parent private views.** Original `BuyerPanel.tsx:217–221,362–365` only cleared the panel; a restriction check first returning 404 left CommentStream's A2/A8 authority live. The corrected child treats 404 exactly as 401/403, expires its fence and calls the outer denial callback; the outer callback expires BuyerPanel and invokes CommentStream's `onUnauthorized`. The partial `onMissing` path was removed. Parent actual-component RED `false != true` and late-A8 counterexample were retained.
2. **P1 UNKNOWN allowed an old fresh-key callback to replace its receipt.** Original controller `run()` at lines 245–249 unconditionally replaced the retained receipt after `inFlight` became false. The corrected admission allows only the existing receipt object while UNKNOWN is retained. Explicit retry therefore retains method/resource/body/key; a captured fresh trigger is refused. Parent actual-controller RED `2 != 1` was retained.
3. **P1 stale callbacks could dispatch after privacy suspension.** At `9c00233e`, `run()` checked a render-time `visible` value while `suspend()` had terminally revoked the fence and cleared refs. A captured old callback could obtain a new non-aborted signal and POST despite departure. Current `run`, pagination and BuyerRestriction `add` check `fence.current(ticket)` before refs/state/network side effects. The exact independent actual-hook counterexample below changed from RED to GREEN.

The bounded review found no additional confirmed issue in the real signed-session BFF/Origin/CSRF/store grammar, backend permission enforcement, or template publication/CAS chain. Publication uses a merchant template receipt before the separate settings command; fixed templates remain read-only, and settings Resolve/Set run inside `command.Run` so replay precedes a newly stale CAS. BFF receipt validation refuses mismatched successful replies as UNKNOWN. Manual reminders remain manual; no unsaveable automatic controls were introduced.

## Independent command and exact result

This same command was run from the author worktree before and after the third fix. It reuses the existing actual-source hook host and production controller/client; only the Node network edge is mocked. It writes no source/test file and runs no browser or PG.

```sh
node --experimental-strip-types --input-type=module <<'NODE'
import test from 'node:test';import assert from 'node:assert/strict';import {registerHooks} from 'node:module';
import {environment,store,response} from './tests/admin/inbox-review-host.test.ts';
registerHooks({resolve(s,c,next){if(s==='next/navigation')return{url:'data:text/javascript,export function useRouter(){return {push(){}}}',shortCircuit:true};return next(s,c);}});
const {useLiveSettingsController}=await import('./apps/admin/lib/live-settings-controller.ts');
test('stale fresh callback after navigation suspension must send zero writes',async t=>{
 const env=environment(t);let writes=0;
 globalThis.fetch=async (_i,init)=>{if(init?.method==='POST'){writes++;return response({code:'unavailable'},503);}return response({items:[],next_cursor:''});};
 const h=env.mount(()=>useLiveSettingsController({store,locale:'en',scene:'',initialError:null}));await h.settle();
 const old=h.output.run;h.output.privacy.suspend();await h.settle();
 await old({method:'POST',resource:`live-sessions/${store.id}/reminders`,key:'stale-after-departure'});await h.settle();
 console.log(JSON.stringify({visible:h.output.privacy.visible,departing:h.output.privacy.departing,writes}));
 assert.equal(writes,0);
});
NODE
```

- At `9c00233ef675a2850e98019da5531c66a22cd3f0`: **exit 1**, 0 PASS / 1 FAIL; `{"visible":false,"departing":true,"writes":1}`, assertion `1 !== 0`.
- At `115bd20251213e8be8fd20e149ff89f2a37754d8`: **exit 0**, 1 PASS / 0 FAIL; `{"visible":false,"departing":true,"writes":0}`.
- `git diff`, source/hash reads and current-vs-commit byte comparisons exited 0. No assertion was weakened to produce the green result.

## Retained parent evidence read directly

Primary `output/w3-u2-live-settings-ui/`:

- `restriction-404-red.log`: actual full CommentStream child-check 404/late A8 RED; SHA256 `4b0736e237e91bc47b76b411f4a57ad5f0c2e6adf2c8755ab43cc61df53f01df`.
- `unknown-red.log`: actual controller fresh-key admission RED; SHA256 `48cfbd95358103d4c6fc62cb25e5d0cc6f37bacbfc4b225692c6e564ea8ede0d`.
- `review-green.log`: 20 PASS / 0 FAIL for the first two fixes and dependency privacy cases.
- `departure-green.log`: 24 PASS / 0 FAIL, including controller hide/departure/unmount and BuyerPanel-add hide; SHA256 `7f2e19ea95b7f420885291e8f4c7045935175c08896bdfc063e6d20995eb26b7`.

These suites were run by the parent and inspected here, not independently rerun in full. The parent's earlier browser 12/12/Node/typecheck/gates results bind `9c00233e`; they are not silently promoted to the changed `115bd202` bytes. Current final browser/gates remain the parent's separate acceptance responsibility.

## Final source binding and limits

- `apps/admin/lib/live-settings-controller.ts`: `cc829f7269a24a319213f204e9df5914720f602d4f51b54dafdd28fe9ed17b02`.
- `apps/admin/components/BuyerPanel.tsx`: `06b77c03b4b06a440ad943f3611a1454b3c189529791ae949223b2e75f3fdcf9`.
- Existing signed BFF/Next proxy/backend adapter were also read; no unreviewed privilege or command-policy expansion is asserted.

Overall review: **E1 source inspection**, with independently executed **MOCK actual-hook RED→GREEN** for the stale-dispatch regression. Reviewer browser/PG/full Node/build/CI/live provider/production: **NOT_RUN**. No unresolved confirmed scoped finding; final runtime and integration gates remain separate.

Owner priority switch: W3-U2 is paused at `115bd202` while the parent moves to Stripe PR20. The final browser run queued as parent session `56460` is not accepted evidence; the parent owns stopping it if still waiting. Current-commit full runtime/CI acceptance is pending/NOT_RUN, not PASS. This bounded source review and its exact independent Node reproduction are complete; this reviewer has no running process and starts no further W3-U2 work.
