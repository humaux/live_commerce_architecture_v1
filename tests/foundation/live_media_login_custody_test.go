package foundation_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/identity"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// These are real identity.Service login-flow sessions mapped to the same
// merchant. The older LMP/LME fixture token remains unrelated to both logins.
type mlcLogins struct {
	a, b, aID, bID   string
	aExpiry, bExpiry time.Time
	revision         int64
	service          *identity.Service
}

func mlcTwoLogins(t *testing.T, h *lmpHarness) mlcLogins {
	t.Helper()
	ctx := context.Background()
	s, provider, _ := identityFixture(t)
	if _, err := h.lp.f.owner.Exec(ctx, `INSERT INTO identity.external_identities(issuer,subject,principal_id)
		VALUES('https://idp.example',$1,$2)`, provider.subject, h.lp.actor); err != nil {
		t.Fatal(err)
	}
	a, b := identityLogin(t, s), identityLogin(t, s)
	if a.PrincipalID != h.lp.actor || b.PrincipalID != h.lp.actor || a.Token == b.Token {
		t.Fatal("login flow did not issue two distinct sessions for the fixture merchant")
	}
	ah, bh := tokenHash(a.Token), tokenHash(b.Token)
	t.Cleanup(func() {
		_, _ = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.session_events WHERE session_id IN
			(SELECT id FROM identity.sessions WHERE token_hash=$1 OR token_hash=$2)`, ah, bh)
		_, _ = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.external_identities
			WHERE issuer='https://idp.example' AND subject=$1 AND principal_id=$2`, provider.subject, h.lp.actor)
	})
	var out mlcLogins
	out.a, out.b, out.aExpiry, out.bExpiry = a.Token, b.Token, a.ExpiresAt, b.ExpiresAt
	out.service = s
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT id::text FROM identity.sessions WHERE token_hash=$1`, ah).Scan(&out.aID); err != nil {
		t.Fatal(err)
	}
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT id::text FROM identity.sessions WHERE token_hash=$1`, bh).Scan(&out.bID); err != nil {
		t.Fatal(err)
	}
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT authz_revision FROM identity.memberships WHERE tenant_id=$1 AND principal_id=$2`,
		h.lp.f.tenantA, h.lp.actor).Scan(&out.revision); err != nil {
		t.Fatal(err)
	}
	if out.aID == out.bID || out.aID == "" || out.bID == "" {
		t.Fatal("distinct login tokens shared one database session ID")
	}
	return out
}

func mlcCustody(t *testing.T, h *lmpHarness, attempt string) (string, int64, time.Time) {
	t.Helper()
	var loginID string
	var revision int64
	var expiry time.Time
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT login_session_id::text,authz_revision,login_expires_at
		FROM live.media_login_custody WHERE attempt_id=$1`, attempt).Scan(&loginID, &revision, &expiry); err != nil {
		t.Fatal(err)
	}
	return loginID, revision, expiry
}

