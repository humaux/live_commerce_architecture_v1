package foundation_test

// TI smoke (contracts/manual-fulfilment-v1.md Amendment "M-7 revoked", unit w3-01b): the bulk
// home-delivery tracking import over the real HTTP handler and REAL_PG — preview rolls back, commit
// ships each row through the same RecordShipment command/audit as the single PUT, the per-file-hash
// idempotency replays/conflicts, a stale preview count is refused with a fresh preview, and the
// per-row result.csv is downloadable. This is the author's smoke subset of TI01–TI15, not acceptance.

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/command"
)

// tiFile renders the exact CSV the importer accepts: the three required columns plus a trailing LF.
func tiFile(rows ...string) string {
	return "order_number,carrier,tracking_number\n" + strings.Join(rows, "\n") + "\n"
}

func tiRow(order, carrier, tracking string) string {
	return order + "," + carrier + "," + tracking
}

// tiCall POSTs text/csv to one tracking-import route with the merchant token.
func (e *rfxEnv) tiCall(method, path, token, body string) (int, []byte, string) {
	hdr := map[string]string{}
	if body != "" {
		hdr["Content-Type"] = "text/csv"
	}
	status, raw, resp := e.call(method, path, token, hdr, body)
	return status, raw, resp.Get("Content-Type")
}

