P2 for issue

Review #294: `apps/admin/components/InboxThread.tsx` compares RFC3339 strings in nextReply and latest-outbound selection. Mixed fractional precision can reverse chronological order: inbound `2026-10-29T06:35:00Z` incorrectly sorts after outbound `2026-10-29T06:35:00.500Z`; inbound `.110Z` incorrectly sorts before outbound `.1Z`.

Integrator: open the module issue. Fix by comparing instants and add same-second mixed-precision cases for next composer eligibility and latest outbound selection. Deferred under RVFIX card's P2 instruction; this P1 repair does not claim it fixed.
