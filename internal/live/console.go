// Purpose: A1 live-console read model (live-console-v1 §7.1, unit LC-B7): one GET snapshot of a session for the console — lifecycle, claim window, stats (comments, keyword comments, buyers, orders, paid), per-offer stock/claims/sales, capabilities, comment-stream state and the last recommended offer. Read-only: no write, no external call except one bridge read for the stream state/comment total.
// Depends on: draft.go (authorize/mapError/mapReadError, readPermission), claims.ReadWindow (window), CommentStream (stream.go: resolveSource + metabridge comment-page limit 1), metaconnect.CapabilityReader (§6 rows, optional), live.sessions/offers/offer_timeline, catalog.skus/products, inventory.balances/warehouses, claims.lines (runtime SELECT); identity.read_live_console_sales (migration 0148: orders/paid money, keyword comments, buyers, per-offer sales; EXECUTE commerce_runtime, live:read only).
// Used by: internal/httpapi/live_console.go (GET /live-sessions/{session_id}/console), cmd/api through httpapi.NewHandler.
// Invariants: I01 (scope from the authenticated transaction, session must be in the store → 404), I05 (paid = captured money of the deployment's payment environment only, never mixed), I11 (no comment text/names are read or returned; the bridge page is requested with limit 1 and only its stream state and next_seq are kept).
// Status: MOCK (bridge via fake Graph).
package live

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/metabridge"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

// lowStockMax is the §7.1 low_stock bound: tracked ∧ 0 < sellable ≤ 5.
const lowStockMax = 5

// defaultConsoleCurrency is the A1 currency when the session has no attributed order and no offer to take one from (TWD is the pilot market).
const defaultConsoleCurrency = "TWD"

// Console builds the A1 snapshot. stream (the A2 comment read-through) and caps (the §6 capability reader) are both optional: nil leaves the
// comment total `unavailable` / the capabilities empty, never an error.
type Console struct {
	stream *CommentStream
	caps   metaconnect.CapabilityReader
	env    string // payment environment "paid" is counted in: SANDBOX (default, pre-LIVE) or LIVE
}

// NewConsole returns the A1 read service. Both arguments may be nil.
func NewConsole(stream *CommentStream, caps metaconnect.CapabilityReader) *Console {
	return &Console{stream: stream, caps: caps, env: "SANDBOX"}
}

// WithPaymentEnvironment returns a copy whose "paid" totals count captured money of the deployment's payment environment (httpapi Options.PaymentEnvironment):
// "LIVE" selects LIVE, anything else (including empty) keeps the pre-LIVE SANDBOX behaviour. Environments are never mixed in one total (I05).
func (c *Console) WithPaymentEnvironment(env string) *Console {
	cp := *c
	cp.env = "SANDBOX"
	if env == "LIVE" {
		cp.env = "LIVE"
	}
	return &cp
}

// ConsoleSnapshot is the §7.1 body (key set closed: the UI parser rejects unknown/missing keys).
type ConsoleSnapshot struct {
	Session      ConsoleSession                          `json:"session"`
	Window       ConsoleWindow                           `json:"window"`
	Stats        ConsoleStats                            `json:"stats"`
	Offers       []ConsoleOffer                          `json:"offers"`
	Capabilities map[string]map[string]ConsoleCapability `json:"capabilities"`
	Stream       ConsoleStreamState                      `json:"stream"`
	Recommended  *ConsoleRecommended                     `json:"recommended"`
}

// ConsoleSession is the session header. Version is lifecycle_version (the A7 CAS counter). StartedAt is the current window's opened_at (else
// the last transition while live); EndedAt is the transition time of an ended session, the window close of an archived one.
type ConsoleSession struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Lifecycle string     `json:"lifecycle"`
	Version   int64      `json:"version"`
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

// ConsoleWindow is the claim window projection (§8).
type ConsoleWindow struct {
	State      string           `json:"state"`
	Generation int64            `json:"generation"`
	OpenedAt   *time.Time       `json:"opened_at"`
	MatchMode  claims.MatchMode `json:"match_mode"`
}

