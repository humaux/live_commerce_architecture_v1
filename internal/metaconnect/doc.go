// Package metaconnect owns the merchant self-serve connection of Facebook Pages (up to 10 per store, each with its optional
// Instagram account) to a store (contracts/meta-claims-intake-v1.md "Merchant connect (R4)", unit meta-connect): the Facebook Login
// for Business start and callback, the Page pick list, the one-transaction bind (sealed Page credential + binding + webhook route +
// connection row) and the per-Page disconnect. It replaces the operator-only cmd/meta-admin path for everyday use; the CLI stays as
// break-glass.
//
// It never stores or logs a plaintext token (the user token lives only in this process's memory between callback and pick; the Page
// token is sealed to HPKE PUBLIC keys, so this process can seal a token but never open one), never decides a rule the SQL definers of
// migration 0095/0100/0108 own (state ownership, single use, 10 minutes, one-store-per-Page, the 10-Page cap, never partial-enable),
// never trusts a tenant or store from the request (scope comes from platform.WithScope), and never touches the Page-token loader of
// the claims worker (which alone holds the private ring).
//
// External service: graph.facebook.com through internal/integrations/meta/oauth (code exchange, long-lived extend,
// /me/permissions, /me/accounts, POST /{page_id}/subscribed_apps); www.facebook.com appears only in the dialog URL
// handed to the merchant's browser.
package metaconnect
