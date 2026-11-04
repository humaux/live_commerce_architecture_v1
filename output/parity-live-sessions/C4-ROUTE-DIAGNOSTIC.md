# C4 passive document diagnostic candidate

task_id: b7a54aa8-302c-49ed-9490-c15e254e88b6; own canvas: b7a54aa8-sub-1.
base_commit: 005b29c50f35abcd828495de5cabc3e03ca9605e; branch: codex/pr365-mock-balance.
worktree: .worktrees/codex-pr365-mock-balance. Requested Sol/high writer; actual telemetry UNKNOWN.
Root holds coord claim and author lock. No separate claim, delegation, runtime, CI, push or provider action.

## Authority and scope

Root-frozen card output/integrator/tools/logs/ci-evidence-1238/365-route-diagnostic-card-1426.md was read completely and SHA256 recomputed: fcba3d793abc3b203093c121cf4e7a47a63b8b6fb892481a73915868bf690f29. Prior repository AGENTS, architecture0/1/2/26/27, tasks DAG and studio-v1/invariants reads and ponytail/development detail are reused from this worker's immediately preceding task. Humaux targeted search and c4MetadataJournal symbol search were read; no full reread or unrelated scan. Worktree started clean at exact approved base.
Only tests/admin/live-console.spec.ts and this report changed. Product, dirty-fix2/2, fixtures, dependencies, JSON schemas, existing prefix/PhaseB pagination, original selectors/clicks/waits/expectations/throws/final-oracles and all budgets remain unchanged.

## Implemented observer

Only the existing two viewport-generated C4 preparation tests create the helper. After login/before original first Studio goto: addInitScript INIT registration and idempotent current-document LATE fallback, each capped100ms. Five existing entries gain one sample: before-settings-route, preflight-back-start (selected_id retained), after-back-route, before-edit-route and failed-route. No phase is added. Existing caps remain32 journal entries /8 main-frame events.
One nonenumerable/configurable private top-frame Window property owns sample/stop closures and a trusted-pageshow listener only. Existing owned observer is preserved across same document/BFCache; foreign property collision is not overwritten. LATE never invents earlier event history. Observed/count<=4/last persisted/overflow are retained; persisted is omitted when no trusted event was seen. Overflow means incomplete observation. No unload listener, synthetic event, DOM mutation, navigation/history/cache/fetch/router/application call or Next internal read exists.
Each capture makes one synchronous reader evaluation capped100ms, returning only timeOrigin, first navigation entry.type and observer fields into Node. Raw positive finite timeOrigin is transient in the protocol result and case-local Map(max4); only ordinal1..4 is projected. Map overflow/invalid data gives UNAVAILABLE. Navigation type is a closed union with unknown fallback. Serializer independently rejects invalid bounds/literals/observed-persisted combinations as UNAVAILABLE, and never copies raw browser objects.
Total extra awaited budget<=800ms (2setup+5snapshots+1cleanup), original action/test budgets unchanged. Bounded utility observes fulfillment/rejection, clears its timer and discards a late result without assigning ordinal or journal mutation. Timeout cannot cancel a protocol evaluation; max8 calls have their lifetime bounded by the owned page fixture. Incomplete setup is observerInstall UNKNOWN, never a skipped/failed product test. Cleanup attempts only the current owned listener/property, clears Node map, and relies on fixture destruction for historical documents/init script; no invented removeInitScript or page close/navigation.
Original primaryError assignment precedes failure sampling; diagnostic failure does not replace original thrown error. Original route projection and final receipts/zero-write oracle execute as before.

## Static evidence

| Actual command | Exit / result |
| --- | --- |
| git status --short; git rev-parse HEAD before edit | 0; clean, exact005b base |
| node --check tests/admin/live-console.spec.ts | 0; syntax only, no test execution |
| git diff --check | 0 |
| pnpm --filter @live-commerce/admin exec tsc --noEmit --incremental false | 254; tsc not found; typecheck NOT_RUN, no install |
| Inline Python inverse normalization, prefix/cap/five-snapshot checks and SHA256 | 0; full normalized file byte-identical to base |

