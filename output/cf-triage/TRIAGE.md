<!-- Purpose: triage seven CF cards against primary owner decisions and frozen contracts.
Depends on: owner-choices-20261026-v1.md, CODE-ALIGNMENT-BACKLOG.md and the authority register below.
Used by: integrator and CA implementation-card owners.
Invariants: DESIGN only; UNKNOWN is not approval; runtime, LIVE and independent acceptance are not claimed. -->
# CF-TRIAGE — 2026-10-28

## Owner questions (yes/no; unanswered = UNKNOWN)

1. **Q-LOG:** 零密钥物流是否统一先使用现有 MANUAL 配送，平台 ECPay 凭据接入及旧商家 binding 迁移另卡处理？
2. **Q-FIN:** 是否确认 OC-Q2 更正为 B，结算退回与帶回下期完全采用 SA-0 §6.7–§6.9？
3. **Q-MSG:** 「未读」是否为店铺共享状态（任一获授权成员读过，即对全店视为已读）？
4. **Q-TEAM:** 在邀请标为「未納入產品」期间，是否也暂停接受此前已经发出的有效邀请？

These are questions, not defaults. Technical continuations below do not authorize new product decisions.

## Authority register

Repository line references are at base `997480141a7959ad6256a29e43bb1af31b97a404`, unless stated otherwise.

Read-only main input blob hashes (`git hash-object`, no object writes): CODE-ALIGNMENT-BACKLOG.md `fb520c93a9962d0abb67348ba39491862b47b960`; owner-decisions-20261026.md `f2b47db44bd8d6f666b3b2f0b21b31f786ce117b`. The backlog itself describes old code at `60268f3`; its observations are not current acceptance.

- **A1:** `contracts/owner-choices-20261026-v1.md:4–6,10–37`. Owner words at line 4: 「ad-01、Stripe不是支持信用卡、储蓄卡、link收款嘛？ad-03、让商家能够简单配置，快速开店，其他的按照你的结论来」. Binding choices apply only to their named points; explicit pending confirmation remains pending.
- **A2:** Humaux `b9197057-8187-4fc9-a8b7-425ad34fd7a9`, full `memory_get`, `source_type=human`, 2026-10-26: 「商家设置需要简单一点,不需要商家配置API key什么的,商家收款统一平台收款方式,能够让商家快速使用起来,同时买家可以填写地址,可以填写门市号码,继续推进」. First-hand quotation also in read-only main `output/integrator/tools/logs/owner-decisions-20261026.md:4`. No merchant keys covers payment and logistics; it does not choose platform logistics credentials versus MANUAL.
- **A3:** Humaux `0d6a98a3-a915-4eaf-9051-05f0ba4779a1`, full `memory_get`, `source_type=human`, 2026-10-25: 「全部收款使用平台统一代收（Stripe 平台账户）；商家不配置 API、密钥、商户号，不接入自有 PSP/Connect」. Repository quotation: `docs/delivery/units/settlement-api-plan.md` §0; contract: `stripe-platform-account-v1.md` §0.3 AD-PF2.1 / AD-PF3. Historical memory's claim that settlement APIs are absent is not adopted.
- **A4:** Owner's current CF-TRIAGE instruction, 2026-10-28: “CVS buyer store entry = brand + store number, server returns exact name/address from the 2026-10-28 snapshot (PR #262).” Replaces LOG-02's older optional client name/address target. GitHub readback 2026-10-28: #262 OPEN, head `03aa811f1f5c37d6e321504507be86d7267eb5a8`, tests-only. That PR is an implementation lead, not owner authority or acceptance; its snapshot hash/reservation is not promoted to a frozen contract here.
- **A5:** First-hand quotation transcript in read-only main `output/integrator/tools/logs/owner-decisions-20261026.md:5–10`: 「要选品牌」; 「保留货到付款，四家超商都开放」; 「用确认码，买家登录画面删掉，请款改成付款」; 「已有SKU归到第一个新规格值」 (code/price/stock/images unchanged); payment result only in order details, UNKNOWN no success/retry. Line 12 expressly labels invitation/notification as the integrator's reversible ruling; **A1:37** supplies the binding read-only boundary without upgrading that agent ruling to owner words.
- **A6:** Frozen contracts: `stripe-platform-account-v1.md` §0.3, §6.7–§6.9, §8; `catalog-inventory-v1.md` §g and product-media-v2; `storefront-v2.md` B-acceptance and §D. Agent plans/backlog are leads only.

