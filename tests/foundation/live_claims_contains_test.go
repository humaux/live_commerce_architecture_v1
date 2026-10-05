// A7 author smoke test (DeepSeek V4-Pro) for the KEYWORD_QTY_CONTAINS "contains KW+N"
// restricted match mode (R5). Evidence label: REAL_PG, MODEL_ONLY (no Meta wire, no UI;
// probes run as the superuser owner and every statement rolls back, as KC04 does).
//
// Owns: the schema-level acceptance of migration 0115 — the widened claim-window/event
// match_mode and grammar_version CHECKs, the QUANTITY_REQUIRED widening, the
// kwc-v1-in-CONTAINS cross-check, the per-interval match_mode column written by the
// window-history trigger, and the p_version argument on claims.insert_meta_intake. It does
// NOT reuse checkIngest (which hardcodes 'kw-v1'): every assertion here is its own.
//
// Non-goals: no Go-ingest flow assertions (that is KC06-KC08's job and uses the kw-v1
// fixtures), no grammar vectors (internal/claims/grammar), no Meta staging flow.
package foundation_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

// eventCols is the fixed column order of the A7 event probes. bundle_id/bundle_version/
// line_version/previous_quantity are omitted: every A7 probe is a REJECTED row (no bundle,
// no line), exactly the shape a QUANTITY_REQUIRED contains-head produces.
var eventCols = []string{"tenant_id", "store_id", "session_id", "window_generation", "source_kind",
	"source_event_id", "platform", "occurred_at", "grammar_version", "grammar_kind", "match_mode",
	"outcome", "reason", "offer_id", "quantity", "explicit_quantity", "principal_id"}

type evRow map[string]any

