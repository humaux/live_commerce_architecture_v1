<!-- Purpose: CI-r1 tool and failure provenance. Depends on: immutable run IDs, source tags and observed local results. Used by: integrator evidence review. -->
# Provenance

- Base: 9bd4ad24440d2e4def4ed45022d988c5c5a077c5; configured reviewer gpt-6.1-sol/high.
- Local Docker Compose: v5.5.1; Node: v24.15.0; macOS arm64. JSON-omission control only removes rendered false fields, forwards all other Docker operations unchanged; see omitempty-control.py.
- Official temporary ShellCheck: v0.10.0, asset shellcheck-v0.10.0.darwin.aarch64.tar.xz, obtained with gh release download from https://github.com/koalaman/shellcheck/releases/tag/v0.10.0 . No permanent install/dependency. Binary and archive removed after verification.
- Asset SHA256: `bbd2f14826328eee7679da7221f2bc3afb011f6a928b848c80c321f6046ddf81`; binary SHA256: `b9e420df8c78ec7d261d66277d5767cbd4cf6da4e4a9f8b02ea4811cd4cc1109`.
- Full foundation-failed log SHA256: `caacc5d4b4cbf6ff43372e9d6d1fa4e14747917206133ec9815c2be796366bde`. Relevant failure retained in foundation-failure-excerpt.log; unrelated shard output omitted from commit.
- Actual artifact API for run37584606194 returned total_count0; gh run download reported no valid artifacts. No S01 artifact result is claimed. Local same-S01 ShellCheck command supplied the diagnostic.
- Static smoke CI verdict: 8 PASS, 0 FAIL, no BLOCKED/NOT_RUN/missing cases. Full GitHub/PG/browser smoke reruns remain owner/integrator work.
