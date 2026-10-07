package ads

// Purpose: REAL_PG guards of Amendment W6-06B that the flow test (unbind_pg_test.go) does not cover: every in-flight
//   state refuses (UNKNOWN and ACKNOWLEDGED, not only DISPATCHING), Service.Unbind maps the frozen 0074 disable guard to the
//   frozen binding_in_use refusal and rolls back whole, the success audit row carries its details, the background
//   sweeper skips drafts behind an unbound binding but never a draft that may still spend, and the catalog feed URL
//   honours the ACTIVE-domain and published-store filters. Each test provisions its own store through setupUnbindFx.
// Depends on: unbind_pg_test.go (setupUnbindFx, insertOp, credentialCount), pg_flow_test.go fixture, migration 0160
//   (integration.meta_ads_unbind, ads.catalog_feed_url, the two sweeper candidate lists), 0074 (bindings_ads_disable_guard).
// Used by: `go test ./internal/ads` (TestAdsUnbind*, TestAdsCatalogFeed*); skips unless LC_TEST_DATABASE_ALLOWED=1.
// Status: REAL_PG, no Meta.

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

// unbind runs Service.Unbind for the store's 9002 asset under the full merchant's session.
func (u *unbindFx) unbind(key string) (UnbindResult, error) {
	var out UnbindResult
	err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = u.svc.Unbind(u.f.ctx, tx, s, u.token2, key, UnbindInput{AdAccountID: "9002"})
		return e
	})
	return out, err
}

// newDraft creates a draft on the store's ad binding through the service (identity binding = fb).
func (u *unbindFx) newDraft(t *testing.T, fb, source, key string) string {
	t.Helper()
	now := time.Now().UTC()
	in := DraftInput{AdBindingID: u.adBinding, IdentityBindingID: fb, Template: "BOOST_POST", SourceRef: source, Currency: "TWD",
		LifetimeBudgetMinor: 300000, StartsAt: now.Add(11 * time.Minute), EndsAt: now.Add(49 * time.Hour),
		Countries: []string{"TW"}, AgeMin: 18, AgeMax: 65}
	var raw json.RawMessage
	if err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
		raw, e = u.svc.CreateDraft(u.f.ctx, tx, s, u.token2, key, in)
		return e
	}); err != nil {
		t.Fatalf("create draft: %v", err)
	}
	var created struct{ ID string }
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		t.Fatalf("draft: %v %s", err, raw)
	}
	return created.ID
}

// facebookBinding inserts the identity (Page) binding a draft needs.
func (u *unbindFx) facebookBinding() string {
	fb := newUUID(u.f)
	u.f.must(`INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'facebook','111')`,
		fb, u.f.tenant, u.store, u.principal2orFixture())
	return fb
}

// remoteOp links an operation of the given kind to the draft (attempt 1), optionally pinning remote_id.
func (u *unbindFx) remoteOp(t *testing.T, draft, kind, op, remoteID string) {
	t.Helper()
	u.f.must(`INSERT INTO ads.remote_objects(tenant_id,store_id,draft_id,publish_attempt,kind,seq,operation_id,remote_id)
		VALUES($1,$2,$3,1,$4,1,$5,NULLIF($6,''))`, u.f.tenant, u.store, draft, kind, op, remoteID)
}

// park ends an operation as BLOCKED_POLICY (AD6: an activate in that state does not make the draft count).
func (u *unbindFx) park(op string) {
	u.f.must(`UPDATE integration.operations SET state='BLOCKED_POLICY', generation=generation+1 WHERE id=$1`, op)
}