// ConsoleStats is §7.1 stats: comments from the poller, the rest from identity.read_live_console_sales.
type ConsoleStats struct {
	Comments        ConsoleComments `json:"comments"`
	KeywordComments int64           `json:"keyword_comments"`
	Buyers          int64           `json:"buyers"`
	Orders          ConsoleTotal    `json:"orders"`
	Paid            ConsoleTotal    `json:"paid"`
	Currency        string          `json:"currency"`
	AsOf            time.Time       `json:"as_of"`
}

// ConsoleComments is the comment total: source stream_seen = unique comments the poller saw this epoch (the bridge has no Graph summary
// yet), unavailable = no poller/bridge (total null).
type ConsoleComments struct {
	Total  *int64 `json:"total"`
	Source string `json:"source"`
}

// ConsoleTotal is an order or paid count with its amount in the one console currency.
type ConsoleTotal struct {
	Count       int64 `json:"count"`
	AmountMinor int64 `json:"amount_minor"`
}

// ConsoleOffer is one offer row (§7.1). sellable follows the 0086 formula summed over the store's warehouses; WarehouseID/BalanceVersion
// name the one balance a stock edit CASes against (lowest warehouse id; no balance yet → the store's first warehouse, version 0).
type ConsoleOffer struct {
	OfferID        string       `json:"offer_id"`
	Keyword        string       `json:"keyword"`
	SKUID          string       `json:"sku_id"`
	ProductName    string       `json:"product_name"`
	VariantLabel   string       `json:"variant_label"`
	Active         bool         `json:"active"`
	Version        int64        `json:"version"`
	LivePriceMinor *int64       `json:"live_price_minor"`
	SKUPriceMinor  int64        `json:"sku_price_minor"`
	Stock          ConsoleStock `json:"stock"`
	Claimed        ConsoleClaim `json:"claimed"`
	OrderedQty     int64        `json:"ordered_qty"`
	PaidQty        int64        `json:"paid_qty"`
	PaidAmount     int64        `json:"paid_amount_minor"`
	SoldOut        bool         `json:"sold_out"`
	LowStock       bool         `json:"low_stock"`
}

// ConsoleStock is the stock cell of an offer.
type ConsoleStock struct {
	Tracked        bool    `json:"tracked"`
	Sellable       int64   `json:"sellable"`
	Reserved       int64   `json:"reserved"`
	WarehouseID    *string `json:"warehouse_id"`
	BalanceVersion int64   `json:"balance_version"`
}

// ConsoleClaim counts the claim lines of an offer (distinct buyers and Σ quantity).
type ConsoleClaim struct {
	Buyers   int64 `json:"buyers"`
	Quantity int64 `json:"quantity"`
}

// ConsoleCapability is one §6 row (provider → capability → row).
type ConsoleCapability struct {
	State     string     `json:"state"`
	Reason    string     `json:"reason"`
	Evidence  string     `json:"evidence"`
	CheckedAt *time.Time `json:"checked_at"`
}

// ConsoleRecommended is the latest "featured" offer event of the session (§7.3).
type ConsoleRecommended struct {
	OfferID string    `json:"offer_id"`
	At      time.Time `json:"at"`
}

// consoleSales is the decoded identity.read_live_console_sales projection.
type consoleSales struct {
	AsOf            time.Time    `json:"as_of"`
	Currency        *string      `json:"currency"`
	KeywordComments int64        `json:"keyword_comments"`
	Buyers          int64        `json:"buyers"`
	Orders          ConsoleTotal `json:"orders"`
	Paid            ConsoleTotal `json:"paid"`
	Offers          []struct {
		OfferID    string `json:"offer_id"`
		OrderedQty int64  `json:"ordered_qty"`
		PaidQty    int64  `json:"paid_qty"`
		PaidAmount int64  `json:"paid_amount_minor"`
	} `json:"offers"`
}

var consoleCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

