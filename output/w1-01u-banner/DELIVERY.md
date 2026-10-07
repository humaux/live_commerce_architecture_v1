<!-- Purpose: W1-01U current CI fixture/UI repair delivery and acceptance limits.
Depends on: r3/integration71235, CI37470022946 traces, actual SSR and focused mobile browser evidence.
Used by: integrator review and GitHub rerun; no live Meta/provider acceptance. -->
# W1-01U CI repair delivery

- Branch `unit/w1-01u-banner`; trunk71235fc4 merged as879be028 before checks. UI worker checkpointfed04ccdad3cddd6648f27daa69cc8345ac98be1; final commit is the commit containing this file (actual SHA in handoff/receipt).
- Supersedes authora8b20776 / int01cb5ded run37470022946. Parent/worker Codex GPT-6, independent read-only source review. Source binding `60d09e787d3d2e9b94984c5511161796651c1c58fc293644ca4f0f5118498b1a` covers18files in `ci-37470022946/source-manifest.json`.

The restriction assertions saw real `DESIGN/ok_snapshot` rows because the test profile wrote connectionstatusactive again **after** the real probe. Migration0125 deletes capability rows on every active UPDATE, including no-ops; the fixture's following UPDATE affected0rows, and B1 legitimately fell back. The fixture now avoids redundant post-probe activation, preserves the blocking transition, and asserts8persisted named rows. UI badges still reflect actual backend state; no forced restriction or relaxed assertions.

The en390 advice test failed its no-overflow measurement before navigation: the full expired/revoked explanation was inside a shared nowrap Badge. MetaConnect now allows that one sentence to wrap, preserving the full text and shared Badge defaults elsewhere.

## Checks and evidence

All paths below are main `output/w1-01u-banner/ci-37470022946/`.

| Check | Exit | Evidence |
|---|---:|---|
| original CI red |1| run37470022946 trace/context; captured actual responses in worker/captured-health-responses.json |
| actual MetaConnect+Badge SSR3locale red→green |1→0| worker logs,11PASS/3FAIL→14PASS/0FAIL; worker/SOURCE-CHECKPOINT.md |
| parent Node model/SSR |0| node-green.log14PASS/0FAIL |
| admin TypeScript / production build |0| types.log / build-green.log |
| configured focused Go/browser command below |0| focused-green-configured.log Go1PASS/0FAIL/0SKIP; actual Playwright6PASS,5intentionally unselected cases NOT_RUN |
| check-gates / diff-check |0| gates.log72modes/header ratchet; clean diff |

Focused command:

`LC_TEST_LOCK_WAIT=600 LC_META_HEALTH_EVIDENCE_ROOT=/Volumes/data/live_commerce_architecture_v1/output/w1-01u-banner/ci-37470022946/local-focused LC_BROWSER_META_HEALTH_UI=1 LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=700s bash scripts/dev/test-focused.sh '^TestBrowserMetaHealthUIFocused$'`

Real browser6/6: zh-TW/zh-CN/en at390, namedFB/IG review restriction and blocking advice settings-card link. The8-row fixture assertion passed. Full durable run at `local-focused/20261006T150532.380319000/`, Playwright6passed11.3s; Go17.530s. First unconfigured invocation failed before browser for missing existing LC_META_HEALTH_EVIDENCE_ROOT (`focused-green.log`,exit1), then correct setup succeeded; no source change for that retry.

Separate focused Go entry supplies an explicit subset flag. Full `TestBrowserMetaHealthUI` supplies emptyflag and retains all original cases/assertions. Five desktop/recheck/viewer/store-switch/reconnect cases intentionally NOT_RUN in the subset and do not count as PASS. Independent source review found no fixture state loss, authority widening, or full-gate weakening.

E3 applies to focused real browser/realPG with MOCK IdP/Graph and actualSSR; full W1 acceptance still requires CI. Strictly serial with W3 PG; no foreign process killed or lock bypass. No backend runtime/SQL/schema/deps changes, release merge/push or real Meta traffic.

## CI gates / NOT_RUN

Integrator pushes/runs:

- `bash scripts/dev/test-local.sh --browser-meta-health-ui`
- `bash scripts/dev/test-local.sh --browser-meta-connect`
- `bash scripts/dev/test-local.sh --browser-click-sweep`
- `bash scripts/dev/test-local.sh --browser-visual-lint`

All full modes above and five unmatched focused-spec cases are NOT_RUN locally this repair turn. LC-U1 findings belong to another thread and were not edited. Prior feature evidence remains under main output; current repair supersedes previous author SHA.
