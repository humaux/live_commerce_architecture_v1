// Purpose: buyer claim preview and delta redemption into the existing cart.
// Depends on: buyer scope/commands, claims definers (claims.preview_link, claims.preview_live_prices, claims.redeem_link, claims.mark_applied), storefront cart and inventory availability.
// Used by: buyerhttp B1/B2 handlers and claims acceptance tests.
// Invariants: live-keyword-claims-v1 amendment "Live price kept on pause" (owner decision 2026-10-07): a claim line of a paused offer is previewed and redeemed like
//   any other (the pause refuses only NEW claims); one SKU is carried into the cart by one claim line only (skuWinners: active, then already applied, then lowest offer id).
// buyer.go owns the buyer side of a claim link: PreviewLink (read-only) and RedeemLink
// (bind, then apply pending claim lines to the buyer's own cart) (contract §4.4, §6).
//
// Non-goals: no inventory reservation, Quote, checkout or order (inventory is touched only
// by a later BeginCheckout); no session title, label, actor key, platform, owner or
// principal in any buyer projection; no cart write except through storefront.SetCart.
//
// Lock order on redeem (§5.6): buyer-command receipt advisory → claims.bundles →
// claims.lines (inside redeem_link) → cart advisory (LockCartOwner) → nested cart.set
// receipt advisory (derived "clm:" key) → storefront.carts → catalog products → SKUs.

package claims

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

// PreviewLine is the buyer-safe claim projection; amounts and stock hints never authorize checkout.
type PreviewLine struct {
	Keyword        string `json:"keyword"`
	SKUID          string `json:"sku_id"`
	SKUCode        string `json:"sku_code"`
	ProductName    string `json:"product_name"`
	Currency       string `json:"currency"`
	UnitPriceMinor int64  `json:"unit_price_minor"` // display only; Quote remains the price authority. The live price when one applies.
	// Live tools (R4): set only when a live price applies; both omitted for a catalog-priced line.
	CatalogUnitPriceMinor int64  `json:"catalog_unit_price_minor,omitempty"`
	PriceRule             string `json:"price_rule,omitempty"`
	Quantity              int64  `json:"quantity"`
	Pending               bool   `json:"pending"`
	Available             bool   `json:"available"`
	SoldOut               bool   `json:"sold_out"`
}

// Preview describes a live claim link without identity, locks or stock reservation.
type Preview struct {
	BundleVersion int64         `json:"bundle_version"`
	Bound         bool          `json:"bound"`
	ExpiresAt     time.Time     `json:"expires_at"`
	Lines         []PreviewLine `json:"lines"` // ORDER BY keyword
}

// RedeemInput pins the observed bundle version for the explicit buyer command.
type RedeemInput struct {
	ExpectedBundleVersion int64 `json:"expected_bundle_version"`
}

// Skipped reports a pending line that B2 did not apply; it remains available to retry later.
type Skipped struct {
	SKUID string `json:"sku_id"`
	// offer_inactive: only a line superseded on its SKU (skuWinners: a paused offer's line when the bundle also holds the SKU's active replacement, typo
	// recovery); a paused offer's line on its own is applied (owner decision 2026-10-07).
	Reason string `json:"reason"` // unavailable | offer_inactive | sold_out
}

// Redeemed reports the same-transaction cart merge, applied lines and pending skips.
type Redeemed struct {
	BundleVersion int64             `json:"bundle_version"`
	Cart          storefront.Cart   `json:"cart"`
	Applied       []storefront.Item `json:"applied"`
	Skipped       []Skipped         `json:"skipped"`
}

// Skip reasons reported for pending lines that stay pending.
const (
	skipUnavailable   = "unavailable"
	skipOfferInactive = "offer_inactive"
	skipSoldOut       = "sold_out"
)

// redeemKeyPrefix is the derived nested cart.set key prefix. buyerhttp rejects client
// Idempotency-Keys with this prefix on PUT /v1/buyer/cart, so a direct cart write and a
// redeem can never share a cart.set receipt key under opposite lock orders (§5.6).
const redeemKeyPrefix = "clm:"

