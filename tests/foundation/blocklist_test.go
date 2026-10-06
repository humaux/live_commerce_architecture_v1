// Purpose: REAL_PG gates BL01-BL08 of W3-05B (restricted-buyer list): a restricted actor's claim is recorded but gets no link and no automatic reply, unblock restores it,
//
//	the definers' scope/permission/limit rules, the claim-vs-block race, erasure of the entry, and the ACL of migration 0154.
//
// Depends on: lbSetup / mciEnv harnesses (signed webhook -> intake -> claims -> reply planner), claims.BlockActor/UnblockActor/ListBlockedActors/BlockedForBundle, retention definer claims.apply_actor_erasure.
// Used by: scripts/dev/test-focused.sh 'Blocklist', GitHub shard gates.
// Invariants: block key is the actor_key of ONE store; no operation row (so no mpr: quota) for a restricted claim; claim_reply_plannable never raises; note/actor_key never in audit or receipts.
// Status: MOCK (REAL_PG; no Graph call is made by a restricted claim); Meta LIVE NOT_RUN.
package foundation_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

func (e *mciEnv) blockWith(t *testing.T, token, store, session string, in claims.BlockInput) (claims.BlockResult, error) {
	t.Helper()
	var out claims.BlockResult
	err := platform.WithScope(context.Background(), e.h.f.runtime, token, store, "live:manage", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = claims.BlockActor(context.Background(), tx, s, token, t04Key("bl"), session, in)
		return err
	})
	return out, err
}

// block restricts the author of one comment of the harness session (the merchant's own token, store A1).
func (e *mciEnv) block(t *testing.T, comment, note string) claims.BlockResult {
	t.Helper()
	out, err := e.blockWith(t, e.h.token, e.h.f.storeA1, e.session, claims.BlockInput{CommentRef: comment, Note: note})
	if err != nil {
		t.Fatalf("BlockActor(%s): %v", comment, err)
	}
	return out
}

func (e *mciEnv) unblock(t *testing.T, id string) error {
	t.Helper()
	return platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "live:manage", func(tx pgx.Tx, s platform.Scope) error {
		_, err := claims.UnblockActor(context.Background(), tx, s, e.h.token, t04Key("bl"), id)
		return err
	})
}

func (e *mciEnv) listBlocked(t *testing.T, token, store string, page pagination.Request) (pagination.Page[claims.BlockedActor], error) {
	t.Helper()
	var out pagination.Page[claims.BlockedActor]
	err := platform.WithScope(context.Background(), e.h.f.runtime, token, store, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = claims.ListBlockedActors(context.Background(), tx, s, token, page)
		return err
	})
	return out, err
}

func (e *mciEnv) restrictedBundle(t *testing.T, token, store, bundle string) (bool, error) {
	t.Helper()
	var out bool
	err := platform.WithScope(context.Background(), e.h.f.runtime, token, store, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = claims.BlockedForBundle(context.Background(), tx, s, token, bundle)
		return err
	})
	return out, err
}

// miText reads one text value (owner pool).
func miText(t *testing.T, p *pgxpool.Pool, query string, args ...any) string {
	t.Helper()
	var out string
	if err := p.QueryRow(context.Background(), query, args...).Scan(&out); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return out
}

func blCleanup(t *testing.T, e *mciEnv) {
	t.Helper()
	f := e.h.f
	clean := func() {
		_, _ = f.owner.Exec(context.Background(), `DELETE FROM claims.blocked_actors WHERE tenant_id=$1 AND store_id IN ($2,$3)`, f.tenantA, f.storeA1, f.storeA2)
	}
	clean()
	t.Cleanup(clean)
}

