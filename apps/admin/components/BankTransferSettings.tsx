"use client";

// Settings -> bank transfer card (mounted by SettingsWizard.tsx next to <LogisticsSettings>; contracts/storefront-v2.md §C): the switch, the
// merchant's bank details and the transfer window (6..168 h). BFF routes (lib/logistics-client.ts) -> Go internal/httpapi/offline.go:
//   GET|PUT /api/stores/{store}/bank-transfer-settings   (integration:read / integration:manage, version CAS, Idempotency-Key)
// A GET 403 hides the card (no integration:read); a PUT 403 shows a no-permission notice. Nothing is optimistic: every save re-GETs the card.
// One Idempotency-Key per distinct body, reused only for a byte-identical retry after an unknown outcome. A change never touches orders
// already placed (each keeps the bank details it was given); the account number is shown only to the merchant and to the buyer of that order.
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import { putTransferSettings, readTransferSettings } from "@/lib/logistics-client";
import { transferSettingsBody, type TransferSettings } from "@/lib/transfer-model";
import { transferCopy, transferError } from "@/lib/transfer-copy";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";

export function BankTransferSettings({ store, locale }: { store: string; locale: Locale }) {
  const tc = transferCopy[locale];
  const [boundary, setBoundary] = useState("");
  const [load, setLoad] = useState<Load>("loading");
  const [saved, setSaved] = useState<TransferSettings | null>(null);
  const [tick, setTick] = useState(0);
  const [enabled, setEnabled] = useState(false);
  const [allowCvs, setAllowCvs] = useState(false);
  const [bankName, setBankName] = useState("");
  const [branch, setBranch] = useState("");
  const [accountName, setAccountName] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [windowHours, setWindowHours] = useState("72");
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
    readTransferSettings(store, active.signal).then(
      (value) => {
        setSaved(value);
        setEnabled(value.enabled);
        setAllowCvs(value.allow_cvs);
        setBankName(value.bank_name);
        setBranch(value.branch);
        setAccountName(value.account_name);
        setAccountNumber(value.account_number);
        setWindowHours(String(value.window_hours));
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
    const body = transferSettingsBody({
      expectedVersion: saved.version, enabled, allowCvs, bankName, branch, accountName, accountNumber, windowHours,
    });
    if (!body) {
      setProblem(tc.invalid);
      return;
    }
    const text = JSON.stringify(body);
    if (pending.current?.body !== text) pending.current = { key: `xfer-set-${crypto.randomUUID()}`, body: text };
    setBusy(true);
    setProblem("");
    const result = await putTransferSettings(store, pending.current.key, text, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setUncertain(false);
      setNotice(tc.saved);
      setTick((value) => value + 1);
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(tc.uncertain);
      return;
    }
    pending.current = null;
    setUncertain(false);
    setProblem(result.code === "forbidden" ? tc.noPermission : transferError(tc, result.code));
    if (result.code === "version_changed") setTick((value) => value + 1);
  }

  return (
    <section className="settings-fields" data-testid="transfer-settings-card" aria-labelledby="transfer-settings-title">
      <h2 id="transfer-settings-title" className="settings-section-title settings-subtitle">{tc.setTitle}</h2>
      <p className="settings-note">{tc.setIntro}</p>
      {load === "loading" && <p role="status">{tc.loading}</p>}
      {load === "error" && (
        <div role="status">
          <p>{tc.loadFailed}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{tc.retry}</button>
        </div>
      )}
      {load === "ready" && saved && (
        <form onSubmit={(event) => void submit(event)} className="settings-fields">
          <label className="settings-check">
            <input type="checkbox" data-testid="transfer-enabled" checked={enabled} disabled={busy} onChange={(event) => setEnabled(event.target.checked)} />
            {tc.setEnabled}
          </label>
          <label className="settings-check">
            <input type="checkbox" data-testid="transfer-allow-cvs" checked={allowCvs} disabled={busy} onChange={(event) => setAllowCvs(event.target.checked)} />
            {tc.setAllowCvs}
          </label>
          <div className="settings-field-grid">
            <label>
              {tc.setBank}
              <input data-testid="transfer-bank-name" value={bankName} maxLength={60} disabled={busy} onChange={(event) => setBankName(event.target.value)} />
            </label>
            <label>
              {tc.setBranch}
              <input data-testid="transfer-branch" value={branch} maxLength={60} disabled={busy} onChange={(event) => setBranch(event.target.value)} />
            </label>
            <label>
              {tc.setAccountName}
              <input data-testid="transfer-account-name" value={accountName} maxLength={60} disabled={busy} onChange={(event) => setAccountName(event.target.value)} />
            </label>
            <label>
              {tc.setAccountNumber}
              <input
                data-testid="transfer-account-number"
                inputMode="numeric"
                autoComplete="off"
                value={accountNumber}
                maxLength={32}
                disabled={busy}
                onChange={(event) => setAccountNumber(event.target.value)}
              />
              <small>{tc.setAccountHint}</small>
            </label>
            <label>
              {tc.setWindow}
              <input data-testid="transfer-window" inputMode="numeric" value={windowHours} maxLength={3} disabled={busy} onChange={(event) => setWindowHours(event.target.value)} />
              <small>{tc.setWindowHint}</small>
            </label>
          </div>
          <p className="settings-note">{tc.setNote}</p>
          <div className="settings-actions">
            <button className="primary" type="submit" data-testid="transfer-settings-save" disabled={busy}>
              {busy ? tc.saving : uncertain ? tc.retrySame : tc.save}
            </button>
          </div>
        </form>
      )}
      {problem && <p className="settings-warning" role="alert" data-testid="transfer-settings-problem">{problem}</p>}
      {notice && <p className="message pending" role="status" data-testid="transfer-settings-notice">{notice}</p>}
    </section>
  );
}
