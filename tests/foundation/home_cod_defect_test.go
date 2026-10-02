package foundation_test

// Home-cod R5 review DEFECT tests (unit home-cod-tests). Each test is red on unit/home-cod @ 32a51f2 and encodes the behaviour the
// contract needs; REVIEW-home-cod.md lists the finding id, file:line and failure scenario. They turn green when the integrator fixes
// the defect; they must not be weakened or deleted to merge. Prefix `hcd`. Tier REAL_PG + HTTP_PG.
//
// P1-1 buyer order page never states the cash due (the order DTO has no surcharge)            -> TestHomeCodDefectBuyerOrderAmount
// P1-2 manual-fulfilment export lists COD orders without payment mode or the collect amount   -> TestHomeCodDefectMerchantExportAmount
// P1-3 a shipment void/correction after collection breaks the collected-after-shipped rule
//      and moves the collected cash to another finance day (day = orders.updated_at)         -> TestHomeCodDefectVoidAfterCollected,
//                                                                                              TestHomeCodDefectFinanceDayDrift,
//      erasure (or any other non-record_collection write) must not move the finance day      -> TestHomeCodDefectFinanceDayDriftErasure

import (
	"encoding/csv"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// hcdMoneyKeys collects every numeric value stored under a key that names a surcharge, a collect amount or an amount due.
func hcdMoneyKeys(v any, path string, out map[string]float64) {
	re := regexp.MustCompile(`(?i)surcharge|collect|amount_due|due_minor|cod_`)
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if n, ok := val.(float64); ok && re.MatchString(k) {
				out[path+"/"+k] = n
			}
			hcdMoneyKeys(val, path+"/"+k, out)
		}
	case []any:
		for i, val := range x {
			hcdMoneyKeys(val, path+"/"+strconv.Itoa(i), out)
		}
	}
}

// HCD-P1-1: after placement the buyer is owed an exact statement of the cash due (total + surcharge). The order page showed the total
// only; the surcharge lives on checkout.orders.cod_surcharge_minor, which no buyer DTO reads.
func TestHomeCodDefectBuyerOrderAmount(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50) // NT$50 surcharge, NT$25 basket => NT$75 due at the door
	order, b := e.hcrPlace()
	r := b.req("GET", "/v1/buyer/orders/"+order, "", nil, nil)
	if r.status != 200 {
		t.Fatalf("buyer order read: %d %s", r.status, r.body)
	}
	var doc map[string]any
	if err := json.Unmarshal(r.body, &doc); err != nil {
		t.Fatal(err)
	}
	found := map[string]float64{}
	hcdMoneyKeys(doc, "", found)
	ok := false
	for _, n := range found {
		if n == 5000 || n == 7500 {
			ok = true
		}
	}
	if !ok {
		t.Errorf("P1-1: the buyer order DTO carries neither the surcharge (5000) nor the cash due (7500) under any surcharge/collect/due key; found %v. "+
			"The carrier collects NT$75 while the order page can only show NT$25.", found)
	}
}

// HCD-P1-2: the merchant hands the unshipped export to the carrier (黑貓/新竹 are manual: no carrier API). A COD parcel needs the
// collect amount on its waybill; the export has no payment_mode and its only amount is total_minor (without the surcharge).
func TestHomeCodDefectMerchantExportAmount(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write", "orders:export")
	e.hcrEnable(50)
	order, _ := e.hcrPlace()
	st, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/unshipped.csv", "", "")
	if st != 200 {
		t.Fatalf("unshipped export: %d %s", st, raw)
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\xEF\xBB\xBF"))).ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("export parse: %v %q", err, raw)
	}
	header, row := rows[0], []string(nil)
	for _, r := range rows[1:] {
		if r[0] == order {
			row = r
		}
	}
	if row == nil {
		t.Fatalf("the COD order %s is missing from the unshipped export (header %v)", order, header)
	}
	hasMode := false
	for _, h := range header {
		if h == "payment_mode" || strings.Contains(h, "cod") || strings.Contains(h, "collect") {
			hasMode = true
		}
	}
	hasAmount := false
	for _, c := range row {
		if c == "7500" {
			hasAmount = true
		}
	}
	if !hasMode || !hasAmount {
		t.Errorf("P1-2: export header %v has a payment-mode/COD column=%v and the row carries the cash due 7500 (total 2500 + surcharge 5000)=%v; the carrier "+
			"waybill needs the collect amount and the packer must be able to tell a COD parcel from a prepaid one", header, hasMode, hasAmount)
	}
}

// HCD-P1-3a: record_manual_shipment lets the merchant VOID a shipment after the order was recorded COLLECTED (it checks only the
// shipment head and fulfillment_state). The order is then COLLECTED but MANUAL_UNASSIGNED (never shipped), and order_money_shippable
// (collection_state PENDING) refuses to ship it again: the 0102 "collected only after shipped" guard is defeated after the fact.
func TestHomeCodDefectVoidAfterCollected(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	writer, _ := e.member("fulfillment:write", "orders:read")
	order, _ := e.hcrPlace()
	e.hcrShip(order)
	if r := e.hcrRecord(writer, order, "PENDING", "collected"); r.status != 200 {
		t.Fatalf("collected: %d %s", r.status, r.raw)
	}
	st, _, raw := e.mcall(e.token(), "PUT", e.hcrShipPath(order), t04Key("hcd-void"), mfxVoid(1, "wrong_tracking"))
	_, fulfilment, _, collection, _, _ := e.hcodRow(order)
	if collection != nil && *collection == "COLLECTED" && fulfilment != "MERCHANT_SHIPPED" {
		t.Errorf("P1-3a: after a shipment void (%d %s) the order is COLLECTED with fulfillment_state %s: cash recorded for a parcel that is no longer shipped "+
			"and can never be shipped again", st, raw, fulfilment)
	}
}

