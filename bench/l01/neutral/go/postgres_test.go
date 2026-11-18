// Purpose: run independent W1 wire/readback checks and transaction faults against an owned PostgreSQL 18 fixture.
// Depends on: the neutral JS checks, pgx, Node, and the integrator's already-running disposable cluster.
// Used by: the integrator's opt-in base and extended fixture runs.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/bench/l01/neutral"
)

const readerRole = "l01_w1_reader"

func TestHTTPPostgres(t *testing.T) {
	mode := os.Getenv("LC_W1_GO_MODE")
	if mode == "" {
		t.Skip("NOT_RUN: select base or extended with LC_W1_GO_MODE on a fresh fixture cluster")
	}
	if mode != "base" && mode != "extended" {
		t.Fatal("LC_W1_GO_MODE must be base or extended")
	}
	if _, err := fixtureConnString(ingressRole); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PGUSER") != "postgres" {
		t.Fatal("the integrator's fixture administrator must be PGUSER=postgres")
	}
	admin := connectFixture(t, "postgres", os.Getenv("PGPASSWORD"))
	var fresh bool
	err := admin.QueryRow(t.Context(), `
		SELECT current_setting('server_version_num')::int BETWEEN 180000 AND 189999
			AND current_database() = 'lc_neutral_w1_test'
			AND session_user = 'postgres' AND current_user = session_user
			AND to_regnamespace('l01_w1') IS NULL
			AND NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname IN ('l01_w1_ingress','l01_w1_reader','l01_w1_owner'))`).Scan(&fresh)
	if err != nil || !fresh {
		t.Fatal("requires a fresh PostgreSQL 18 fixture cluster and empty fixture database")
	}

	nodeFixture(t, "postgres", os.Getenv("PGPASSWORD"), "../pg/check.mjs", "setup")
	ingressPassword, readerPassword := rand.Text(), rand.Text()
	// rand.Text uses only the base32 alphabet. The fixture admin alone provisions these logins.
	if _, err := admin.Exec(t.Context(), "ALTER ROLE l01_w1_ingress LOGIN PASSWORD '"+ingressPassword+
		"'; ALTER ROLE l01_w1_reader LOGIN PASSWORD '"+readerPassword+"'"); err != nil {
		t.Fatal("could not provision restricted fixture logins")
	}

	pool, err := openIngress(t.Context(), ingressPassword)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	reader := connectFixture(t, readerRole, readerPassword)
	var principal string
	if err := reader.QueryRow(t.Context(), "SELECT session_user").Scan(&principal); err != nil || principal != readerRole {
		t.Fatal("independent readback requires the actual reader login")
	}
	_, err = pool.Exec(t.Context(), "SELECT * FROM l01_w1.inbox LIMIT 0")
	requireDenied(t, err)
	_, err = reader.Exec(t.Context(), "INSERT INTO l01_w1.inbox DEFAULT VALUES")
	requireDenied(t, err)
	_, err = reader.Exec(t.Context(), "SELECT l01_w1.admit(NULL,NULL,NULL,NULL,NULL)")
	requireDenied(t, err)

	server := httptest.NewServer(newWebhook(pool.BeginTx))
	t.Cleanup(server.Close)
	port, ok := strings.CutPrefix(server.URL, "http://127.0.0.1:")
	if !ok {
		t.Fatal("IPv4 loopback listener required")
	}
	url := server.URL + endpoint
	client := server.Client()
	client.Timeout = 7 * time.Second

	wire := []string{"../http/check.mjs", port}
	readback := "readback"
	if mode == "extended" {
		wire = append(wire, "--extended")
		readback = "readback-extended"
	}
	nodeFixture(t, "", "", wire...)
	nodeFixture(t, readerRole, readerPassword, "../pg/check.mjs", readback)

	if mode == "extended" {
		t.Run("quarantine keeps first envelope", func(t *testing.T) {
			corpus, err := neutral.LoadFile("../w1_corpus.json")
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range corpus.Cases {
				if !item.ExpectedAdmit {
					continue
				}
				for _, asset := range []string{"1000000001901104", "1000000001901103"} {
					envelope := func() string {
						var saved string
						err := reader.QueryRow(t.Context(), `SELECT concat_ws(':', id, key_id,
							encode(nonce,'hex'), encode(ciphertext,'hex'))
							FROM l01_w1.quarantine WHERE asset_id=$1 AND external_key=$2`, asset, item.EventID).Scan(&saved)
						if err != nil {
							t.Fatal("could not read committed quarantine envelope")
						}
						return saved
					}
					before := envelope()
					raw := strings.ReplaceAll(item.RawBody, neutral.DefaultAuthoritativeAssetID, asset)
					requireHTTP(t, sendFixture(client, url, raw), http.StatusOK)
					if envelope() != before {
						t.Fatal("repeat replaced committed quarantine envelope")
					}
				}
				return
			}
			t.Fatal("missing valid fixture")
		})
	}

	// The independent exact corpus snapshot above precedes these two new MIDs.
	pgCommitFaults(t, admin, reader, client, url)
}

func connectFixture(t *testing.T, role, password string) *pgx.Conn {
	t.Helper()
	dsn, err := fixtureConnString(role)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PG fixture settings")
	}
	if password != "" {
		cfg.Password = password
	}
	conn, err := pgx.ConnectConfig(t.Context(), cfg)
	if err != nil {
		t.Fatalf("could not connect to the disposable fixture as %s", role)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = conn.Close(ctx)
	})
	return conn
}

