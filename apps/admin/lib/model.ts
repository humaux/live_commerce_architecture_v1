export type LedgerRow = {
  product_id: string;
  product_name: string;
  product_description: string;
  sku_id: string;
  code: string;
  status: "active" | "archived";
  currency: string;
  price_minor: number;
  sku_version: number;
  warehouse_id: string;
  on_hand: number;
  reserved: number;
  allocated: number;
  unavailable: number;
  available: number;
  balance_version: number;
  // catalog-media: product row version/status for rename+archive, id of the position-0 photo (null: none).
  product_version: number;
  product_status: "active" | "archived";
  cover_image_id: string | null;
};
export type Page<T> = { items: T[]; next_cursor: string };
export type Warehouse = { id: string; name: string };
export type Store = { id: string; name: string; currency: string };
export type InitialStore = {
  tenant_id: string;
  store_id: string;
  warehouse_id: string;
};
export type APIError = {
  code: string;
  message: string;
  request_id: string;
  retryable: boolean;
  details: Record<string, unknown>;
};
export type PurchaseEntry = {
  product_id: string;
  locale: string;
  state:
    | "configured"
    | "product_inactive"
    | "no_active_sku"
    | "storefront_unavailable"
    | "domain_selection_required";
  url: string;
};
export type WorkspaceData = {
  storeID: string;
  storeName: string;
  fixture: boolean;
  rows: Page<LedgerRow>;
  warehouses: Warehouse[];
  warehouseCursor: string;
  warehouseID: string;
  error: APIError | null;
};

// catalog-media: one product photo as returned by GET products/{id}/images (Go catalog.Image).
export type ProductImage = {
  id: string;
  product_id: string;
  position: number;
  content_type: "image/jpeg" | "image/png" | "image/webp";
  size_bytes: number;
  width: number | null;
  height: number | null;
  version: number;
};
