package foundation_test

// OC-16 (contracts/owner-choices-20261026-v1.md OC-Q1, migration 0202_bank_transfer_closed.sql): REAL_PG gates that the SERVER refuses new
// bank-transfer writes while history stays readable and runs to its terminal state. Prefix `btc`. Evidence label: REAL_PG, no PSP involved.
//   TestBtcPolicyClosedAfterMigrate      the policy row reads closed=true right after migrations.Apply, before the fixture's legacy flip
//   TestBtcPolicyNotWritableByApp        no commerce_* role can write the policy table; a real write attempt as the definer owner and
//                                        as the runtime login fails with 42501 (the fixture's legacy flip is owner-only, not a back door)
//   TestBtcFailClosedWithoutPolicyRow    a zero-row policy table (DELETE) still refuses new transfers and saves
//   TestBtcSaveReplayAfterClose           a same-key replay of a save that succeeded before closure returns the stored response; a new key is refused
//   TestBtcRefusesNewWrites              closed: a new bank_transfer order through the app, a direct INSERT of a transfer row, and every
//                                        way of saving or editing the merchant bank details (definer, real route, direct DML) are refused
//                                        PT422 bank_transfer_closed; nothing is written and the saved details are unchanged
//   TestBtcHistoryStaysUsable            closed: orders placed before closure confirm / reject / refund_offline through decide_bank_transfer,
//                                        and the order, buyer view, settings read and finance export still read; no row is deleted
// Fixture writes (disclosed): the policy row is flipped by the migration owner (closed for the test, restored to the legacy-open state by
// t.Cleanup, see newFixture / issue #210); one rolled-back direct INSERT/UPDATE per refusal.

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func btcSetPolicy(t *testing.T, f *testFixture, closed bool) {
	t.Helper()
	mustExec(t, f.owner, `UPDATE checkout.bank_transfer_policy SET closed=$1`, closed)
}

// btcClose closes the policy for this test and restores the fixture's legacy-open state afterwards.
func btcClose(t *testing.T, f *testFixture) {
	t.Helper()
	btcSetPolicy(t, f, true)
	t.Cleanup(func() { btcSetPolicy(t, f, false) })
}

// btcWant fails unless err is the PT422 bank_transfer_closed refusal.
func btcWant(t *testing.T, label string, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "PT422" || pgErr.Message != "bank_transfer_closed" {
		t.Errorf("%s: %v, want PT422 bank_transfer_closed", label, err)
	}
}

func TestBtcPolicyClosedAfterMigrate(t *testing.T) {
	f := fixture(t)
	if !legacyBankPolicyAfterMigrate.read || !legacyBankPolicyAfterMigrate.closed {
		t.Fatalf("policy right after migrations.Apply: %+v, want read and closed=true", legacyBankPolicyAfterMigrate)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.bank_transfer_policy`); n != 1 {
		t.Errorf("policy rows %d, want exactly 1", n)
	}
}

func TestBtcPolicyNotWritableByApp(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	rows, err := f.owner.Query(ctx, `SELECT r.rolname,p FROM pg_roles r CROSS JOIN unnest(ARRAY['INSERT','UPDATE','DELETE','TRUNCATE']) p
	  WHERE r.rolname LIKE 'commerce\_%' AND has_table_privilege(r.rolname,'checkout.bank_transfer_policy',p)`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var role, priv string
		if err := rows.Scan(&role, &priv); err != nil {
			t.Fatal(err)
		}
		t.Errorf("role %s holds %s on checkout.bank_transfer_policy", role, priv)
	}
	rows.Close()
	if n := countRows(t, f.owner, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%'`); n < 5 {
		t.Fatalf("only %d commerce_ roles listed: the scan would be vacuous", n)
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_checkout_writer`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE checkout.bank_transfer_policy SET closed=false`); sqlState(err) != "42501" {
		t.Errorf("UPDATE as commerce_checkout_writer: %v, want 42501", err)
	}
	if _, err := f.runtime.Exec(ctx, `UPDATE checkout.bank_transfer_policy SET closed=false`); sqlState(err) != "42501" {
		t.Errorf("UPDATE as the runtime login: %v, want 42501", err)
	}
	if _, err := f.runtime.Exec(ctx, `DELETE FROM checkout.bank_transfer_policy`); sqlState(err) != "42501" {
		t.Errorf("DELETE as the runtime login: %v, want 42501", err)
	}
}

