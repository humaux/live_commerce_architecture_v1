-- Purpose: serialize original-order award recovery with cancellation and LP10 credits.
-- Depends on: 0094 merchant manual source, 0139 scoped authority, 0223 loyalty ledger.
-- Used by: internal/loyalty and the order member-award-recovery endpoint.

-- Row locks need an UPDATE privilege. RLS permits scoped locks and rejects every actual update.
GRANT SELECT (created_at), UPDATE (commercial_state) ON checkout.orders TO commerce_privacy_writer;
CREATE POLICY loyalty_order_lock ON checkout.orders FOR UPDATE TO commerce_privacy_writer
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK (false);
CREATE POLICY loyalty_order_lock_only ON checkout.orders AS RESTRICTIVE FOR UPDATE TO commerce_privacy_writer
 USING (true) WITH CHECK (false);

CREATE POLICY loyalty_manual_receipt_read ON ops.command_results FOR SELECT TO commerce_privacy_writer
 USING (operation='merchanttools.order.manual'
     AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE INDEX command_results_manual_order ON ops.command_results (tenant_id,store_id,(response->>'order_id'))
 WHERE operation='merchanttools.order.manual';

CREATE OR REPLACE FUNCTION loyalty.adjust_points(h bytea,s uuid,i uuid,d integer,n text,o uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;e uuid;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');
 IF d IS NULL OR d=0 OR d NOT BETWEEN -1000000 AND 1000000 OR n IS NULL OR char_length(n) NOT BETWEEN 1 AND 200 OR btrim(n)='' THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END IF;
 -- A positive compensation for any member must serialize with recovery's complete order ledger check.
 IF d>0 AND o IS NOT NULL THEN
  PERFORM 1 FROM checkout.orders WHERE tenant_id=a.tenant_id AND store_id=s AND id=o FOR NO KEY UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 END IF;
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.erased_at IS NOT NULL THEN RAISE EXCEPTION 'member_erased' USING ERRCODE='PT409';END IF;
 IF d<0 AND o IS NOT NULL AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=a.tenant_id AND store_id=s AND id=o) THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.balance+d<0 THEN RAISE EXCEPTION 'points_insufficient' USING ERRCODE='PT409';END IF;
 IF m.balance+d>1000000000000 THEN RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT422';END IF;
 INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,note,order_id,actor_principal_id) VALUES(a.tenant_id,s,i,d,'manual_adjust',n,o,a.principal_id) RETURNING id INTO e;
 UPDATE loyalty.members SET balance=balance+d,version=version+1,updated_at=clock_timestamp() WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
 PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('entry_id',e,'balance',m.balance+d);
END $$;

CREATE FUNCTION loyalty.manual_order_award_recovery(h bytea,s uuid,o uuid,apply boolean) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 a record; ord record; m loyalty.members; e loyalty.point_entries;
 awards jsonb[]; original jsonb; original_state text; i uuid; p integer;
 permission text; state text; refusal text; base jsonb;
