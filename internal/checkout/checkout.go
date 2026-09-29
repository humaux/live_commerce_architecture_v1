// Package checkout owns the trusted buyer checkout transaction. Its pool is a
// separate SQL authority; buyer capability scope remains the buyer identity.
package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/inventory"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

var checkoutKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)
var errCheckoutDatabase = errors.New("checkout database unavailable")

const holdDuration = 15 * time.Minute

type Service struct {
	pool *pgxpool.Pool
	jobs *river.Client[pgx.Tx]
}

type Input struct {
	QuoteID           string `json:"quote_id"`
	DestinationID     string `json:"destination_id"`
	CartVersion       int64  `json:"cart_version"`
	ServiceVersion    int64  `json:"service_version"`
	AllocationVersion int64  `json:"allocation_version"`
}

type Result struct {
	OrderID       string    `json:"order_id"`
	ReservationID string    `json:"reservation_id"`
	Generation    int64     `json:"generation"`
	ExpiresAt     time.Time `json:"expires_at"`
	JobID         int64     `json:"job_id"`
}

type Snapshot struct {
	Quote       storefront.Quote       `json:"quote"`
	Destination storefront.Destination `json:"destination"`
	Service     fulfillment.Service    `json:"service"`
	Allocation  fulfillment.Allocation `json:"allocation"`
}

type Order struct {
	Result
	CommercialState  string   `json:"commercial_state"`
	FulfillmentState string   `json:"fulfillment_state"`
	Snapshot         Snapshot `json:"snapshot"`
	// Shipment is always emitted: null unless the order's manual shipment head is SHIPPED
	// (manual-fulfilment-v1 §5.2). Merchant-only fields (note, void_reason, principal) are never read.
	Shipment *BuyerShipment `json:"shipment"`
}

// BuyerShipment is the merchant's attestation that the parcel was dispatched with this carrier and
// tracking (MERCHANT_SHIPPED); it is never in-transit or delivered evidence (I13).
type BuyerShipment struct {
	Status         string    `json:"status"`
	CarrierCode    string    `json:"carrier_code"`
	CarrierName    *string   `json:"carrier_name"`
	TrackingNumber string    `json:"tracking_number"`
	TrackingURL    *string   `json:"tracking_url"`
	RecordedAt     time.Time `json:"recorded_at"`
}

func New(ctx context.Context, checkoutPool *pgxpool.Pool, jobs *river.Client[pgx.Tx]) (*Service, error) {
	if ctx == nil || jobs == nil {
		return nil, command.ErrInvalid
	}
	if err := platform.ValidateCheckoutPool(ctx, checkoutPool); err != nil {
		return nil, err
	}
	return &Service{pool: checkoutPool, jobs: jobs}, nil
}

