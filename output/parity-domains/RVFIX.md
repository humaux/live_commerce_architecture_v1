# RVFIX-407 finding -> change -> RED -> GREEN

| Finding | Change | RED run | GREEN run |
|---|---|---|---|
| P1-1 journal after definitive failure | `apps/admin/components/DesignDomains.tsx` `failDefinitive` (~L137) called after the uncertain check in handleApplyAdd (~L191), handleReapply (~L235), handleConfirmSuspend (~L277), handleConfirmDetach (~L319), handleRetry (~L352, ~L381); `actionError` alert rendered at section top | Gates 41223251352 (cases a, b) | PENDING |
| P1-2 DNS rows by IP family | same file, apex branch of the receipt DNS block: partition on `:` -> AAAA, one row per address, `missingIP` only for an empty family | Gates 41223251352 (cases c, d) | PENDING |
| P1-3 receipts | `output/parity-domains/DELIVERY.md`, `PR-BODY.md`, `red.log` rewritten: cite RED 41223251352, original RED never observed, GREEN PENDING | n/a | PENDING |

Assumption: the client exposes no HTTP status, so the add path treats server code `invalid_request` as the validation reject (400/422); any other definitive code shows `readFailed`.
