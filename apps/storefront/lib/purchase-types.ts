// Wire shapes of the buyer purchase BFF routes (cart, catalog, checkout-options, quote, destination, checkout, order), split out of
// purchase.ts (G-UI3 legacy ceiling). Types only, plus the carrier-code label list the Shipment type is derived from; every validator
// and transport function stays in purchase.ts, which re-exports all of this so imports of "./purchase.ts" are unchanged.
import type {
  BuyerCvsShipment,
  CollectionState,
  CvsKind,
  PaymentMode,
} from "./cvs-contract.ts";
import type { QuotePromotion } from "./promo-contract.ts";

export type Item = { sku_id: string; quantity: number; live_unit_price_minor?: number };
export type Cart = {
  id: string;
  currency: string;
  version: number;
  items: Item[];
};
export type Product = {
  product_id: string;
  sku_id: string;
  name: string;
  description: string;
  sku_code: string;
  currency: string;
  price_minor: number;
  // catalog-media CM4: the product's photos in display order ([] when none). Optional so older fixtures still parse.
  images?: ProductImageMeta[];
};
export type ProductImageMeta = { id: string; width: number | null; height: number | null };
export type Option = {
  market_id: string;
  country: string;
  currency: string;
  method: string;
  delivery_kind: string;
  service_version: number;
  allocation_version: number;
  mode: "MANUAL" | "API";
  name_hans: string;
  name_hant: string;
  name_en: string;
  sort_order: number;
  // CVS rows only (taiwan-cvs-logistics-v1 §5.1): how the store is chosen and which payment modes exist.
  pickup_selection?: "ecpay_map" | "buyer_entered";
  // Home rows carry payment_modes only when the store enabled bank transfer (storefront-v2 §C); absent = card only.
  payment_modes?: PaymentMode[];
  store_search_url?: string;
  // Present exactly when payment_modes lists bank_transfer: hours the stock stays reserved for the transfer (6..168).
  transfer_window_hours?: number;
  // home-cod R5 (migration 0107): present on a home row that lists cash_on_delivery with a non-zero surcharge (Go's
  // omitempty hides a zero surcharge); the whole-TWD surcharge in minor units the buyer pays on delivery on top of the
  // order total. The quote stays the only authority on the order total (I05); this field only lets the UI show the fee.
  cod_surcharge_minor?: number;
  cod_max_minor?: number;
  cod_carrier?: "black_cat" | "hsinchu";
  // Per-policy free-delivery threshold in minor units (storefront-v2 §A/§C): Go emits it on every row (number or null,
  // internal/checkout/options.go; the MOCK fake of tests/storefront/shop-fake-api.mjs mirrors it). The cart page and the delivery step
  // show their "add X more for free delivery" hint only when a row carries a positive one. The quote still decides shipping, never this.
  free_shipping_threshold_minor?: number | null;
};
// A configured chain that cannot be sold yet (ECPay chain gate, §5.1): listed but disabled ("Coming soon").
// Go sends only the identity/label fields plus available:false and a reason; versions are not needed.
export type UnavailableOption = Pick<
  Option,
  | "market_id"
  | "country"
  | "currency"
  | "method"
  | "delivery_kind"
  | "name_hans"
  | "name_hant"
  | "name_en"
  | "sort_order"
