package foundation_test

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreDomainsNumericConcurrentCreation(t *testing.T) {
	s := sdSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	type result struct {
		handle string
		err    error
	}
	start, results := make(chan struct{}), make(chan result, 50)
	for i := 0; i < 50; i++ {
		id := randomUUID()
		go func() {
			<-start
			var h string
			err := s.b.owner.QueryRow(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'admin','TWD') RETURNING handle`, s.f.tenant, id).Scan(&h)
			results <- result{h, err}
		}()
	}
	close(start)
	seen := map[string]bool{}
	numbers := []int{}
	for i := 0; i < 50; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("concurrent insert: %v", r.err)
			continue
		}
		if !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(r.handle) || seen[r.handle] {
			t.Errorf("invalid or duplicate number %q", r.handle)
		}
		seen[r.handle] = true
		n, _ := strconv.Atoi(r.handle)
		numbers = append(numbers, n)
	}
	if len(seen) != 50 {
		t.Fatalf("got %d unique stores, want 50", len(seen))
	}
	sort.Ints(numbers)
	if numbers[49]-numbers[0] == 49 {
		t.Fatal("store numbers must not be a sequential allocation")
	}
}

func TestStoreDomainsNumericCandidateRetryAndBound(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	for _, exhausted := range []bool{false, true} {
		t.Run(fmt.Sprintf("exhausted=%t", exhausted), func(t *testing.T) {
			// A pinned disposable DB transaction uses PG's real PRNG. No production test hook,
			// function replacement or caller-controlled candidate is introduced.
			conn, release := seededConn(t, s.b.owner, 0.271828)
			defer release()
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			rows, err := tx.Query(ctx, `SELECT (10000000 + floor(random()*90000000))::bigint::text FROM generate_series(1,51)`)
			if err != nil {
				t.Fatal(err)
			}
			var candidates []string
			for rows.Next() {
				var c string
				if err := rows.Scan(&c); err != nil {
					t.Fatal(err)
				}
				candidates = append(candidates, c)
			}
			rows.Close()
			if rows.Err() != nil {
				t.Fatal(rows.Err())
			}
			// The foundation database is shared with every other test in the shard:
			// an earlier test (or an earlier run) may already own one of these PRNG
			// draws. ON CONFLICT turns claiming it into a no-op instead of a 23505;
			// the committed row still blocks the allocator through the same EXISTS
			// check production uses.
			n := 1
			if exhausted {
				n = 50
			}
			for _, c := range candidates[:n] {
				if _, err := tx.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'Occupied number','TWD',$3) ON CONFLICT (handle) WHERE handle IS NOT NULL DO NOTHING`, s.f.tenant, randomUUID(), c); err != nil {
					t.Fatal(err)
				}
			}
			want := candidates[2]
			if !exhausted {
				// If a store already owns candidates[1], the stores-handle check blocks it
				// and an allocator that ignored store_handle_taken would still pass: fail
				// loudly instead of silently losing the platform-origin coverage.
				var owned bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM control.stores WHERE handle=$1)`, candidates[1]).Scan(&owned); err != nil {
					t.Fatal(err)
				}
				if owned {
					t.Fatalf("candidates[1] %q is already a committed store handle; the platform-origin blocker cannot be exercised", candidates[1])
				}
				// A still-serving platform origin also blocks reuse, even without a stores.handle owner.
				// A prior test may already own this origin; then it already blocks the draw.
				if _, err := tx.Exec(ctx, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref) VALUES($1,$2,$3,'ACTIVE',now(),now(),now()+interval '1 day','platform-subdomain') ON CONFLICT (origin) DO NOTHING`, s.f.tenant, s.f.store, "https://"+candidates[1]+".example.com"); err != nil {
					t.Fatal(err)
				}
				// A later draw may also already be owned, so the expectation is the first
				// seeded draw that neither blocker (stores.handle, serving origin) covers:
				// the allocator must return exactly that draw, not an arbitrary free one.
				want = ""
				for _, c := range candidates[2:] {
					var occupied bool
					if err := tx.QueryRow(ctx, `
						SELECT EXISTS(SELECT 1 FROM control.stores WHERE handle=$1)
						    OR EXISTS(SELECT 1 FROM control.storefront_domains d
						              WHERE d.evidence_ref='platform-subdomain' AND d.state <> 'DETACHED'
						                AND position('https://' || $1 || '.' in d.origin) = 1)`, c).Scan(&occupied); err != nil {
						t.Fatal(err)
					}
					if !occupied {
						want = c
						break
					}
				}
				if want == "" {
					t.Fatal("every seeded draw is already occupied; cannot stage the retry assertion")
				}
			}
			if _, err := tx.Exec(ctx, `SELECT setseed(0.271828)`); err != nil {
				t.Fatal(err)
			}
			var got string
			err = tx.QueryRow(ctx, `SELECT control.assign_store_handle('Any name',$1::uuid)`, randomUUID()).Scan(&got)
			if exhausted {
				if pgCode(err) != "PT409" {
					t.Fatalf("50 occupied draws: got %q, %v; want PT409", got, err)
				}
			} else if err != nil || got != want {
				t.Fatalf("must skip occupied store and serving origin: got %q, %v; want first unblocked draw %q", got, err, want)
			}
		})
	}
}

