# Independent review — orders-v2

Scope: local `unit/orders-v2`; no production or external provider review.

## Read model / privacy

Read-only reviewer `/root/orders_contract` examined the contract, SQL/ACL,
Go/parser/cursor and BFF/UI boundaries. Findings resolved before delivery:

1. SQL session choices were UUID-ordered: changed to actual session creation
   recency; store-scoped durable claim relation is retained.
2. Private queries appeared in URLs: moved to bounded read-only POST with CSRF,
   private/no-store and component-only query/cursor state; never URL/storage.
3. Poll/reload swallowed authoritative denials: now clear list and full detail.
4. Later cursor pages skipped polling: now probe authority without replacing
   page-N rows; denied state persists after erasing the private query.

Final static verdict: no remaining P0/P1 in the reviewed scope. The reviewer
explicitly did not claim to run runtime tests. Main agent separately ran the
actual PG and browser gates. MOU08 covers both first-page GET and private-search
page-two POST with an open detail, then 403 and disappearance of all PII.

Humaux review titles: `Orders v2 final review — cursor-page revocation leaves PII`
and `Orders v2 cursor revocation fix — statically resolved`.

## Visual review

Fresh `/root/orders_visual_final`, no inherited transcript, inspected the approved
comps and all 12 initial captures, plus relevant source. Named Impeccable reviewer
role was unavailable, so a fresh read-only default agent used its output contract.

Initial disposition: **fix**. One batch then resolved:

| Finding | Evidence / final verdict |
|---|---|
| Mobile secondary filters displaced all orders | Native expand/collapse; tabs and first order above fold in three locales. Resolved. |
| Desktop totals left aligned | All three locales now align ledger amounts right. Resolved. |
| English date affordance | Explicit English year/month/day hint; platform calendar remains native. Resolved. |

Final disposition: **ship** for these scored fixes; no new regressions found in
that bounded pass. C open detail remains present; separate MOU05 regression PNGs
are included. No generic design rule overrode the approved C workflow.

Humaux title: `orders-v2 visual fix-round-1: three findings resolved`.
Detector: one run only, `detector.json` = `[]`, exit 0. No raster assets generated.
