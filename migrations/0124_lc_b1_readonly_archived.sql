-- Purpose: LC-B1 K3 fix (integrator ruling 2026-10-05, F-ARCH-OFFER-*/F-ARCH-TIMELINE): an archived live session is read-only
--   at the database level for its offers and offer_timeline too (0122 only froze live.sessions itself). Offer INSERT/UPDATE and
--   offer_timeline INSERT on an archived session raise 23514 'session archived' (-> command.ErrInvalid, same fixed class as
--   guard_archived_session), whichever code path (offer create/update, keyword-library apply, RecordOfferFeatured) writes.
-- Depends on: live.sessions (lifecycle, 0122), live.offers (0060), live.offer_timeline (0122), commerce_runtime role.
-- Used by: internal/claims/merchant.go (CreateOffer/UpdateOffer), internal/claims/keyword_library.go, internal/live/lifecycle.go
--   (RecordOfferFeatured). No Go change needed; tests/foundation/k3_lc_b1_adversarial_test.go asserts the refusal.
-- Invariants: contracts/live-console-v1.md §9 "archive ... read-only afterwards". DELETE is deliberately not guarded (session purge cascades).
-- Status: DESIGN->MOCK (REAL_PG tests only).

-- The session row is read FOR SHARE so a concurrent archive (UPDATE ... NO KEY UPDATE) and an offer write serialize: the
-- write either sees 'archived' or commits before the archive takes the row. Invoker rights: commerce_runtime holds
-- SELECT + UPDATE(lifecycle) on live.sessions under the same tenant/store RLS scope as the offer row being written.
CREATE FUNCTION live.guard_archived_offer_write() RETURNS trigger
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE lc text;
BEGIN
 SELECT s.lifecycle INTO lc FROM live.sessions s
  WHERE s.tenant_id=NEW.tenant_id AND s.store_id=NEW.store_id AND s.id=NEW.session_id FOR SHARE;
 IF lc = 'archived' THEN
  RAISE EXCEPTION 'session archived' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION live.guard_archived_offer_write() FROM PUBLIC;
CREATE TRIGGER live_offers_guard_archived BEFORE INSERT OR UPDATE ON live.offers
 FOR EACH ROW EXECUTE FUNCTION live.guard_archived_offer_write();
CREATE TRIGGER live_offer_timeline_guard_archived BEFORE INSERT ON live.offer_timeline
 FOR EACH ROW EXECUTE FUNCTION live.guard_archived_offer_write();
COMMENT ON FUNCTION live.guard_archived_offer_write() IS
 'internal/live + internal/claims trigger function (live.offers INSERT/UPDATE, live.offer_timeline INSERT). Rejects writes whose session is archived with 23514. Invoker rights; reads live.sessions FOR SHARE. No caller EXECUTE.';