// Force an actual collision, rather than relying on the tiny probability of
// two draws colliding in the 50-store concurrency test. No production test seam.
func TestStoreDomainsNumericConcurrentCandidateCollision(t *testing.T) {
	s := sdSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn1, release1 := seededConn(t, s.b.owner, 0.141421)
	defer release1()
	tx1, err := conn1.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	conn2, release2 := seededConn(t, s.b.owner, 0.141421)
	defer release2()
	tx2, err := conn2.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(ctx)
	var first string
	const insert = `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'Collision','TWD') RETURNING handle`
	if err := tx1.QueryRow(ctx, insert, s.f.tenant, randomUUID()).Scan(&first); err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := tx2.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	type result struct {
		handle string
		err    error
	}
	finished := make(chan result, 1)
	go func() {
		var next string
		err := tx2.QueryRow(ctx, insert, s.f.tenant, randomUUID()).Scan(&next)
		finished <- result{next, err}
	}()
	consumed := false
	defer func() {
		if !consumed {
			// On a failed lock assertion, finish the in-flight query before Rollback.
			cancel()
			<-finished
		}
	}()
	// Prove T2 waits on the candidate advisory lock, not the unique constraint.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err := s.b.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second allocator never waited on the candidate lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	r := <-finished
	consumed = true
	if r.err != nil || r.handle == first || !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(r.handle) {
		t.Fatalf("collision retry = %q, %v; first=%q", r.handle, r.err, first)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// seededConn borrows a connection from pool with its PRNG seeded with seed and returns the
// cleanup the caller must defer. setseed is session state that ROLLBACK does not undo, so the
// connection is hijacked from the pool and closed: it never goes back, and the next borrower
// cannot inherit this deterministic sequence (#147).
func seededConn(t *testing.T, pool *pgxpool.Pool, seed float64) (*pgx.Conn, func()) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw := conn.Hijack()
	if _, err := raw.Exec(ctx, `SELECT setseed($1)`, seed); err != nil {
		raw.Close(ctx)
		t.Fatal(err)
	}
	return raw, func() { raw.Close(ctx) }
}

// TestStoreDomainsNumericSeedDoesNotLeakThroughPool pins a pool to one connection, runs the
// fixture's seeded work and gives the connection straight back. setseed is session state that
// ROLLBACK does not undo, so if the fixture releases the connection the next borrower sees this
// fixture's deterministic stream (draw 52 after the 51 candidate draws) and later commits its
// handles, which is how #147 came back on a rerun. The expectation is computed on a connection
// this test hijacks and closes itself, so the oracle cannot pollute a pool.
func TestStoreDomainsNumericSeedDoesNotLeakThroughPool(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(s.b.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	oracleConn, err := s.b.owner.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oracle := oracleConn.Hijack()
	defer oracle.Close(ctx)
	if _, err := oracle.Exec(ctx, `SELECT setseed(0.271828)`); err != nil {
		t.Fatal(err)
	}
	var seeded []float64
	rows, err := oracle.Query(ctx, `SELECT random() FROM generate_series(1,52)`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		seeded = append(seeded, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 52 {
		t.Fatalf("seeded oracle drew %d values, want 52", len(seeded))
	}
	want := seeded[51]

	conn, release := seededConn(t, pool, 0.271828)
	defer release() // P2-A: a t.Fatal before the explicit release below must not leave the hijacked connection open (a second Close is harmless)
	rows, err = conn.Query(ctx, `SELECT (10000000 + floor(random()*90000000))::bigint::text FROM generate_series(1,51)`)
	if err != nil {
		t.Fatal(err)
	}
	drawn := 0
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		drawn++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if drawn != 51 {
		t.Fatalf("fixture drew %d candidates, want 51", drawn)
	}
	release()

	// MaxConns=1: the borrowed connection is the one the fixture just returned.
	var got float64
	if err := pool.QueryRow(ctx, `SELECT random()`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == want {
		t.Fatalf("setseed leaked through the pool: the next borrower of the fixture's connection drew %v, the seeded continuation (draw 52); a seeded connection must be closed, not returned to the pool", got)
	}
}
