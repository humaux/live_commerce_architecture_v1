# Red-run evidence only

Files in this directory are preserved failed-run diagnostics, not final acceptance screenshots.
`pre-closeout-failure.*` came from the earlier shell implementation and were superseded by its green handoff.
`supplement-red-*` proves the integrator's store-brand/session-context assertions failed before the supplement fix.
Final screenshots live one directory above; exact green/red commands and exit codes are in SUMMARY.md.

`run-1790948019481/failure.*` is the intermediate 17f1b842 run: its new unknown-store test
waited for a mounted-shell 403, but the server correctly returned 404 before mounting.
a6449fb3 separately asserts the server 404 and the mounted-shell invalid-store 403.
This archived screenshot is not the final 403 visual evidence.

`ledger-scroll-widths.json` preserves the final-source measurement for the still-blocked
legacy scrolling assertion. No tested width caused overflow; `scrollWidth === clientWidth`
throughout. No overflow was injected and the original assertion remains unchanged.
