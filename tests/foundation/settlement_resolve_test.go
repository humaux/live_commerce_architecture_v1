package foundation_test

// S2-OPEN-1 (PF15): append-only resolution of payments.settlement_unattributed rows with reason unmapped_source, so a
// close blocked by a charge the system never created can proceed without a red-line DELETE
// (contracts/stripe-platform-account-v1.md §6.6, migrations/0163_settlement_resolve.sql).
// Tier: MOCK (fixture balance-transaction lists, never Stripe) + REAL_PG: the blocking rows are seeded through the
// product sync path (the PF10 unmapped vectors), close and resolve run through stripeadmin.Registrar (what
// cmd/stripe-admin calls), and settlement-resolve also runs as a REAL CLI process against a disposable registrar login
// (the OP09 pattern). Owner-pool SQL is used only to read pins and to prove SQL-level refusals (22023) that the Go layer
// refuses even earlier. No owner-pool fixture writes: nothing is deleted or edited anywhere in this test — that is
// exactly the property under test.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/payments/stripeadmin"
)

const pslResFn = "payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid)"

func TestPlatformSettlementPF15Resolve(t *testing.T) {
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2, w3, w4 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14), pslDay(2026, 9, 21), pslDay(2026, 9, 28)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 } // the fixture rate 0.2564
	resolve := func(txn, resolution, tTenant, tStore, note string) (stripeadmin.ResolvedUnattributed, error) {
		return e.reg.SettlementResolve(ctx, e.op, "SANDBOX", txn, resolution, tTenant, tStore, note, "op@test", pslTicket)
	}

	// ---------------------------------------------------------------------------------------------------------
	// PF15 schema pins: FORCE RLS, append-only grants (never UPDATE/DELETE, never PUBLIC, no merchant or worker
	// role), and the definer's owner/search_path/EXECUTE inventory — plus a LIVE proof that the registrar login
	// itself cannot read the table (only the definer can).
	// ---------------------------------------------------------------------------------------------------------
	t.Run("PF15_schema_privileges", func(t *testing.T) {
		o := e.f.owner
		var rls, force bool
		if err := o.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class
			WHERE oid='payments.settlement_unattributed_resolutions'::regclass`).Scan(&rls, &force); err != nil || !rls || !force {
			t.Fatalf("rls=%v force=%v err=%v", rls, force, err)
		}
		// only the registry writer holds any privilege (superusers aside): no merchant runtime, worker, ingress or checkout role
		over := lcStrings(t, o, `SELECT r.rolname FROM pg_roles r WHERE NOT r.rolsuper AND r.rolname NOT LIKE 'pg\_%' AND r.rolname<>'commerce_payment_registry_writer'
			AND (has_table_privilege(r.oid,'payments.settlement_unattributed_resolutions'::regclass,'SELECT,INSERT,UPDATE,DELETE')
				OR has_any_column_privilege(r.oid,'payments.settlement_unattributed_resolutions'::regclass,'SELECT,INSERT,UPDATE'))
			AND r.rolname<>(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='payments.settlement_unattributed_resolutions'::regclass)`)
		if len(over) != 0 {
			t.Fatalf("resolutions table is reachable by %v", over)
		}
		// append-only: even the owner role has no UPDATE or DELETE, and PUBLIC holds nothing
		var del, upd, pub bool
		if err := o.QueryRow(ctx, `SELECT has_table_privilege('commerce_payment_registry_writer','payments.settlement_unattributed_resolutions'::regclass,'DELETE'),
			has_table_privilege('commerce_payment_registry_writer','payments.settlement_unattributed_resolutions'::regclass,'UPDATE'),
			EXISTS(SELECT 1 FROM pg_class c,aclexplode(c.relacl) a WHERE c.oid='payments.settlement_unattributed_resolutions'::regclass AND a.grantee=0)`).
			Scan(&del, &upd, &pub); err != nil || del || upd || pub {
			t.Fatalf("append-only broken: DELETE=%v UPDATE=%v PUBLIC=%v err=%v", del, upd, pub, err)
		}
		// the definer: SECURITY DEFINER, registry-writer owned, pinned search_path, EXECUTE for the registrar + the owner only
		var secdef bool
		var owner, cfg string
		var acl []string
		if err := o.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),
			coalesce((SELECT array_agg(a.grantee::regrole::text ORDER BY a.grantee::regrole::text) FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE'),'{}')
			FROM pg_proc p WHERE p.oid=$1::regprocedure`, pslResFn).Scan(&secdef, &owner, &cfg, &acl); err != nil {
			t.Fatalf("%s missing: %v", pslResFn, err)
		}
		if !secdef || owner != "commerce_payment_registry_writer" || !strings.Contains(cfg, "search_path=pg_catalog") {
			t.Fatalf("%s: secdef=%v owner=%s cfg=%s", pslResFn, secdef, owner, cfg)
		}
		lcSameSet(t, pslResFn+" EXECUTE grantees", acl, []string{"commerce_payment_registrar", "commerce_payment_registry_writer"})
		overF := lcStrings(t, o, `SELECT r.rolname FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' AND NOT r.rolsuper
			AND r.rolname NOT IN ('commerce_payment_registrar','commerce_payment_registry_writer') AND has_function_privilege(r.oid,$1::regprocedure,'EXECUTE')`, pslResFn)
		if len(overF) != 0 {
			t.Fatalf("%s is executable by %v", pslResFn, overF)
		}
		// live privilege pin: the registrar login (the only role the operator CLI uses) cannot read or write the table directly
		regURL := sstLogin(t, e.f, "commerce_payment_registrar")
		rp, err := pgxpool.New(ctx, regURL)
		if err != nil {
			t.Fatalf("registrar pool: %v", err)
		}
		defer rp.Close()
		var n int
		if err := rp.QueryRow(ctx, `SELECT count(*) FROM payments.settlement_unattributed_resolutions`).Scan(&n); sqlState(err) != "42501" {
			t.Fatalf("registrar direct read: want 42501, got %v (n=%d)", err, n)
		}
	})

	// ---------------------------------------------------------------------------------------------------------
	// Week 1 the way it happens in production: one attributed charge for A plus the two PF10 unmapped vectors (a
	// charge with a payment intent the system never created, a dispute-shaped adjustment Stripe did not classify),
	// a foreign-connection row and an unsupported-type row. Close refuses while any unmapped row is unresolved,
	// and only unmapped_source rows are resolvable.
	// ---------------------------------------------------------------------------------------------------------
	t.Run("PF15_seed_and_refuse", func(t *testing.T) {
		e.sync(t, w0, w1) // close of w1 requires sync coverage of [w0, w2)
		odd2 := pslDispute("txn_PF15Odd2", "dp_pf15b", e.oa3.pi, 100, 26, 0, false, d(w1, 6))
		odd2.ReportingCategory = pslS("dispute_other")
		e.sync(t, w1, w2,
			pslCharge("txn_PF15A1", e.oa1.pi, e.oa1.captured, settleOf(e.oa1.captured), 31, d(w1, 1)),
			pslCharge("txn_PF15Odd", "pi_pf15nevercreated", 2500, 641, 26, d(w1, 5)), // a charge the system never created
			pslCharge("txn_PF15X1", e.ox1.pi, 2500, 641, 30, d(w1, 2)),                // another Stripe account
			pslRaw("txn_PF15Payout", "payout", "payout", -5000, pslI(0), pslS("po_pf15"), d(w1, 3)),
			odd2)
		for txn, want := range map[string]string{"txn_PF15Odd": "unmapped_source", "txn_PF15Odd2": "unmapped_source",
			"txn_PF15X1": "foreign_connection", "txn_PF15Payout": "unsupported_type"} {
			if got := e.unattributed(t, txn); got != want {
				t.Fatalf("%s reason = %s, want %s", txn, got, want)
			}
		}
		// the unresolved unmapped_source rows block the WHOLE environment's close (0150 §6.2)
		_, err := e.closeWeek(w1, "")
		pslWantRefused(t, "unmapped source blocks close", err, "settlement_unattributed")
		// a foreign connection and an unsupported type are not attribution questions: they never block close and are not resolvable
		_, err = resolve("txn_PF15X1", stripeadmin.ResolveNotStoreRevenue, "", "", "another account's money")
		pslWantRefused(t, "foreign_connection is unresolvable", err, "unresolvable_reason")
		_, err = resolve("txn_PF15Payout", stripeadmin.ResolveNotStoreRevenue, "", "", "platform payout")
		pslWantRefused(t, "unsupported_type is unresolvable", err, "unresolvable_reason")
		// a row that does not exist (or exists in the other environment) is unavailable, never silently accepted
		_, err = resolve("txn_PF15Never", stripeadmin.ResolveNotStoreRevenue, "", "", "no such row")
		pslWantRefused(t, "unknown row", err, "unattributed_unavailable")
		// malformed calls are 22023 at the SQL level before anything is read (the Go layer refuses these even earlier;
		// this pins the SQL contract for any other future caller)
		for _, args := range []string{
			`'SANDBOX','txn_PF15Odd','write_off','op@test','` + pslTicket + `','not a store sale',NULL,NULL`,
			`'SANDBOX','bal_PF15Odd','not_store_revenue','op@test','` + pslTicket + `','not a store sale',NULL,NULL`,
			`'SANDBOX','txn_PF15Odd','assigned_to_store','op@test','` + pslTicket + `','to store C',NULL,NULL`,
			`'SANDBOX','txn_PF15Odd','not_store_revenue','op@test','` + pslTicket + `','',NULL,NULL`,
			`'SANDBOX','txn_PF15Odd','not_store_revenue','op@test',NULL,'not a store sale',NULL,NULL`,
			`'PROD','txn_PF15Odd','not_store_revenue','op@test','` + pslTicket + `','not a store sale',NULL,NULL`,
			`'SANDBOX','txn_PF15Odd','not_store_revenue','o','` + pslTicket + `','not a store sale',NULL,NULL`,
			`'SANDBOX','txn_PF15Odd','not_store_revenue','op@test','short','not a store sale',NULL,NULL`,
		} {
			var out string
			err := e.f.owner.QueryRow(ctx, `SELECT payments.record_settlement_resolution($1::uuid,$2::uuid,$3::uuid,`+args+`)::text`,
				e.op.TenantID, e.op.StoreID, e.op.PrincipalID).Scan(&out)
			if sqlState(err) != "22023" {
				t.Fatalf("SQL accepted %s: %v", args, err)
			}
		}
		// a half target pair (tenant without store) is structural too
		var out string
		if err := e.f.owner.QueryRow(ctx, `SELECT payments.record_settlement_resolution($1::uuid,$2::uuid,$3::uuid,'SANDBOX','txn_PF15Odd',
			'assigned_to_store','op@test',$4,'to store C',$5,NULL)::text`,
			e.op.TenantID, e.op.StoreID, e.op.PrincipalID, pslTicket, e.c.f.tenantA).Scan(&out); sqlState(err) != "22023" {
			t.Fatalf("SQL accepted a half target pair: %v", err)
		}
	})

	// ---------------------------------------------------------------------------------------------------------
	// The resolve flow itself: both resolutions, every assigned_to_store target refusal, idempotent replay,
	// payload conflict, and the append-only/audit pins.
	// ---------------------------------------------------------------------------------------------------------
	t.Run("PF15_resolve_flow", func(t *testing.T) {
		// assigned_to_store needs a target, and the target must be enrolled on the platform account (or be the platform store)
		_, err := resolve("txn_PF15Odd", stripeadmin.ResolveAssignedToStore, "", "", "target missing")
		pslWantRefused(t, "assigned_to_store without a target", err, "")
		_, err = resolve("txn_PF15Odd", stripeadmin.ResolveAssignedToStore, randomUUID(), randomUUID(), "unknown store")
		pslWantRefused(t, "unknown target store", err, "unknown_target_store")
		_, err = resolve("txn_PF15Odd", stripeadmin.ResolveAssignedToStore, e.ox1.s.p.f.tenantA, e.ox1.store(), "store of another Stripe account")
		pslWantRefused(t, "foreign target store", err, "unknown_target_store")
		// not_store_revenue must not carry a target (Go-side pairing refusal)
		_, err = resolve("txn_PF15Odd", stripeadmin.ResolveNotStoreRevenue, e.c.f.tenantA, e.storeC, "wrong pairing")
		pslWantRefused(t, "not_store_revenue with a target", err, "")
		// success: the never-created charge is assigned to enrolled store C; v1 RECORDS the assignment, it moves no money
		assign, err := resolve("txn_PF15Odd", stripeadmin.ResolveAssignedToStore, e.c.f.tenantA, e.storeC,
			"Buyer of store C paid through the platform page; owner pays C out of band.")
		if err != nil || assign.Replayed || assign.BalanceTxnID != "txn_PF15Odd" || assign.Resolution != stripeadmin.ResolveAssignedToStore ||
			assign.TargetTenantID != e.c.f.tenantA || assign.TargetStoreID != e.storeC || assign.Operator != "op@test" ||
			assign.Ticket != pslTicket || assign.ResolvedAt == "" {
			t.Fatalf("assigned resolve: %+v %v", assign, err)
		}
		note := "Stripe-side dispute adjustment with an unclassified category; not a store sale."
		writeoff, err := resolve("txn_PF15Odd2", stripeadmin.ResolveNotStoreRevenue, "", "", note)
		if err != nil || writeoff.Replayed || writeoff.TargetTenantID != "" || writeoff.TargetStoreID != "" {
			t.Fatalf("not_store_revenue resolve: %+v %v", writeoff, err)
		}
		// idempotent replay (I02/I06/I20): the stored row comes back, same resolved_at, nothing is written again
		again, err := resolve("txn_PF15Odd2", stripeadmin.ResolveNotStoreRevenue, "", "", note)
		if err != nil || !again.Replayed || again.ResolvedAt != writeoff.ResolvedAt || again.Note != note {
			t.Fatalf("replay: %+v %v", again, err)
		}
		// a different payload for the same balance transaction is a conflict, in either field
		_, err = resolve("txn_PF15Odd2", stripeadmin.ResolveNotStoreRevenue, "", "", "a different note")
		pslWantRefused(t, "changed note", err, "resolution_conflict")
		_, err = e.reg.SettlementResolve(ctx, e.op, "SANDBOX", "txn_PF15Odd2", stripeadmin.ResolveAssignedToStore,
			e.c.f.tenantA, e.storeC, note, "op@test", pslTicket)
		pslWantRefused(t, "changed resolution", err, "resolution_conflict")
		// append-only: both unattributed rows are untouched, one resolution row each, and the replay audited nothing
		for _, txn := range []string{"txn_PF15Odd", "txn_PF15Odd2"} {
			if got := e.unattributed(t, txn); got != "unmapped_source" {
				t.Fatalf("%s was edited: reason = %s", txn, got)
			}
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_unattributed_resolutions`); n != 2 {
			t.Fatalf("%d resolution rows, want 2", n)
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events
			WHERE action='stripe.settlement.resolve' AND details->>'ticket'=$1`, pslTicket); n != 2 {
			t.Fatalf("%d resolve audit rows, want 2 (the replay is silent)", n)
		}
	})

	// ---------------------------------------------------------------------------------------------------------
	// With both rows resolved, close succeeds and the totals are the charge-only week: a resolution is a note,
	// never a line (§6.6 v1). The assigned_to_store resolution is printed as an operator note, again on replay.
	// ---------------------------------------------------------------------------------------------------------
	t.Run("PF15_close_with_notes", func(t *testing.T) {
		res, err := e.reg.SettlementClose(ctx, e.op, "SANDBOX", w1.Format("2006-01-02"), "op@test", "", pslTicket)
		if err != nil {
			t.Fatalf("close after resolve: %v", err)
		}
		if len(res.Statements) != 1 || res.Statements[0].StoreID != e.storeA || res.Statements[0].LineCount != 1 || res.Statements[0].Replayed {
			t.Fatalf("close: %+v", res.Statements)
		}
		if want := e.oa1.captured + pslFee(31, e.oa1.captured, settleOf(e.oa1.captured)); res.Statements[0].NetPayableMinor != want {
			t.Fatalf("net = %d, want %d (the resolutions moved no money)", res.Statements[0].NetPayableMinor, want)
		}
		if len(res.OperatorNotes) != 1 {
			t.Fatalf("operator notes: %+v", res.OperatorNotes)
		}
		n := res.OperatorNotes[0]
		if n.BalanceTxnID != "txn_PF15Odd" || n.TargetTenantID != e.c.f.tenantA || n.TargetStoreID != e.storeC ||
			n.Operator != "op@test" || n.Ticket != pslTicket || !strings.Contains(n.Note, "out of band") {
			t.Fatalf("operator note: %+v", n)
		}
		// a replayed close prints the notes again: they belong to the close result, not to its first run
		again, err := e.reg.SettlementClose(ctx, e.op, "SANDBOX", w1.Format("2006-01-02"), "op@test", "", pslTicket)
		if err != nil || len(again.Statements) != 1 || !again.Statements[0].Replayed || len(again.OperatorNotes) != 1 {
			t.Fatalf("close replay: %+v %v", again, err)
		}
	})

	// ---------------------------------------------------------------------------------------------------------
	// settlement-resolve as the operator really runs it: the built CLI process against a disposable registrar
	// login (the OP09 pattern) — one JSON line, one fixed refusal code, no secret, a real audit row, and the
	// close it unblocks.
	// ---------------------------------------------------------------------------------------------------------
	t.Run("PF15_cli_e2e", func(t *testing.T) {
		// week 2: a charge for C plus one more never-created charge; the resolve happens through the CLI process
		e.sync(t, w2, w3,
			pslCharge("txn_PF15C1", e.oc1.pi, e.oc1.captured, settleOf(e.oc1.captured), 29, d(w2, 1)),
			pslCharge("txn_PF15Cli", "pi_pf15clinever", 2500, 641, 26, d(w2, 2)))
		regURL := sstLogin(t, e.f, "commerce_payment_registrar")
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(t.TempDir(), "stripe-admin")
		build := exec.Command("go", "build", "-o", bin, "./cmd/stripe-admin")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build cmd/stripe-admin: %v\n%s", err, out)
		}
		cfg, _ := pgxpool.ParseConfig(regURL)
		secret := cfg.ConnConfig.Password
		run := func(args ...string) (int, string, string) {
			cmd := exec.Command(bin, args...)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_STRIPE_REGISTRAR_DATABASE_URL=" + regURL}
			var so, se bytes.Buffer
			cmd.Stdout, cmd.Stderr = &so, &se
			code := 0
			var ee *exec.ExitError
			if err := cmd.Run(); errors.As(err, &ee) {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("run %v: %v", args, err)
			}
			if all := so.String() + se.String(); strings.Contains(all, secret) || strings.Contains(all, "postgres://") {
				t.Fatalf("%v leaked the DSN/password: %q", args, all)
			}
			return code, so.String(), se.String()
		}
		ok := func(args ...string) map[string]any {
			t.Helper()
			code, so, se := run(args...)
			var out map[string]any
			if code != 0 || se != "" || strings.Count(so, "\n") != 1 || json.Unmarshal([]byte(so), &out) != nil {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, so, se)
			}
			return out
		}
		fail := func(want string, args ...string) {
			t.Helper()
			if code, so, se := run(args...); code != 1 || so != "" || strings.TrimSpace(se) != want {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q, want exit 1 and %q", args, code, so, se, want)
			}
		}
		scope := []string{"--tenant", e.op.TenantID, "--store", e.op.StoreID, "--principal", e.op.PrincipalID,
			"--operator", "op@test", "--ticket", pslTicket}
		closeArgs := append([]string{"settlement-close", "--environment", "SANDBOX", "--period-start", w2.Format("2006-01-02")}, scope...)
		fail("stripeadmin: rejected: settlement_unattributed", closeArgs...)
		resolveArgs := append([]string{"settlement-resolve", "--environment", "SANDBOX", "--balance-txn", "txn_PF15Cli",
			"--resolution", "not_store_revenue", "--note", "Dispute-shaped adjustment the CLI e2e wrote off; not a store sale."}, scope...)
		out := ok(resolveArgs...)
		if out["environment"] != "SANDBOX" || out["balance_txn"] != "txn_PF15Cli" || out["resolution"] != "not_store_revenue" ||
			out["replayed"] != false || out["resolved_at"] == "" || out["ticket"] != pslTicket || out["target_tenant"] != "" {
			t.Fatalf("resolve JSON: %v", out)
		}
		if again := ok(resolveArgs...); again["replayed"] != true || again["resolved_at"] != out["resolved_at"] {
			t.Fatalf("resolve replay JSON: %v", again)
		}
		// a refusal is the one fixed code on stderr and nothing on stdout (the unresolvable reason check runs first)
		fail("stripeadmin: rejected: unresolvable_reason", append([]string{"settlement-resolve", "--environment", "SANDBOX",
			"--balance-txn", "txn_PF15X1", "--resolution", "not_store_revenue", "--note", "another account's money"}, scope...)...)
		// the close the CLI unblocks: C's charged week plus A's chained empty statement, and week 1's note reprinted
		out = ok(closeArgs...)
		stmts, _ := out["statements"].([]any)
		notes, _ := out["operator_notes"].([]any)
		if len(stmts) != 2 || len(notes) != 1 {
			t.Fatalf("close JSON: %v", out)
		}
		var cStmt map[string]any
		for _, s := range stmts {
			if m, _ := s.(map[string]any); m["store_id"] == e.storeC {
				cStmt = m
			}
		}
		want := float64(e.oc1.captured + pslFee(29, e.oc1.captured, settleOf(e.oc1.captured)))
		if cStmt == nil || cStmt["line_count"] != float64(1) || cStmt["net_payable_minor"] != want {
			t.Fatalf("C statement: %v (want net %v)", cStmt, want)
		}
		if n, _ := notes[0].(map[string]any); n["balance_txn_id"] != "txn_PF15Odd" || n["target_store_id"] != e.storeC || n["ticket"] != pslTicket {
			t.Fatalf("close note: %v", notes[0])
		}
		// the CLI's resolve row and its audit row exist; the CLI replay wrote neither
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_unattributed_resolutions`); n != 3 {
			t.Fatalf("%d resolution rows, want 3", n)
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events
			WHERE action='stripe.settlement.resolve' AND details->>'balance_txn_id'='txn_PF15Cli' AND details->>'ticket'=$1`, pslTicket); n != 1 {
			t.Fatalf("%d CLI resolve audit rows, want 1", n)
		}
	})

	// ---------------------------------------------------------------------------------------------------------
	// MUST RUN LAST: this phase leaves an unresolvable blocking row behind on purpose. A LATE unmapped row synced
	// into an already-closed week fails closed — resolving it would silently rewrite what that close had to
	// refuse, so the remedy is the owner escalation of §9 (red-line SQL), never this tool.
	// ---------------------------------------------------------------------------------------------------------
	t.Run("PF15_closed_period", func(t *testing.T) {
		e.sync(t, w3, w4) // close of w3 requires coverage of [w2, w4)
		if _, err := e.closeWeek(w3, ""); err != nil {
			t.Fatalf("close week 3: %v", err)
		}
		e.sync(t, w3, w4, pslCharge("txn_PF15Late", "pi_pf15late", 2500, 641, 26, d(w3, 2)))
		if got := e.unattributed(t, "txn_PF15Late"); got != "unmapped_source" {
			t.Fatalf("late row reason = %s", got)
		}
		_, err := resolve("txn_PF15Late", stripeadmin.ResolveNotStoreRevenue, "", "", "late never-created charge")
		pslWantRefused(t, "row in a closed period", err, "period_already_closed")
		// replay idempotency survives the period close: an operator re-running the same fix never hits a conflict
		note := "Stripe-side dispute adjustment with an unclassified category; not a store sale."
		again, err := resolve("txn_PF15Odd2", stripeadmin.ResolveNotStoreRevenue, "", "", note)
		if err != nil || !again.Replayed {
			t.Fatalf("replay of a resolved row in a closed period: %+v %v", again, err)
		}
	})
}
