package foundation_test

// MRR05 exercises the production mixed recovery SQL through the restricted
// observer role with a locally issued MOCK input and its original River job.
// Admission masks and committed proof remain bound to the original operation.
import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/live"
)

func TestLiveMediaRecoveryMRR05InputOnlyEpisode(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	login, recovery := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+name)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+name+" WITH INHERIT TRUE, SET FALSE")
	episode := randomUUID()
	mrr05Cleanup(t, h, episode)
	var generation int64
	var exists bool
	err := h.lp.f.owner.QueryRow(ctx, `SELECT o.generation, EXISTS (SELECT 1 FROM live.media_execution_state x WHERE x.operation_id=o.id) FROM integration.operations o WHERE o.id=$1::uuid`, h.plan.OperationID).Scan(&generation, &exists)
	if err != nil || generation != 0 || exists {
		t.Fatalf("issued input precondition generation=%d projection=%v err=%v", generation, exists, err)
	}
	var disp, ep, op, profile string
	var job, baseline int64
	var count int
	var known, in, out bool
	err = recovery.QueryRow(ctx, `SELECT disposition,episode_id::text,operation_id::text,job_id,baseline_generation,candidate_count,coverage_known,execution_profile,input_required,egress_required FROM live.begin_media_recovery_episode_with_input($1::uuid,0,1,true)`, episode).Scan(&disp, &ep, &op, &job, &baseline, &count, &known, &profile, &in, &out)
	if err != nil || disp != "pending" || ep != episode || op != h.plan.OperationID || job != h.plan.JobID || baseline != 0 || count != 1 || !known || profile != "LOCAL_SFU_MOCK_EGRESS" || !in || out {
		t.Fatalf("mixed admission disposition=%s episode=%s operation=%s job=%d baseline=%d count=%d known=%v profile=%s input=%v egress=%v err=%v", disp, ep, op, job, baseline, count, known, profile, in, out, err)
	}
	var witness string
	err = recovery.QueryRow(ctx, `SELECT live.witness_media_recovery_episode_with_input($1::uuid,$2::uuid,NULL,NULL,1000)`, episode, op).Scan(&witness)
	if err != nil || witness != "unqualified" {
		t.Fatalf("before proof witness=%s err=%v", witness, err)
	}
	err = recovery.QueryRow(ctx, `SELECT live.witness_media_recovery_episode($1::uuid,$2::uuid,$3::uuid,1000)`, episode, op, randomUUID()).Scan(&witness)
	if sqlState(err) != "ME409" {
		t.Fatalf("legacy witness bypass result=%s state=%s err=%v", witness, sqlState(err), err)
	}
	var legacy string
	err = recovery.QueryRow(ctx, `SELECT scope_status FROM live.read_media_recovery_episode($1::uuid)`, episode).Scan(&legacy)
	if sqlState(err) != "ME409" {
		t.Fatalf("legacy read bypass result=%s state=%s err=%v", legacy, sqlState(err), err)
	}
	token := randomBytes(32)
	err = recovery.QueryRow(ctx, `SELECT disposition,generation,execution_profile,input_required,egress_required FROM live.claim_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea)`, episode, op, job, token).Scan(&disp, &generation, &profile, &in, &out)
	if err != nil || disp != "claimed" || generation <= baseline || profile != "LOCAL_SFU_MOCK_EGRESS" || !in || out {
		t.Fatalf("claim disposition=%s generation=%d profile=%s input=%v egress=%v err=%v", disp, generation, profile, in, out, err)
	}
	var receipt string
	record := `SELECT disposition,input_observation_id::text FROM live.record_browser_input_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`
	err = recovery.QueryRow(ctx, record, episode, op, generation, randomBytes(32)).Scan(&disp, &receipt)
	if sqlState(err) != "ME409" {
		t.Fatalf("wrong token state=%s err=%v", sqlState(err), err)
	}
	err = recovery.QueryRow(ctx, record, episode, op, generation-1, token).Scan(&disp, &receipt)
	if err == nil {
		t.Fatal("old generation accepted")
	}
	err = recovery.QueryRow(ctx, record, episode, op, generation, token).Scan(&disp, &receipt)
	if err != nil || disp != "checked" || receipt == "" {
		t.Fatalf("record disposition=%s receipt=%s state=%s err=%v", disp, receipt, sqlState(err), err)
	}
	err = recovery.QueryRow(ctx, `SELECT live.finish_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea,'not_observed')`, episode, op, generation, token).Scan(&disp)
	if err != nil || disp != "released" {
		t.Fatalf("finish=%s state=%s err=%v", disp, sqlState(err), err)
	}
	var scope, readop, readprofile, readreceipt, source, result string
	var readjob, readbase, readgen int64
	var readcount int
	var ri, re bool
	var egress *string
	rows, err := recovery.Query(ctx, `SELECT scope_status,candidate_count,operation_id::text,baseline_generation,observation_id::text,execution_profile,input_required,egress_required,input_observation_id::text,input_observation_source,input_observation_result,input_observation_generation,job_id FROM live.read_media_recovery_episode_with_input($1::uuid)`, episode)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	n := 0
	for rows.Next() {
		n++
		err = rows.Scan(&scope, &readcount, &readop, &readbase, &egress, &readprofile, &ri, &re, &readreceipt, &source, &result, &readgen, &readjob)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || n != 1 || scope != "pending" || readcount != 1 || readop != op || readbase != baseline || egress != nil || readprofile != profile || !ri || re || readreceipt != receipt || source != "ROOM" || result != "ABSENT" || readgen != generation || readjob != job {
		t.Fatalf("readback rows=%d scope=%s count=%d op=%s baseline=%d egress=%v profile=%s input=%v out=%v receipt=%s source=%s result=%s generation=%d job=%d err=%v", n, scope, readcount, readop, readbase, egress, readprofile, ri, re, readreceipt, source, result, readgen, readjob, err)
	}
	err = recovery.QueryRow(ctx, `SELECT live.witness_media_recovery_episode_with_input($1::uuid,$2::uuid,$3::uuid,$4::uuid,1000)`, episode, op, receipt, randomUUID()).Scan(&witness)
	if err != nil || witness != "unqualified" {
		t.Fatalf("extraneous egress witness=%s err=%v", witness, err)
	}
	err = recovery.QueryRow(ctx, `SELECT live.witness_media_recovery_episode_with_input($1::uuid,$2::uuid,$3::uuid,NULL,1000)`, episode, op, receipt).Scan(&witness)
	if err != nil || witness != "witnessed" {
		t.Fatalf("committed witness=%s state=%s err=%v", witness, sqlState(err), err)
	}
}