func TestAdsUnbindGuardsRealPG(t *testing.T) {
	u := setupUnbindFx(t)
	f := u.f
	const receipts = `SELECT count(*)::text FROM ops.command_results WHERE store_id=$1 AND operation='ads.meta.unbind'`
	const audits = `SELECT count(*)::text FROM ops.audit_events WHERE store_id=$1 AND action='ads.account_unbound'`

	// DISPATCHING is covered by the flow test; the other two states that must never be cancelled are covered here
	// (dropping either from the definer's IN-list must turn this red).
	for _, state := range []string{"UNKNOWN", "ACKNOWLEDGED"} {
		t.Run("in-flight "+state+" refuses with the operation listed", func(t *testing.T) {
			op := u.insertOp(t, u.adBinding, "meta.ads.pause", state, "u6-guard-"+state+"-01")
			_, err := u.unbind("u6-guard-key-" + state)
			var refused *Refusal
			if !errors.As(err, &refused) || refused.Code != "operations_in_flight" || refused.Status != 409 {
				t.Fatalf("want 409 operations_in_flight, got %v", err)
			}
			ops, _ := refused.ErrorDetails()["operations"].([]map[string]any)
			if len(ops) != 1 || ops[0]["operation_id"] != op || ops[0]["state"] != state {
				t.Fatalf("details: %v", refused.ErrorDetails())
			}
			if creds, heads := u.credentialCount(u.adBinding); creds != "1" || heads != "1" {
				t.Fatalf("credentials destroyed despite refusal: %s/%s", creds, heads)
			}
			f.must(`DELETE FROM integration.operations WHERE id=$1`, op) // fixture cleanup: the op never dispatched
		})
	}

	fb := u.facebookBinding()
	d1 := u.newDraft(t, fb, "111_222", "u6-guard-draft-001")
	d2 := u.newDraft(t, fb, "111_333", "u6-guard-draft-002")
	// Both drafts are "published" (attempt 1) with a pinned campaign; neither has an activate, so neither counts (AD6).
	for i, d := range []string{d1, d2} {
		f.must(`UPDATE ads.campaign_drafts SET publish_attempt=1 WHERE id=$1`, d)
		campaign := u.insertOp(t, u.adBinding, "meta.ads.create_campaign", "READY", "u6-guard-camp-"+d[:8])
		u.park(campaign)
		u.remoteOp(t, d, "campaign", campaign, "5550001"+string(rune('1'+i)))
	}
	candidates := func(sql string) map[string]bool {
		t.Helper()
		rows, err := f.worker.Query(f.ctx, sql)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		got := map[string]bool{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			got[id] = true
		}
		return got
	}
	advance := func() map[string]bool { return candidates(`SELECT d::text FROM ads.advance_candidates(500,0) d`) }
	insights := func() map[string]bool { return candidates(`SELECT d::text FROM ads.insights_candidates(500) d`) }

	t.Run("a counting draft refuses Service.Unbind with the frozen binding_in_use, whole", func(t *testing.T) {
		if a, i := advance(), insights(); !a[d1] || !a[d2] || !i[d1] || !i[d2] {
			t.Fatalf("enabled binding: both drafts must be sweeper candidates (advance %v insights %v)", a[d1] && a[d2], i[d1] && i[d2])
		}
		activate := u.insertOp(t, u.adBinding, "meta.ads.activate", "READY", "u6-guard-activate-1")
		u.remoteOp(t, d1, "activate", activate, "")
		_, err := u.unbind("u6-guard-key-inuse")
		var refused *Refusal
		if !errors.As(err, &refused) || refused.Code != "binding_in_use" || refused.Status != 409 {
			t.Fatalf("want 409 binding_in_use, got %v", err)
		}
		if creds, heads := u.credentialCount(u.adBinding); creds != "1" || heads != "1" {
			t.Fatalf("credentials destroyed despite binding_in_use: %s/%s", creds, heads)
		}
		if enabled := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, u.adBinding); enabled != "true" {
			t.Fatal("binding disabled despite binding_in_use")
		}
		if n := f.str(receipts, u.store); n != "0" {
			t.Fatalf("refused unbind left a receipt: %s", n)
		}
		if n := f.str(audits, u.store); n != "0" {
			t.Fatalf("refused unbind left an audit row: %s", n)
		}
		u.park(activate) // BLOCKED_POLICY: d1 no longer counts, so the unbind below is allowed
	})

	t.Run("unbind audit row carries the ad account and counts, no secret", func(t *testing.T) {
		out, err := u.unbind("u6-guard-key-ok")
		if err != nil || !out.Unbound || len(out.BindingIDs) != 1 {
			t.Fatalf("unbind: %v %+v", err, out)
		}
		var details struct {
			AdAccountID          string `json:"ad_account_id"`
			Bindings             int    `json:"bindings"`
			CredentialsDestroyed int    `json:"credentials_destroyed"`
		}
		raw := f.str(`SELECT details::text FROM ops.audit_events WHERE store_id=$1 AND action='ads.account_unbound'`, u.store)
		if err = json.Unmarshal([]byte(raw), &details); err != nil || details.AdAccountID != "9002" || details.Bindings != 1 || details.CredentialsDestroyed != 1 {
			t.Fatalf("audit details: %v %s", err, raw)
		}
		var generic map[string]any
		_ = json.Unmarshal([]byte(raw), &generic)
		if len(generic) != 3 {
			t.Fatalf("audit details must hold exactly the three scalar facts: %s", raw)
		}
	})

	t.Run("sweeper skips drafts of an unbound binding but never one that may still spend", func(t *testing.T) {
		if a, i := advance(), insights(); a[d1] || a[d2] || i[d1] || i[d2] {
			t.Fatalf("unbound binding: non-counting drafts must leave both candidate lists (advance %v/%v insights %v/%v)", a[d1], a[d2], i[d1], i[d2])
		}
		// The race the guard cannot see: an activate planned as the unbind committed lands READY on the disabled
		// binding (it will go STALE_BINDING at its claim). The draft counts again, so it stays a candidate and keeps
		// its pause path; the other draft stays skipped.
		activate := u.insertOp(t, u.adBinding, "meta.ads.activate", "READY", "u6-guard-activate-2")
		u.remoteOp(t, d2, "activate", activate, "")
		if a, i := advance(), insights(); !a[d2] || !i[d2] || a[d1] || i[d1] {
			t.Fatalf("counting draft behind a disabled binding must stay a candidate (advance %v/%v insights %v/%v)", a[d2], a[d1], i[d2], i[d1])
		}
	})
}

