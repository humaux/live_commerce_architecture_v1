# PARITY-DOMAINS · implementation handoff (RVFIX-407)

Updated 2026-11-01. PR #407 on branch `unit/parity-domains`; implementation head = the commit that carries this file (tests-only RED head `c5c0bde7a`, tests `4ba42761d`). Author verdict E0; no local tests (NO_LOCAL_TESTS_OR_SERVERS), MOCK-level API only.

## Evidence status (truthful)

- The original domains-parity RED (tests-only head `34d85c73e`) was **not observed** before the implementation was written; its WAIT_RUN ids `41109244416`/`41109244430` were never recorded as a failing result. Do not cite them as RED.
- This round's run is the **first observed RED**: Gates run `41223251352` on tests-only head `c5c0bde7a` - FAILED n=45: gate (bundle:03), gate (bundle:04), receipts, required. Cases targeted: (a) definitive 409 on add, (b) definitive 403 on suspend, (c) apex DNS v6-first, (d) apex DNS v4-only (see `red.log`; per-assertion text was not extracted from the job log by this lane).
- GREEN: PENDING - run id recorded by the integrator after pushing the implementation; until then GREEN is NOT_RUN.
- Implemented in `apps/admin/components/DesignDomains.tsx`: P1-1 `failDefinitive` clears journal and pendingCommand on a definitive (`!ok && !uncertain`) failure in add/reapply/suspend/detach/retry and shows `readFailed` (add keeps `invalid` only for code `invalid_request`); P1-2 apex rows partitioned by IP family (`:` = AAAA), one row per address, `missingIP` only for an empty family.
- NOT_RUN: browser clicks and persistence, SANDBOX/LIVE DNS/TLS, independent review/E4.

## Authority and current-state audit

Owner card controls this implementation. Read AGENTS.md, 架构.md §§0/1/2/26/27, docs/delivery/units/store-domains.md and current DesignDomains/client/model/journal code at base629f42152f. PR361 is merged. Domain UI currently only lists status and links to Settings; this is the legacy layout the spec rejects. Data access remains existing scoped server responses. No backend/API/contract/migration/pin/lockfile edits.

Figma14 roots on formal page4:8829 were read now using Fleet Bridge get_design_context and returned PNGs; deep children were read separately where the root response stopped. This is current design evidence, not a claim that historical output reflects current app state. Original domain design delivery: `output/figma-ui-completion-20261025/domains-20261026/DELIVERY.md`; ux-editor later delivery intentionally only covered status. The prototype disclaimer/fixed sample values are review scaffolding and will not enter product copy. Existing admin shell A and green primary follow owner card.

## Field/fact parity contract

Every row below is required in desktop, mobile and applicable review/summary. Current app acceptance is NOT_RUN; these are test/implementation obligations.

| Required field/fact | Main desktop/mobile | DNS/copy | Confirm/receipt/recovery | Source |
|---|---|---|---|---|
| Platform full origin retained; numeric identity immutable here | Platform card/full-value action | Full platform value | Receipt retains platform | GET domains platform row |
| Each exact custom origin, kind and lifecycle | One independent row per record | Selected origin + actual state | Exact selected/original target | GET domains / original command |
| ACTIVE differs from ACTIVE + serving | Separate state and serving text | Same distinction | Receipt must not imply readiness | state + serving |
| Full DNS type, host, value, no truncated TXT | Settings action | Request receipt only; full-value copy | Return to same selected record | POST domains dns |
| Root-domain A/AAAA only with returned concrete IPs | Same record | Show all returned addresses or missing-IP notice | Retain original request | POST dns.apex/edge_addresses |
| Missing full DNS must not guess CNAME/IP or reuse foreign TXT | Existing record persists | Missing-instructions notice | Recover original receipt if retained | GET does not contain full instructions |
| B request does not replace A; platform cannot be detached | Independent rows | Exact target actions | Confirm/cancel/receipt apply to one origin | Existing request/move endpoints |
| Failed read retains last observed state | Last snapshot + read error | No verified-success claim | Uncertain write remains fenced | Client result |
| Uncertain response preserves original action, key and body | New writes fenced | No new TXT until original receipt | Frozen original command + explicit same-key replay, survives refresh | Existing DomainCommand journal |

## BOURDIEU

Q1: Task is connecting an owned domain while retaining the platform address. Q2: Novice/expert means recent DNS setup experience, never class or device. Q3: Type/host/value stay grouped and complete. Q4: Explain DNS then HTTPS and preserve recovery context. Q5: Mobile gets the same fields/actions; no capability inferred from device. Q6: Aesthetic preference remains UNKNOWN unless owner decides; signed frames control current layout. These are design interpretations, not user-research findings.