func TestLiveA7ContainsMode(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	ctx := context.Background()
	a, st, actor := f.tenantA, f.storeA1, h.actor
	sku0 := h.stock.skus[0].ID

	s := h.draft(t, f.storeA1) // live DRAFT session; no window row yet (h.draft never opens one)
	offerID := randomUUID()
	now := time.Now().UTC().Truncate(time.Microsecond)

	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	// 1. claim window: the new CONTAINS mode is accepted (this also arms the trigger below).
	win := `INSERT INTO live.claim_windows(tenant_id,store_id,session_id,state,match_mode,generation,opened_at,principal_id)
		VALUES($1,$2,$3,'OPEN','KEYWORD_QTY_CONTAINS',1,$4,$5)`
	if _, err := tx.Exec(ctx, win, a, st, s, now, actor); err != nil {
		t.Fatalf("open KEYWORD_QTY_CONTAINS window: %v", err)
	}

	// 2. the window-history trigger recorded the interval's match_mode (comment-time fidelity).
	var intervalMode string
	if err := tx.QueryRow(ctx, `SELECT match_mode FROM live.claim_window_intervals WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
		a, st, s).Scan(&intervalMode); err != nil {
		t.Fatalf("read interval match_mode: %v", err)
	}
	if intervalMode != "KEYWORD_QTY_CONTAINS" {
		t.Fatalf("interval match_mode=%q, want KEYWORD_QTY_CONTAINS", intervalMode)
	}

	// 3. an offer so a kwc-v1 MATCH head can resolve (the only way a QUANTITY_REQUIRED event persists).
	if _, err := tx.Exec(ctx, `INSERT INTO live.offers(tenant_id,store_id,id,session_id,keyword,sku_id,max_quantity_per_claim,active,version,principal_id)
		VALUES($1,$2,$3,$4,'A1',$5,5,true,1,$6)`, a, st, offerID, s, sku0, actor); err != nil {
		t.Fatalf("offer: %v", err)
	}

	base := evRow{
		"tenant_id": a, "store_id": st, "session_id": s, "window_generation": 1,
		"source_kind": "manual", "source_event_id": randomUUID(), "platform": "manual",
		"occurred_at": now, "grammar_version": "kwc-v1", "grammar_kind": "MATCH",
		"match_mode": "KEYWORD_QTY_CONTAINS", "outcome": "REJECTED", "reason": "QUANTITY_REQUIRED",
		"offer_id": offerID, "quantity": 1, "explicit_quantity": false, "principal_id": actor,
	}
	probe := func(want, label string, over evRow) {
		t.Helper()
		row := evRow{}
		for k, v := range base {
			row[k] = v
		}
		for k, v := range over {
			row[k] = v
		}
		args := make([]any, 0, len(eventCols))
		marks := make([]string, 0, len(eventCols))
		for i, c := range eventCols {
			args = append(args, row[c])
			marks = append(marks, fmt.Sprintf("$%d", i+1))
		}
		lcProbe(t, tx, want, "event "+label,
			`INSERT INTO claims.events(`+strings.Join(eventCols, ",")+`) VALUES(`+strings.Join(marks, ",")+`)`, args...)
	}

	// 4. events: kwc-v1 only ever lands in a CONTAINS window (the new cross-check), and the
	//    QUANTITY_REQUIRED reason now also covers CONTAINS (still explicit=false only).
	probe("", "kwc-v1 CONTAINS QUANTITY_REQUIRED", nil)
	probe("23514", "kwc-v1 in EXACT", evRow{"match_mode": "EXACT"})
	probe("23514", "kwc-v1 in KEYWORD_QTY_ONLY", evRow{"match_mode": "KEYWORD_QTY_ONLY"})
	probe("23514", "kwc-v1 QUANTITY_REQUIRED explicit", evRow{"explicit_quantity": true})
	probe("23514", "kw-v2 grammar", evRow{"grammar_version": "kw-v2"})
	probe("23514", "bare CONTAINS mode", evRow{"grammar_version": "kw-v1", "match_mode": "CONTAINS"})
	// regression: the two pre-existing modes are unchanged.
	probe("", "kw-v1 KEYWORD_QTY_ONLY QUANTITY_REQUIRED", evRow{"grammar_version": "kw-v1", "match_mode": "KEYWORD_QTY_ONLY"})
	probe("23514", "kw-v1 EXACT QUANTITY_REQUIRED", evRow{"grammar_version": "kw-v1", "match_mode": "EXACT"})

	// 5. claims.meta_intake.grammar_version CHECK now admits kwc-v1 (staging stores p_version).
	var intakeCheck string
	if err := tx.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid='claims.meta_intake'::regclass AND conname='meta_intake_grammar_version_check'`).Scan(&intakeCheck); err != nil {
		t.Fatalf("read meta_intake grammar_version CHECK: %v", err)
	}
	if !strings.Contains(intakeCheck, "'kwc-v1'") {
		t.Fatalf("meta_intake grammar_version CHECK not widened: %s", intakeCheck)
	}

	// 6. claims.insert_meta_intake carries p_version: kwc-v1 passes validation (no matching
	//    source -> NULL), kw-v2 is rejected with 22023 before any lookup.
	actorKey := hex.EncodeToString(randomBytes(32))
	callIntake := func(version string) error {
		var staged *string
		return tx.QueryRow(ctx, `SELECT claims.insert_meta_intake($1::uuid,$2::uuid,$3::uuid,$4::timestamptz,
			'123','page','123','123_45','456_78',$5::text,$6::timestamptz,'NO_MATCH',NULL::text,NULL::integer,NULL::boolean,false,$7::text)::text`,
			a, st, randomUUID(), now, actorKey, now, version).Scan(&staged)
	}
	if err := callIntake("kwc-v1"); err != nil {
		t.Fatalf("insert_meta_intake kwc-v1 (no source) returned error, want NULL: %v", err)
	}
	if err := callIntake("kw-v2"); sqlState(err) != "22023" {
		t.Fatalf("insert_meta_intake kw-v2 sqlstate=%q (%v), want 22023", sqlState(err), err)
	}
}