## Seven-card triage

DECIDABLE means product boundaries can be amended from existing authority, not that implementation/review passed. PARTIAL permits only decided slices; the listed question stays UNKNOWN. No card is wholly BLOCKED_OWNER.

| CF card / backlog rows | Status | Decided boundary and exact authority | Owner question | CA cards affected / unlock boundary |
|---|---|---|---|---|
| **CF-SF**: SF-01/04/07 | **DECIDABLE** | A1:18–20 (OC-Q9/10/11): no replay entry; publish UNKNOWN checks **original request identity and version**, never new publish; one not-open message, no pause state. OC-Q18 (A1:27) permits read recovery only. Amendment: storefront-v2. | None. A request-bound receipt mechanism still needs a technical continuation; version list alone cannot prove which request committed. | **CA-SF-EDITOR**, **CA-SF-READSTATE**: product boundary resolved; publish receipt technical contract still required. **CA-SF-REPLAY**: close/de-scope as no-build, not a replay implementation unlock. |
| **CF-PAY**: AD-01/02, FIN-01/02, CP-02 | **DECIDABLE** | A2/A3; OC-Q1 (A1:10): zero merchant credentials; reject new own-bank details/orders server-side, retain history/pending completion/report fields/CSV shape. A6 AD-PF3 retains terms/self-stop and server/environment method set; AD-PF2.1 retains LIVE allowlist. A5 buyer 付款 never conflates authorization/capture/settlement. Amendments: platform and buyer contracts. | None. No LIVE authorization, fee default or migration number follows. | **CA-PLATFORM-CARD**, **CA-BANK-SETTINGS**, **CA-PAYMENT-COPY**: boundary resolved. **CA-QUICKSTART**, **CA-CVS-BUYER**, **CA-ORD-MANUAL**: payment dependency only resolved; LOG/TEAM dependencies remain. FIN-02 compatibility also feeds CF-FIN. |
| **CF-LOG**: LOG-01/02/03/04/05/08, CP-07 | **PARTIAL** | A2 bans merchant credentials. A4: buyer brand+number → server snapshot name/address. A5 four brands/COD are product targets, not API qualification. OC-Q4/8/16/17 (A1:13,17,25–26): country_flat + threshold, no address label, no separate self-delivery, shipped **email** retained, no Messenger/IG (MD10 already corrected). | **Q-LOG**. Platform credential ownership/binding transition/fallback UNKNOWN; existing enabled/visible/mode/qualification gates remain. Buyer snapshot decision does not silently extend to merchant manual orders. | **CA-LOG-SETTINGS**, **CA-QUICKSTART**: credential continuation blocked. **CA-CVS-BUYER**: product input resolved; await #262 CI/integration. **CA-ORD-MANUAL**: bank retirement resolved, logistics transition/manual snapshot scope pending. **CA-SHIPMENT-REVIEW**: OC-Q20 already authorizes review then existing command; no new notification/write authority. |
| **CF-FIN**: FIN-02/03/04 | **PARTIAL** | OC-Q1/2/3 (A1:10–12): preserve historical transfer reports/CSV; keep 已請款 / 上期結轉（扣除）/ 應撥淨額 / 已撥付. OC-Q2 explicitly says **Pending one-line owner confirmation**. SA-0 §6.7–§6.9 is sole returned-payout/returned_in_minor model. Plan:11–13 points there; no settlement-v1 created. | **Q-FIN**. Confirmation is not obtained; SA-1/2/3 continue per OC-Q2 without duplicate settlement contract. | **CA-FIN-UI**: existing SA-0 DTO/CSV after backend CI acceptance; confirmation stays explicit. No new fee default, report-column removal or duplicate DB card. |
| **CF-MSG**: AD-04/05/08 | **PARTIAL** | OC-Q12/13/14 (A1:21–23): unread filter after written definition + store-scoped/RLS query; no marketing consent display/inference; no health-mail off switch. Notification center stays unincluded/read-only (A1:37). Meta ingress contract does not define conversation read state. | **Q-MSG**. Read-state owner UNKNOWN; mark-read transition, cursor and query/ACL contract must follow the answer. Until then no unread chip. | **CA-MESSAGE-UI**: relative deadline/consent suppression can use existing contract; unread filter blocked. **CA-ADMIN-STATE** remains its independent dependency. |
| **CF-CAT**: CAT-01/02/03 | **DECIDABLE** | A5:9 「已有SKU归到第一个新规格值」 with code/price/stock/images retained. A6 §g and catalog contract:94 already accept original id + option_values; `internal/catalog/document.go:878–881,978–980` updates same row. OC-Q15 (A1:24) rejects per-SKU eligibility. New permanent axis/value IDs are unapproved strategy. Amendment: catalog contract, no OpenAPI change. | None for existing-ID fix. New permanent IDs need a separate decision; not required/authorized here. | **CA-CATALOG**: existing-API identity fix resolved; preserve media-v2 and limits. No axis-id/schema/migration work unlocked. |
| **CF-TEAM**: AD-13 | **PARTIAL** | A1:37 explicitly keeps invitations 「未納入產品」 read-only; A1:5 means no new records. Close new invite submission at UI/BFF/server; preserve readable history, no real-email verification. merchant-identity:20–26 separates first-store owner bootstrap from invitation. | **Q-TEAM**. Previously issued token acceptance is not specified by “no new invitations”; do not silently decide it. Preserve existing expiry/revocation until a scoped amendment resolves acceptance. | **CA-TEAM-BOUNDARY**: new-invite boundary resolved, old-token acceptance blocked. **CA-QUICKSTART**: bootstrap is not staff invitation; three-step new-store flow already authorized by OC-AD03, subject to PAY/LOG dependencies. |

