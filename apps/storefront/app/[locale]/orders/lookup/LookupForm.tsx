// Purpose: Buyer order lookup form and signed in-place errors.
// Depends on: lookup-copy, lookup-contract and state/StorefrontState.
// Used by: localized order lookup route.
// Frames: 26:708 / 26:995 default, 1253:17225 / 1253:17256 not found.
// Invariants: no persisted lookup credentials; refusals never reveal which field matched.
"use client";

// Guest order lookup form: order number + (email or phone) -> BFF POST /api/buyer/orders/lookup -> Go POST /v1/buyer/orders/lookup
// (internal/buyerhttp/lookup.go, contracts/storefront-v2.md §E5). On 200 the BFF has already set the buyer cookie, so the page navigates to
// /{locale}/orders/{order_id}. Every refusal other than a format error or a throttle is the same "no match" text: the page never says which
// half was wrong. Nothing is stored in the browser and the values are never logged.
import { useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { StorefrontReadFailure } from "../../../../components/state/StorefrontState";
import { lookupCopy } from "../../../../lib/lookup-copy";
import { validContact, validOrderRef, validLookupResult } from "../../../../lib/lookup-contract";

type Problem = "" | "noMatch" | "invalid" | "tooMany" | "unavailable";

export default function LookupForm({ locale }: { locale: Locale }) {
  const copy = lookupCopy[locale];
  const [ref, setRef] = useState("");
  const [contact, setContact] = useState("");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<Problem>("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    if (!validOrderRef(ref) || !validContact(contact)) return setProblem("invalid");
    setBusy(true);
    setProblem("");
    try {
      const response = await fetch("/api/buyer/orders/lookup", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ order_ref: ref.trim(), contact: contact.trim() }),
      });
      if (response.status === 200) {
        const value: unknown = await response.json().catch(() => null);
        if (validLookupResult(value)) {
          window.location.assign(`/${locale}/orders/${value.order_id}`);
          return;
        }
        return setProblem("unavailable");
      }
      setProblem(response.status === 404 ? "noMatch" : response.status === 429 ? "tooMany" : response.status === 422 ? "invalid" : "unavailable");
    } catch {
      setProblem("unavailable");
    } finally {
      setBusy(false);
    }
  }

  // 1253:17225: a not-found answer keeps the typed values and switches to the "values kept" state; every other problem keeps the default frame.
  const miss = problem === "noMatch";
  return (
    <main className="purchase-main claim-main order-section" data-testid="order-lookup" data-state={miss ? "not-found" : "default"}>
      <h1>{copy.title}</h1>
      {!miss && <h2>{copy.heading}</h2>}
      <p>{miss ? copy.intro : copy.introDefault}</p>
      <form className="lookup-form" onSubmit={(event) => void submit(event)} noValidate>
        <label>
          {copy.orderRef}
          <input
            name="order_ref"
            data-testid="lookup-ref"
            value={ref}
            maxLength={64}
            autoComplete="off"
            autoCapitalize="characters"
            spellCheck={false}
            inputMode="text"
            disabled={busy}
            aria-describedby={miss ? "lookup-ref-hint" : undefined}
            onChange={(event) => setRef(event.target.value)}
          />
        </label>
        {miss && <p id="lookup-ref-hint" className="order-note">{copy.orderRefHint}</p>}
        <label>
          {copy.contact}
          <input
            name="contact"
            data-testid="lookup-contact"
            value={contact}
            maxLength={254}
            autoComplete="email"
            spellCheck={false}
            disabled={busy}
            aria-describedby={miss ? "lookup-contact-hint" : undefined}
            onChange={(event) => setContact(event.target.value)}
          />
        </label>
        {miss && <p id="lookup-contact-hint" className="order-note">{copy.contactHint}</p>}
        {problem && (
          <div data-testid="lookup-problem" data-problem={problem}>
            <StorefrontReadFailure message={copy[problem]} />
          </div>
        )}
        <button type="submit" data-testid="lookup-submit" disabled={busy}>
          {busy ? copy.submitting : miss ? copy.submitMiss : copy.submit}
        </button>
      </form>
      <p className="order-note">{miss ? copy.kept : copy.footNote}</p>
    </main>
  );
}
