-- 0215_customer_consent_state.sql (unit PARITY-CATALOG-PEOPLE, P1-1 RVFIX).
-- Purpose: compute customer consent_state ('never', 'granted', 'withdrawn') in SQL inside
--   identity.read_merchant_customers projection, eliminating N+1 detail reads in List.
-- Depends on: 0078 (customers schema, customers.consent_events), 0139 (customer tags/notes, 8-arg read_merchant_customers),
--   0152 (customer import profiles, 8-arg read_merchant_customers with imported projection).
-- Used by: internal/customers/read.go (List and Get/loadDetail decodes consent_state strictly from the row).
-- Invariants: identical signature, SECURITY DEFINER, search_path=pg_catalog, owner commerce_auth, and EXECUTE
--   grant to commerce_runtime as 0152; no new GRANT, no new writer, no table or column change, no schema mutation;
--   subquery matches 'consents' purpose/channel latest-event ordering; NULL -> 'never', true -> 'granted', false -> 'withdrawn'.

CREATE OR REPLACE FUNCTION identity.read_merchant_customers(p_hash bytea,p_store uuid,p_customer uuid,p_limit integer,
 p_after_ts timestamptz,p_after_id uuid,p_q text,p_tag uuid DEFAULT NULL)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_result jsonb; v_auth_error text; v_digits text; v_phone boolean:=false;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101
  OR (p_after_ts IS NULL)<>(p_after_id IS NULL)
  OR (p_after_ts IS NOT NULL AND NOT isfinite(p_after_ts))
  OR (p_customer IS NOT NULL AND (p_limit<>1 OR p_after_id IS NOT NULL OR p_q IS NOT NULL OR p_tag IS NOT NULL))
  OR (p_q IS NOT NULL AND (char_length(p_q) NOT BETWEEN 1 AND 40 OR p_q<>btrim(p_q) OR p_q ~ '[[:cntrl:]]'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid customer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'customers:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- D6: digits (after removing spaces, '-' and '+') = phone-digits suffix; anything else = recipient_name prefix.
 IF p_q IS NOT NULL THEN
  v_digits:=regexp_replace(p_q,'[ +-]','','g');
  v_phone:=v_digits ~ '^[0-9]+$';
 END IF;

 -- ponytail: read-time aggregates (CD3); materialize when p95 > 300 ms at a measured store size (§9).
 -- The row set is driven from owners that HAVE activity (orders / bound bundles), never from every buyer.owners row:
 -- buyer.issue_capability creates an owner per anonymous visitor, so scanning owners made each page O(visitors)
 -- (lane-close review P2, CB03 10k idle owners). The per-customer aggregates below then run over customers only.
 WITH active_owner AS MATERIALIZED (
  -- W6-01B: tag filter. The active owners are restricted to the carriers of p_tag (an index probe on
  -- customer_owner_tags_by_tag through the set-returning helper), so the search starts from the tagged set, not from every
  -- customer. An unknown or other-store tag id matches nobody.
  SELECT o.owner_id AS id FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND (p_customer IS NULL OR o.owner_id=p_customer)
   AND (p_tag IS NULL OR o.owner_id IN (SELECT customers.tn_owners_with_tag(s.tenant_id,p_store,p_tag)))
  UNION
  SELECT b.owner_id FROM claims.bundles b WHERE b.tenant_id=s.tenant_id AND b.store_id=p_store AND b.owner_id IS NOT NULL
   AND (p_customer IS NULL OR b.owner_id=p_customer)
   AND (p_tag IS NULL OR b.owner_id IN (SELECT customers.tn_owners_with_tag(s.tenant_id,p_store,p_tag)))
  UNION
  -- W5-02B: an imported customer (customers.import_profiles) is a customer without an order or a bound bundle.
  SELECT ip.owner_id FROM customers.import_profiles ip WHERE ip.tenant_id=s.tenant_id AND ip.store_id=p_store
   AND (p_customer IS NULL OR ip.owner_id=p_customer)
   AND (p_tag IS NULL OR ip.owner_id IN (SELECT customers.tn_owners_with_tag(s.tenant_id,p_store,p_tag)))
 ), base AS MATERIALIZED (
  SELECT ow.tenant_id,ow.store_id,ow.id,ow.created_at AS first_seen,ow.active,
   greatest(
    (SELECT max(o.created_at) FROM checkout.orders o WHERE o.tenant_id=ow.tenant_id AND o.store_id=ow.store_id AND o.owner_id=ow.id),
    (SELECT max(b.bound_at) FROM claims.bundles b WHERE b.tenant_id=ow.tenant_id AND b.store_id=ow.store_id AND b.owner_id=ow.id),
    -- W5-02B: imported_at never changes, so the keyset on (last_activity,id) stays stable across re-imports.
    (SELECT ip.imported_at FROM customers.import_profiles ip WHERE ip.tenant_id=ow.tenant_id AND ip.store_id=ow.store_id AND ip.owner_id=ow.id)
   ) AS last_activity
  FROM active_owner ao
  -- LATERAL ... LIMIT 1 forces one primary-key probe per customer: without it the planner (no statistics right after a
  -- bulk insert of visitors) chose a hash join that read the whole owners table again.
  CROSS JOIN LATERAL (SELECT w.tenant_id,w.store_id,w.id,w.created_at,w.active FROM buyer.owners w
    WHERE w.id=ao.id AND w.tenant_id=s.tenant_id AND w.store_id=p_store LIMIT 1) ow
  WHERE (p_customer IS NULL OR ow.id=p_customer)
   AND (p_q IS NULL OR EXISTS(SELECT 1 FROM checkout.orders qo WHERE qo.tenant_id=ow.tenant_id AND qo.store_id=ow.store_id
     AND qo.owner_id=ow.id AND CASE WHEN v_phone
      THEN right(regexp_replace(coalesce(qo.snapshot#>>'{destination,phone}',''),'[^0-9]','','g'),char_length(v_digits))=v_digits
      ELSE starts_with(lower(coalesce(qo.snapshot#>>'{destination,recipient_name}','')),lower(p_q)) END)
    -- W5-02B: the same D6 rule over the import profile.
    OR EXISTS(SELECT 1 FROM customers.import_profiles qp WHERE qp.tenant_id=ow.tenant_id AND qp.store_id=ow.store_id
     AND qp.owner_id=ow.id AND CASE WHEN v_phone
      THEN right(regexp_replace(regexp_replace(coalesce(qp.phone_e164,''),'^\+886','0'),'[^0-9]','','g'),char_length(v_digits))=v_digits
      ELSE starts_with(lower(qp.display_name),lower(p_q)) END))
 ), paged AS MATERIALIZED (
  SELECT b.* FROM base b
  WHERE b.last_activity IS NOT NULL AND (p_after_id IS NULL OR (b.last_activity,b.id)<(p_after_ts,p_after_id))
  ORDER BY b.last_activity DESC,b.id DESC LIMIT p_limit
 ), projected AS (
  SELECT pg.last_activity,pg.id,
   jsonb_build_object(
    'customer_id',pg.id,
    'first_seen_at',to_char(pg.first_seen AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'last_activity_at',to_char(pg.last_activity AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'display_name',coalesce(lo.name,ip.display_name),
    'phone_last3',coalesce(lo.last3,ip.last3),
    'orders_count',coalesce(ag.orders_count,0),'paid_orders_count',coalesce(ag.paid_orders_count,0),
    'captured_minor',coalesce(ag.captured_minor,0),'refunded_minor',coalesce(ag.refunded_minor,0),
    'currency',lo.currency,
    'claims_count',coalesce(cl.claims_count,0),'platforms',coalesce(cl.platforms,'[]'::jsonb),
    'consents',jsonb_build_object(
     'marketing_messages',coalesce((SELECT e.granted FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id
       AND e.store_id=pg.store_id AND e.owner_id=pg.id AND e.purpose='marketing_messages' AND e.channel='meta_dm'
       ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1),false),
     'ads_personalization',coalesce((SELECT e.granted FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id
       AND e.store_id=pg.store_id AND e.owner_id=pg.id AND e.purpose='ads_personalization' AND e.channel='meta_ads'
       ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1),false)),
    'consent_state',jsonb_build_object(
     'marketing_messages',CASE (SELECT e.granted FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id
       AND e.store_id=pg.store_id AND e.owner_id=pg.id AND e.purpose='marketing_messages' AND e.channel='meta_dm'
       ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1)
      WHEN true THEN 'granted' WHEN false THEN 'withdrawn' ELSE 'never' END,
     'ads_personalization',CASE (SELECT e.granted FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id
       AND e.store_id=pg.store_id AND e.owner_id=pg.id AND e.purpose='ads_personalization' AND e.channel='meta_ads'
       ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1)
      WHEN true THEN 'granted' WHEN false THEN 'withdrawn' ELSE 'never' END),
    'active',pg.active,
    'imported',ip.owner_id IS NOT NULL,
    -- W6-01B: merchant-typed store tags, name-ordered; the only tag key of the list row.
    'tags',customers.tn_tags_json(pg.tenant_id,pg.store_id,pg.id))
   ||CASE WHEN p_customer IS NULL THEN '{}'::jsonb ELSE jsonb_build_object(
    -- W6-01B detail extras: CAS token for PUT .../tags and the newest 50 merchant notes (older ones: customers.list_notes).
    'tags_revision',customers.tn_tags_revision(pg.tenant_id,pg.store_id,pg.id),
    'notes',customers.tn_notes_json(pg.tenant_id,pg.store_id,pg.id,50,NULL,NULL),
    'order_ids',coalesce((SELECT jsonb_agg(x.id ORDER BY x.created_at DESC,x.id DESC)
       FROM (SELECT o.id,o.created_at FROM checkout.orders o WHERE o.tenant_id=pg.tenant_id AND o.store_id=pg.store_id
         AND o.owner_id=pg.id ORDER BY o.created_at DESC,o.id DESC LIMIT 201) x),'[]'::jsonb),
    'claims',coalesce((SELECT jsonb_agg(jsonb_build_object('session_id',b.session_id,'platform',b.platform,
       'bound_at',to_char(b.bound_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'line_count',b.line_count)
       ORDER BY b.bound_at DESC,b.id DESC)
       FROM (SELECT c.id,c.session_id,c.platform,c.bound_at,c.line_count FROM claims.bundles c
         WHERE c.tenant_id=pg.tenant_id AND c.store_id=pg.store_id
         AND c.owner_id=pg.id ORDER BY c.bound_at DESC,c.id DESC LIMIT 100) b),'[]'::jsonb),
    'consent_history',coalesce((SELECT jsonb_agg(jsonb_build_object('purpose',x.purpose,'channel',x.channel,
       'granted',x.granted,'source',x.source,'policy_version',x.policy_version,
       'occurred_at',to_char(x.occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
       ORDER BY x.occurred_at DESC,x.id DESC)
       FROM (SELECT e.* FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id AND e.store_id=pg.store_id
         AND e.owner_id=pg.id ORDER BY e.occurred_at DESC,e.id DESC LIMIT 500) x),'[]'::jsonb),
    'privacy_actions',coalesce((SELECT jsonb_agg(jsonb_build_object('kind',y.kind,'via',y.via,
       'completed_at',to_char(y.completed_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'summary',y.summary)
       ORDER BY y.completed_at DESC,y.id DESC)
       FROM (SELECT a.* FROM customers.privacy_actions a WHERE a.tenant_id=pg.tenant_id AND a.store_id=pg.store_id
         AND a.owner_id=pg.id ORDER BY a.completed_at DESC,a.id DESC LIMIT 100) y),'[]'::jsonb)) END AS value
  FROM paged pg
  -- D7: display name, phone last 3 and currency come from the LATEST order's destination (null for bundle-only owners).
  LEFT JOIN LATERAL (
   SELECT o.currency,o.snapshot#>>'{destination,recipient_name}' AS name,
    right(regexp_replace(coalesce(o.snapshot#>>'{destination,phone}',''),'[^0-9]','','g'),3) AS last3
   FROM checkout.orders o WHERE o.tenant_id=pg.tenant_id AND o.store_id=pg.store_id AND o.owner_id=pg.id
   ORDER BY o.created_at DESC,o.id DESC LIMIT 1
  ) lo ON true
  -- W5-02B: imported name and phone tail for a customer without orders (an order's destination wins when both exist).
  LEFT JOIN LATERAL (
   SELECT p.owner_id,p.display_name,
    nullif(right(regexp_replace(regexp_replace(coalesce(p.phone_e164,''),'^\+886','0'),'[^0-9]','','g'),3),'') AS last3
   FROM customers.import_profiles p WHERE p.tenant_id=pg.tenant_id AND p.store_id=pg.store_id AND p.owner_id=pg.id LIMIT 1
  ) ip ON true
  -- I05: money sums cover only the latest order's currency (single-currency stores in v1). "Captured" uses the same
  -- fact-matches-attempt-matches-order predicate as identity.read_merchant_orders; refunded = held (no FAILED/CANCELED/
  -- REJECTED fact) with a SUCCEEDED fact, the stripe-refund-v1 §7.1 rule.
  LEFT JOIN LATERAL (
   SELECT count(*) AS orders_count,count(*) FILTER (WHERE x.captured) AS paid_orders_count,
    coalesce(sum(x.total_minor) FILTER (WHERE x.captured AND x.currency=lo.currency),0)::bigint AS captured_minor,
    coalesce(sum(x.refunded) FILTER (WHERE x.captured AND x.currency=lo.currency),0)::bigint AS refunded_minor
   FROM (
    SELECT o.total_minor,o.currency,
     EXISTS(SELECT 1 FROM checkout.payment_attempts a JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id
       AND f.attempt_id=a.id AND f.kind='CAPTURED' AND f.connection_id=a.connection_id AND f.execution_profile=a.execution_profile
       AND f.environment=a.environment AND f.currency=a.currency AND f.amount_minor=a.amount_minor
       AND f.currency=o.currency AND f.amount_minor=o.total_minor
      WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.owner_id=o.owner_id AND a.order_id=o.id) AS captured,
     coalesce((SELECT sum(r.amount_minor) FROM checkout.payment_attempts a JOIN payments.stripe_refunds r
        ON r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.attempt_id=a.id
       WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.owner_id=o.owner_id AND a.order_id=o.id
        AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
          AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'))
        AND EXISTS(SELECT 1 FROM payments.refund_facts rs WHERE rs.tenant_id=r.tenant_id AND rs.store_id=r.store_id
          AND rs.refund_id=r.id AND rs.kind='SUCCEEDED')),0) AS refunded
    FROM checkout.orders o WHERE o.tenant_id=pg.tenant_id AND o.store_id=pg.store_id AND o.owner_id=pg.id
   ) x
  ) ag ON true
  LEFT JOIN LATERAL (
   SELECT count(*) AS claims_count,coalesce(jsonb_agg(DISTINCT c.platform ORDER BY c.platform),'[]'::jsonb) AS platforms
   FROM claims.bundles c WHERE c.tenant_id=pg.tenant_id AND c.store_id=pg.store_id AND c.owner_id=pg.id
  ) cl ON true
 )
 SELECT coalesce(jsonb_agg(value ORDER BY last_activity DESC,id DESC),'[]'::jsonb) INTO v_result FROM projected;

 -- Fresh final fence AFTER the data reads, BEFORE the empty/not-found branches (no post-revocation existence oracle).
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['customers:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'customer read access denied' USING ERRCODE=v_auth_error; END IF;
 IF p_customer IS NOT NULL AND jsonb_array_length(v_result)=0 THEN
  RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
 IF octet_length(v_result::text)>524288 THEN
  RAISE EXCEPTION 'customer read unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;

ALTER FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) TO commerce_runtime;

COMMENT ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) IS
 'internal/customers (customers:read): list (limit 1..101, keyset on (last_activity_at,id) DESC, optional p_tag filter) or detail (p_customer). Rows carry tags, imported and consent_state (0215: computed in SQL, distinguishes never/granted/withdrawn); the detail adds tags_revision and the newest 50 notes.';
