# meta-connect independent gates — evidence summary (test author: Kimi K3, 2026-10-01)

Worktree `.worktrees/meta-connect-tests`, branch `unit/meta-connect-tests`. Contract: contracts/meta-claims-intake-v1.md
"Merchant connect (R4)" amendment + §7; brief docs/delivery/units/meta-connect.md. All Meta traffic is the loopback MOCK
(tests/metaconnect/fakegraph); nothing ever dialed graph.facebook.com.

## Gates, red/green

| Gate | Red run (mutation, reverted) | Green run | Counts |
| --- | --- | --- | --- |
| PG `TestMetaConnectGate*` MCG01–MCG07 (`bash scripts/dev/test-focused.sh '^TestMetaConnectGate'`) | pg-gate-red.log exit=1 — mutations: (a) compose api mounts the v2 private ring → MCG07 compose guard red; (b) 0095 `missing_permission` raises no-oped → all 9 MCG02 refusal cases red | pg-gate-green-final.log exit=1 | green run: PASS=6 FAIL=1 SKIP=0 (the 1 FAIL is product defect D1, kept failing); red run: PASS=4 FAIL=3 |
| MCG09 model `tests/admin/meta-connect-gate.test.ts` (node, via test-node.sh) | mcg09-model-red.log exit=1 — mutation: `safeDialogURL` host pin removed → "dialog URL is facebook.com only" red (fail 1/5) | mcg09-model-green.log exit=0 | 5 pass / 0 fail |
| MCG08 preflight `tests/deploy/meta-connect-preflight.test.mjs` (node, via test-node.sh) | mcg08-preflight-red.log exit=1 — mutation: preflight P08 CONFIG_ID grammar `[0-9]{1,40}`→`.*` → 5 numeric-grammar cases red | mcg08-preflight-green.log exit=0 | 25 pass / 0 fail |
| Node umbrella `bash scripts/dev/test-node.sh` | — | test-node-green.log exit=0 | 255 pass / 0 fail (r04-input-runner NOT_RUN, accepted: no pinned binary) |
| Browser MCG10 `--browser-meta-connect` (`TestBrowserMetaConnectGate` + spec tests/admin/meta-connect-gate.spec.ts) | NOT_RUN | NOT_RUN | environment blocker below |

Also: node-gates-first-run.log = both new node files green on first run (30/30). pg-gate-green-1/2.log = intermediate
iterations while fixing test bugs (idempotency-scope assumption, 10-minute interval strictness, 0095 CHECK on expires_at,
compose service names, no-session 401-vs-422, ads-worker's own ADS ring tripping the page-ring regex).

## Browser gate NOT_RUN (environment, not product)

`bash scripts/dev/test-local.sh --browser-meta-connect` (browser-gate-green-1.log) fails at `next build`:
this worktree has no `node_modules` and this session may not run `pnpm install` (command not permitted) or read outside the
worktree, so the admin/storefront Next builds cannot be produced here. The mode wiring (test-local.sh) and registrations are
in place and the node pre-gates of the mode pass; the Go runner compiles only under `-tags browser`, which this session also
cannot select (env-prefixed commands not permitted; ambient GOTOOLCHAIN=local pins go 1.26.2 < go.mod's 1.27.1). Every symbol
the runner/spec uses was checked against its definition by grep (mabStartAdmin, brfPlaywright, metaconnect.New, httpapi.Options
fields, live.CreateDraft, claims.NewLabelKey, copy/testid vocabulary). Re-run on a machine with deps installed:
`bash scripts/dev/test-local.sh --browser-meta-connect`.

## Registration

`bash scripts/dev/check-gates.sh` → "ok (53 modes, all documented; every tracked test file is run)".
- test-node.sh: + tests/admin/meta-connect-gate.test.ts, tests/deploy/meta-connect-preflight.test.mjs
- test-local.sh `--browser-meta-connect`: + refuses a missing gate file, runs meta-connect-gate.test.ts first, then
  `TestBrowserMetaConnect` and `TestBrowserMetaConnectGate`
- GATES.md: `--browser-meta-connect` row names the independent halves; spec table row; Node suite table rows MCG08/MCG09;
  focused-PG table row MCG01–MCG07.

## Defects

See DEFECTS.md: D1 one-store-one-Page race in `meta_connect_finish` (kept failing by policy).

## NOT_RUN

- Browser gate MCG10 (environment blocker above), incl. its red run.
- Meta SANDBOX/LIVE and App Review (per contract, always NOT_RUN here).
- Evidence copy to the main checkout's output/meta-connect/ (PROCESS.md §4): this session cannot write outside the worktree;
  the committed kimi-evidence/ directory is the record.
