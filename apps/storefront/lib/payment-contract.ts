// Buyer-safe payment wire projections. Admission remains in the Go service.
export type OrderPayment = {
  order_id: string;
  currency: string;
  total_minor: number;
  commercial_state: "DRAFT" | "AWAITING_PAYMENT" | "CONFIRMED" | "CANCELLED";
  test_mode: boolean;
  payment_state:
    "NOT_STARTED" | "PENDING" | "AUTHORIZED" | "CAPTURED" | "REVIEW_REQUIRED";
  handoff_state: "NONE" | "PREPARED" | "ISSUED" | "EXPIRED" | "UNAVAILABLE";
  handoff_expires_at: string | null;
  methods: {
    code: "payuni_credit";
    version: number;
    name_hans: string;
    name_hant: string;
    name_en: string;
  }[];
};

export type PaymentPrepared = {
  order_id: string;
  state: "PAYMENT_PENDING";
  currency: "TWD";
  amount_minor: number;
};

export type HostedForm = {
  action:
    | "https://sandbox-api.payuni.com.tw/api/upp"
    | "https://api.payuni.com.tw/api/upp";
  fields: {
    Version: "2.0";
    MerID: string;
    EncryptInfo: string;
    HashInfo: string;
  };
};

export type HostedHandoff = {
  order_id: string;
  disposition: "ISSUED" | "ALREADY_ISSUED";
  expires_at: string;
  form?: HostedForm;
};

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function exact(
  value: unknown,
  keys: readonly string[],
): value is Record<string, unknown> {
  return (
    value !== null &&
    typeof value === "object" &&
    !Array.isArray(value) &&
    Object.getPrototypeOf(value) === Object.prototype &&
    Reflect.ownKeys(value).length === keys.length &&
    keys.every((key) => Object.hasOwn(value, key))
  );
}

function utc(value: unknown): value is string {
  if (
    typeof value !== "string" ||
    !/^\d{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?Z$/.test(
      value,
    )
  )
    return false;
  const time = Date.parse(value);
  return (
    Number.isFinite(time) &&
    new Date(time).toISOString().slice(0, 19) === value.slice(0, 19)
  );
}

function name(value: unknown): value is string {
  return (
    typeof value === "string" &&
    [...value].length >= 1 &&
    [...value].length <= 120 &&
    value.trim().length > 0 &&
    !/\p{Cc}/u.test(value)
  );
}

export function validOrderPayment(
  value: unknown,
  orderID: string,
): value is OrderPayment {
  if (!UUID.test(orderID)) return false;
  if (
    !exact(value, [
      "order_id",
      "currency",
      "total_minor",
      "commercial_state",
      "test_mode",
      "payment_state",
      "handoff_state",
      "handoff_expires_at",
      "methods",
    ])
  )
    return false;
  if (
    value.order_id !== orderID ||
    typeof value.currency !== "string" ||
    !/^[A-Z]{3}$/.test(value.currency) ||
    !Number.isSafeInteger(value.total_minor) ||
    (value.total_minor as number) < 0 ||
    (value.total_minor as number) > 1e12 ||
    typeof value.test_mode !== "boolean" ||
    typeof value.commercial_state !== "string" ||
    !["DRAFT", "AWAITING_PAYMENT", "CONFIRMED", "CANCELLED"].includes(
      value.commercial_state,
    ) ||
    typeof value.payment_state !== "string" ||
    ![
      "NOT_STARTED",
      "PENDING",
      "AUTHORIZED",
      "CAPTURED",
      "REVIEW_REQUIRED",
    ].includes(value.payment_state) ||
    typeof value.handoff_state !== "string" ||
    !["NONE", "PREPARED", "ISSUED", "EXPIRED", "UNAVAILABLE"].includes(
      value.handoff_state,
    )
  )
    return false;
  if (
    value.handoff_state === "NONE"
      ? value.handoff_expires_at !== null
      : value.handoff_state !== "UNAVAILABLE"
        ? !utc(value.handoff_expires_at)
        : value.handoff_expires_at !== null && !utc(value.handoff_expires_at)
  )
    return false;
  if (!Array.isArray(value.methods) || value.methods.length > 1) return false;
  if (
    value.methods.length &&
    (value.payment_state !== "NOT_STARTED" ||
      value.commercial_state !== "DRAFT" ||
      value.handoff_state !== "NONE")
  )
    return false;
  if (value.methods.length === 0) return true;
  const method: unknown = value.methods[0];
  return (
    exact(method, ["code", "version", "name_hans", "name_hant", "name_en"]) &&
    method.code === "payuni_credit" &&
    Number.isSafeInteger(method.version) &&
    (method.version as number) > 0 &&
    name(method.name_hans) &&
    name(method.name_hant) &&
    name(method.name_en)
  );
}

export function validPaymentPrepared(
  value: unknown,
  orderID: string,
): value is PaymentPrepared {
  return (
    UUID.test(orderID) &&
    exact(value, ["order_id", "state", "currency", "amount_minor"]) &&
    value.order_id === orderID &&
    value.state === "PAYMENT_PENDING" &&
    value.currency === "TWD" &&
    Number.isSafeInteger(value.amount_minor) &&
    (value.amount_minor as number) >= 100 &&
    (value.amount_minor as number) <= 19999900 &&
    (value.amount_minor as number) % 100 === 0
  );
}

export function validHostedHandoff(
  value: unknown,
  orderID: string,
): value is HostedHandoff {
  if (
    !UUID.test(orderID) ||
    !exact(
      value,
      value !== null &&
        typeof value === "object" &&
        (value as Record<string, unknown>).disposition === "ISSUED"
        ? ["order_id", "disposition", "expires_at", "form"]
        : ["order_id", "disposition", "expires_at"],
    ) ||
    value.order_id !== orderID ||
    !utc(value.expires_at)
  )
    return false;
  if (value.disposition === "ALREADY_ISSUED") return true;
  if (
    value.disposition !== "ISSUED" ||
    !exact(value.form, ["action", "fields"])
  )
    return false;
  const form = value.form;
  if (
    form.action !== "https://sandbox-api.payuni.com.tw/api/upp" &&
    form.action !== "https://api.payuni.com.tw/api/upp"
  )
    return false;
  if (!exact(form.fields, ["Version", "MerID", "EncryptInfo", "HashInfo"]))
    return false;
  const fields = form.fields;
  return (
    fields.Version === "2.0" &&
    typeof fields.MerID === "string" &&
    /^[A-Za-z0-9_-]{1,64}$/.test(fields.MerID) &&
    typeof fields.EncryptInfo === "string" &&
    fields.EncryptInfo.length >= 16 &&
    fields.EncryptInfo.length <= 24576 &&
    fields.EncryptInfo.length % 2 === 0 &&
    /^[0-9a-f]+$/.test(fields.EncryptInfo) &&
    typeof fields.HashInfo === "string" &&
    /^[0-9A-F]{64}$/.test(fields.HashInfo)
  );
}
