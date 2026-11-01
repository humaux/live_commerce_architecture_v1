# FEAT-BUYER-RMA-FIX2 — buyerhttp.Routes candidate (independent review pending)

Authority: integrator's explicit 2026-10-31 grant in the dispatched FIX2 card,
corroborated by Humaux human record `a93bc0b3-7673-4bf5-bbb2-ed233a8fa11a`.
PR #395, branch `unit/feat-buyer-rma`, target `r3/integration`.
Merged integration base: `629f42152f864621a99ac2247c3baf5d751a422f`.
This is a candidate and closure explanation, **not independent approval**.

## Fingerprint and provenance

- Old SHA: `bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a`.
- New SHA: `aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05`.
- Original source: [Gates 41096837305 / plan 124208924912](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41096837305/job/124208924912),
  which printed the exact new candidate and old reviewed SHA. Filtered original
  line: `output/FEAT-BUYER-RMA/FIX2-current-red.log`.
- That run's PR head is `d699e74ea6c7c0fa3d4e4683b895b27c86f09136`.
  `git diff d699e74ea6c7c0fa3d4e4683b895b27c86f09136 -- internal/buyerhttp`
  is empty after the mandatory integration merge. Thus the entire package input
  to the original fingerprint is unchanged. No locally invented hashing tool or
  scanner/test execution was used. Current-head CI must confirm the candidate.
- `reviewed` points here and explicitly says independent review is pending.
  Only the `buyerhttp.Routes` entry changes; the feed entry is untouched.

## Every added or changed closure member

| Member | Change | Why it belongs to the frozen closure |
| --- | --- | --- |
| `handler.ServeHTTP` | Changed | Calls `isReturnRequestRoute` to set private/no-store before the existing admission/dispatch flow. This is the existing HTTP dispatch root. |
| `handler.dispatch` | Changed | Adds the four return-request kinds and directly calls `h.returnRequest`. This existing consumer reaches the new helper. |
| `isReturnRequestRoute` | Added function and declaration | Referenced directly by ServeHTTP; compares the closed kind range, without changing route parsing or authorization. |
| `handler.returnRequest` | Added function and declaration | Referenced directly by dispatch; uses existing strict decoding, body refusal, scoped transaction and sanitized response errors. Its receiver references the existing handler type. |
| `const routeReturnRequestCreate,routeReturnRequestDetail,routeReturnRequestList,routeReturnRequestWithdraw` | Added whole const-block declaration | Referenced by both new helpers and dispatch; the whole iota block is frozen to preserve values. Declaration names are sorted; source order remains create/list/detail/withdraw. |

Functions: **82 → 84**. Declarations: **151 → 154**. No removed member;
all existing same-package functions/types/vars/const blocks remain frozen.
The handler header comment is the only other package edit and is excluded from
canonical hashing. Four literal `routeRows` additions are data, excluded by the
existing rule; they already use the neighbouring Method/Path/kind shape.

Cross-package boundaries are unchanged: `returns.CreateBuyerRequest`,
`returns.DecideBuyerRequest`, `returns.ReadBuyerRequests`, buyer scope and
platform errors remain subject to their contract/runtime gates and review.
They are not claimed frozen by this same-package pin. Read
`contracts/returns-v1.md` Amendment 0210 and the affected returns transport/domain
code during independent acceptance.

## Companion inventory change and reviewer handoff

The independent repository AST inventory increases **46 + 4 = 50**, exactly the
four added buyer endpoints in Amendment 0210. Every per-row location/proof,
unresolved-route, and legacy-mount assertion stays unchanged. No extractor,
baseline, allow-list, gate-adapter assertion or fixture changes.

Independent reviewer must read the full pinned closure per
`docs/engineering/contractdrift.md`, confirm the delta above against the merged
PR, and bind approval to the actual PR head. This implementer does not approve
the candidate. Author judgement E0; local static readiness E1; current-head CI
and independent acceptance remain pending. No local tests or servers ran.
