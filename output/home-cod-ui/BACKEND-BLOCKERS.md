# home-cod-ui backend boundaries

**CLOSED 2026-10-02**: backend `e3ab330` is an ancestor of this closeout base `1735195d`. Buyer DTO now carries the persisted `cod_carrier`; the Go manual-shipment allowlist accepts `black_cat` and `hsinchu`. UI commits `e8bfeaab` / `f6bf3c95` consume that snapshot and label it explicitly as the carrier at checkout. The final-source COD browser gate passed with an actual isolated-PG order shipped using `black_cat`, then read back by the buyer. No backend work remains for these two findings.

The following is the historical finding at base `0c6d965c`, retained for traceability; it is **not current status**.

## Historical carrier (previously BLOCKED; now CLOSED)

`checkout.orders.cod_carrier` persists the chosen carrier, but `checkout.Order` and the buyer `orderResponse` do not expose it. Current checkout options are not a substitute for the order-time snapshot.

Needed in backend: read the persisted field under existing buyer scope and add a COD-only allowlisted projection. UI currently displays 黑貓 / 新竹 in checkout and settings; existing order shipment renders only the merchant's recorded manual carrier.

## Manual carrier Go allowlist (previously BLOCKED; now CLOSED)

`migrations/0107_home_cod.sql` extends the SQL carrier enum with `black_cat` / `hsinchu`, but `internal/merchantorders/shipments.go:47` still omits both from the input AND output validation list (used at lines 110 and 270).

Needed in backend: align Go write/read validation and tests with the SQL contract. Until then, the UI does not send unsupported codes. No made-up API integration or current-settings historical substitution.

## Not a blocker

The backend SUMMARY's old `updated_at` finance-day deviation was corrected by starting HEAD `0c6d965c`, which adds `collected_at`. This unit does not claim that obsolete issue is still open.

No Go, SQL, deployment, credentials or production changes were made here.
