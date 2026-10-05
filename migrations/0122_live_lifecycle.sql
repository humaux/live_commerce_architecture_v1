-- Purpose: LC-B1 (W2-01B, contracts/live-console-v1.md §7.3/§8/§9): the session lifecycle column on live.sessions
--   (draft -> live -> ended -> archived, its own CAS counter), the removal of the one-OPEN-window-per-store index in favour
--   of the application-enforced cap of 5 (OPEN-15), the archived-session read-only guard, and the append-only
--   live.offer_timeline ("offer featured" events used by per-offer results).
-- Depends on: live.sessions / live.offers / live.claim_windows (0033/0060, RLS policies session_*, offer_*, window_*),
--   identity.memberships (principal FK), the commerce_runtime role. No SECURITY DEFINER function is added.
-- Used by: internal/live/lifecycle.go (A7 Lifecycle, RecordOfferFeatured), internal/claims/merchant.go (window cap),
--   internal/httpapi/live_lifecycle.go (POST /live-sessions/{id}/lifecycle).
-- Invariants: lifecycle is separate from the media programme state (arch §9.1) and from live.sessions.version, because
--   0035's frozen_media_session trigger forbids version/updated_at changes once a media attempt exists; a lifecycle
--   transition must still work for a session with attempts, so it uses lifecycle_version/lifecycle_at only.
-- Non-goals: no recommend endpoint (A6, LC-B4/B7), no console read model, no poller gating (LC-B2 reads lifecycle).

-- ---------------------------------------------------------------------------------------
-- §9 lifecycle columns. Existing rows are drafts (lifecycle 'draft'); a session that already has an OPEN window is
-- promoted to 'live' so the new state never contradicts the window.
-- ---------------------------------------------------------------------------------------
ALTER TABLE live.sessions
  ADD COLUMN lifecycle text NOT NULL DEFAULT 'draft' CHECK (lifecycle IN ('draft','live','ended','archived')),
  ADD COLUMN lifecycle_version bigint NOT NULL DEFAULT 1 CHECK (lifecycle_version > 0),
  ADD COLUMN lifecycle_at timestamptz;
UPDATE live.sessions s SET lifecycle='live', lifecycle_at=clock_timestamp()
 WHERE EXISTS (SELECT 1 FROM live.claim_windows w WHERE w.tenant_id=s.tenant_id AND w.store_id=s.store_id
                AND w.session_id=s.id AND w.state='OPEN');
GRANT UPDATE (lifecycle, lifecycle_version, lifecycle_at) ON live.sessions TO commerce_runtime;
COMMENT ON COLUMN live.sessions.lifecycle IS
 'internal/live (Lifecycle, A7): draft|live|ended|archived. Separate from the media programme state. Written only by live.Lifecycle under live:manage with lifecycle_version CAS; archived is read-only (guard_archived_session).';
COMMENT ON COLUMN live.sessions.lifecycle_version IS
 'internal/live: CAS counter of A7 (expected_version). Independent of live.sessions.version so a transition works after a media attempt froze version (0035).';
COMMENT ON COLUMN live.sessions.lifecycle_at IS 'internal/live: time of the last lifecycle transition; NULL while never transitioned.';

-- Archived sessions are read-only: the draft (title/schedule) can no longer be edited. lifecycle columns themselves may
-- not move off 'archived' either (no reopen after archive, §9).
CREATE FUNCTION live.guard_archived_session() RETURNS trigger
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
BEGIN
 IF OLD.lifecycle = 'archived' THEN
  RAISE EXCEPTION 'session archived' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION live.guard_archived_session() FROM PUBLIC;
CREATE TRIGGER live_guard_archived_session BEFORE UPDATE ON live.sessions
 FOR EACH ROW EXECUTE FUNCTION live.guard_archived_session();
COMMENT ON FUNCTION live.guard_archived_session() IS
 'internal/live trigger function (live.sessions only); invoker rights, no table reads. Rejects every UPDATE of an archived session with 23514 (-> command.ErrInvalid, the same class as the frozen media draft). No caller EXECUTE.';

-- ---------------------------------------------------------------------------------------
-- §8 several OPEN windows per store. The cap (5, OPEN-15) is enforced in claims.SetWindow/ApplyWindow under a
-- store-level advisory lock taken before the count; the unique index is dropped so more than one window can be OPEN.
-- ---------------------------------------------------------------------------------------
DROP INDEX live.live_claim_window_one_open;
CREATE INDEX live_claim_window_open ON live.claim_windows(tenant_id,store_id) WHERE state='OPEN';
COMMENT ON INDEX live.live_claim_window_open IS
 'internal/claims: supports the per-store OPEN-window count of the 5-window cap (contract §8.5). Not unique: replaces live_claim_window_one_open (0060).';

-- ---------------------------------------------------------------------------------------
-- §7.3 live.offer_timeline: append-only (no UPDATE/DELETE grant). Rows disappear only with their offer (cascade) at
-- session purge. kind is a closed set; 'featured' is the only one LC-B1 writes.
-- ---------------------------------------------------------------------------------------
CREATE TABLE live.offer_timeline (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
 session_id uuid NOT NULL, offer_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('featured')),
 at timestamptz NOT NULL DEFAULT clock_timestamp(),
 principal_id uuid NOT NULL,
 PRIMARY KEY (tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,session_id,offer_id) REFERENCES live.offers(tenant_id,store_id,session_id,id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id));
CREATE INDEX live_offer_timeline_offer ON live.offer_timeline(tenant_id,store_id,session_id,offer_id,at);
ALTER TABLE live.offer_timeline ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.offer_timeline FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.offer_timeline FROM PUBLIC;
CREATE POLICY offer_timeline_read ON live.offer_timeline FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY offer_timeline_insert ON live.offer_timeline FOR INSERT TO commerce_runtime WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
GRANT SELECT, INSERT ON live.offer_timeline TO commerce_runtime;
COMMENT ON TABLE live.offer_timeline IS
 'internal/live (RecordOfferFeatured). Append-only per-offer timeline events of a live session (kind featured), used by per-offer results; stores no buyer data. Roles: commerce_runtime SELECT/INSERT under FORCE RLS, no UPDATE/DELETE. Non-goals: no comment text, no recommend operation state.';
