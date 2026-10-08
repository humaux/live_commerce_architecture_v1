// Purpose: Defines controlled shared manual-order field values and fresh blank drafts.
// Depends on: @live-commerce/i18n and merchant-tools-model frozen draft/payment types.
// Used by: ManualOrder, ManualOrderFormFields, ManualOrderItemPicker and CreateOrderDrawer.
// Invariants: I05, I08; displayed prices are catalog text only, never authoritative amounts.
import type { Locale } from "@live-commerce/i18n";
import type { ManualDraft, ManualPaymentMode } from "./merchant-tools-model";

/** Holds a selected SKU and its informational catalog label and unit price. */
export type ManualFormLine = { sku_id: string; quantity: number; label: string; code: string; price: string; note?: string };
/** Controlled values shared by the legacy page and the for-buyer drawer. */
export type ManualFormValues = {
  lines: ManualFormLine[]; name: string; phone: string; email: string; optionKey: string;
  mode: ManualPaymentMode | ""; home: ManualDraft["home"]; cvs: ManualDraft["cvs"]; buyerLocale: Locale;
};

/** Returns new nested field objects for every form; performs no reads or writes. */
export function emptyManualForm(locale: Locale = "zh-TW"): ManualFormValues {
  return { lines: [], name: "", phone: "", email: "", optionKey: "", mode: "",
    home: { region: "", city: "", postal_code: "", line1: "", line2: "" },
    cvs: { store_code: "", store_name: "", store_address: "" }, buyerLocale: locale };
}