// Read builds the A1 snapshot for one session (live:read). Side effects: none in PG; on a Facebook source it reads one bridge page (limit 1) for the
// stream state, which also refreshes the poller's demand lease. Errors: command.ErrNotFound (session not in the store), command.ErrInvalid,
// platform.Err* from the authority fences; capability or bridge failures degrade the affected cell instead of failing the snapshot.
func (c *Console) Read(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string) (ConsoleSnapshot, error) {
	if !command.ValidID(sessionID) {
		return ConsoleSnapshot{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return ConsoleSnapshot{}, err
	}
	var out ConsoleSnapshot
	var lifecycleAt *time.Time
	if err := tx.QueryRow(ctx, `SELECT id::text,title,lifecycle,lifecycle_version,lifecycle_at FROM live.sessions
		WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, sessionID).
		Scan(&out.Session.ID, &out.Session.Title, &out.Session.Lifecycle, &out.Session.Version, &lifecycleAt); err != nil {
		return ConsoleSnapshot{}, mapError(err) // no row → 404 (also a foreign tenant/store session, I01)
	}
	w, err := claims.ReadWindow(ctx, tx, scope, sessionID)
	if err != nil {
		return ConsoleSnapshot{}, err
	}
	out.Window = ConsoleWindow{State: w.State, Generation: w.Generation, OpenedAt: w.OpenedAt, MatchMode: w.MatchMode}
	out.Session.StartedAt, out.Session.EndedAt = sessionTimes(out.Session.Lifecycle, lifecycleAt, w)

	sales, err := readConsoleSales(ctx, tx, scope, token, sessionID, c.env)
	if err != nil {
		return ConsoleSnapshot{}, err
	}
	if out.Offers, err = readConsoleOffers(ctx, tx, scope, sessionID, sales); err != nil {
		return ConsoleSnapshot{}, err
	}
	if out.Recommended, err = readRecommended(ctx, tx, scope, sessionID); err != nil {
		return ConsoleSnapshot{}, err
	}
	out.Stats = ConsoleStats{KeywordComments: sales.KeywordComments, Buyers: sales.Buyers, Orders: sales.Orders, Paid: sales.Paid, AsOf: sales.AsOf,
		Currency: defaultConsoleCurrency}
	if sales.Currency != nil && consoleCurrency.MatchString(*sales.Currency) {
		out.Stats.Currency = *sales.Currency
	} else if len(out.Offers) > 0 {
		out.Stats.Currency = offerCurrency(ctx, tx, scope, out.Offers[0].SKUID)
	}
	out.Stream, out.Stats.Comments = c.streamAndTotal(ctx, tx, scope, sessionID, out.Session.Lifecycle)
	out.Capabilities = c.capabilities(ctx, tx, scope)
	// A replay or a long bridge wait stays an authenticated request.
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return ConsoleSnapshot{}, err
	}
	return out, nil
}

// sessionTimes derives started/ended (the schema stores only the last lifecycle transition time and the window times).
func sessionTimes(lifecycle string, at *time.Time, w claims.Window) (started, ended *time.Time) {
	started = w.OpenedAt
	if started == nil && lifecycle == LifecycleLive {
		started = at
	}
	switch lifecycle {
	case LifecycleEnded:
		ended = at
	case LifecycleArchived:
		ended = w.ClosedAt
	}
	return started, ended
}

// readConsoleSales calls identity.read_live_console_sales (0148) and strictly decodes it; a malformed projection is ErrResultsUnavailable.
func readConsoleSales(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID, environment string) (consoleSales, error) {
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// Calls identity.read_live_console_sales (live-console-v1 §7.1; migration 0148): live:read re-authenticated inside the definer.
	if err := tx.QueryRow(ctx, `SELECT identity.read_live_console_sales($1,$2::uuid,$3::uuid,$4)`, hash[:], scope.StoreID, sessionID, environment).Scan(&raw); err != nil {
		return consoleSales{}, mapReadError(err)
	}
	var s consoleSales
	if json.Unmarshal(raw, &s) != nil || s.AsOf.IsZero() || s.KeywordComments < 0 || s.Buyers < 0 ||
		s.Orders.Count < 0 || s.Orders.AmountMinor < 0 || s.Paid.Count < 0 || s.Paid.AmountMinor < 0 {
		return consoleSales{}, ErrResultsUnavailable
	}
	return s, nil
}

// readConsoleOffers lists the session's offers (≤200, ORDER BY keyword) with SKU, stock, claim lines and the sales merged in.
func readConsoleOffers(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, sales consoleSales) ([]ConsoleOffer, error) {
	rows, err := tx.Query(ctx, `SELECT o.id::text,o.keyword,o.sku_id::text,p.name,array_to_string(s.option_values,' / '),o.active,o.version,
		o.live_price_minor,s.price_minor,s.inventory_tracked,
		coalesce(st.sellable,0),coalesce(st.reserved,0),coalesce(st.warehouse_id,wh.id)::text,coalesce(st.version,0),
		coalesce(cl.buyers,0),coalesce(cl.quantity,0)
		FROM live.offers o
		JOIN catalog.skus s ON s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=o.sku_id
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		LEFT JOIN LATERAL (SELECT sum(b.on_hand-b.reserved-b.allocated-b.unavailable)::bigint AS sellable,sum(b.reserved)::bigint AS reserved,
		   (array_agg(b.warehouse_id ORDER BY b.warehouse_id))[1] AS warehouse_id,(array_agg(b.version ORDER BY b.warehouse_id))[1] AS version
		  FROM inventory.balances b WHERE b.tenant_id=o.tenant_id AND b.store_id=o.store_id AND b.sku_id=o.sku_id) st ON true
		LEFT JOIN LATERAL (SELECT w.id FROM inventory.warehouses w WHERE w.tenant_id=o.tenant_id AND w.store_id=o.store_id ORDER BY w.id LIMIT 1) wh ON true
		LEFT JOIN LATERAL (SELECT count(*)::bigint AS buyers,sum(l.quantity)::bigint AS quantity FROM claims.lines l
		  WHERE l.tenant_id=o.tenant_id AND l.store_id=o.store_id AND l.session_id=o.session_id AND l.offer_id=o.id) cl ON true
		WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.session_id=$3 ORDER BY o.keyword LIMIT 200`, scope.TenantID, scope.StoreID, sessionID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	bySales := make(map[string]int, len(sales.Offers))
	for i, so := range sales.Offers {
		bySales[so.OfferID] = i
	}
	offers := []ConsoleOffer{}
	for rows.Next() {
		var o ConsoleOffer
		if err := rows.Scan(&o.OfferID, &o.Keyword, &o.SKUID, &o.ProductName, &o.VariantLabel, &o.Active, &o.Version,
			&o.LivePriceMinor, &o.SKUPriceMinor, &o.Stock.Tracked, &o.Stock.Sellable, &o.Stock.Reserved, &o.Stock.WarehouseID,
			&o.Stock.BalanceVersion, &o.Claimed.Buyers, &o.Claimed.Quantity); err != nil {
			return nil, mapError(err)
		}
		if i, ok := bySales[o.OfferID]; ok {
			o.OrderedQty, o.PaidQty, o.PaidAmount = sales.Offers[i].OrderedQty, sales.Offers[i].PaidQty, sales.Offers[i].PaidAmount
		}
		o.SoldOut = o.Stock.Tracked && o.Stock.Sellable <= 0
		o.LowStock = o.Stock.Tracked && o.Stock.Sellable > 0 && o.Stock.Sellable <= lowStockMax
		offers = append(offers, o)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return offers, nil
}

// offerCurrency is the SKU currency of the first offer (used only when no order fixes the console currency).
func offerCurrency(ctx context.Context, tx pgx.Tx, scope platform.Scope, skuID string) string {
	var cur string
	if err := tx.QueryRow(ctx, `SELECT currency FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, skuID).Scan(&cur); err != nil ||
		!consoleCurrency.MatchString(cur) {
		return defaultConsoleCurrency
	}
	return cur
}

// readRecommended returns the latest 'featured' offer_timeline event of the session (§7.3), nil when none.
func readRecommended(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string) (*ConsoleRecommended, error) {
	var r ConsoleRecommended
	err := tx.QueryRow(ctx, `SELECT offer_id::text,at FROM live.offer_timeline WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND kind='featured'
		ORDER BY at DESC,id DESC LIMIT 1`, scope.TenantID, scope.StoreID, sessionID).Scan(&r.OfferID, &r.At)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapError(err)
	}
	return &r, nil
}