func TestLiveMediaExecutionMLC01AtomicCustody(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		h := lmpSetup(t, false)
		login := mlcTwoLogins(t, h)
		rollback := errors.New("rollback after verified custody")
		err := platform.WithScope(context.Background(), h.lp.f.runtime, login.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := h.planner.PlanStart(context.Background(), tx, scope, login.a, t04Key("mlc-rollback"), h.input)
			if err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) || lmpFacts(t, h) != [6]int64{} {
			t.Fatalf("rollback retained media artifacts: %v facts=%v", err, lmpFacts(t, h))
		}
		var count int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM live.media_login_custody WHERE tenant_id=$1 AND store_id=$2`,
			h.lp.f.tenantA, h.lp.f.storeA1).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rollback retained custody: %d %v", count, err)
		}
	})

	t.Run("concurrent-replay", func(t *testing.T) {
		h := lmpSetup(t, false)
		login := mlcTwoLogins(t, h)
		key := t04Key("mlc-concurrent")
		var results [2]live.MediaStartResult
		var errs [2]error
		var wg sync.WaitGroup
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i], errs[i] = lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, key, h.input)
			}(i)
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil || results[0] != results[1] || results[0].AttemptID == "" {
			t.Fatalf("same login concurrency: %v/%v %+v/%+v", errs[0], errs[1], results[0], results[1])
		}
		bound, revision, expiry := mlcCustody(t, h, results[0].AttemptID)
		if bound != login.aID || revision != login.revision || !expiry.Equal(login.aExpiry) || lmpFacts(t, h) != [6]int64{1, 1, 1, 1, 1, 1} {
			t.Fatalf("not one exact fixed custody: %s/%d/%s facts=%v", bound, revision, expiry, lmpFacts(t, h))
		}
		if _, err := lmpStart(context.Background(), h, h.planner, login.b, h.lp.f.storeA1, key, h.input); err == nil {
			t.Fatal("same principal's second login took over Start replay")
		}
		changed := h.input
		changed.ExpectedSessionVersion++
		if _, err := lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, key, changed); !errors.Is(err, command.ErrConflict) {
			t.Fatalf("same key changed body: %v", err)
		}
		bound2, revision2, expiry2 := mlcCustody(t, h, results[0].AttemptID)
		if bound2 != bound || revision2 != revision || !expiry2.Equal(expiry) || lmpFacts(t, h) != [6]int64{1, 1, 1, 1, 1, 1} {
			t.Fatal("replay changed attempt, custody, expiry or artifacts")
		}
	})

	t.Run("committed-ack-loss", func(t *testing.T) {
		h := lmpSetup(t, false)
		login := mlcTwoLogins(t, h)
		config := h.lp.f.runtime.Config()
		config.MaxConns = 1
		loss := &hpCommitLoss{}
		dial := config.ConnConfig.DialFunc
		config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &hpCommitLossConn{Conn: conn, loss: loss}, nil
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		key := t04Key("mlc-ack-loss")
		loss.armed.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := mlcStartOnPool(ctx, h, pool, lmpPlanner(t, pool, "river_media"), login.a, key)
		if err == nil || !loss.committed.Load() {
			t.Fatalf("lost real COMMIT ACK was not ambiguous: %+v %v committed=%t", out, err, loss.committed.Load())
		}
		if facts := lmpFacts(t, h); facts != [6]int64{1, 1, 1, 1, 1, 1} {
			t.Fatalf("committed Start missing: %v", facts)
		}
		result, err := lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, key, h.input)
		if err != nil {
			t.Fatalf("same initiating login cannot recover committed result: %v", err)
		}
		if out.AttemptID != "" && out != result {
			t.Fatal("local pre-ACK result differed from durable replay identity")
		}
		bound, revision, expiry := mlcCustody(t, h, result.AttemptID)
		if bound != login.aID || revision != login.revision || !expiry.Equal(login.aExpiry) || lmpFacts(t, h) != [6]int64{1, 1, 1, 1, 1, 1} {
			t.Fatal("ACK-loss recovery rebound login, extended expiry or duplicated artifacts")
		}
	})

	t.Run("nonfinite-session-and-custody", func(t *testing.T) {
		h := lmpSetup(t, false)
		login := mlcTwoLogins(t, h)
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET expires_at='infinity'::timestamptz WHERE id=$1`, login.aID); err != nil {
			t.Fatal(err)
		}
		if _, err := lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, t04Key("mlc-infinite-login"), h.input); err == nil || lmpFacts(t, h) != [6]int64{} {
			t.Fatalf("nonfinite login admitted Start: %v facts=%v", err, lmpFacts(t, h))
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET expires_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, login.aID); err != nil {
			t.Fatal(err)
		}
		out, err := lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, t04Key("mlc-finite-control"), h.input)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `DELETE FROM live.media_login_custody WHERE attempt_id=$1`, out.AttemptID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `INSERT INTO live.media_login_custody
			(attempt_id,tenant_id,store_id,login_session_id,authz_revision,login_expires_at)
			VALUES($1,$2,$3,$4,$5,'infinity'::timestamptz)`, out.AttemptID, h.lp.f.tenantA, h.lp.f.storeA1, login.aID, login.revision); sqlState(err) != "23514" {
			t.Fatalf("nonfinite custody insert was not constrained: %v", err)
		}
	})
}

func mlcStartOnPool(ctx context.Context, h *lmpHarness, pool *pgxpool.Pool, planner *live.MediaPlanner, token, key string) (live.MediaStartResult, error) {
	var out live.MediaStartResult
	err := platform.WithScope(ctx, pool, token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = planner.PlanStart(ctx, tx, scope, token, key, h.input)
		return err
	})
	return out, err
}

