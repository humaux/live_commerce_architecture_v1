# Unit meta-multi-page — one store connects several Facebook Pages / Instagram accounts (R5)

Owner decision 2026-10-02: upgrade the pilot first, then close this gap.

Evidence (docs/discovery/2026-10-02-shopline-admin-study.md §3/§6):
- SHOPLINE lets a store connect up to 10 Pages, and the pilot merchant uses two (渠道仓-香港 and 大梦甄选女包), each with its own live and post selling.
- Ours allows exactly one: `integration.meta_connections PRIMARY KEY(tenant_id,store_id)`. meta-connect D1 (migration 0100, per-store advisory lock) enforces one Page per store.

Read: contracts/meta-claims-intake-v1.md ("Merchant connect (R4)" + §7), migrations/0064, 0095, 0100, internal/metaconnect, internal/integrations/metareply (Unsubscriber), the claims-intake routing (Page → store binding), the admin Settings > Facebook card and the Studio claim-source picker.

## Decisions
1. **Connections are per (store, page_id).**
   - At most 10 per store. A page_id stays UNIQUE platform-wide, so one Page never feeds two stores.
   - Instagram professional accounts attach to their Page.
   - Forward migration: re-key the table and backfill the existing row.
2. **Connect adds or refreshes a Page.**
   - Connect never replaces another Page. D1's race guard becomes "the same Page cannot be connected twice concurrently", and the cap of 10 is checked under the store lock.
   - Disconnect is per Page. The D2 unsubscribe job is per Page and must be kept.
3. **Claim sources:** a Studio session binds one specific Page post or live. The picker lists every connected Page. Intake routing stays keyed by page_id, which already exists.
4. **Page tokens:** custody is unchanged (HPKE meta-page-token-v2, only the claims-worker opens tokens). One sealed token per Page.
5. **Admin:** Settings > Facebook lists the connected Pages with add, disconnect and re-auth actions, and shows the "N / 10" cap.

## Gate
- **PG:** re-key and backfill on populated data; cap of 10; page_id uniqueness across stores; per-Page disconnect plus unsubscribe; concurrent connect of the same Page and of different Pages.
- **Go:** connect flow selects multiple Pages; intake routes a comment to the right store and session per Page.
- **Browser:** `--browser-meta-connect` extended: connect Page A and Page B → both listed → Studio picker offers both → disconnect B leaves A working.
- **Upgrade:** the R2 count and KC03/T06 ACL inventories follow. The meta-connect independent gate is re-run.