func (e *mciEnv) blAudit(t *testing.T, action string) int64 {
	return miCount(t, e.h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action=$3`, e.h.f.tenantA, e.h.f.storeA1, action)
}

func (e *mciEnv) linkCount(t *testing.T, bundle string) int64 {
	return miCount(t, e.h.f.owner, `SELECT count(*) FROM claims.links WHERE bundle_id=$1`, bundle)
}

// BL01 + BL02 + BL03 + console mark: a restricted actor's claim is ACCEPTED and recorded with reply_kind restricted, no link, no operation, one audited skip;
// the merchant keeps one manual private reply; another actor is unaffected; unblocking restores the link for the next claim.
func TestBlocklistRestrictedClaim(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	blCleanup(t, e.mciEnv)
	f := e.h.f

	// An identifying comment (any comment of the author works; here a non-claim "hello" so no bundle exists yet).
	from := mciDigits(15)
	c0 := e.planReply(t, false, from, "hello")
	entry := e.block(t, c0.s.comment, "BL-NOTE-internal-reason")
	if entry.ID == "" || entry.Platform != "facebook" || !entry.Created {
		t.Fatalf("block result %+v", entry)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.blocked_actors WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1); n != 1 {
		t.Fatalf("blocked rows = %d, want 1", n)
	}

	skipBefore := e.blAudit(t, "claim_reply_skipped:restricted")
	r := e.planReply(t, false, from, "A1")
	if r.ev.outcome != "ACCEPTED" || r.bundleID == "" {
		t.Fatalf("BL01 the claim must still be recorded: %+v", r.ev)
	}
	if r.op != "" || e.opCount(t, r.s.comment) != 0 {
		t.Fatalf("BL01 restricted actor got an operation: %q", r.op)
	}
	if n := e.linkCount(t, r.bundleID); n != 0 {
		t.Fatalf("BL01 restricted actor got %d claim links", n)
	}
	if got := e.blAudit(t, "claim_reply_skipped:restricted"); got != skipBefore+1 {
		t.Fatalf("restricted skip audit +%d, want +1", got-skipBefore)
	}
	var kind *string
	var pending bool
	if err := f.owner.QueryRow(context.Background(), `SELECT e.reply_kind, b.link_pending_manual FROM claims.events e JOIN claims.bundles b ON b.id=e.bundle_id WHERE e.id=$1`, r.intake.AppliedEvent).Scan(&kind, &pending); err != nil {
		t.Fatal(err)
	}
	if kind == nil || *kind != "restricted" || pending {
		t.Fatalf("event reply_kind=%v link_pending_manual=%v, want restricted/false", kind, pending)
	}
	// Console mark: the claim stays ACCEPTED, its reason reads restricted.
	var status, reason string
	err := platform.WithScope(context.Background(), f.runtime, e.h.token, f.storeA1, "live:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT claim_outcome, claim_reason FROM live.console_marks($1::uuid,$2::text[])`, e.session, []string{r.s.comment}).Scan(&status, &reason)
	})
	if err != nil || status != "ACCEPTED" || reason != "restricted" {
		t.Fatalf("console mark outcome=%q reason=%q err=%v", status, reason, err)
	}

	// BL03: no mpr: quota was spent, so the merchant may reply by hand once (and only once).
	if _, err := e.manual(r.s.comment, "bl-manual-"+t04Tag(), time.Now().Add(-10*time.Minute), false); err != nil {
		t.Fatalf("BL03 manual private reply to a restricted actor: %v", err)
	}
	if _, err := e.manual(r.s.comment, "bl-manual2-"+t04Tag(), time.Now().Add(-10*time.Minute), false); planCode(err) != "used" {
		t.Fatalf("BL03 second manual reply: %v", err)
	}

	// Another actor of the same store is unaffected.
	other := e.planReply(t, false, "", "A1")
	if other.op == "" || e.linkCount(t, other.bundleID) == 0 {
		t.Fatalf("an unrestricted actor must get the claim link: op=%q", other.op)
	}

	// BL02: block -> unblock before the claim restores the normal link flow.
	from2 := mciDigits(15)
	d0 := e.planReply(t, false, from2, "hello")
	e2 := e.block(t, d0.s.comment, "")
	if err := e.unblock(t, e2.ID); err != nil {
		t.Fatal(err)
	}
	d1 := e.planReply(t, false, from2, "A1")
	if d1.op == "" || e.linkCount(t, d1.bundleID) == 0 {
		t.Fatalf("BL02 after unblock the claim must get its link: op=%q", d1.op)
	}
	if err := e.unblock(t, e2.ID); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("unblocking a removed entry: %v, want not found", err)
	}
}

