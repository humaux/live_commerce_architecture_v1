"use client";

// Settings -> 网店发布 card (mounted by SettingsWizard.tsx below the setup steps): shows whether the store is published,
// the platform-bound ACTIVE origin (read-only, or "awaiting platform domain"), and a publish / unpublish button with
// an inline confirm. BFF routes (lib/storefront-client.ts) -> Go internal/httpapi/storefront.go (contract
// published-storefront-resolver-v1 "Writer (R3)"): GET /api/stores/{store}/storefront, POST .../storefront/publication
// {published, expected_version}. A GET 403 hides the card (no integration:read); a stale version (409 conflict) reloads
// it. Nothing is optimistic: every write re-GETs. Domain binding is the platform operator's CLI, deliberately not here.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import { readStorefront, setPublication, type StorefrontState } from "@/lib/storefront-client";
import { storefrontCopy } from "@/lib/storefront-copy";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";

export function StorefrontSettings({ store, locale }: { store: string; locale: Locale }) {
  const sc = storefrontCopy[locale];
  const [load, setLoad] = useState<Load>("loading");
  const [state, setState] = useState<StorefrontState | null>(null);
  const [tick, setTick] = useState(0);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [boundary, setBoundary] = useState("");
  const pending = useRef<{ key: string; published: boolean } | null>(null);

  // The session fence captured when the card loads: writeSettings refuses to send if the session changed since.
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
    readStorefront(store, active.signal).then(
      (value) => {
        setState(value);
        setLoad("ready");
      },
      (error) => {
        if (active.signal.aborted) return;
        setState(null);
        setLoad(error instanceof OrderReadError && error.code === "forbidden" ? "hidden" : "error");
      },
    );
    return () => active.abort();
  }, [store, tick]);

  if (load === "hidden") return null;

  const serving = state?.domains.find((domain) => domain.serving) ?? null;
  const bound = serving ?? state?.domains[0] ?? null;
  const target = state ? !state.published : false;

  async function submit() {
    if (busy || !state) return;
    // A retry of the same intent reuses the key; a new intent gets a new one.
    if (pending.current?.published !== target) pending.current = { key: `storefront-${crypto.randomUUID()}`, published: target };
    setBusy(true);
    setProblem("");
    setNotice("");
    const result = await setPublication(store, pending.current.key, target, state.version, boundary);
    setBusy(false);
    setConfirming(false);
    if (result.ok) {
      pending.current = null;
      setNotice(target ? sc.savedPublished : sc.savedUnpublished);
    } else if (result.uncertain) {
      setProblem(sc.uncertain);
    } else {
      pending.current = null;
      setProblem(result.code === "forbidden" ? sc.noPermission : (sc.errors[result.code] ?? sc.errors.unavailable));
    }
    setTick((value) => value + 1); // re-GET: the server's state, never our guess
  }

  return (
    <section className="settings-fields storefront-card" data-testid="storefront-card" aria-labelledby="storefront-title">
      <h2 id="storefront-title" className="settings-section-title settings-subtitle">{sc.title}</h2>
      <p className="settings-note">{sc.intro}</p>
      {load === "loading" && <p role="status">{sc.loading}</p>}
      {load === "error" && (
        <div role="status">
          <p>{sc.unavailable}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{sc.retry}</button>
        </div>
      )}
      {load === "ready" && state && (
        <>
          <dl className="settings-status-list">
            <div>
              <dt>{sc.state}</dt>
              <dd data-testid="storefront-state">{state.published ? sc.published : sc.unpublished}</dd>
            </div>
            <div>
              <dt>{sc.domain}</dt>
              <dd data-testid="storefront-domain">
                {bound ? (
                  <>
                    <a href={bound.origin} target="_blank" rel="noopener noreferrer">{bound.origin}</a>
                    <small className="storefront-expiry">
                      {sc.validUntil}: {bound.valid_until.slice(0, 10)}
                    </small>
                  </>
                ) : (
                  sc.awaitingDomain
                )}
              </dd>
            </div>
          </dl>
          <p className={serving && state.published ? "settings-note" : "settings-warning"} data-testid="storefront-hint">
            {serving
              ? state.published
                ? sc.live
                : sc.readyToPublish
              : bound
                ? sc.domainExpired
                : state.published
                  ? sc.publishedNoDomain
                  : sc.awaitingDomain}
          </p>
          {confirming ? (
            <div className="settings-pending" role="alertdialog" aria-label={target ? sc.publish : sc.unpublish} data-testid="storefront-confirm">
              <p>{target ? sc.confirmPublish : sc.confirmUnpublish}</p>
              <div className="settings-actions">
                <button type="button" disabled={busy} onClick={() => setConfirming(false)}>{sc.cancel}</button>
                <button className="primary" type="button" data-testid="storefront-confirm-yes" disabled={busy} onClick={() => void submit()}>
                  {busy ? sc.saving : sc.confirm}
                </button>
              </div>
            </div>
          ) : (
            <div className="settings-actions">
              <button className="primary" type="button" data-testid="storefront-toggle" disabled={busy}
                onClick={() => { setNotice(""); setProblem(""); setConfirming(true); }}>
                {target ? sc.publish : sc.unpublish}
              </button>
            </div>
          )}
        </>
      )}
      {problem && <p className="settings-warning" role="alert" data-testid="storefront-problem">{problem}</p>}
      {notice && <p className="message pending" role="status" data-testid="storefront-notice">{notice}</p>}
    </section>
  );
}
