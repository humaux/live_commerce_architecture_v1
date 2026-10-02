# W0 legacy baselines

Captured before replacing WorkspaceFrame: 23 routes × 1586×992 / 390×844, zh-CN. See `manifest.json` for hashes and rendered text.

Tier: **MOCK**. The session and two stores are synthetic, list resources empty. Unsupported backend fixtures deliberately show unavailable/detail error states; these are not healthy populated business-flow baselines. The shipping-print capture is the invalid-link state and the invitation is an unaccepted synthetic token. Existing signed Go/PG browser suites remain the business-flow regression authority.

Reproduce (before shell migration): `node tests/admin/shell-runner.mjs --baseline`. Never regenerate these against the replacement shell. Owner/integrator human baseline review is a separate handoff check, not asserted by capture success.

Source baseline: b4223c8; the money/time move performed during capture is output-equivalent (all 7 stop-bleed unit assertions pass) and did not change the shell or page markup.
