package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BRW07 receipts are append-only, including against the fixture owner. Keep
// receipt writes in one READ COMMITTED transaction and roll them back before
// bicSetup's committed-fixture teardown runs. The lock-race test commits only
// a known-empty scope and removes that exact row afterward.
func brw07RecoveryTx(t *testing.T, h *brwHarness) (*pgxpool.Pool, pgx.Tx) {
	t.Helper()
	login, pool := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+name)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+name+" WITH INHERIT TRUE, SET FALSE")
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("BRW07 rollback before fixture teardown: %v", err)
		}
	})
	return pool, tx
}

type brw07Member struct {
	disposition, profile string
	operation            *string
	job, baseline        *int64
	candidates           int
	known, input, egress bool
}

func brw07Begin(t *testing.T, tx pgx.Tx, episode string, elapsed int64, known bool) []brw07Member {
	t.Helper()
	rows, err := tx.Query(context.Background(), `SELECT disposition,operation_id::text,job_id,
	 baseline_generation,candidate_count,coverage_known,coalesce(execution_profile,''),input_required,
	 egress_required FROM live.begin_media_recovery_episode_with_input($1::uuid,$2::bigint,32,$3::boolean)`,
		episode, elapsed, known)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var members []brw07Member
	for rows.Next() {
		var m brw07Member
		if err := rows.Scan(&m.disposition, &m.operation, &m.job, &m.baseline,
			&m.candidates, &m.known, &m.profile, &m.input, &m.egress); err != nil {
			t.Fatal(err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(members) == 0 {
		t.Fatal("mixed Begin returned no scope/member row")
	}
	return members
}

type brw07Claim struct {
	disposition, profile                       string
	generation                                 *int64
	project, endpoint, room, publisher, egress *string
	credential                                 *int64
	input, output                              bool
	inputID, egressID                          *string
}

func brw07ClaimMember(t *testing.T, tx pgx.Tx, h *brwHarness, episode string, token []byte) brw07Claim {
	t.Helper()
	var c brw07Claim
	err := tx.QueryRow(context.Background(), `SELECT disposition,generation,project_id,
	 credential_version,endpoint_identity,room_name,publisher_identity,egress_id,
	 execution_profile,input_required,egress_required,input_observation_id::text,
	 egress_observation_id::text FROM live.claim_media_recovery_observation_with_input(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea)`, episode, h.plan.OperationID, h.plan.JobID, token).
		Scan(&c.disposition, &c.generation, &c.project, &c.credential, &c.endpoint,
			&c.room, &c.publisher, &c.egress, &c.profile, &c.input, &c.output,
			&c.inputID, &c.egressID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func brw07SQLState(t *testing.T, tx pgx.Tx, code, query string, args ...any) {
	t.Helper()
	sp, err := tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = sp.Exec(context.Background(), query, args...)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != code {
		t.Errorf("wanted SQLSTATE %s, got %v", code, err)
	}
	if err := sp.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func brw07Witness(t *testing.T, tx pgx.Tx, episode, operation string, input, egress any, elapsed int64) string {
	t.Helper()
	var result string
	if err := tx.QueryRow(context.Background(), `SELECT live.witness_media_recovery_episode_with_input(
	 $1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::bigint)`, episode, operation, input, egress, elapsed).
		Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestLiveBrowserInputBRW07ReadyACLAndIssuedAdmission(t *testing.T) {
	h := brwStart(t)
	pool, tx := brw07RecoveryTx(t, h)
	var ready bool
	if err := pool.QueryRow(context.Background(), `SELECT live.media_browser_input_recovery_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("actual recovery-role BRW07 readiness: %t %v", ready, err)
	}
	for _, signature := range []string{
		"live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean)",
		"live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)",
		"live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)",
		"live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)",
		"live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)",
		"live.read_media_recovery_episode_with_input(uuid)",
		"live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)",
	} {
		var owner, result string
		var definer, fixedPath, recoveryAllowed, runtimeAllowed, publicAllowed bool
		err := h.lp.f.owner.QueryRow(context.Background(), `SELECT p.proowner::regrole::text,
		 pg_get_function_result(p.oid),p.prosecdef,p.proconfig @> ARRAY['search_path=pg_catalog']::text[],
		 has_function_privilege('commerce_media_recovery',p.oid,'EXECUTE'),
		 has_function_privilege('commerce_runtime',p.oid,'EXECUTE'),
		 EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) acl
		  WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE') FROM pg_proc p
		 WHERE p.oid=to_regprocedure($1)`, signature).
			Scan(&owner, &result, &definer, &fixedPath, &recoveryAllowed, &runtimeAllowed, &publicAllowed)
		if err != nil || owner != "commerce_media_writer" || result == "" || !definer || !fixedPath ||
			!recoveryAllowed || runtimeAllowed || publicAllowed {
			t.Fatalf("frozen function authority %s: owner=%q result=%q flags=%t/%t/%t/%t/%t err=%v",
				signature, owner, result, definer, fixedPath, recoveryAllowed, runtimeAllowed, publicAllowed, err)
		}
	}
	var rawAllowed, forceRLS bool
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT
	 has_table_privilege('commerce_media_recovery','live.media_input_recovery_observations','SELECT'),
	 relforcerowsecurity FROM pg_class WHERE oid='live.media_input_recovery_observations'::regclass`).
		Scan(&rawAllowed, &forceRLS); err != nil || rawAllowed || !forceRLS {
		t.Fatalf("receipt raw privilege/RLS: select=%t force=%t err=%v", rawAllowed, forceRLS, err)
	}
	if _, err := h.executor.Exec(context.Background(), `SELECT * FROM live.begin_media_recovery_episode_with_input(
	 $1::uuid,0,32,true)`, randomUUID()); err == nil {
		t.Fatal("executor role called recovery-only Begin")
	} else {
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "42501" {
			t.Fatalf("wrong-role rejection was not ACL denial: %v", err)
		}
	}
	var generation int64
	var projectionCount int
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.generation,
	 (SELECT count(*) FROM live.media_execution_state x WHERE x.operation_id=o.id)
	 FROM integration.operations o WHERE o.id=$1::uuid`, h.plan.OperationID).
		Scan(&generation, &projectionCount); err != nil || generation != 0 || projectionCount != 0 {
		t.Fatalf("pre-grant original is not READY/gen0 without projection: %d %d %v", generation, projectionCount, err)
	}
	if rows := brw07Begin(t, tx, randomUUID(), 0, true); len(rows) != 1 ||
		rows[0].operation != nil || rows[0].candidates != 0 || rows[0].disposition != "empty" {
		t.Fatalf("UNISSUED input entered recovery: %+v", rows)
	}
	// Release the empty scope before the grant commits on the other connection.
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	brwReserveGrant(t, h)
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.generation,
	 (SELECT count(*) FROM live.media_execution_state x WHERE x.operation_id=o.id)
	 FROM integration.operations o WHERE o.id=$1::uuid`, h.plan.OperationID).
		Scan(&generation, &projectionCount); err != nil || generation != 0 || projectionCount != 0 {
		t.Fatalf("issued READY/gen0 unexpectedly claimed/executed: %d %d %v", generation, projectionCount, err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	episode := randomUUID()
	rows := brw07Begin(t, tx, episode, 0, true)
	if len(rows) != 1 || rows[0].operation == nil || *rows[0].operation != h.plan.OperationID ||
		rows[0].job == nil || *rows[0].job != h.plan.JobID || rows[0].baseline == nil ||
		*rows[0].baseline != 0 || rows[0].profile != "LOCAL_SFU_MOCK_EGRESS" ||
		!rows[0].input || rows[0].egress || rows[0].candidates != 1 || !rows[0].known {
		t.Fatalf("issued READY/gen0 omitted or wrong mask: %+v", rows)
	}
	claim := brw07ClaimMember(t, tx, h, episode, bytes.Repeat([]byte{0x70}, 32))
	if claim.disposition != "claimed" || claim.generation == nil || *claim.generation != 1 ||
		!claim.input || claim.output {
		t.Fatalf("issued gen0 input projection not recoverable: %+v", claim)
	}
}

func TestLiveBrowserInputBRW07InputProofAndLegacyFence(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	_, tx := brw07RecoveryTx(t, h)
	episode := randomUUID()
	rows := brw07Begin(t, tx, episode, 0, true)
	if len(rows) != 1 || rows[0].operation == nil || *rows[0].operation != h.plan.OperationID ||
		!rows[0].input || rows[0].egress {
		t.Fatalf("input-only admission: %+v", rows)
	}
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.begin_media_recovery_episode($1::uuid,0,32,true)`, episode)
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.read_media_recovery_episode($1::uuid)`, episode)
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.claim_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea)`, episode, h.plan.OperationID, h.plan.JobID, randomBytes(32))
	brw07SQLState(t, tx, "ME409", `SELECT live.witness_media_recovery_episode($1::uuid,$2::uuid,$3::uuid,0)`, episode, h.plan.OperationID, randomUUID())
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.claim_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea)`, episode, h.plan.OperationID, h.plan.JobID+1, randomBytes(32))
	token := bytes.Repeat([]byte{0x71}, 32)
	claim := brw07ClaimMember(t, tx, h, episode, token)
	if claim.disposition != "claimed" || claim.generation == nil || *claim.generation != 1 ||
		claim.project == nil || *claim.project != "project_lma" || claim.credential == nil || *claim.credential != 1 ||
		claim.endpoint == nil || *claim.endpoint != "https://unit.livekit.cloud" ||
		claim.room == nil || *claim.room != h.plan.RoomName ||
		claim.publisher == nil || *claim.publisher != h.grant.PublisherIdentity ||
		claim.profile != "LOCAL_SFU_MOCK_EGRESS" || !claim.input || claim.output ||
		claim.inputID != nil || claim.egressID != nil {
		t.Fatalf("input claim target/proof leak: %+v", claim)
	}
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.record_browser_input_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`,
		episode, h.plan.OperationID, *claim.generation+1, token)
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.record_browser_input_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`,
		episode, h.plan.OperationID, *claim.generation, bytes.Repeat([]byte{0x72}, 32))
	brw07SQLState(t, tx, "ME400", `SELECT * FROM live.record_browser_input_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'PARTICIPANT','PRESENT','bad','ACTIVE')`,
		episode, h.plan.OperationID, *claim.generation, token)
	var disposition, inputID string
	if err := tx.QueryRow(context.Background(), `SELECT disposition,input_observation_id::text
	 FROM live.record_browser_input_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`,
		episode, h.plan.OperationID, *claim.generation, token).Scan(&disposition, &inputID); err != nil ||
		disposition != "checked" || inputID == "" {
		t.Fatalf("input proof not persisted in transaction: %q %q %v", disposition, inputID, err)
	}
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.record_browser_input_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`,
		episode, h.plan.OperationID, *claim.generation, token)
	var readInput, readSource, readResult, readDisposition string
	var readGeneration int64
	var readJob int64
	if err := tx.QueryRow(context.Background(), `SELECT input_observation_id::text,
	 input_observation_source,input_observation_result,input_observation_generation,
	 disposition,job_id FROM live.read_media_recovery_episode_with_input($1::uuid)`, episode).
		Scan(&readInput, &readSource, &readResult, &readGeneration, &readDisposition, &readJob); err != nil ||
		readInput != inputID || readSource != "ROOM" || readResult != "ABSENT" ||
		readGeneration != *claim.generation || readDisposition != "checked" || readJob != h.plan.JobID {
		t.Fatalf("input proof readback mismatch: %q %q %q %d %q %d %v",
			readInput, readSource, readResult, readGeneration, readDisposition, readJob, err)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, randomUUID(), nil, 100); got != "unqualified" {
		t.Fatalf("foreign input receipt witnessed: %q", got)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, randomUUID(), 100); got != "unqualified" {
		t.Fatalf("input-only proof accepted extra Egress receipt: %q", got)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, nil, 100); got != "witnessed" {
		t.Fatalf("exact input-only witness: %q", got)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, nil, 100); got != "already_witnessed" {
		t.Fatalf("exact witness replay: %q", got)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, nil, 101); got != "unqualified" {
		t.Fatalf("different elapsed replay accepted: %q", got)
	}
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.read_media_recovery_episode($1::uuid)`, episode)
}

func TestLiveBrowserInputBRW07CompositeNeedsBothAndStickyTimeout(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	original, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x73}, 32))
	if err != nil || original.disposition != "claimed" || original.mode != "reconcile" {
		t.Fatalf("original input claim: %+v %v", original, err)
	}
	brwSelectObserve(t, h, original)
	if _, err := brwReserveStart(context.Background(), h, original, h.plan.RoomName,
		h.grant.PublisherIdentity, "PA_brw07", "ACTIVE", true, false, true, false); err != nil {
		t.Fatalf("original exact Egress reservation: %v", err)
	}
	var released string
	if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_input_turn(
	 $1::uuid,$2::bigint,$3::bytea,'input_unknown')`, h.plan.OperationID,
		original.generation, original.key).Scan(&released); err != nil || released != "observe" {
		t.Fatalf("normal original-lease release: %q %v", released, err)
	}
	_, tx := brw07RecoveryTx(t, h)
	episode := randomUUID()
	rows := brw07Begin(t, tx, episode, 0, true)
	if len(rows) != 1 || rows[0].operation == nil || *rows[0].operation != h.plan.OperationID ||
		!rows[0].input || !rows[0].egress || rows[0].baseline == nil ||
		*rows[0].baseline != original.generation {
		t.Fatalf("composite immutable admission: %+v", rows)
	}
	token := bytes.Repeat([]byte{0x74}, 32)
	claim := brw07ClaimMember(t, tx, h, episode, token)
	if claim.disposition != "claimed" || claim.generation == nil || *claim.generation <= original.generation ||
		!claim.input || !claim.output || claim.inputID != nil || claim.egressID != nil {
		t.Fatalf("composite claim: %+v", claim)
	}
	var disposition, inputID string
	if err := tx.QueryRow(context.Background(), `SELECT disposition,input_observation_id::text
	 FROM live.record_browser_input_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'PARTICIPANT','PRESENT','PA_brw07','ACTIVE')`,
		episode, h.plan.OperationID, *claim.generation, token).Scan(&disposition, &inputID); err != nil || disposition != "checked" {
		t.Fatalf("first composite side: %q %q %v", disposition, inputID, err)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, nil, 100); got != "unqualified" {
		t.Fatalf("composite missing Egress witnessed: %q", got)
	}
	var releasedRecovery string
	if err := tx.QueryRow(context.Background(), `SELECT live.finish_media_recovery_observation_with_input(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'remote_unknown')`, episode,
		h.plan.OperationID, *claim.generation, token).Scan(&releasedRecovery); err != nil || releasedRecovery != "released" {
		t.Fatalf("partial proof did not release its lease: %q %v", releasedRecovery, err)
	}
	nextToken := bytes.Repeat([]byte{0x76}, 32)
	next := brw07ClaimMember(t, tx, h, episode, nextToken)
	if next.disposition != "claimed" || next.generation == nil || *next.generation != *claim.generation+1 ||
		next.inputID == nil || *next.inputID != inputID || next.egressID != nil || !next.input || !next.output {
		t.Fatalf("fresh generation lost immutable first proof: %+v", next)
	}
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.record_media_recovery_observation_with_input(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','EG_brw07',$5::text,'EGRESS_ACTIVE',100,120,0)`,
		episode, h.plan.OperationID, *claim.generation, token, h.plan.RoomName)
	brw07SQLState(t, tx, "ME409", `SELECT * FROM live.record_media_recovery_observation_with_input(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','EG_brw07',$5::text,'EGRESS_ACTIVE',100,120,0)`,
		episode, h.plan.OperationID, *next.generation, bytes.Repeat([]byte{0x75}, 32), h.plan.RoomName)
	var egressID string
	if err := tx.QueryRow(context.Background(), `SELECT disposition,observation_id::text
	 FROM live.record_media_recovery_observation_with_input(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','EG_brw07',$5::text,'EGRESS_ACTIVE',100,120,0)`,
		episode, h.plan.OperationID, *next.generation, nextToken, h.plan.RoomName).
		Scan(&disposition, &egressID); err != nil || disposition != "checked" || egressID == "" {
		t.Fatalf("second composite side: %q %q %v", disposition, egressID, err)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, randomUUID(), 100); got != "unqualified" {
		t.Fatalf("foreign Egress receipt witnessed: %q", got)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, inputID, egressID, 100); got != "witnessed" {
		t.Fatalf("exact composite witness: %q", got)
	}
	var affected int
	var timeoutDisposition string
	if err := tx.QueryRow(context.Background(), `SELECT disposition,affected_count
	 FROM live.timeout_media_recovery_episode($1::uuid,90000)`, episode).
		Scan(&timeoutDisposition, &affected); err != nil || timeoutDisposition != "already_finished" || affected != 0 {
		t.Fatalf("witness-first timeout changed result: %q %d %v", timeoutDisposition, affected, err)
	}
}

