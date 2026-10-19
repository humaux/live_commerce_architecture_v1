<!--
Purpose: GitHub-run receipts for PR #81 (setup-go cache only on shard:gNN, key go.mod+go.sum): cold vs warm vs the 735cb2ce2 baseline.
Depends on: jobs-<run>.json (raw GitHub jobs API, foundation legs only) and pretest-<run>.tsv (from each job's log) in this directory.
Used by: the PR #81 reviewer/integrator deciding keep vs revert to `cache: false` (review-735cb2ce2 P2-1).
-->
# Go compiler cache: receipts (LIVE GitHub runs, foundation-shards set, 11 jobs)

Pre-test = Gate step start to the first `=== RUN` line in the job log. That window is compile plus link, i.e. what the cache can save.

| Run | Head | Cache | Longest Gate | Job-min | Σ g01–g10 Gate | g-shard pre-test |
|---|---|---|---|---|---|---|
| 38947914663 (baseline, PR #78) | 735cb2ce2 | 37 MB ui-gallery entry, all hit | 1042 s | 177.0 | 9610 s | 89–130 s |
| 39204046804 (cold, dispatch) | 09a3944b8 | miss; g07 saved 199 MB | 1279 s (g09) | 177.9 | 9721 s | 80–135 s (g04, which started late and hit: 4 s) |
| 39209457669 (warm, dispatch) | 09a3944b8 | 199 MB hit on all g-shards | **914 s** | **158.3** | **8419 s** | **3–5 s** |


PR-scope sample, PR run 39204047209 (09a3944b8, full 76-job matrix, all green): g01–g09 hit the 198 MB entry (pre-test 3–5 s, Gate 723–914 s); g10 missed, compiled cold (pre-test 96 s) and saved; shard:unit stays uncached (78 s).

- Warm vs baseline: longest Gate −128 s (−12%), job-min −18.7 (−11%), Σ g-shard Gate −1191 s, i.e. ≈119 s per shard. That matches the drop in pre-test time, so the cache is the cause.
- Each shard still rebuilds the cmd/* binaries its own tests `go build` at runtime (review P2-a); pre-test excludes those, so they sit inside Gate.
- Restoring 199 MB costs about 3–8 s more in the setup-go step. That is small next to ~110 s of compile.
- shard:unit is uncached by design (a different build graph), so 463–612 s is unchanged and is noise.
- Ceiling: g-shards are still floored by about 700–900 s of real-clock tests. Further wins come from shard balance, not caching.
- Limits: one cold and one warm sample. The real-clock variance (g09 at 1279 s cold) is the same order as the effect on wall time. The pre-test column is the stable signal.
- Steady state: push runs on r3/integration seed the base scope. PR runs then restore it by exact key and recompile only the packages they changed.

Verdict: measurable benefit, so keep the cache (no revert to `cache: false`).