BEGIN
 IF o IS NULL OR apply IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';
 END IF;
 permission := CASE WHEN apply THEN 'customers:points' ELSE 'customers:read' END;
 SELECT * INTO a FROM customers.tn_authority(h,s,'orders:read');
 PERFORM customers.tn_authority(h,s,permission);
 IF apply THEN
  SELECT source,commercial_state,created_at INTO ord FROM checkout.orders
   WHERE tenant_id=a.tenant_id AND store_id=s AND id=o FOR NO KEY UPDATE;
 ELSE
  SELECT source,commercial_state,created_at INTO ord FROM checkout.orders
   WHERE tenant_id=a.tenant_id AND store_id=s AND id=o FOR SHARE;
 END IF;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;

 SELECT ARRAY(SELECT response->'member_award' FROM ops.command_results
   WHERE tenant_id=a.tenant_id AND store_id=s AND operation='merchanttools.order.manual' AND response->>'order_id'=o::text
   LIMIT 2) INTO awards;
 original := awards[1];
 IF cardinality(awards)=1 AND jsonb_typeof(original)='object'
    AND original->>'state' IN ('not_awarded','awarded')
    AND jsonb_typeof(original->'member_id')='string'
    AND original->>'member_id' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    AND jsonb_typeof(original->'points')='number' AND original->>'points' ~ '^[1-9][0-9]{0,6}$' THEN
  p := (original->>'points')::integer;
  IF p<=1000000 THEN
   i := (original->>'member_id')::uuid;
   original_state := original->>'state';
  END IF;
 END IF;
 IF i IS NOT NULL THEN
  IF apply THEN
   SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR UPDATE;
  ELSE
   SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR SHARE;
  END IF;
 END IF;
 SELECT * INTO e FROM loyalty.point_entries
  WHERE tenant_id=a.tenant_id AND store_id=s AND order_id=o AND loyalty.point_entries.reason='manual_order_award';

 IF e.id IS NOT NULL THEN
  IF original_state IS NOT NULL AND (e.member_id<>i OR e.delta<>p
     OR (original_state='awarded' AND original->>'entry_id' IS DISTINCT FROM e.id::text)) THEN
   state:='review_required'; refusal:='award_review_required';
  ELSE
   state:='already_awarded'; refusal:='award_already_recorded';
  END IF;
 ELSIF ord.source<>'merchant_manual' OR ord.commercial_state IN ('DRAFT','CANCELLED')
       OR (cardinality(awards)=1 AND original IS NULL) THEN
  state:='not_eligible'; refusal:='award_not_eligible';
 ELSIF original_state IS DISTINCT FROM 'not_awarded' OR original->'entry_id' IS NOT NULL OR m.id IS NULL THEN
  state:='review_required'; refusal:='award_review_required';
 ELSIF m.erased_at IS NOT NULL THEN
  state:='not_eligible'; refusal:='member_erased';
 ELSIF EXISTS (SELECT 1 FROM loyalty.point_entries x
    WHERE x.tenant_id=a.tenant_id AND x.store_id=s AND x.reason='manual_adjust' AND x.delta>0 AND x.order_id=o)
    OR EXISTS (SELECT 1 FROM loyalty.point_entries x
    WHERE x.tenant_id=a.tenant_id AND x.store_id=s AND x.reason='manual_adjust' AND x.delta>0
      AND x.order_id IS NULL AND x.member_id=i AND x.created_at>=ord.created_at) THEN
  state:='review_required'; refusal:='award_review_required';
 ELSIF m.balance+p>1000000000000 THEN
  state:='not_eligible'; refusal:='limit_reached';
 ELSE
  state:='eligible';
 END IF;

 IF apply THEN
  IF state='eligible' THEN
   INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,order_id,actor_principal_id)
    VALUES(a.tenant_id,s,i,p,'manual_order_award',o,a.principal_id)
    ON CONFLICT (tenant_id,store_id,order_id) WHERE reason='manual_order_award' DO NOTHING RETURNING * INTO e;
   IF FOUND THEN
    UPDATE loyalty.members SET balance=balance+p,version=version+1,updated_at=clock_timestamp()
     WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
    state:='awarded';
   ELSE
    SELECT * INTO STRICT e FROM loyalty.point_entries
     WHERE tenant_id=a.tenant_id AND store_id=s AND order_id=o AND loyalty.point_entries.reason='manual_order_award';
    IF e.member_id<>i OR e.delta<>p THEN RAISE EXCEPTION 'award_review_required' USING ERRCODE='PT409';END IF;
    state:='already_awarded';
   END IF;
  ELSIF state='already_awarded' AND original_state='not_awarded' AND original->'entry_id' IS NULL THEN
   refusal:=NULL;
  ELSIF refusal='limit_reached' THEN
   RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT422';
  ELSE
   RAISE EXCEPTION '%',refusal USING ERRCODE='PT409';
  END IF;
 END IF;
 PERFORM customers.tn_fence(h,s,permission,a.tenant_id,a.principal_id,a.authz_revision);
 PERFORM customers.tn_fence(h,s,'orders:read',a.tenant_id,a.principal_id,a.authz_revision);
 base:=jsonb_build_object('order_id',o,'state',state);
 IF refusal IS NOT NULL THEN base:=base||jsonb_build_object('reason',refusal);END IF;
 IF state IN ('awarded','already_awarded') THEN
  RETURN base||jsonb_build_object('member_id',e.member_id,'points',e.delta,'entry_id',e.id);
 END IF;
 IF state='eligible' THEN RETURN base||jsonb_build_object('member_id',i,'points',p);END IF;
 RETURN base;
END $$;
ALTER FUNCTION loyalty.manual_order_award_recovery(bytea,uuid,uuid,boolean) OWNER TO commerce_privacy_writer;
REVOKE ALL ON FUNCTION loyalty.manual_order_award_recovery(bytea,uuid,uuid,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION loyalty.manual_order_award_recovery(bytea,uuid,uuid,boolean) TO commerce_runtime;
