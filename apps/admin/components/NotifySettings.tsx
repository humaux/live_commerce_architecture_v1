"use client";

// Settings -> new-order email card (mounted by SettingsWizard.tsx under <BankTransferSettings>; contracts/storefront-v2.md §E6). BFF
// `GET|PUT /api/stores/{store}/notification-settings` (lib/logistics-client.ts) -> Go internal/httpapi/notify.go (integration:read /
// integration:manage). A GET 403 hides the card; a PUT 403 shows a no-permission notice. Nothing is optimistic: every save re-GETs the card.
// The PUT is an idempotent set, so one Idempotency-Key per distinct body is reused only for a retry after an unknown outcome.
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import { putNotifySettings, readNotifySettings } from "@/lib/logistics-client";
import { notifySettingsBody } from "@/lib/notify-model";
import { notifyCopy } from "@/lib/notify-copy";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";

export function NotifySettings({ store, locale }: { store: string; locale: Locale }) {
  const nc = notifyCopy[locale];
  const [boundary, setBoundary] = useState("");
  const [load, setLoad] = useState<Load>("loading");
  const [tick, setTick] = useState(0);
  const [enabled, setEnabled] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const pending = useRef<{ key: string; body: string } | null>(null);

  useEffect(() => {
    let live = true;
    sessionBoundary().then(
      (value) => live && setBoundary(value),
      () => live && setBoundary(""),
    );
    return () => {
      live = false;
    };
  }, [store]);

  useEffect(() => {
    const active = new AbortController();
    readNotifySettings(store, active.signal).then(
      (value) => {
        setEnabled(value.merchant_new_order_email);
        setLoad("ready");
      },
      (error) => {
        if (active.signal.aborted) return;
        setLoad(error instanceof OrderReadError && error.code === "forbidden" ? "hidden" : "error");
      },
    );
    return () => active.abort();
  }, [store, tick]);

  if (load === "hidden") return null;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    const body = notifySettingsBody(enabled);
    if (pending.current?.body !== body) pending.current = { key: `notify-set-${crypto.randomUUID()}`, body };
    setBusy(true);
    setMessage("");
    const result = await putNotifySettings(store, pending.current.key, body, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setMessage(nc.saved);
      setTick((value) => value + 1);
    } else if (result.uncertain) setMessage(nc.uncertain);
    else {
      pending.current = null;
      setMessage(result.code === "forbidden" ? nc.noPermission : nc.failed);
    }
  }

  return (
    <section className="settings-fields" data-testid="notify-settings-card" aria-labelledby="notify-settings-title">
      <h2 id="notify-settings-title" className="settings-section-title settings-subtitle">{nc.title}</h2>
      {load === "loading" && <p role="status">{nc.loading}</p>}
      {load === "error" && (
        <div role="status">
          <p>{nc.loadFailed}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{nc.retry}</button>
        </div>
      )}
      {load === "ready" && (
        <form onSubmit={(event) => void submit(event)} className="settings-fields">
          <label className="settings-check">
            <input type="checkbox" data-testid="notify-new-order" checked={enabled} disabled={busy} onChange={(event) => setEnabled(event.target.checked)} />
            {nc.label}
          </label>
          <p className="settings-note">{nc.note}</p>
          <div className="settings-actions">
            <button className="primary" type="submit" disabled={busy}>{busy ? nc.saving : nc.save}</button>
          </div>
          {message && <p role="status" data-testid="notify-message">{message}</p>}
        </form>
      )}
    </section>
  );
}
