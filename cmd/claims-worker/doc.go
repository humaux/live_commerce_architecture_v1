// Command claims-worker is the T10c claims host (meta-claims-intake-v1 §5.3, IR-13): it runs the
// claims intake poller (internal/claimsintake) and the main-schema external_operation_v1 River
// worker whose only routes are the Meta private replies (internal/integrations/metareply).
//
// It never serves HTTP, never reads the Meta payload keyring, K_actor (COMMERCE_CLAIMS_ACTOR_KEY) or any
// STRIPE_* variable, never sends anything but the first private reply of a new claim bundle, and never
// prints a key, token, DSN or driver error (one fixed error string per failure class).
//
// Depends on:
//   - internal/claimsintake: Poller (commerce_claims_intake login: apply intake rows, plan replies).
//   - internal/integrations/metareply: Routes, LoadPageTokenKeyring (dispatcher routes and Page-token keyring;
//     talks to graph.facebook.com only, never in tests).
//   - internal/integrations/core: NewDispatcher (the external_operation_v1 River worker, commerce_worker login).
//   - internal/jobqueue: Run (River start/stop and the readiness witness).
//   - internal/platform: OpenClaimsIntakePool, OpenWorkerPool (authority-validated pools).
//   - internal/claims: ReplyLinkKey (K_link).
//
// Environment: COMMERCE_CLAIMS_WORKER_ENABLED (""/0 off, 1 on), COMMERCE_CLAIMS_INTAKE_DATABASE_URL,
// COMMERCE_WORKER_DATABASE_URL (same database), COMMERCE_CLAIMS_REPLY_LINK_KEY (std base64, 32 bytes),
// COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID + COMMERCE_META_PAGE_TOKEN_KEYS_JSON, COMMERCE_META_GRAPH_VERSION
// (required, no default: probe U5), optional COMMERCE_META_GRAPH_BASE_URL (default https://graph.facebook.com;
// loopback http://127.0.0.1:<port> for MOCK) and COMMERCE_META_GRAPH_AUTH_HEADER (""/0 token in JSON body, 1 Bearer).
//
// Used by: operators and deploy manifests (one instance per environment); tests/foundation builds and
// kills it in the MCI04 crash gate.
package main
