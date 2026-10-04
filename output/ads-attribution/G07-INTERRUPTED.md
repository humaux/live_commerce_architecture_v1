# G07 checkpoint: interrupted, not full acceptance

- Command: `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/release-gate.sh --strict --only G07`.
- Compiled checkpoint: `3dab4b9ff854`; release runner exit **1**.
- Durable raw log: `/Volumes/data/live_commerce_architecture_v1/output/release-gate/20261003T215312Z-3dab4b9ff854/G07.log`.
- Start load sampled approximately 3; later samples approximately 4–10. No claim of continuous load sampling.
- Partial emitted top-level Go events: **1336 PASS / 142 FAIL / 2 SKIP**. These are not a complete suite count.
- Genuine early failures: Begin rejects legitimate orders with PT404; frozen CB02 detects redundant customers-domain grants; historical pre-0113 fixture cart writer lacks claim_line_version.
- Author error: changing 0113 while this run was active caused one subsequent `migration checksum mismatch: 0113_ads_attribution.sql` in the populated KC03 upgrade check. That failure is **not a product root cause**. The source was no longer frozen; the run cannot be accepted as a stable-source gate.
- The verified task-owned foundation PID 70933 (parent 67411, task binary under go-build1531092952) and its then-current descendants 77946/77947 were sent SIGTERM. No foreign process or lock was touched. The release wrapper subsequently exited 1 and its EXIT trap removed its labelled ephemeral PG fixture `lc-foundation-test-67352`.
- Postflight: own runner/test PIDs absent; `docker ps -a --filter name=lc-foundation-test-67352` empty (exit0).
- The isolated CB02 rerun after e530bfdc is independently green (consent-acl-pg.log); the separate 7-test read-chain run was completed before this contamination and remains valid for its recorded source.
- Required next run: after the Begin ruling/fix and remaining fixture changes, freeze all source/migrations for the whole final G07. No PASS or full-suite completion is claimed here.