func TestTrackingImportSmoke(t *testing.T) {
	e := rfxNew(t)
	e.startWorker(t)
	base := e.storeFor(t)
	o1 := e.payMore(t, base)
	o2 := e.payMore(t, base)
	o3 := e.payMore(t, base)
	o4 := e.payMore(t, base)
	for _, o := range []rfxOrder{o1, o2, o3, o4} {
		e.await(t, "capture jobs settled", o.attempt, 45*time.Second, `SELECT NOT EXISTS(SELECT 1 FROM river_payment.river_job WHERE args->>'operation_id'=$1 AND state NOT IN ('completed','cancelled','discarded'))`)
	}
	path := "/v1/admin/stores/" + o1.store() + "/shipments/tracking-import"

	t.Run("preview rolls back, commit ships, replay/conflict and result.csv", func(t *testing.T) {
		file := tiFile(tiRow(o1.order, "black_cat", "0012345678"), tiRow(o2.order, "chunghwa_post", "RA123456789TW"))
		sum := sha256.Sum256([]byte(file))

		status, raw, ctype := e.tiCall("POST", path+"/preview", o1.token(), file)
		if status != 200 {
			t.Fatalf("preview answered %d: %s", status, raw)
		}
		pv := sraJSON(raw)
		if pv["file_sha256"] != hex.EncodeToString(sum[:]) {
			t.Fatalf("file_sha256: %v", pv["file_sha256"])
		}
		if pv["rows_total"] != float64(2) || pv["apply_rows"] != float64(2) || pv["unchanged_rows"] != float64(0) ||
			pv["failed_rows"] != float64(0) || pv["mail_eta_hours"] != float64(1) {
			t.Fatalf("preview totals: %v", pv)
		}
		rows := pv["rows"].([]any)
		if len(rows) != 2 {
			t.Fatalf("preview rows: %v", rows)
		}
		for i, want := range []string{o1.order, o2.order} {
			m := rows[i].(map[string]any)
			if m["outcome"] != "apply" || m["order_number"] != want {
				t.Fatalf("preview row %d: %v", i, m)
			}
		}
		// A preview writes nothing (its transaction always rolls back).
		if e.mfxVersions(t, o1) != 0 || e.mfxVersions(t, o2) != 0 {
			t.Fatal("preview wrote shipment versions")
		}
		if e.mfxAudit(t, o1, "fulfillment.shipment_recorded") != 0 || e.mfxAudit(t, o2, "fulfillment.shipment_recorded") != 0 {
			t.Fatal("preview wrote a shipment audit row")
		}

		status, raw, _ = e.tiCall("POST", path+"/commit?expected_apply_rows=2", o1.token(), file)
		if status != 200 {
			t.Fatalf("commit answered %d: %s", status, raw)
		}
		cm := sraJSON(raw)
		batchID, _ := cm["batch_id"].(string)
		if !command.ValidID(batchID) || cm["applied"] != float64(2) || cm["unchanged"] != float64(0) ||
			cm["failed"] != float64(0) || cm["replayed"] != false {
			t.Fatalf("commit body: %v", cm)
		}
		for _, o := range []rfxOrder{o1, o2} {
			if got := e.mfxFulfilmentState(t, o); got != "MERCHANT_SHIPPED" {
				t.Fatalf("order %s fulfilment_state %s", o.order, got)
			}
			if e.mfxVersions(t, o) != 1 {
				t.Fatalf("order %s versions %d", o.order, e.mfxVersions(t, o))
			}
		}
		// shipment_recorded is store-scoped (mfxAudit counts by store + action): two applied rows, two audits.
		if got := e.mfxAudit(t, o1, "fulfillment.shipment_recorded"); got != 2 {
			t.Fatalf("shipment audit rows %d (want 2, one per applied order)", got)
		}
		if e.mfxAudit(t, o1, "fulfillment.tracking_imported") != 1 {
			t.Fatal("commit wrote no fulfillment.tracking_imported audit row")
		}

		// Re-submit the same bytes + same count: replay, never re-ship.
		status, raw, _ = e.tiCall("POST", path+"/commit?expected_apply_rows=2", o1.token(), file)
		if status != 200 {
			t.Fatalf("replay answered %d: %s", status, raw)
		}
		re := sraJSON(raw)
		if re["batch_id"] != batchID || re["replayed"] != true || re["applied"] != float64(2) {
			t.Fatalf("replay body: %v", re)
		}
		if e.mfxVersions(t, o1) != 1 || e.mfxAudit(t, o1, "fulfillment.tracking_imported") != 1 {
			t.Fatal("replay double-applied")
		}

		// Same bytes, different count: idempotency conflict, never re-ship.
		status, raw, _ = e.tiCall("POST", path+"/commit?expected_apply_rows=1", o1.token(), file)
		if status != 409 || srqCode(sraJSON(raw)) != "idempotency_conflict" {
			t.Fatalf("idempotency conflict: %d %s", status, raw)
		}

		// The now-shipped rows re-preview as unchanged (same carrier/tracking), not as apply.
		status, raw, _ = e.tiCall("POST", path+"/preview", o1.token(), file)
		if status != 200 {
			t.Fatalf("re-preview answered %d: %s", status, raw)
		}
		pv2 := sraJSON(raw)
		if pv2["apply_rows"] != float64(0) || pv2["unchanged_rows"] != float64(2) || pv2["failed_rows"] != float64(0) {
			t.Fatalf("re-preview totals: %v", pv2)
		}

		// result.csv: BOM, header, both order ids, exactly two applied rows, private CSV content type.
		status, raw, ctype = e.tiCall("GET", path+"/"+batchID+"/result.csv", o1.token(), "")
		if status != 200 || ctype != "text/csv; charset=utf-8" {
			t.Fatalf("result.csv answered %d ctype %q: %s", status, ctype, raw)
		}
		body := string(raw)
		if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
			t.Fatal("result.csv has no UTF-8 BOM")
		}
		body = strings.TrimPrefix(body, "\xEF\xBB\xBF")
		if !strings.Contains(body, "row,order_number,carrier,tracking_number,outcome,code") ||
			!strings.Contains(body, o1.order) || !strings.Contains(body, o2.order) ||
			strings.Count(body, ",apply,") != 2 {
			t.Fatalf("result.csv body: %q", body)
		}
		// ?only=failed keeps just the failed rows: none here, so only the header remains.
		status, raw, _ = e.tiCall("GET", path+"/"+batchID+"/result.csv?only=failed", o1.token(), "")
		if status != 200 || strings.Count(strings.TrimPrefix(string(raw), "\xEF\xBB\xBF"), ",apply,") != 0 {
			t.Fatalf("result.csv?only=failed answered %d: %q", status, raw)
		}
	})

	t.Run("stale preview, nothing to apply, row-level failures", func(t *testing.T) {
		// o3 alone applies once; committing it with the wrong count refuses 409 preview_stale and writes nothing.
		file3 := tiFile(tiRow(o3.order, "hsinchu", "HS0001"))
		status, raw, _ := e.tiCall("POST", path+"/commit?expected_apply_rows=2", o3.token(), file3)
		if status != 409 {
			t.Fatalf("stale commit answered %d: %s", status, raw)
		}
		stale := sraJSON(raw)
		if stale["apply_rows"] != float64(1) || stale["rows_total"] != float64(1) {
			t.Fatalf("stale body must carry the fresh preview: %v", stale)
		}
		if e.mfxVersions(t, o3) != 0 {
			t.Fatal("a stale commit wrote shipment versions")
		}

		// A file with no applicable rows commits nothing (422 nothing_to_apply).
		unknown := "00000000-0000-0000-0000-000000000000"
		file5 := tiFile(tiRow(unknown, "black_cat", "X1"))
		status, raw, _ = e.tiCall("POST", path+"/commit?expected_apply_rows=0", o3.token(), file5)
		if status != 422 || srqCode(sraJSON(raw)) != "nothing_to_apply" {
			t.Fatalf("nothing_to_apply answered %d: %s", status, raw)
		}

		// One valid row plus one unknown order and one invalid carrier: apply 1, fail 2, and the commit keeps only the valid row.
		file4 := tiFile(tiRow(o3.order, "hsinchu", "HS0001"), tiRow(unknown, "black_cat", "X2"), tiRow(o4.order, "sf_express", "SF0001"))
		status, raw, _ = e.tiCall("POST", path+"/preview", o3.token(), file4)
		if status != 200 {
			t.Fatalf("failure preview answered %d: %s", status, raw)
		}
		fp := sraJSON(raw)
		if fp["apply_rows"] != float64(1) || fp["failed_rows"] != float64(2) || fp["rows_total"] != float64(3) {
			t.Fatalf("failure preview totals: %v", fp)
		}
		status, raw, _ = e.tiCall("POST", path+"/commit?expected_apply_rows=1", o3.token(), file4)
		if status != 200 {
			t.Fatalf("failure commit answered %d: %s", status, raw)
		}
		fcm := sraJSON(raw)
		fBatch, _ := fcm["batch_id"].(string)
		if !command.ValidID(fBatch) || fcm["applied"] != float64(1) || fcm["failed"] != float64(2) {
			t.Fatalf("failure commit body: %v", fcm)
		}
		if got := e.mfxFulfilmentState(t, o3); got != "MERCHANT_SHIPPED" {
			t.Fatalf("valid row did not ship: %s", got)
		}
		if got := e.mfxFulfilmentState(t, o4); got == "MERCHANT_SHIPPED" {
			t.Fatalf("invalid-carrier row shipped: %s", got)
		}
		// ?only=failed returns exactly the two failed rows with their codes.
		status, raw, _ = e.tiCall("GET", path+"/"+fBatch+"/result.csv?only=failed", o3.token(), "")
		if status != 200 {
			t.Fatalf("failure result.csv answered %d: %s", status, raw)
		}
		fbody := strings.TrimPrefix(string(raw), "\xEF\xBB\xBF")
		if strings.Count(fbody, ",failed,") != 2 || !strings.Contains(fbody, "order_not_found") || !strings.Contains(fbody, "invalid_carrier") {
			t.Fatalf("failure result.csv body: %q", fbody)
		}
	})
}
