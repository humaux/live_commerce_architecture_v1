<!-- Purpose: index of byte-for-byte W3-U3 final acceptance artifacts and their limitations.
Depends on: native raw artifacts from frozen source4f0e08e5 and completed serial supervisor state.
Used by: integrator verification without treating historical or MOCK results as LIVE. -->
# Final evidence manifest

Source: **4f0e08e5003838d2746ce7dc9c77d1cccf7471e3**. Copied evidence is native and synthetic, not regenerated assertions.
SHA256SUMS.txt lists the bytes of every copied artifact below. Original run directories remain available locally.

| Copy directory | Native source | Observations |
|---|---|---|
| console-1/ |output/playwright/run.20izueha/live-console-4018777712/ and live-console-808004645/|workspace23,labels9;two404 JSONs;9ledgers45PASS;counts7/7 |
| console-2/ |output/playwright/run.9LPrSopL/live-console-2930805195/ and live-console-286510693/|workspace23,labels9;two404 JSONs;9ledgers45PASS;counts7/7 |
| shots/ |output/playwright/run.9LPrSopL/live-console-286510693/labels-*.png|18 PNG:3locales×2widths×3states |
| inbox/ |output/playwright/inbox-ui/20261010T062236.231619000/|13cases,31observedledgerrows |
| sweep/ |output/playwright/run.M88wYZH4/ui-click-sweep/|147pages0loadfail,1135PASS0FAIL31SKIP,18journeysPASS |
| visual/ |output/playwright/run.Wzs7RSvr/ui-visual-audit/20261010T071704Z/|342/342shots,blocking0,R5=21/R7=112warnings |

Five original command logs and state.json are adjacent in ../local-acceptance/restart-20261010T055908059Z/;
all exit0, no timeout, supervisor completed073000Z.
Only safe selected reports are committed; runner.env, ephemeral credentials and server environment files are excluded.
Full342-shot lint corpus and sweep screenshots remain at unique native paths, not duplicated into Git.

Pre-document static checks: node1376/1376, admin tsc, check-gates and contractdrift all exit0.
Post-document contractdrift: exit0, errors=0, warnings362; ../handoff-contractdrift.log (session49111).
Post-document check-gates: exit0, ../handoff-gates.log (session36505).
Staged full check-gates: exit0,82 documented modes,inventory1262,headers OK;
../handoff-staged-gates.log (session1898). No runtime source edits followed these gates.
Supplementary git diff --cached --check exited2 on native .log trailing spaces/CR progress lines;
raw logs are intentionally unchanged, not silently normalized. The same check restricted to source/docs/JSON/TXT exits0.
Artifact reader: w3_final_evidence_audit, test_worker, gpt-6.1-sol/medium, E1 only; no rerun or product edit.
Physical printer, provider LIVE, current required PR CI and K3 are NOT_RUN.

## Process cleanup

Original mode/supervisor PIDs ended. Five orphaned Next servers were found in their owned PGIDs:
40590/43585 (34207),48376/52132 (43721),53034 (52260).
lsof confirmed each cwd inside this worktree's apps/admin/.next/standalone/apps/admin.
Only those explicit PIDs received SIGTERM (exit0); subsequent ps found none (exit1/no rows).
No foreign process/lock or raw evidence was deleted. Native-run artifacts are retained for independent review.