func TestLiveMediaExecutionMLC02ReplayCurrentLoginAndStopAuthority(t *testing.T) {
	for _, change := range []string{"other-login-logout", "initiator-logout", "initiator-expiry", "revision", "store-read", "live-manage"} {
		t.Run(change, func(t *testing.T) {
			h := lmpSetup(t, false)
			login := mlcTwoLogins(t, h)
			key := t04Key("mlc-current")
			out, err := lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, key, h.input)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "other-login-logout":
				err = login.service.Logout(context.Background(), login.b)
			case "initiator-logout":
				err = login.service.Logout(context.Background(), login.a)
			case "initiator-expiry":
				_, err = h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, login.aID)
			case "revision":
				tx, beginErr := h.lp.f.owner.Begin(context.Background())
				if beginErr != nil {
					t.Fatal(beginErr)
				}
				defer tx.Rollback(context.Background())
				_, err = tx.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='store:read'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor)
				if err == nil {
					_, err = tx.Exec(context.Background(), `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.lp.f.tenantA, h.lp.actor)
				}
				if err == nil {
					_, err = tx.Exec(context.Background(), `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'store:read')`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor)
				}
				if err == nil {
					err = tx.Commit(context.Background())
				}
			case "store-read", "live-manage":
				permission := "store:read"
				if change == "live-manage" {
					permission = "live:manage"
				}
				_, err = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`,
					h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor, permission)
			}
			if err != nil {
				t.Fatal(err)
			}
			replay, replayErr := lmpStart(context.Background(), h, h.planner, login.a, h.lp.f.storeA1, key, h.input)
			if change == "other-login-logout" {
				if replayErr != nil || replay != out {
					t.Fatalf("unrelated login logout changed publisher: %+v %v", replay, replayErr)
				}
			} else if replayErr == nil {
				t.Fatalf("%s retained Start replay permission", change)
			}
			if _, err := lmpStart(context.Background(), h, h.planner, login.b, h.lp.f.storeA1, key, h.input); err == nil {
				t.Fatal("second login rebound existing attempt")
			}
			bound, revision, expiry := mlcCustody(t, h, out.AttemptID)
			if bound != login.aID || revision != login.revision || !expiry.Equal(login.aExpiry) || lmpFacts(t, h) != [6]int64{1, 1, 1, 1, 1, 1} {
				t.Fatal("authority change mutated frozen Start")
			}
		})
	}
	t.Run("independent-authorized-stop", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no wire expected", 500) })
		out, err := lmrRequest(context.Background(), h, h.lp.peerToken, h.lp.f.storeA1, t04Key("mlc-peer-stop"),
			live.MediaStopInput{SessionID: h.session, AttemptID: h.plan.AttemptID})
		if err != nil || out.AttemptID != h.plan.AttemptID {
			t.Fatalf("other authorized merchant cannot Stop: %+v %v", out, err)
		}
	})
}

func mlcWaiter(h *lmpHarness, token, key string) (<-chan int, <-chan lmpWaitResult) {
	pid := make(chan int, 1)
	done := make(chan lmpWaitResult, 1)
	go func() {
		var out live.MediaStartResult
		err := platform.WithScope(context.Background(), h.lp.f.runtime, token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			var backend int
			if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
				return err
			}
			pid <- backend
			var err error
			out, err = h.planner.PlanStart(context.Background(), tx, scope, token, key, h.input)
			return err
		})
		done <- lmpWaitResult{out: out, err: err}
	}()
	return pid, done
}

