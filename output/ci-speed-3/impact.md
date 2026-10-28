# CI-SPEED-3 static selection impact

Measured at 2026-10-28T07:51:01.868Z; E1 static source/diff measurement, not runtime acceptance.
Source: GitHub merged PR metadata and paginated PR files API receipts in this directory. All 15 PRs merged since 2026-10-27 were retrieved, sorted by mergedAt descending, then the newest ten selected (not PR creation order).
Both policies use the same integration registry snapshot (997480141a7959ad6256a29e43bb1af31b97a404) and current source observations; historical PR CI/registry sizes are not compared. Unknown/deleted paths conservatively use the full current universe. Counts exclude foundation-shards, deploy, shards and job bundling.

Registry SHA256: 718729e925920fd1b9b80bab166c01d717c36700d45750b7b168eb5217992f95; runnable browser universe: 53. Old planner: RED SHA 04036e51ba4e7ed1236a825e1f96c635f563ec44, frozen in pr-modes-before.mjs.

| PR | Merged UTC | Paths | Old browser modes | New browser modes | Reduction |
| --- | --- | ---: | ---: | ---: | ---: |
| [#237](https://github.com/humaux/live_commerce_architecture_v1/pull/237) | 2026-10-28T06:24:23Z | 44 | 53 | 53 | 0 |
| [#248](https://github.com/humaux/live_commerce_architecture_v1/pull/248) | 2026-10-28T05:03:45Z | 7 | 53 | 53 | 0 |
| [#240](https://github.com/humaux/live_commerce_architecture_v1/pull/240) | 2026-10-28T01:41:12Z | 2 | 53 | 53 | 0 |
| [#234](https://github.com/humaux/live_commerce_architecture_v1/pull/234) | 2026-10-28T01:40:53Z | 2 | 53 | 53 | 0 |
| [#243](https://github.com/humaux/live_commerce_architecture_v1/pull/243) | 2026-10-27T23:02:09Z | 5 | 53 | 53 | 0 |
| [#238](https://github.com/humaux/live_commerce_architecture_v1/pull/238) | 2026-10-27T22:34:48Z | 10 | 53 | 53 | 0 |
| [#229](https://github.com/humaux/live_commerce_architecture_v1/pull/229) | 2026-10-27T21:19:38Z | 22 | 53 | 53 | 0 |
| [#227](https://github.com/humaux/live_commerce_architecture_v1/pull/227) | 2026-10-27T19:17:29Z | 13 | 53 | 53 | 0 |
| [#233](https://github.com/humaux/live_commerce_architecture_v1/pull/233) | 2026-10-27T12:58:35Z | 2 | 0 | 0 | 0 |
| [#211](https://github.com/humaux/live_commerce_architecture_v1/pull/211) | 2026-10-27T12:58:12Z | 9 | 49 | 49 | 0 |

Total mode selections: 473 -> 473 (0.0% reduction).

App-level ownership remains broad: admin 47, storefront 29. The <=15 single-app target is NOT achieved by safe app-level ownership on this registry; meeting it needs a separately authorised component/route selection contract. No ownership was omitted to meet a count.
The base-present live-settings.spec.ts has 1 executing browser owner, versus 53 previously. native-device.ts has 6 importing owners.
Shared infrastructure in historical diffs can dominate the ten-PR sample and keep the full universe. This comparison claims neither wall-clock savings nor production acceptance.

Reproduce the static measurement: node output/ci-speed-3/measure-impact.mjs. CI acceptance is separate and pending.
