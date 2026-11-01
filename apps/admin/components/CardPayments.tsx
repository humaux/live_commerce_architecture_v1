// Purpose: Card payments page — platform Stripe activation state, verbatim-terms enable dialog, disable control.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/card-payments-client, @/lib/customers-client, @/lib/card-payments-model, @/lib/card-payments-copy, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, ./orders.css, ./customers.css, ./finance/money.css
// Used by: apps/admin/app/[locale]/settings/payments/card/page.tsx
"use client";

// (/{locale}/settings/payments/card): platform-collected card payments (contract stripe-platform-account-v1 §5
// and the integrator ruling): state badges, the enable dialog quotes the §5 terms verbatim with a live
// descriptor-length check and resulting preview; no credential input and no account ids/keys/approval ids
// anywhere. The PUT is CAS-guarded by expected_version; a 409 or an uncertain outcome reloads the summary
// (reload-and-retry) instead of retrying blindly.
import { useState } from "react";
import { moneyCopy, type Locale } from "@live-commerce/i18n";
import { Badge } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { readCardSummary, setCardPayments } from "@/lib/card-payments-client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import {
  buildCardInput,
  cardView,
  descriptorBase,
  descriptorPreview,
  descriptorSuffixOf,
  validDescriptorSuffix,
  type CardSummary,
  type PlatformState,
  type StoreCardState,
} from "@/lib/card-payments-model.ts";
import { cardPaymentsCopy, type CardPaymentsCopy } from "@/lib/card-payments-copy.ts";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./orders.css";
import "./customers.css";
import "./finance/money.css";

const stateTone: Record<StoreCardState, "neutral" | "success" | "warning" | "danger"> = {
  NONE: "neutral",
  ENABLED: "success",
  DISABLED: "warning",
  BLOCKED: "danger",
};
const platformTone: Record<PlatformState, "neutral" | "success" | "info" | "danger"> = {
  NONE: "neutral",
  DESIGNATED: "info",
  OPEN: "success",
  CLOSED: "danger",
  REVOKED: "danger",
};