// The check endpoint's data (BL06) and the list: restricted bundle true, others false, only the merchant sees the note; foreign store -> not found.
func TestBlocklistCheckAndList(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	blCleanup(t, e.mciEnv)
	f := e.h.f
	from := mciDigits(15)
	c0 := e.planReply(t, false, from, "A1") // creates the bundle (link planned)
	if c0.bundleID == "" {
		t.Fatal("no bundle")
	}
	other := e.planReply(t, false, "", "A1")

	if got, err := e.restrictedBundle(t, e.h.token, f.storeA1, c0.bundleID); err != nil || got {
		t.Fatalf("before blocking: %v %v", got, err)
	}
	// Block through the BUNDLE reference this time.
	res, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{BundleID: c0.bundleID, Note: "BL06-secret-note"})
	if err != nil || !res.Created {
		t.Fatalf("block by bundle: %+v %v", res, err)
	}
	if got, err := e.restrictedBundle(t, e.h.token, f.storeA1, c0.bundleID); err != nil || !got {
		t.Fatalf("BL06 restricted bundle: %v %v", got, err)
	}
	if got, err := e.restrictedBundle(t, e.h.token, f.storeA1, other.bundleID); err != nil || got {
		t.Fatalf("BL06 other bundle: %v %v", got, err)
	}
	// Re-blocking is idempotent and keeps the first note; the list shows it with the note and never an actor key.
	again, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{CommentRef: c0.s.comment, Note: "second"})
	if err != nil || again.Created || again.ID != res.ID {
		t.Fatalf("re-block: %+v %v", again, err)
	}
	page, err := e.listBlocked(t, e.h.token, f.storeA1, pagination.Request{})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != res.ID || page.Items[0].Note == nil || *page.Items[0].Note != "BL06-secret-note" ||
		page.Items[0].SourceBundleID == nil || *page.Items[0].SourceBundleID != c0.bundleID {
		t.Fatalf("list: %+v %v", page, err)
	}
	// Store B scope: nothing listed, and the A1 bundle / comment / session are not found there.
	empty, err := e.listBlocked(t, e.h.token, f.storeA2, pagination.Request{})
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("other store list: %+v %v", empty, err)
	}
	if _, err := e.restrictedBundle(t, e.h.token, f.storeA2, c0.bundleID); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("other store check: %v", err)
	}
	if _, err := e.blockWith(t, e.h.token, f.storeA2, e.session, claims.BlockInput{CommentRef: c0.s.comment}); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("BL04 other store comment_ref: %v, want not found", err)
	}
	// Paging: 2 entries, limit 1 -> cursor -> the second.
	if _, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{BundleID: other.bundleID}); err != nil {
		t.Fatal(err)
	}
	p1, err := e.listBlocked(t, e.h.token, f.storeA1, pagination.Request{Limit: 1})
	if err != nil || len(p1.Items) != 1 || p1.NextCursor == "" {
		t.Fatalf("page 1: %+v %v", p1, err)
	}
	p2, err := e.listBlocked(t, e.h.token, f.storeA1, pagination.Request{Limit: 1, Cursor: p1.NextCursor})
	if err != nil || len(p2.Items) != 1 || p2.Items[0].ID == p1.Items[0].ID || p2.NextCursor != "" {
		t.Fatalf("page 2: %+v %v", p2, err)
	}
}