// Test construction and scope isolation before any pool or provider access.
// An uninitialized pool would panic if a rejected call reached pgx.
func TestLiveMediaRecoveryMRR05GoModeIsolation(t *testing.T) {
	ctx := context.Background()
	mixed := live.NewMediaRecoveryLedgerWithBrowserInput(&pgxpool.Pool{})
	legacy := live.NewMediaRecoveryLedger(&pgxpool.Pool{})
	if _, err := mixed.Witness(ctx, "", "", "", 0); err != live.ErrMediaRecovery {
		t.Fatal(err)
	}
	if _, err := legacy.WitnessWithInput(ctx, "", "", "", "", 0); err != live.ErrMediaRecovery {
		t.Fatal(err)
	}
	if _, err := mixed.Observe(ctx, "", live.RecoveryMember{Disposition: "pending", OperationID: "operation", JobID: 1}); err != live.ErrMediaRecovery {
		t.Fatal(err)
	}
	for _, elapsed := range []int64{-1, 90001} {
		if _, err := mixed.WitnessWithInput(ctx, "", "", "", "", elapsed); err != live.ErrMediaRecovery {
			t.Fatal(err)
		}
	}
	for _, ledger := range []*live.MediaRecoveryObserver{nil, live.NewMediaRecoveryLedger(nil), live.NewMediaRecoveryLedgerWithBrowserInput(nil)} {
		if _, err := ledger.Begin(ctx, "", 0, 1, true); err != live.ErrMediaRecovery {
			t.Fatal(err)
		}
		if _, err := ledger.Read(ctx, ""); err != live.ErrMediaRecovery {
			t.Fatal(err)
		}
		if _, err := ledger.WitnessWithInput(ctx, "", "", "", "", 0); err != live.ErrMediaRecovery {
			t.Fatal(err)
		}
		if _, _, err := ledger.Timeout(ctx, "", 90000); err != live.ErrMediaRecovery {
			t.Fatal(err)
		}
	}
	_ = live.RecoveryMember{ExecutionProfile: "", InputRequired: false, EgressRequired: false}
	_ = live.RecoveryReadback{ExecutionProfile: "", InputRequired: false, EgressRequired: false, InputObservationID: "", InputObservationSource: "", InputObservationResult: "", InputObservationGeneration: 0, JobID: 0}
}

func mrr05Cleanup(t *testing.T, h *brwHarness, episode string) {
	t.Helper()
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		tx, err := h.lp.f.owner.Begin(clean)
		if err != nil {
			t.Errorf("fixture cleanup begin: %v", err)
			return
		}
		defer tx.Rollback(clean)
		for _, q := range []string{`SET CONSTRAINTS ALL DEFERRED`, `ALTER TABLE live.media_input_recovery_observations DISABLE TRIGGER media_input_recovery_receipt_guard`, `DELETE FROM integration.operation_events WHERE episode_id=$1::uuid`, `DELETE FROM live.media_input_recovery_observations WHERE episode_id=$1::uuid`, `ALTER TABLE live.media_input_recovery_observations ENABLE TRIGGER media_input_recovery_receipt_guard`} {
			if q == `DELETE FROM integration.operation_events WHERE episode_id=$1::uuid` || q == `DELETE FROM live.media_input_recovery_observations WHERE episode_id=$1::uuid` {
				_, err = tx.Exec(clean, q, episode)
			} else {
				_, err = tx.Exec(clean, q)
			}
			if err != nil {
				t.Errorf("fixture cleanup: %v", err)
				return
			}
		}
		if err = tx.Commit(clean); err != nil {
			t.Errorf("fixture cleanup commit: %v", err)
		}
	})
}