// streamAndTotal reads the comment-stream state. Facebook: one bridge page with limit 1 (its next_seq = unique comments seen this epoch). Instagram has no poller
// (webhook copy), so only the coarse state is reported and the total is unavailable. Every failure degrades to `unavailable`, never to an error.
func (c *Console) streamAndTotal(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID, lifecycle string) (ConsoleStreamState, ConsoleComments) {
	off := func(plat, reason string) ConsoleStreamState {
		return ConsoleStreamState{State: "unavailable", SourcePlatform: plat, Reason: reason}
	}
	unavailable := ConsoleComments{Source: "unavailable"}
	if c.stream == nil {
		return off(c.sourcePlatform(ctx, tx, scope, sessionID), "bridge_disabled"), unavailable
	}
	src, err := c.stream.resolveSource(ctx, tx, scope, sessionID)
	if err != nil {
		st := ConsoleStreamState{State: "not_started", SourcePlatform: c.sourcePlatform(ctx, tx, scope, sessionID), Reason: "no_source"}
		if !errors.Is(err, ErrNoSource) {
			st = off(st.SourcePlatform, "stream_unavailable")
		}
		return st, unavailable
	}
	if src.Object == "instagram" {
		st := ConsoleStreamState{State: "not_started", SourcePlatform: "instagram", VideoEmbeddable: false}
		if lifecycle == LifecycleLive {
			st.State = "live"
		}
		return st, unavailable
	}
	page, err := c.stream.bridge.CommentPage(ctx, metabridge.BridgePageRequest{TenantID: scope.TenantID, StoreID: scope.StoreID, SessionID: sessionID, SourceID: src.ID, Limit: 1})
	if err != nil {
		return off("facebook", "stream_unavailable"), unavailable
	}
	total := page.NextSeq
	return bridgeStream(page.Stream), ConsoleComments{Total: &total, Source: "stream_seen"}
}

