-- Purpose: make the 0043 Studio media/input readers answer from one snapshot.
-- Depends on: 0043 live.read_studio_media, live.read_studio_input, live.read_studio_input_prepared
-- (bodies, owner commerce_media_writer, ACL unchanged here); 0153 identity.resolve_access (STABLE, read-only).
-- Used by: internal/live/studio.go GetStudio/GetStudioInput/GetStudioInputPrepared and STU/studio-input gates.
-- Invariants: each DTO is derived from one snapshot, so a commit landing mid-call is either fully visible or
-- not at all. As VOLATILE, every statement took its own READ COMMITTED snapshot: operation_state could predate
-- resource_state, can_stop could pair new custody with old execution state, and a candidate could pass its
-- binding check against a different snapshot than its destinations. None of the three writes, locks or calls
-- a volatile SQL-reading function (clock_timestamp is still evaluated per call, so the wall-clock expiry check
-- is unchanged). The fresh post-read revocation/revision fence is Go's GetDraft authorize, a separate statement.
-- All three move together: a VOLATILE outer caller would re-snapshot per nested call.

ALTER FUNCTION live.read_studio_media(bytea,uuid,uuid) STABLE;
ALTER FUNCTION live.read_studio_input(bytea,uuid,uuid) STABLE;
ALTER FUNCTION live.read_studio_input_prepared(bytea,uuid,uuid) STABLE;
