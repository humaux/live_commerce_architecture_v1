<!-- Purpose: hand off CA-UI-COPY implementation and exact evidence limits. Depends on: card, owner choices, frozen RED tests and CI receipts. Used by: integrator and independent reviewer. Invariants: no local runtime acceptance or GREEN claim. -->
# CA-UI-COPY — implementation handoff, 2026-10-28

Status: E1 static structure/syntax only; implementation acceptance PENDING. Author self-review is E0. No completion claim.
PR #267 targets r3/integration. Branch unit/ca-ui-copy; assigned worktree /Volumes/data/live_commerce_architecture_v1/.worktrees/ui-copy.
Role: ui_worker responsibility, Codex executor; exact runtime model/effort not exposed. Base: 26d5f721a9e519fe6e4e5736a0a4c1288854903b.
Integrator pushes the single implementation commit; no push, merge, auto-merge or PR mutation performed here.

## Scope and authority

Read AGENTS.md; 架构.md §§0, 1, 2, 26, 27; all three card JSON objects and ORD-01/CP-05/CU-01 backlog rows.
Backlog: output/figma-ui-completion-20261025/CODE-ALIGNMENT-BACKLOG.md, read-only main checkout, SHA256 e4f1c7a2120d26b21fed7a99eaaae12fdfedbd4ae350d17b18d96e277b577619.
Owner choices: contracts/owner-choices-20261026-v1.md, SHA256 6266427d0ad8dc46cbbf7616b49e8d724c82240fa4ef7ba4e0cc0d222d9757fe.
The resumed dispatch authorizes ORD-01 + CU-01 only and existing-test registration. CP-05 is SKIPPED per OC-Q6: no management-link label or Meta/IG disconnect action.

- ORD-01: align MERCHANT_SHIPPED/NOT_STARTED labels with existing queues in three locales. Server state codes, predicates and filters retain their original implementation.
- CU-01: add localized source-number help for external_id/customer_id mapping, connected with aria-describedby; clarify the existing failed-row CSV button and download notice. No CSV, API, erasure, idempotency or UNKNOWN/retry implementation changes.
- Tests in tests/admin/orders-v2.test.ts and tests/admin/import-view.test.ts are inherited from RED and remain byte-identical to base. They are outside the original JSON write paths, explicitly authorized by the integrator registration ruling. No other out-of-card files in the final diff.
- No Go dependency changes; no dependency-map regeneration needed. Header Purpose / Depends on / Used by / Invariants lines are preserved or added in the three edited source files.

## Project understanding

| Source read | Business rule imposed on this copy |
|---|---|
| 架构.md §§0, 1, 2 | Distinguish design from runtime evidence; Go/PG retain business authority; do not invent merchant/provider facts. |
| 架构.md §§26, 27; AGENTS.md | Assigned worktree and paths; author is not sole acceptor; real clicks required for UI acceptance. |
| owner-choices OC-Q21; merchant-orders-v2 request/bucket sections | Use 已出貨/待付款; preserve codes, scoped filters and overlapping task queues. |
| manual-fulfilment-v1 MD3, MD6 and state table | Merchant shipment is dispatch attestation, never carrier delivery or payment proof. |
| owner-choices OC-Q6 | Skip CP-05; do not introduce disconnect or 管理連結. |
| migration-import-v1 §§2–5, results.csv and erasure rules | Failed rows reveal row number/outcome/reason only; never revive erased source identifiers. |
| ImportWizard download/unknown handlers; import-view-model; import-client | Reuse existing safe downloads; preserve original-request recovery and session checks. |
| packages/i18n/src/index.ts | Use existing Locale = zh-CN / zh-TW / en and module-local typed copy dictionaries. |

Live Figma readback via Fleet Bridge get_metadata: 1032:6987 contains 下載未匯入的列（CSV）. 1194:10455 still describes source IDs in the CSV and prototype behavior. Product copy follows the current privacy contract and actual safe-failure serializer instead; no prototype scaffolding enters product text. Raw readbacks: figma-download.json and figma-csv-note.json. Figma was read only.

## Three-language copy table

Only new/changed copy is listed; existing surface strings are retained.

| zh-TW | zh-CN | en |
|---|---|---|
| 已出貨 | 已出货 | Shipped |
| 待付款 | 待付款 | Awaiting payment |
| 你舊系統裡每位顧客的唯一編號（source_id）。 | 你旧系统里每位顾客的唯一编号（source_id）。 | Unique customer ID in your old system (source_id). |
| 下載未匯入的列（CSV） | 下载未导入的行（CSV） | Download rows not imported (CSV) |
| 已開始下載。CSV 只含來源列號、結果與原因代碼，不含姓名、電話或來源編號。 | 已开始下载。CSV 只含来源行号、结果与原因代码，不含姓名、电话或来源编号。 | Download started. The CSV contains only source row numbers, outcomes and reason codes; no customer names, phone numbers or source IDs. |

## Desktop/mobile and summary parity checklist

This is a code-copy handoff, not a new Figma design delivery. Evidence below is static shared-render-path inspection; rendered parity and refresh behavior remain NOT_RUN.

