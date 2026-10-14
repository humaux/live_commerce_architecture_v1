// Command claims-worker owns the T10c claims host (meta-claims-intake-v1 §5.3, IR-13): it runs the
// claims intake poller (internal/claimsintake) and the main-schema external_operation_v1 River
// worker whose only routes are the Meta private replies (internal/integrations/metareply), the merchant-disconnect unsubscribe
// jobs (metareply.Unsubscriber, migration 0100: one best-effort DELETE /{page-id}/subscribed_apps per disconnected Page),
// and the inbox resubscribe jobs (metareply.Resubscriber, migration 0119: one best-effort POST /{page-id}/subscribed_apps
// subscribed_fields=feed,messages per already-connected Page, the live-console inbox read side's subscription).
//
// It serves exactly one internal HTTP listener when COMMERCE_CLAIMS_CONSOLE_ADDR is set (live-console-v1
// §2.3, unit LC-B2): the API↔worker comment bridge (POST /internal/v1/comment-page and /comment-facts,
// shared 32-byte bearer, backend Docker network only) plus the comment poller. It never reads the
// claims actor key K_actor (owned by cmd/meta-worker) or any Stripe variable. OA1 prepares notifications
// through the existing outbox and dispatch routes. The process never
// prints a key, token, DSN or driver error (one fixed error string per failure class).
//
// External: graph.facebook.com only (private replies via internal/integrations/metareply; never in tests).
//
// Environment: COMMERCE_CLAIMS_WORKER_ENABLED (""/0 off, 1 on), COMMERCE_CLAIMS_INTAKE_DATABASE_URL,
// COMMERCE_WORKER_DATABASE_URL (same database), COMMERCE_RETENTION_JOB_DATABASE_URL (lc_retention_job, same
// database: the hourly claims_retention_v1 purge, internal/retention), COMMERCE_CLAIMS_REPLY_LINK_KEY (std base64, 32 bytes),
// COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID + COMMERCE_META_PAGE_TOKEN_KEYS_JSON (meta-page-token-v1, AES), optional
// COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS (_FILE) (meta-page-token-v2 HPKE private ring: opens the Page tokens the merchant connect
// sealed to the public ring; only this process holds it), COMMERCE_META_GRAPH_VERSION
// (required, no default: probe U5), optional COMMERCE_META_GRAPH_BASE_URL (default https://graph.facebook.com;
// loopback http://127.0.0.1:<port> for MOCK) and COMMERCE_META_GRAPH_AUTH_HEADER (""/0 token in JSON body, 1 Bearer).
// Live-console comment bridge (LC-B2): optional COMMERCE_CLAIMS_CONSOLE_ADDR (a TCP listen address like
// ":8081"; empty = bridge off), and, when set, COMMERCE_CLAIMS_BRIDGE_TOKEN (std base64, 32 bytes, shared with
// cmd/api's BridgeClient) and COMMERCE_CLAIMS_CURSOR_KEY (std base64, 32 bytes, the worker's older_cursor HMAC key).
// The meta connection-health probe (contract meta-connection-health-v1 §4) additionally reads
// COMMERCE_META_PAGE_APP_ID (required: the app id P3 matches against), COMMERCE_META_ADVANCED_ACCESS (comma list of
// permissions with Advanced Access, default empty) and COMMERCE_META_DM_RECEIVER_CONFIRMED ("1" closes LC-U11, default off).
// ECPay CVS route (taiwan-cvs-logistics-v1 §12, registered only when CVS_ECPAY_ENABLED=1): CVS_ECPAY_ENABLED,
// CVS_ECPAY_LIVE_CREATE, COMMERCE_CVS_HOOKS_ORIGIN, and, only when enabled, COMMERCE_PAYMENT_PROFILE (one ECPay
// environment per deployment) and ECPAY_LOGISTICS_KEYRING. The worker calls logistics(-stage).ecpay.com.tw through
// internal/integrations/shipping/ecpay.
// OA1 is separately opt-in: LC_ORDER_ATTEMPT_ENABLED (""/0 off, 1 on). When enabled, it requires
// LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY + LC_ORDER_ATTEMPT_PLAN_KEYS and LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY
// + LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS (separate JSON keyrings with standard-base64 32-byte keys),
// COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID + COMMERCE_META_PAYLOAD_KEYS_JSON for notification body encryption.
// and COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID + COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON for dispatch sealing.
// Disabled OA1 reads none of these keyrings. Enabled OA1 registers order_attempt_v1 and the 30-second
// order_attempt_sweep_v1 recovery job on the claims queue, sharing the physically verified worker pool.
package main