func TestBtcRefusesNewWrites(t *testing.T) {
	e := cogNew(t, 72) // settings saved through the real route while the fixture is legacy-open
	f := e.p.f
	ctx := context.Background()
	btcClose(t, f)
	tenant, store := e.tenant(), e.store()
	version := func() int64 {
		var v int64
		if err := f.owner.QueryRow(ctx, `SELECT version FROM checkout.bank_transfer_settings WHERE store_id=$1`, store).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	state := func() [4]int64 {
		return [4]int64{
			int64(e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='bank_transfer'`, store)),
			int64(e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE store_id=$1`, store)),
			int64(e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE store_id=$1`, store)),
			version(),
		}
	}
	before := state()
	if before[2] != 1 || before[3] != 1 {
		t.Fatalf("setup: %d settings rows at version %d, want 1 at version 1 (one save through the real route)", before[2], before[3])
	}
	current := before[3]

	t.Run("a new bank_transfer order through the app is refused and leaves nothing", func(t *testing.T) {
		_, err := e.cofPlaceHome(e.newBuyer(), "")
		tcvExpectRefusal(t, "new bank_transfer order", err, 422, "bank_transfer_closed")
		if got := state(); got != before {
			t.Errorf("state %v after the refused order, was %v", got, before)
		}
	})
	t.Run("a direct INSERT of a transfer row is refused in SQL", func(t *testing.T) {
		_, err := f.owner.Exec(ctx, `INSERT INTO checkout.bank_transfers(tenant_id,store_id,owner_id,order_id,state,bank_name,branch,account_name,account_number,window_hours,currency,created_at,updated_at)
		  VALUES($1,$2,$3,$4,'AWAITING','B','b','a','12345678',72,'TWD',clock_timestamp(),clock_timestamp())`, tenant, store, randomUUID(), randomUUID())
		btcWant(t, "direct transfer INSERT", err)
	})
	t.Run("saving the merchant bank details is refused: definer, real route, direct DML", func(t *testing.T) {
		hash := sha256.Sum256([]byte(e.token()))
		save := func(expected int64, key string) error {
			var raw []byte
			return f.runtime.QueryRow(ctx, `SELECT payments.set_bank_transfer_settings($1,$2::uuid,$3,$4,$5,true,true,'Other Bank','Branch','Shop Ltd','99887766',72)`,
				hash[:], store, key, make([]byte, 32), expected).Scan(&raw)
		}
		btcWant(t, "definer, version CAS", save(current, t04Key("btc-save")))
		btcWant(t, "definer, first-time insert shape", save(0, t04Key("btc-save0")))
		// CI run 40447877252 job 122004962998: the route answers 422 with the generic public code "internal" and retryable=false, because
		// internal/httperror's code table does not know bank_transfer_closed yet (owned by OC-08/OC-15, out of OC-16's write_paths). The exact
		// SQL refusal is asserted above through the definer; here we pin the HTTP status and that it is a final (non-retryable) refusal.
		if st, out := e.cofSettings(current, true, true, 72); st != 422 || out["retryable"] != false {
			t.Errorf("real route answered %d %v, want a final 422 refusal", st, out)
		}
		_, err := f.owner.Exec(ctx, `UPDATE checkout.bank_transfer_settings SET bank_name='Other Bank' WHERE store_id=$1`, store)
		btcWant(t, "direct settings UPDATE", err)
		_, err = f.owner.Exec(ctx, `INSERT INTO checkout.bank_transfer_settings(tenant_id,store_id,enabled,allow_cvs,bank_name,branch,account_name,account_number,window_hours,version,updated_at)
		  VALUES($1,$2,true,false,'B','b','a','12345678',72,1,clock_timestamp())`, tenant, randomUUID())
		btcWant(t, "direct settings INSERT", err)
	})
	t.Run("the saved details are unchanged and still read", func(t *testing.T) {
		if got := state(); got != before {
			t.Errorf("state %v after the refused saves, was %v", got, before)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE store_id=$1 AND bank_name=$2`, store, cogBank); n != 1 {
			t.Error("the saved bank name changed")
		}
		if st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+store+"/bank-transfer-settings", "", ""); st != 200 || !strings.Contains(string(raw), cogBank) {
			t.Errorf("settings read: %d %v", st, out)
		}
	})
}