## PROJECT UNDERSTANDING

Architecture0 distinguishes design/model/mock/live;1 keeps TypeScript strict and one Go/PG authority;2 keeps scoped merchant boundaries;26 requires isolated ownership and independent acceptance;27 requires real gate evidence. Owner numeric-domain ruling in store-domains unit preserves8digit platform identity, DNS-only checks, on-demand certificates, lifecycle and explicit independent serving state. Existing storefront-client/readDomains/requestDomain/moveDomain and strict parsers are the available contract. Original command journal must never persist a DNS token.

## MOBBIN → EAGLE

Current Eagle API item/info readback succeeded for `MVOLQ0BMNMQIC` (Base44 DNS table, Mobbin1ea0b43d-56cb-4553-8aed-a0ade7036175) and `MVOLQ0BRZTVSY` (Plane TXT, Mobbinc9fdb2f7-626c-43a3-8609-d87ef220db88), both currently in module folder `MVQL9YWH6013B`. Structural principles from the approved domain delivery: grouped record presentation, complete-value visibility, waiting versus done. No images, vendor branding, verification promises or product facts copied. No new Mobbin import needed for this signed implementation; lane does not have a Mobbin tool.

## FIGMA OFFICIAL GUIDANCE

Read `ux-editor/output/ux-audit-ref/FIGMA-GUIDE.md` G1–G7 (official source links in that guide, checked2026-10-28). Planned application: G1 nested auto-layout translates into responsive grid/flex;G2 existing semantic tokens and locale modes;G3 shared native controls/components;G4 every control has a result/return;G5 explicit uncertain/missing-data branches;G6 scoped route/data/focus/scroll/fixed footer documentation;G7 incumbent design inventory before composition. No Figma writes. Runtime validation belongs to CI and independent review.

## Three-language assertion contract

These exact strings are independently authored test expectations. Production i18n namespace and all additional visible help/error strings will be added in the implementation push after causal RED.

| zh-TW | zh-CN | en |
|---|---|---|
| 網域設定 | 域名设置 | Domain settings |
| 網店網址 | 网店网址 | Domains |
| 平台網址 · 系統配置 | 平台网址 · 系统分配 | Platform address · System assigned |
| 查看完整平台網址 | 查看完整平台网址 | View full platform address |
| 新增網域 | 新增域名 | Add domain |
| 新增自訂網域 | 新增自定义域名 | Add custom domain |
| 網域（不含 https://） | 域名（不含 https://） | Domain (without https://) |
| 申請此網域 | 申请此域名 | Request this domain |
| 取消 | 取消 | Cancel |
| 查看設定 | 查看设置 | View settings |
| 這筆自訂網域 | 这笔自定义域名 | This custom domain |
| 查看 DNS 記錄 | 查看 DNS 记录 | View DNS records |
| 返回網域設定 | 返回域名设置 | Back to domain settings |
| 回網域設定 | 回域名设置 | Return to domain settings |
| 重新讀取狀態 | 重新加载状态 | Reload status |
| 複製主機 | 复制主机 | Copy host |
| 複製值 | 复制值 | Copy value |
| 複製完整值 | 复制完整值 | Copy full value |
| 已複製完整值 | 已复制完整值 | Full value copied |
| 返回剛才頁面 | 返回上一页 | Return to previous page |
| 暫停此網址 | 暂停此网址 | Suspend this address |
| 解除此網址 | 解绑此网址 | Detach this address |
| 暫停自訂網域 | 暂停自定义域名 | Suspend custom domain |
| 解除自訂網域 | 解绑自定义域名 | Detach custom domain |
| 確認暫停 | 确认暂停 | Confirm suspension |
| 確認解除 | 确认解绑 | Confirm detachment |
| 操作結果 | 操作结果 | Operation result |
| 先確認這一次操作 | 先确认本次操作 | Confirm this operation first |
| 重試原操作 | 重试原操作 | Retry original operation |
| 查看目前網域記錄 | 查看当前域名记录 | View current domain records |
| 重新申請此網域 | 重新申请此域名 | Request this domain again |
| 目前缺少完整 DNS 指示 | 当前缺少完整 DNS 指引 | Complete DNS instructions unavailable |
| 根網域的 IP 尚未提供 | 尚未提供根域名 IP | Root domain IP not provided |
| 狀態未更新 | 状态未更新 | Status not updated |
| 請輸入有效的網域名稱，不含 https:// 或路徑。 | 请输入有效的域名，不含 https:// 或路径。 | Enter a valid domain without https:// or a path. |
| 平台網址仍保留；尚未透過此自訂網址服務。 | 平台网址仍保留；尚未通过此自定义网址提供服务。 | The platform address is retained; this custom address is not serving buyers yet. |
| 正在服務買家 | 正在服务买家 | Serving buyers |

