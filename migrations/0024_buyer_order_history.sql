-- Owner-scoped newest-first history. No new privilege or identity linkage;
-- checkout runtime continues to use the existing forced owner RLS policy.
CREATE INDEX checkout_owner_history ON checkout.orders
 (tenant_id, store_id, owner_id, created_at DESC, id DESC);
