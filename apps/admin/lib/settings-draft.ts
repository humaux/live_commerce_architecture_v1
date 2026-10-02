// Draft model of the merchant settings wizard (components/SettingsWizard.tsx): the one persisted/restored form state, its empty value,
// the default method labels and the retry_later fallback error. Split out of the component (G-UI3 legacy ceiling); no behaviour of its own.
import type { APIError } from "./model";
import type { MethodCode, Service } from "./settings-model";

export type Branch = "payuni" | "manual";
export type Observation = { target: string; version: number; dirty: boolean };
export type MethodBinding = {
  connectionID: string;
  bindingVersion: number;
  environment: "SANDBOX" | "LIVE";
};
export type Draft = {
  branch: Branch;
  accountID: string;
  accountChoiceTouched: boolean;
  environment: "SANDBOX" | "LIVE";
  merchantID: string;
  rotate: boolean;
  marketID: string;
  marketCode: string;
  marketName: string;
  country: string;
  methodCode: MethodCode;
  nameHans: string;
  nameHant: string;
  nameEN: string;
  visible: boolean;
  sort: string;
  // Money fields (min, max, shipping, freeShipping) are MAJOR-unit text of the market currency ("60" = NT$60); the wire minor amount is
  // toMinor(text, currency) at submit time (D02). Draft version 1 held minor-unit text and is dropped on restore.
  min: string;
  max: string;
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
  methodObservation: Observation | null;
  methodBinding: MethodBinding | null;
  policyObservation: Observation | null;
  serviceObservation: Observation | null;
};
export const emptyDraft: Draft = {
  branch: "payuni",
  accountID: "",
  accountChoiceTouched: false,
  environment: "SANDBOX",
  merchantID: "",
  rotate: false,
  marketID: "",
  marketCode: "",
  marketName: "",
  country: "TW",
  methodCode: "payuni_credit",
  nameHans: "信用卡",
  nameHant: "信用卡",
  nameEN: "Credit card",
  visible: false,
  sort: "10",
  min: "1",
  max: "10000000000",
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
  methodObservation: null,
  methodBinding: null,
  policyObservation: null,
  serviceObservation: null,
};
export const names: Record<MethodCode, [string, string, string]> = {
  payuni_credit: ["信用卡", "信用卡", "Credit card"],
  payuni_installment: ["信用卡分期", "信用卡分期", "Card installments"],
  payuni_atm: ["ATM 转账", "ATM 轉帳", "ATM transfer"],
  payuni_cvs: ["超商代码缴费", "超商代碼繳費", "Convenience store code"],
  payuni_linepay: ["LINE Pay", "LINE Pay", "LINE Pay"],
};
export const unknown: APIError = {
  code: "retry_later",
  message: "",
  request_id: "",
  retryable: true,
  details: {},
};
