// Purpose: public replay list/detail with current buyer catalog products and Facebook's official player URL.
// Depends on: live.buyer_replays (0213), catalog.buyer_v2_product, projectV2/attachV2ImageSizes.
// Used by: handler.dispatch after strict route selection; storefront public BFF.
// Invariants: verified origin + BFF key only; no buyer bearer, body or query; no historical price/video copy.
package buyerhttp

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
)

type replayItem struct {
	SessionID    string    `json:"session_id"`
	Title        string    `json:"title"`
	EndedAt      time.Time `json:"ended_at"`
	PermalinkURL string    `json:"permalink_url"`
}
type replayOffer struct {
	OfferID string `json:"offer_id"`
	SKUID   string `json:"sku_id"`
	Product any    `json:"product"`
}

func (h *handler) replayGet(ctx context.Context, w http.ResponseWriter, r *http.Request, selected route) error {
	w.Header().Set("Cache-Control", "no-store")
	if err := noBody(r); err != nil {
		return err
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || selected.kind == replayDetailRoute && !command.ValidID(selected.id) {
		return responseError{422, "invalid_request"}
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	origin := r.Header.Get("X-Commerce-Storefront-Origin")
	var id *string
	if selected.kind == replayDetailRoute {
		id = &selected.id
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT live.buyer_replays($1::text,$2::uuid)`, origin, id).Scan(&raw); err != nil {
		return mapPublicCatalogError(err)
	}
	if selected.kind == replayListRoute {
		var out struct {
			Items []replayItem `json:"items"`
		}
		if err = strictDecode(raw, &out); err != nil {
			return err
		}
		writeOK(w, out)
		return nil
	}
	var source struct {
		replayItem
		Offers []struct {
			OfferID   string `json:"offer_id"`
			SKUID     string `json:"sku_id"`
			ProductID string `json:"product_id"`
		} `json:"offers"`
	}
	if err = strictDecode(raw, &source); err != nil {
		return err
	}
	out := struct {
		replayItem
		EmbedURL string        `json:"embed_url"`
		Offers   []replayOffer `json:"offers"`
	}{source.replayItem, "https://www.facebook.com/plugins/video.php?" + url.Values{"href": {source.PermalinkURL}}.Encode(), []replayOffer{}}
	for _, offer := range source.Offers {
		var productRaw []byte
		err = tx.QueryRow(ctx, `SELECT live.buyer_replay_product($1::text,$2::text)`, origin, offer.ProductID).Scan(&productRaw)
		// The wrapper converts only catalog PT404 to NULL without aborting this read transaction.
		if err != nil {
			return mapPublicCatalogError(err)
		}
		if productRaw == nil {
			continue
		}
		product, err := projectV2(catalogV2ProductRoute, productRaw, v2Query{})
		if err != nil {
			return err
		}
		if err = attachV2ImageSizes(ctx, tx, origin, product); err != nil {
			return mapPublicCatalogError(err)
		}
		visible := false
		for _, variant := range product.(v2Detail).Variants {
			visible = visible || variant.SKUID == offer.SKUID
		}
		if visible {
			out.Offers = append(out.Offers, replayOffer{offer.OfferID, offer.SKUID, product})
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	writeOK(w, out)
	return nil
}