// PreviewLink shows what a link would prefill, in a buyer.WithScope transaction. It calls
// claims.preview_link (no lock, no write) and enriches lines from buyer-readable catalog
// columns; Available = the line is not superseded on its SKU (skuWinners) ∧ SKU and product
// active ∧ SKU currency = store currency. The offer being paused does NOT make a line
// unavailable (owner decision 2026-10-07). Unknown, expired, rotated, other-owner,
// other-store and other-tenant links are all ErrNotFound. No receipt, event, lock or
// write. Called by B1.
func PreviewLink(ctx context.Context, tx pgx.Tx, s buyer.Scope, token LinkToken) (Preview, error) {
	if ctx == nil || tx == nil {
		return Preview{}, command.ErrInvalid
	}
	if _, err := ParseLinkToken(string(token)); err != nil {
		return Preview{}, err
	}
	if err := buyer.CheckScope(ctx, tx, s); err != nil {
		return Preview{}, err
	}
	// catalog.skus/products, control.stores: the buyer's own read grants (0007), no lock.
	rows, err := tx.Query(ctx, `SELECT p.bundle_version,p.bound,p.expires_at,p.keyword,p.sku_id::text,s.code,pr.name,
		s.currency,s.price_minor,p.quantity,p.pending,
		s.status='active' AND pr.status='active' AND s.currency=st.currency,p.offer_id::text,p.offer_active
		FROM claims.preview_link($1::bytea) p
		JOIN catalog.skus s ON s.tenant_id=$2 AND s.store_id=$3 AND s.id=p.sku_id
		JOIN catalog.products pr ON pr.tenant_id=s.tenant_id AND pr.store_id=s.store_id AND pr.id=s.product_id
		JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
		ORDER BY p.keyword`, token.hash(), s.TenantID, s.StoreID)
	if err != nil {
		return Preview{}, mapError(err)
	}
	defer rows.Close()
	out := Preview{Lines: []PreviewLine{}}
	var contenders []claimLine // every line of the link with its offer, for the one-line-per-SKU rule
	for rows.Next() {
		var line PreviewLine
		var c claimLine
		if err := rows.Scan(&out.BundleVersion, &out.Bound, &out.ExpiresAt, &line.Keyword, &line.SKUID, &line.SKUCode,
			&line.ProductName, &line.Currency, &line.UnitPriceMinor, &line.Quantity, &line.Pending, &line.Available,
			&c.offerID, &c.offerActive); err != nil {
			return Preview{}, mapError(err)
		}
		c.skuID, c.pending = line.SKUID, line.Pending
		contenders = append(contenders, c)
		out.Lines = append(out.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return Preview{}, mapError(err)
	}
	if len(out.Lines) == 0 {
		return Preview{}, command.ErrNotFound
	}
	// A line that loses its SKU to another claim line of the bundle is not available (redeem would skip it as offer_inactive).
	winners, err := skuWinners(contenders)
	if err != nil {
		return Preview{}, err
	}
	for i := range out.Lines {
		if winners[contenders[i].skuID] != contenders[i].offerID {
			out.Lines[i].Available = false
		}
	}
	// inventory.buyer_sku_availability: read-only stock hint, Direct checkout amendment (I03).
	lines := make([]claimLine, 0, len(out.Lines))
	for _, line := range out.Lines {
		lines = append(lines, claimLine{skuID: line.SKUID})
	}
	availability, err := availableSKUs(ctx, tx, s, lines)
	if err != nil {
		return Preview{}, err
	}
	for i := range out.Lines {
		out.Lines[i].SoldOut = availability[out.Lines[i].SKUID].soldOut(out.Lines[i].Quantity)
	}
	return out, overlayPreviewPrices(ctx, tx, token, &out)
}

// overlayPreviewPrices shows each line's live price (claims.preview_live_prices: same link guards as
// preview_link, priced offers whether or not they are paused, 0158) as the display unit price, keeping the catalog price beside
// it. Display only: the Quote re-proves the origin and decides the charge (amendment "Live tools (R4)" rule 6).
func overlayPreviewPrices(ctx context.Context, tx pgx.Tx, token LinkToken, out *Preview) error {
	rows, err := tx.Query(ctx, `SELECT keyword,live_price_minor FROM claims.preview_live_prices($1::bytea)`, token.hash())
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	live := map[string]int64{}
	for rows.Next() {
		var keyword string
		var price int64
		if err := rows.Scan(&keyword, &price); err != nil {
			return mapError(err)
		}
		live[keyword] = price
	}
	if err := rows.Err(); err != nil {
		return mapError(err)
	}
	for i := range out.Lines {
		line := &out.Lines[i]
		if price, ok := live[line.Keyword]; ok {
			// pricing.ResolveUnitPrice is the one rule (storefront applies it at Quote time).
			unit, rule := pricing.ResolveUnitPrice(line.UnitPriceMinor, &price)
			if rule == pricing.RuleLiveClaim {
				line.CatalogUnitPriceMinor, line.UnitPriceMinor, line.PriceRule = line.UnitPriceMinor, unit, rule
			}
		}
	}
	return nil
}

// claimLine is one row of claims.redeem_link.
type claimLine struct {
	bundleID, offerID, skuID string
	quantity, version        int64
	pending, offerActive     bool
}

// RedeemLink binds the link's bundle to the caller (first owner wins) and applies its
// pending lines as absolute cart quantities, in a buyer.WithScope transaction under one
// buyer receipt ("claims.redeem", request {link_sha256, expected_bundle_version}). Pending
// lines superseded on their SKU (skuWinners), whose SKU/product is unavailable or sold out
// are skipped and stay pending; a line of a PAUSED offer is applied like any other (owner
// decision 2026-10-07); non-claim and already-applied lines are untouched. Nothing to apply → the
// current cart, no cart write. Errors: unknown/expired/rotated/other-owner link →
// ErrNotFound; bundle version changed, a pre-existing unavailable cart item, or a cart
// post-condition failure → ErrConflict; merged cart above SetCart's 50-SKU bound →
// ErrInvalid. Every error rolls back the whole command, including the binding. Same key +
// token + body replays the stored result. Called by B2.
func RedeemLink(ctx context.Context, tx pgx.Tx, s buyer.Scope, key string, token LinkToken, in RedeemInput) (Redeemed, error) {
	if ctx == nil || tx == nil || in.ExpectedBundleVersion < 1 || in.ExpectedBundleVersion == math.MaxInt64 {
		return Redeemed{}, command.ErrInvalid
	}
	if _, err := ParseLinkToken(string(token)); err != nil {
		return Redeemed{}, err
	}
	hash := token.hash()
	request := struct {
		LinkSHA256            string `json:"link_sha256"`
		ExpectedBundleVersion int64  `json:"expected_bundle_version"`
	}{hex.EncodeToString(hash), in.ExpectedBundleVersion}
	var out Redeemed
	// buyer.RunCommand: owner+session-scoped receipt in buyer.command_results (replay/409).
	err := buyer.RunCommand(ctx, tx, s, "claims.redeem", key, request, &out, func() error {
		bundleVersion, lines, err := redeemLines(ctx, tx, hash, in.ExpectedBundleVersion)
		if err != nil {
			return err
		}
		available, err := availableSKUs(ctx, tx, s, lines)
		if err != nil {
			return err
		}
		apply, skipped, err := splitPending(lines, available)
		if err != nil {
			return err
		}
		out = Redeemed{BundleVersion: bundleVersion, Applied: []storefront.Item{}, Skipped: skipped}
		if len(apply) == 0 {
			// storefront.GetCart: read-only projection of the caller's cart (no cart write).
			out.Cart, err = storefront.GetCart(ctx, tx, s)
			return err
		}
		// storefront (the cart owner package): serialize same-owner cart writers before the
		// read-modify-write; SetCart then revalidates and locks the whole catalog selection,
		// so a pre-existing unavailable item fails the redeem with ErrConflict.
		if err := storefront.LockCartOwner(ctx, tx, s); err != nil {
			return err
		}
		cart, err := storefront.GetCart(ctx, tx, s)
		if err != nil {
			return err
		}
		// storefront.SetCart is the only cart writer; claims never writes storefront tables.
		// Its nested cart.set receipt key is derived from the redeem key (never client input).
		derived := sha256.Sum256([]byte("claims.redeem|" + key))
		// Origins: the claim line behind each applied quantity. This is the only writer of cart-line origins
		// (CartInput.Origins is json:"-"); claims.live_prices re-proves them at every Quote.
		origins := make(map[string]storefront.ClaimOrigin, len(apply))
		for _, line := range apply {
			origins[line.skuID] = storefront.ClaimOrigin{BundleID: line.bundleID, OfferID: line.offerID, Quantity: line.quantity, LineVersion: line.version}
		}
		out.Cart, err = storefront.SetCart(ctx, tx, s, redeemKeyPrefix+hex.EncodeToString(derived[:])[:48],
			storefront.CartInput{ExpectedVersion: cart.Version, Items: mergeCart(cart.Items, apply), Origins: origins})
		if err != nil {
			return err
		}
		for _, line := range apply {
			if !cartHas(out.Cart.Items, line.skuID, line.quantity) {
				return command.ErrConflict
			}
			out.Applied = append(out.Applied, storefront.Item{SKUID: line.skuID, Quantity: line.quantity})
		}
		return markApplied(ctx, tx, apply)
	})
	if err != nil {
		return Redeemed{}, mapError(err)
	}
	return out, nil
}

// redeemLines calls claims.redeem_link (bind + CAS + line lock) and returns the bundle
// version with its lines ordered by offer. Zero rows is the uniform not-found; PT409
// (version changed) maps to ErrConflict through mapError.
func redeemLines(ctx context.Context, tx pgx.Tx, hash []byte, expected int64) (int64, []claimLine, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text,bundle_version,offer_id::text,sku_id::text,quantity,line_version,pending,offer_active
		FROM claims.redeem_link($1::bytea,$2::bigint)`, hash, expected)
	if err != nil {
		return 0, nil, mapError(err)
	}
	defer rows.Close()
	var bundleVersion int64
	var lines []claimLine
	for rows.Next() {
		var l claimLine
		if err := rows.Scan(&l.bundleID, &bundleVersion, &l.offerID, &l.skuID, &l.quantity, &l.version, &l.pending, &l.offerActive); err != nil {
			return 0, nil, mapError(err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, mapError(err)
	}
	if len(lines) == 0 {
		return 0, nil, command.ErrNotFound
	}
	return bundleVersion, lines, nil
}

// skuAvailability combines the unchanged catalog rule with a read-only stock snapshot.
type skuAvailability struct {
	available, tracked bool
	quantity           int64
}

func (a skuAvailability) soldOut(quantity int64) bool { return a.tracked && a.quantity < quantity }

// availableSKUs reads catalog availability and inventory.buyer_sku_availability in one
// bounded query. No locks/writes: Begin is the only stock authority (Direct checkout).
func availableSKUs(ctx context.Context, tx pgx.Tx, s buyer.Scope, lines []claimLine) (map[string]skuAvailability, error) {
	var skus []string
	for _, line := range lines {
		skus = append(skus, line.skuID)
	}
	available := map[string]skuAvailability{}
	if len(skus) == 0 {
		return available, nil
	}
	// Calls inventory.buyer_sku_availability: Direct checkout scoped hint, no raw stock grant.
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.status='active' AND p.status='active' AND s.currency=st.currency,a.tracked,a.available
		FROM inventory.buyer_sku_availability($3::uuid[]) a
		JOIN catalog.skus s ON s.id=a.sku_id
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.id=ANY($3::uuid[])`, s.TenantID, s.StoreID, skus)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var sku string
		var state skuAvailability
		if err := rows.Scan(&sku, &state.available, &state.tracked, &state.quantity); err != nil {
			return nil, mapError(err)
		}
		available[sku] = state
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return available, nil
}

