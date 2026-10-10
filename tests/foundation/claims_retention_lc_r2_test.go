// Purpose: LC-R2 C3x purge gate — live.comment_prints, the live.comment.print receipts in ops.command_results
// and inbox.bundle_peers age out at intake_days; a C2 de-identify drops the bundle's peer links; an RD4 erasure
// deletes the peer's links and counts them, on EVERY erasure path — including the three that carry no peer keys
// (comment-ref selector, bundle selector, restore replay), which derive the erased actor's links from their
// bundles (contracts/live-console-v1.md §7.4/§10 "C3 (extended)"; contracts/claims-retention-purge-v1.md §1 C3x;
// migration 0169).
// Depends on: REAL_PG migrations and the crSetup retention harness (claims_retention_test.go).
// Used by: CRP/LC gates (scripts/dev/test-focused.sh '^TestClaimsRetention|LCN05').
//
// RED-first (PROCESS §2.4): on trunk before 0169, claims.run_retention (0127) contains no DELETE for
// live.comment_prints, ops.command_results or inbox.bundle_peers, so the aged rows below survive every run and
// every purge/count assertion fails with the rows still present — that failure is this unit's red evidence. The
// committed assertions state the post-0169 (green) behaviour. Round 2 (PR #35 review thread 4236426681): with
// 0169 as committed in round 1, apply_actor_erasure deleted bundle_peers only for caller-supplied p_peer_keys,
// so the three peer-key-less erasure paths below failed with the actor's links still present
// (r2-red-erasure-paths.log); the fix derives them from the actor's bundles inside the function.
// Evidence label: REAL_PG (PG 18 via scripts/dev/test-focused.sh); no network, no LIVE, no production DSN, no
// real buyer data — every comment ref, idempotency key, actor key, peer key and receipt body is a synthetic sentinel.
//
// Conventions of this file (as in claims_retention_test.go): aged timestamps are crOld/crNew around the
// threshold (crMargin); the retention definers are only reached through the job/operator logins; every seed is
// owner SQL and removed by the scenario (its defer runs before the harness' w.cleanup, so the FK order
// comment_prints → sessions holds); the policy row is restored by crSetup.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"livecommerce/internal/retention"
)

