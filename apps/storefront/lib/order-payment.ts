import { buyerRequest, BuyerClientError } from "./buyer-client.ts";
import {
  validHostedHandoff,
  validOrderPayment,
  validPaymentPrepared,
  type HostedForm,
  type OrderPayment,
} from "./payment-contract.ts";
import {
  assertPurchaseContext,
  pendingPurchase,
  readPurchase,
  validOrder,
  type Order,
} from "./purchase.ts";

const LOCK = "commerce-purchase-write-v1";
const CONTEXT = /^[A-Za-z0-9_-]{43}$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const FORM_CHECK_ID = "00000000-0000-0000-0000-000000000000";
const FORM_CHECK_TIME = "2026-01-01T00:00:00Z";
const FORM_POLICY =
  "default-src 'none'; base-uri 'none'; form-action https://sandbox-api.payuni.com.tw/api/upp https://api.payuni.com.tw/api/upp";

type Locale = "zh-CN" | "zh-TW" | "en";
type Method = OrderPayment["methods"][number];
export type PaymentMarker = {
  v: 1;
  context: string;
  order_id: string;
  key: string;
  body: {
    method_code: "payuni_credit";
    method_version: number;
    locale: Locale;
  };
  stage: "prepare" | "handoff_started";
};
export type PaymentDestination = {
  ready(): boolean;
  submit(form: HostedForm): void;
  close(): void;
};

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

function markerKey(context: string, orderID: string): string {
  if (!CONTEXT.test(context)) throw new BuyerClientError("context_changed");
  if (!UUID.test(orderID)) throw new BuyerClientError("request_failed");
  return `commerce-order-payment-v1:${context}:${orderID}`;
}

function parseMarker(
  raw: string,
  context: string,
  orderID: string,
): PaymentMarker {
  try {
    if (raw.length > 1024) throw new Error("marker length");
    const value: unknown = JSON.parse(raw);
    if (
      !exact(value, ["v", "context", "order_id", "key", "body", "stage"]) ||
      value.v !== 1 ||
      value.context !== context ||
      value.order_id !== orderID ||
      typeof value.key !== "string" ||
      !UUID.test(value.key) ||
      (value.stage !== "prepare" && value.stage !== "handoff_started") ||
      !exact(value.body, ["method_code", "method_version", "locale"]) ||
      value.body.method_code !== "payuni_credit" ||
      !Number.isSafeInteger(value.body.method_version) ||
      (value.body.method_version as number) < 1 ||
      !["zh-CN", "zh-TW", "en"].includes(value.body.locale as string)
    )
      throw new Error("marker shape");
    return value as PaymentMarker;
  } catch {
    throw new BuyerClientError("uncertain");
  }
}

export function pendingOrderPayment(
  context: string,
  orderID: string,
): PaymentMarker | null {
  const key = markerKey(context, orderID);
  let raw: string | null;
  try {
    raw = localStorage.getItem(key);
  } catch {
    throw new BuyerClientError("unavailable");
  }
  return raw === null ? null : parseMarker(raw, context, orderID);
}

function persistMarker(
  marker: PaymentMarker,
  previous: PaymentMarker | null,
): void {
  const key = markerKey(marker.context, marker.order_id);
  const raw = JSON.stringify(marker);
  parseMarker(raw, marker.context, marker.order_id); // No form, token, amount or PII can enter storage.
  const current = pendingOrderPayment(marker.context, marker.order_id);
  if (JSON.stringify(current) !== JSON.stringify(previous))
    throw new BuyerClientError("uncertain");
  try {
    localStorage.setItem(key, raw);
    if (localStorage.getItem(key) !== raw) throw new Error("marker readback");
  } catch {
    throw new BuyerClientError("unavailable");
  }
}

function matchesOrder(view: OrderPayment, order: Order): boolean {
  return (
    view.order_id === order.order_id &&
    view.currency === order.snapshot.quote.currency &&
    view.total_minor === order.snapshot.quote.amount.total_minor
  );
}

