# Final G07 source and machine-load evidence

- Source: `4acbad5353814fc3940812e9e399fdb1253ed40f` throughout; only output artifacts changed.
- Command: `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/release-gate.sh --strict --only G07`.
- Start/end UTC: 2026-10-04 04:44:52 / 05:50:00 (65m 08s including orchestration).
- Actual exit: **0**. G07: **6,725 tests/subtests PASS, 0 FAIL** (2,027 top-level PASS).
- G07+: **13 accepted NOT_RUN** entries, not product passes. Exact list: `g07-final/G07.skipped`.
- Foundation package: 3,670.196s. Real PostgreSQL fixture removal is confirmed at the log end.

## Quiet start, measured fluctuation

`g07-quiet-preflight.log` contains three samples 30 seconds apart. Both 1-minute
and 5-minute load were below the machine's 10 logical CPUs before launch.
The task then sampled every 30 seconds; no foreign process or lock was changed.

| Load window | Samples | Minimum | Median | Mean | Maximum | Samples >=10 |
|---|---:|---:|---:|---:|---:|---:|
| 1 minute | 131 | 2.64 | 5.53 | 5.9431 | 20.87 | 6 |
| 5 minutes | 131 | 4.15 | 5.96 | 5.9391 | 8.51 | 0 |
| 15 minutes | 131 | 5.12 | 6.05 | 6.0511 | 6.76 | 0 |

First sample 04:44:52 UTC; last sample 05:49:55 UTC. Raw source:
`g07-machine-load.log`. The process-owned sampler exits with the command wrapper.
This is **quiet-start, load-sampled evidence**, not a claim of continuous idle
or exclusive host ownership. Transient peaks were preserved; no causal
attribution to another task is asserted. All tests passed without threshold
relaxation or a second full G07 attempt on this final source.

The final-source REAL_PG report timing inside G07 was 3.008085583s (100 drafts,
10,000 synthetic collected COD orders), 3.1868625s (101 drafts), HTTP 200 in both
cases under the unchanged 5-second bound. The earlier final-source focused gate
measured 2.561327167s / 2.514677458s. Both sets are retained, not cherry-picked.