func nodeFixture(t *testing.T, role, password string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", args...)
	for _, key := range []string{"PATH", "HOME", "TMPDIR"} {
		if v, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+v)
		}
	}
	if role != "" {
		for _, key := range []string{"PGHOST", "PGPORT", "LC_TEST_DATABASE_ALLOWED",
			"LC_W1_PG_CONTAINER", "DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG"} {
			if v, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+v)
			}
		}
		cmd.Env = append(cmd.Env, "PGUSER="+role, "PGPASSWORD="+password)
		if role == "postgres" && password == "" {
			if passfile := os.Getenv("PGPASSFILE"); passfile != "" {
				cmd.Env = append(cmd.Env, "PGPASSFILE="+passfile)
			}
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node %v: %v\n%s", args, err, out)
	}
	t.Log(strings.TrimSpace(string(out)))
}

func requireDenied(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatal("expected insufficient privilege (42501) under a restricted login")
	}
}

type wireReply struct {
	status int
	body   string
	err    error
}

func sendFixture(client *http.Client, url, raw string) wireReply {
	res, err := client.Do(requestFor(url, raw))
	if err != nil {
		return wireReply{err: err}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 513))
	return wireReply{status: res.StatusCode, body: string(body), err: err}
}

func requireHTTP(t *testing.T, reply wireReply, status int) {
	t.Helper()
	if reply.err != nil || reply.status != status || len(reply.body) > 512 ||
		(reply.body == "EVENT_RECEIVED") != (status == http.StatusOK) {
		t.Fatalf("expected HTTP %d, received %+v", status, reply)
	}
}

func pgCommitFaults(t *testing.T, admin, reader *pgx.Conn, client *http.Client, url string) {
	t.Helper()
	// A deferred constraint trigger runs during COMMIT, after admit has supplied a provisional status.
	_, err := admin.Exec(t.Context(), `
		CREATE FUNCTION l01_w1.test_commit_boundary() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.external_key = 'm.w1.commit-failure' THEN
				RAISE EXCEPTION 'fixture commit rejected' USING ERRCODE = 'P1902';
			ELSIF NEW.external_key = 'm.w1.concurrent' THEN
				PERFORM pg_advisory_xact_lock(1901118);
			END IF;
			RETURN NEW;
		END;
		$$;
		CREATE CONSTRAINT TRIGGER test_commit_boundary AFTER INSERT ON l01_w1.inbox
			DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION l01_w1.test_commit_boundary();`)
	if err != nil {
		t.Fatalf("install deferred commit fault: %v", err)
	}
	assertPair := func(t *testing.T, mid, raw string, want int) {
		t.Helper()
		var inbox, intent int
		var digest string
		err := reader.QueryRow(t.Context(), `
			SELECT count(*), count(n.inbox_id), coalesce(min(i.body_sha256), '')
			FROM l01_w1.inbox i LEFT JOIN l01_w1.intent n ON n.inbox_id = i.id
			WHERE i.external_key = $1`, mid).Scan(&inbox, &intent, &digest)
		wantDigest := ""
		if want == 1 {
			wantDigest = sha([]byte(raw))
		}
		if err != nil || inbox != want || intent != want || digest != wantDigest {
			t.Fatalf("reader pair for %s: inbox=%d intent=%d digest=%q err=%v", mid, inbox, intent, digest, err)
		}
	}

	t.Run("failed COMMIT has no success ACK or pair", func(t *testing.T) {
		const mid = "m.w1.commit-failure"
		raw := fixtureBody(neutral.DefaultAuthoritativeAssetID, mid)
		requireHTTP(t, sendFixture(client, url, raw), http.StatusServiceUnavailable)
		assertPair(t, mid, raw, 0)
	})

	t.Run("held COMMIT and concurrent same-key requests", func(t *testing.T) {
		const mid = "m.w1.concurrent"
		raw := fixtureBody(neutral.DefaultAuthoritativeAssetID, mid)
		var locked bool
		if err := admin.QueryRow(t.Context(), "SELECT pg_try_advisory_lock(1901118)").Scan(&locked); err != nil || !locked {
			t.Fatal("could not acquire the fixture commit lock")
		}
		unlock := func(ctx context.Context) error {
			return admin.QueryRow(ctx, "SELECT pg_advisory_unlock(1901118)").Scan(&locked)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = unlock(ctx)
		})

		start := func(raw string) <-chan wireReply {
			reply := make(chan wireReply, 1)
			go func() { reply <- sendFixture(client, url, raw) }()
			return reply
		}
		pending := func(reply <-chan wireReply) {
			select {
			case got := <-reply:
				t.Fatalf("HTTP replied while PostgreSQL was blocked: %+v", got)
			default:
			}
		}
		first := start(raw)
		waitForPG(t, admin, "advisory", 1)
		pending(first)
		assertPair(t, mid, raw, 0)

		replay := start(raw)
		changed := start(strings.Replace(raw, "fixed-body", "changed-body", 1))
		waitForPG(t, admin, "transactionid", 2)
		pending(first)
		pending(replay)
		pending(changed)
		if err := unlock(t.Context()); err != nil || !locked {
			t.Fatal("could not release the fixture commit lock")
		}
		requireHTTP(t, <-first, http.StatusOK)
		assertPair(t, mid, raw, 1)
		requireHTTP(t, <-replay, http.StatusOK)
		requireHTTP(t, <-changed, http.StatusConflict)
		assertPair(t, mid, raw, 1)
	})
}

func waitForPG(t *testing.T, admin *pgx.Conn, waitEvent string, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	defer cancel()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var count int
		err := admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND usename = 'l01_w1_ingress'
			AND application_name = 'l01-w1-go' AND wait_event_type = 'Lock' AND wait_event = $1`, waitEvent).Scan(&count)
		if err == nil && count == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("did not observe %d PostgreSQL %s waiter(s), last count=%d, err=%v", want, waitEvent, count, err)
		case <-tick.C:
		}
	}
}