func TestLiveBrowserInputBRW07TimeoutFirstIsSticky(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	_, tx := brw07RecoveryTx(t, h)
	episode := randomUUID()
	rows := brw07Begin(t, tx, episode, 0, true)
	if len(rows) != 1 || rows[0].operation == nil || !rows[0].input {
		t.Fatalf("timeout candidate missing: %+v", rows)
	}
	var disposition string
	var affected int
	if err := tx.QueryRow(context.Background(), `SELECT disposition,affected_count
	 FROM live.timeout_media_recovery_episode($1::uuid,90000)`, episode).
		Scan(&disposition, &affected); err != nil || disposition != "timed_out" || affected != 1 {
		t.Fatalf("SQL timeout did not persist: %q %d %v", disposition, affected, err)
	}
	if got := brw07Witness(t, tx, episode, h.plan.OperationID, randomUUID(), nil, 100); got != "timeout_wins" {
		t.Fatalf("timeout-first witness escaped: %q", got)
	}
	if err := tx.QueryRow(context.Background(), `SELECT disposition,affected_count
	 FROM live.timeout_media_recovery_episode($1::uuid,90000)`, episode).
		Scan(&disposition, &affected); err != nil || disposition != "already_timed_out" || affected != 0 {
		t.Fatalf("timeout replay not sticky: %q %d %v", disposition, affected, err)
	}
	var status string
	if err := tx.QueryRow(context.Background(), `SELECT scope_status
	 FROM live.read_media_recovery_episode_with_input($1::uuid)`, episode).Scan(&status); err != nil || status != "overdue" {
		t.Fatalf("sticky timeout missing from readback: %q %v", status, err)
	}
}