export function readOrderPayment(
  context: string,
  orderID: string,
): Promise<OrderPayment> {
  markerKey(context, orderID);
  return readPurchase(
    `orders/${orderID}/payment`,
    context,
    (value): value is OrderPayment => validOrderPayment(value, orderID),
  );
}

// The child inherits the storefront CSP; its own meta policy narrows form-action.
// Keep the reference to the exact initial document so navigation cannot be reused.
export function openPaymentDestination(locale: Locale): PaymentDestination {
  if (
    !["zh-CN", "zh-TW", "en"].includes(locale) ||
    typeof window === "undefined"
  )
    throw new BuyerClientError("request_failed");
  const child = window.open("about:blank", "_blank");
  if (!child) throw new BuyerClientError("unavailable");
  let document: Document | undefined;
  let policy: HTMLMetaElement;
  let referrer: HTMLMetaElement;
  try {
    if (child.closed || child.location.href !== "about:blank")
      throw new Error("not blank");
    document = child.document;
    if (document.URL !== "about:blank" || !document.head || !document.body)
      throw new Error("not owned");
    child.opener = null;
    if (child.opener !== null) throw new Error("opener retained");
    referrer = document.createElement("meta");
    referrer.name = "referrer";
    referrer.content = "no-referrer";
    document.head.appendChild(referrer);
    policy = document.createElement("meta");
    policy.httpEquiv = "Content-Security-Policy";
    policy.content = FORM_POLICY;
    document.head.appendChild(policy);
    const line = document.createElement("p");
    line.textContent =
      locale === "en"
        ? "Please wait for payment and keep the original store tab open."
        : locale === "zh-TW"
          ? "請稍候，並保留原本的商店分頁。"
          : "请稍候，并保留原来的商店标签页。";
    document.body.appendChild(line);
  } catch {
    try {
      if (
        document &&
        !child.closed &&
        child.location.href === "about:blank" &&
        child.document === document &&
        document.URL === "about:blank"
      )
        child.close();
    } catch {
      /* The child may already have navigated; never close a provider page. */
    }
    throw new BuyerClientError("unavailable");
  }
  let used = false;
  const owned = () => {
    try {
      return (
        !child.closed &&
        child.opener === null &&
        child.location.href === "about:blank" &&
        child.document === document &&
        document.URL === "about:blank" &&
        policy.isConnected &&
        policy.content === FORM_POLICY &&
        referrer.isConnected &&
        referrer.content === "no-referrer"
      );
    } catch {
      return false;
    }
  };
  return {
    ready: () => !used && owned(),
    submit(form: HostedForm) {
      if (
        used ||
        !owned() ||
        !validHostedHandoff(
          {
            order_id: FORM_CHECK_ID,
            disposition: "ISSUED",
            expires_at: FORM_CHECK_TIME,
            form,
          },
          FORM_CHECK_ID,
        )
      )
        throw new BuyerClientError("invalid_response");
      const node = document.createElement("form");
      node.method = "POST";
      node.action = form.action;
      node.target = "_self";
      node.setAttribute("rel", "noreferrer");
      for (const [name, value] of Object.entries(form.fields)) {
        const input = document.createElement("input");
        input.type = "hidden";
        input.name = name;
        input.value = value;
        node.appendChild(input);
      }
      if (!owned()) throw new BuyerClientError("uncertain");
      document.body.appendChild(node);
      used = true; // A thrown or lost form submission must not trigger a second POST.
      try {
        node.submit();
      } finally {
        node.remove();
      }
    },
    close() {
      if (owned()) child.close();
    },
  };
}

