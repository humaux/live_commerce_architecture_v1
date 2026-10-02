# home-cod-ui backend boundaries

Verified against base `0c6d965c` (not just the older backend SUMMARY).

## Historical carrier (BLOCKED)

`checkout.orders.cod_carrier` persists the chosen carrier, but `checkout.Order` and the buyer `orderResponse` do not expose it. Current checkout options are not a substitute for the order-time snapshot.

Needed in backend: read the persisted field under existing buyer scope and add a COD-only allowlisted projection. UI currently displays 黑貓 / 新竹 in checkout and settings; existing order shipment renders only the merchant's recorded manual carrier.

## Manual carrier Go allowlist (BLOCKED)

`migrations/0107_home_cod.sql` extends the SQL carrier enum with `black_cat` / `hsinchu`, but `internal/merchantorders/shipments.go:47` still omits both from the input AND output validation list (used at lines 110 and 270).

Needed in backend: align Go write/read validation and tests with the SQL contract. Until then, the UI does not send unsupported codes. No made-up API integration or current-settings historical substitution.

## Not a blocker

The backend SUMMARY's old `updated_at` finance-day deviation was corrected by starting HEAD `0c6d965c`, which adds `collected_at`. This unit does not claim that obsolete issue is still open.

No Go, SQL, deployment, credentials or production changes were made here.
