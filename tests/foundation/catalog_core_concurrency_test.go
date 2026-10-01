package foundation_test

// catalog_core_concurrency_test.go: CC11b of unit catalog-core (contracts/storefront-v2.md section A: "Optimistic expected_version
// everywhere", acceptance CC11 "every command replays its first answer"). Several requests race on the real handler and real PG:
// the same key and bytes must collapse to ONE write answered identically, and two writers holding the same version must not both win.

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestCatalogCoreCC11bConcurrency(t *testing.T) {
	e := ccNew(t)
	race := func(n int, call func(i int) (int, []byte)) (codes []int, bodies [][]byte) {
		t.Helper()
		codes, bodies = make([]int, n), make([][]byte, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				codes[i], bodies[i] = call(i)
			}()
		}
		close(start)
		wg.Wait()
		return codes, bodies
	}

	// the same key + bytes, 8 racers: every answer is 200 and identical, exactly one product exists
	key := e.key("race-create")
	name := e.name("raced product")
	codes, bodies := race(8, func(int) (int, []byte) {
		return e.a.call("POST", "/products", key, map[string]any{"name": name})
	})
	for i := range codes {
		if codes[i] != 200 || !bytes.Equal(bodies[i], bodies[0]) {
			t.Fatalf("racer %d: %d %s (first: %s)", i, codes[i], bodies[i], bodies[0])
		}
	}
	if n := countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE name=$1`, name); n != 1 {
		t.Fatalf("%d products created by one idempotent request", n)
	}

	// two writers with the same expected_version and DIFFERENT keys: exactly one wins, the other is 409
	var p ccProduct
	e.a.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("versioned")}, &p)
	codes, _ = race(6, func(i int) (int, []byte) {
		return e.a.call("PATCH", "/products/"+p.ID, e.key("race-patch"), map[string]any{"expected_version": p.Version, "seo_title": strings.Repeat("x", i+1)})
	})
	wins, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case 200:
			wins++
		case 409:
			conflicts++
		default:
			t.Fatalf("unexpected status %d in %v", c, codes)
		}
	}
	if wins != 1 || conflicts != 5 {
		t.Fatalf("same expected_version: %d winners and %d conflicts, want 1 and 5 (%v)", wins, conflicts, codes)
	}
	var cur ccProduct
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Version != p.Version+1 {
		t.Fatalf("one winner must bump the version once: %d -> %d", p.Version, cur.Version)
	}

	// the same on a collection's membership (the complete ordered list replaces under the row lock)
	a, _ := e.simple("race member a", 100, 1, nil)
	b, _ := e.simple("race member b", 100, 1, nil)
	col := e.collection("Raced", nil)
	codes, _ = race(6, func(i int) (int, []byte) {
		ids := []string{a.ID, b.ID}
		if i%2 == 1 {
			ids = []string{b.ID, a.ID}
		}
		return e.a.call("PUT", "/collections/"+col.ID+"/products", e.key("race-members"), map[string]any{"expected_version": col.Version, "product_ids": ids})
	})
	wins = 0
	for _, c := range codes {
		if c == 200 {
			wins++
		} else if c != 409 {
			t.Fatalf("unexpected status %d in %v", c, codes)
		}
	}
	if wins != 1 {
		t.Fatalf("collection membership race: %d winners (%v), want 1", wins, codes)
	}
	var got ccCollection
	e.a.ok("GET", "/collections/"+col.ID, "", nil, &got)
	if len(got.Products) != 2 || got.Products[0].Position == got.Products[1].Position {
		t.Fatalf("membership after the race must be one complete ordered list: %+v", got.Products)
	}
}
