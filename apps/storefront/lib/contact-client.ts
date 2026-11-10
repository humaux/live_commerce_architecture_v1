// Purpose: send a guest store-contact attempt through its fixed same-origin BFF endpoint.
// Depends on: the contact key validator and browser fetch; no buyer session or retry journal.
// Used by: ContactForm for the original attempt and explicit same-key recovery.
// Invariants: exactly one attempt with the caller's unchanged serialized body and idempotency key.
import { validContactKey } from "./contact-contract.ts";

export async function postContactMessage(key: string, body: string): Promise<Response> {
  if (!validContactKey(key)) throw new TypeError("Invalid contact idempotency key");
  return fetch("/api/buyer/contact-messages", {
    method: "POST",
    headers: { "Content-Type": "application/json", "Idempotency-Key": key },
    body,
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
  });
}