// Restricted production SQL seeds receipts because the mixed provider observer
// is outside this card. Begin, Read, Witness and Timeout use the Go diagnostic API.
func TestLiveMediaRecoveryMRR05GoInputLedger(t *testing.T) {
	for _, timeoutFirst := range []bool{false, true} {
		name := "witness"
		if timeoutFirst {
			name = "gen0_timeout"
		}
		t.Run(name, func(t *testing.T) {
			h := brwStart(t)
			brwReserveGrant(t, h)
			ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
			defer cancel()
			login, recovery := lmaLogin(t, h.lp.f, "commerce_media_recovery")
			role := pgx.Identifier{login}.Sanitize()
			mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+role)
			mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+role+" WITH INHERIT TRUE, SET FALSE")
			ledger := live.NewMediaRecoveryLedgerWithBrowserInput(recovery)
			episode := randomUUID()
			mrr05Cleanup(t, h, episode)
			var gen int64
			var exists bool
			if err := h.lp.f.owner.QueryRow(ctx, `SELECT o.generation, EXISTS(SELECT 1 FROM live.media_execution_state x WHERE x.operation_id=o.id) FROM integration.operations o WHERE o.id=$1::uuid`, h.plan.OperationID).Scan(&gen, &exists); err != nil || gen != 0 || exists {
				t.Fatalf("gen0 precondition gen=%d projection=%v err=%v", gen, exists, err)
			}
			members, err := ledger.Begin(ctx, episode, 0, 1, true)
			if err != nil || len(members) != 1 {
				t.Fatalf("begin=%+v err=%v", members, err)
			}
			m := members[0]
			if m.Disposition != "pending" || m.EpisodeID != episode || m.OperationID != h.plan.OperationID || m.JobID != h.plan.JobID ||
				m.BaselineGeneration != 0 || m.CandidateCount != 1 || !m.CoverageKnown || m.BlockedByEpisodeID != "" ||
				m.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS" || !m.InputRequired || m.EgressRequired || m.DeadlineAt.IsZero() {
				t.Fatalf("admission=%+v", m)
			}
			read := func(disposition, scope, receipt string, generation, elapsed int64) live.RecoveryReadback {
				t.Helper()
				rows, err := ledger.Read(ctx, episode)
				if err != nil || len(rows) != 1 {
					t.Fatalf("read=%+v err=%v", rows, err)
				}
				r := rows[0]
				if r.EpisodeID != episode || r.OperationID != m.OperationID || r.JobID != m.JobID || r.ExecutionProfile != m.ExecutionProfile ||
					!r.InputRequired || r.EgressRequired || !r.CoverageKnown || r.CandidateCount != 1 || r.BlockedByEpisodeID != "" ||
					r.BaselineGeneration != 0 || r.Disposition != disposition || r.ScopeStatus != scope ||
					r.InputObservationID != receipt || r.InputObservationGeneration != generation ||
					r.ObservationID != "" || r.ObservationSource != "" || r.ObservationGeneration != 0 || r.WitnessElapsedMS != elapsed || r.CleanupRequired {
					t.Fatalf("readback=%+v", r)
				}
				wantSource, wantResult := "", ""
				if receipt != "" {
					wantSource, wantResult = "ROOM", "ABSENT"
				}
				if r.InputObservationSource != wantSource || r.InputObservationResult != wantResult {
					t.Fatalf("input receipt=%+v", r)
				}
				return r
			}
			if r := read("pending", "pending", "", 0, 0); !r.TimeoutAt.IsZero() {
				t.Fatalf("premature timeout=%+v", r)
			}
			if got, err := ledger.WitnessWithInput(ctx, episode, m.OperationID, "", "", 1000); err != nil || got != "unqualified" {
				t.Fatalf("unqualified=%s %v", got, err)
			}
			if timeoutFirst {
				if got, count, err := ledger.Timeout(ctx, episode, 90000); err != nil || got != "timed_out" || count != 1 {
					t.Fatalf("timeout=%s %d %v", got, count, err)
				}
				first := read("timeout", "overdue", "", 0, 0)
				if first.TimeoutAt.IsZero() {
					t.Fatal("missing timeout")
				}
				if got, err := ledger.WitnessWithInput(ctx, episode, m.OperationID, "", "", 1000); err != nil || got != "timeout_wins" {
					t.Fatalf("sticky=%s %v", got, err)
				}
				if got, count, err := ledger.Timeout(ctx, episode, 91000); err != nil || got != "already_timed_out" || count != 0 {
					t.Fatalf("replay=%s %d %v", got, count, err)
				}
				if r := read("timeout", "overdue", "", 0, 0); !r.TimeoutAt.Equal(first.TimeoutAt) {
					t.Fatalf("timeout changed=%+v", r)
				}
				return
			}
			var disposition, receipt string
			token := randomBytes(32)
			if err := recovery.QueryRow(ctx, `SELECT disposition,generation FROM live.claim_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea)`, episode, m.OperationID, m.JobID, token).Scan(&disposition, &gen); err != nil || disposition != "claimed" || gen <= m.BaselineGeneration {
				t.Fatalf("claim=%s %d %v", disposition, gen, err)
			}
			if err := recovery.QueryRow(ctx, `SELECT disposition,input_observation_id::text FROM live.record_browser_input_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`, episode, m.OperationID, gen, token).Scan(&disposition, &receipt); err != nil || disposition != "checked" || receipt == "" {
				t.Fatalf("record=%s %v", disposition, err)
			}
			if err := recovery.QueryRow(ctx, `SELECT live.finish_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea,'not_observed')`, episode, m.OperationID, gen, token).Scan(&disposition); err != nil || disposition != "released" {
				t.Fatalf("finish=%s %v", disposition, err)
			}
			read("checked", "pending", receipt, gen, 0)
			if got, err := ledger.WitnessWithInput(ctx, episode, m.OperationID, receipt, randomUUID(), 1000); err != nil || got != "unqualified" {
				t.Fatalf("extra egress=%s %v", got, err)
			}
			if got, err := ledger.WitnessWithInput(ctx, episode, m.OperationID, receipt, "", 1000); err != nil || got != "witnessed" {
				t.Fatalf("witness=%s %v", got, err)
			}
			if r := read("witnessed", "finished", receipt, gen, 1000); !r.TimeoutAt.IsZero() {
				t.Fatalf("unexpected timeout=%+v", r)
			}
			if got, err := ledger.WitnessWithInput(ctx, episode, m.OperationID, receipt, "", 1000); err != nil || got != "already_witnessed" {
				t.Fatalf("replay=%s %v", got, err)
			}
			read("witnessed", "finished", receipt, gen, 1000)
		})
	}
}