// Begin authenticates before the private receipt lookup. An exact replay
// returns its saved IDs without extending an expired hold or rechecking market.
func (s *Service) Begin(ctx context.Context, token, storeID, key string, in Input) (Result, error) {
	if ctx == nil || s == nil || s.pool == nil || s.jobs == nil || !checkoutKey.MatchString(key) || !validInput(in) {
		return Result{}, command.ErrInvalid
	}
	request, err := json.Marshal(in)
	if err != nil {
		return Result{}, command.ErrInvalid
	}
	digest := sha256.Sum256(request)
	tokenHash := sha256.Sum256([]byte(token))
	var out Result
	err = buyer.WithScope(ctx, s.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		if err := advisory(callCtx, tx, "checkout.begin|"+scope.TenantID+"|"+scope.StoreID+"|"+scope.OwnerID+"|"+key); err != nil {
			return err
		}
		saved, found, err := readReceipt(callCtx, tx, scope, key, digest)
		if err != nil {
			return err
		}
		if found {
			if err = checkCapability(callCtx, tx, tokenHash[:], storeID, scope); err != nil {
				return err
			}
			out = saved
			return nil
		}
		if err = advisory(callCtx, tx, "checkout.cart|"+scope.TenantID+"|"+scope.StoreID+"|"+scope.OwnerID); err != nil {
			return err
		}
		quote, err := storefront.RevalidateQuote(callCtx, tx, scope, in.QuoteID, in.CartVersion)
		if err != nil {
			return err
		}
		var active bool
		err = tx.QueryRow(callCtx, `SELECT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2
			AND owner_id=$3 AND cart_id=$4 AND cart_version=$5
			AND commercial_state IN ('DRAFT','AWAITING_PAYMENT','CONFIRMED'))`,
			scope.TenantID, scope.StoreID, scope.OwnerID, quote.CartID, quote.CartVersion).Scan(&active)
		if err != nil {
			return err
		}
		if active {
			return command.ErrConflict
		}
		historical, err := storefront.GetDestination(callCtx, tx, scope, in.DestinationID)
		if err != nil {
			return err
		}
		destination, err := storefront.RevalidateDestination(callCtx, tx, scope, in.DestinationID, in.CartVersion, quote.Policy.Country, historical.Kind)
		if err != nil {
			return err
		}
		code, ok := strings.CutPrefix(quote.Policy.Method, "delivery:")
		if !ok || code == "" {
			return command.ErrConflict
		}
		service, err := lockService(callCtx, tx, scope, quote.Policy.MarketID, quote.Policy.Country, code)
		if err != nil {
			return err
		}
		if service.Version != in.ServiceVersion || !service.Enabled || !service.Visible || service.Mode != "MANUAL" ||
			service.MarketID != quote.Policy.MarketID || service.Country != quote.Policy.Country ||
			service.Currency != quote.Currency || service.PolicyMethod != quote.Policy.Method ||
			service.PolicyVersion != quote.Policy.Version || service.DeliveryKind != destination.Kind ||
			service.BindingID != "" || service.BindingVersion != 0 {
			return command.ErrConflict
		}
		allocation, err := lockAllocation(callCtx, tx, scope, service.MarketID, service.Country, service.Code)
		if err != nil {
			return err
		}
		if allocation.Version != in.AllocationVersion || len(allocation.WarehouseIDs) == 0 {
			return command.ErrConflict
		}
		plan, err := planLocked(callCtx, tx, allocation, quote)
		if err != nil {
			return err
		}
		var now time.Time
		if err = tx.QueryRow(callCtx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if quote.CreatedAt.After(now) || !quote.ExpiresAt.After(now) || destination.SelectedAt.After(now) ||
			!destination.ExpiresAt.After(now) ||
			(destination.Pickup != nil && !destination.Pickup.ValidUntil.After(now)) {
			return command.ErrConflict
		}
		if err = checkCapability(callCtx, tx, tokenHash[:], storeID, scope); err != nil {
			return err
		}
		var orderID string
		if err = tx.QueryRow(callCtx, `SELECT gen_random_uuid()::text`).Scan(&orderID); err != nil {
			return err
		}
		job, err := s.jobs.InsertTx(callCtx, tx, expiryArgs{OrderID: orderID, Generation: 1, Version: 1},
			&river.InsertOpts{Queue: jobqueue.CheckoutExpiry, ScheduledAt: now.Add(holdDuration)})
		if err != nil {
			return err
		}
		snapshotJSON, err := json.Marshal(Snapshot{Quote: quote, Destination: destination, Service: service, Allocation: allocation})
		if err != nil {
			return command.ErrInvalid
		}
		linesJSON, err := json.Marshal(plan)
		if err != nil {
			return command.ErrInvalid
		}
		var response []byte
		err = tx.QueryRow(callCtx, `SELECT checkout.begin_hold($1,$2::uuid,$3,$4,$5::uuid,$6::jsonb,$7::jsonb,$8::bigint)`,
			tokenHash[:], storeID, key, digest[:], orderID, string(snapshotJSON), string(linesJSON), job.Job.ID).Scan(&response)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(response, &out); err != nil {
			return command.ErrConflict
		}
		command.InLocalTime(&out) // same location as row-scanned reads/replays
		if out.OrderID != orderID || out.ReservationID != orderID || out.Generation != 1 ||
			out.JobID != job.Job.ID || !out.ExpiresAt.After(now) || out.ExpiresAt.After(now.Add(holdDuration+5*time.Second)) {
			return command.ErrConflict
		}
		return nil
	})
	if err != nil {
		return Result{}, safeError(ctx, err)
	}
	return out, nil
}

