// Package merchanttools owns three admin-side tools of contracts/storefront-v2.md section G: the dashboard read model (dashboard.go),
// the product CSV export and all-or-nothing import (csvfile.go, csvexport.go, csvimport.go) and the merchant-created ("manual")
// order (manual.go).
//
// It never owns a rule of the domain it drives: the dashboard money is internal/reporting (identity.read_finance_summary, no second
// ledger) and its order rows are internal/merchantorders; the CSV import writes only through internal/catalog and internal/inventory
// functions (journaled commands with a derived key, optimistic versions, stock only through inventory.AdjustOnHand); the manual order
// runs the buyer path unchanged (storefront.SetCart / CreateQuote / SetDestination, checkout.Service.Begin) under a server-held buyer
// capability, so the quote is the only price, stock is reserved by the same begin_hold and the expiry job is the same one. It never
// deletes or archives a catalog row, never lets a merchant set a price or take a card, never opens a payment, shipment or message, and
// never trusts a tenant or store from a request (scope comes from the merchant bearer, platform.WithScope).
//
// External services: none. Tables read directly (under RLS, commerce_runtime): catalog.products/skus/collections/collection_products/
// product_images, inventory.balances/warehouses, control.stores; the privileged reads are the definers of migrations/0094.
package merchanttools

import (
	"errors"
	"net/http"
)

// Error is a refusal that carries its HTTP status and its contract code (the httperror table knows every code used here).
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return "merchanttools: " + e.Code }

// Sentinels the HTTP layer classifies. ErrPreviewRolledBack is returned by a dry run after it computed its answer: the transaction must
// roll back (nothing is written) and the caller still serves the result. ErrImportHasErrors is the commit refusal (also a rollback).
var (
	ErrPreviewRolledBack = errors.New("csv preview rolled back")
	ErrImportHasErrors   = &Error{Status: http.StatusUnprocessableEntity, Code: "import_has_errors"}
	ErrExportTooLarge    = &Error{Status: http.StatusUnprocessableEntity, Code: "export_too_large"}
	ErrUnavailable       = &Error{Status: http.StatusServiceUnavailable, Code: "retry_later"}
	ErrManualDisabled    = &Error{Status: http.StatusServiceUnavailable, Code: "manual_order_unavailable"}
)