func TestLiveMediaRecoveryMRR05GoEmptyScope(t *testing.T) {
	h := brwRegistered(t)
	ctx := context.Background()
	login, recovery := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	role := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+role)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+role+" WITH INHERIT TRUE, SET FALSE")
	ledger := live.NewMediaRecoveryLedgerWithBrowserInput(recovery)
	episode := randomUUID()
	mrr05Cleanup(t, h, episode)
	members, err := ledger.Begin(ctx, episode, 0, 1, true)
	if err != nil || len(members) != 1 {
		t.Fatalf("empty begin=%+v %v", members, err)
	}
	m := members[0]
	if m.Disposition != "empty" || m.EpisodeID != episode || m.OperationID != "" || m.ExecutionProfile != "" || m.InputRequired || m.EgressRequired || m.JobID != 0 || m.BaselineGeneration != 0 || m.CandidateCount != 0 || !m.CoverageKnown {
		t.Fatalf("scope-only member=%+v", m)
	}
	rows, err := ledger.Read(ctx, episode)
	if err != nil || len(rows) != 1 {
		t.Fatalf("empty read=%+v %v", rows, err)
	}
	r := rows[0]
	if r.EpisodeID != episode || r.ScopeStatus != "empty" || r.OperationID != "" || r.Disposition != "" || r.ExecutionProfile != "" ||
		r.InputRequired || r.EgressRequired || r.JobID != 0 || r.BaselineGeneration != 0 || r.CandidateCount != 0 || !r.CoverageKnown ||
		r.InputObservationID != "" || r.InputObservationSource != "" || r.InputObservationResult != "" || r.InputObservationGeneration != 0 ||
		r.ObservationID != "" || r.ObservationSource != "" || r.ObservationGeneration != 0 || r.WitnessElapsedMS != 0 || !r.TimeoutAt.IsZero() || r.CleanupRequired {
		t.Fatalf("scope-only readback=%+v", r)
	}
}