/** Owns the platform card-payments state for one store; writes go through the CAS-guarded keyless PUT. */
export function CardPayments({
  locale,
  stores: _stores,
  store,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = cardPaymentsCopy[locale], p = moneyCopy[locale];
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}`,
    store ? (signal) => readCardSummary(store.id, signal) : null,
    initialError,
  );
  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? c.notFound
    : read.status === "unavailable" ? c.unavailable
    : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.title} active="card-payments">
      <div className="money-page" data-testid="card-payments-page">
        <h1>{p.card}</h1>
        {(read.status === "loading" || read.status === "hidden") && (
          <p className="orders-message" role="status">{c.loading}</p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && read.data && store && (
          <Sections summary={read.data} store={store} boundary={read.boundary} refresh={read.refresh} reload={read.reload} locale={locale} c={c} />
        )}
      </div>
    </WorkspaceFrame>
  );
}

function Sections({
  summary,
  store,
  boundary,
  refresh,
  reload,
  locale,
  c,
}: {
  summary: CardSummary;
  store: Store;
  boundary: string;
  refresh: () => Promise<boolean>;
  reload: () => void;
  locale: Locale;
  c: CardPaymentsCopy;
}) {
  const p = moneyCopy[locale];
  const ready = summary.store_state === "ENABLED" && summary.platform_state === "OPEN" && summary.allowed;
  const canManage = store.role === "owner" || (store.permissions ?? []).includes("billing:manage");
  const [dialog, setDialog] = useState<"enable" | "disable" | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<{ kind: "saved" | "conflict" | "error"; code?: string } | null>(null);
  // The display state machine lives in card-payments-model.ts (unit-tested): not-open, not-allowlisted, BLOCKED, enable/disable.
  const { notOpen, showPlatform, blocked, canEnable, canDisable } = cardView(summary, canManage);

  // A failed re-read (tab hidden, network) must not leave "Saved." beside a stale badge: fall back to the full reload.
  async function resync() {
    if (!(await refresh())) reload();
  }

  async function submit(enabled: boolean, suffix: string | null) {
    setBusy(true);
    setNote(null);
    const outcome = await setCardPayments(store.id, buildCardInput(summary, enabled, suffix), boundary);
    setBusy(false);
    if (outcome.ok) {
      setDialog(null);
      setNote({ kind: "saved" });
      await resync();
      return;
    }
    // CAS lost, terms drifted, or the platform moved underneath us: reload-and-retry, never blind-retry.
    if (
      outcome.uncertain ||
      outcome.code === "version_changed" ||
      outcome.code === "terms_version_stale" ||
      outcome.code === "platform_stripe_unavailable" ||
      outcome.code === "platform_stripe_closed"
    ) {
      setDialog(null);
      setNote({ kind: "conflict" });
      await resync();
      return;
    }
    // The platform suspended or de-listed the store while the dialog was open: say why, and reload so the badge catches up.
    if (outcome.code === "platform_stripe_blocked" || outcome.code === "platform_stripe_not_allowed") {
      setDialog(null);
      setNote({ kind: "error", code: outcome.code });
      await resync();
      return;
    }
    setNote({ kind: "error", code: outcome.code });
  }

  return (
    <>
      {ready && <>
        <p>{p.cardIntro}</p>
        <section className="money-card"><h2>{p.unified}</h2><Badge tone="success">{p.ready}</Badge><div className="money-flow">{[[p.buyerPays,p.buyerPaysHint],[p.platformCollects,p.platformCollectsHint],[p.storePayout,p.storePayoutHint]].map(([heading,text]) => <div key={heading}><strong>{heading}</strong><p>{text}</p></div>)}</div><h2>{p.cardBasis}</h2><p>{p.cardBasisHint}</p></section>
        <section className="money-card"><h2>{p.fees}</h2><dl><div><dt>{p.weekly}</dt><dd>{p.weekly}</dd></div><div><dt>{p.platformFee}</dt><dd>{p.feePending}</dd></div></dl><p className="money-note">{p.feeHint}</p></section>
      </>}
      <section data-testid="card-state">
        <p>
          {c.stateLabel}{" "}
          <Badge tone={stateTone[summary.store_state]} data-testid="card-state-badge">
            {c[`state${summary.store_state}`]}
          </Badge>
        </p>
        {showPlatform && (
          <p>
            {c.platformLabel}{" "}
            <Badge tone={platformTone[summary.platform_state]} data-testid="platform-state-badge">
              {c[`platform${summary.platform_state}`]}
            </Badge>
          </p>
        )}
      </section>
      {notOpen && <p className="orders-message" data-testid="card-not-open">{c.notOpen}</p>}
      {blocked && <p className="orders-message" data-testid="card-blocked-note">{c.blockedNote}</p>}
      {note?.kind === "saved" && <p className="orders-message" role="status" data-testid="card-saved">{c.saved}</p>}
      {note?.kind === "conflict" && <p className="orders-message" role="alert" data-testid="card-conflict">{c.conflict}</p>}
      {note?.kind === "error" && (
        <p className="orders-message" role="alert" data-testid="card-write-error">{c.errors[note.code ?? ""] ?? c.errors.default}</p>
      )}
      {dialog === null && (
        <div>
          {canEnable && (
            <button type="button" data-testid="card-enable-open" onClick={() => { setNote(null); setDialog("enable"); }}>
              {c.enable}
            </button>
          )}
          {canDisable && (
            <button type="button" data-testid="card-disable-open" onClick={() => { setNote(null); setDialog("disable"); }}>
              {c.disable}
            </button>
          )}
        </div>
      )}
      <details className="money-card" data-testid="card-advanced"><summary>{p.advanced}</summary>
        {summary.descriptor_preview && (
          <p data-testid="card-descriptor-preview">
            {c.previewLabel}: {summary.descriptor_preview}
          </p>
        )}
      {(summary.currency || summary.min_minor !== null || summary.max_minor !== null) && (
        <section data-testid="card-limits">
          <h2>{c.limitsTitle}</h2>
          <dl>
            <div>
              <dt>{c.currencyLabel}</dt>
              <dd data-testid="card-currency">{summary.currency ?? "—"}</dd>
            </div>
            <div>
              <dt>{c.minLabel}</dt>
              <dd data-testid="card-min">
                {summary.currency && summary.min_minor !== null ? money(locale, summary.currency, summary.min_minor) : "—"}
              </dd>
            </div>
            <div>
              <dt>{c.maxLabel}</dt>
              <dd data-testid="card-max">
                {summary.currency && summary.max_minor !== null ? money(locale, summary.currency, summary.max_minor) : "—"}
              </dd>
            </div>
          </dl>
        </section>
      )}
      </details>
      {/* the settlement statements have no nav entry (nav:false); billing:manage holders reach them from here */}
      {canManage && dialog === null && (
        <p>
          <a href={`/${locale}/settings/settlements?store=${store.id}`} data-testid="card-settlements-link">{p.settlementLink}</a>
        </p>
      )}
      {dialog === "enable" && (
        <EnableDialog summary={summary} busy={busy} c={c} onConfirm={(suffix) => void submit(true, suffix)} onCancel={() => setDialog(null)} />
      )}
      {dialog === "disable" && (
        <section role="dialog" aria-label={c.disable} data-testid="card-disable-dialog">
          <h2>{c.disable}</h2>
          <p>{c.disableConfirm}</p>
          <div>
            <button type="button" data-testid="card-disable-confirm" disabled={busy} onClick={() => void submit(false, null)}>
              {busy ? c.saving : c.disable}
            </button>
            <button type="button" data-testid="card-disable-cancel" disabled={busy} onClick={() => setDialog(null)}>
              {c.cancel}
            </button>
          </div>
        </section>
      )}
    </>
  );
}

/** The enable dialog: verbatim §5 terms + accept checkbox, optional suffix with live rule checks and preview. */
function EnableDialog({
  summary,
  busy,
  c,
  onConfirm,
  onCancel,
}: {
  summary: CardSummary;
  busy: boolean;
  c: CardPaymentsCopy;
  onConfirm: (suffix: string | null) => void;
  onCancel: () => void;
}) {
  const [accepted, setAccepted] = useState(false);
  // A store that disabled and comes back keeps its suffix unless it clears the field (the GET only exposes the final preview).
  const [suffix, setSuffix] = useState(descriptorSuffixOf(summary.descriptor_preview) ?? "");
  // The summary preview carries a DISABLED store's retained suffix; descriptorBase strips it back to the
  // platform base so the live check and preview never double-count ("* " can appear in neither part).
  const base = summary.descriptor_preview ? descriptorBase(summary.descriptor_preview) : null;
  const value = suffix === "" ? null : suffix;
  // Length first (the server answers descriptor_suffix_too_long before the charset rule). The page enforces only the 10 the charset rule
  // allows; the 22-character total (SANDBOX L=10, LIVE the approval's PrefixLength) is the server's and comes back as its refusal.
  const tooLong = value !== null && Array.from(value).length > 10;
  const invalid = value !== null && !tooLong && !validDescriptorSuffix(value);
  const canConfirm = accepted && !invalid && !tooLong && !busy && summary.terms_version !== null;
  return (
    <section role="dialog" aria-label={c.dialogTitle} data-testid="card-enable-dialog">
      <h2>{c.dialogTitle}</h2>
      <p>{c.dialogIntro}</p>
      <blockquote data-testid="card-terms">{c.terms}</blockquote>
      {summary.terms_version && (
        <p data-testid="card-terms-version">
          {c.termsVersion}: {summary.terms_version}
        </p>
      )}
      <label>
        <input
          type="checkbox"
          data-testid="card-terms-accept"
          checked={accepted}
          disabled={busy}
          onChange={(event) => setAccepted(event.target.checked)}
        />
        {c.termsAccept}
      </label>
      <label>
        {c.suffixLabel}
        <input
          type="text"
          data-testid="card-suffix-input"
          value={suffix}
          disabled={busy}
          maxLength={22}
          aria-describedby="card-suffix-hint"
          onChange={(event) => setSuffix(event.target.value)}
        />
      </label>
      <p id="card-suffix-hint">
        {c.suffixHint} {c.suffixRule}
      </p>
      {invalid && (
        <p role="alert" data-testid="card-suffix-invalid">{c.suffixRule}</p>
      )}
      {tooLong && (
        <p role="alert" data-testid="card-suffix-too-long">{c.suffixTooLong}</p>
      )}
      {base !== null && (
        <p data-testid="card-preview">
          {c.previewLabel}: {descriptorPreview(base, value !== null && !invalid && !tooLong ? value : null)}
        </p>
      )}
      <div>
        <button type="button" data-testid="card-enable-confirm" disabled={!canConfirm} onClick={() => onConfirm(value)}>
          {busy ? c.saving : c.confirm}
        </button>
        <button type="button" data-testid="card-enable-cancel" disabled={busy} onClick={onCancel}>
          {c.cancel}
        </button>
      </div>
    </section>
  );
}
