package foundation_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// MRR tests use the existing real-PG18 media fixture and its original River
// job. Elapsed arguments below exercise the SQL boundary; only the process
// test exercises the parent's real monotonic 90-second clock.
func mrrRecoveryPool(t *testing.T, h *lmeHarness) (*pgxpool.Pool, string) {
	t.Helper()
	login, pool := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+name)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+name+" WITH INHERIT TRUE, SET FALSE")
	return pool, login
}

type mrrMember struct {
	disposition string
	episode     string
	operation   *string
	job         *int64
	baseline    *int64
	candidates  int
	known       bool
	blocked     *string
}

func mrrBegin(t *testing.T, pool *pgxpool.Pool, episode string, elapsed int64, capacity int, known bool) []mrrMember {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT disposition,episode_id::text,operation_id::text,job_id,
	 baseline_generation,candidate_count,coverage_known,blocked_by_episode_id::text
	 FROM live.begin_media_recovery_episode($1::uuid,$2::bigint,$3::integer,$4::boolean)`, episode, elapsed, capacity, known)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []mrrMember
	for rows.Next() {
		var row mrrMember
		if err := rows.Scan(&row.disposition, &row.episode, &row.operation, &row.job,
			&row.baseline, &row.candidates, &row.known, &row.blocked); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("begin returned no scope/member row")
	}
	return out
}

type mrrReadback struct {
	scope, disposition string
	known              bool
	candidates         int
	operation, obs     *string
	source             *string
	generation         *int64
	witnessElapsed     *int64
	timeoutAt          *time.Time
	cleanup            *bool
}

func mrrRead(t *testing.T, pool *pgxpool.Pool, episode string) []mrrReadback {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT scope_status,coverage_known,candidate_count,operation_id::text,
	 disposition,observation_id::text,observation_source,observation_generation,witness_elapsed_ms,timeout_at,cleanup_required
	 FROM live.read_media_recovery_episode($1::uuid)`, episode)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []mrrReadback
	for rows.Next() {
		var row mrrReadback
		if err := rows.Scan(&row.scope, &row.known, &row.candidates, &row.operation,
			&row.disposition, &row.obs, &row.source, &row.generation, &row.witnessElapsed,
			&row.timeoutAt, &row.cleanup); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("committed recovery readback returned no row")
	}
	return out
}

func mrrReserveUnansweredStart(t *testing.T, h *lmeHarness) lmeLease {
	t.Helper()
	lease := h.claim(t, 30)
	if lease.disposition != "claimed" || lease.mode != "dispatch" {
		t.Fatalf("original dispatch claim: %+v", lease)
	}
	if err := h.reserve(t, lease); err != nil {
		t.Fatal(err)
	}
	if !h.facts(t).reserved {
		t.Fatal("original Start wire reservation did not commit")
	}
	return lease
}

func mrrClaim(t *testing.T, pool *pgxpool.Pool, episode string, h *lmeHarness, token []byte) (string, int64, *string) {
	t.Helper()
	var disposition string
	var generation *int64
	var project, endpoint, room, egress *string
	var credential *int64
	err := pool.QueryRow(context.Background(), `SELECT disposition,generation,project_id,credential_version,endpoint_identity,room_name,egress_id
	 FROM live.claim_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea)`,
		episode, h.plan.OperationID, h.plan.JobID, token).Scan(&disposition, &generation, &project, &credential, &endpoint, &room, &egress)
	if err != nil {
		t.Fatal(err)
	}
	if disposition == "claimed" {
		if generation == nil || project == nil || *project != "project_lma" || credential == nil || *credential != 1 ||
			endpoint == nil || *endpoint != "https://unit.livekit.cloud" || room == nil || *room != h.plan.RoomName {
			t.Fatalf("wrong frozen target or generation: %s %v %v %v %v %v", disposition, generation, project, credential, endpoint, room)
		}
		return disposition, *generation, egress
	}
	if generation != nil || project != nil || credential != nil || endpoint != nil || room != nil || egress != nil {
		t.Fatalf("nonclaim disclosed target: %s", disposition)
	}
	return disposition, 0, nil
}

