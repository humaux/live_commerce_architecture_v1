<!-- Purpose: Record the integrator-requested union merge of PR8 inbox into PR5, not new product behavior.
Depends on: d90991b3 LC-U1 batch and origin/r3/integration f2ac619f; shared mode registry and current static/compile gates.
Used by: integrator pre-review/push and subsequent required PR CI. -->
# PR5 / PR8 union merge

- Started clean at `d90991b32357f118798b2fa3275bc72ea2d12b6e`; fetched and merged `origin/r3/integration` **f2ac619f15be832475b37ffabd4bbd497e5c4bb3**. No rebase or force push.
- Three conflicts resolved as unions:
  - `scripts/dev/test-local.sh`: preserve **--browser-inbox** and **--browser-live-console** in validation, usage and admin-build lists. Both executable dispatch branches remain. Parent-mode inventory comparison: **79 + 79 → 80 unique modes, zero omissions** (`registry-union.log`).
  - `apps/admin/next.config.ts`: combine descriptive headers. Existing CSP/external-Facebook-link behavior and incoming M7 `no-referrer` override both remain; no policy choice was needed.
  - `tests/foundation/browser_click_sweep_test.go`: retain the LC-U1 comment bridge/stream plus the inbox keyring/service/templates/send setup; the one router options object includes **CommentStream, Inbox, MsgTemplates**. Real empty-inbox route validation from trunk remains.
- GATES.md and the live-console contract auto-merged; both browser mode rows and inbox calibration guidance remain. Other incoming PR8 changes are retained. The stock-edit fix and ops-polish spec from d90991b3 are unchanged.

## Checks on merged source

| Command | Exit / result | Evidence |
| --- | --- | --- |
| Parent usage-list union assertion | 0; 80 modes, missing=[] | `registry-union.log` |
| `LC_HEADER_BASE=d90991b3 bash scripts/dev/check-gates.sh` | **0; 80 modes, all documented; every tracked test file is run** | `check-gates.log` |
| `bash scripts/dev/test-node.sh` | **0; 815 PASS / 0 FAIL**, summed runners | `node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc.log` |
| `GOTOOLCHAIN=go1.27.1 go test -tags browser -run '^$' ./tests/foundation` | **0; compile only, no tests run** | `browser-compile.log` |
| `gofmt` on conflicted fixture; `bash -n scripts/dev/test-local.sh`; diff whitespace check; unresolved index check | 0; no unmerged paths | author receipts |

No browser/PG suite or provider/production action was run for this merge. **Previous ops-polish/live-console runtime results belong to de968f18/d90991b3 and are not relabeled as results on this merged source.** Fresh required CI/K3 review remain pending; the ignored old serial foundation job is not a new acceptance requirement. Optional Node R04 remains NOT_RUN without its configured binary. E3 covers registry/Node/static assertions; browser-tag compilation is E1 only.

Source SHA-256: `next.config.ts` 6117b0410b0c4a6fefab905b7907f3dffcb1f32ecde3208e5bbdcef0d3c6f9d1; `test-local.sh` b75e39f129c24bf0caab71d99c00113a33ade2b13b4dce605d12a6f5bdceb96d; `browser_click_sweep_test.go` 78f02c945f6bdd4d6b930e79dcf4074a727b86e94eabdc4a62a06825b27e7a84; `GATES.md` 237ecefa3e522ada7025e7dccd1cdc8201c39e47cc2de36822d983347613ca9f.

Commit locally and stop; integrator owns push and PR acceptance. No owned background process remains.
