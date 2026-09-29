// Package metareply owns the first Meta private reply of a keyword-claim bundle
// (meta-claims-intake-v1 §6.3, §7): the dispatcher routes (facebook|instagram, meta.private_reply,
// service), the per-store Page-token custody (AES-256-GCM seal/open and the registrar call), and
// the fixed reply text.
//
// It never plans operations or issues links (internal/claimsintake and the SQL definers do), never
// sends more than one POST per operation (every non-2xx or doubt is UNKNOWN; Reconcile is
// query-only and returns UNKNOWN until probe U4), never falls back to another channel, never logs
// or returns a token, comment id or response body, and never refreshes or issues Page tokens (T07).
//
// Depends on:
//   - internal/integrations/core: DispatchRoute, Secret, ErrPolicyDenied (the dispatcher owns leases,
//     the final binding gate and outcome persistence).
//   - internal/claims: SystemLinkToken/ReplyLinkKey (the adapter re-derives the link token; only
//     sha256(token) reaches SQL).
//   - internal/platform: ValidateWorkerPool for the Check pool.
//   - PostgreSQL: claims.check_meta_reply (commerce_worker pool, definer commerce_claims_writer),
//     integration.load_meta_page_token (dispatcher LoadSecret transaction, definer
//     commerce_integration_writer), integration.register_meta_page_token (registrar login).
//   - graph.facebook.com: POST /{version}/{asset_id}/messages (loopback httptest only in MOCK); docs
//     https://developers.facebook.com/docs/messenger-platform/discovery/private-replies/ and
//     https://developers.facebook.com/docs/instagram-platform/private-replies/ (retrieved 2026-09-28).
//
// Used by: cmd/claims-worker (Routes), cmd/meta-admin (RegisterPageToken, LoadPageTokenKeyring).
package metareply
