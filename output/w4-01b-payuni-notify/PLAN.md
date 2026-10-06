# W4-01B payuni-notify plan

1. Wire: add `payuni.NewNotify` (notify-only client) + bounded `AuthenticateNotification` (parse+authenticate, return inner MerTradeNo/amount/status; no business projection) in `client.go`; reuse `VerifyNotification` for amount/currency/status check.
2. Migration `0136_payuni_notify.sql`: role `commerce_payuni_ingress`; tables `payuni_notify_endpoints`, `payuni_notify_receipts` (RLS, no login grants); definers `payuni_resolve_endpoint` (scope+current/prev credential refs) and `payuni_record_notify` (dedup, map MerTradeNo→attempt, outcome queued/duplicate/mismatch/unknown_trade, review case, River InsertTx) owned by `commerce_payment_registry_writer`, EXECUTE ingress only; post-river river_payment grants + migrate.go UPDATE(kind).
3. Go `internal/payments/payuninotify/`: handler (route, body ≤8KiB, form/content-type checks, token hash), inbox/verify (Keyring decrypt current+prev, VerifyNotification), record (one tx receipt + payment_query_v1 with reason:notify). ACK 200 empty body (EVIDENCE_GAP).
4. `internal/platform`: `OpenPayuniIngressPool`/`ValidatePayuniIngressPool` + payuni_ingress authority in validatePoolAuthority.
5. `cmd/api`: mount `POST /v1/hooks/payuni/notify/{endpoint_token}` + `COMMERCE_PAYUNI_INGRESS_DATABASE_URL`; DB-free full-router test.
6. Tests: unit (payuninotify handler/inbox, golden MOCK vector), foundation ACL-pin `payuni_notify_authority_test.go` (+worker_authority_split/hosted additions), httperror table.
7. Red→green per slice; commit each; DELIVERY.md.
