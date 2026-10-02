package buyerhttp

import (
	"time"

	"livecommerce/internal/checkout"
	"livecommerce/internal/pagination"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

type optionResponse struct {
	MarketID          string `json:"market_id"`
	MarketCode        string `json:"market_code"`
	MarketName        string `json:"market_name"`
	Country           string `json:"country"`
	Currency          string `json:"currency"`
	DeliveryCode      string `json:"delivery_code"`
	Method            string `json:"method"`
	ServiceVersion    int64  `json:"service_version"`
	AllocationVersion int64  `json:"allocation_version"`
	DeliveryKind      string `json:"delivery_kind"`
	Mode              string `json:"mode"`
	NameHans          string `json:"name_hans"`
	NameHant          string `json:"name_hant"`
	NameEN            string `json:"name_en"`
	SortOrder         int    `json:"sort_order"`
	// taiwan-cvs-logistics-v1 §5.1/§16.5: CVS rows only, omitted otherwise (unavailable rows carry available:false + reason).
	PickupSelection string   `json:"pickup_selection,omitempty"`
	PaymentModes    []string `json:"payment_modes,omitempty"`
	StoreSearchURL  string   `json:"store_search_url,omitempty"`
	Available       *bool    `json:"available,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	// storefront-v2 §C: present only when payment_modes lists bank_transfer (the hold the order will keep, 6..168 hours).
	TransferWindowHours int `json:"transfer_window_hours,omitempty"`
	// storefront-v2 §C: the delivery policy's free-shipping threshold (minor units) or null; always present (a hint only, the quote decides).
	FreeShippingThresholdMinor *int64 `json:"free_shipping_threshold_minor"`
	// home-cod R5: present only on a home row whose payment_modes lists cash_on_delivery (the whole-TWD surcharge in minor units; 0 = none).
	CodSurchargeMinor int64 `json:"cod_surcharge_minor,omitempty"`
	// home-cod R5 (P2-4/P2-3): the COD carrier label and the whole-TWD per-order cap in minor units, present only alongside
	// cod_surcharge_minor. The cap lets the storefront omit COD from a row it knows will exceed it (the options request has no basket).
	CodCarrier  string `json:"cod_carrier,omitempty"`
	CodMaxMinor int64  `json:"cod_max_minor,omitempty"`
}

type optionsResponse struct {
	Items      []optionResponse `json:"items"`
	NextCursor string           `json:"next_cursor"`
}

func projectOptions(page pagination.Page[checkout.Option]) optionsResponse {
	out := optionsResponse{Items: make([]optionResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, item := range page.Items {
		out.Items = append(out.Items, optionResponse{
			MarketID: item.MarketID, MarketCode: item.MarketCode, MarketName: item.MarketName,
			Country: item.Country, Currency: item.Currency, DeliveryCode: item.DeliveryCode, Method: item.Method,
			ServiceVersion: item.ServiceVersion, AllocationVersion: item.AllocationVersion,
			DeliveryKind: item.DeliveryKind, Mode: item.Mode, NameHans: item.NameHans, NameHant: item.NameHant,
			NameEN: item.NameEN, SortOrder: item.SortOrder, PickupSelection: item.PickupSelection,
			PaymentModes: item.PaymentModes, StoreSearchURL: item.StoreSearchURL, Available: item.Available, Reason: item.Reason,
			TransferWindowHours: item.TransferWindowHours, FreeShippingThresholdMinor: item.FreeShippingThresholdMinor,
			CodSurchargeMinor: item.CodSurchargeMinor, CodCarrier: item.CodCarrier, CodMaxMinor: item.CodMaxMinor,
		})
	}
	return out
}

type catalogItemResponse struct {
	ProductID   string `json:"product_id"`
	SKUID       string `json:"sku_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SKUCode     string `json:"sku_code"`
	Currency    string `json:"currency"`
	PriceMinor  int64  `json:"price_minor"`
	// Images is the product's photos in display order, [] when none (catalog-media CM4); bytes are fetched from
	// /media/p/{product_id}/{id} on the storefront origin.
	Images []catalogImageResponse `json:"images"`
}

type catalogImageResponse struct {
	ID     string `json:"id"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
}

// catalogResult is one scoped transaction's catalog page plus the store name (same buyer.WithScope read).
type catalogResult struct {
	page pagination.Page[storefront.CatalogItem]
	name string
}

type catalogResponse struct {
	Items      []catalogItemResponse `json:"items"`
	NextCursor string                `json:"next_cursor"`
	// StoreName is the published store's public name (storefront home heading), set by the route, not projectCatalog.
	StoreName string `json:"store_name"`
}

func projectCatalog(page pagination.Page[storefront.CatalogItem]) catalogResponse {
	out := catalogResponse{Items: make([]catalogItemResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, item := range page.Items {
		images := make([]catalogImageResponse, 0, len(item.Images))
		for _, img := range item.Images {
			images = append(images, catalogImageResponse{ID: img.ID, Width: img.Width, Height: img.Height})
		}
		out.Items = append(out.Items, catalogItemResponse{
			ProductID: item.ProductID, SKUID: item.SKUID, Name: item.Name, Description: item.Description,
			SKUCode: item.SKUCode, Currency: item.Currency, PriceMinor: item.PriceMinor, Images: images,
		})
	}
	return out
}

type cartItemResponse struct {
	SKUID    string `json:"sku_id"`
	Quantity int64  `json:"quantity"`
	// LiveUnitPriceMinor is the live (claim-origin) unit price for this line when claims.live_prices still
	// honours its origin, omitted otherwise (the storefront then shows the catalog price).
	LiveUnitPriceMinor int64 `json:"live_unit_price_minor,omitempty"`
}

type cartResponse struct {
	ID       string             `json:"id"`
	Currency string             `json:"currency"`
	Version  int64              `json:"version"`
	Items    []cartItemResponse `json:"items"`
}

func projectCart(cart storefront.Cart) cartResponse {
	out := cartResponse{ID: cart.ID, Currency: cart.Currency, Version: cart.Version, Items: make([]cartItemResponse, 0, len(cart.Items))}
	for _, item := range cart.Items {
		out.Items = append(out.Items, cartItemResponse{SKUID: item.SKUID, Quantity: item.Quantity, LiveUnitPriceMinor: item.LiveUnitPriceMinor})
	}
	return out
}

type lineAmountResponse struct {
	SubtotalMinor int64 `json:"subtotal_minor"`
	DiscountMinor int64 `json:"discount_minor"`
	TaxMinor      int64 `json:"tax_minor"`
	TotalMinor    int64 `json:"total_minor"`
}

type quoteAmountResponse struct {
	SubtotalMinor    int64 `json:"subtotal_minor"`
	DiscountMinor    int64 `json:"discount_minor"`
	ShippingMinor    int64 `json:"shipping_minor"`
	ShippingTaxMinor int64 `json:"shipping_tax_minor"`
	TaxMinor         int64 `json:"tax_minor"`
	TotalMinor       int64 `json:"total_minor"`
}

type quoteLineResponse struct {
	SKUID          string             `json:"sku_id"`
	Code           string             `json:"code"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Quantity       int64              `json:"quantity"`
	UnitPriceMinor int64              `json:"unit_price_minor"`
	Amount         lineAmountResponse `json:"amount"`
}

type quoteResponse struct {
	ID          string              `json:"id"`
	CartID      string              `json:"cart_id"`
	CartVersion int64               `json:"cart_version"`
	MarketID    string              `json:"market_id"`
	Country     string              `json:"country"`
	Method      string              `json:"method"`
	Currency    string              `json:"currency"`
	CreatedAt   time.Time           `json:"created_at"`
	ExpiresAt   time.Time           `json:"expires_at"`
	Lines       []quoteLineResponse `json:"lines"`
	Amount      quoteAmountResponse `json:"amount"`
	Promotion   *promotionResponse  `json:"promotion,omitempty"`
}

func projectQuoteLines(lines []storefront.QuoteLine) []quoteLineResponse {
	out := make([]quoteLineResponse, 0, len(lines))
	for _, line := range lines {
		out = append(out, quoteLineResponse{
			SKUID: line.SKUID, Code: line.Code, Name: line.Name, Description: line.Description,
			Quantity: line.Quantity, UnitPriceMinor: line.UnitPriceMinor,
			Amount: lineAmountResponse{SubtotalMinor: line.Amount.SubtotalMinor, DiscountMinor: line.Amount.DiscountMinor, TaxMinor: line.Amount.TaxMinor, TotalMinor: line.Amount.TotalMinor},
		})
	}
	return out
}

func projectQuote(quote storefront.Quote) quoteResponse {
	out := quoteResponse{
		ID: quote.ID, CartID: quote.CartID, CartVersion: quote.CartVersion,
		MarketID: quote.Policy.MarketID, Country: quote.Policy.Country, Method: quote.Policy.Method,
		Currency: quote.Currency, CreatedAt: quote.CreatedAt, ExpiresAt: quote.ExpiresAt,
		Lines: projectQuoteLines(quote.Lines),
		Amount: quoteAmountResponse{
			SubtotalMinor: quote.Amount.SubtotalMinor, DiscountMinor: quote.Amount.DiscountMinor,
			ShippingMinor: quote.Amount.ShippingMinor, ShippingTaxMinor: quote.Amount.ShippingTaxMinor,
			TaxMinor: quote.Amount.TaxMinor, TotalMinor: quote.Amount.TotalMinor,
		},
		Promotion: projectPromotion(quote.Promotion),
	}
	return out
}

type homeAddressResponse struct {
	Region     string `json:"region"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
}

type pickupResponse struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Country   string `json:"country"`
}

type destinationResponse struct {
	ID            string              `json:"id"`
	Version       int64               `json:"version"`
	CartID        string              `json:"cart_id"`
	CartVersion   int64               `json:"cart_version"`
	Kind          string              `json:"kind"`
	Country       string              `json:"country"`
	RecipientName string              `json:"recipient_name"`
	Phone         string              `json:"phone"`
	HomeAddress   homeAddressResponse `json:"home_address"`
	Pickup        *pickupResponse     `json:"pickup,omitempty"`
	SelectedAt    time.Time           `json:"selected_at"`
	ExpiresAt     time.Time           `json:"expires_at"`
}

func projectDestination(destination storefront.Destination) destinationResponse {
	out := destinationResponse{
		ID: destination.ID, Version: destination.Version, CartID: destination.CartID,
		CartVersion: destination.CartVersion, Kind: destination.Kind, Country: destination.Country,
		RecipientName: destination.RecipientName, Phone: destination.Phone,
		HomeAddress: homeAddressResponse{
			Region: destination.HomeAddress.Region, City: destination.HomeAddress.City,
			PostalCode: destination.HomeAddress.PostalCode, Line1: destination.HomeAddress.Line1,
			Line2: destination.HomeAddress.Line2,
		},
		SelectedAt: destination.SelectedAt, ExpiresAt: destination.ExpiresAt,
	}
	if destination.Pickup != nil {
		out.Pickup = &pickupResponse{
			ID: destination.Pickup.ID, Kind: destination.Pickup.Kind, Namespace: destination.Pickup.Namespace,
			Code: destination.Pickup.Code, Name: destination.Pickup.Name, Address: destination.Pickup.Address,
			Country: destination.Pickup.Country,
		}
	}
	return out
}

type checkoutResponse struct {
	OrderID       string    `json:"order_id"`
	HoldExpiresAt time.Time `json:"hold_expires_at"`
	// §16.2: how the order is paid and its state at placement (pay_at_pickup orders are CONFIRMED with no payment step).
	PaymentMode     string `json:"payment_mode"`
	CommercialState string `json:"commercial_state"`
}

func projectCheckout(result checkout.Result) checkoutResponse {
	return checkoutResponse{OrderID: result.OrderID, HoldExpiresAt: result.ExpiresAt,
		PaymentMode: result.PaymentMode, CommercialState: result.CommercialState}
}

type orderQuoteResponse struct {
	Currency  string              `json:"currency"`
	Lines     []quoteLineResponse `json:"lines"`
	Amount    quoteAmountResponse `json:"amount"`
	Promotion *promotionResponse  `json:"promotion,omitempty"`
}

// promotionResponse is the buyer's view of the code a quote/order was priced with (storefront-v2 §F). Present only when a code applied (the
// key is absent otherwise, so code-less responses keep their exact pre-0091 shape); the merchant's internal ids and version never leave Go.
type promotionResponse struct {
	Code       string `json:"code"`
	Kind       string `json:"kind"`
	Percent    int64  `json:"percent"`
	FixedMinor int64  `json:"fixed_minor"`
}

func projectPromotion(p *pricing.Promo) *promotionResponse {
	if p == nil {
		return nil
	}
	return &promotionResponse{Code: p.Code, Kind: p.Kind, Percent: p.Percent, FixedMinor: p.FixedMinor}
}

type orderPickupResponse struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Country   string `json:"country"`
}

type orderDestinationResponse struct {
	Kind          string               `json:"kind"`
	Country       string               `json:"country"`
	RecipientName string               `json:"recipient_name"`
	Phone         string               `json:"phone"`
	HomeAddress   homeAddressResponse  `json:"home_address"`
	Pickup        *orderPickupResponse `json:"pickup,omitempty"`
}

type orderServiceResponse struct {
	Code         string `json:"code"`
	NameHans     string `json:"name_hans"`
	NameHant     string `json:"name_hant"`
	NameEN       string `json:"name_en"`
	DeliveryKind string `json:"delivery_kind"`
	Mode         string `json:"mode"`
}

type orderSnapshotResponse struct {
	Quote       orderQuoteResponse       `json:"quote"`
	Destination orderDestinationResponse `json:"destination"`
	Service     orderServiceResponse     `json:"service"`
}

type orderResponse struct {
	OrderID          string                `json:"order_id"`
	CartID           string                `json:"cart_id"`
	CartVersion      int64                 `json:"cart_version"`
	CommercialState  string                `json:"commercial_state"`
	FulfillmentState string                `json:"fulfillment_state"`
	HoldExpiresAt    *time.Time            `json:"hold_expires_at,omitempty"`
	Snapshot         orderSnapshotResponse `json:"snapshot"`
	// Shipment is always emitted, null unless the merchant's manual shipment head is SHIPPED
	// (manual-fulfilment-v1 §5.2); checkout.Get reads it under buyer RLS without merchant-only columns.
	Shipment *checkout.BuyerShipment `json:"shipment"`
	// taiwan-cvs-logistics-v1 §5.3/§16: payment mode, pay-at-pickup collection state (null for card) and the current ECPay attempt.
	PaymentMode     string                     `json:"payment_mode"`
	CollectionState *string                    `json:"collection_state"`
	CVSShipment     *checkout.BuyerCVSShipment `json:"cvs_shipment"`
	// home-cod R5: present only on cash_on_delivery orders — the cash due on delivery (total + surcharge) and the surcharge folded
	// into it, so the order page states 「到貨需付 NT$X（含貨到付款手續費 NT$Y）」.
	CodCollectMinor   int64 `json:"cod_collect_minor,omitempty"`
	CodSurchargeMinor int64 `json:"cod_surcharge_minor,omitempty"`
}

func projectOrder(order checkout.Order) orderResponse {
	quote := order.Snapshot.Quote
	destination := order.Snapshot.Destination
	out := orderResponse{
		OrderID: order.OrderID, CommercialState: order.CommercialState, FulfillmentState: order.FulfillmentState,
		CartID: quote.CartID, CartVersion: quote.CartVersion, Shipment: order.Shipment,
		PaymentMode: order.PaymentMode, CollectionState: order.CollectionState, CVSShipment: order.CVSShipment,
		CodCollectMinor: order.CodCollectMinor, CodSurchargeMinor: order.CodSurchargeMinor,
		Snapshot: orderSnapshotResponse{
			Quote: orderQuoteResponse{
				Currency: quote.Currency, Lines: projectQuoteLines(quote.Lines),
				Amount: quoteAmountResponse{
					SubtotalMinor: quote.Amount.SubtotalMinor, DiscountMinor: quote.Amount.DiscountMinor,
					ShippingMinor: quote.Amount.ShippingMinor, ShippingTaxMinor: quote.Amount.ShippingTaxMinor,
					TaxMinor: quote.Amount.TaxMinor, TotalMinor: quote.Amount.TotalMinor,
				},
				Promotion: projectPromotion(quote.Promotion),
			},
			Destination: orderDestinationResponse{
				Kind: destination.Kind, Country: destination.Country, RecipientName: destination.RecipientName,
				Phone: destination.Phone,
				HomeAddress: homeAddressResponse{
					Region: destination.HomeAddress.Region, City: destination.HomeAddress.City,
					PostalCode: destination.HomeAddress.PostalCode, Line1: destination.HomeAddress.Line1,
					Line2: destination.HomeAddress.Line2,
				},
			},
			Service: orderServiceResponse{
				Code: order.Snapshot.Service.Code, NameHans: order.Snapshot.Service.NameHans,
				NameHant: order.Snapshot.Service.NameHant, NameEN: order.Snapshot.Service.NameEN,
				DeliveryKind: order.Snapshot.Service.DeliveryKind, Mode: order.Snapshot.Service.Mode,
			},
		},
	}
	if order.CommercialState == "DRAFT" {
		expiresAt := order.ExpiresAt
		out.HoldExpiresAt = &expiresAt
	}
	if destination.Pickup != nil {
		out.Snapshot.Destination.Pickup = &orderPickupResponse{
			Kind: destination.Pickup.Kind, Namespace: destination.Pickup.Namespace,
			Code: destination.Pickup.Code, Name: destination.Pickup.Name,
			Address: destination.Pickup.Address, Country: destination.Pickup.Country,
		}
	}
	return out
}

type orderSummaryResponse struct {
	OrderID          string    `json:"order_id"`
	CreatedAt        time.Time `json:"created_at"`
	CartID           string    `json:"cart_id"`
	CartVersion      int64     `json:"cart_version"`
	CommercialState  string    `json:"commercial_state"`
	FulfillmentState string    `json:"fulfillment_state"`
	Currency         string    `json:"currency"`
	TotalMinor       int64     `json:"total_minor"`
}

type ordersResponse struct {
	Items      []orderSummaryResponse `json:"items"`
	NextCursor string                 `json:"next_cursor"`
}

func projectOrders(page pagination.Page[checkout.OrderSummary]) ordersResponse {
	out := ordersResponse{Items: make([]orderSummaryResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, order := range page.Items {
		out.Items = append(out.Items, orderSummaryResponse{
			OrderID: order.OrderID, CreatedAt: order.CreatedAt, CartID: order.CartID, CartVersion: order.CartVersion,
			CommercialState: order.CommercialState, FulfillmentState: order.FulfillmentState,
			Currency: order.Currency, TotalMinor: order.TotalMinor,
		})
	}
	return out
}
