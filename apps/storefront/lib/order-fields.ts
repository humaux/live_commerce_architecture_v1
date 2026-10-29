// Purpose: define the checkout's address fields and native input constraints.
// Depends on: purchase.ts wire types only; no transport or persistent state.
// Used by: OrderFlow for form initialization, recovery and explicit destination writes.
// Invariants: empty fields contain no contact data; projection preserves server destination facts.
import type { Destination, DestinationWrite, HomeAddress } from "./purchase";

export type OrderFields = HomeAddress & { recipient_name: string; phone: string };
export const emptyOrderFields: OrderFields = {
  recipient_name: "",
  phone: "",
  region: "",
  city: "",
  postal_code: "",
  line1: "",
  line2: "",
};
export const orderFieldsOf = (d: DestinationWrite | Destination): OrderFields => ({
  recipient_name: d.recipient_name,
  phone: d.phone,
  ...d.home_address,
});
export const orderInputs = [
  ["recipient_name", "name", 120, true],
  ["phone", "tel", 32, true],
  ["region", "address-level1", 100, false],
  ["city", "address-level2", 100, true],
  ["postal_code", "postal-code", 20, false],
  ["line1", "address-line1", 200, true],
  ["line2", "address-line2", 200, false],
] as const;
