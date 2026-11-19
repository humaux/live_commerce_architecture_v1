// Home recipient fields shared by cart entry and quote-bound destination confirmation.
// Depends on order field constraints and locale copy; the caller owns the in-memory draft.
import type { Locale } from "@live-commerce/i18n";
import { orderCopy } from "../../lib/order-copy";
import { orderInputs as inputs, type OrderFields as Fields } from "../../lib/order-fields";

export default function CheckoutAddressFields({ locale, fields, onChange }: {
  locale: Locale; fields: Fields; onChange: (fields: Fields) => void;
}) {
  const copy = orderCopy[locale];
  return <>{inputs.map(([name, autoComplete, maxLength, required]) => (
    <label
      key={name}
      className={
        name === "line1" || name === "line2" ? "wide" : undefined
      }
    >
      <span>{copy[name]}</span>
      <input
        name={name}
        type={name === "phone" ? "tel" : "text"}
        autoComplete={autoComplete}
        maxLength={maxLength}
        required={required}
        value={fields[name]}
        onChange={(event) => onChange({ ...fields, [name]: event.target.value })}
      />
    </label>
  ))}</>;
}