func mrrTimeout(t *testing.T, pool *pgxpool.Pool, episode string, elapsed int64) (string, int) {
	t.Helper()
	var disposition string
	var affected int
	if err := pool.QueryRow(context.Background(), `SELECT disposition,affected_count FROM live.timeout_media_recovery_episode($1::uuid,$2::bigint)`,
		episode, elapsed).Scan(&disposition, &affected); err != nil {
		t.Fatal(err)
	}
	return disposition, affected
}

func TestLiveMediaRecoveryMRR02ScopeClockAndWitness(t *testing.T) {
	h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
	recovery, _ := mrrRecoveryPool(t, h)
	knownEmpty := randomUUID()
	empty := mrrBegin(t, recovery, knownEmpty, 0, 1, true)
	if len(empty) != 1 || empty[0].disposition != "empty" || empty[0].candidates != 0 || !empty[0].known {
		t.Fatalf("known empty must be NO_WORK: %+v", empty)
	}
	if disposition, count := mrrTimeout(t, recovery, knownEmpty, 90000); disposition != "already_finished" || count != 0 {
		t.Fatalf("known empty emitted alarm: %s %d", disposition, count)
	}
	unknownEmpty := randomUUID()
	if rows := mrrBegin(t, recovery, unknownEmpty, 91000, 1, false); len(rows) != 1 || rows[0].known {
		t.Fatalf("unknown coverage upgraded: %+v", rows)
	}
	if disposition, count := mrrTimeout(t, recovery, unknownEmpty, 91000); disposition != "timed_out" || count != 0 {
		t.Fatalf("unknown empty must retain scope miss: %s %d", disposition, count)
	}
	if got := mrrRead(t, recovery, unknownEmpty); got[0].timeoutAt == nil || got[0].known {
		t.Fatalf("unknown coverage miss lost: %+v", got)
	}

	mrrReserveUnansweredStart(t, h)
	episode := randomUUID()
	first := mrrBegin(t, recovery, episode, 1000, 1, true)
	if len(first) != 1 || first[0].disposition != "pending" || first[0].operation == nil || *first[0].operation != h.plan.OperationID ||
		first[0].job == nil || *first[0].job != h.plan.JobID || first[0].baseline == nil {
		t.Fatalf("original unresolved member not captured: %+v", first)
	}
	if replay := mrrBegin(t, recovery, episode, 80000, 1, false); len(replay) != 1 || !replay[0].known || replay[0].candidates != first[0].candidates {
		t.Fatalf("same-episode replay changed immutable coverage: %+v", replay)
	}
	if disposition, _, _ := mrrClaim(t, recovery, episode, h, randomBytes(32)); disposition != "busy" {
		t.Fatalf("active old lease stolen: %s", disposition)
	}
	// This SQL gate changes only the business lease, never River attempted_at,
	// native job state or the parent wall clock.
	mustExec(t, h.lp.f.owner, `UPDATE integration.operations SET lease_until=NULL WHERE id=$1`, h.plan.OperationID)
	token := randomBytes(32)
	if disposition, generation, egress := mrrClaim(t, recovery, episode, h, token); disposition != "claimed" || generation <= *first[0].baseline || egress != nil {
		t.Fatalf("ROOM recovery claim: %s gen=%d egress=%v", disposition, generation, egress)
	} else {
		var state string
		var obs string
		err := recovery.QueryRow(context.Background(), `SELECT disposition,observation_id::text FROM live.record_recovery_observation(
		 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM',$5::text,$6::text,'EGRESS_ACTIVE',100,120,0)`,
			episode, h.plan.OperationID, generation, token, "EG_mrr_room", h.plan.RoomName).Scan(&state, &obs)
		if err != nil || state != "checked" || obs == "" {
			t.Fatalf("ROOM record: %s %s %v", state, obs, err)
		}
		read := mrrRead(t, recovery, episode)
		if len(read) != 1 || read[0].obs == nil || *read[0].obs != obs || read[0].source == nil || *read[0].source != "ROOM" ||
			read[0].generation == nil || *read[0].generation != generation {
			t.Fatalf("committed correlation missing: %+v", read)
		}
		var witnessed string
		if err := recovery.QueryRow(context.Background(), `SELECT live.witness_media_recovery_episode($1::uuid,$2::uuid,$3::uuid,89999)`,
			episode, h.plan.OperationID, obs).Scan(&witnessed); err != nil || witnessed != "witnessed" {
			t.Fatalf("delayed witness for timely readback: %s %v", witnessed, err)
		}
		if disposition, count := mrrTimeout(t, recovery, episode, 90000); disposition != "already_finished" || count != 0 {
			t.Fatalf("witnessed member timed out: %s %d", disposition, count)
		}
	}
	if h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
		t.Fatal("SQL-only observer made provider call")
	}
}

