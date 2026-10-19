-- Purpose: make live.read_media_recovery_episode answer from one snapshot.
-- Depends on: 0041 live.read_media_recovery_episode (body, owner, ACL unchanged here).
-- Used by: the media recovery supervisor readback (internal/live/media_recovery.go) and MRR foundation gates.
-- Invariants: scope_status and every per-row disposition come from the same snapshot, so a Witness or
-- Timeout committing mid-call is either fully visible or not at all. As VOLATILE, each of the function's two
-- statements took its own READ COMMITTED snapshot and could return scope=pending beside a witnessed row
-- (PR #75 CI run 38799313810, MRR02 post90). The function only reads; STABLE uses the calling query's snapshot.

ALTER FUNCTION live.read_media_recovery_episode(uuid) STABLE;