## Amendment and continuation ledger

- Append-only amendments: `contracts/storefront-v2.md` (CF-SF), `contracts/stripe-platform-account-v1.md` and `contracts/stripe-buyer-ui-v1.md` (CF-PAY), `contracts/catalog-inventory-v1.md` (CF-CAT). Review-ready; **independent review NOT_RUN**. Frozen history preserved.
- PARTIAL cards get no speculative frozen amendment. Follow-up technical contracts must cover request-specific publish receipts; logistics ownership/binding transition and integrator-assigned migration; snapshot persistence/manual scope; unread state/query/cursor; outstanding invites. This card assigns no migration numbers.
- Out-of-path integrator follow-ups only: old PSP wording in product-payment-entry/merchant-service-settings; merchant-orders/reporting/buyer-checkout-options bank-entry gates; publish receipt API if needed; logistics continuation; staff acceptance policy. OC-Q1 already governs its named history/retirement points. No code/schema/task/old migration/R2 pin/baseline/dispatcher file is changed.

## Delivery and evidence

- Dispatched contract worker; GPT-6 family (exact runtime variant/effort not exposed); no recursive delegation. Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/cf-triage`, branch `unit/cf-contract-triage`, target `r3/integration`, base above. Write scope: this output directory plus assigned CF contract paths.
- **RED: NOT_APPLICABLE** by explicit CF-TRIAGE exception: docs/contracts only, no executable behavior. No artificial failing push.
- Static command: `python3 scripts/check_packet.py`, exit **0**, `PASS_PACKET_STRUCTURE_ONLY` at `2026-10-28T06:56:48.149662+00:00`; raw receipt `output/cf-triage/static.log`. Its generated packet-check.json timestamp-only change was restored. E1 only. Exact-head GitHub CI/independent review **PENDING/NOT_RUN** until receipts exist; own correctness claim E0.
- NOT_RUN: local tests/builds/servers, REAL_PG, browser/Figma, provider SANDBOX/LIVE, four owner confirmations, independent review, production. No process/server/fixture started. No files outside card write paths.
- Final SHA and Gates/Foundation run IDs go in PR comment/handoff; no post-GREEN evidence-only commit.
