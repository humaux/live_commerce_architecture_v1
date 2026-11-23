"use client";

// Buyer privacy page UI (/{locale}/privacy, U8), with the B11 overview and signed consent, export and erasure states: consent settings
// (checkboxes + save, context "settings"), data copy download (done / failed screens), erase confirm (typed ERASE, one Idempotency-Key per erase
// attempt, reused when the buyer confirms the same request) with completed / blocked / unconfirmed screens, and the erased state on 410.
// BFF GET /api/buyer/privacy, PUT /api/buyer/consents, POST /api/buyer/privacy/export, POST /api/buyer/privacy/erasure
// -> Go /v1/buyer/{privacy,consents,privacy/export,privacy/erasure} (internal/buyerhttp/privacy.go; internal/customers.*). Session/context handling
// reuses buyer-client.ts exactly like ClaimLink. No consent or erasure authority here: every save re-reads GET privacy before it shows a state, and
// an unknown result is retried with the same key or re-read, never assumed.
// Depends on: lib/privacy-copy.ts (notice wording), components/privacy/copy.ts (frame copy), components/privacy/ContactStoreButton.tsx.
// Non-goals: no local/session storage of privacy data, no third-party script, no merchant data.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import LocaleSelect from "./LocaleSelect";
import ShopChromeLink from "./ShopChromeLink";
import {
  BuyerClientError,
  buyerRequest,
  initializeBuyerSession,
  readBuyerSession,
  resetBuyerSession,
} from "../lib/buyer-client";
import { orderRecoveryRequired } from "../lib/purchase";
import { privacyCopy } from "../lib/privacy-copy";
import { privacyFrameCopy } from "./privacy/copy";
import ContactStoreButton from "./privacy/ContactStoreButton";
import styles from "./privacy/PrivacyCenter.module.css";
import {
  consentBody,
  consentPairs,
  erasureBody,
  LC_PRIVACY_POLICY_VERSION,
  privacyFailure,
  validBuyerExport,
  validBuyerPrivacy,
  validConsentResult,
  validErasureSummary,
  type BuyerPrivacy,
  type ConsentChannel,
  type ConsentPurpose,
  type PrivacyFailure,
} from "../lib/privacy-contract";

type View = "loading" | "ready" | "erased" | "failed" | "session";
type Screen = "overview" | "settings" | "exportDone" | "exportFail" | "confirm" | "blocked" | "unknown";
type OverviewAction = ConsentPurpose | "download" | "erase";

async function errorCode(response: Response): Promise<string> {
  const body: unknown = await response.json().catch(() => null);
  const code = body && typeof body === "object" ? (body as { code?: unknown }).code : null;
  return typeof code === "string" && /^[a-z_]{1,64}$/.test(code) ? code : "";
}

