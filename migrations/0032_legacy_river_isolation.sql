-- Native family schemas are initialized by rivermigrate before post0005.
-- A partially completed Apply must never advertise either old lane as ready.
CREATE SCHEMA river_payment;
CREATE SCHEMA river_expiry;
REVOKE ALL ON SCHEMA river_payment,river_expiry FROM PUBLIC;

-- A fresh database has no post0001/0002 functions yet; absent is already
-- fail-closed. Do not pre-create them and break historical CREATE statements.
DO $$ BEGIN
 IF to_regprocedure('integration.payment_queue_ready()') IS NOT NULL THEN
  EXECUTE $fn$CREATE OR REPLACE FUNCTION integration.payment_queue_ready()
   RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS 'SELECT false'$fn$;
 END IF;
 IF to_regprocedure('checkout.expiry_queue_ready()') IS NOT NULL THEN
  EXECUTE $fn$CREATE OR REPLACE FUNCTION checkout.expiry_queue_ready()
   RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS 'SELECT false'$fn$;
 END IF;
END $$;
-- Replacements retain original owners and EXECUTE grants. No destination
-- producer/lifecycle privileges are installed until the atomic post phase.