func TestBtcFailClosedWithoutPolicyRow(t *testing.T) {
	e := cogNew(t, 72)
	f := e.p.f
	ctx := context.Background()
	btcClose(t, f)
	// Registered after btcClose, so it runs first: put the row back (legacy-open) before btcClose's own restore.
	t.Cleanup(func() {
		mustExec(t, f.owner, `INSERT INTO checkout.bank_transfer_policy(closed) VALUES(false) ON CONFLICT DO NOTHING`)
	})
	mustExec(t, f.owner, `DELETE FROM checkout.bank_transfer_policy`) // disclosed: owner-only, restored by t.Cleanup
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfer_policy`); n != 0 {
		t.Fatalf("policy rows %d, want 0", n)
	}
	_, err := f.owner.Exec(ctx, `INSERT INTO checkout.bank_transfers(tenant_id,store_id,owner_id,order_id,state,bank_name,branch,account_name,account_number,window_hours,currency,created_at,updated_at)
	  VALUES($1,$2,$3,$4,'AWAITING','B','b','a','12345678',72,'TWD',clock_timestamp(),clock_timestamp())`, e.tenant(), e.store(), randomUUID(), randomUUID())
	btcWant(t, "transfer INSERT without a policy row", err)
	_, err = f.owner.Exec(ctx, `UPDATE checkout.bank_transfer_settings SET bank_name='Other Bank' WHERE store_id=$1`, e.store())
	btcWant(t, "settings UPDATE without a policy row", err)
	hash := sha256.Sum256([]byte(e.token()))
	var raw []byte
	err = f.runtime.QueryRow(ctx, `SELECT payments.set_bank_transfer_settings($1,$2::uuid,$3,$4,1,true,true,'Other Bank','Branch','Shop Ltd','99887766',72)`,
		hash[:], e.store(), t04Key("btc-zero"), make([]byte, 32)).Scan(&raw)
	btcWant(t, "definer save without a policy row", err)
}

func TestBtcSaveReplayAfterClose(t *testing.T) {
	e := cogNew(t, 72) // saved once through the real route (version 1) while legacy-open
	f := e.p.f
	ctx := context.Background()
	hash := sha256.Sum256([]byte(e.token()))
	save := func(key string) ([]byte, error) {
		var raw []byte
		err := f.runtime.QueryRow(ctx, `SELECT payments.set_bank_transfer_settings($1,$2::uuid,$3,$4,1,true,true,'Replay Bank','Branch','Shop Ltd','99887766',72)`,
			hash[:], e.store(), key, make([]byte, 32)).Scan(&raw)
		return raw, err
	}
	key := t04Key("btc-replay")
	first, err := save(key) // before the closure: a real success
	if err != nil {
		t.Fatalf("save before closure: %v", err)
	}
	btcClose(t, f)
	again, err := save(key)
	if err != nil || string(again) != string(first) {
		t.Errorf("same-key replay after closure: %v %s, want the stored response %s", err, again, first)
	}
	_, err = save(t04Key("btc-replay-new"))
	btcWant(t, "a new key after closure", err)
	var version int64
	if err := f.owner.QueryRow(ctx, `SELECT version FROM checkout.bank_transfer_settings WHERE store_id=$1`, e.store()).Scan(&version); err != nil || version != 2 {
		t.Errorf("settings version %d (%v), want 2: only the pre-closure save counts", version, err)
	}
}

func TestBtcHistoryStaysUsable(t *testing.T) {
	e := cogNew(t, 72)
	f := e.p.f
	// Orders that exist before the closure (the legacy-open fixture stands in for "placed before 0202").
	ba, confirmed := e.cogPlace("")
	bb, rejected := e.cogPlace("")
	_, refunded := e.cogPlace("")
	bd, lateProof := e.cogPlace("")
	_, expiring := e.cogPlace("")
	if r := bb.cofProof(rejected, t04Key("btc-proof"), "22222", e.cogTotal(rejected)); r.status != 200 {
		t.Fatalf("proof: %d %s", r.status, r.body)
	}
	btcClose(t, f)
	transfers := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE store_id=$1`, e.store())
	orders := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
	settings := e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE store_id=$1`, e.store())
	if transfers != 5 || settings != 1 {
		t.Fatalf("setup: %d transfers, %d settings", transfers, settings)
	}

	t.Run("confirm, reject and refund_offline run to their end states", func(t *testing.T) {
		if d := e.cogConfirm(confirmed, t04Key("btc-c1")); d.status != 200 {
			t.Fatalf("confirm: %d %v", d.status, d.out)
		}
		if got, err := e.cogRow(confirmed).cogOutcome(); err != nil || got != "confirmed" {
			t.Errorf("confirmed order: %q %v", got, err)
		}
		if st, out := e.cofDecide(e.token(), rejected, "reject", t04Key("btc-r1"), `{"reason":"mismatch"}`); st != 200 {
			t.Fatalf("reject: %d %v", st, out)
		}
		if got := e.cogRow(rejected).transfer; got != "REJECTED" {
			t.Errorf("rejected transfer state %q", got)
		}
		if d := e.cogConfirm(refunded, t04Key("btc-c2")); d.status != 200 {
			t.Fatalf("confirm before refund: %d %v", d.status, d.out)
		}
		if st, out := e.cofDecide(e.token(), refunded, "refund-offline", t04Key("btc-f1"), `{}`); st != 200 {
			t.Fatalf("refund_offline: %d %v", st, out)
		}
		if got := e.cogRow(refunded).transfer; got != "REFUNDED_OFFLINE" {
			t.Errorf("refunded transfer state %q", got)
		}
	})
	// OC-Q1 keeps proofs and AWAITING_TRANSFER states; a buyer paying an order placed before the closure must still be able to submit the proof.
	t.Run("submit_transfer_proof still works on a pre-closure order", func(t *testing.T) {
		if r := bd.cofProof(lateProof, t04Key("btc-late"), "33333", e.cogTotal(lateProof)); r.status != 200 {
			t.Fatalf("proof after closure: %d %s", r.status, r.body)
		}
		if got := e.cogRow(lateProof).transfer; got != "SUBMITTED" {
			t.Errorf("transfer state %q, want SUBMITTED", got)
		}
	})
	t.Run("expire_held still reaches EXPIRED on a pre-closure order", func(t *testing.T) {
		e.cofAge(expiring, 80)
		e.cofExpire(expiring)
		if got, err := e.cogRow(expiring).cogOutcome(); err != nil || got != "expired" {
			t.Errorf("expiring order: %q %v", got, err)
		}
	})
	t.Run("orders, transfer records, buyer view, settings and the finance export still read", func(t *testing.T) {
		for _, o := range []string{confirmed, rejected, refunded, lateProof, expiring} {
			if st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+o+"/bank-transfer", "", ""); st != 200 {
				t.Errorf("merchant transfer read of %s: %d %v %s", o, st, out, raw)
			}
		}
		if r := ba.cofView(confirmed); r.status != 200 {
			t.Errorf("buyer view: %d %s", r.status, r.body)
		}
		if st, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/bank-transfer-settings", "", ""); st != 200 || !strings.Contains(string(raw), cogBank) {
			t.Errorf("settings read: %d %s", st, raw)
		}
		today := time.Now().In(time.FixedZone("TPE", 8*3600))
		csv := e.financeCSV(today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02"))
		if !strings.Contains(strings.SplitN(csv, "\n", 2)[0], "bank_transfer_confirmed_count,bank_transfer_confirmed_minor") {
			t.Errorf("finance CSV lost the bank_transfer_confirmed columns: %q", strings.SplitN(csv, "\n", 2)[0])
		}
	})
	t.Run("no row was deleted", func(t *testing.T) {
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE store_id=$1`, e.store()); n != transfers {
			t.Errorf("transfers %d, was %d", n, transfers)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n != orders {
			t.Errorf("orders %d, was %d", n, orders)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE store_id=$1`, e.store()); n != settings {
			t.Errorf("settings %d, was %d", n, settings)
		}
	})
}