// BL04 + BL05 + BL08 at the definer: malformed references are rejected, a client-supplied actor key is not a reference, a read-only principal cannot write,
// and neither the note nor the actor key reaches the audit or the receipts.
func TestBlocklistDefinerRulesAndPrivacy(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	blCleanup(t, e.mciEnv)
	f := e.h.f
	c0 := e.planReply(t, false, "", "A1")
	note := "BL08-note-" + t04Tag()

	// BL04: two references, an empty input, a malformed id, a manual bundle, an unknown comment.
	for name, in := range map[string]claims.BlockInput{
		"two refs":        {CommentRef: c0.s.comment, BundleID: c0.bundleID},
		"none":            {},
		"bad bundle":      {BundleID: "not-a-uuid"},
		"bad comment":     {CommentRef: "abc def"},
		"too long note":   {CommentRef: c0.s.comment, Note: strings.Repeat("x", 201)},
		"control in note": {CommentRef: c0.s.comment, Note: "a\x01b"},
	} {
		if _, err := e.blockWith(t, e.h.token, f.storeA1, e.session, in); !errors.Is(err, command.ErrInvalid) && !errors.Is(err, platform.ErrScopeNotFound) {
			t.Errorf("BL04 %s: %v, want invalid", name, err)
		}
	}
	if _, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{CommentRef: "999000111222333_4445556667"}); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Errorf("BL04 unknown comment_ref: %v, want not found", err)
	}
	// An actor key is never a reference: the raw definer call with {"actor_key": ...} is 22023, and the runtime role cannot read the table.
	key := miText(t, f.owner, `SELECT actor_key FROM claims.bundles WHERE id=$1`, c0.bundleID)
	err := platform.WithScope(context.Background(), f.runtime, e.h.token, f.storeA1, "live:manage", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(context.Background(), `SELECT * FROM claims.block_actor($1::uuid, jsonb_build_object('actor_key',$2::text), NULL)`, e.session, key)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "22023") {
		t.Errorf("BL04 actor_key reference: %v, want SQLSTATE 22023", err)
	}
	err = platform.WithScope(context.Background(), f.runtime, e.h.token, f.storeA1, "live:read", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(context.Background(), `SELECT * FROM claims.blocked_actors`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "42501") {
		t.Errorf("runtime role read claims.blocked_actors directly: %v, want permission denied", err)
	}
	// Manual bundle: no social actor.
	manual := e.h.accepted(t, e.session, "", "bl-manual-"+t04Tag(), "A1")
	if _, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{BundleID: manual.BundleID}); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("BL04 manual bundle: %v, want invalid", err)
	}

	// BL05: live:read only.
	_, ro := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	err = platform.WithScope(context.Background(), f.runtime, ro, f.storeA1, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		_, err := claims.BlockActor(context.Background(), tx, s, ro, t04Key("bl"), e.session, claims.BlockInput{CommentRef: c0.s.comment})
		return err
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Errorf("BL05 live:read principal blocked an actor: %v", err)
	}
	if _, err := e.listBlocked(t, ro, f.storeA1, pagination.Request{}); err != nil {
		t.Errorf("live:read principal cannot list: %v", err)
	}

	// BL08: one audit row per NEW block / unblock; no note or actor key in audit or receipts.
	blocked, unblocked := e.blAudit(t, "claims.actor_blocked"), e.blAudit(t, "claims.actor_unblocked")
	res := e.block(t, c0.s.comment, note)
	e.block(t, c0.s.comment, "")
	if got := e.blAudit(t, "claims.actor_blocked"); got != blocked+1 {
		t.Errorf("claims.actor_blocked audit +%d, want +1 (the idempotent re-block writes none)", got-blocked)
	}
	if err := e.unblock(t, res.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.blAudit(t, "claims.actor_unblocked"); got != unblocked+1 {
		t.Errorf("claims.actor_unblocked audit +%d, want +1", got-unblocked)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation LIKE 'claims.actor.%' AND (response::text LIKE '%'||$3||'%' OR response::text LIKE '%'||$4||'%')`,
		f.tenantA, f.storeA1, note, key); n != 0 {
		t.Errorf("%d receipts contain the note or the actor key", n)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND (details::text LIKE '%'||$3||'%' OR details::text LIKE '%'||$4||'%')`,
		f.tenantA, f.storeA1, note, key); n != 0 {
		t.Errorf("%d audit rows contain the note or the actor key", n)
	}
}

// BL07: the 5000-entries-per-store bound is enforced under the store lock; an already listed actor is still idempotent at the bound.
func TestBlocklistLimit(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	blCleanup(t, e.mciEnv)
	f := e.h.f
	c0 := e.planReply(t, false, "", "A1")
	c1 := e.planReply(t, false, "", "A1")
	e.block(t, c0.s.comment, "")
	mustExec(t, f.owner, `INSERT INTO claims.blocked_actors(tenant_id,store_id,actor_key,platform,principal_id)
		SELECT $1,$2,md5(g::text)||md5(g::text||'x'),'facebook',$3 FROM generate_series(1,4999) g`, f.tenantA, f.storeA1, e.h.actor)
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.blocked_actors WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1); n != 5000 {
		t.Fatalf("seeded %d rows, want 5000", n)
	}
	if _, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{CommentRef: c1.s.comment}); !errors.Is(err, claims.ErrBlocklistFull) {
		t.Fatalf("BL07 the 5001st entry: %v, want limit_reached", err)
	}
	if res, err := e.blockWith(t, e.h.token, f.storeA1, e.session, claims.BlockInput{CommentRef: c0.s.comment}); err != nil || res.Created {
		t.Fatalf("an already listed actor at the bound: %+v %v", res, err)
	}
}