// HCD-P1-3b: finance puts a collected COD order on the Taipei day of orders.updated_at ("no collected_at column exists"). Every later
// write to the order row moves it: a tracking-number correction (MD7, allowed at any time after shipping) rewrites updated_at, so the
// cash collected yesterday is reported today and yesterday's finance row silently shrinks.
func TestHomeCodDefectFinanceDayDrift(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	writer, _ := e.member("fulfillment:write", "orders:read")
	order, _ := e.hcrPlace()
	e.hcrShip(order)
	if r := e.hcrRecord(writer, order, "PENDING", "collected"); r.status != 200 {
		t.Fatalf("collected: %d %s", r.status, r.raw)
	}
	tpe := time.FixedZone("TPE", 8*3600)
	now := time.Now().In(tpe)
	y := now.AddDate(0, 0, -1)
	collectedAt := time.Date(y.Year(), y.Month(), y.Day(), 12, 0, 0, 0, tpe)
	// disclosed owner-pool fixture: the collection was recorded yesterday at noon (updated_at and collected_at both say yesterday)
	mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET updated_at=$2, collected_at=$2 WHERE id=$1`, order, collectedAt)
	prev, today := collectedAt.Format("2006-01-02"), now.Format("2006-01-02")
	if n, m := e.hcrFinanceDay(prev, today, prev); n != 1 || m != 7500 {
		t.Fatalf("setup: yesterday's COD row %d/%d, want 1/7500", n, m)
	}
	// the merchant fixes a typo in the tracking number
	st, _, raw := e.mcall(e.token(), "PUT", e.hcrShipPath(order), t04Key("hcd-correct"), mfxShip(1, "sf_express", "FIXED"+t04Tag()))
	if st != 200 && st != 409 && st != 422 {
		t.Fatalf("tracking correction: %d %s", st, raw)
	}
	if n, m := e.hcrFinanceDay(prev, today, prev); n != 1 || m != 7500 {
		t.Errorf("P1-3b: after a tracking correction (%d) yesterday's COD row is %d/%d, want 1/7500; today's row is %d/%d: collected cash moved days",
			st, n, m, e.hcrFinanceCodDay(prev, today, today), e.hcrFinanceCodMinorDay(prev, today, today))
	}
}

// HCD-P1-3b (red-first): the correction no-op closes record_manual_shipment, but any OTHER later write to a collected order's row —
// an erasure anonymising the row is the canonical example — still rewrites updated_at. Finance must group the collected cash on
// collected_at, so the cash stays on the day it was collected. Red on 75931ee (finance groups on updated_at, and collected_at does
// not exist yet); green after collected_at is added and finance groups on it.
func TestHomeCodDefectFinanceDayDriftErasure(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	writer, _ := e.member("fulfillment:write", "orders:read")
	order, _ := e.hcrPlace()
	e.hcrShip(order)
	if r := e.hcrRecord(writer, order, "PENDING", "collected"); r.status != 200 {
		t.Fatalf("collected: %d %s", r.status, r.raw)
	}
	tpe := time.FixedZone("TPE", 8*3600)
	now := time.Now().In(tpe)
	y := now.AddDate(0, 0, -1)
	collectedAt := time.Date(y.Year(), y.Month(), y.Day(), 12, 0, 0, 0, tpe)
	// disclosed owner-pool fixture: the collection was recorded yesterday at noon (updated_at and collected_at both say yesterday)
	mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET updated_at=$2, collected_at=$2 WHERE id=$1`, order, collectedAt)
	prev, today := collectedAt.Format("2006-01-02"), now.Format("2006-01-02")
	if n, m := e.hcrFinanceDay(prev, today, prev); n != 1 || m != 7500 {
		t.Fatalf("setup: yesterday's COD row %d/%d, want 1/7500", n, m)
	}
	// a later write to the order row that is not record_collection (an erasure anonymising the row rewrites updated_at only)
	mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET updated_at=$2 WHERE id=$1`, order, now)
	if n, m := e.hcrFinanceDay(prev, today, prev); n != 1 || m != 7500 {
		t.Errorf("P1-3b: after an erasure-style row rewrite yesterday's COD row is %d/%d, want 1/7500; today's row is %d/%d: collected cash moved days",
			n, m, e.hcrFinanceCodDay(prev, today, today), e.hcrFinanceCodMinorDay(prev, today, today))
	}
}

func (e *tcvEnv) hcrFinanceCodDay(from, to, day string) int64 {
	n, _ := e.hcrFinanceDay(from, to, day)
	return n
}

func (e *tcvEnv) hcrFinanceCodMinorDay(from, to, day string) int64 {
	_, m := e.hcrFinanceDay(from, to, day)
	return m
}