func TestLiveMediaRecoveryMRR03NegativeAuthorityAndFences(t *testing.T) {
	h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
	recovery, login := mrrRecoveryPool(t, h)
	ctx := context.Background()
	const beginSig = "live.begin_media_recovery_episode(uuid,bigint,integer,boolean)"
	for _, old := range map[string]*pgxpool.Pool{"worker": h.worker, "executor": h.executor, "runtime": h.lp.f.runtime, "registrar": h.registrar} {
		var allowed bool
		if err := old.QueryRow(ctx, `SELECT has_function_privilege(current_user,$1,'EXECUTE')`, beginSig).Scan(&allowed); err != nil || allowed {
			t.Fatalf("old role reaches observer begin: %v %v", allowed, err)
		}
	}
	var ready bool
	if err := recovery.QueryRow(ctx, `SELECT live.media_recovery_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("recovery readiness: %v %v", ready, err)
	}
	for _, table := range []string{"live.media_execution_state", "live.media_observations", "integration.operations", "river_media.river_job"} {
		var count int
		if err := recovery.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err == nil {
			t.Fatalf("recovery role directly read %s", table)
		}
	}
	var allowed bool
	if err := recovery.QueryRow(ctx, `SELECT has_function_privilege(current_user,'live.claim_media_operation(uuid,bigint,integer,bytea)','EXECUTE')`).Scan(&allowed); err != nil || allowed {
		t.Fatalf("observer reaches native executor: %v %v", allowed, err)
	}
	if err := platform.ValidateMediaRecoveryPool(ctx, recovery); err != nil {
		t.Fatalf("clean observer pool: %v", err)
	}
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_executor TO "+name+" WITH INHERIT TRUE, SET FALSE")
	if err := platform.ValidateMediaRecoveryPool(ctx, recovery); err == nil {
		t.Fatal("mixed observer/executor role admitted")
	}
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_executor FROM "+name)
	if err := platform.ValidateMediaRecoveryPool(ctx, recovery); err != nil {
		t.Fatalf("clean observer role not restored: %v", err)
	}
	mrrReserveUnansweredStart(t, h)
	episode := randomUUID()
	if got := mrrBegin(t, recovery, episode, 0, 1, true); len(got) != 1 || got[0].operation == nil {
		t.Fatalf("missing member: %+v", got)
	}
	mustExec(t, h.lp.f.owner, `UPDATE integration.operations SET lease_until=NULL WHERE id=$1`, h.plan.OperationID)
	token := randomBytes(32)
	disposition, generation, _ := mrrClaim(t, recovery, episode, h, token)
	if disposition != "claimed" {
		t.Fatalf("fresh claim: %s", disposition)
	}
	for _, bad := range []struct {
		name  string
		gen   int64
		token []byte
		room  string
	}{
		{"old generation", generation - 1, token, h.plan.RoomName},
		{"wrong token", generation, randomBytes(32), h.plan.RoomName},
		{"wrong target", generation, token, "wrong_room"},
	} {
		var result, id string
		err := recovery.QueryRow(ctx, `SELECT disposition,observation_id::text FROM live.record_recovery_observation(
		 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','EG_bad',$5::text,'EGRESS_ACTIVE',100,120,0)`,
			episode, h.plan.OperationID, bad.gen, bad.token, bad.room).Scan(&result, &id)
		if err == nil {
			t.Fatalf("%s recorded observer result %s/%s", bad.name, result, id)
		}
	}
	if f := h.facts(t); f.observations != 0 || f.cleanup || h.stops.Load() != 0 {
		t.Fatalf("negative fences changed custody: %+v", f)
	}
	if _, err := migrations.Apply(ctx, h.lp.f.owner); err != nil {
		t.Fatalf("idempotent additive migration: %v", err)
	}
	if err := platform.ValidateMediaRecoveryPool(ctx, recovery); err != nil {
		t.Fatalf("migration drifted readiness: %v", err)
	}
	if h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
		t.Fatal("negative SQL gate made provider call")
	}
	if !strings.HasPrefix(h.plan.RoomName, "lc_") {
		t.Fatal("fixture target not frozen room")
	}
}