// TestAdsCatalogFeedFiltersRealPG covers the two filters of ads.catalog_feed_url: only ACTIVE domains, and only while the
// store is published (the feed is served per published store; a draft/unpublished store has no feed URL). Both are
// enforced twice (the definer's predicates AND the 0074 ads_writer_domain_read / ads_writer_publication_read RLS
// policies), so this pins the observable behaviour, not one layer.
func TestAdsCatalogFeedFiltersRealPG(t *testing.T) {
	u := setupUnbindFx(t)
	f := u.f
	feed := func() (feedURL *string, domains []string) {
		t.Helper()
		var raw string
		if err := u.scoped(u.token3, "ads:read", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(f.ctx, `SELECT ads.catalog_feed_url($1,$2)::text`, u.hash3, u.store).Scan(&raw)
		}); err != nil {
			t.Fatalf("catalog_feed_url: %v", err)
		}
		var out struct {
			FeedURL *string `json:"feed_url"`
			Domains []struct {
				Origin string `json:"origin"`
			} `json:"domains"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("feed json: %v %s", err, raw)
		}
		for _, d := range out.Domains {
			domains = append(domains, d.Origin)
		}
		return out.FeedURL, domains
	}
	domain := func(origin, state string) {
		if state == "ACTIVE" {
			f.must(`INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
				VALUES($1,$2,$3,'ACTIVE',now()-interval '2 days',now()-interval '1 day',now()+interval '300 days','platform-subdomain')`, f.tenant, u.store, origin)
			return
		}
		f.must(`INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state) VALUES($1,$2,$3,$4)`, f.tenant, u.store, origin, state)
	}
	domain("https://z-feed-w606b.example.test", "ACTIVE")
	domain("https://a-feed-w606b.example.test", "ACTIVE")
	domain("https://s-feed-w606b.example.test", "SUSPENDED")
	domain("https://d-feed-w606b.example.test", "DETACHED")
	domain("https://t-feed-w606b.example.test", "TLS_PENDING")

	t.Run("no publication row: no feed URL", func(t *testing.T) {
		if url, domains := feed(); url != nil || len(domains) != 0 {
			t.Fatalf("unpublished store listed a feed: %v %v", url, domains)
		}
	})
	f.must(`INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,false)`, f.tenant, u.store)
	t.Run("published=false: no feed URL even with ACTIVE domains", func(t *testing.T) {
		if url, domains := feed(); url != nil || len(domains) != 0 {
			t.Fatalf("unpublished store listed a feed: %v %v", url, domains)
		}
	})
	f.must(`UPDATE control.storefront_publications SET published=true WHERE tenant_id=$1 AND store_id=$2`, f.tenant, u.store)
	t.Run("published: only ACTIVE domains, first in C order is the feed_url", func(t *testing.T) {
		url, domains := feed()
		if len(domains) != 2 || domains[0] != "https://a-feed-w606b.example.test" || domains[1] != "https://z-feed-w606b.example.test" {
			t.Fatalf("domains: %v", domains)
		}
		if url == nil || *url != "https://a-feed-w606b.example.test/feeds/meta.csv" {
			t.Fatalf("feed_url: %v", url)
		}
	})
}
