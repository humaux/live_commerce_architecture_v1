// Purpose: prove order-intent definers recheck bearer authority after the exact pay-link lock wait.
// Depends on: real manual HTTP/Begin fixtures, runtime WithScope, 0172 definers and owner-only lock/revocation seams.
// Used by: REAL_PG CI red/green; no local test, build or browser execution substitutes for this concurrency proof.
// Invariants: I01/I07/I11; revoked callers cannot validate a private link or commit source/link writes.
// Every transaction callback returns errors: Goexit inside WithScope would strand its acquired transaction.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

func TestOrderIntentDefinersRecheckAfterPayLinkLock(t *testing.T) {
	for _, kind := range []string{"ValidLink", "ManualSource"} {
		for _, revoke := range []string{"inventory:reserve", "catalog:read", "session"} {
			t.Run(kind+"/"+revoke, func(t *testing.T) {
				e := lbuNew(t)
				baseOrders := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
				key, body := t04Key("intent-pay-lock"), mnrBody(e, 1)
				var order string
				var linkHash []byte
				if kind == "ValidLink" {
					status, placed := e.admin(e.token()).place(key, body)
					if status != 201 {
						t.Fatalf("real completed order fixture: status=%d", status)
					}
					order = lbuStr(placed, "order_id")
					linkHash = oilsLinkHash(t, lbuStr(placed, "buyer_link"), order)
				} else {
					mnrPartialKill(t, e, key, body) // Real Begin committed; only its upper source/link transaction rolled back.
					oiaRead(t, e, `SELECT r.order_id::text FROM ops.order_intents i JOIN checkout.command_results r
 ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.owner_id=i.buyer_owner_id
 AND r.operation='checkout.begin' AND r.idempotency_key=i.begin_key
 WHERE i.tenant_id=$1 AND i.store_id=$2 AND i.principal_id=$3 AND i.idempotency_key=$4 AND i.state='pending'`,
						[]any{e.tenant(), e.store(), e.p.f.principalA, key}, &order)
					// Disclosed candidate hash for the real source definer; it is never printed or presented as an issued capability.
					digest := sha256.Sum256([]byte("intent-lock-link|" + randomUUID()))
					linkHash = digest[:]
				}
				before := oilsFactsOf(t, e, order)
				if before.orders != baseOrders+1 || before.held != 1 || e.plan.planned() != 0 {
					t.Fatal("fixture must contain one exact new held order and no payment DM")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if kind == "ValidLink" {
					valid, err := oilsDefiner(ctx, e, kind, key, order, linkHash, nil)
					if err != nil || !valid || before.source != "merchant_manual" || before.links != 1 {
						t.Fatal("positive sentinel: original issued link must validate before authority is revoked")
					}
				} else {
					var committed string
					err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
						return tx.QueryRow(ctx, `SELECT order_id::text FROM fulfillment.order_intent_begin($1,$2::uuid,$3)`, tokenHash(e.token()), e.store(), key).Scan(&committed)
					})
					if err != nil || committed != order || before.source != "storefront" || before.links != 0 {
						t.Fatal("positive sentinel: source repair must have the exact admitted Begin and no prior source/link commit")
					}
				}

				holder, err := e.p.f.owner.Begin(ctx)
				if err != nil {
					t.Fatal("acquire owned pay-link barrier transaction")
				}
				defer func() {
					cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
					defer stop()
					_ = holder.Rollback(cleanup)
				}()
				lock := "order-pay-link|" + e.tenant() + "|" + e.store() + "|" + order
				if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lock); err != nil {
					t.Fatal("acquire exact order pay-link advisory lock")
				}
				pids, done := make(chan int, 1), make(chan oilsResult, 1)
				go func() {
					accepted, callErr := oilsDefiner(ctx, e, kind, key, order, linkHash, pids)
					done <- oilsResult{accepted: accepted, err: callErr}
				}()
				joined := false
				defer func() {
					cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
					defer stop()
					_ = holder.Rollback(cleanup)
					cancel()
					if !joined {
						select {
						case <-done:
						case <-cleanup.Done():
							t.Error("owned definer did not exit after barrier rollback and context cancellation")
						}
					}
				}()
				var pid int
				select {
				case pid = <-pids:
				case <-done:
					joined = true
					t.Fatal("definer failed before entering the authenticated runtime transaction")
				case <-ctx.Done():
					t.Fatal("definer did not enter its authenticated runtime transaction")
				}
				function := "fulfillment.order_pay_link_valid"
				if kind == "ManualSource" {
					function = "fulfillment.mark_order_intent_manual"
				}
				// Observe the exact runtime backend and owned holder. This deadline stays below the existing 1 s lock budget.
				wait, stopWait := context.WithTimeout(ctx, 700*time.Millisecond)
				blocked := false
				for wait.Err() == nil {
					err = e.p.f.owner.QueryRow(wait, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity a
 WHERE a.pid=$1 AND a.datname=current_database() AND a.state='active' AND a.wait_event_type='Lock'
 AND $2::int=ANY(pg_blocking_pids(a.pid)) AND position($3 IN a.query)>0)`, pid, int(holder.Conn().PgConn().PID()), function).Scan(&blocked)
					if err != nil || blocked {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				stopWait()
				if err != nil || !blocked {
					t.Fatal("exact order-intent definer did not observably wait on the owned pay-link lock")
				}
				wanted := "PT403"
				var changed int64
				if revoke == "session" {
					wanted = "PT401"
					tag, revokeErr := e.p.f.owner.Exec(ctx, `UPDATE identity.sessions SET revoked_at=clock_timestamp()
 WHERE token_hash=$1 AND principal_id=$2 AND revoked_at IS NULL`, tokenHash(e.token()), e.p.f.principalA)
					err, changed = revokeErr, tag.RowsAffected()
				} else {
					tag, revokeErr := e.p.f.owner.Exec(ctx, `DELETE FROM identity.store_grants
 WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, e.tenant(), e.store(), e.p.f.principalA, revoke)
					err, changed = revokeErr, tag.RowsAffected()
				}
				if err != nil || changed != 1 {
					t.Fatal("owned authority revocation did not commit exactly one change during the lock wait")
				}
				if err = holder.Commit(ctx); err != nil {
					t.Fatal("release owned pay-link advisory barrier")
				}
				var result oilsResult
				select {
				case result = <-done:
					joined = true
				case <-ctx.Done():
					t.Fatal("order-intent definer did not finish after authority loss and lock release")
				}
				// Require the definer itself to deny. A caller's later rollback cannot substitute for this SQL authority fence.
				if result.accepted || sqlState(result.err) != wanted {
					t.Errorf("definer accepted after authority loss: accepted=%t SQLSTATE=%s want=%s", result.accepted, sqlState(result.err), wanted)
				}
				if after := oilsFactsOf(t, e, order); after != before || e.plan.planned() != 0 {
					t.Error("lock-wait authority loss changed exact source/link, stock, intent, receipt, audit or order facts")
				}
			})
		}
	}
}

