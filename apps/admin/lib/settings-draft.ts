// Draft model of the merchant settings wizard (components/SettingsWizard.tsx): the one persisted/restored form state, its empty value
// and the retry_later fallback error. Split out of the component (G-UI3 legacy ceiling); no behaviour of its own.
import type { APIError } from "./model";
import type { Service } from "./settings-model";

export type Observation = { target: string; version: number; dirty: boolean };
export type Draft = {
  marketID: string;
  marketCode: string;
  marketName: string;
  country: string;
  nameHans: string;
  nameHant: string;
  nameEN: string;
  sort: string;
  // Money fields (shipping, freeShipping) are MAJOR-unit text of the market currency ("60" = NT$60); the wire minor amount is
  // toMinor(text, currency) at submit time (D02). Draft version 1 held minor-unit text and is dropped on restore.
  serviceCode: string;
  serviceKind: Service["delivery_kind"];
  serviceMode: Service["mode"];
  shipping: string;
  // "" = no free-shipping threshold (storefront-v2 §C); otherwise major-unit text.
  freeShipping: string;
  taxMode: "none" | "inclusive" | "exclusive";
  taxBasis: "goods" | "goods_and_shipping";
  taxRate: string;
  ttl: string;
  policyEnabled: boolean;
  serviceEnabled: boolean;
  serviceVisible: boolean;
  reference: string;
  policyObservation: Observation | null;
  serviceObservation: Observation | null;
};
export const emptyDraft: Draft = {
  marketID: "",
  marketCode: "",
  marketName: "",
  country: "TW",
  nameHans: "",
  nameHant: "",
  nameEN: "",
  sort: "10",
  serviceCode: "",
  serviceKind: "home",
  serviceMode: "MANUAL",
  shipping: "0",
  freeShipping: "",
  taxMode: "none",
  taxBasis: "goods",
  taxRate: "0",
  ttl: "300",
  policyEnabled: false,
  serviceEnabled: false,
  serviceVisible: false,
  reference: "",
  policyObservation: null,
  serviceObservation: null,
};
export const unknown: APIError = {
  code: "retry_later",
  message: "",
  request_id: "",
  retryable: true,
  details: {},
};
