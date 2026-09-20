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
