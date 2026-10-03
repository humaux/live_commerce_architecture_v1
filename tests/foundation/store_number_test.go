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
			tx, err := s.b.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			// A pinned disposable DB transaction uses PG's real PRNG. No production test hook,
			// function replacement or caller-controlled candidate is introduced.
			if _, err := tx.Exec(ctx, `SELECT setseed(0.271828)`); err != nil {
				t.Fatal(err)
			}
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
			n := 1
			if exhausted {
				n = 50
			}
			for _, c := range candidates[:n] {
				if _, err := tx.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'Occupied number','TWD',$3)`, s.f.tenant, randomUUID(), c); err != nil {
					t.Fatal(err)
				}
			}
			if !exhausted {
				// A still-serving platform origin also blocks reuse, even without a stores.handle owner.
				if _, err := tx.Exec(ctx, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref) VALUES($1,$2,$3,'ACTIVE',now(),now(),now()+interval '1 day','platform-subdomain')`, s.f.tenant, s.f.store, "https://"+candidates[1]+".example.com"); err != nil {
					t.Fatal(err)
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
			} else if err != nil || got != candidates[2] {
				t.Fatalf("must skip occupied store and serving origin: got %q, %v; want third draw %q", got, err, candidates[2])
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
	tx1, err := s.b.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	tx2, err := s.b.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(ctx)
	for _, tx := range []pgx.Tx{tx1, tx2} {
		if _, err := tx.Exec(ctx, `SELECT setseed(0.141421)`); err != nil {
			t.Fatal(err)
		}
	}
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
