"use client";

// Settings -> 网店发布 card (mounted by SettingsWizard.tsx below the setup steps): shows whether the store is published,
// the current serving origin (read-only), and a publish / unpublish button with an inline confirm. R5 store-domains
// (Decision 3) adds an independent merchant domain settings section: GET/POST storefront/domains and POST
// storefront/domains/{suspend,detach} (lib/storefront-client.ts -> Go internal/httpapi/storefront.go). A GET 403 hides the
// card (no integration:read); a stale version (409 conflict) reloads it. Nothing is optimistic: publication and move writes
// re-GET. The domain-request response is the one exception: its one-time DNS instructions (TXT token) appear only there,
// so they are rendered directly. The operator CLI (cmd/store-admin) remains the break-glass for the same state.
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import {
  moveDomain,
  readDomains,
  readStorefront,
  requestDomain,
  setPublication,
  type StorefrontState,
} from "@/lib/storefront-client";
import type { DomainRequestResult, StorefrontDomains } from "@/lib/storefront-model";
import { storefrontCopy } from "@/lib/storefront-copy";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";

const hostnameShape = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

function hostOf(origin: string) {
  return origin.slice("https://".length);
}

export function StorefrontSettings({ store, locale }: { store: string; locale: Locale }) {
  // Do not carry a DNS token, confirmation or uncertain command into another store.
  return <StorefrontSettingsForStore key={store} store={store} locale={locale} />;
}