type oilsResult struct {
	accepted bool
	err      error
}

// oilsDefiner runs the actual scoped runtime helper. It never calls a testing helper or Goexit in the callback.
func oilsDefiner(ctx context.Context, e *lbuEnv, kind, key, order string, link []byte, pids chan<- int) (bool, error) {
	accepted := false
	err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
		if pids != nil {
			var pid int
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				return err
			}
			pids <- pid
		}
		if kind == "ValidLink" {
			return tx.QueryRow(ctx, `SELECT fulfillment.order_pay_link_valid($1,$2::uuid,$3::uuid,$4)`, tokenHash(e.token()), e.store(), order, link).Scan(&accepted)
		}
		_, callErr := tx.Exec(ctx, `SELECT fulfillment.mark_order_intent_manual($1,$2::uuid,$3,$4::uuid,$5)`, tokenHash(e.token()), e.store(), key, order, link)
		accepted = callErr == nil
		return callErr
	})
	return accepted, err
}

// Owner reads prove immutable before/after facts without broadening any runtime definer or table grant.
type oilsFacts struct {
	source                                string
	orders, links, held, receipts, audits int
	intentState                           string
	privateInput                          bool
}

func oilsFactsOf(t *testing.T, e *lbuEnv, order string) oilsFacts {
	t.Helper()
	var out oilsFacts
	oiaRead(t, e, `SELECT o.source,
 (SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2),
 (SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3),
 (SELECT coalesce(sum(l.quantity),0) FROM inventory.reservation_lines l JOIN inventory.reservations r
  ON r.tenant_id=l.tenant_id AND r.store_id=l.store_id AND r.id=l.reservation_id
  WHERE r.tenant_id=$1 AND r.store_id=$2 AND r.id=$3 AND r.state='HELD'),
 (SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2),
 (SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2),
 i.state,i.frozen_request IS NOT NULL
 FROM checkout.orders o JOIN checkout.command_results r
 ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id AND r.owner_id=o.owner_id AND r.order_id=o.id AND r.operation='checkout.begin'
 JOIN ops.order_intents i ON i.tenant_id=r.tenant_id AND i.store_id=r.store_id AND i.buyer_owner_id=r.owner_id AND i.begin_key=r.idempotency_key
 WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.id=$3`, []any{e.tenant(), e.store(), order},
		&out.source, &out.orders, &out.links, &out.held, &out.receipts, &out.audits, &out.intentState, &out.privateInput)
	return out
}

func oilsLinkHash(t *testing.T, link, order string) []byte {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal("real manual fixture returned an invalid link URL")
	}
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil || fragment.Get("o") != order || fragment.Get("t") == "" {
		t.Fatal("real manual fixture link does not bind its exact order")
	}
	hash := sha256.Sum256([]byte(fragment.Get("t")))
	return hash[:]
}
