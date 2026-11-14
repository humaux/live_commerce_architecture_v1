"use client";

// Purpose: buyer store-contact form task surface with review, single submission, recovery and memory-only fields.
// Depends on: contact-client, contact-copy, contact-contract, design tokens and withPreview.
// Used by: the storefront locale contact page.
// Invariants: the message is memory-only; no automatic retries; an uncertain attempt retains its original key and body.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { contactReceipt, validContactBody } from "../../lib/contact-contract.ts";
import { postContactMessage } from "../../lib/contact-client.ts";
import { contactCopy } from "../../lib/contact-copy.ts";
import { withPreview } from "../../lib/design.ts";
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
  const intent = useRef<{ key: string; body: string; fields: Fields; recoverable: boolean } | null>(null);
  const busy = useRef(false);

  useEffect(() => {
    try {
      const shared = localStorage.getItem(marker);
      if (shared === "pending" || shared === "conflict") setPhase("unknown");
      else if (sessionStorage.getItem(marker) === "sent") setPhase("sent");
      else setPhase("form");
    } catch {
      setStorageError(true);
      setPhase("form");
    }
    const onStorage = (event: StorageEvent) => {
      if (event.key === marker && (event.newValue === "pending" || event.newValue === "conflict")) {
        setPhase((previous) => (previous === "sent" ? previous : "unknown"));
      }
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const body = () => ({
    name: fields.name,
    email: fields.email,
    ...(fields.order_number ? { order_number: fields.order_number } : {}),
    message: fields.message,
  });

  function next() {
    if (!validContactBody(body())) {
      setInvalid(true);
      return;
    }
    setInvalid(false);
    setPhase("confirm");
  }

  async function send(recovery: boolean) {
    if (busy.current) return;
    if (!navigator.locks) {
      setStorageError(true);
      return;
    }
    busy.current = true;
    try {
      await navigator.locks.request(marker, { ifAvailable: true }, async (lock) => {
        if (!lock) {
          setPhase("unknown");
          return;
        }
        let shared: string | null;
        try {
          shared = localStorage.getItem(marker);
        } catch {
          setStorageError(true);
          return;
        }
        if ((!recovery && shared) || (recovery && shared !== "pending")) {
          setPhase("unknown");
          return;
        }
        if (!recovery) {
          intent.current = {
            key: crypto.randomUUID(),
            body: JSON.stringify(body()),
            fields: { ...fields },
            recoverable: false,
          };
        }
        const current = intent.current;
        if (!current || (recovery && !current.recoverable)) {
          setPhase("unknown");
          return;
        }
        try {
          localStorage.setItem(marker, "pending");
        } catch {
          setStorageError(true);
          return;
        }
        // Only an admitted POST may become ambiguous; display memory alone grants no recovery authority.
        current.recoverable = true;
        setPhase("sending");
        try {
          const response = await postContactMessage(current.key, current.body);
          if (response.status === 201) {
            const receipt: unknown = await response.json();
            if (!contactReceipt(receipt)) {
              setPhase("unknown");
              return;
            }
            current.recoverable = false;
            sessionStorage.setItem(marker, "sent");
            localStorage.removeItem(marker);
            intent.current = null;
            setPhase("sent");
          } else if (response.status === 429) {
            current.recoverable = false;
            localStorage.removeItem(marker);
            setPhase("throttle");
          } else if (response.status === 409) {
            intent.current = null;
            localStorage.setItem(marker, "conflict");
            setPhase("conflict");
          } else if ([400, 413, 415, 422].includes(response.status)) {
            intent.current = null;
            localStorage.removeItem(marker);
            setPhase("failed");
          } else {
            setPhase("unknown");
          }
        } catch {
          setPhase("unknown");
        }
      });
    } catch {
      setStorageError(true);
    } finally {
      busy.current = false;
    }
  }

  function field(key: keyof Fields, label: string, type = "text") {
    return (
      <label className={styles.field}>
        {label}
        {key === "message" ? (
          <textarea
            data-testid={`contact-${key}`}
            value={fields[key]}
            onChange={(e) => setFields({ ...fields, [key]: e.target.value })}
            rows={2}
            maxLength={5000}
          />
        ) : (
          <input
            data-testid={`contact-${key === "order_number" ? "order" : key}`}
            type={type}
            value={fields[key]}
            onChange={(e) => setFields({ ...fields, [key]: e.target.value })}
            maxLength={key === "name" ? 120 : key === "email" ? 254 : 64}
          />
        )}
      </label>
    );
  }

  if (phase === "boot") return <main className={styles.task} aria-busy="true" />;
  const edit = () => {
    intent.current = null;
    setPhase("form");
  };
  const home = withPreview(`/${locale}`, preview);
  const recoverable = intent.current?.recoverable === true;
  const summary = (values: Fields = fields) => (
    <p data-testid="contact-summary">
      {values.name} · {values.email}
      <br />
      {values.order_number && <>{values.order_number} · </>}
      {values.message}
    </p>
  );

  return (
    <div className={styles.surface}>
      <main className={styles.task}>
        {phase === "form" && (
          <div data-testid="contact-form">
            <h1>{c.title}</h1>
            <div className={styles.panel}>
              {field("name", c.name)}
              {field("email", c.email, "email")}
              {field("order_number", c.order)}
              {field("message", c.message)}
              {invalid && <p role="alert">{c.invalid}</p>}
              {storageError && <p role="alert">{c.storage}</p>}
            </div>
            <div className={styles.actions}>
              <button type="button" data-testid="contact-next" onClick={next}>
                {c.next}
              </button>
            </div>
          </div>
        )}
        {phase === "confirm" && (
          <div data-testid="contact-confirm">
            <h1>{c.confirm}</h1>
            <div className={styles.panel}>
              <p>
                {c.name}
                <br />
                {fields.name}
              </p>
              <p>
                {c.email}
                <br />
                {fields.email}
              </p>
              <p>
                {c.order}
                <br />
                {fields.order_number || "—"}
              </p>
              <p>
                {c.message}
                <br />
                {fields.message}
              </p>
            </div>
            <div className={styles.actions}>
              <button type="button" data-testid="contact-send" onClick={() => void send(false)}>
                {c.send}
              </button>
              <button type="button" onClick={edit}>
                {c.edit}
              </button>
            </div>
            {storageError && <p role="alert">{c.storage}</p>}
          </div>
        )}
        {phase === "sending" && (
          <div aria-busy="true">
            <h1>{c.confirm}</h1>
            <div className={styles.actions}>
              <button type="button" disabled>
                {c.send}
              </button>
            </div>
          </div>
        )}
        {phase === "unknown" && (
          <div data-testid="contact-unknown" role="status">
            <h1>{c.unknown}</h1>
            <div className={styles.panel}>
              {recoverable && intent.current && summary(intent.current.fields)}
              <p className={styles.pendingNotice}>{recoverable ? c.unknownBody : c.lostBody}</p>
            </div>
            {recoverable && (
              <div className={styles.actions}>
                <button type="button" data-testid="contact-recover" onClick={() => void send(true)}>
                  {c.recover}
                </button>
              </div>
            )}
          </div>
        )}
        {phase === "sent" && (
          <div data-testid="contact-sent">
            <h1>{c.sent}</h1>
            <div className={styles.panel}>
              {fields.name && summary()}
              <p className={styles.notice}>{c.sentBody}</p>
            </div>
            <div className={styles.actions}>
              <a
                href={home}
                onClick={() => {
                  try {
                    sessionStorage.removeItem(marker);
                  } catch {
                    /* keep the confirmed marker if storage is unavailable */
                  }
                }}
              >
                {c.back}
              </a>
            </div>
          </div>
        )}
        {(phase === "throttle" || phase === "failed" || phase === "conflict") && (
          <div role="alert">
            <h1>{phase === "throttle" ? c.throttle : phase === "conflict" ? c.conflict : c.failed}</h1>
            {phase === "throttle" && (
              <div className={styles.panel}>
                {intent.current && summary(intent.current.fields)}
                <p className={styles.notice}>{c.throttle}</p>
              </div>
            )}
            {phase !== "conflict" && (
              <div className={styles.actions}>
                <button type="button" onClick={edit}>
                  {c.edit}
                </button>
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
