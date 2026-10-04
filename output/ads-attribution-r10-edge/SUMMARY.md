# R10 shipping-Caddy trust-boundary regression

task_id: 0c63db14-dce6-470f-85b8-17fbb000fbd0
base_commit: 39466dd4ce5ccdd8f0524c5c98909a5da7f43772
branch/worktree: unit/ads-attribution-r10-ui / .worktrees/ads-attribution-r10-ui
role: regression test author; root independent acceptance pending.
model: GPT-6 family per runtime instructions; exact model ID and reasoning UNKNOWN.
write_paths: tests/deploy/platform-edge.mjs; output/ads-attribution-r10-edge/.

The existing PS3 pinned-image shipping-config validate/fmt/adapt and platform routing checks
are retained. LC_PLATFORM_EDGE_EVIDENCE now selects an isolated output directory while the
existing output/platform-site default remains compatible. Only container-local :3200 MOCK
upstream fixture configuration is appended; deploy/caddy/Caddyfile is unchanged.

Four added cases use real TLS requests through that Caddy. Upstream Host/XFF/XFH echo values
are compared with Caddy access-log request.remote_ip, rather than assuming loopback is the
Docker connection peer. Single and chained hostile XFF become exactly 172.17.0.1 in this run;
hostile XFH=admin.localhost leaves actual upstream Host/XFH=shop.localhost. Known shop SNI
plus actual HTTP Host=unknown.localhost routes through the shipping catch-all: upstream sees
unknown.localhost, its MOCK allowlist rejects 404, and XFF still equals the peer. Unknown SNI
fails TLS with EPROTO / tlsv1 alert internal error (no local tls-ask service).

These are LOCAL Caddy TLS / MOCK upstream assertions. They do not prove real BFF independent
edge authentication, production configured-edge provenance, domain-service allow/deny behavior,
CDN behavior, provider capability, SANDBOX or LIVE.

## Actual commands / exit codes

Commands ran from /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r10-ui.
Permanent evidence: /Volumes/data/live_commerce_architecture_v1/output/ads-attribution-r10-edge/.

| Command | Exit | Evidence |
| --- | --- | --- |
| LC_PLATFORM_EDGE_EVIDENCE=output/ads-attribution-r10-edge node tests/deploy/platform-edge.mjs | 0 | edge.log; original PS3 checks and four new cases PASS |
| LC_PLATFORM_EDGE_EVIDENCE=output/ads-attribution-r10-edge/red node tests/deploy/platform-edge.mjs | 1 | edge-red.log; temporary container-runtime-only mutation forces upstream XFF=203.0.113.9; assertion rejects actual203.0.113.9 versus connection-peer172.17.0.1 |
| LC_PLATFORM_EDGE_EVIDENCE=output/ads-attribution-r10-edge/green node tests/deploy/platform-edge.mjs | 0 | edge-green.log; mutation removed; original PS3 checks and four cases PASS |
| node --check tests/deploy/platform-edge.mjs | 0 | syntax-final.log |
| pnpm exec prettier --check tests/deploy/platform-edge.mjs | 0 | format-check.log |
| git diff --check | 0 | source diff checked |
| docker ps --filter name=lc-platform-ps3 --format '{{.Names}}' | 0 / empty | no owned PS3 container remains |

green/ps3-edge.json contains the actual upstream values, observed peer, precise denial stage,
and scope limitations. green/caddy-runtime.log contains the matching access records.
Both red and green validate/adapt/runtime artifacts remain available. The mutation touched only
the own test's temporary runtime text builder, not shipping config, and was removed before GREEN.

No PG/browser/focused/full gates were run. REAL_PG/BROWSER/SANDBOX/LIVE: NOT_RUN.
Root must independently inspect/cherry-pick and rerun the isolated test with a fresh evidence
directory. No production config/deploy, secrets, external mutations or recursive delegation.
The test stopped its own disposable Caddy containers and removed only its mkdtemp fixtures.
