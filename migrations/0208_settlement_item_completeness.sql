-- Purpose: #196/#197 make latest sync coverage require identical transaction IDs, currencies and stored nets, including partial chunks.
-- Depends on: 0200 reconciliation rows and the existing record_settlement_lines/close_settlement definers (0150 + 0163 + 0200).
-- Used by: stripe-admin settlement-sync and settlement-close; REAL_PG settlement PF17 regression gate.
-- Invariants: preserve signatures, owners, ACLs, observation order, environment lock, RLS, returns and idempotence; no money moves.
-- Pre-0200 grandfathering is unchanged; existing 0200 comparisons need a resync because their aggregates cannot prove item completeness.

ALTER TABLE payments.settlement_sync_reconciliations
 ADD COLUMN items_complete boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN payments.settlement_sync_reconciliations.items_complete IS
 'payments owner (#196/#197); true only when the manifest and effective stored ledger match by balance transaction ID, currency and net; lines take precedence over unattributed history; immutable with the comparison; existing aggregate-only observations default to false and need a resync';

-- Replace the stored comparison, retaining the exact effective-ledger UNION/anti-join and all validation before writes.
DO $do$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_fn := pg_get_functiondef('payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb)'::regprocedure);
 v_needle := $n$  WITH zs AS (SELECT zm.e->>'currency' AS cur,sum((zm.e->>'net')::bigint)::bigint AS net,count(*)::integer AS n
    FROM jsonb_array_elements(p_window_manifest) zm(e) GROUP BY zm.e->>'currency'),
   zr AS (SELECT u.cur,sum(u.net)::bigint AS net,count(*)::integer AS n FROM (
     SELECT x.settle_currency AS cur,x.settle_net AS net FROM payments.settlement_lines x
      WHERE x.environment=p_environment AND x.txn_created_at>=p_window_from AND x.txn_created_at<p_window_to
     UNION ALL
     SELECT y.settle_currency,y.settle_net FROM payments.settlement_unattributed y
      WHERE y.environment=p_environment AND y.txn_created_at>=p_window_from AND y.txn_created_at<p_window_to
       AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines z WHERE z.balance_txn_id=y.balance_txn_id)) u GROUP BY u.cur)
  INSERT INTO payments.settlement_sync_reconciliations(sync_run_id,settle_currency,tenant_id,store_id,environment,stripe_net_minor,
   recorded_net_minor,stripe_txn_count,recorded_txn_count)
  SELECT v_run,k.cur,p_tenant,p_store,p_environment,coalesce(zs.net,0),coalesce(zr.net,0),coalesce(zs.n,0),coalesce(zr.n,0)
   FROM (SELECT zs0.cur FROM zs zs0 UNION SELECT zr0.cur FROM zr zr0) k LEFT JOIN zs ON zs.cur=k.cur LEFT JOIN zr ON zr.cur=k.cur;$n$;
 v_repl := $r$  WITH manifest_items AS (
    SELECT zm.e->>'id' AS id,zm.e->>'currency' AS cur,(zm.e->>'net')::bigint AS net
     FROM jsonb_array_elements(p_window_manifest) zm(e)),
   recorded_items AS (
    SELECT x.balance_txn_id AS id,x.settle_currency AS cur,x.settle_net AS net FROM payments.settlement_lines x
     WHERE x.environment=p_environment AND x.txn_created_at>=p_window_from AND x.txn_created_at<p_window_to
    UNION ALL
    SELECT y.balance_txn_id,y.settle_currency,(y.settle_amount-y.settle_fee) AS net FROM payments.settlement_unattributed y
     WHERE y.environment=p_environment AND y.txn_created_at>=p_window_from AND y.txn_created_at<p_window_to
      AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines z WHERE z.balance_txn_id=y.balance_txn_id)),
   zs AS (SELECT cur,sum(net)::bigint AS net,count(*)::integer AS n FROM manifest_items GROUP BY cur),
   zr AS (SELECT cur,sum(net)::bigint AS net,count(*)::integer AS n FROM recorded_items GROUP BY cur),
   zi AS (SELECT coalesce(s.cur,r.cur) AS cur,count(*)::integer AS n
    FROM manifest_items s FULL JOIN recorded_items r ON r.id=s.id
    WHERE s.id IS NULL OR r.id IS NULL OR s.cur IS DISTINCT FROM r.cur OR s.net IS DISTINCT FROM r.net
    GROUP BY coalesce(s.cur,r.cur))
  INSERT INTO payments.settlement_sync_reconciliations(sync_run_id,settle_currency,tenant_id,store_id,environment,stripe_net_minor,
   recorded_net_minor,stripe_txn_count,recorded_txn_count,items_complete)
  SELECT v_run,k.cur,p_tenant,p_store,p_environment,coalesce(zs.net,0),coalesce(zr.net,0),coalesce(zs.n,0),coalesce(zr.n,0),coalesce(zi.n,0)=0
   FROM (SELECT zs0.cur FROM zs zs0 UNION SELECT zr0.cur FROM zr zr0) k LEFT JOIN zs ON zs.cur=k.cur LEFT JOIN zr ON zr.cur=k.cur
    LEFT JOIN zi ON zi.cur=k.cur;$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0208 item comparison'; END IF;
 EXECUTE replace(v_fn,v_needle,v_repl);
END $do$;

-- Choose the latest covering observation BEFORE judging cleanliness, exactly as 0200 does. Neither equal sums nor equal counts suffice.
-- Registrar now supplies the full window/manifest on every chunk, so a committed prefix cannot leave an older clean observation usable.
DO $do$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_fn := pg_get_functiondef('payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)'::regprocedure);
 v_needle := $n$WHERE rc.sync_run_id=g.id AND rc.difference_minor<>0$n$;
 v_repl := $r$WHERE rc.sync_run_id=g.id AND (rc.difference_minor<>0 OR NOT rc.items_complete)$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0208 completeness gate'; END IF;
 EXECUTE replace(v_fn,v_needle,v_repl);
END $do$;

COMMENT ON FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb) IS
 'payments owner (#196/#197, 0208); registrar-only operator definer: records and attributes a chunk idempotently; each window observation compares the full validated manifest against effective stored ledger IDs, currencies and nets, storing item mismatches alongside net totals atomically under the environment lock; identical replays recheck unassigned lines; preserves 0163 resolutions and 0200 permissions; moves no money';
COMMENT ON FUNCTION payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text) IS
 'payments owner (#196/#197, 0208); registrar-only operator definer: closes weekly store statements with the existing 72-hour, coverage, attribution, mismatch, return-carry and idempotence rules; refuses sync_unreconciled before writes when the latest covering observation has any net difference or item mismatch, including an incomplete chunk; preserves 0200 owners and grants; moves no money';