export async function payOrder({
  context,
  order,
  locale,
  method,
  destination,
  isCurrent,
}: {
  context: string;
  order: Order;
  locale: Locale;
  method?: Method;
  destination: PaymentDestination;
  isCurrent: () => boolean;
}): Promise<void> {
  try {
    if (
      !validOrder(order) ||
      !UUID.test(order.order_id) ||
      !["zh-CN", "zh-TW", "en"].includes(locale) ||
      typeof navigator === "undefined" ||
      !navigator.locks?.request
    )
      throw new BuyerClientError("request_failed");
    await navigator.locks.request(LOCK, { mode: "exclusive" }, async () => {
      const fence = async () => {
        if (!isCurrent() || !destination.ready())
          throw new BuyerClientError("uncertain");
        await assertPurchaseContext(context);
        if (pendingPurchase(context) || !isCurrent() || !destination.ready())
          throw new BuyerClientError("uncertain");
      };
      await fence();
      const view = await readOrderPayment(context, order.order_id);
      await fence();
      if (!matchesOrder(view, order))
        throw new BuyerClientError("invalid_response");
      let marker = pendingOrderPayment(context, order.order_id);
      if (marker?.stage === "handoff_started")
        throw new BuyerClientError("uncertain");
      if (marker) {
        const fresh =
          view.payment_state === "NOT_STARTED" &&
          view.commercial_state === "DRAFT" &&
          view.handoff_state === "NONE";
        const prepared =
          view.payment_state === "PENDING" &&
          view.commercial_state === "AWAITING_PAYMENT" &&
          view.handoff_state === "PREPARED";
        if (!fresh && !prepared) throw new BuyerClientError("uncertain");
      } else {
        if (
          view.payment_state !== "NOT_STARTED" ||
          view.commercial_state !== "DRAFT" ||
          view.handoff_state !== "NONE" ||
          !method ||
          !view.methods.some(
            (item) =>
              item.code === method.code && item.version === method.version,
          )
        )
          throw new BuyerClientError("request_failed");
        marker = {
          v: 1,
          context,
          order_id: order.order_id,
          key: crypto.randomUUID(),
          body: {
            method_code: "payuni_credit",
            method_version: method.version,
            locale,
          },
          stage: "prepare",
        };
        await fence();
        persistMarker(marker, null);
      }
      await fence();
      const response = await buyerRequest(
        "POST",
        `orders/${order.order_id}/payment/prepare`,
        context,
        marker.body,
        marker.key,
      );
      await fence();
      if (!response.ok)
        throw new BuyerClientError("request_failed", response.status);
      let prepared: unknown;
      try {
        prepared = await response.json();
      } catch {
        throw new BuyerClientError("uncertain");
      }
      await fence();
      if (
        !validPaymentPrepared(prepared, order.order_id) ||
        prepared.currency !== order.snapshot.quote.currency ||
        prepared.amount_minor !== order.snapshot.quote.amount.total_minor
      )
        throw new BuyerClientError("invalid_response");
      const current = await readOrderPayment(context, order.order_id);
      await fence();
      if (
        !matchesOrder(current, order) ||
        current.payment_state !== "PENDING" ||
        current.commercial_state !== "AWAITING_PAYMENT" ||
        current.handoff_state !== "PREPARED"
      )
        throw new BuyerClientError("uncertain");
      const started: PaymentMarker = { ...marker, stage: "handoff_started" };
      await fence();
      persistMarker(started, marker);
      await fence();
      if (
        JSON.stringify(pendingOrderPayment(context, order.order_id)) !==
        JSON.stringify(started)
      )
        throw new BuyerClientError("uncertain");
      // TakeHosted is one-shot. No catch path may clear this marker or replay Take.
      const handoff = await buyerRequest(
        "POST",
        `orders/${order.order_id}/payment/handoff`,
        context,
      );
      await fence();
      if (!handoff.ok) throw new BuyerClientError("uncertain", handoff.status);
      let result: unknown;
      try {
        result = await handoff.json();
      } catch {
        throw new BuyerClientError("uncertain");
      }
      await fence();
      if (
        !validHostedHandoff(result, order.order_id) ||
        result.disposition !== "ISSUED" ||
        !result.form
      )
        throw new BuyerClientError("uncertain");
      await fence();
      if (
        JSON.stringify(pendingOrderPayment(context, order.order_id)) !==
        JSON.stringify(started)
      )
        throw new BuyerClientError("uncertain");
      destination.submit(result.form);
    });
  } catch (error) {
    try {
      destination.close();
    } catch {
      /* Preserve the payment failure. */
    }
    throw error;
  }
}
