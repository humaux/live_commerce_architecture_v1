# G-UI9 visual lint — dca90c2ae8 — 2026-10-07T09:21:56.978Z

Verdict: **PASS**

Shots: 6 captured of 6 enumerated (1 admin + 0 storefront + 0 platform pages x 3 locales x 2 viewports 1586x992, 390x844). NOT_RUN: 0. Missing: 0. HTTP >= 400: 0. States not reached: 0.

Blocking (exit 1): R1, R2, R3, R6, R9 and R7:clipped-control, R7:text-overflows-control, R7:select-value-clipped; every other finding is WARN for now. Thresholds: `tests/ui/visual-lint-lib.mjs` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).

## Counts per rule

| rule | what | severity | instances | of which blocking | groups | page-variants affected |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| R1 | horizontal overflow | block | 0 | 0 | 0 | 0 |
| R2 | form-row misalignment | block | 0 | 0 | 0 | 0 |
| R3 | table row consistency | block | 0 | 0 | 0 | 0 |
| R4 | min text size | warn | 0 | 0 | 0 | 0 |
| R5 | tap target (390) | warn | 6 | 0 | 3 | 3 |
| R6 | overlap | block | 0 | 0 | 0 | 0 |
| R7 | clipped text | block for controls, warn for labels/links | 3 | 0 | 3 | 3 |
| R8 | duplicate list label | warn | 0 | 0 | 0 | 0 |
| R9 | edge padding (390) | block | 0 | 0 | 0 | 0 |
| R10 | fixed-bar occlusion | warn | 0 | 0 | 0 | 0 |
| R11 | narrow control | warn | 0 | 0 | 0 | 0 |

## Counts per app (instances)

| app | page-variants | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | blocking |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| admin | 6 | 0 | 0 | 0 | 0 | 6 | 0 | 3 | 0 | 0 | 0 | 0 | 0 |

## Top 15 routes by violations

| # | app | route | state | blocking | warn | total |
| ---: | --- | --- | --- | ---: | ---: | ---: |
| 1 | admin | `/settings/operations` (settings-operations) | registry | 0 | 9 | 9 |

## admin

### `/settings/operations` (settings-operations, registry) — blocking 0, warn 9

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-TW-390x844.png` |
