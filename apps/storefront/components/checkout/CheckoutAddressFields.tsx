// Purpose: render the home recipient fields before and after quotation.
// Depends on: order field constraints and localized order copy.
// Used by: CheckoutFlow cart entry and OrderFlow destination confirmation.
// Invariant: contact data stays in the caller's in-memory draft until explicit confirmation.
import type { Locale } from "@live-commerce/i18n";
import { orderCopy } from "../../lib/order-copy";
import { orderInputs as inputs, type OrderFields as Fields } from "../../lib/order-fields";

// Keep the street field just after contact in the compact entry's reading and tab order.
const entryInputs = [inputs[0], inputs[1], inputs[5], inputs[2], inputs[3], inputs[4], inputs[6]];

export default function CheckoutAddressFields({ locale, fields, onChange, entry = false }: {
  locale: Locale;
  entry?: boolean; fields: Fields; onChange: (fields: Fields) => void;
}) {
  const copy = orderCopy[locale];
  return <>{(entry ? entryInputs : inputs).map(([name, autoComplete, maxLength, required]) => (
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