// The race the per-actor advisory key closes: a block that holds the key and commits while the claim is being applied is SEEN by the claim (it waits, then
// reads a fresh snapshot); and concurrent claim + block never leave a half state (link without the plain kind, or no link without the restricted mark).
func TestBlocklistClaimBlockRace(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	blCleanup(t, e.mciEnv)
	f := e.h.f
	ctx := context.Background()

	t.Run("a block holding the actor key is seen by the waiting claim", func(t *testing.T) {
		from := mciDigits(15)
		c0 := e.planReply(t, false, from, "hello")
		key := c0.intake.ActorKey
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('claims-blockactor|'||$1::text||'|'||$2::text||'|'||$3::text,0))`, f.tenantA, f.storeA1, key); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO claims.blocked_actors(tenant_id,store_id,actor_key,platform,principal_id) VALUES($1,$2,$3,'facebook',$4)`, f.tenantA, f.storeA1, key, e.h.actor); err != nil {
			t.Fatal(err)
		}
		s := e.postFB(t, "", from, "A1", mciAt(3*time.Second), nil)
		// The claim transaction needs the actor key while the block holds it: lock_timeout (55P03) ends that attempt, which the poller records as a
		// retryable failure (nothing is half-applied: no event, no operation, no link). It must not slip through unrestricted.
		e.apply(t)
		held := e.mustIntake(t, "page", e.pageAsset, s.comment)
		if held.State != "PENDING" || held.FailCode != "sqlstate_55p03" || held.AppliedEvent != "" || e.opCount(t, s.comment) != 0 {
			t.Fatalf("claim during the block: state=%s fail=%s applied=%q ops=%d, want a deferred PENDING row", held.State, held.FailCode, held.AppliedEvent, e.opCount(t, s.comment))
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		mustExec(t, f.owner, `UPDATE claims.meta_intake SET not_before=clock_timestamp() WHERE id=$1`, held.ID)
		e.apply(t)
		in := e.mustIntake(t, "page", e.pageAsset, s.comment)
		if e.opCount(t, s.comment) != 0 || in.AppliedEvent == "" {
			t.Fatalf("the waiting claim must be recorded and restricted: ops=%d applied=%q", e.opCount(t, s.comment), in.AppliedEvent)
		}
		if k := miText(t, f.owner, `SELECT coalesce(reply_kind,'') FROM claims.events WHERE id=$1`, in.AppliedEvent); k != "restricted" {
			t.Fatalf("reply_kind = %q, want restricted", k)
		}
	})

	t.Run("concurrent claim and block stay consistent", func(t *testing.T) {
		for i := 0; i < 6; i++ {
			from := mciDigits(15)
			c0 := e.planReply(t, false, from, "hello")
			s := e.postFB(t, "", from, "A1", mciAt(3*time.Second), nil)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); e.apply(t) }()
			go func() { defer wg.Done(); e.block(t, c0.s.comment, "") }()
			wg.Wait()
			in := e.mustIntake(t, "page", e.pageAsset, s.comment)
			ops := e.opCount(t, s.comment)
			kind := miText(t, f.owner, `SELECT coalesce(reply_kind,'') FROM claims.events WHERE id=$1`, in.AppliedEvent)
			bundle := miText(t, f.owner, `SELECT bundle_id::text FROM claims.events WHERE id=$1`, in.AppliedEvent)
			links := e.linkCount(t, bundle)
			switch {
			case ops == 1 && links >= 1 && kind == "": // the claim won: answered, the block applies from the next comment
			case ops == 0 && links == 0 && kind == "restricted": // the block won
			default:
				t.Fatalf("iteration %d inconsistent: ops=%d links=%d reply_kind=%q", i, ops, links, kind)
			}
		}
	})
}

// Erasure: claims.apply_actor_erasure (the helper of erase_actor / replay) removes the actor's entry and only that actor's; the retention role holds only the grants it needs.
func TestBlocklistErasureRemovesEntry(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	blCleanup(t, e.mciEnv)
	f := e.h.f
	a := e.planReply(t, false, "", "A1")
	b := e.planReply(t, false, "", "A1")
	e.block(t, a.s.comment, "erase-me")
	e.block(t, b.s.comment, "keep-me")
	keyA := a.intake.ActorKey
	// Owner calls the internal helper (EXECUTE nobody): the DELETE inside it runs as commerce_retention_writer, so the new grant and policies are exercised.
	var counts string
	if err := f.owner.QueryRow(context.Background(), `SELECT claims.apply_actor_erasure('facebook',$1,NULL,NULL,NULL,NULL)::text`, keyA).Scan(&counts); err != nil {
		t.Fatalf("apply_actor_erasure: %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.blocked_actors WHERE tenant_id=$1 AND store_id=$2 AND actor_key=$3`, f.tenantA, f.storeA1, keyA); n != 0 {
		t.Fatalf("erased actor's blocklist entry survived (%d)", n)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.blocked_actors WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1); n != 1 {
		t.Fatalf("another actor's entry must stay: %d rows", n)
	}
}
