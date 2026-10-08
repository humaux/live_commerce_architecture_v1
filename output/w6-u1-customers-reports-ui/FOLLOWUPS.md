<!-- Purpose: preserve deferred PR3 review items under the post-round-one convergence rule.
Depends on: integrator packet output/integrator/triage/pr3-a83e2aea.md and the explicit batch scope.
Used by: integrator and future W6 UX/layout work; not a waiver for failing current gates. -->
# W6-U1 follow-ups

| Comment | Priority | Deferred work / acceptance |
|---|---|---|
| 4212542044 | P2 | Scope report column layout independently of global product-table fixed widths; compare wide report tables at 1280/1281/1440px. Current CI report failure was HTTP503 on CSV, before the width check; no causal layout evidence in this round. |
| 4212542053 | P2 | Contain wide-table scrolling within shrinking report panels at 320/390/768px. Current CI failure did not reach the width check; preserve the scrollable table and never clip values with page overflow:hidden. |
| 4212542070 | P2 | Bring tag rename/delete controls and focus into view for long directories, with cancel restoring the trigger. Explicitly deferred by integrator. |
| 4212542075 | P3 | Keep long-note editing and feedback near the target, preserving CAS, draft and same-key UNKNOWN behavior. Explicitly deferred by integrator. |

4212542034 (bodyless DELETE) and 4212542061 (applied report dates, data-correctness P1) belong to the current repair batch, not this queue. Known browser-identity readiness flake remains assigned to Codex-1.