// sourcePlatform is the platform of the session's newest claim source (active or not), "facebook" when none (the UI needs a closed value).
func (c *Console) sourcePlatform(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string) string {
	var p string
	if err := tx.QueryRow(ctx, `SELECT platform FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 ORDER BY updated_at DESC,id LIMIT 1`,
		scope.TenantID, scope.StoreID, sessionID).Scan(&p); err != nil || (p != "facebook" && p != "instagram") {
		return "facebook"
	}
	return p
}

// capabilities embeds the §6 rows (first row per provider/capability in the reader's stable order). A reader failure yields an empty map;
// it runs in a savepoint so a failed statement cannot abort the console transaction.
func (c *Console) capabilities(ctx context.Context, tx pgx.Tx, scope platform.Scope) map[string]map[string]ConsoleCapability {
	out := map[string]map[string]ConsoleCapability{}
	if c.caps == nil {
		return out
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return out
	}
	rows, err := c.caps.Capabilities(ctx, sp, scope, "")
	if err != nil {
		_ = sp.Rollback(ctx)
		return out
	}
	_ = sp.Commit(ctx)
	for _, r := range rows {
		if r.Provider != "facebook" && r.Provider != "instagram" {
			continue
		}
		if out[r.Provider] == nil {
			out[r.Provider] = map[string]ConsoleCapability{}
		}
		if _, dup := out[r.Provider][r.Capability]; dup {
			continue
		}
		cc := ConsoleCapability{State: r.State, Reason: r.Reason, Evidence: r.Evidence}
		if !r.CheckedAt.IsZero() {
			t := r.CheckedAt
			cc.CheckedAt = &t
		}
		out[r.Provider][r.Capability] = cc
	}
	return out
}