// skuWinners picks, per SKU, the one claim line of the bundle that may carry the SKU into the cart (the cart holds one claim origin per SKU). Rank:
//  1. the line of the ACTIVE offer (typo recovery: a paused A11 and its active replacement A1 on one SKU; live_offer_active_sku makes the active one unique, so two
//     is a broken invariant = ErrConflict);
//  2. else the line ALREADY APPLIED into the cart (!pending): the line that already carries the SKU keeps it. Without this, pausing the replacement after its redeem
//     (an end-of-live batch deactivate) would let a lower-id superseded typo line flip the cart to the typo claim, and after the replacement's order grant 2 more
//     live-price units (Opus review P1-1);
//  3. else the lowest offer id (deterministic final tie-break; it only decides when nothing is active and nothing was applied yet, e.g. the reminder flow. The claim
//     time is not exposed by claims.preview_link / claims.redeem_link, so "earliest claim" would need a signature change: open owner choice, see the contract).
//
// ALL lines of the SKU compete, applied or not. Every line stays within its own claimed quantity, so no outcome grants units the buyer did not claim. The map holds
// the winning offer id per SKU. claims.for_buyer_lines (0158) applies the same ranking in SQL for the merchant order-for-buyer prefill. Pure.
func skuWinners(lines []claimLine) (map[string]string, error) {
	winners := map[string]claimLine{}
	for _, line := range lines {
		cur, ok := winners[line.skuID]
		switch {
		case !ok:
			winners[line.skuID] = line
		case line.offerActive && cur.offerActive:
			return nil, command.ErrConflict
		case line.offerActive != cur.offerActive:
			if line.offerActive {
				winners[line.skuID] = line
			}
		case line.pending != cur.pending:
			if !line.pending {
				winners[line.skuID] = line
			}
		case line.offerID < cur.offerID:
			winners[line.skuID] = line
		}
	}
	out := make(map[string]string, len(winners))
	for sku, line := range winners {
		out[sku] = line.offerID
	}
	return out, nil
}

