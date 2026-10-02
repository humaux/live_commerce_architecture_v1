"use client";

// Settings -> cash on delivery card (mounted by SettingsWizard.tsx next to <BankTransferSettings>; home-cod R5, migration 0107): the
// switch, the per-order amount cap (whole TWD), the optional surcharge (whole TWD) and the carrier label (黑貓 / 新竹). BFF routes
// (lib/logistics-client.ts) -> Go internal/httpapi/cod.go:
//   GET|PUT /api/stores/{store}/cash-on-delivery-settings   (integration:read / integration:manage, version CAS, Idempotency-Key)
// A GET 403 hides the card (no integration:read); a PUT 403 shows a no-permission notice. Nothing is optimistic: every save re-GETs the
// card. One Idempotency-Key per distinct body, reused only for a byte-identical retry after an unknown outcome. A change never touches
// orders already placed (each keeps its own surcharge snapshot); the surcharge is shown only to the merchant and to the buyer of that order.
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import { putCodSettings, readCodSettings } from "@/lib/logistics-client";
import { codSettingsBody, type CodCarrier, type CodSettings } from "@/lib/cod-model";
import { codCopy, codError } from "@/lib/cod-copy";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";

export function CodSettings({ store, locale }: { store: string; locale: Locale }) {
  const cc = codCopy[locale];
  const [boundary, setBoundary] = useState("");
  const [load, setLoad] = useState<Load>("loading");
  const [saved, setSaved] = useState<CodSettings | null>(null);
  const [tick, setTick] = useState(0);
  const [enabled, setEnabled] = useState(false);
  const [maxTwd, setMaxTwd] = useState("20000");
  const [surchargeTwd, setSurchargeTwd] = useState("0");
  const [carrier, setCarrier] = useState<CodCarrier>("black_cat");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [uncertain, setUncertain] = useState(false);
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
    readCodSettings(store, active.signal).then(
      (value) => {
        setSaved(value);
        setEnabled(value.enabled);
        setMaxTwd(String(value.max_twd));
        setSurchargeTwd(String(value.surcharge_twd));
        setCarrier(value.carrier);
        setLoad("ready");
      },
      (error) => {
        if (active.signal.aborted) return;
        setSaved(null);
        setLoad(error instanceof OrderReadError && error.code === "forbidden" ? "hidden" : "error");
      },
    );
    return () => active.abort();
  }, [store, tick]);

  if (load === "hidden") return null;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy || !saved) return;
    const body = codSettingsBody({
      expectedVersion: saved.version, enabled, maxTwd, surchargeTwd, carrier,
    });
    if (!body) {
      setProblem(cc.invalid);
      return;
    }
    const text = JSON.stringify(body);
    if (pending.current?.body !== text) pending.current = { key: `cod-set-${crypto.randomUUID()}`, body: text };
    setBusy(true);
    setProblem("");
    const result = await putCodSettings(store, pending.current.key, text, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setUncertain(false);
      setNotice(cc.saved);
      setTick((value) => value + 1);
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(cc.uncertain);
      return;
    }
    pending.current = null;
    setUncertain(false);
    setProblem(result.code === "forbidden" ? cc.noPermission : codError(cc, result.code));
    if (result.code === "version_changed") setTick((value) => value + 1);
  }

  return (
    <section className="settings-fields" data-testid="cod-settings-card" aria-labelledby="cod-settings-title">
      <h2 id="cod-settings-title" className="settings-section-title settings-subtitle">{cc.setTitle}</h2>
      <p className="settings-note">{cc.setIntro}</p>
      {load === "loading" && <p role="status">{cc.loading}</p>}
      {load === "error" && (
        <div role="status">
          <p>{cc.loadFailed}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{cc.retry}</button>
        </div>
      )}
      {load === "ready" && saved && (
        <form onSubmit={(event) => void submit(event)} className="settings-fields">
          <label className="settings-check">
            <input type="checkbox" data-testid="cod-enabled" checked={enabled} disabled={busy} onChange={(event) => setEnabled(event.target.checked)} />
            {cc.setEnabled}
          </label>
          <div className="settings-field-grid">
            <label>
              {cc.setMax}
              <input data-testid="cod-max" inputMode="numeric" value={maxTwd} maxLength={5} disabled={busy} onChange={(event) => setMaxTwd(event.target.value)} />
              <small>{cc.setMaxHint}</small>
            </label>
            <label>
              {cc.setSurcharge}
              <input data-testid="cod-surcharge" inputMode="numeric" value={surchargeTwd} maxLength={4} disabled={busy} onChange={(event) => setSurchargeTwd(event.target.value)} />
              <small>{cc.setSurchargeHint}</small>
            </label>
            <label>
              {cc.setCarrier}
              <select data-testid="cod-carrier" value={carrier} disabled={busy} onChange={(event) => setCarrier(event.target.value as CodCarrier)}>
                <option value="black_cat">{cc.carrierBlackCat}</option>
                <option value="hsinchu">{cc.carrierHsinchu}</option>
              </select>
            </label>
          </div>
          <p className="settings-note">{cc.setNote}</p>
          <div className="settings-actions">
            <button className="primary" type="submit" data-testid="cod-settings-save" disabled={busy}>
              {busy ? cc.saving : uncertain ? cc.retrySame : cc.save}
            </button>
          </div>
        </form>
      )}
      {problem && <p className="settings-warning" role="alert" data-testid="cod-settings-problem">{problem}</p>}
      {notice && <p className="message pending" role="status" data-testid="cod-settings-notice">{notice}</p>}
    </section>
  );
}
