# Superseded-source scan stopped, 2026-10-05 07:29 UTC

Source stayed `1ded587749e5718f78c3fff1e2233a0ffd7321c7` throughout the 31 completed modes in results.tsv. Six modes are RED (product editor, CVS, legacy inventory, merchant-orders UI, catalog media, WebKit CVS); their logs remain unchanged.

The remaining full click sweep was stopped deliberately after bounded fixes were reviewed, to run the expensive final scans on the corrected source instead. It is **INTERRUPTED / not a completed acceptance run**, not PASS. Frozen visual lint for this particular source was **NOT_RUN**. Earlier complete green visual evidence belongs to `5a895dea` only.

Resolved exact owned PID/cwd chain: evidence runner 84498 -> test-local 16234 -> go test 16502 -> foundation test 16511 -> Node wrapper 16539 -> Node runner 16540, all this worktree. Parent 84498 was suspended; only owned Node 16540 was sent TERM. Go/test-local finished naturally (exit 1, including expected missing unfinished journeys), running their normal fixture cleanup. PIDs 16234/16502/16511/16539/16540 were then confirmed absent. Parent 84498 was terminated/resumed only after those exits; terminal session 98950 returned 143. Parent PID was also confirmed absent. No foreign process or test lock was touched.

Click log: `browser-click-sweep.log`; detailed partial log: `output/playwright/click-sweep/20261005T072240.082807000/click-sweep.mjs.log`. Missing journey errors after the deliberate stop are not classified as new product defects.