// A valid former observer must be fenced after a real lease expiry and a second
// production claim. This is a stale-lease regression, not the 90s process gate.
// Dependencies: existing BRW/native fixtures and the frozen mixed recovery ABI.
func TestLiveMediaRecoveryMRR05StaleObserverNewOwner(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	login, recovery := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+name)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+name+" WITH INHERIT TRUE, SET FALSE")
	episode := randomUUID()
	mrr05Cleanup(t, h, episode)
	var disposition, operation string
	var job, baseline int64
	var inputRequired, egressRequired bool
	if err := recovery.QueryRow(ctx, `SELECT disposition,operation_id::text,job_id,baseline_generation,input_required,egress_required
	 FROM live.begin_media_recovery_episode_with_input($1::uuid,0,1,true)`, episode).
		Scan(&disposition, &operation, &job, &baseline, &inputRequired, &egressRequired); err != nil || disposition != "pending" || operation != h.plan.OperationID || job != h.plan.JobID || !inputRequired || egressRequired {
		t.Fatalf("stale-owner admission failed: disposition=%s state=%s", disposition, sqlState(err))
	}
	oldKey, newKey := randomBytes(32), randomBytes(32)
	if len(oldKey) != 32 || len(newKey) != 32 || string(oldKey) == string(newKey) {
		t.Fatal("independent 32-byte owner keys required")
	}
	claim := func(key []byte) int64 {
		t.Helper()
		var generation int64
		if err := recovery.QueryRow(ctx, `SELECT disposition,generation FROM live.claim_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea)`, episode, operation, job, key).
			Scan(&disposition, &generation); err != nil || disposition != "claimed" || generation <= baseline {
			t.Fatalf("production claim failed: disposition=%s generation=%d state=%s", disposition, generation, sqlState(err))
		}
		return generation
	}
	oldGeneration := claim(oldKey)
	if oldGeneration < 1 {
		t.Fatal("old owner must have a genuine positive generation")
	}
	// No backdating or fixture UPDATE: wait for the server's actual 30s lease.
	// The local deadline only bounds a failed wait; the DB decides expiry.
	waitDeadline := time.Now().Add(40 * time.Second)
	for {
		var expired bool
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT lease_until IS NOT NULL AND lease_until<=clock_timestamp()
		 FROM integration.operations WHERE id=$1::uuid AND generation=$2`, operation, oldGeneration).Scan(&expired); err != nil {
			t.Fatalf("server-clock lease check failed: state=%s", sqlState(err))
		}
		if expired {
			break
		}
		if time.Now().After(waitDeadline) {
			t.Fatal("old production lease did not expire within bounded wait")
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatal("context expired waiting for old lease")
		case <-timer.C:
		}
	}
	newGeneration := claim(newKey)
	if newGeneration <= oldGeneration {
		t.Fatal("new production owner did not advance the original generation")
	}
	assertCurrentOwner := func() {
		t.Helper()
		var valid bool
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT generation=$2 AND job_id=$3 AND state='UNKNOWN'
		 AND lease_mode='reconcile' AND lease_until>clock_timestamp() AND lease_token_hash=sha256($4::bytea)
		 FROM integration.operations WHERE id=$1::uuid`, operation, newGeneration, job, newKey).Scan(&valid); err != nil || !valid {
			t.Fatalf("new owner no longer active: valid=%v state=%s", valid, sqlState(err))
		}
	}
	assertCurrentOwner()
	// Keep secret-bearing snapshots private: only equality/counts reach failures.
	// Include the complete original operation/native job and business rows, so an
	// old finish cannot silently clear a lease, change disposition or spend work.
	type baselineState struct {
		privateJSON string
		events      int64
		receipts    int64
	}
	snapshot := func() baselineState {
		t.Helper()
		var state baselineState
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT jsonb_build_object(
		 'operation',to_jsonb(o),'execution',to_jsonb(x),'custody',to_jsonb(i),
		 'attempt',to_jsonb(a),'native',to_jsonb(j),'scope',to_jsonb(s),
		 'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM integration.operation_events e WHERE e.operation_id=o.id),
		 'receipts',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM live.media_input_recovery_observations r WHERE r.operation_id=o.id))::text,
		 (SELECT count(*) FROM integration.operation_events e WHERE e.operation_id=o.id),
		 (SELECT count(*) FROM live.media_input_recovery_observations r WHERE r.operation_id=o.id)
		 FROM integration.operations o JOIN live.media_execution_state x ON x.operation_id=o.id
		 JOIN live.media_input_custody i ON i.operation_id=o.id
		 JOIN live.media_attempts a ON a.id=o.media_attempt_id
		 JOIN river_media.river_job j ON j.id=o.job_id
		 JOIN live.media_recovery_episode_scope s ON s.episode_id=$2::uuid
		 WHERE o.id=$1::uuid`, operation, episode).Scan(&state.privateJSON, &state.events, &state.receipts); err != nil {
			t.Fatalf("private baseline read failed: state=%s", sqlState(err))
		}
		return state
	}
	before := snapshot()
	if before.events < 3 || before.receipts != 0 {
		t.Fatalf("expected admission/two claims and no receipts: events=%d receipts=%d", before.events, before.receipts)
	}
	assertUnchanged := func(path string) {
		t.Helper()
		after := snapshot()
		if after != before {
			t.Fatalf("stale %s changed durable state: equal=%v event_delta=%d receipt_delta=%d", path, after.privateJSON == before.privateJSON, after.events-before.events, after.receipts-before.receipts)
		}
		assertCurrentOwner()
	}
	record := `SELECT disposition,input_observation_id::text FROM live.record_browser_input_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`
	finish := `SELECT live.finish_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea,'not_observed')`
	var receipt string
	// Both old arguments came from a successful claim; all other arguments are
	// valid. ME409 must be the stale fence, never ME400 from a generation-zero shape.
	if err := recovery.QueryRow(ctx, record, episode, operation, oldGeneration, oldKey).Scan(&disposition, &receipt); sqlState(err) != "ME409" {
		t.Fatalf("stale record must raise ME409: state=%s", sqlState(err))
	}
	assertUnchanged("record")
	if err := recovery.QueryRow(ctx, finish, episode, operation, oldGeneration, oldKey).Scan(&disposition); sqlState(err) != "ME409" {
		t.Fatalf("stale finish must raise ME409: state=%s", sqlState(err))
	}
	assertUnchanged("finish")
	// Positive control through the same production record path proves that the
	// episode remains usable by its current owner, rather than rejecting everyone.
	if err := recovery.QueryRow(ctx, record, episode, operation, newGeneration, newKey).Scan(&disposition, &receipt); err != nil || disposition != "checked" || receipt == "" {
		t.Fatalf("current owner record failed: disposition=%s state=%s", disposition, sqlState(err))
	}
	var readOperation, readReceipt, readSource, readResult, memberDisposition string
	var readJob, readGeneration int64
	if err := recovery.QueryRow(ctx, `SELECT operation_id::text,job_id,input_observation_id::text,input_observation_generation,
	 input_observation_source,input_observation_result,disposition FROM live.read_media_recovery_episode_with_input($1::uuid)`, episode).
		Scan(&readOperation, &readJob, &readReceipt, &readGeneration, &readSource, &readResult, &memberDisposition); err != nil || readOperation != operation || readJob != job || readReceipt != receipt || readGeneration != newGeneration || readSource != "ROOM" || readResult != "ABSENT" || memberDisposition != "checked" {
		t.Fatalf("current owner committed readback failed: state=%s", sqlState(err))
	}
	afterCurrent := snapshot()
	if afterCurrent.events != before.events+1 || afterCurrent.receipts != before.receipts+1 {
		t.Fatalf("current proof must add exactly one event/receipt: event_delta=%d receipt_delta=%d", afterCurrent.events-before.events, afterCurrent.receipts-before.receipts)
	}
	assertCurrentOwner()
	if err := recovery.QueryRow(ctx, finish, episode, operation, newGeneration, newKey).Scan(&disposition); err != nil || disposition != "released" {
		t.Fatalf("current owner finish failed: disposition=%s state=%s", disposition, sqlState(err))
	}
}

// Both immutable liability bits belong to one original operation/native job.
// SQL/ledger only: fixture observations are not provider or process acceptance.
// Input precedes Egress as required by the contract: Egress clears the lease.
func TestLiveMediaRecoveryMRR05BothMembers(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	lease, err := brwClaim(ctx, h.bicHarness, h.plan, randomBytes(32))
	if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" || lease.generation < 1 {
		t.Fatalf("original claim disposition=%s state=%s", lease.disposition, sqlState(err))
	}
	brwSelectObserve(t, h, lease)
	material, err := brwReserveStart(ctx, h, lease, h.plan.RoomName,
		h.grant.PublisherIdentity, "PA_mrr05_both", "ACTIVE", true, false, true, false)
	if err != nil || material["mode"] != "dispatch" {
		t.Fatalf("original wire reservation failed: state=%s", sqlState(err))
	}
	// Do not log private dispatch material, tokens, or JSON snapshots.
	material = nil
	login, recovery := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+name)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+name+" WITH INHERIT TRUE, SET FALSE")
	episode := randomUUID()
	mrr05Cleanup(t, h, episode)
	started := time.Now()
	var disposition, operation, profile string
	var job, baseline int64
	var inputRequired, egressRequired bool
	if err := recovery.QueryRow(ctx, `SELECT disposition,operation_id::text,job_id,baseline_generation,
	 execution_profile,input_required,egress_required FROM live.begin_media_recovery_episode_with_input($1::uuid,0,1,true)`, episode).
		Scan(&disposition, &operation, &job, &baseline, &profile, &inputRequired, &egressRequired); err != nil ||
		disposition != "pending" || operation != h.plan.OperationID || job != h.plan.JobID || baseline != lease.generation ||
		profile != "LOCAL_SFU_MOCK_EGRESS" || !inputRequired || !egressRequired {
		t.Fatalf("both-bit admission failed: disposition=%s state=%s", disposition, sqlState(err))
	}
	// Read-only owner inspection binds immutable identity and native/dispatch/Stop
	// budgets. Recovery may advance generation, never spend these budgets.
	budget := func() string {
		t.Helper()
		var private string
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT jsonb_build_object(
		 'operation',o.id,'job',o.job_id,'attempt',to_jsonb(a),'native',to_jsonb(j),
		 'wire_at',x.wire_reserved_at,'wire_generation',x.wire_generation,'stop_count',x.stop_wire_count,
		 'custody',to_jsonb(i))::text
		 FROM integration.operations o JOIN live.media_attempts a ON a.id=o.media_attempt_id
		 JOIN river_media.river_job j ON j.id=o.job_id
		 JOIN live.media_execution_state x ON x.operation_id=o.id
		 JOIN live.media_input_custody i ON i.operation_id=o.id
		 WHERE o.id=$1::uuid AND a.id=$2::uuid AND j.id=$3`, operation, h.plan.AttemptID, job).Scan(&private); err != nil {
			t.Fatalf("identity/budget read failed: state=%s", sqlState(err))
		}
		return private
	}
	before := budget()
	assertOpen := func() {
		t.Helper()
		var open bool
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT i.state<>'CLOSED' AND x.resource_state<>'TERMINAL'
		 AND o.state IN ('DISPATCHING','UNKNOWN') AND j.finalized_at IS NULL
		 FROM integration.operations o JOIN live.media_input_custody i ON i.operation_id=o.id
		 JOIN live.media_execution_state x ON x.operation_id=o.id JOIN river_media.river_job j ON j.id=o.job_id
		 WHERE o.id=$1::uuid`, operation).Scan(&open); err != nil || !open || budget() != before {
			t.Fatalf("proof changed liability/identity/budget: open=%v state=%s", open, sqlState(err))
		}
	}
	read := func(wantInput, wantEgress string, inputGen, egressGen int64, wantDisposition, wantScope string) {
		t.Helper()
		var valid bool
		if err := recovery.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(
		 operation_id=$2::uuid AND job_id=$3 AND baseline_generation=$4 AND candidate_count=1 AND coverage_known
		 AND execution_profile='LOCAL_SFU_MOCK_EGRESS' AND input_required AND egress_required
		 AND coalesce(input_observation_id::text,'')=$5 AND coalesce(observation_id::text,'')=$6
		 AND coalesce(input_observation_generation,0)=$7 AND coalesce(observation_generation,0)=$8
		 AND coalesce(input_observation_source,'')=CASE WHEN $5='' THEN '' ELSE 'ROOM' END
		 AND coalesce(input_observation_result,'')=CASE WHEN $5='' THEN '' ELSE 'ABSENT' END
		 AND coalesce(observation_source,'')=CASE WHEN $6='' THEN '' ELSE 'ROOM' END
		 AND disposition=$9 AND scope_status=$10 AND timeout_at IS NULL)
		 FROM live.read_media_recovery_episode_with_input($1::uuid)`, episode, operation, job, baseline,
			wantInput, wantEgress, inputGen, egressGen, wantDisposition, wantScope).Scan(&valid); err != nil || !valid {
			t.Fatalf("committed composite readback mismatch: state=%s", sqlState(err))
		}
	}
	witness := func(input, egress any, want string, elapsed int64) {
		t.Helper()
		var got string
		if err := recovery.QueryRow(ctx, `SELECT live.witness_media_recovery_episode_with_input($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::bigint)`,
			episode, operation, input, egress, elapsed).Scan(&got); err != nil || got != want {
			t.Fatalf("composite witness got=%s want=%s state=%s", got, want, sqlState(err))
		}
	}
	read("", "", 0, 0, "pending", "pending")
	witness(nil, nil, "unqualified", time.Since(started).Milliseconds())
	assertOpen()
	// Wait on the actual dispatch lease; no backdating or manual generation.
	waitDeadline := time.Now().Add(40 * time.Second)
	for {
		var expired bool
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT lease_until IS NOT NULL AND lease_until<=clock_timestamp()
		 FROM integration.operations WHERE id=$1::uuid AND generation=$2`, operation, lease.generation).Scan(&expired); err != nil {
			t.Fatalf("dispatch lease read failed: state=%s", sqlState(err))
		}
		if expired {
			break
		}
		if time.Now().After(waitDeadline) {
			t.Fatal("dispatch lease did not expire within 40s")
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatal("context expired waiting for dispatch lease")
		case <-timer.C:
		}
	}
	claim := func(key []byte, wantInput string) int64 {
		t.Helper()
		var generation int64
		var valid bool
		if len(key) != 32 {
			t.Fatal("claim requires a 32-byte key")
		}
		if err := recovery.QueryRow(ctx, `SELECT disposition,generation,
		 execution_profile='LOCAL_SFU_MOCK_EGRESS' AND input_required AND egress_required
		 AND coalesce(input_observation_id::text,'')=$5 AND egress_observation_id IS NULL
		 FROM live.claim_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea)`,
			episode, operation, job, key, wantInput).Scan(&disposition, &generation, &valid); err != nil || disposition != "claimed" || generation <= baseline || !valid {
			t.Fatalf("both-bit claim failed: disposition=%s state=%s", disposition, sqlState(err))
		}
		return generation
	}
	key := randomBytes(32)
	generation := claim(key, "")
	var inputID, egressID string
	inputRecord := `SELECT disposition,input_observation_id::text FROM live.record_browser_input_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','ABSENT',NULL,NULL)`
	if err := recovery.QueryRow(ctx, inputRecord, episode, operation, generation, key).Scan(&disposition, &inputID); err != nil || disposition != "checked" || inputID == "" {
		t.Fatalf("input record failed: disposition=%s state=%s", disposition, sqlState(err))
	}
	read(inputID, "", generation, 0, "pending", "pending")
	witness(inputID, nil, "unqualified", time.Since(started).Milliseconds())
	witness(nil, inputID, "unqualified", time.Since(started).Milliseconds())
	witness(inputID, randomUUID(), "unqualified", time.Since(started).Milliseconds())
	assertOpen()
	// Fail the missing side under a wrong token, then recover its committed
	// counterpart by readback. Each call is an independent committed statement.
	egressRecord := `SELECT disposition,observation_id::text FROM live.record_media_recovery_observation_with_input(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,'ROOM','EG_mrr05_both',$5::text,'EGRESS_ACTIVE',100,120,0)`
	if err := recovery.QueryRow(ctx, egressRecord, episode, operation, generation, randomBytes(32), h.plan.RoomName).Scan(&disposition, &egressID); sqlState(err) != "ME409" {
		t.Fatalf("wrong egress authority state=%s", sqlState(err))
	}
	read(inputID, "", generation, 0, "pending", "pending")
	assertOpen()
	if err := recovery.QueryRow(ctx, `SELECT live.finish_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea,'not_observed')`,
		episode, operation, generation, key).Scan(&disposition); err != nil || disposition != "released" {
		t.Fatalf("partial finish disposition=%s state=%s", disposition, sqlState(err))
	}
	read(inputID, "", generation, 0, "pending", "pending")
	newKey := randomBytes(32)
	if string(newKey) == string(key) {
		t.Fatal("new owner requires an independent key")
	}
	newGeneration := claim(newKey, inputID)
	if newGeneration <= generation {
		t.Fatal("new claim failed to advance generation")
	}
	var duplicate string
	if err := recovery.QueryRow(ctx, inputRecord, episode, operation, newGeneration, newKey).Scan(&disposition, &duplicate); sqlState(err) != "ME409" {
		t.Fatalf("first input proof was replaceable: state=%s", sqlState(err))
	}
	read(inputID, "", generation, 0, "pending", "pending")
	assertOpen()
	if err := recovery.QueryRow(ctx, egressRecord, episode, operation, newGeneration, newKey, h.plan.RoomName).Scan(&disposition, &egressID); err != nil || disposition != "checked" || egressID == "" || egressID == inputID {
		t.Fatalf("egress record failed: disposition=%s state=%s", disposition, sqlState(err))
	}
	read(inputID, egressID, generation, newGeneration, "checked", "pending")
	// Egress status is not exported by the public reader; verify the exact
	// persisted result via read-only owner inspection, never fabricate proof.
	var correlated bool
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT r.operation_id=v.operation_id AND r.attempt_id=v.attempt_id
	 AND r.operation_id=$3::uuid AND r.attempt_id=$4::uuid AND r.job_id=$5 AND r.episode_id=$6::uuid
	 AND r.generation=$7 AND v.generation=$8 AND r.source='ROOM' AND r.result='ABSENT'
	 AND v.source='ROOM' AND v.status='EGRESS_ACTIVE' AND v.egress_id='EG_mrr05_both'
	 AND r.room_name=v.room_name AND r.project_id=v.project_id
	 FROM live.media_input_recovery_observations r CROSS JOIN live.media_observations v
	 WHERE r.id=$1::uuid AND v.id=$2::uuid`, inputID, egressID, operation, h.plan.AttemptID, job, episode, generation, newGeneration).Scan(&correlated); err != nil || !correlated {
		t.Fatalf("exact committed proof correlation failed: state=%s", sqlState(err))
	}
	witness(nil, egressID, "unqualified", time.Since(started).Milliseconds())
	witness(egressID, inputID, "unqualified", time.Since(started).Milliseconds())
	witness(randomUUID(), egressID, "unqualified", time.Since(started).Milliseconds())
	read(inputID, egressID, generation, newGeneration, "checked", "pending")
	assertOpen()
	elapsed := time.Since(started).Milliseconds()
	if elapsed > 90000 {
		t.Fatal("composite committed readback exceeded SQL episode budget")
	}
	witness(inputID, egressID, "witnessed", elapsed)
	read(inputID, egressID, generation, newGeneration, "witnessed", "finished")
	witness(inputID, egressID, "already_witnessed", elapsed)
	assertOpen()
}
