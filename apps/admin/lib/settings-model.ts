// Purpose: Types for the store settings wizard: markets, delivery options, policies and status rows (no PAYUNi since PAY-RM2).
// Depends on: ./model types (APIError, Page, Store).
// Used by: components/SettingsWizard.tsx, LiveSettings.tsx, BuyerPanel.tsx, lib/settings-draft.ts, lib/live-settings-*, proxy.ts.
import type { APIError, Page, Store } from "./model";

export type Market = {
  id: string;
  code: string;
  name: string;
  currency: string;
  version: number;
  active: boolean;
};
export type Policy = {
  market_id: string;
  country: string;
  method: string;
  currency: string;
  shipping_mode: "country_flat";
  tax_mode: "none" | "inclusive" | "exclusive";
  tax_basis: "goods" | "goods_and_shipping";
  version: number;
  shipping_minor: number;
  tax_rate_bps: number;
  quote_ttl_seconds: number;
  enabled: boolean;
  // storefront-v2 §C: merchandise subtotal at or above which the quote charges shipping 0; absent/null = no threshold.
  free_shipping_threshold_minor?: number | null;
};
export type Service = {
  market_id: string;
  country: string;
  code: string;
  version: number;
  policy_method: string;
  policy_version: number;
  currency: string;
  name_hans: string;
  name_hant: string;
  name_en: string;
  delivery_kind: "home" | "cvs_711" | "cvs_familymart" | "cvs_hilife" | "cvs_okmart";
  mode: "MANUAL" | "API";
  enabled: boolean;
  visible: boolean;
  sort_order: number;
};
export type SettingsInitial = { store: Store | null; error: APIError | null };
export type SettingsPages = {
  markets: Page<Market>;
  services: Page<Service>;
};
