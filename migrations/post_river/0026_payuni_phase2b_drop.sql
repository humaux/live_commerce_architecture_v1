-- Purpose: PAY-RM2 phase 2b, post-River half: drop the two PAYUNi execution definers that post_river/0005
--   CREATE OR REPLACEs (checkout.start_payment, integration.record_payment_query) with their EXECUTE grants.
-- Depends on: numbered 0197_payuni_phase2b_drop.sql (its unreconciled-PAYUNi refusal ran in the numbered phase
--   of the same migrations.Apply, before this file); post_river/0005 bodies.
-- Used by: migrations.Apply (post-River phase); tests/foundation/payuni_phase2b_test.go (P2B01-P2B03).
-- Invariants: on a fresh database 0197 runs before post_river/0005 recreates these two, so the drop must live
--   here or a fresh install and an upgrade would diverge. No data is touched; no CASCADE.

DROP FUNCTION checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint);
DROP FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint);

DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE (n.nspname,p.proname) IN (('checkout','start_payment'),('integration','record_payment_query'))) THEN
  RAISE EXCEPTION 'PAYUNi phase-2b drop left a legacy definer overload'; END IF;
END $$;
