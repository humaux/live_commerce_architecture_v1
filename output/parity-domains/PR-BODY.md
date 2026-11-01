PARITY-DOMAINS replaces the read-only design-domain list with the fourteen signed domain settings states. History: the first push was tests only; this head adds the RVFIX-407 implementation after an observed RED.

Base: `629f42152f864621a99ac2247c3baf5d751a422f` (PR #361, PARITY-INBOX-EDITOR merged). Head: this branch tip (tests-only RED head `c5c0bde7a`).

Authority: [Figma page 4:8829](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=4-8829), inventory §3 domains follow-up and Appendix A `U:domains`. All fourteen current frames read through Fleet Bridge on 2026-10-31 Asia/Shanghai; screenshots and raw reads retained in `output/parity-domains/figma/`. Domain rules remain `docs/delivery/units/store-domains.md`, current frontend DTOs/clients and Go domain service. DNS-only and certificate-on-demand remain unchanged.

The registered `store-design` browser suite now includes a real-route domains spec in zh-TW, zh-CN and en at 1440×900 / 390×844. Native clicks exercise requests, cancellation, full DNS values and copying, suspend/detach, refresh persistence, failed reads, all lifecycle states, ACTIVE without serving, exact original-operation recovery after response loss, apex A/AAAA and missing IPs. Request B must retain A and the platform origin. No action may detach the platform address or invent DNS data.

Legacy mappings (preserved intent):

| Old assertion/control | Signed replacement |
|---|---|
| Read-only `系統網址` / `系统网址` / `Store address` heading | `平台網址 · 系統配置` and full authoritative origin |
| Generic `自有域名` / `Custom domain` heading | Per-record hostname, lifecycle and actual serving state |
| `管理網址` / `管理网址` / `Manage domains` link to Settings | In-module request / DNS / confirm / receipt / recovery states; assert old `design-domain-settings` link absent |
| Editor screenshots `1429:66030` / `1429:66073` | Dedicated main frames `5:16526` / `5:16621` |

Parity manifest below is the capture contract. Browser PNGs are **NOT_RUN** until CI executes the spec; current authority PNGs come from this session's Fleet Bridge reads, with exact filenames indexed by `output/parity-domains/DELIVERY.md`.

| Frame | Browser artifact | DELIVERY PNG |
|---|---|---|
| 5:16526 | parity/5-16526.png | figma/5-16526.log → IMAGE |
| 5:16621 | parity/5-16621.png | figma/5-16621.log → IMAGE |
| 1076:7028 | parity/1076-7028.png | figma/1076-7028.log → IMAGE |
| 1076:7029 | parity/1076-7029.png | figma/1076-7029.log → IMAGE |
| 1076:7030 | parity/1076-7030.png | figma/1076-7030.log → IMAGE |
| 1076:7031 | parity/1076-7031.png | figma/1076-7031.log → IMAGE |
| 1076:7032 | parity/1076-7032.png | figma/1076-7032.log → IMAGE |
| 1076:7033 | parity/1076-7033.png | figma/1076-7033.log → IMAGE |
| 1076:7034 | parity/1076-7034.png | figma/1076-7034.log → IMAGE |
| 1076:7035 | parity/1076-7035.png | figma/1076-7035.log → IMAGE |
| 1076:7036 | parity/1076-7036.png | figma/1076-7036.log → IMAGE |
| 1076:7037 | parity/1076-7037.png | figma/1076-7037.log → IMAGE |
| 1076:7038 | parity/1076-7038.png | figma/1076-7038.log → IMAGE |
| 1076:7039 | parity/1076-7039.png | figma/1076-7039.log → IMAGE |

**RED/GREEN (truthful).** The original domains-parity RED was not observed before implementation. This round's tests-only head `c5c0bde7a` produced the first observed RED: Gates run `41223251352` FAILED n=45 (gate bundle:03, bundle:04, receipts, required), covering the new cases (a) 409 add, (b) 403 suspend, (c) v6-first apex, (d) v4-only apex. GREEN: PENDING (run id recorded after the implementation push; NOT_RUN until then). MOCK level only; no local tests or servers.

RVFIX-407 fixes: definitive 4xx failures now clear the sessionStorage journal/fence and show the action's error copy (only uncertain outcomes keep the fence); apex A/AAAA rows are partitioned by IP family with one row per returned address and no invented address. See `output/parity-domains/RVFIX.md`.

Files outside literal component/route paths: `tests/admin/editor-v2.spec.ts` migrates existing domain assertions with the same intent; `playwright.config.ts` registers the new spec; `output/ci-select-backend-browser/browser-ownership.json` is generated. These are the card's authorized specs/registration/generated exceptions. No backend, contract, migration, pin, global stylesheet, lockfile, gate-adapter assertion, exemption or baseline edits.

NOT_RUN: implementation, CI RED/GREEN, actual browser clicks and persistence, parity screenshots, independent review/E4, SANDBOX/LIVE DNS/TLS, production. Author assessment is E0. Rollback is a revert of this card's commits through an integrator-managed PR. No merge/automerge, status posting or deployment is authorized for this lane.

🤖 Generated with Codex (dispatched by Claude Code integrator).
