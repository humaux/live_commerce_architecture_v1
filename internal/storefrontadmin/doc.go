// Package storefrontadmin owns the production writer of storefront publication and domain binding
// (R3 unit storefront-publish, migrations/0081, contracts/published-storefront-resolver-v1.md "Writer (R3)"):
// the merchant's publish/unpublish consent (Read, SetPublished, on the caller's WithScope transaction) and the
// platform operator's domain lifecycle (BindDomain, SuspendDomain, DetachDomain, Status, on the registrar pool).
//
// It never decides authority or a lifecycle rule itself: every rule is in the SQL definers (control.read_storefront,
// control.set_storefront_published, control.operator_*), this package validates inputs at the trust boundary, calls
// exactly one definer per operation, strictly decodes the jsonb result and maps SQLSTATE classes to stable sentinels
// (never a driver message). It never verifies DNS or TLS (the operator attests --evidence/--valid-until), never
// resolves an origin for buyers (internal/domains does), never exposes an operator function over merchant HTTP and
// never serves a store by itself: publication needs an ACTIVE domain and the domain needs publication.
//
// It calls no external service. Tables are reached only through the definers: control.storefront_publications,
// control.storefront_domains, ops.audit_events, identity.initial_stores (audit attribution). Origin grammar is
// reused from internal/domains.ValidOrigin, not duplicated.
package storefrontadmin
