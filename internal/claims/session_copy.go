// Purpose: seed a freshly created session's claim configuration from a source session in one caller-owned transaction (A5-2): a CLOSED generation-0 window in the source's match mode (EXACT when the source never opened one), and every ACTIVE source offer copied WITH its live_price_minor (Live tools (R4) rule 1 as amended by A5: whole-session copy keeps the live price; the keyword-import action still never copies it).
// Depends on: merchant.go (readWindow/writeWindow/readOffers), keyword_library.go (importCandidates/existingOffers/sellableSKUs/planImport/normalizeLivePrice), live.offers, live.claim_windows, command, platform.
// Used by: internal/live/copy.go CopySession (the only caller), inside its live.session.copy receipt.
package claims

import (
	"context"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// CopyContent copies the claim configuration of sourceSessionID onto the freshly created
// targetSessionID. It takes no receipt and writes no audit row: the caller (internal/live
// CopySession) owns the live.session.copy receipt and the live.session.copied audit, so this
// work commits or rolls back with that single transaction. It never locks the source: the
// live layer already holds the source session FOR SHARE, which is the one order every offer
// mutation serializes behind. It never copies bundles, links, claim sources, price-uses or the
// keyword library (those are session-scoped or store-scoped, per A5-2).
func CopyContent(ctx context.Context, tx pgx.Tx, scope platform.Scope, sourceSessionID, targetSessionID string) (Window, []Offer, []ImportConflict, error) {
	empty := []ImportConflict{}
	if !command.ValidID(sourceSessionID) || !command.ValidID(targetSessionID) || sourceSessionID == targetSessionID {
		return Window{}, nil, empty, command.ErrInvalid
	}
	source, err := readWindow(ctx, tx, scope, sourceSessionID, "")
	if err != nil {
		return Window{}, nil, empty, err
	}
	window, err := writeWindow(ctx, tx, scope, targetSessionID, 0, WindowInput{
		ExpectedVersion: 0,
		State:           WindowClosed,
		MatchMode:       source.MatchMode,
	})
	if err != nil {
		return Window{}, nil, empty, err
	}
	candidates, err := importCandidates(ctx, tx, scope, importSourceSession, sourceSessionID)
	if err != nil {
		return Window{}, nil, empty, err
	}
	existing, err := existingOffers(ctx, tx, scope, targetSessionID)
	if err != nil {
		return Window{}, nil, empty, err
	}
	sellable, err := sellableSKUs(ctx, tx, scope, candidates)
	if err != nil {
		return Window{}, nil, empty, err
	}
	create, conflicts := planImport(candidates, existing, sellable, maxOffersPerSession)
	created := []Offer{}
	if len(create) > 0 {
		ids := map[string]bool{}
		for _, c := range create {
			livePrice, ok := normalizeLivePrice(c.LivePriceMinor)
			if !ok {
				return Window{}, nil, empty, command.ErrInvalid
			}
			var id string
			// Same insert as CreateOffer but WITH the live price (rule 1): the source offer's stored
			// price is already bounded by the column CHECK, so normalize only re-asserts the contract.
			if err = tx.QueryRow(ctx, `INSERT INTO live.offers(tenant_id,store_id,session_id,keyword,sku_id,max_quantity_per_claim,principal_id,live_price_minor)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`,
				scope.TenantID, scope.StoreID, targetSessionID, c.Keyword, c.SKUID, c.MaxQuantity, scope.PrincipalID, livePrice).Scan(&id); err != nil {
				return Window{}, nil, empty, mapError(err)
			}
			ids[id] = true
		}
		all, err := readOffers(ctx, tx, scope, targetSessionID, "")
		if err != nil {
			return Window{}, nil, empty, err
		}
		for _, o := range all {
			if ids[o.ID] {
				created = append(created, o)
			}
		}
	}
	return window, created, conflicts, nil
}