func TestClaimsRetentionLCR2C3xPrintPurge(t *testing.T) {
	e := crSetup(t)
	ctx := context.Background()
	w := e.w

	// ---------------------------------------------------------------------------
	// Seed helpers (owner-seeded; production writes these rows through the A3 print route / command.Run and
	// inbox.finish_send). ageExpr is embedded SQL text from crOld/crNew/clock_timestamp (integers only).
	// ---------------------------------------------------------------------------
	seedPrint := func(t *testing.T, tenant, store, session, ref, prin, ageExpr string) {
		t.Helper()
		mustExec(t, w.owner, `INSERT INTO live.comment_prints(tenant_id,store_id,session_id,comment_ref,print_count,first_printed_at,last_printed_at,last_principal_id)
			VALUES($1,$2,$3,$4,3,`+ageExpr+`,`+ageExpr+`,$5)`, tenant, store, session, ref, prin)
	}
	countPrint := func(t *testing.T, tenant, store, session, ref string) int {
		t.Helper()
		return crCount(t, w.owner, `SELECT count(*) FROM live.comment_prints WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND comment_ref=$4`, tenant, store, session, ref)
	}
	seedReceipt := func(t *testing.T, tenant, store, operation, key, prin, ageExpr, response string) {
		t.Helper()
		hash := sha256.Sum256([]byte("lcr2|" + operation + "|" + key)) // synthetic request hash, never a real request
		mustExec(t, w.owner, `INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id,created_at)
			VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,`+ageExpr+`)`, tenant, store, operation, key, hash[:], response, prin)
	}
	countReceipt := func(t *testing.T, tenant, store, operation, key string) int {
		t.Helper()
		return crCount(t, w.owner, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation=$3 AND idempotency_key=$4`, tenant, store, operation, key)
	}
	seedPeer := func(t *testing.T, tenant, store, bundle, peer, ageExpr string) {
		t.Helper()
		mustExec(t, w.owner, `INSERT INTO inbox.bundle_peers(tenant_id,store_id,bundle_id,peer_key,app_id,object,asset_id,operation_id,created_at)
			VALUES($1,$2,$3,$4,$5,'page',$6,$7,`+ageExpr+`)`, tenant, store, bundle, peer, miApp, miAsset(), randomUUID())
	}
	countPeer := func(t *testing.T, bundle, peer string) int {
		t.Helper()
		return crCount(t, w.owner, `SELECT count(*) FROM inbox.bundle_peers WHERE bundle_id=$1 AND peer_key=$2`, bundle, peer)
	}
	dbNow := func(t *testing.T) time.Time {
		t.Helper()
		var now time.Time
		if err := w.owner.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			t.Fatal(err)
		}
		return now
	}
	// checkLog asserts a `run` log row holds numbers only (RD6) and the wanted C3x values.
	checkLog := func(t *testing.T, row crLogRow, want map[string]float64) {
		t.Helper()
		if row.kind != "run" {
			t.Fatalf("log row kind=%q, want run", row.kind)
		}
		for k, v := range row.counts {
			if _, isNum := v.(float64); !isNum {
				t.Errorf("run row counts[%s]=%v (%T), want a number", k, v, v)
			}
		}
		for k, v := range want {
			if row.counts[k] != v {
				t.Errorf("run row counts[%s]=%v, want %v", k, row.counts[k], v)
			}
		}
	}
	printResponse := `{"print_count":3,"last_printed_at":"2026-01-01T00:00:00Z"}` // synthetic A3 receipt body shape

	// C3x at intake_days: aged print facts + receipts + peer links are DELETEd and counted; younger rows, other
	// stores/tenants' young rows and receipts of other operations survive; the p_limit batch cap and more=1 cover
	// the new counters; the run row carries the new keys as numbers.
	e.sub(t, "c3x-age-enforced", func(t *testing.T) {
		sA := w.session(t, w.store)
		sA2 := w.session(t, w.store2)
		// Tenant-B world (owner-seeded; crWorld is tenant-A only): one synthetic session on the fixture's tenant-B
		// store with a tenant-B principal (FK identity.memberships).
		var prinB string
		if err := w.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.memberships WHERE tenant_id=$1 LIMIT 1`, e.f.tenantB).Scan(&prinB); err != nil {
			t.Fatalf("tenant B principal: %v", err)
		}
		sB := randomUUID()
		mustExec(t, w.owner, `INSERT INTO live.sessions(id,tenant_id,store_id,principal_id,title) VALUES($1,$2,$3,$4,$5)`,
			sB, e.f.tenantB, e.f.storeB, prinB, "lcr2 gate "+t04Tag())
		bX := w.bundle(t, sA, "manual", "", "lcr2-bp-"+t04Tag())
		refOldA, refNewA := miAsset()+"_"+crDigits(10), miAsset()+"_"+crDigits(10)
		refOldA2, refNewA2 := miAsset()+"_"+crDigits(10), miAsset()+"_"+crDigits(10)
		refOldB, refNewB := miAsset()+"_"+crDigits(10), miAsset()+"_"+crDigits(10)
		kOldA, kNewA := t04Key("lcr2a-old"), t04Key("lcr2a-new")
		kOldA2, kNewA2 := t04Key("lcr2a2-old"), t04Key("lcr2a2-new")
		kOldB, kNewB := t04Key("lcr2b-old"), t04Key("lcr2b-new")
		kOther := t04Key("lcr2a-draft") // an aged receipt of ANOTHER operation: never read, never deleted
		peerOld, peerNew := crHex64(), crHex64()
		defer func() { // runs before the harness' w.cleanup (FK: comment_prints → sessions); sB is not w's
			mustExec(t, w.owner, `DELETE FROM live.comment_prints WHERE session_id=ANY($1::uuid[])`, []string{sA.id, sA2.id, sB})
			mustExec(t, w.owner, `DELETE FROM ops.command_results WHERE operation IN ('live.comment.print','live.draft.create') AND idempotency_key=ANY($1::text[])`,
				[]string{kOldA, kNewA, kOldA2, kNewA2, kOldB, kNewB, kOther})
			mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=ANY($1::text[])`, []string{peerOld, peerNew})
			mustExec(t, w.owner, `DELETE FROM live.sessions WHERE id=$1`, sB)
		}()
		seedPrint(t, w.tenant, w.store, sA.id, refOldA, w.prin, crOld(30))
		seedPrint(t, w.tenant, w.store, sA.id, refNewA, w.prin, crNew(30))
		seedPrint(t, w.tenant, w.store2, sA2.id, refOldA2, w.prin, crOld(30))
		seedPrint(t, w.tenant, w.store2, sA2.id, refNewA2, w.prin, crNew(30))
		seedPrint(t, e.f.tenantB, e.f.storeB, sB, refOldB, prinB, crOld(30))
		seedPrint(t, e.f.tenantB, e.f.storeB, sB, refNewB, prinB, crNew(30))
		seedReceipt(t, w.tenant, w.store, "live.comment.print", kOldA, w.prin, crOld(30), printResponse)
		seedReceipt(t, w.tenant, w.store, "live.comment.print", kNewA, w.prin, crNew(30), printResponse)
		seedReceipt(t, w.tenant, w.store2, "live.comment.print", kOldA2, w.prin, crOld(30), printResponse)
		seedReceipt(t, w.tenant, w.store2, "live.comment.print", kNewA2, w.prin, crNew(30), printResponse)
		seedReceipt(t, e.f.tenantB, e.f.storeB, "live.comment.print", kOldB, prinB, crOld(30), printResponse)
		seedReceipt(t, e.f.tenantB, e.f.storeB, "live.comment.print", kNewB, prinB, crNew(30), printResponse)
		seedReceipt(t, w.tenant, w.store, "live.draft.create", kOther, w.prin, crOld(30), `{"v":1}`)
		seedPeer(t, w.tenant, w.store, bX.id, peerOld, crOld(30))
		seedPeer(t, w.tenant, w.store, bX.id, peerNew, crNew(30))

		mark := dbNow(t)
		before := crDigests(t, w.owner)
		e.markSeed()
		c := e.run(500)
		crWant(t, "C3x enforced", c, map[string]int64{"enforced": 1, "links": 0, "bundles": 0, "intake": 0, "operations": 0,
			"comment_events": 0, "messages": 0, "conversations": 0, "prints": 3, "print_receipts": 3, "bundle_peers": 1, "more": 0})
		crSameDigests(t, "C3x enforced (C1-C6 tables)", before, crDigests(t, w.owner))
		// The trunk-red core: on a database without 0169 every aged row is still here and every young row check
		// below passes vacuously; these assertions are the unit's red evidence.
		if countPrint(t, w.tenant, w.store, sA.id, refOldA) != 0 {
			t.Error("the aged print fact survives (live-console-v1 §10 C3-extended)")
		}
		if countReceipt(t, w.tenant, w.store, "live.comment.print", kOldA) != 0 {
			t.Error("the aged live.comment.print receipt survives (it must be purged with the fact)")
		}
		if countPrint(t, w.tenant, w.store2, sA2.id, refOldA2) != 0 || countPrint(t, e.f.tenantB, e.f.storeB, sB, refOldB) != 0 {
			t.Error("an aged print fact of another store/tenant survives (the class is platform-level like C1-C6)")
		}
		if countReceipt(t, w.tenant, w.store2, "live.comment.print", kOldA2) != 0 || countReceipt(t, e.f.tenantB, e.f.storeB, "live.comment.print", kOldB) != 0 {
			t.Error("an aged live.comment.print receipt of another store/tenant survives")
		}
		if countPeer(t, bX.id, peerOld) != 0 {
			t.Error("the aged inbox.bundle_peers link survives")
		}
		// Younger rows survive — in every store/tenant.
		if countPrint(t, w.tenant, w.store, sA.id, refNewA) != 1 || countPrint(t, w.tenant, w.store2, sA2.id, refNewA2) != 1 ||
			countPrint(t, e.f.tenantB, e.f.storeB, sB, refNewB) != 1 {
			t.Error("a print fact inside intake_days was deleted")
		}
		if countReceipt(t, w.tenant, w.store, "live.comment.print", kNewA) != 1 || countReceipt(t, w.tenant, w.store2, "live.comment.print", kNewA2) != 1 ||
			countReceipt(t, e.f.tenantB, e.f.storeB, "live.comment.print", kNewB) != 1 {
			t.Error("a live.comment.print receipt inside intake_days was deleted")
		}
		if countPeer(t, bX.id, peerNew) != 1 {
			t.Error("a peer link inside intake_days was deleted")
		}
		// Receipts of other operations are untouched even when aged (the shared ledger holds every operation).
		if countReceipt(t, w.tenant, w.store, "live.draft.create", kOther) != 1 {
			t.Error("an aged receipt of another operation was deleted (C3x must stay scoped to live.comment.print)")
		}
		// A second run finds nothing.
		again := e.run(500)
		crWant(t, "C3x second run", again, map[string]int64{"enforced": 1, "prints": 0, "print_receipts": 0, "bundle_peers": 0, "more": 0})
		rows := crLogSince(t, w.owner, mark)
		if len(rows) != 2 || rows[0].kind != "run" || rows[1].kind != "run" {
			t.Fatalf("C3x runs wrote %d log rows %+v, want two `run` rows", len(rows), rows)
		}
		checkLog(t, rows[0], map[string]float64{"enforced": 1, "prints": 3, "print_receipts": 3, "bundle_peers": 1, "more": 0})
		checkLog(t, rows[1], map[string]float64{"enforced": 1, "prints": 0, "print_receipts": 0, "bundle_peers": 0, "more": 0})
		// The batch cap: two aged facts with p_limit=1 delete one, and the enforced more=1 covers the new counters.
		refCap1, refCap2 := miAsset()+"_"+crDigits(10), miAsset()+"_"+crDigits(10)
		seedPrint(t, w.tenant, w.store, sA.id, refCap1, w.prin, crOld(30))
		seedPrint(t, w.tenant, w.store, sA.id, refCap2, w.prin, crOld(30))
		e.markSeed()
		capped := e.run(1)
		crWant(t, "C3x capped", capped, map[string]int64{"enforced": 1, "prints": 1, "more": 1})
		if countPrint(t, w.tenant, w.store, sA.id, refCap1)+countPrint(t, w.tenant, w.store, sA.id, refCap2) != 1 {
			t.Error("p_limit=1 did not leave exactly one aged print fact")
		}
		final := e.run(500)
		crWant(t, "C3x final", final, map[string]int64{"enforced": 1, "prints": 1, "more": 0})
	})

	// Report-only (enforced=false): the C3x classes COUNT their eligible rows (capped at p_limit) and delete
	// nothing — including the operation scoping of the receipt count.
	e.sub(t, "c3x-report-only", func(t *testing.T) {
		s := w.session(t, w.store)
		b := w.bundle(t, s, "manual", "", "lcr2-ro-"+t04Tag())
		ref := miAsset() + "_" + crDigits(10)
		key, keyOther := t04Key("lcr2ro-a"), t04Key("lcr2ro-x")
		peer := crHex64()
		defer func() {
			mustExec(t, w.owner, `DELETE FROM live.comment_prints WHERE session_id=$1`, s.id)
			mustExec(t, w.owner, `DELETE FROM ops.command_results WHERE operation IN ('live.comment.print','live.draft.create') AND idempotency_key=ANY($1::text[])`, []string{key, keyOther})
			mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=$1`, peer)
		}()
		seedPrint(t, w.tenant, w.store, s.id, ref, w.prin, crOld(30))
		seedReceipt(t, w.tenant, w.store, "live.comment.print", key, w.prin, crOld(30), printResponse)
		seedReceipt(t, w.tenant, w.store, "live.draft.create", keyOther, w.prin, crOld(30), `{"v":1}`)
		seedPeer(t, w.tenant, w.store, b.id, peer, crOld(30))
		e.policy(false, 7, 30, 90, 30)
		c3xTables := []string{"live.comment_prints", "ops.command_results", "inbox.bundle_peers"}
		before := crDigests(t, w.owner, c3xTables...)
		mark := dbNow(t)
		e.markSeed()
		c := e.run(500)
		crWant(t, "C3x report-only", c, map[string]int64{"enforced": 0, "prints": 1, "print_receipts": 1, "bundle_peers": 1, "more": 0})
		crSameDigests(t, "C3x report-only", before, crDigests(t, w.owner, c3xTables...))
		if countPrint(t, w.tenant, w.store, s.id, ref) != 1 || countReceipt(t, w.tenant, w.store, "live.comment.print", key) != 1 ||
			countReceipt(t, w.tenant, w.store, "live.draft.create", keyOther) != 1 || countPeer(t, b.id, peer) != 1 {
			t.Error("report-only deleted a row (it must only count)")
		}
		rows := crLogSince(t, w.owner, mark)
		if len(rows) != 1 {
			t.Fatalf("report-only run wrote %d log rows %+v, want exactly one", len(rows), rows)
		}
		checkLog(t, rows[0], map[string]float64{"enforced": 0, "prints": 1, "print_receipts": 1, "bundle_peers": 1, "more": 0})
	})

	// C2 hook: de-identifying a bundle deletes its inbox.bundle_peers rows with its links — uncounted (C2 is a
	// de-identify, not a purge class; the age rule counts the intake_days purge separately).
	e.sub(t, "c2-deidentify-drops-peer-links", func(t *testing.T) {
		s := w.session(t, w.store)
		w.closedAt(t, s, crOld(90))
		b := w.bundle(t, s, "manual", "", "lcr2-c2-"+t04Tag())
		owner := w.bind(t, s, b)
		peer := crHex64()
		defer func() { mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=$1`, peer) }()
		seedPeer(t, w.tenant, w.store, b.id, peer, `(clock_timestamp())`) // young: only the C2 hook may remove it
		mark := dbNow(t)
		e.markSeed()
		c := e.run(500)
		crWant(t, "C2 peer-link hook", c, map[string]int64{"enforced": 1, "bundles": 1, "prints": 0, "print_receipts": 0, "bundle_peers": 0, "more": 0})
		if countPeer(t, b.id, peer) != 0 {
			t.Error("the de-identified bundle's peer link survives (live-console-v1 §10 C3-extended: bundle_peers also on C2)")
		}
		r := e.bundleRow(t, b.id)
		if r.purged == nil || r.owner != nil || r.bound != nil {
			t.Errorf("bundle after the C2 run: purged=%v owner=%v bound=%v, want de-identified", r.purged, r.owner, r.bound)
		}
		_ = owner
		rows := crLogSince(t, w.owner, mark)
		if len(rows) != 1 {
			t.Fatalf("C2 run wrote %d log rows %+v, want exactly one", len(rows), rows)
		}
		checkLog(t, rows[0], map[string]float64{"enforced": 1, "bundles": 1, "bundle_peers": 0})
	})

	// RD4 hook: an actor erasure deletes the supplied peer's inbox.bundle_peers rows in every store and COUNTS
	// them; another peer's link and bundle stay byte-identical; the actor_erased row and the request-id replay
	// carry the counted key (internal/retention's closed count set includes bundle_peers).
	e.sub(t, "rd4-erasure-deletes-peer-links", func(t *testing.T) {
		s1 := w.session(t, w.store)
		w.closedAt(t, s1, crAgo(10, 0))
		s2 := w.session(t, w.store2)
		w.closedAt(t, s2, crAgo(10, 0))
		actor, peer, otherPeer := crHex64(), crHex64(), crHex64()
		b1 := w.bundle(t, s1, "facebook", actor, "")
		b2 := w.bundle(t, s2, "facebook", actor, "")
		b3 := w.bundle(t, s1, "facebook", crHex64(), "") // another actor's bundle with another peer
		defer func() {
			mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=ANY($1::text[])`, []string{peer, otherPeer})
		}()
		seedPeer(t, w.tenant, w.store, b1.id, peer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store2, b2.id, peer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store, b3.id, otherPeer, `(clock_timestamp())`)
		otherBefore := crDigest(t, w.owner, "claims.bundles", "id=$1", b3.id)
		req := randomUUID()
		sel := retention.Selector{Request: req, Object: "page", Asset: miAsset(), ActorKey: actor, PeerKeys: []string{peer}}
		c, held, err := retention.Erase(ctx, e.op, sel)
		if err != nil || held.IsHeld() {
			t.Fatalf("Erase(RD4 peer links): counts=%v held=%v err=%v", c, held.IsHeld(), err)
		}
		crWant(t, "RD4 erasure", c, map[string]int64{"bundles": 2, "lines": 0, "links": 0, "intake": 0, "comment_events": 0,
			"operations": 0, "messages": 0, "conversations": 0, "bundle_peers": 2})
		if countPeer(t, b1.id, peer) != 0 || countPeer(t, b2.id, peer) != 0 {
			t.Error("the erased actor's peer links survive (live-console-v1 §10 RD4)")
		}
		if countPeer(t, b3.id, otherPeer) != 1 {
			t.Error("another peer's link was deleted (the hook is peer_key-scoped)")
		}
		if crDigest(t, w.owner, "claims.bundles", "id=$1", b3.id) != otherBefore {
			t.Error("another actor's bundle changed")
		}
		row, ok := e.erasedRow(req)
		if !ok {
			t.Fatal("no actor_erased log row")
		}
		for k, v := range row.counts {
			if _, isNum := v.(float64); !isNum {
				t.Errorf("log counts[%s]=%v is not a number", k, v)
			}
		}
		if row.counts["bundle_peers"] != float64(2) {
			t.Errorf("actor_erased counts %v lack bundle_peers=2", row.counts)
		}
		// Replay of the same request: the stored counts (with bundle_peers) come back through internal/retention's
		// closed count set — a key missing from allowedCounts would silently drop here.
		c2, held2, err := retention.Erase(ctx, e.op, sel)
		if err != nil || held2.IsHeld() || c2["replayed"] != 1 {
			t.Fatalf("replay: %v held=%v err=%v", c2, held2.IsHeld(), err)
		}
		for k, v := range c {
			if c2[k] != v {
				t.Errorf("replay counts[%s]=%d, want the stored %d", k, c2[k], v)
			}
		}
	})

	// A counted peer-link delete keeps a peer-link-only erasure away from PT404 (the 0154 blocked_actors
	// rationale): the actor has no bundle/intake/social row, only the peer link names them.
	e.sub(t, "rd4-peer-link-only-not-pt404", func(t *testing.T) {
		s := w.session(t, w.store)
		w.closedAt(t, s, crAgo(10, 0))
		otherActor, peer := crHex64(), crHex64()
		b := w.bundle(t, s, "facebook", otherActor, "")
		defer func() { mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=$1`, peer) }()
		seedPeer(t, w.tenant, w.store, b.id, peer, `(clock_timestamp())`)
		before := crDigest(t, w.owner, "claims.bundles", "id=$1", b.id)
		c, held, err := retention.Erase(ctx, e.op, retention.Selector{Request: randomUUID(), Object: "page", Asset: miAsset(),
			ActorKey: crHex64(), PeerKeys: []string{peer}})
		if err != nil || held.IsHeld() {
			t.Fatalf("peer-link-only erasure: counts=%v held=%v err=%v (a counted bundle_peers delete must not PT404)", c, held.IsHeld(), err)
		}
		crWant(t, "peer-link-only erasure", c, map[string]int64{"bundles": 0, "bundle_peers": 1})
		if countPeer(t, b.id, peer) != 0 {
			t.Error("the peer link survives")
		}
		if crDigest(t, w.owner, "claims.bundles", "id=$1", b.id) != before {
			t.Error("the bundle that referenced the peer link changed")
		}
	})

	// ---------------------------------------------------------------------------
	// Round 2 (PR #35 review thread 4236426681): the round-1 hook deleted bundle_peers only for caller-supplied
	// p_peer_keys, but two of the supported erasure paths never carry peer keys — internal/retention's
	// validSelector rejects them for the comment-ref and bundle selectors, and replay_actor_erasures calls
	// apply_actor_erasure(..., NULL) — so the erased actor's own links survived until the age purge. The fix
	// derives them from the actor's bundles inside the function (the same set the links delete covers, same
	// tenant/store scoping) and counts both deletes into bundle_peers. Each subtest below is red on round-1 0169
	// (the links survive) and green with the fix; other actors' and stores' links stay byte-identical throughout.
	// ---------------------------------------------------------------------------

	// Selector (b): the comment ref resolves the actor through claims.meta_intake; no peer keys exist on this
	// path, so only the bundle-derived delete can remove the actor's links — in BOTH stores, counted.
	e.sub(t, "rd4-comment-ref-erasure-derives-peer-links", func(t *testing.T) {
		s1 := w.session(t, w.store)
		w.closedAt(t, s1, crAgo(10, 0))
		s2 := w.session(t, w.store2)
		w.closedAt(t, s2, crAgo(10, 0))
		asset := miAsset()
		m1 := w.sourceOn(t, s1, "page", asset)
		actor, peer := crHex64(), crHex64()
		otherPeer, otherPeer2 := crHex64(), crHex64()
		ref := asset + "_" + crDigits(10)
		b1 := w.bundle(t, s1, "facebook", actor, "")
		b2 := w.bundle(t, s2, "facebook", actor, "")
		b3 := w.bundle(t, s1, "facebook", crHex64(), "") // another actor, same store
		b4 := w.bundle(t, s2, "facebook", crHex64(), "") // another actor, other store
		defer func() {
			mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=ANY($1::text[])`, []string{peer, otherPeer, otherPeer2})
		}()
		// the selector resolves through this intake row; APPLIED and 20 days old: terminal and past the RD5 8-day hold
		w.intake(t, m1, ref, actor, "APPLIED", crAgo(20, 0), "")
		seedPeer(t, w.tenant, w.store, b1.id, peer, `(clock_timestamp())`)  // young: only an erasure hook may remove it
		seedPeer(t, w.tenant, w.store2, b2.id, peer, `(clock_timestamp())`) // the same person's key, other store
		seedPeer(t, w.tenant, w.store, b3.id, otherPeer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store2, b4.id, otherPeer2, `(clock_timestamp())`)
		othersBefore := crDigest(t, w.owner, "inbox.bundle_peers", "peer_key=ANY($1::text[])", []string{otherPeer, otherPeer2}) +
			crDigest(t, w.owner, "claims.bundles", "id=ANY($1::uuid[])", []string{b3.id, b4.id})
		req := randomUUID()
		c, held, err := retention.Erase(ctx, e.op, retention.Selector{Request: req, Object: "page", Asset: asset, CommentRef: ref})
		if err != nil || held.IsHeld() {
			t.Fatalf("Erase(b): counts=%v held=%v err=%v", c, held.IsHeld(), err)
		}
		crWant(t, "comment-ref erasure", c, map[string]int64{"bundles": 2, "lines": 0, "links": 0, "intake": 1,
			"comment_events": 0, "operations": 0, "messages": 0, "conversations": 0, "bundle_peers": 2})
		if countPeer(t, b1.id, peer) != 0 || countPeer(t, b2.id, peer) != 0 {
			t.Error("the erased actor's peer links survive a comment-ref erasure (this path passes p_peer_keys=NULL)")
		}
		if countPeer(t, b3.id, otherPeer) != 1 || countPeer(t, b4.id, otherPeer2) != 1 {
			t.Error("another actor's/store's peer link was deleted")
		}
		if crDigest(t, w.owner, "inbox.bundle_peers", "peer_key=ANY($1::text[])", []string{otherPeer, otherPeer2})+
			crDigest(t, w.owner, "claims.bundles", "id=ANY($1::uuid[])", []string{b3.id, b4.id}) != othersBefore {
			t.Error("another actor's peer link or bundle changed (the derived delete is scoped to the erased actor's bundles)")
		}
		row, ok := e.erasedRow(req)
		if !ok {
			t.Fatal("no actor_erased log row")
		}
		for k, v := range row.counts {
			if _, isNum := v.(float64); !isNum {
				t.Errorf("log counts[%s]=%v is not a number", k, v)
			}
		}
		if row.counts["bundle_peers"] != float64(2) {
			t.Errorf("actor_erased counts %v lack bundle_peers=2", row.counts)
		}
	})

	// Selector (c) on a manual bundle: single-bundle mode never enters the p_peer_keys block at all, yet the
	// bundle's peer link must go with its de-identification, counted; a sibling bundle's link in the same
	// session/store and another store's link stay byte-identical.
	e.sub(t, "rd4-bundle-selector-erasure-derives-peer-links", func(t *testing.T) {
		s := w.session(t, w.store)
		w.closedAt(t, s, crAgo(5, 0))
		s2 := w.session(t, w.store2)
		w.closedAt(t, s2, crAgo(5, 0))
		target := w.bundle(t, s, "manual", "", "lcr2-r2t-"+t04Tag())
		sibling := w.bundle(t, s, "manual", "", "lcr2-r2s-"+t04Tag())
		far := w.bundle(t, s2, "manual", "", "lcr2-r2f-"+t04Tag())
		peer, sibPeer, farPeer := crHex64(), crHex64(), crHex64()
		defer func() {
			mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=ANY($1::text[])`, []string{peer, sibPeer, farPeer})
		}()
		seedPeer(t, w.tenant, w.store, target.id, peer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store, sibling.id, sibPeer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store2, far.id, farPeer, `(clock_timestamp())`)
		othersBefore := crDigest(t, w.owner, "inbox.bundle_peers", "peer_key=ANY($1::text[])", []string{sibPeer, farPeer}) +
			crDigest(t, w.owner, "claims.bundles", "id=ANY($1::uuid[])", []string{sibling.id, far.id})
		c, held, err := retention.Erase(ctx, e.op, retention.Selector{Request: randomUUID(), Tenant: w.tenant, Store: w.store, Bundle: target.id})
		if err != nil || held.IsHeld() {
			t.Fatalf("Erase(c manual): counts=%v held=%v err=%v", c, held.IsHeld(), err)
		}
		crWant(t, "bundle-selector erasure", c, map[string]int64{"bundles": 1, "lines": 0, "links": 0, "intake": 0,
			"comment_events": 0, "operations": 0, "messages": 0, "conversations": 0, "bundle_peers": 1})
		if countPeer(t, target.id, peer) != 0 {
			t.Error("the erased bundle's peer link survives a bundle-selector erasure (single-bundle mode passes p_peer_keys=NULL)")
		}
		if countPeer(t, sibling.id, sibPeer) != 1 || countPeer(t, far.id, farPeer) != 1 {
			t.Error("a sibling's/another store's peer link was deleted")
		}
		if crDigest(t, w.owner, "inbox.bundle_peers", "peer_key=ANY($1::text[])", []string{sibPeer, farPeer})+
			crDigest(t, w.owner, "claims.bundles", "id=ANY($1::uuid[])", []string{sibling.id, far.id}) != othersBefore {
			t.Error("a sibling bundle's peer link or row changed (the derived delete is scoped to the erased bundle)")
		}
		if crRestoredManual(t, w.owner, target.id) == "" {
			t.Error("the erased manual bundle lacks its erased- label (the erasure itself must have run)")
		}
	})

	// Restore replay: replay_actor_erasures calls apply_actor_erasure(..., NULL) for BOTH tombstone shapes (the
	// actor digest resolves through claims.bundles/meta_intake; the bundle ref is single-bundle mode), so only the
	// bundle-derived delete can remove a restored actor's peer links. CRP08 models the full snapshot/restore; this
	// drives the same definer path through internal/retention.Replay with an explicit tombstone list.
	e.sub(t, "rd4-replay-derives-peer-links", func(t *testing.T) {
		s1 := w.session(t, w.store)
		w.closedAt(t, s1, crAgo(10, 0))
		s2 := w.session(t, w.store2)
		w.closedAt(t, s2, crAgo(10, 0))
		s3 := w.session(t, w.store)
		w.closedAt(t, s3, crAgo(10, 0))
		actor, peer := crHex64(), crHex64()
		otherPeer, manPeer := crHex64(), crHex64()
		b1 := w.bundle(t, s1, "facebook", actor, "")
		b2 := w.bundle(t, s2, "facebook", actor, "")
		b3 := w.bundle(t, s1, "facebook", crHex64(), "") // another actor's bundle, same store
		mb := w.bundle(t, s3, "manual", "", "lcr2-r2m-"+t04Tag())
		defer func() {
			mustExec(t, w.owner, `DELETE FROM inbox.bundle_peers WHERE peer_key=ANY($1::text[])`, []string{peer, otherPeer, manPeer})
		}()
		seedPeer(t, w.tenant, w.store, b1.id, peer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store2, b2.id, peer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store, b3.id, otherPeer, `(clock_timestamp())`)
		seedPeer(t, w.tenant, w.store, mb.id, manPeer, `(clock_timestamp())`)
		othersBefore := crDigest(t, w.owner, "inbox.bundle_peers", "peer_key=$1", otherPeer) +
			crDigest(t, w.owner, "claims.bundles", "id=$1", b3.id)
		reqA, reqB := randomUUID(), randomUUID()
		selA := sha256.Sum256([]byte("lcr2r2|a|" + reqA)) // synthetic selector digests (32 octets, never a real selector)
		selB := sha256.Sum256([]byte("lcr2r2|c|" + reqB))
		act := sha256.Sum256([]byte(actor)) // the replay resolves sha256(convert_to(actor_key,'UTF8'))
		mark := dbNow(t)
		n, err := retention.Replay(ctx, e.op, []retention.Tombstone{
			{RequestID: reqA, SelectorDigest: selA[:], ActorDigest: act[:]},
			{RequestID: reqB, SelectorDigest: selB[:], BundleTenant: w.tenant, BundleStore: w.store, BundleRef: mb.id},
		})
		if err != nil || n != 2 {
			t.Fatalf("Replay: n=%d err=%v, want 2 tombstones applied", n, err)
		}
		if countPeer(t, b1.id, peer) != 0 || countPeer(t, b2.id, peer) != 0 {
			t.Error("the replayed actor's peer links survive (replay_actor_erasures passes p_peer_keys=NULL)")
		}
		if countPeer(t, mb.id, manPeer) != 0 {
			t.Error("the replayed manual bundle's peer link survives")
		}
		if countPeer(t, b3.id, otherPeer) != 1 {
			t.Error("another actor's peer link was deleted by the replay")
		}
		if crDigest(t, w.owner, "inbox.bundle_peers", "peer_key=$1", otherPeer)+
			crDigest(t, w.owner, "claims.bundles", "id=$1", b3.id) != othersBefore {
			t.Error("another actor's peer link or bundle changed (the derived delete is scoped to the replayed tombstones)")
		}
		// the erasures themselves ran: bundles de-identified, both tombstones inserted with empty counts, one
		// replay row whose counts are numbers only (its v_keys sum carries no bundle_peers — purge contract B3).
		for _, b := range []string{b1.id, b2.id} {
			if r := e.bundleRow(t, b); r.purged == nil || r.actor == actor {
				t.Errorf("bundle %s not de-identified by the replay: purged=%v key kept=%t", b, r.purged, r.actor == actor)
			}
		}
		if crRestoredManual(t, w.owner, mb.id) == "" {
			t.Error("the manual bundle tombstone was not replayed (no erased- label)")
		}
		if got := crCount(t, w.owner, `SELECT count(*) FROM claims.retention_log WHERE kind='actor_erased' AND request_id=ANY($1::uuid[]) AND counts='{}'::jsonb`,
			[]string{reqA, reqB}); got != 2 {
			t.Errorf("inserted actor_erased rows with empty counts: %d, want 2", got)
		}
		var replayRow *crLogRow
		rows := crLogSince(t, w.owner, mark)
		for i := range rows {
			if rows[i].kind == "replay" {
				if replayRow != nil {
					t.Fatal("two replay rows for one replay")
				}
				replayRow = &rows[i]
			}
		}
		if replayRow == nil {
			t.Fatalf("no replay log row (rows: %+v)", rows)
		}
		for k, v := range replayRow.counts {
			if _, isNum := v.(float64); !isNum {
				t.Errorf("replay counts[%s]=%v is not a number", k, v)
			}
		}
		if replayRow.counts["tombstones"] != float64(2) || replayRow.counts["inserted"] != float64(2) || replayRow.counts["bundles"] != float64(3) {
			t.Errorf("replay counts %v, want tombstones=2 inserted=2 bundles=3 (two actor bundles + the manual one)", replayRow.counts)
		}
	})
}