export default function PrivacyCenter({
  locale: initialLocale,
  notice = false,
}: {
  locale: Locale;
  notice?: boolean;
}) {
  const [locale, setLocale] = useState(initialLocale);
  const copy = privacyCopy[locale];
  const f = privacyFrameCopy[locale];
  const [view, setView] = useState<View>("loading");
  const [screen, setScreen] = useState<Screen>("overview");
  const [context, setContext] = useState("");
  const [privacy, setPrivacy] = useState<BuyerPrivacy | null>(null);
  const [draft, setDraft] = useState<Record<ConsentPurpose, boolean>>({ marketing_messages: false, ads_personalization: false });
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const epoch = useRef(0);
  const [typed, setTyped] = useState("");
  const [problem, setProblem] = useState("");
  const eraseKey = useRef("");
  const returnTo = useRef<OverviewAction | null>(null);
  const overviewButtons = useRef<Partial<Record<OverviewAction, HTMLButtonElement | null>>>({});
  const consentInputs = useRef<Partial<Record<ConsentPurpose, HTMLInputElement | null>>>({});
  const confirmationInput = useRef<HTMLInputElement>(null);
  const resultHeading = useRef<HTMLHeadingElement>(null);

  function fail(reason: unknown) {
    if (reason instanceof BuyerClientError && (["context_changed", "requires_reset"].includes(reason.code) || reason.status === 401))
      setView("session");
    else setView("failed");
  }
  async function readPrivacy(ctx: string) {
    const response = await buyerRequest("GET", "privacy", ctx);
    if (response.status === 410) return null;
    if (!response.ok) throw new BuyerClientError("request_failed", response.status);
    const value: unknown = await response.json().catch(() => null);
    if (!validBuyerPrivacy(value)) throw new BuyerClientError("invalid_response");
    return value;
  }
  async function load() {
    const version = ++epoch.current;
    setView("loading");
    setScreen("overview");
    try {
      const session = await initializeBuyerSession();
      if (!session.context) throw new BuyerClientError("requires_reset");
      const current = await readPrivacy(session.context);
      if (version !== epoch.current) return;
      setContext(session.context);
      setPrivacy(current);
      setView(current === null || current.erased ? "erased" : "ready");
    } catch (reason) {
      if (version === epoch.current) fail(reason);
    }
  }
  useEffect(() => {
    void load();
    // Personal data leaves memory with the page (bfcache must not keep it).
    const forget = () => {
      setPrivacy(null);
    };
    window.addEventListener("pagehide", forget);
    return () => {
      epoch.current++;
      window.removeEventListener("pagehide", forget);
    };
  }, []);
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  useEffect(() => {
    const action = returnTo.current;
    if (!action) return;
    if (view === "erased") {
      resultHeading.current?.focus();
    } else if (view === "ready") {
      if (screen === "overview") {
        overviewButtons.current[action]?.focus();
        returnTo.current = null;
      } else if (screen === "settings" && action !== "download" && action !== "erase") {
        consentInputs.current[action]?.focus();
      } else if (screen === "confirm") {
        confirmationInput.current?.focus();
      } else {
        resultHeading.current?.focus();
      }
    }
  }, [screen, view]);

  async function renew() {
    setBusy("renew");
    try {
      const current = await readBuyerSession();
      if (current.context && current.state !== "active") {
        if (orderRecoveryRequired(current.context)) throw new BuyerClientError("requires_reset");
        await resetBuyerSession(current.context);
      }
      await load();
    } catch (reason) {
      fail(reason);
    } finally {
      setBusy("");
    }
  }

  // One consent write; "erased" on 410, true only when Go answered a valid result. An unknown result is settled by the re-read in save().
  async function putConsent(purpose: ConsentPurpose, channel: ConsentChannel, granted: boolean) {
    const response = await buyerRequest("PUT", "consents", context, consentBody(purpose, channel, granted, "settings"), `consent-${crypto.randomUUID()}`);
    if (response.status === 410) return "erased" as const;
    const value: unknown = response.ok ? await response.json().catch(() => null) : null;
    return response.ok && validConsentResult(value);
  }
  async function save() {
    if (busy || !privacy) return;
    setBusy("save");
    setMessage(null);
    try {
      let saved = true;
      for (const { purpose, channel } of consentPairs) {
        if (draft[purpose] === privacy.consents[purpose]) continue;
        const result = await putConsent(purpose, channel, draft[purpose]);
        if (result === "erased") return setView("erased");
        saved = saved && result;
      }
      const current = await readPrivacy(context).catch(() => undefined);
      if (current === null || current?.erased) return setView("erased");
      if (current) setPrivacy(current);
      setMessage(saved ? { tone: "ok", text: f.saved } : { tone: "bad", text: f.saveFailed });
      setScreen("overview");
    } catch (reason) {
      if (reason instanceof BuyerClientError && (reason.code === "context_changed" || reason.status === 401)) fail(reason);
      else setMessage({ tone: "bad", text: f.saveFailed });
    } finally {
      setBusy("");
    }
  }

  async function download() {
    if (busy) return;
    returnTo.current = "download";
    setBusy("export");
    setMessage(null);
    try {
      const response = await buyerRequest("POST", "privacy/export", context, {}, `export-${crypto.randomUUID()}`);
      if (response.status === 410) return setView("erased");
      if (!response.ok) return setScreen("exportFail");
      const text = await response.text();
      let parsed: unknown = null;
      try {
        parsed = JSON.parse(text);
      } catch {
        /* falls through to the failure screen */
      }
      if (!validBuyerExport(parsed)) return setScreen("exportFail");
      // The file is the buyer's own data: a Blob download, object URL revoked right after the click, nothing stored.
      const href = URL.createObjectURL(new Blob([text], { type: "application/json" }));
      const link = document.createElement("a");
      link.href = href;
      link.download = "my-data.json";
      link.rel = "noopener";
      document.body.append(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(href), 1000);
      setScreen("exportDone");
    } catch (reason) {
      if (reason instanceof BuyerClientError && (reason.code === "context_changed" || reason.status === 401)) fail(reason);
      else setScreen("exportFail");
    } finally {
      setBusy("");
    }
  }

  function beginErase() {
    returnTo.current = "erase";
    eraseKey.current = `erase-${crypto.randomUUID()}`;
    setTyped("");
    setProblem("");
    setScreen("confirm");
  }
  async function erase() {
    if (typed !== "ERASE" || busy) return;
    setBusy("erase");
    setProblem("");
    try {
      const response = await buyerRequest("POST", "privacy/erasure", context, erasureBody, eraseKey.current);
      if (response.status === 410) return setView("erased");
      if (response.ok) {
        const value: unknown = await response.json().catch(() => null);
        if (!validErasureSummary(value)) throw new BuyerClientError("invalid_response");
        setPrivacy(null);
        return setView("erased");
      }
      const kind: PrivacyFailure = privacyFailure(response.status, await errorCode(response));
      if (kind === "other" && response.status >= 500) return setScreen("unknown"); // result not known: the same key replays it
      // A definite refusal ends this key; the next attempt is a new request.
      eraseKey.current = `erase-${crypto.randomUUID()}`;
      if (kind === "erasure_blocked") return setScreen("blocked");
      setProblem(kind === "idempotency_conflict" ? f.conflict : f.generic);
    } catch (reason) {
      if (reason instanceof BuyerClientError && reason.code === "context_changed") return fail(reason);
      // Lost answer or unreadable body: the same key replays the stored result, so confirming the same request is safe.
      setScreen("unknown");
    } finally {
      setBusy("");
    }
  }

  const label = (purpose: ConsentPurpose) => (purpose === "marketing_messages" ? f.marketing : f.ads);
  const overview = screen === "overview";
  const showOverview = view === "ready" && overview;
  return (
    <main className={showOverview ? styles.overview : "purchase-main claim-main"} aria-busy={view === "loading" || busy !== ""}>
      <div className={showOverview ? `sf-wrap ${styles.content}` : undefined}>
        <div className={styles.toolbar}>
          {showOverview && (
            <ShopChromeLink className="sf-link" href={`/${locale}`}><span aria-hidden="true">‹&nbsp;</span>{f.backToStore}</ShopChromeLink>
          )}
          <LocaleSelect
            locale={locale}
            label={copy.language}
            path="/privacy"
            disabled={busy !== ""}
            onSelect={(next) => {
              window.history.replaceState(window.history.state, "", `/${next}/privacy`);
              setLocale(next);
            }}
          />
        </div>
        {view === "loading" && <p role="status">{copy.loading}</p>}
        {view === "failed" && (
          <div role="alert" className="purchase-error">
            <p>{copy.failed}</p>
            <button disabled={busy !== ""} onClick={() => void load()}>{copy.reload}</button>
          </div>
        )}
        {view === "session" && (
          <div role="alert" className="purchase-error">
            <p>{copy.session}</p>
            <button disabled={busy !== ""} onClick={() => void renew()}>{copy.renew}</button>
          </div>
        )}
        {view === "erased" && (
          <section role="status" className="claim-preview" data-testid="privacy-erased">
            <h1 ref={resultHeading} tabIndex={-1}>{f.erasedTitle}</h1>
            <p>{f.erasedText}</p>
            <p><ShopChromeLink className="sf-link" href={`/${locale}`}>{f.backToStore}</ShopChromeLink></p>
          </section>
        )}
        {showOverview && privacy && (
          <>
            <header className={`sf-pagehead ${styles.heading}`}>
              <h1>{f.title}</h1>
              <p className="sf-pagehead__intro">{f.intro}</p>
            </header>
            {message && (
              <p role={message.tone === "ok" ? "status" : "alert"} className={message.tone === "ok" ? "claim-success" : "claim-note"} data-testid="privacy-message">
                {message.text}
              </p>
            )}
            <div className={styles.cards}>
              <section className={styles.card} aria-labelledby="privacy-choices-title" data-testid="privacy-choices">
                <h2 id="privacy-choices-title">{f.choicesTitle}</h2>
                <ul className={styles.choices}>
                  {consentPairs.map(({ purpose }) => (
                    <li key={purpose} className={styles.choice} data-testid={`privacy-${purpose}`}>
                      <h3 id={`privacy-label-${purpose}`}>{label(purpose)}</h3>
                      <p>{purpose === "ads_personalization" ? f.adsNote : f.marketingNote}</p>
                      <div className={styles.choiceAction}>
                        <button
                          className="sf-btn sf-btn--ghost"
                          id={`privacy-edit-${purpose}`}
                          aria-labelledby={`privacy-label-${purpose} privacy-edit-${purpose}`}
                          aria-describedby={`privacy-status-${purpose}`}
                          data-testid={`privacy-manage-${purpose}`}
                          ref={(button) => { overviewButtons.current[purpose] = button; }}
                          disabled={busy !== ""}
                          onClick={() => {
                            returnTo.current = purpose;
                            setDraft({ ...privacy.consents });
                            setMessage(null);
                            setScreen("settings");
                          }}
                        >
                          {f.manage}
                        </button>
                        <span id={`privacy-status-${purpose}`}>{privacy.consents[purpose] ? f.on : f.off}</span>
                      </div>
                    </li>
                  ))}
                </ul>
                <p>{f.choicesIntro}</p>
              </section>
              <div>
                <section className={styles.card} aria-labelledby="privacy-download-title" data-testid="privacy-data-card">
                  <h2 id="privacy-download-title">{f.downloadTitle}</h2>
                  <p>{f.downloadText}</p>
                  <button
                    className="sf-btn sf-btn--primary sf-btn--block"
                    data-testid="privacy-download"
                    ref={(button) => { overviewButtons.current.download = button; }}
                    disabled={busy !== ""}
                    onClick={() => void download()}
                  >
                    {busy === "export" ? f.downloading : f.download}
                  </button>
                  <div className={styles.erase}>
                    <h3>{f.eraseTitle}</h3>
                    <p>{f.eraseText}</p>
                    <p>{f.eraseWait}</p>
                    <button
                      className="sf-btn sf-btn--ghost sf-btn--block"
                      data-testid="privacy-erase"
                      ref={(button) => { overviewButtons.current.erase = button; }}
                      disabled={busy !== ""}
                      onClick={beginErase}
                    >
                      {f.eraseReview}
                    </button>
                  </div>
                </section>
                <a className="sf-link" href="#notice">{f.readNotice}</a>
              </div>
            </div>
            <section className={`claim-cart ${styles.supplement}`} aria-labelledby="privacy-use-title">
              <h2 id="privacy-use-title">{f.useTitle}</h2>
              <p>{f.useShip}</p>
              <p>{f.usePay}</p>
            </section>
          </>
        )}
        {view === "ready" && privacy && screen === "settings" && (
          <form
            className="claim-preview"
            data-testid="privacy-settings"
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <h1>{f.settingsTitle}</h1>
            <h2>{f.requiredTitle}</h2>
            <p>{f.requiredText}</p>
            <h2>{f.optionalTitle}</h2>
            {(
              [
                ["marketing_messages", f.optMarketing],
                ["ads_personalization", f.optAds],
              ] as const
            ).map(([purpose, text]) => (
              <label key={purpose} className="consent-box">
                <input
                  type="checkbox"
                  ref={(input) => { consentInputs.current[purpose] = input; }}
                  data-testid={`privacy-check-${purpose}`}
                  checked={draft[purpose]}
                  disabled={busy !== ""}
                  onChange={(event) => setDraft((old) => ({ ...old, [purpose]: event.target.checked }))}
                />{" "}
                {text}
              </label>
            ))}
            {/* The O6 disclosure (purchase events go to Meta, sent events cannot be recalled) stays beside the ads choice; the frames do not draw it. */}
            <p className="claim-note" data-testid="privacy-ads-disclosure">{copy.adsDisclosure}</p>
            <p className="claim-note">{f.settingsNote}</p>
            <div className="claim-actions">
              <button type="submit" className="primary" data-testid="privacy-save" disabled={busy !== ""}>{busy === "save" ? f.saving : f.save}</button>
              <button type="button" disabled={busy !== ""} onClick={() => setScreen("overview")}>{f.cancel}</button>
            </div>
          </form>
        )}
        {view === "ready" && screen === "exportDone" && (
          <section className="claim-preview" role="status" data-testid="privacy-export-done">
            <h1 ref={resultHeading} tabIndex={-1}>{f.exportDoneTitle}</h1>
            <h2>{f.exportDoneBadge}</h2>
            <p>{f.exportDoneText}</p>
            <p className="claim-note">{f.exportDoneNote}</p>
            <button className="primary" onClick={() => setScreen("overview")}>{f.back}</button>
          </section>
        )}
        {view === "ready" && screen === "exportFail" && (
          <section className="claim-preview" role="alert" data-testid="privacy-export-failed">
            <h1 ref={resultHeading} tabIndex={-1}>{f.exportFailTitle}</h1>
            <h2>{f.exportFailBadge}</h2>
            <p>{f.exportFailText}</p>
            <div className="claim-actions">
              <button className="primary" disabled={busy !== ""} onClick={() => void download()}>{f.retry}</button>
              <ContactStoreButton label={f.contact} locale={locale} />
            </div>
          </section>
        )}
        {view === "ready" && screen === "confirm" && (
          <form
            className="claim-preview"
            data-testid="privacy-erase-dialog"
            onSubmit={(event) => {
              event.preventDefault();
              void erase();
            }}
          >
            <h1>{f.confirmTitle}</h1>
            <h2>{f.confirmSub}</h2>
            <p>{f.confirmText}</p>
            <label className="wide">
              {f.typeLabel}
              <input
                ref={confirmationInput}
                data-testid="privacy-erase-typed"
                autoComplete="off"
                autoCapitalize="off"
                spellCheck={false}
                value={typed}
                onChange={(event) => setTyped(event.target.value)}
              />
            </label>
            {problem && <p role="alert" className="purchase-error" data-testid="privacy-erase-problem">{problem}</p>}
            <div className="claim-actions">
              <button type="submit" className="primary" data-testid="privacy-erase-submit" disabled={typed !== "ERASE" || busy !== ""}>
                {busy === "erase" ? f.erasing : f.confirm}
              </button>
              <button type="button" disabled={busy !== ""} onClick={() => setScreen("overview")}>{f.cancel}</button>
            </div>
          </form>
        )}
        {view === "ready" && screen === "blocked" && (
          <section className="claim-preview" role="alert" data-testid="privacy-erase-blocked">
            <h1 ref={resultHeading} tabIndex={-1}>{f.blockedTitle}</h1>
            <h2>{f.blockedSub}</h2>
            <p>{f.blockedText}</p>
            <p className="claim-note">{f.blockedNote}</p>
            <div className="claim-actions">
              <a className="tap-link" href={`/${locale}/orders/lookup`}>{f.viewOrders}</a>
              <button onClick={() => setScreen("overview")}>{f.back}</button>
            </div>
          </section>
        )}
        {view === "ready" && screen === "unknown" && (
          <section className="claim-preview" role="alert" data-testid="privacy-erase-unknown">
            <h1 ref={resultHeading} tabIndex={-1}>{f.unknownTitle}</h1>
            <h2>{f.unknownSub}</h2>
            <p>{f.unknownText}</p>
            <div className="claim-actions">
              <button className="primary" disabled={busy !== ""} onClick={() => void erase()}>{busy === "erase" ? f.erasing : f.confirmSame}</button>
              <button disabled={busy !== ""} onClick={() => setScreen("overview")}>{f.closeLater}</button>
            </div>
          </section>
        )}
        {(overview || view !== "ready") && (
          <section
            className={`claim-cart ${styles.supplement}`}
            aria-labelledby="privacy-notice-title"
            id="notice"
            data-testid="privacy-notice"
            data-highlight={notice ? "true" : undefined}
          >
            <h2 id="privacy-notice-title">
              {copy.noticeTitle} ({copy.noticeVersion(LC_PRIVACY_POLICY_VERSION)})
            </h2>
            <ul>
              {copy.noticeItems.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </main>
  );
}