func TestLiveBrowserInputBRW07LegacyReadWaitsForMixedAdmission(t *testing.T) {
	h := brwStart(t) // UNISSUED: the mixed Begin commits only an empty scope.
	pool, tx := brw07RecoveryTx(t, h)
	episode := randomUUID()
	if rows := brw07Begin(t, tx, episode, 0, true); len(rows) != 1 ||
		rows[0].disposition != "empty" || rows[0].operation != nil {
		t.Fatalf("unissued mixed scope is not empty: %+v", rows)
	}
	var holderPID int
	if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var waiterPID int
	if err := conn.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&waiterPID); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	waiterDone := false
	defer func() {
		_ = tx.Rollback(context.Background()) // release the episode lock before draining waiter
		cancel()
		if !waiterDone {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("legacy read waiter did not stop after rollback/cancel")
			}
		}
	}()
	go func() {
		_, err := conn.Exec(waitCtx, `SELECT * FROM live.read_media_recovery_episode($1::uuid)`, episode)
		done <- err
	}()
	// Observe the actual advisory wait in pg_stat_activity. No timer-based
	// assumption about when the second connection started is sufficient here.
	lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID, true)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Only this generated, known-empty scope was committed. Remove that exact
	// row before the shared foundation fixture teardown; no receipt exists.
	t.Cleanup(func() {
		ctx := context.Background()
		tag, err := h.lp.f.owner.Exec(ctx, `DELETE FROM live.media_recovery_episode_scope s
		 WHERE s.episode_id=$1::uuid AND s.include_browser_input AND s.candidate_count=0
		 AND NOT EXISTS (SELECT 1 FROM integration.operation_events e WHERE e.episode_id=s.episode_id)`, episode)
		if err != nil || tag.RowsAffected() != 1 {
			t.Errorf("remove exact empty BRW07 scope: rows=%d err=%v", tag.RowsAffected(), err)
		}
	})
	select {
	case err := <-done:
		waiterDone = true
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "ME409" {
			t.Fatalf("legacy read after mixed Begin commit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("legacy read did not finish after mixed Begin committed")
	}
}