// splitPending decides, per pending line, apply or skip (superseded on its SKU before
// unavailable before sold out); skipped lines stay pending. A paused offer's line is
// applicable (owner decision 2026-10-07). skuWinners leaves exactly one line per SKU in
// play. Output is ordered by SKU. Pure.
func splitPending(lines []claimLine, available map[string]skuAvailability) ([]claimLine, []Skipped, error) {
	winners, err := skuWinners(lines)
	if err != nil {
		return nil, nil, err
	}
	apply, skipped := []claimLine{}, []Skipped{}
	for _, line := range lines {
		switch {
		case !line.pending:
		case winners[line.skuID] != line.offerID:
			skipped = append(skipped, Skipped{SKUID: line.skuID, Reason: skipOfferInactive})
		case !available[line.skuID].available:
			skipped = append(skipped, Skipped{SKUID: line.skuID, Reason: skipUnavailable})
		case available[line.skuID].soldOut(line.quantity):
			skipped = append(skipped, Skipped{SKUID: line.skuID, Reason: skipSoldOut})
		default:
			apply = append(apply, line)
		}
	}
	sort.Slice(apply, func(i, j int) bool { return apply[i].skuID < apply[j].skuID })
	sort.SliceStable(skipped, func(i, j int) bool { return skipped[i].SKUID < skipped[j].SKUID })
	return apply, skipped, nil
}

