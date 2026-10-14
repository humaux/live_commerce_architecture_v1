-- Purpose: forbid committing partial OA1 materialization after a real C2 start.
-- Depends on: 0170 terminal identity/RLS, 0171 native order provenance, 0178 xid fence.
-- Used by: deferred attempt-cart INSERT constraint at the outer C2 commit boundary.
-- Invariants: a lock-only PENDING Tx remains legal; surviving attempt carts require
-- current-fence COMMITTED authority and a native order with their exact identity.
-- Full source/pay/claim completion remains the composite executor's responsibility.

CREATE FUNCTION checkout.order_attempt_order_matches(
 p_tenant uuid,p_store uuid,p_owner uuid,p_attempt uuid,p_cart uuid,p_order uuid)
RETURNS boolean LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM checkout.orders o
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.owner_id=p_owner
   AND o.create_attempt_id=p_attempt AND o.cart_id=p_cart AND o.id=p_order
   AND o.creator_kind='order_attempt' AND o.creator_session_id IS NULL AND o.cart_version=1)
$$;
ALTER FUNCTION checkout.order_attempt_order_matches(uuid,uuid,uuid,uuid,uuid,uuid)
 OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_order_matches(uuid,uuid,uuid,uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_order_matches(uuid,uuid,uuid,uuid,uuid,uuid)
 TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_order_matches(uuid,uuid,uuid,uuid,uuid,uuid) IS
 'Private native order proof for deferred C2 commit: exact tenant/store/owner/attempt/cart/order, attempt creator and cart version. Existing checkout writer SELECT/RLS only; no locks, GUC mutation, caller-visible order data or new coordinator table grants.';

CREATE FUNCTION ops.check_order_attempt_execution_commit() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
BEGIN
 -- Deferred NEW is the trusted inserted cart, not the final attempt version.
 -- Entries restore caller context before COMMIT. Scope this final metadata read
 -- from NEW and use existing tenant/store RLS, which also admits terminal rows.
 -- Do not call the PENDING-only execution/fence context or widen its locator.
 PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true);
 PERFORM set_config('app.store_id',NEW.store_id::text,true);
 SELECT x.* INTO a FROM ops.order_attempts x
  WHERE x.attempt_id=NEW.create_attempt_id AND x.tenant_id=NEW.tenant_id
   AND x.store_id=NEW.store_id AND x.buyer_owner_id=NEW.owner_id;
 IF NOT FOUND OR NOT a.admission_finished OR a.state IS DISTINCT FROM 'COMMITTED'
  OR a.order_id IS NULL OR NOT EXISTS(SELECT 1 FROM ops.order_attempt_execution_fences f
   WHERE f.attempt_id=a.attempt_id AND f.tenant_id=a.tenant_id AND f.store_id=a.store_id
    AND f.execution_xid=pg_current_xact_id()) THEN
  RAISE EXCEPTION 'order attempt execution is unfinished' USING ERRCODE='23514';
 END IF;
 -- 0170 binds receipt state/attempt/generation/order to this immutable terminal
 -- row. A matching opaque pointer alone is insufficient: prove the native order.
 -- Its existing provenance FKs bind the exact quote, destination and reservation.
 IF checkout.order_attempt_order_matches(NEW.tenant_id,NEW.store_id,NEW.owner_id,
  NEW.create_attempt_id,NEW.id,a.order_id) IS NOT TRUE THEN
  RAISE EXCEPTION 'order attempt order is missing' USING ERRCODE='23514';
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN NULL;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.check_order_attempt_execution_commit() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.check_order_attempt_execution_commit() FROM PUBLIC;
COMMENT ON FUNCTION ops.check_order_attempt_execution_commit() IS
 'Internal deferred attempt-cart INSERT guard: final exact COMMITTED attempt, current protected execution xid, and native order/cart identity are required at commit. Lock-only PENDING has no cart event. No scope/subject/domain locks, deadline restart, public execution or fixture bypass; caller tenant/store GUCs restored.';
CREATE CONSTRAINT TRIGGER order_attempt_execution_commit AFTER INSERT ON storefront.carts
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.cart_kind='order_attempt')
 EXECUTE FUNCTION ops.check_order_attempt_execution_commit();
