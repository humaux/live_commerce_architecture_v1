# BUYER wiring — STOP / partial structural baseline

2026-10-29 · DESIGN · **E1: structural baseline only. Card incomplete. Author conclusions E0; E2/E3/E4 NOT_RUN.**

## Stop condition

Fleet Bridge control-read receipts were truncated at approximately 64 KiB although the CLI returned exit 0. Two targeted fixes were attempted: 10 roots → 3 roots, then 1 root with compact reactions. The final attempt read 219 of 299 product roots, then returned exactly 65,536 bytes of invalid JSON for `1429:14166` (依地區/list loaded). Per card, stopped after the second unsuccessful fix. No Figma mutation, uncertain-write replay, bridge restart, visual/copy edit or frame deletion occurred. Resume requires the integrator to assign a fresh bounded plan (for example, paginate descendants within one root); no such follow-up was executed.

## Authority and ownership

- Current dispatched LANE BUYER card; original `output/integrator/tools/cards/cxf-wire-buyer.md` read from main checkout, read-only.
- Read `AGENTS.md`, `架构.md` §§0/1/2/26/27, Fleet Bridge README, figma-use skill, Plugin API index/reaction typings/gotchas, local `output/ux-audit-ref/FIGMA-GUIDE.md`, owner checkout/simplify cards.
- Assigned worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-cvs-match`; branch `design/ux-cvs-match`; base/head `68eb47046714790200e320129787ab51e77309ef`. Role: dispatched Buyer wiring implementer. Actual runtime model/effort identifiers not exposed; no delegation.
- Figma default `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`. Bridge connection `554b0088-8b4f-4e95-a16b-1bbd25cdc369` reports Untitled/fileKey:null; identity relies on configured default and known node/page IDs, not direct file-key confirmation.
- Fresh root inventory: 299 non-SPEC Buyer Desktop/Mobile product FRAMEs, including `DESIGN ·` / `DESIGN v2 ·` prefixes. SPEC explanation sheets excluded from product navigation; names containing 已被取代 / 非主路徑 untouched. Other lanes untouched.
- Humaux search/get/rejected/coord/store/canvas unavailable; no shared claim/store/update claimed. Integrator must persist this handoff. Higher-priority local memory guidance was consulted only for scope/readback discipline; current figures come from live reads.

## Before / after numbers

|Measure|Before|After|
|---|---:|---|
|Buyer Desktop from desktop home|41 / 148|NOT_RUN|
|Buyer Mobile from mobile home|44 / 151|NOT_RUN|
|All active product frames, original reach predicate|1189|NOT_RUN|
|Reached across original entry set|133|NOT_RUN|
|CHANGE_TO, bounded original wire algorithm|0|NOT_RUN|
|bad, bounded original wire algorithm|0|NOT_RUN|
|Strict invalid NAVIGATE targets|0|NOT_RUN|
|Written sources|0|0|

These are structural graph observations, not real prototype clicks or application acceptance. `reach-before.json` lists all unreached frames by side/module and per Buyer platform. `evidence/before-summary.json` retains the wire audit's 1,235 active FRAME count (including documentation); the original reach predicate additionally excludes SPEC and pure-text frames, yielding 1189 product frames. Supplied whole-page scripts were read but not executed unbounded: adapted existing `scan.py` reads at most 10 roots per call, then aggregates the original predicates/actions locally. No audit predicate or shared tool was edited. A full supplied-tool run remains NOT_RUN.

## Every written source → destination

None. No `setReactionsAsync` call was made. `controls-before.json` was not produced because inspection stopped; `evidence/controls-000.json` through `controls-218.json` are the successful final partial read set. `controls-219.json` is intentionally retained invalid failure evidence. No proposed links were applied.

## Evidence / commands

- `python3 output/figma-wiring/buyer/scan.py before`: exit 0, requested read-only Figma audit, not an application test. Original reach aggregation saved in `reach-before.json`.
- `python3 output/figma-wiring/buyer/inspect.py`: exit 1, twice reduced as described in `failure.log`; JSON truncation remains. No test/server started.
- Fleet Bridge `figma_status`, shallow root enumeration and `get_metadata`: exit 0. A shared bridge uncertain-outcome marker was encountered before the first baseline batch; no write was retried, `get_metadata` of `4:9192` cleared the required inspection, and read-only work resumed. Final status idle / needsInspection=false in `evidence/status-final.json`.
- Main checkout read-only. New local paths only `output/figma-wiring/buyer/**` and `output/figma-wiring/buyer-VERIFY.md`; pre-existing artifacts untouched. No application source, headers, dependency map, generated ownership or gate files changed.
- No commit, push, PR, merge, auto-merge or CI status. PR: NOT_CREATED. CI RED/GREEN run/job IDs: NOT_RUN. WAIT_RUN: none, no push. No own background process remains.

## NOT_RUN / handoff

Remaining control inspection (80 roots); all wiring; after audit; exact supplied full-page acceptance; real Figma clicks; browser clicks/persistence; CI RED/GREEN; copy lint; locale/fact parity and density revalidation; independent review; Humaux persistence/canvas update. Safety, production and delivery claims are not inferred from the readback. No rollback needed because no Figma writes occurred; original partial read receipts retained for review, not a restorable full backup.