func TestLiveMediaExecutionMLC03ObservedWaitAndFinalDispatch(t *testing.T) {
	for _, change := range []string{"logout", "expiry"} {
		t.Run("exact-login-row-wait-"+change, func(t *testing.T) {
			h := lmpSetup(t, false)
			login := mlcTwoLogins(t, h)
			ctx := context.Background()
			holder, err := h.lp.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(ctx)
			var holderPID int
			if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if _, err := holder.Exec(ctx, `SELECT id FROM identity.sessions WHERE id=$1 FOR UPDATE`, login.aID); err != nil {
				t.Fatal(err)
			}
			waiterPID, done := mlcWaiter(h, login.a, t04Key("mlc-login-row-"+change))
			lmaObserveBlock(t, h.lp.f.owner, <-waiterPID, holderPID, false)
			switch change {
			case "logout":
				_, err = holder.Exec(ctx, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE id=$1`, login.aID)
			case "expiry":
				_, err = holder.Exec(ctx, `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, login.aID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			result := lmpAwaitWaiter(t, done)
			if result.err == nil || result.out != (live.MediaStartResult{}) || lmpFacts(t, h) != [6]int64{} {
				t.Fatalf("exact login wait admitted revoked/expired Start: %+v %v facts=%v", result.out, result.err, lmpFacts(t, h))
			}
		})
	}
	for _, change := range []string{"logout", "expiry", "revision"} {
		t.Run(change, func(t *testing.T) {
			h := lmpSetup(t, false)
			login := mlcTwoLogins(t, h)
			ctx := context.Background()
			holder, err := h.lp.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(ctx)
			var holderPID int
			if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if _, err := holder.Exec(ctx, `SELECT id FROM integration.bindings WHERE id=$1 FOR UPDATE`, h.media); err != nil {
				t.Fatal(err)
			}
			waiterPID, done := mlcWaiter(h, login.a, t04Key("mlc-final-"+change))
			lmaObserveBlock(t, h.lp.f.owner, <-waiterPID, holderPID, false)
			switch change {
			case "logout":
				err = login.service.Logout(ctx, login.a)
			case "expiry":
				_, err = holder.Exec(ctx, `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, login.aID)
			case "revision":
				_, err = holder.Exec(ctx, `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.lp.f.tenantA, h.lp.actor)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			result := lmpAwaitWaiter(t, done)
			if result.err == nil || result.out != (live.MediaStartResult{}) || lmpFacts(t, h) != [6]int64{} {
				t.Fatalf("%s after real PG wait admitted Start: %+v %v facts=%v", change, result.out, result.err, lmpFacts(t, h))
			}
		})
	}
	t.Run("pre-reservation-revocation", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { t.Fatal("revoked login caused provider Start") })
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(h.lp.token)); err != nil {
			t.Fatal(err)
		}
		lease := h.claim(t, 30)
		if lease.disposition != "terminal" || h.starts.Load() != 0 || h.facts(t).reserved {
			t.Fatalf("revoked login passed final dispatch: %+v facts=%+v", lease, h.facts(t))
		}
	})
	t.Run("final-reservation-revocation", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) {
			t.Error("revoked login caused provider Start")
			http.Error(w, "unexpected", 500)
		})
		lease := h.claim(t, 30)
		if lease.disposition != "claimed" || lease.mode != "dispatch" {
			t.Fatalf("not a dispatch claim: %+v", lease)
		}
		h.load(t, lease)
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(h.lp.token)); err != nil {
			t.Fatal(err)
		}
		if err := h.reserve(t, lease); err == nil || h.starts.Load() != 0 || h.facts(t).reserved {
			t.Fatalf("post-claim revocation passed final reservation: %v facts=%+v", err, h.facts(t))
		}
	})
}

func TestLiveMediaExecutionMLC04PostReservationLifetime(t *testing.T) {
	for _, change := range []string{"logout", "expiry", "store-read", "revision", "historical-unbound"} {
		t.Run(change, func(t *testing.T) {
			var h *lmeHarness
			h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/twirp/livekit.Egress/ListEgress":
					lmeReply(w, `{"items":[`+h.observation("EG_mlc04", "EGRESS_ACTIVE", 100, 120, 0)+`]}`)
				case "/twirp/livekit.Egress/StopEgress":
					lmeReply(w, h.observation("EG_mlc04", "EGRESS_ENDING", 100, 130, 0))
				default:
					http.Error(w, "unexpected wire", 500)
				}
			})
			lease := h.claim(t, 30)
			if lease.disposition != "claimed" || lease.mode != "dispatch" {
				t.Fatalf("not a dispatch claim: %+v", lease)
			}
			if err := h.reserve(t, lease); err != nil {
				t.Fatal(err)
			}
			var err error
			switch change {
			case "logout":
				_, err = h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(h.lp.token))
			case "expiry":
				_, err = h.lp.f.owner.Exec(context.Background(), `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE token_hash=$1`, tokenHash(h.lp.token))
			case "store-read":
				_, err = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='store:read'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor)
			case "revision":
				_, err = h.lp.f.owner.Exec(context.Background(), `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.lp.f.tenantA, h.lp.actor)
			case "historical-unbound":
				// Owner-only simulation of a pre-0039 already-reserved row.
				_, err = h.lp.f.owner.Exec(context.Background(), `DELETE FROM live.media_login_custody WHERE attempt_id=$1`, h.plan.AttemptID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if disposition, err := h.record(t, lease, "START", "EG_mlc04", "EGRESS_ACTIVE", 100, 110, 0); err != nil || disposition != "observe" {
				t.Fatalf("post-reservation exact fact discarded: %s %v", disposition, err)
			}
			before := h.facts(t)
			if !before.cleanup || before.operation != "UNKNOWN" || !before.reserved || before.egress != "EG_mlc04" {
				t.Fatalf("access loss discarded reserved resource: %+v", before)
			}
			h.startWorker(t)
			h.await(t, 25*time.Second, func(f lmeFacts) bool { return f.observations >= 2 && !f.leaseOpen })
			var operation, attempt string
			var job int64
			if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.id::text,a.id::text,o.job_id FROM integration.operations o
				JOIN live.media_attempts a ON a.id=o.media_attempt_id WHERE o.id=$1`, h.plan.OperationID).Scan(&operation, &attempt, &job); err != nil {
				t.Fatal(err)
			}
			if operation != h.plan.OperationID || attempt != h.plan.AttemptID || job != h.plan.JobID || h.starts.Load() != 0 || h.stops.Load() == 0 {
				t.Fatal("post-loss worker changed identity or repeated Start/failed Stop")
			}
			if f := h.facts(t); f.operation != "UNKNOWN" || !f.cleanup || f.resource == "TERMINAL" {
				t.Fatalf("ambiguous cleanup falsely closed: %+v", f)
			}
		})
	}
}

