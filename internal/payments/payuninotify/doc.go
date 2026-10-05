// Purpose: package doc of the PAYUNi NotifyURL admission package (verify, record receipt, wake the existing query job).
// Depends on: none at runtime (documentation only).
// Used by: handler.go and inbox.go of this package; cmd/api/payuni_notify.go.
// Invariants: a notification never writes a payment/stock fact (payment-capture-v1); never LIVE.

// Package payuninotify owns admission of signed PAYUNi NotifyURL callbacks for one registered
// endpoint token. A notification is only a trigger to query: the handler verifies the callback
// signature with the connection's decrypted HashKey/HashIV, then one definer transaction records
// an idempotent receipt and wakes the attempt's existing payment_query_v1 job before any ACK. It
// never writes a payment/stock fact, never reads the merchant API key for anything but signature
// verification, and never trusts an unsigned field for tenant/connection selection.
//
// It admits profiles PROVIDER_MOCK and SANDBOX only; LIVE is rejected by NewInbox and by cmd/api
// (contracts/payuni-wire-v1.md amendment W4-01B: never LIVE). An endpoint of another profile is
// answered with the fixed 404, and an unknown or disabled token is invisible (404).
//
// Transport, Content-Type and ACK semantics are officially unverified (payuni-wire :19-21), so the
// success ACK is HTTP 200 with an empty body and is flagged EVIDENCE_GAP in the unit delivery.
package payuninotify