## Frame → screenshot → authority PNG

| Frame | CI browser artifact (NOT_RUN) | Current Fleet Bridge PNG | SHA256 |
|---|---|---|---|
| 5:16526 | parity/5-16526.png | figma/get_design_context-5-16526-1793385445352-0.png | 2b576ba72eb10972de705eadb68832542f1c7ffb615d42dd0da08020647e1441 |
| 5:16621 | parity/5-16621.png | figma/get_design_context-5-16621-1793385445483-0.png | 76da506086d914af822821c1399fddc60e19d95cbd2d45033180b4d18fbf376a |
| 1076:7028 | parity/1076-7028.png | figma/get_design_context-1076-7028-1793385445685-0.png | acb3ca6826e68274fbfec21767ee2af09e44143767c759499447b902c0edd623 |
| 1076:7029 | parity/1076-7029.png | figma/get_design_context-1076-7029-1793385445823-0.png | ac065b010bbeb49ce834b6d462b2e407d2de4359855fbb572ca410d0b61741f0 |
| 1076:7030 | parity/1076-7030.png | figma/get_design_context-1076-7030-1793385446089-0.png | c5287900b11a636a61f2ed954fbfbcb5fca1cd9d838caad6da168f682edd4244 |
| 1076:7031 | parity/1076-7031.png | figma/get_design_context-1076-7031-1793385446303-0.png | ed16fe6b8e208aa5ab1ee8a9e679dd7bcc1210ec86b301497a201ee8ffb825b3 |
| 1076:7032 | parity/1076-7032.png | figma/get_design_context-1076-7032-1793385446481-0.png | cf68e9875fdc3fabf9cb78c7fa19e310805fdaa9a48565640a82b8a26608d970 |
| 1076:7033 | parity/1076-7033.png | figma/get_design_context-1076-7033-1793385446587-0.png | e6ac78585a0d500febd3352e4fcfafdb81fae5c13c77aa1ffafabea26509f80a |
| 1076:7034 | parity/1076-7034.png | figma/get_design_context-1076-7034-1793385446779-0.png | 6b332f1639db07efb72022cc9cd6d5b37a44e744665fa39307496a78b2c4805b |
| 1076:7035 | parity/1076-7035.png | figma/get_design_context-1076-7035-1793385446900-0.png | 364c44a358875497acef58740d7bb5bcd3c5f59068e17ce92cfdd9a3b06a2cc8 |
| 1076:7036 | parity/1076-7036.png | figma/get_design_context-1076-7036-1793385447121-0.png | ae60b56c6e30bf347dd72128154c00781a8684e724e82732501a109f922b4b6d |
| 1076:7037 | parity/1076-7037.png | figma/get_design_context-1076-7037-1793385447305-0.png | 4cbaa369e5fb025b7d5180d8032db4066b20da8def439e528b890ac51a415d3c |
| 1076:7038 | parity/1076-7038.png | figma/get_design_context-1076-7038-1793385447767-0.png | 53b1836312d60b507b55ffe4e4cb1e7e9c67418d0f672b06320376a0788be768 |
| 1076:7039 | parity/1076-7039.png | figma/get_design_context-1076-7039-1793385447899-0.png | feb6a109be64bb1a8eb745fcaa4510364b37dbb52cc27a14722266235c814eed |

## Evidence and continuation

E1: node --check new spec, ownership generation+--check, diff--check exit0. Tests-only commit includes new spec, existing editor domain assertion migration, Playwright registration and generated ownership. Existing gate-adapter assertions remain unchanged. RED pending Gates41109244416; Foundation41109244430/job124247071271 queued. Gates currently exposes only plan job124247070200, which is not test evidence. Integrator relays conclusion into a fresh session; >3h escalates; no polling loop/process remains.

NOT_RUN: implementation; actual browser clicks, all controls ledger, refresh persistence, real-route parity PNGs/density measurement; new i18n runtime validation; CI RED/GREEN; independent review; sandbox/live DNS/TLS; production. Browser spec attaches click-ledger; review cannot claim success from that planned artifact. Generated files will be regenerated again after implementation/spec changes. No evidence-only commit after GREEN.