function StorefrontSettingsForStore({ store, locale }: { store: string; locale: Locale }) {
  const sc = storefrontCopy[locale];
  const [load, setLoad] = useState<Load>("loading");
  const [state, setState] = useState<StorefrontState | null>(null);
  const [domainsLoad, setDomainsLoad] = useState<Load>("loading");
  const [domains, setDomains] = useState<StorefrontDomains | null>(null);
  const [tick, setTick] = useState(0);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [boundary, setBoundary] = useState("");
  const [hostname, setHostname] = useState("");
  const [requesting, setRequesting] = useState(false);
  const [requestProblem, setRequestProblem] = useState("");
  const [dns, setDns] = useState<DomainRequestResult | null>(null);
  const [moving, setMoving] = useState<{ origin: string; action: "suspend" | "detach" } | null>(null);
  const [moveBusy, setMoveBusy] = useState(false);
  const [copyNotice, setCopyNotice] = useState("");
  const [unconfirmed, setUnconfirmed] = useState(false);
  const [journalReady, setJournalReady] = useState(false);
  const journalKey = useRef("");
  const pending = useRef<{ key: string; published: boolean } | null>(null);
  const domainCommand = useRef<{ key: string; hostname: string } | null>(null);
  const moveCommand = useRef<{ key: string; origin: string; action: "suspend" | "detach" } | null>(null);
  const domainBusy = useRef(false);

  // The session fence captured when the card loads: writeSettings refuses to send if the session changed since.
  useEffect(() => {
    let live = true;
    sessionBoundary().then((value) => {
      if (!live) return;
      setBoundary(value);
      journalKey.current = `commerce-domain-pending:${store}:${value}`;
      try {
        setUnconfirmed(sessionStorage.getItem(journalKey.current) !== null);
        setJournalReady(true);
      } catch { setJournalReady(false); }
    }, () => { if (live) setBoundary(""); });
    return () => {
      live = false;
    };
  }, [store]);

  useEffect(() => {
    const active = new AbortController();
    readStorefront(store, active.signal).then(
      (value) => {
        if (active.signal.aborted) return;
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

  useEffect(() => {
    const active = new AbortController();
    readDomains(store, active.signal).then(
      (value) => {
        if (active.signal.aborted) return;
        setDomains(value);
        setDomainsLoad("ready");
      },
      (error) => {
        if (active.signal.aborted) return;
        setDomains(null);
        setDomainsLoad(error instanceof OrderReadError && error.code === "forbidden" ? "hidden" : "error");
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

  function validHostname(value: string) {
    return value.length <= 253 && hostnameShape.test(value);
  }

  function beginDomainWrite() {
    // The current Go endpoints do NOT deduplicate keys. Preserve an unresolved
    // write across reloads; never offer a blind replay, even with the same key.
    if (!journalReady || unconfirmed || !journalKey.current) return false;
    try {
      if (sessionStorage.getItem(journalKey.current) !== null) {
        setUnconfirmed(true);
        return false;
      }
      sessionStorage.setItem(journalKey.current, "pending");
      return true;
    } catch { setJournalReady(false); return false; }
  }

  function endDomainWrite(uncertain: boolean) {
    setUnconfirmed(uncertain);
    if (!uncertain) {
      try { sessionStorage.removeItem(journalKey.current); }
      catch { setUnconfirmed(true); setJournalReady(false); }
    }
  }

  async function submitDomain(event: FormEvent) {
    event.preventDefault();
    const proposed = hostname.trim().toLowerCase();
    if (domainBusy.current) return;
    if (moveCommand.current) { setRequestProblem(sc.uncertain); return; }
    setRequestProblem("");
    if (!validHostname(proposed)) {
      setRequestProblem(sc.domains.errors.invalid_request);
      return;
    }
    if (!beginDomainWrite()) return;
    domainCommand.current ??= { key: `storefront-domain-${crypto.randomUUID()}`, hostname: proposed };
    domainBusy.current = true;
    setRequesting(true);
    const result = await requestDomain(store, domainCommand.current.key, proposed, boundary);
    endDomainWrite(!result.ok && result.uncertain);
    domainBusy.current = false;
    setRequesting(false);
    if (result.ok) {
      domainCommand.current = null;
      setDns(result.result);
      setCopyNotice("");
      setHostname("");
      setTick((value) => value + 1); // the list now shows the REQUESTED row
    } else {
      if (!result.uncertain) domainCommand.current = null;
      setDns(null);
      if (result.uncertain) setRequestProblem(sc.domains.unconfirmed);
      else if (result.code === "forbidden") setRequestProblem(sc.domains.errors.forbidden);
      else setRequestProblem(sc.domains.errors[result.code] ?? sc.domains.errors.unavailable);
      setTick((value) => value + 1); // the server may have applied it; re-read either way
    }
  }

  async function confirmMove() {
    if (domainBusy.current || !moving) return;
    if (domainCommand.current) { setRequestProblem(sc.uncertain); return; }
    if (!beginDomainWrite()) return;
    if (moveCommand.current && (moveCommand.current.origin !== moving.origin || moveCommand.current.action !== moving.action)) {
      setMoving({ origin: moveCommand.current.origin, action: moveCommand.current.action });
      return;
    }
    moveCommand.current ??= { key: `storefront-move-${crypto.randomUUID()}`, ...moving };
    domainBusy.current = true;
    setMoveBusy(true);
    setRequestProblem("");
    const command = moveCommand.current;
    const result = await moveDomain(store, command.key, command.origin, command.action, boundary);
    endDomainWrite(!result.ok && result.uncertain);
    domainBusy.current = false;
    setMoveBusy(false);
    if (result.ok || !result.uncertain) {
      moveCommand.current = null;
      setMoving(null);
    } else { setMoving(null); setRequestProblem(sc.domains.unconfirmed); }
    if (!result.ok && !result.uncertain) {
      if (result.code === "forbidden") setRequestProblem(sc.domains.errors.forbidden);
      else setRequestProblem(sc.domains.errors[result.code] ?? sc.domains.errors.unavailable);
    }
    setTick((value) => value + 1); // re-GET: the server's state, never our guess
  }

  const domainRows = domains?.domains ?? [];
  const domainsLocked = !journalReady || unconfirmed || requesting || moveBusy || domainCommand.current !== null || moveCommand.current !== null;
  const moveable = (row: { state: string }) =>
    row.state === "REQUESTED" || row.state === "OWNERSHIP_PENDING" || row.state === "TLS_PENDING" || row.state === "ACTIVE";

  return (
    <>
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
    {domainsLoad !== "hidden" && <section className="settings-fields storefront-domains-card" data-testid="storefront-domains-card" aria-label={sc.domains.title}>
      {domainsLoad === "loading" && <p role="status">{sc.loading}</p>}
      {domainsLoad === "ready" && domains && (
        <div className="storefront-domains" data-testid="storefront-domains">
          <h2 className="settings-section-title settings-subtitle">{sc.domains.title}</h2>
          <p className="settings-note">{sc.domains.intro}</p>
          {(!journalReady || unconfirmed) && <div className="settings-warning" role="status" data-testid="storefront-domain-unconfirmed">
            <p>{unconfirmed ? sc.domains.unconfirmed : sc.domains.storageUnavailable}</p>
            <button type="button" onClick={() => setTick(value => value + 1)}>{sc.domains.refresh}</button>
          </div>}
          <ul className="storefront-domain-list">
            {domainRows.map((row) => (
              <li key={row.origin} className="storefront-domain-row" data-testid="storefront-domain-row" data-state={row.state}>
                <span className="storefront-domain-origin" data-testid="storefront-domain-origin">
                  <a href={row.origin} target="_blank" rel="noopener noreferrer">{row.origin}</a>
                  {row.serving && <span className="storefront-domain-badge">{sc.domains.serving}</span>}
                </span>
                <span className="storefront-domain-state" data-testid="storefront-domain-state">{sc.domains.states[row.state]}</span>
                {row.token && row.state !== "DETACHED" && (
                  <div className="storefront-domain-token" data-testid="storefront-domain-token">
                    <code>{sc.domains.dnsTxtName}: _lc-verify.{hostOf(row.origin)}</code>
                    <code>{sc.domains.dnsTxtValue}: {row.token}</code>
                  </div>
                )}
                <div className="storefront-domain-actions">
                  {moveable(row) && (
                    <button type="button" data-testid="storefront-domain-suspend" disabled={domainsLocked}
                      onClick={() => { setRequestProblem(""); setMoving({ origin: row.origin, action: "suspend" }); }}>
                      {sc.domains.suspend}
                    </button>
                  )}
                  {row.state !== "DETACHED" && (
                    <button type="button" data-testid="storefront-domain-detach" disabled={domainsLocked}
                      onClick={() => { setRequestProblem(""); setMoving({ origin: row.origin, action: "detach" }); }}>
                      {sc.domains.detach}
                    </button>
                  )}
                </div>
              </li>
            ))}
            {domainRows.length === 0 && (
              <li className="settings-note" data-testid="storefront-domains-empty">{sc.domains.platform}</li>
            )}
          </ul>

          <form className="storefront-domain-request" onSubmit={submitDomain} data-testid="storefront-domain-request">
            <label>
              <span>{sc.domains.hostnameLabel}</span>
              <input
                name="hostname"
                value={hostname}
                onChange={(event) => setHostname(event.target.value)}
                placeholder={sc.domains.hostnamePlaceholder}
                autoCapitalize="none"
                spellCheck={false}
                disabled={domainsLocked}
              />
            </label>
            <button className="primary" type="submit" data-testid="storefront-domain-submit" disabled={domainsLocked}>
              {requesting ? sc.domains.requesting : sc.domains.request}
            </button>
          </form>

          {dns && (
            <div className="storefront-dns" data-testid="storefront-dns" role="status">
              <h4>{sc.domains.dnsTitle}</h4>
              <p className="settings-note">{sc.domains.dnsIntro}</p>
              <dl>
                <div>
                  <dt>{sc.domains.dnsTxtName}</dt>
                  <dd><code data-testid="storefront-dns-txt-name">{dns.dns.txt_name}</code></dd>
                </div>
                <div>
                  <dt>{sc.domains.dnsTxtValue}</dt>
                  <dd><code data-testid="storefront-dns-txt-value">{dns.dns.txt_value}</code></dd>
                </div>
                {dns.dns.apex ? (
                  <div>
                    <dt>{sc.domains.dnsApex}</dt>
                    <dd><code data-testid="storefront-dns-apex">{hostOf(dns.origin)}</code></dd>
                    <dd className="settings-note">{sc.domains.dnsApexNote}</dd>
                  </div>
                ) : (
                  <div>
                    <dt>{sc.domains.dnsCname}</dt>
                    <dd><code data-testid="storefront-dns-cname">{hostOf(dns.origin)} → {dns.dns.cname_target}</code></dd>
                  </div>
                )}
              </dl>
              <p className="settings-note">{sc.domains.verifying}</p>
              <button type="button" data-testid="storefront-dns-copy" onClick={async () => {
                try {
                  const addressRecord = dns.dns.apex ? sc.domains.dnsApexNote : `CNAME ${hostOf(dns.origin)} → ${dns.dns.cname_target}`;
                  const records = `TXT ${dns.dns.txt_name}\n${dns.dns.txt_value}\n${addressRecord}`;
                  await navigator.clipboard.writeText(records);
                  setCopyNotice(sc.domains.copied);
                } catch { setCopyNotice(sc.domains.copyFailed); }
              }}>{sc.domains.copy}</button>
              <p role="status" className="settings-note">{copyNotice}</p>
            </div>
          )}

          {moving && (
            <div className="settings-pending" role="alertdialog" aria-label={sc.domains[moving.action]} data-testid="storefront-domain-confirm">
              <p>{moving.action === "suspend" ? sc.domains.confirmSuspend : sc.domains.confirmDetach}</p>
              <p className="storefront-domain-origin"><strong>{moving.origin}</strong></p>
              <div className="settings-actions">
                <button type="button" disabled={moveBusy || moveCommand.current !== null} onClick={() => setMoving(null)}>{sc.cancel}</button>
                <button className="primary" type="button" data-testid="storefront-domain-confirm-yes" disabled={moveBusy}
                  onClick={() => void confirmMove()}>
                  {moveBusy ? sc.saving : sc.confirm}
                </button>
              </div>
            </div>
          )}

          {requestProblem && <p className="settings-warning" role="alert" data-testid="storefront-domain-problem">{requestProblem}</p>}
        </div>
      )}
      {domainsLoad === "error" && (
        <div role="status" data-testid="storefront-domains-error">
          <p>{sc.domains.errors.unavailable}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{sc.retry}</button>
        </div>
      )}
    </section>}
    </>
  );
}
