-- Purpose: forward-only removal of the never-deployed W4-01B PAYUNi NotifyURL receiver (created by 0136):
--   the commerce_payuni_ingress role, the endpoint/receipt tables, the resolve/record/set-endpoint/guard
--   definers and the review_cases relaxations (NOTIFY_MISMATCH reason, nullable source_report_hash).
-- Depends on: 0136 objects; payments.review_cases pre-0136 shape from 0018/0061/0062. The matching
--   river_payment.river_job wake-grant REVOKE lives in the post-River phase of migrations/migrate.go —
--   on a fresh database the River tables do not exist yet while this numbered file runs.
-- Used by: migrations.Apply (numbered phase); tests/foundation/payuni_removal_test.go (RM01/RM02).
-- Invariants: never delete data silently — refuses with SQLSTATE 22023 when any notify receipt row or
--   NOTIFY_MISMATCH review case exists (owner 2026-10-06 「不使用PAYUNi…」; W4-02B/W4-03B cancelled).
--   The older PAYUNi hosted/query/capture path (0014–0018, apply_capture_payuni_v1) is NOT in scope and stays.

-- 1) Refuse over real data. Both counts run before any DDL, and the numbered phase of migrations.Apply
--    is one transaction, so a refusal leaves the database completely unchanged (RM02 pins this).
DO $$
DECLARE v_receipts bigint; v_reviews bigint;
BEGIN
 SELECT count(*) INTO v_receipts FROM payments.payuni_notify_receipts;
 SELECT count(*) INTO v_reviews FROM payments.review_cases WHERE reason='NOTIFY_MISMATCH';
 IF v_receipts>0 OR v_reviews>0 THEN
  RAISE EXCEPTION 'PAYUNi notify removal refused: % receipt row(s), % NOTIFY_MISMATCH review case(s); an operator must review/archive them first, data is never deleted silently',
   v_receipts, v_reviews USING ERRCODE='22023';
 END IF;
END $$;

-- 2) Restore payments.review_cases to its pre-0136 shape: drop the ingress INSERT policy, remove
--    NOTIFY_MISMATCH from the reason list (inverse of the 0136 regexp, re-derived from the live
--    definition so any intermediate drift is caught instead of guessed), drop the hash-scope pair
--    check, then restore NOT NULL on source_report_hash (safe: step 1 proved no NULL-hash rows exist).
DROP POLICY payuni_notify_review ON payments.review_cases;
DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='payments.review_cases'::regclass AND c.conname='review_cases_reason_check';
 IF v_def IS NULL OR v_def NOT LIKE '%NOTIFY_MISMATCH%' OR v_def NOT LIKE '%REFUND_CONFLICTING%'
  OR v_def !~ ', ''NOTIFY_MISMATCH''::text\]\)' THEN
  RAISE EXCEPTION 'review_cases_reason_check has an unexpected shape: %',v_def; END IF;
 v_def:=regexp_replace(v_def,', ''NOTIFY_MISMATCH''::text\]\)','::text])');
 IF v_def LIKE '%NOTIFY_MISMATCH%' THEN
  RAISE EXCEPTION 'review_cases_reason_check still holds NOTIFY_MISMATCH after the inverse: %',v_def; END IF;
 ALTER TABLE payments.review_cases DROP CONSTRAINT review_cases_reason_check;
 EXECUTE format('ALTER TABLE payments.review_cases ADD CONSTRAINT review_cases_reason_check %s',v_def);
END $$;
ALTER TABLE payments.review_cases DROP CONSTRAINT review_cases_report_hash_scope;
ALTER TABLE payments.review_cases ALTER COLUMN source_report_hash SET NOT NULL;

-- 3) Drop the ingress tables (their policies, the receipt guard trigger and every grant on them go with
--    the tables), then the four definers. Receipts reference endpoints, so the FK decides drop order.
DROP TABLE payments.payuni_notify_receipts;
DROP TABLE payments.payuni_notify_endpoints;
DROP FUNCTION payments.payuni_record_notify(bytea,bytea,text,text,bigint,text,text);
DROP FUNCTION payments.payuni_resolve_endpoint(bytea);
DROP FUNCTION payments.set_payuni_notify_endpoint(uuid,uuid,uuid,uuid,uuid,text,boolean,bytea);
DROP FUNCTION payments.guard_payuni_notify_receipt();

-- 4) Retire the ingress role. It owns nothing (the definers were owned by commerce_integration_writer /
--    commerce_payment_registry_writer, the tables by the migration owner), so after the drops above its
--    only remaining dependency is the schema USAGE grant; no objects are reassigned.
REVOKE USAGE ON SCHEMA payments FROM commerce_payuni_ingress;
DROP ROLE commerce_payuni_ingress;