// Get reads the immutable checkout snapshot alongside its current order state.
func (s *Service) Get(ctx context.Context, token, storeID, orderID string) (Order, error) {
	if ctx == nil || s == nil || s.pool == nil || !command.ValidID(orderID) {
		return Order{}, command.ErrInvalid
	}
	var out Order
	tokenHash := sha256.Sum256([]byte(token))
	err := buyer.WithScope(ctx, s.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var snapshotJSON []byte
		err := tx.QueryRow(callCtx, `SELECT id::text,id::text,generation,expires_at,job_id,
			commercial_state,fulfillment_state,snapshot FROM checkout.orders
			WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4`,
			scope.TenantID, scope.StoreID, scope.OwnerID, orderID).Scan(&out.OrderID, &out.ReservationID,
			&out.Generation, &out.ExpiresAt, &out.JobID, &out.CommercialState, &out.FulfillmentState, &snapshotJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return command.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err = json.Unmarshal(snapshotJSON, &out.Snapshot); err != nil {
			return command.ErrConflict
		}
		// The snapshot is SQL-built JSON; align it with pgx-scanned quote reads.
		command.InLocalTime(&out.Snapshot)
		if out.OrderID != orderID || out.Generation < 1 || out.JobID < 1 ||
			out.Snapshot.Quote.ID == "" || out.Snapshot.Destination.ID == "" {
			return command.ErrConflict
		}
		// fulfillment.manual_shipment_*: buyer RLS (owner scope) and column grants (0063) hide the
		// merchant-only columns; only a SHIPPED head is shown, a voided head reads as null.
		var shipment BuyerShipment
		err = tx.QueryRow(callCtx, `SELECT v.status,v.carrier_code,v.carrier_name,v.tracking_number,v.tracking_url,v.recorded_at
			FROM fulfillment.manual_shipment_heads h JOIN fulfillment.manual_shipment_versions v
			 ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id AND v.version=h.current_version
			WHERE h.tenant_id=$1 AND h.store_id=$2 AND h.owner_id=$3 AND h.order_id=$4 AND v.status='SHIPPED'`,
			scope.TenantID, scope.StoreID, scope.OwnerID, orderID).Scan(&shipment.Status, &shipment.CarrierCode,
			&shipment.CarrierName, &shipment.TrackingNumber, &shipment.TrackingURL, &shipment.RecordedAt)
		if err == nil {
			shipment.RecordedAt = shipment.RecordedAt.UTC()
			out.Shipment = &shipment
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// The deferred 0063 guard makes MERCHANT_SHIPPED <=> SHIPPED head; a disagreement is drift, not data.
		if (out.Shipment != nil) != (out.FulfillmentState == "MERCHANT_SHIPPED") {
			return command.ErrConflict
		}
		return checkCapability(callCtx, tx, tokenHash[:], storeID, scope)
	})
	if err != nil {
		return Order{}, safeError(ctx, err)
	}
	return out, nil
}

func validInput(in Input) bool {
	return command.ValidID(in.QuoteID) && command.ValidID(in.DestinationID) &&
		in.CartVersion > 0 && in.ServiceVersion > 0 && in.AllocationVersion > 0
}

func readReceipt(ctx context.Context, tx pgx.Tx, scope buyer.Scope, key string, digest [32]byte) (Result, bool, error) {
	var savedHash, response []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response FROM checkout.command_results
		WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND operation='checkout.begin' AND idempotency_key=$4`,
		scope.TenantID, scope.StoreID, scope.OwnerID, key).Scan(&savedHash, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if !bytes.Equal(savedHash, digest[:]) {
		return Result{}, false, command.ErrConflict
	}
	var out Result
	if err = json.Unmarshal(response, &out); err != nil || !command.ValidID(out.OrderID) ||
		out.ReservationID != out.OrderID || out.Generation < 1 || out.JobID < 1 || out.ExpiresAt.IsZero() {
		return Result{}, false, command.ErrConflict
	}
	command.InLocalTime(&out) // replay equals the first result on any host TZ
	return out, true, nil
}

func checkCapability(ctx context.Context, tx pgx.Tx, hash []byte, storeID string, expected buyer.Scope) error {
	var actual buyer.Scope
	err := tx.QueryRow(ctx, `SELECT tenant_id::text,store_id::text,owner_id::text,session_id::text
		FROM buyer.resolve_scope($1,$2::uuid)`, hash, storeID).Scan(&actual.TenantID, &actual.StoreID, &actual.OwnerID, &actual.SessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return buyer.ErrUnauthorized
	}
	if err != nil {
		return err
	}
	if actual != expected {
		return buyer.ErrUnauthorized
	}
	return nil
}

func advisory(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key)
	return err
}

func safeError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, known := range []error{command.ErrInvalid, command.ErrConflict, command.ErrNotFound,
		command.ErrInsufficient, buyer.ErrUnauthorized} {
		if errors.Is(err, known) {
			return known
		}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "PT400":
			return command.ErrInvalid
		case "PT401":
			return buyer.ErrUnauthorized
		case "PT402":
			return command.ErrInsufficient
		case "PT404":
			return command.ErrNotFound
		case "PT409":
			return command.ErrConflict
		}
	}
	return errCheckoutDatabase
}

func lockService(ctx context.Context, tx pgx.Tx, scope buyer.Scope, marketID, country, code string) (fulfillment.Service, error) {
	var version int64
	err := tx.QueryRow(ctx, `SELECT current_version FROM fulfillment.service_heads
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 FOR SHARE`,
		scope.TenantID, scope.StoreID, marketID, country, code).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return fulfillment.Service{}, command.ErrConflict
	}
	if err != nil {
		return fulfillment.Service{}, err
	}
	var out fulfillment.Service
	err = tx.QueryRow(ctx, `SELECT market_id::text,country,code,version,policy_method,policy_version,currency,
		name_hans,name_hant,name_en,delivery_kind,mode,enabled,visible,sort_order,
		coalesce(binding_id::text,''),coalesce(binding_version,0)
		FROM fulfillment.service_versions WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3
		AND country=$4 AND code=$5 AND version=$6`, scope.TenantID, scope.StoreID, marketID, country, code, version).
		Scan(&out.MarketID, &out.Country, &out.Code, &out.Version, &out.PolicyMethod, &out.PolicyVersion,
			&out.Currency, &out.NameHans, &out.NameHant, &out.NameEN, &out.DeliveryKind, &out.Mode,
			&out.Enabled, &out.Visible, &out.SortOrder, &out.BindingID, &out.BindingVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return fulfillment.Service{}, command.ErrConflict
	}
	return out, err
}

func lockAllocation(ctx context.Context, tx pgx.Tx, scope buyer.Scope, marketID, country, code string) (fulfillment.Allocation, error) {
	var version int64
	err := tx.QueryRow(ctx, `SELECT current_version FROM fulfillment.allocation_heads
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 FOR SHARE`,
		scope.TenantID, scope.StoreID, marketID, country, code).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return fulfillment.Allocation{}, command.ErrConflict
	}
	if err != nil {
		return fulfillment.Allocation{}, err
	}
	var out fulfillment.Allocation
	var count int
	err = tx.QueryRow(ctx, `SELECT market_id::text,country,code,version,service_version,warehouse_count
		FROM fulfillment.allocation_versions WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3
		AND country=$4 AND code=$5 AND version=$6`, scope.TenantID, scope.StoreID, marketID, country, code, version).
		Scan(&out.MarketID, &out.Country, &out.Code, &out.Version, &out.ServiceVersion, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return fulfillment.Allocation{}, command.ErrConflict
	}
	if err != nil {
		return fulfillment.Allocation{}, err
	}
	rows, err := tx.Query(ctx, `SELECT warehouse_id::text FROM fulfillment.allocation_warehouses
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 AND version=$6
		ORDER BY position`, scope.TenantID, scope.StoreID, marketID, country, code, version)
	if err != nil {
		return fulfillment.Allocation{}, err
	}
	defer rows.Close()
	out.WarehouseIDs = make([]string, 0, count)
	seen := make(map[string]bool, count)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return fulfillment.Allocation{}, err
		}
		if !command.ValidID(id) || seen[id] {
			return fulfillment.Allocation{}, command.ErrConflict
		}
		seen[id] = true
		out.WarehouseIDs = append(out.WarehouseIDs, id)
	}
	if err = rows.Err(); err != nil {
		return fulfillment.Allocation{}, err
	}
	if count != len(out.WarehouseIDs) || count < 1 || count > 16 || out.Version != version ||
		out.MarketID != marketID || out.Country != country || out.Code != code {
		return fulfillment.Allocation{}, command.ErrConflict
	}
	return out, nil
}

func planLocked(ctx context.Context, tx pgx.Tx, allocation fulfillment.Allocation, quote storefront.Quote) ([]inventory.Line, error) {
	warehouses := sortedIDs(allocation.WarehouseIDs)
	for _, id := range warehouses {
		var active bool
		err := tx.QueryRow(ctx, `SELECT active FROM inventory.lock_warehouse($1::uuid)`, id).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || !active && err == nil {
			return nil, command.ErrConflict
		}
		if err != nil {
			return nil, err
		}
	}
	demands := make([]inventory.Demand, 0, len(quote.Lines))
	skus := make([]string, 0, len(quote.Lines))
	for _, line := range quote.Lines {
		demands = append(demands, inventory.Demand{SKUID: line.SKUID, Quantity: line.Quantity})
		skus = append(skus, line.SKUID)
	}
	sort.Strings(skus)
	balances := make([]inventory.Balance, 0, len(warehouses)*len(skus))
	for _, warehouse := range warehouses {
		for _, sku := range skus {
			var balance inventory.Balance
			err := tx.QueryRow(ctx, `SELECT warehouse_id::text,sku_id::text,on_hand,reserved,allocated,unavailable,version
				FROM inventory.lock_balance($1::uuid,$2::uuid)`, warehouse, sku).
				Scan(&balance.WarehouseID, &balance.SKUID, &balance.OnHand, &balance.Reserved,
					&balance.Allocated, &balance.Unavailable, &balance.Version)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return nil, err
			}
			balances = append(balances, balance)
		}
	}
	return inventory.PlanAllocation(allocation.WarehouseIDs, demands, balances)
}

// sortedIDs makes all warehouse and SKU lock orders independent of client order.
func sortedIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