| Required fact/field affected | Desktop + mobile source | Summary/review state | Evidence limit |
|---|---|---|---|
| Shipment/payment labels | Same ordersCopy locale dictionary used by MerchantOrders | Existing detail/status rendering uses the same dictionary; queue copy already matches | Static only; real clicks NOT_RUN |
| Old-system customer number | ImportColumnMapping, external_id and customer_id selects | Mapping help explains the field; preview/results intentionally disclose no identifier values | Static only; real clicks NOT_RUN |
| Failed rows CSV action | ImportWizard common button, preview present and failed count positive | Same control outside stage-specific content | Static only; real download NOT_RUN |
| Safe CSV contents | Common downloadStarted notice via existing download handler | Same notice for preview-only and saved-batch downloads | Static only; privacy assertions await GREEN |
| Preview counts and erased-row count | Shared ImportPreviewCounts | Preview and confirm retain this component | Existing rendering retained; runtime NOT_RUN |
| Unknown result / same-request retry | ImportWizard existing unknownCommit and retry-same control | Existing recovery state retained | Source unmodified; runtime NOT_RUN |

No recipient, destination, order total, payment timing or summary field is added/removed by this copy-only diff. Whole-screen design parity is not asserted.

## Bourdieu — task familiarity and Q1–Q6

Novice/expert means recent experience with order queues or CSV mapping, never class, language or device. The change makes payment status consistent and explains the source-number concept next to the relevant task. No user research or aesthetic preference is inferred.

| Question | Verdict |
|---|---|
| Q1 | UNKNOWN: exact current rubric is in unavailable Humaux; not invented here. |
| Q2 | UNKNOWN: exact current rubric unavailable. |
| Q3 | UNKNOWN: exact current rubric unavailable. |
| Q4 | UNKNOWN: exact current rubric unavailable. |
| Q5 | UNKNOWN: exact current rubric unavailable. |
| Q6 aesthetics | UNKNOWN until owner; no aesthetic change made. |

## Mobbin → Eagle

Read existing module delivery/manifest entries and verified candidates through the running Eagle API, without importing or writing assets. Module folders were confirmed with GET /api/item/list?folders=<id>.

| Module folder / candidate | Structural principle retained | Read receipt |
|---|---|---|
| MVQL9YR448VTK / MVP0WJO2CM2IC, Squarespace orders | Keep payment and fulfillment separate within the existing task hierarchy | eagle-orders.json: success, 8 items, candidate present |
| MVQL9YUYU0QJ0 / MVOIV0IXOVPBM, Unify CSV mapping | Explain field meaning adjacent to the mapping control | eagle-import.json: success, 10 items, candidate present |

No imagery, brand, product fact or foreign business rule copied. No new pattern needed; no Mobbin import query requested. Original images were not re-reviewed for this copy-only implementation.

## Figma official guidance

The requested output/ux-audit-ref/FIGMA-GUIDE.md is absent in both this worktree and the main checkout; its prescribed entries are UNKNOWN. No new layout, component, variable or prototype is authored, so no official-guidance application is claimed. Existing implementation and frozen owner copy govern. Figma copy lint is NOT_RUN / not applicable to new frames: zero frames created or changed. No lint PASS summary is claimed. Impeccable clarify and ponytail were read; local detector execution was omitted to honor the card's static-command allowlist.

## Evidence and checks

- RED: red.log plus filtered raw CI excerpts. Foundation run 40645100067 / job 122671969067 fails ORD-01 on Unpaid vs Awaiting payment. Gates run 40645100039 / job 122683775768 fails CU-01 on absent sourceIdHint. Both bind PR head 26d5f721a9e519fe6e4e5736a0a4c1288854903b; synthetic merge SHA not extracted.
- Allowed local static check: python3 scripts/check_packet.py, exit 0, PASS_PACKET_STRUCTURE_ONLY; static-packet.log. Its auto-written experiments/results/packet-check.json was restored to the clean starting version, leaving no final out-of-scope change.
- Allowed syntax checks: node --check apps/admin/lib/orders-copy.ts; node --check apps/admin/lib/import-copy.ts; both exit 0. Not type checks and not UI tests.
- TypeScript tsc: NOT_RUN, executable/dependencies absent; no dependency installation attempted. ImportWizardSteps JSX/type checking awaits CI.
- GREEN run/job IDs: PENDING; implementation not pushed in this sandbox. WAIT_RUN: NONE until integrator pushes. Do not reuse historical RED or another branch's GREEN as acceptance.
- Acceptance gate: PR's own Foundation/Gates must execute unchanged ORD-01/CU-01 assertions and existing privacy/protocol tests on the implementation head, followed by independent review. Real browser clicks and three-locale main-screen checks remain separately pending.

## Click ledger

| Page/control | Operation / expected result | Actual |
|---|---|---|
| Orders state/filter, zh-TW / zh-CN / en | Real select/click; aligned labels, same filtering; reload preserves supported URL filters | NOT_RUN, CI only |
| Import customer/order mapping select, all locales | Real select; localized help attached; mapping remains selectable | NOT_RUN, CI only |
| Preview/result failed CSV, all locales | Real click; safe metadata file and localized notice, no IDs/PII | NOT_RUN, CI only |
| Unknown import retry | Real click retries original request; does not replace frozen context | NOT_RUN, source unmodified |

## Trace and outstanding handoff

Humaux memory/coord/code-index tools were not exposed. No memory_store, shared canvas update, lock, claim or code-memory link is claimed. This is the integrator-assigned continuation; no duplicate claim or agent delegation was attempted. Integrator must store/attach this handoff and update its existing task.
NOT_RUN: all local tests/servers (prohibited), typecheck, browser clicks/refresh/three-locale rendering, live/PG/provider operations, implementation CI, independent review, Figma copy lint. Risk: long localized notices/help need rendered-width verification in CI; source_id is descriptive legacy-field terminology, not a new API key.
No task fixture, server, container, background CI watcher or temporary download was started. All files under output/ui-copy are intentional retained handoff evidence.