Base/normalized whole file:143580bytes SHA25634a1f6d4a784d53787e993cee8f3e26f938f6d40a73c1a448fc174698f2edf90.
Untouched prefix before // C4 metadata freeze::109092bytes SHA25607e3bd132fa3ee1152a0d21e91a8d1dd7aeba253b3c844b48dd5062502da7dc1.
Candidate whole file:151468bytes SHA25668fe08d6322d09b96eb7e142f06656ef0514ef247ca40938dc69537bf0c8973c.
This comparison preserves every original statement, not merely the number of assertions. The following is the actual normalization used (run inline, no new script artifact):

```python
from pathlib import Path
import subprocess, hashlib
base = subprocess.check_output(['git','show','005b29c50f35abcd828495de5cabc3e03ca9605e:tests/admin/live-console.spec.ts']).decode()
text = Path('tests/admin/live-console.spec.ts').read_text()
normalized = text
for start, end in [
    ('// C4 document projection begin', '// C4 document projection end.\n'),
    ('    // C4 document whitelist begin:', '    // C4 document whitelist end.\n'),
    ('// C4 document probe begin', '// C4 document probe end.\n'),
]:
    a = normalized.index(start); b = normalized.index(end,a) + len(end)
    normalized = normalized[:a] + normalized[b:]
normalized = normalized.replace('type C4PhaseProjection = C4DocumentProjection & {','type C4PhaseProjection = {',1)
normalized = normalized.replace('  const documentProbe = c4DocumentProbe(page); // C4 document setup owner.\n','',1)
normalized = normalized.replace('  const noteRoute = (phase: string, value: string, document: C4DocumentProjection = {}) => {','  const noteRoute = (phase: string, value: string) => {',1)
normalized = normalized.replace('      ...document, // C4 document projection is validated again by the serializer.\n','',1)
normalized = normalized.replace('    await documentProbe.install(); // C4 document setup: two bounded calls, never retried.\n','',1)
for phase in ['before-settings-route','after-back-route','before-edit-route','failed-route']:
    normalized = normalized.replace(f'noteRoute("{phase}", page.url(), await documentProbe.capture());',f'noteRoute("{phase}", page.url());',1)
normalized = normalized.replace('diagnostic.note("preflight-back-start", { selected_id: url.searchParams.get("scene"), ...await documentProbe.capture() });','diagnostic.note("preflight-back-start", { selected_id: url.searchParams.get("scene") });',1)
normalized = normalized.replace('    await documentProbe.stop(); // C4 document cleanup: one bounded call; fixture owns historical documents.\n','',1)
assert normalized == base
prefix = lambda s: s[:s.index('// C4 metadata freeze:')]
assert prefix(text) == prefix(base)
assert text.count('await documentProbe.capture()') == 5
assert 'entries.length >= 32' in text and 'navigationNotes >= 8' in text
print(hashlib.sha256(normalized.encode()).hexdigest())
```

## Evidence limits and handoff

E1 syntax/static implementation only. Actual callback coverage, timing/late-result behavior, typecheck/build, both original viewport paths with unchanged whole-mode zero-write oracle, browser/Go/PG and independent acceptance NOT_RUN. No fabricated event or mock browser self-test was used as event proof. Root's already-running fixed005b CI is unrelated to this local candidate and was not touched.
Instrumentation can perturb timing or BFCache eligibility. Instrumented PASS does not repair or explain historical #preparation#edit; same ordinal alone does not prove BFCache, different ordinal only supports document recreation, trusted persisted=true supports that document's page-cache restoration, and none proves Next cache content/canonicalUrl/hydration cause. Missing/timeout/overflow retains UNKNOWN. No product/sourceownership/helper third repair is included.
No local process/server/container/port/fixture/background job was started; no temporary cleanup is needed. Root performs independent diff/normalization review, derives ownership, pushes and runs exact-SHA acceptance; writer stops after local commit.
