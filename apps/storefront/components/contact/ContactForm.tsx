"use client";
// Purpose: buyer contact form, confirmation and keyed receipt recovery.
// Depends on the closed buyer contact BFF contract and tab-local non-PII status marker.
// Invariants: the message is memory-only; no automatic retries; an uncertain attempt retains its original key and body.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { contactReceipt, validContactBody } from "../../lib/contact-contract";
import { contactCopy } from "../../lib/contact-copy";
import { withPreview } from "../../lib/design";
import styles from "./ContactForm.module.css";

type Phase = "boot" | "form" | "confirm" | "sending" | "unknown" | "sent" | "throttle" | "conflict" | "failed";
type Fields = { name: string; email: string; order_number: string; message: string };
const marker = "buyer-contact-status-v1";
const blank: Fields = { name: "", email: "", order_number: "", message: "" };

export default function ContactForm({ locale, preview }: { locale: Locale; preview: string | null }) {
  const c = contactCopy[locale];
  const [phase, setPhase] = useState<Phase>("boot");
  const [fields, setFields] = useState<Fields>(blank);
  const [invalid, setInvalid] = useState(false);
  const [storageError, setStorageError] = useState(false);
  const intent = useRef<{ key: string; body: string } | null>(null);
  const busy = useRef(false);
  useEffect(() => {
    try {
      const value = sessionStorage.getItem(marker);
      if (value === "sent") setPhase("sent");
      else if (value === "pending" || value === "conflict") setPhase("unknown");
      else setPhase("form");
    } catch { setStorageError(true); setPhase("form"); }
  }, []);
  const body = () => ({ name: fields.name, email: fields.email, ...(fields.order_number ? { order_number: fields.order_number } : {}), message: fields.message });
  function next() {
    if (!validContactBody(body())) { setInvalid(true); return; }
    setInvalid(false); setPhase("confirm");
  }
  async function send(recovery: boolean) {
    if (busy.current) return;
    if (!recovery) intent.current = { key: crypto.randomUUID(), body: JSON.stringify(body()) };
    const current = intent.current;
    if (!current) return;
    try { sessionStorage.setItem(marker, "pending"); }
    catch { setStorageError(true); return; }
    busy.current = true;
    setPhase("sending");
    try {
      const response = await fetch("/api/buyer/contact-messages", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": current.key }, body: current.body, cache: "no-store" });
      if (response.status === 201) {
        const receipt: unknown = await response.json();
        if (!contactReceipt(receipt)) { setPhase("unknown"); return; }
        sessionStorage.setItem(marker, "sent");
        intent.current = null;
        setPhase("sent");
      } else if (response.status === 429) { sessionStorage.removeItem(marker); setPhase("throttle"); }
      else if (response.status === 409) { sessionStorage.setItem(marker, "conflict"); setPhase("conflict"); }
      else if ([400, 413, 415, 422].includes(response.status)) { sessionStorage.removeItem(marker); setPhase("failed"); }
      else setPhase("unknown");
    } catch { setPhase("unknown"); }
    finally { busy.current = false; }
  }
  function field(key: keyof Fields, label: string, type = "text") {
    return <label className={styles.field}>{label}{key === "message" ? <textarea data-testid={`contact-${key}`} value={fields[key]} onChange={e => setFields({ ...fields, [key]: e.target.value })} rows={2} maxLength={5000} /> : <input data-testid={`contact-${key === "order_number" ? "order" : key}`} type={type} value={fields[key]} onChange={e => setFields({ ...fields, [key]: e.target.value })} maxLength={key === "name" ? 120 : key === "email" ? 254 : 64} />}</label>;
  }
  if (phase === "boot") return <main className={styles.task} aria-busy="true" />;
  const edit = () => setPhase("form");
  const home = withPreview(`/${locale}`, preview);
  return <main className={styles.task}>
    {phase === "form" && <div data-testid="contact-form"><h1>{c.title}</h1><div className={styles.panel}>
      {field("name", c.name)}{field("email", c.email, "email")}{field("order_number", c.order)}{field("message", c.message)}
      {invalid && <p role="alert">{c.invalid}</p>}{storageError && <p role="alert">{c.storage}</p>}
    </div><div className={styles.actions}><button type="button" data-testid="contact-next" onClick={next}>{c.next}</button></div></div>}
    {phase === "confirm" && <div data-testid="contact-confirm"><h1>{c.confirm}</h1><div className={styles.panel}>
      <p>{c.name}<br />{fields.name}</p><p>{c.email}<br />{fields.email}</p><p>{c.order}<br />{fields.order_number || "—"}</p><p>{c.message}<br />{fields.message}</p>
    </div><div className={styles.actions}><button type="button" data-testid="contact-send" onClick={() => void send(false)}>{c.send}</button><button type="button" onClick={edit}>{c.edit}</button></div>{storageError && <p role="alert">{c.storage}</p>}</div>}
    {phase === "sending" && <div aria-busy="true"><h1>{c.confirm}</h1><div className={styles.actions}><button type="button" disabled>{c.send}</button></div></div>}
    {phase === "unknown" && <div data-testid="contact-unknown" role="status"><h1>{c.unknown}</h1><div className={styles.panel}><p>{intent.current ? c.unknownBody : c.lostBody}</p></div>{intent.current && <div className={styles.actions}><button type="button" data-testid="contact-recover" onClick={() => void send(true)}>{c.recover}</button></div>}</div>}
    {phase === "sent" && <div data-testid="contact-sent"><h1>{c.sent}</h1><div className={styles.panel}>{fields.name && <p>{fields.name} · {fields.email}<br />{fields.order_number && <>{fields.order_number} · </>}{fields.message}</p>}<p className={styles.notice}>{c.sentBody}</p></div><div className={styles.actions}><a href={home} onClick={() => { try { sessionStorage.removeItem(marker); } catch { /* keep the confirmed marker if storage is unavailable */ } }}>{c.back}</a></div></div>}
    {(phase === "throttle" || phase === "failed" || phase === "conflict") && <div role="alert"><h1>{phase === "throttle" ? c.throttle : phase === "conflict" ? c.conflict : c.failed}</h1>{phase !== "conflict" && <div className={styles.actions}><button type="button" onClick={edit}>{c.edit}</button></div>}</div>}
  </main>;
}