// mergeCart overrides the cart quantity of each applied SKU with its absolute claim target
// and keeps every other cart line unchanged. SetCart canonicalizes order and bounds. Pure.
func mergeCart(items []storefront.Item, apply []claimLine) []storefront.Item {
	merged := append([]storefront.Item{}, items...)
	for _, line := range apply {
		found := false
		for i := range merged {
			if merged[i].SKUID == line.skuID {
				merged[i].Quantity, found = line.quantity, true
			}
		}
		if !found {
			merged = append(merged, storefront.Item{SKUID: line.skuID, Quantity: line.quantity})
		}
	}
	return merged
}

func cartHas(items []storefront.Item, skuID string, quantity int64) bool {
	for _, item := range items {
		if item.SKUID == skuID {
			return item.Quantity == quantity
		}
	}
	return false
}

// markApplied records, via claims.mark_applied, exactly the line versions just written to
// the cart; a count mismatch raises PT409 (→ ErrConflict) and rolls the redeem back.
func markApplied(ctx context.Context, tx pgx.Tx, apply []claimLine) error {
	offers, versions := make([]string, len(apply)), make([]int64, len(apply))
	for i, line := range apply {
		offers[i], versions[i] = line.offerID, line.version
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT claims.mark_applied($1::uuid,$2::uuid[],$3::bigint[])`,
		apply[0].bundleID, offers, versions).Scan(&count); err != nil {
		return mapError(err)
	}
	if count != len(apply) {
		return command.ErrConflict
	}
	return nil
}