> & { available: false; reason: "coming_soon" | "temporarily_unavailable" };
export type OptionRow = Option | UnavailableOption;
export type Quote = {
  id: string;
  cart_id: string;
  cart_version: number;
  market_id: string;
  country: string;
  method: string;
  currency: string;
  expires_at: string;
  lines: {
    sku_id: string;
    name: string;
    code: string;
    quantity: number;
    unit_price_minor: number;
  }[];
  amount: {
    subtotal_minor: number;
    discount_minor: number;
    shipping_minor: number;
    shipping_tax_minor: number;
    tax_minor: number;
    total_minor: number;
  };
  // storefront-v2 §F: present only when a discount code applied (the server quote already carries its discount_minor).
  promotion?: QuotePromotion;
};
export type CartWrite = { expected_version: number; items: Item[] };
export type QuoteWrite = {
  cart_version: number;
  market_id: string;
  country: string;
  method: string;
  // storefront-v2 §F: optional canonical (upper-case) discount code; omitted = no code. The server prices it, never this field.
  promo_code?: string;
};
export type Pending = { v: 1; context: string; key: string } & (
  | { kind: "cart"; body: CartWrite }
  | { kind: "next-cart"; body: CartWrite }
  | { kind: "quote"; body: QuoteWrite }
  | { kind: "destination"; body: DestinationMarker }
  | { kind: "checkout"; body: CheckoutWrite }
);
export type HomeAddress = {
  region: string;
  city: string;
  postal_code: string;
  line1: string;
  line2: string;
};
export type DestinationKind = "home" | CvsKind;
export type DestinationMarker = {
  expected_version: number;
  cart_version: number;
  kind: DestinationKind;
  country: string;
};
// CVS destinations (§16.1/§5.2) carry the pickup_id from a verified map selection or a buyer-entered store,
// TW only, and an all-empty home_address; home destinations carry no pickup_id (Go validDestinationInput).
export type DestinationWrite = DestinationMarker & {
  recipient_name: string;
  phone: string;
  home_address: HomeAddress;
  pickup_id?: string;
};
export type DestinationDetails = {
  kind: DestinationKind;
  country: string;
  recipient_name: string;
  phone: string;
  home_address: HomeAddress;
  pickup?: {
    id?: string;
    verification_kind?: string;
    kind: string;
    namespace: string;
    code: string;
    name: string;
    address: string;
    country: string;
  };
};
export type Destination = DestinationDetails & {
  id: string;
  version: number;
  cart_id: string;
  cart_version: number;
  selected_at: string;
  expires_at: string;
};
export type CheckoutWrite = {
  quote_id: string;
  destination_id: string;
  cart_version: number;
  service_version: number;
  allocation_version: number;
  // Set for CVS options (§16.2) and for a home option that offers several modes (storefront-v2 §C); a card-only home body is unchanged
  // and Go reads a missing mode as card.
  payment_mode?: PaymentMode;
  // Optional buyer email (storefront-v2 §C), trimmed, validated by validBuyerEmail; Go and the SQL CHECK re-validate.
  buyer_email?: string;
  expected_cod_surcharge_minor?: number;
};
// manual-fulfilment-v1 §3.1 carrier codes; a label, never an integration binding.
export const CARRIER_CODES = [
  "seven_eleven_cvs",
  "familymart_cvs",
  "hilife_cvs",
  "okmart_cvs",
  "sf_express",
  "black_cat",
  "hsinchu",
  "chunghwa_post",
  "other",
] as const;
export type CarrierCode = (typeof CARRIER_CODES)[number];
// Buyer shipment (manual-fulfilment-v1 §5.2): the seller's attestation of dispatch, never
// in-transit or delivered evidence. Mirrors internal/checkout.BuyerShipment.
export type Shipment = {
  status: "SHIPPED";
  carrier_code: CarrierCode;
  carrier_name: string | null;
  tracking_number: string;
  tracking_url: string | null;
  recorded_at: string;
};
export type Order = {
  order_id: string;
  cart_id: string;
  cart_version: number;
  commercial_state: "DRAFT" | "AWAITING_PAYMENT" | "AWAITING_TRANSFER" | "AWAITING_COLLECTION" | "CONFIRMED" | "CANCELLED";
  fulfillment_state:
    | "MANUAL_UNASSIGNED"
    | "CANCELLED"
    | "PAID_ALLOCATION_FAILED"
    | "MERCHANT_SHIPPED"
    | "PROVIDER_LABEL_CREATED";
  // Go always emits it (null unless the head is SHIPPED); absent is read as null.
  shipment?: Shipment | null;
  // taiwan-cvs-logistics-v1 §16.2/§5.3: Go always emits all three; absent reads as card / null / null so
  // an order from before the CVS release still validates. pay_at_pickup <=> collection_state != null.
  payment_mode?: PaymentMode;
  collection_state?: CollectionState | null;
  cod_collect_minor?: number;
  cod_surcharge_minor?: number;
  // Immutable order-time carrier; never substitute the store's current COD settings.
  cod_carrier?: "black_cat" | "hsinchu" | null;
  cvs_shipment?: BuyerCvsShipment | null;
  hold_expires_at?: string;
  snapshot: {
    quote: Pick<Quote, "currency" | "lines" | "amount" | "promotion">;
    destination: DestinationDetails;
    service: {
      code: string;
      name_hans: string;
      name_hant: string;
      name_en: string;
      delivery_kind: string;
      mode: string;
    };
  };
};
export type OrderSummary = Pick<
  Order,
  | "order_id"
  | "cart_id"
  | "cart_version"
  | "commercial_state"
  | "fulfillment_state"
> & { created_at: string; currency: string; total_minor: number };
