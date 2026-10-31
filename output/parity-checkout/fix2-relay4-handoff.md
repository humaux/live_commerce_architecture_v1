PARITY-CHECKOUT-FIX2 RELAY4 — BLOCKED by the explicit same-test stop clause; no third approach.

PR #351 remote head `7c2e866c3270c24ed978b3d7edf60a2c72001fae`; local inherited merge `a07ddd930c04e98599418ad7e13dabc37ea071fc` stays unpushed. Fresh integration629f42152 is already merged (`Already up to date`).

Current regression RED: Gates41105720054/job124237319015, TestBrowserHomeCod. Original current-head artifact13003475702 raw home-cod-buyer.mjs.log29–43: `COD-255 gap-up: server due refresh required`, expected true/received false, predicate timeout5000ms at home-cod-buyer.mjs255. Six earlier card re-quote cases passed, but the same containing test remains RED. Static diagnosis E0: initial card selection skips COD due refresh in OrderFlow. No further implementation under this card.

Fresh checks also show sweep5/10 job124237319091 (retained receipt: artifact-download failure, NOT_RUN), sweep7/10 job124237319763 (UNKNOWN assertion), visual3/4 job124237319491 (UNKNOWN; receipt counts0), WebKit124237318093 (UNKNOWN). No base-identical failure claimed. Native log request now says workflow still in progress and logs available when complete. Full inventory is in output/parity-checkout/FIX2.md RELAY4.

Causal tests-only RED41051681920/job124052345726 (new parity assertions). GREEN absent. Foundation41105719747/job124234877774 passed for remote head only (E3 for Foundation). Author diagnosis E0; whitespace static check exit0 E1.

WAIT_RUN: 41105720054 (existing Gates workflow, no new push). Integrator owns remaining completion relay and next disposition. No >10-minute polling.

Only card evidence changed this relay; outside-card paths: none. Existing dirty FIX1.md/FIX2.md and artifacts preserved. No source/test edits, commits/pushes/reruns, merge-to-integration, auto-merge or production. Humaux problem stored and own canvas updated under acquired lock.

NOT_RUN: whole-PR GREEN, CI on unpushed local merge, independent E4, local runtime tests/servers (prohibited), provider/production execution.
