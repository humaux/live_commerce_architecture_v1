// Package notify owns the buyer and merchant notification mail of an order (contracts/storefront-v2.md §E): the pure renderer of the six
// mail kinds in zh-TW / zh-CN / en, the polling Worker that drains the notify.outbox table, and the merchant opt-out toggle for the
// new-order mail.
//
// It never decides when a mail is due (the SQL triggers of migration 0090 enqueue inside the order transaction; Go only reads claimed rows),
// never sends inside an order transaction, never retries an uncertain SMTP outcome (SMTP has no idempotency key, I06: mail.ErrUnknown is
// recorded UNKNOWN and left alone), never stores a body or a recipient address (only a hash, cleared by erasure), never embeds remote
// images or tracking links, and never reads the daily mail budget of the login codes (separate ledger).
//
// SQL touched, all through definers of commerce_checkout_writer: notify.claim_batch and notify.record_result (commerce_expiry_worker pool),
// notify.read_store_settings / notify.set_store_settings (merchant request transaction). External host: the configured SMTP server only,
// through internal/mail (the same sender as password auth and staff invitations).
package notify