func TestLiveMediaExecutionMLC05PrivateACLAndLegacyUnbound(t *testing.T) {
	h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy unbound attempt caused provider Start")
		http.Error(w, "unexpected", 500)
	})
	ctx := context.Background()
	var definer, path, owner, runtimeOnly bool
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT p.prosecdef,
		coalesce(p.proconfig,ARRAY[]::text[]) @> ARRAY['search_path=pg_catalog'],
		pg_get_userbyid(p.proowner)='commerce_media_writer',
		has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
		FROM pg_proc p WHERE p.oid=to_regprocedure('live.assert_media_start_login(bytea,uuid,uuid)')`).Scan(&definer, &path, &owner, &runtimeOnly); err != nil || !definer || !path || !owner || !runtimeOnly {
		t.Fatalf("unsafe login guard: %t/%t/%t/%t %v", definer, path, owner, runtimeOnly, err)
	}
	for label, pool := range map[string]*pgxpool.Pool{"runtime": h.lp.f.runtime, "worker": h.worker, "executor": h.executor, "registrar": h.registrar} {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM live.media_login_custody`).Scan(&count); sqlState(err) != "42501" {
			t.Fatalf("%s did not get table permission denial: %v", label, err)
		}
		for _, signature := range []string{"identity.lock_media_login(bytea,uuid)", "live.media_login_eligible(uuid)"} {
			var oid uint32
			if err := h.lp.f.owner.QueryRow(ctx, `SELECT to_regprocedure($1)::oid`, signature).Scan(&oid); err != nil || oid == 0 {
				t.Fatalf("private function missing %s: %v", signature, err)
			}
			var allowed bool
			if err := pool.QueryRow(ctx, `SELECT has_function_privilege(current_user,$1::oid,'EXECUTE')`, oid).Scan(&allowed); err != nil || allowed {
				t.Fatalf("%s can execute private %s: %t %v", label, signature, allowed, err)
			}
		}
	}
	// Owner-only simulation of a pre-0039 attempt: no current login may be
	// guessed or backfilled to it. This is not a full historical migration run.
	if _, err := h.lp.f.owner.Exec(ctx, `DELETE FROM live.media_login_custody WHERE attempt_id=$1`, h.plan.AttemptID); err != nil {
		t.Fatal(err)
	}
	lease := h.claim(t, 30)
	if lease.disposition != "terminal" || h.starts.Load() != 0 {
		t.Fatalf("unbound legacy attempt gained dispatch: %+v", lease)
	}
	var count int
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM live.media_login_custody WHERE attempt_id=$1`, h.plan.AttemptID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("missing historical custody fabricated: %d %v", count, err)
	}
	guard := func(hash []byte, store string) error {
		return platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `SELECT live.assert_media_start_login($1,$2::uuid,$3::uuid)`, hash, store, h.plan.AttemptID)
			return err
		})
	}
	if err := guard(tokenHash(h.lp.token), h.lp.f.storeA1); err == nil {
		t.Fatal("runtime guard accepted unbound historical attempt")
	}
	if err := guard(tokenHash(h.lp.token), h.lp.f.storeA2); err == nil {
		t.Fatal("runtime guard accepted a different store")
	}
	if err := guard([]byte{1}, h.lp.f.storeA1); sqlState(err) != "MP400" {
		t.Fatalf("malformed login hash did not fail closed: %v", err)
	}
}
