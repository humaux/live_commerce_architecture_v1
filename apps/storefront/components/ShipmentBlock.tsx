"use client";

// Buyer-facing shipment block of the order page (BFF orders/{id} `shipment`; no route of its own), split out of OrderFlow.tsx
// (G-UI3 legacy ceiling). Rendered by <OrderDetails> in OrderFlow.tsx only when the order carries a shipment.
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { carrierNames, orderCopy } from "../lib/order-copy";
import type { Shipment } from "../lib/purchase";

// The seller's attestation of dispatch (manual-fulfilment-v1 §5.2): never "in transit"/"delivered".
// Link: plain external anchor, host shown so the buyer sees where it goes (ruling 14, Q6);
// rel="noopener noreferrer nofollow": ruling 25 (manual-fulfilment-v1 §3.2 governs; ruling 14 was incomplete).
export function ShipmentBlock({
  shipment,
  locale,
}: {
  shipment: Shipment;
  locale: Locale;
}) {
  const copy = orderCopy[locale];
  const [copied, setCopied] = useState<"" | "ok" | "failed">("");
  const link = shipment.tracking_url;
  // Validated https by validOrder (validTrackingURL) before it reaches an href.
  const host = link ? new URL(link).hostname : "";
  async function copyNumber() {
    try {
      await navigator.clipboard.writeText(shipment.tracking_number);
      setCopied("ok");
    } catch {
      setCopied("failed");
    }
  }
  return (
    <section data-testid="order-shipment" aria-labelledby="shipment-title">
      <h2 id="shipment-title" data-testid="shipment-title">
        {copy.shipped}
      </h2>
      <p data-testid="shipment-carrier">
        {copy.carrier}:{" "}
        {shipment.carrier_name ?? carrierNames[locale][shipment.carrier_code]}
      </p>
      <p>
        {copy.tracking}:{" "}
        <span data-testid="shipment-tracking" className="order-id">
          {shipment.tracking_number}
        </span>{" "}
        <button
          type="button"
          data-testid="copy-tracking"
          onClick={() => void copyNumber()}
        >
          {copy.copyTracking}
        </button>
      </p>
      <p role="status" data-testid="copy-status">
        {copied === "ok" ? copy.copied : copied === "failed" ? copy.copyFailed : ""}
      </p>
      {link && (
        <p>
          <a
            data-testid="shipment-link"
            href={link}
            target="_blank"
            rel="noopener noreferrer nofollow"
          >
            {copy.trackLink}
          </a>{" "}
          <span data-testid="shipment-host">({host})</span>
        </p>
      )}
      <p className="order-note">{copy.shipNote}</p>
    </section>
  );
}
