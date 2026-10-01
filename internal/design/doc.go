// Package design owns the merchant's storefront design document (contracts/storefront-v2.md section B, unit
// store-design): strict validation and normalisation of the closed JSON schema (profile, header/footer nav, home
// sections, info pages), the single editable draft per store (compare-and-set on version), the append-only list of
// published versions with publish and rollback, store-owned media (logo, favicon, hero and section images) and
// 15-minute preview tokens.
//
// It never renders anything (the storefront Next app renders the published document), never accepts HTML, CSS or
// script (unknown keys are refused with the path of the first offender, markdown is restricted to paragraphs, bold,
// italic, lists and https links), never trusts a tenant or store id from a request (scope comes from platform.Scope,
// i.e. server auth), and never mutates published history (rollback is a new version that copies an old one).
//
// Tables (migrations/0087): design.documents, design.published_versions, design.store_media, design.preview_tokens,
// all as commerce_runtime inside platform.WithScope; buyers read them only through the design.buyer_* definers called
// by internal/buyerhttp. Image validation reuses catalog.SniffImage (same CM1/CM2 rules as product photos).
package design
