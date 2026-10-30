// Purpose: Renders ads draft forms, details and status badges.
// Depends on: @live-commerce/format, @live-commerce/ui, @/lib/presentation-copy, react, @live-commerce/i18n, @/lib/ads-model, @/lib/ads-copy, @/lib/attribution-copy
// Used by: apps/admin/components/Ads.tsx
// Invariants: server validation and version checks remain authoritative; no external action on save.
"use client";
import { DateControl } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
// Extracted ads panel: existing BFF ads-client calls -> Go /v1/admin/stores/{store}/ads; no command or DTO changes.
import { useEffect, useState } from "react";
import { growthCopy, type Locale } from "@live-commerce/i18n";
import { decimal, instantToTaipei, taipeiToInstant } from "@live-commerce/format";
import {
  Badge as StatusBadge,
  Field,
  FormRow,
  TableFrame,
} from "@live-commerce/ui";
import {
  adsManagerHref,
  canApprove,
  canCopy,
  canEdit,
  canEnd,
  canPause,
  canPublish,
  formatMinor,
  hasRemote,
  type AdsCode,
  type Draft,
  type DraftForm as FormState,
  type Settings,
  type Template,
} from "@/lib/ads-model";
import { errorText, type AdsCopy } from "@/lib/ads-copy";
import { attributionCopy } from "@/lib/attribution-copy";

/** Describes Mode values shared by this presentation module. */
export type Mode =
  | { kind: "none" }
  | { kind: "new" | "copy"; form: FormState }
  | { kind: "edit"; form: FormState; draft: Draft };

/** Renders the server-reported ads draft status. */
export function Badge({ c, status }: { c: AdsCopy; status: Draft["status"] }) {
  const tone =
    status === "ACTIVE"
      ? "success"
      : status === "FAILED" || status === "REJECTED"
        ? "danger"
        : status === "UNKNOWN"
          ? "warning"
          : status === "SUBMITTING" ||
              status === "APPROVED" ||
              status === "REMOTE_PAUSED"
            ? "info"
            : "neutral";
  return (
    <StatusBadge tone={tone} data-state={status}>
      {c.statuses[status]}
    </StatusBadge>
  );
}

/** Renders one signed draft task at a time; the server still validates every field. */
export function DraftFormPanel({ c, locale, settings, mode, problems, busy, uncertain, onSubmit, onCancel, onRetry,
  sources = [], conflict = false, onKeep, onLatest,
}: {
  c: AdsCopy; locale: Locale; settings: Settings; mode: Exclude<Mode, { kind: "none" }>;
  problems: AdsCode[]; busy: boolean; uncertain: boolean;
  onSubmit: (form: FormState) => void; onCancel: () => void; onRetry: () => void;
  sources?: Draft[]; conflict?: boolean; onKeep?: () => void; onLatest?: () => void;
}) {
  const g = growthCopy[locale];
  const [form, setForm] = useState<FormState>(mode.form);
  const [pane, setPane] = useState<"editor" | "source" | "settings" | "leave">("editor");
  const [working, setWorking] = useState<FormState>(mode.form);
  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => setWorking(f => ({ ...f, [key]: value }));
  const accounts = settings.connections.filter(x => x.provider === "meta_ads" && x.enabled);
  const identities = settings.identities.filter(x => form.template === "PRODUCT_TRAFFIC" ? x.provider === "facebook" : x.provider === "facebook" || x.provider === "instagram");
  useEffect(() => {
    setForm(f => ({ ...f,
      ad_binding_id: accounts.some(a => a.binding_id === f.ad_binding_id) ? f.ad_binding_id : accounts.length === 1 ? accounts[0].binding_id : "",
      identity_binding_id: identities.some(a => a.binding_id === f.identity_binding_id) ? f.identity_binding_id : identities.length === 1 ? identities[0].binding_id : "",
    }));
  }, [form.template]); // eslint-disable-line react-hooks/exhaustive-deps
  function chooseDays(days: number) {
    const parsed = taipeiToInstant(working.starts_local);
    const starts = parsed && Number.isFinite(Date.parse(parsed)) ? Date.parse(parsed) : Date.now() + 3600_000;
    setWorking(f => ({ ...f, starts_local: instantToTaipei(new Date(starts).toISOString()), ends_local: instantToTaipei(new Date(starts + days * 86400_000).toISOString()) }));
  }
  const openSettings = () => { setWorking({ ...form }); setPane("settings"); };
  const budget = form.budget ? (Number.isFinite(Number(form.budget)) ? decimal(locale, Number(form.budget), form.currency === "TWD" ? 0 : 2) : form.budget) : "—";
  const title = conflict ? g.conflict : pane === "source" ? g.sourceTitle : pane === "settings" ? g.settings : pane === "leave" ? g.leaveTitle : mode.kind === "edit" ? g.edit : g.create;
  const candidates = sources.filter((d, i, all) => all.findIndex(x => x.source_ref === d.source_ref && x.template === d.template) === i);
  return <div className="growth-form" data-testid="ads-form">
    <h1>{title}</h1>
    <p className="growth-intro">{conflict ? g.conflictHint : g.sequence}</p>
    {conflict ? <>
      <div className="growth-card"><p>{g.notSaved}</p></div>
      <div className="growth-dock"><button type="button" onClick={onKeep}>{g.keepChanges}</button><button type="button" className="primary" onClick={onLatest}>{g.latest}</button></div>
    </> : pane === "leave" ? <>
      <div className="growth-card"><p>{g.notSaved}</p><p>{g.separation}</p></div>
      <div className="growth-dock"><button type="button" onClick={() => setPane("editor")}>{g.keepEditing}</button><button type="button" onClick={onCancel}>{g.discard}</button></div>
    </> : pane === "source" ? <>
      <div className="growth-columns"><div className="growth-stack">
        <section className="growth-card"><h2>{g.savedSources}</h2>
          {candidates.map(d => <div className="growth-card" key={d.id} data-source-ref={d.source_ref}><p>{c.templates[d.template]}</p><p>{d.source_ref}</p><button type="button" onClick={() => { setForm(f => ({ ...f, source_ref: d.source_ref, template: d.template, ad_binding_id: d.ad_binding_id, identity_binding_id: d.identity_binding_id })); setPane("editor"); }}>{g.useSource}</button></div>)}
          {!candidates.length && <p>{g.sourceGap}</p>}
        </section>
        <details className="growth-card" data-testid="ads-source-advanced"><summary>{g.advanced}</summary>
          <div role="radiogroup" aria-label={c.fTemplate}>{(["BOOST_POST", "PRODUCT_TRAFFIC"] as Template[]).map(t => <label key={t}><input type="radio" name="template" value={t} checked={form.template === t} data-testid={`ads-template-${t}`} onChange={() => setForm(f => ({ ...f, template: t }))} />{c.templates[t]}</label>)}</div>
          <label>{c.fSource}<input value={form.source_ref} maxLength={128} autoComplete="off" data-testid="ads-f-source" onChange={e => setForm(f => ({ ...f, source_ref: e.target.value }))} /></label>
        </details>
      </div><aside className="growth-note"><h2>{g.content}</h2><p>{g.separation}</p></aside></div>
      <div className="growth-dock"><button type="button" onClick={() => setPane("editor")}>{g.keepSource}</button></div>
    </> : pane === "settings" ? <form noValidate onSubmit={e => { e.preventDefault(); setForm(working); setPane("editor"); }}>
      <fieldset disabled={busy || uncertain}><div className="growth-columns"><div className="growth-stack">
        <section className="growth-card"><label>{c.fBudget(working.currency)}<input value={working.budget} inputMode="decimal" autoComplete="off" maxLength={14} data-testid="ads-f-budget" onChange={e => set("budget", e.target.value)} /></label><small>{c.fBudgetHint(working.currency)}</small><p>{g.budgetHint}</p><small>{g.quickAmounts}</small><div className="growth-actions">{([["1500", 150000], ["3000", 300000]] as const).map(([value, minor]) => <button type="button" key={value} onClick={() => set("budget", value)}>{formatMinor(locale, working.currency, minor)}</button>)}</div></section>
        <section className="growth-card"><div className="growth-row"><label>{c.fStarts}<DateControl emptyLabel={presentationCopy[locale].dateTime} lang={locale} type="datetime-local" value={working.starts_local} data-testid="ads-f-starts" onChange={e => set("starts_local", e.target.value)} /></label><label>{c.fEnds}<DateControl emptyLabel={presentationCopy[locale].dateTime} lang={locale} type="datetime-local" value={working.ends_local} data-testid="ads-f-ends" onChange={e => set("ends_local", e.target.value)} /></label></div><small>{c.fDatesHint}</small><div className="growth-actions"><button type="button" onClick={() => chooseDays(3)}>{g.threeDays}</button><button type="button" onClick={() => chooseDays(7)}>{g.sevenDays}</button></div></section>
      </div><div className="growth-stack"><section className="growth-card">
        <label>{c.fCountries}<input value={working.countries} data-testid="ads-f-countries" onChange={e => set("countries", e.target.value)} /></label>
        <div className="growth-actions"><button type="button" onClick={() => set("countries", "TW")}>{g.taiwan}</button><button type="button" onClick={() => set("countries", "HK")}>{g.hongKong}</button></div>
        <div className="growth-row"><label>{c.fAgeMin}<input value={working.age_min} type="number" min={18} max={65} data-testid="ads-f-agemin" onChange={e => set("age_min", e.target.value)} /></label><label>{c.fAgeMax}<input value={working.age_max} type="number" min={18} max={65} data-testid="ads-f-agemax" onChange={e => set("age_max", e.target.value)} /></label></div>
        <div className="growth-actions"><button type="button" onClick={() => setWorking(f => ({ ...f, age_min: "18", age_max: "65" }))}>{g.agesDefault}</button><button type="button" onClick={() => setWorking(f => ({ ...f, age_min: "25", age_max: "44" }))}>{g.agesFocused}</button></div>
        <label>{c.fAdAccount}<select value={working.ad_binding_id} data-testid="ads-f-account" onChange={e => set("ad_binding_id", e.target.value)}><option value="">—</option>{accounts.map(a => <option key={a.binding_id} value={a.binding_id}>{a.asset_id}</option>)}</select></label>
        <label>{c.fIdentity}<select value={working.identity_binding_id} data-testid="ads-f-identity" onChange={e => set("identity_binding_id", e.target.value)}><option value="">—</option>{identities.map(a => <option key={a.binding_id} value={a.binding_id}>{a.asset_id}</option>)}</select></label>
      </section><aside className="growth-note"><h2>{g.automatic}</h2><p>{c.fPlacement}</p></aside></div></div>
      <div className="growth-dock"><button type="button" data-testid="ads-f-cancel" onClick={() => setPane("editor")}>{g.cancel}</button><button type="submit" className="primary">{g.apply}</button></div></fieldset>
    </form> : <form noValidate onSubmit={e => { e.preventDefault(); onSubmit(form); }}>
      <fieldset disabled={busy || uncertain}><p className="ads-note">{mode.kind === "edit" ? mode.draft.id : g.notSaved}</p>
        <div className="growth-columns"><div className="growth-stack">
          <section className="growth-card"><h2>{g.sourceStep}</h2><p>{c.templates[form.template]}</p><h3>{form.source_ref || g.noSource}</h3><button type="button" onClick={() => setPane("source")}>{g.source}</button></section>
          <section className="growth-card"><h2>{g.budgetStep}</h2><div className="growth-row"><p>{form.currency} {budget}</p><p>{form.starts_local || "—"} – {form.ends_local || "—"}</p></div><small>{g.budgetHint}</small><div className="growth-actions"><button type="button" onClick={openSettings}>{g.settings}</button></div></section>
          <section className="growth-card"><h2>{g.audienceStep}</h2><p>{form.countries} · {form.age_min}–{form.age_max}</p><p>{settings.identities.find(i => i.binding_id === form.identity_binding_id)?.asset_id || "—"}</p><small>{c.fPlacement}</small></section>
        </div><aside><div className="growth-note"><h2>{g.separate}</h2><p>{g.separation}</p></div><div className="growth-note growth-warning"><h2>{g.budgetWarning}</h2><p>{g.budgetHint}</p></div></aside></div>
        {problems.length > 0 && <div role="alert" data-testid="ads-form-problems"><p>{c.fixFields}</p>{problems.map(p => <p key={p}>{errorText(c, p)}</p>)}</div>}
        <div className="growth-dock"><button type="button" data-testid="ads-f-cancel" onClick={() => setPane("leave")}>{g.leave}</button><button type="submit" className="primary" data-testid="ads-f-save">{c.save}</button></div>
      </fieldset>
      {uncertain && <button type="button" onClick={onRetry}>{c.retrySame}</button>}
    </form>}
  </div>;
}

/** Renders a loaded ads draft and delegates action requests to callbacks. */
export function DraftDetail({
  c,
  locale,
  d,
  busy,
  allowanceOff,
  confirmEnd,
  setConfirmEnd,
  onEdit,
  onCopy,
  act,
  formOpen,
  when, identity, account,
}: {
  when: (locale: Locale, iso: string | null, empty: string) => string;
  identity?: string; account?: string;
  c: AdsCopy;
  locale: Locale;
  d: Draft;
  busy: string;
  allowanceOff: boolean;
  confirmEnd: boolean;
  formOpen: boolean;
  setConfirmEnd: (v: boolean) => void;
  onEdit: () => void;
  onCopy: () => void;
  act: (d: Draft, a: "approve" | "publish" | "pause" | "end") => void;
}) {
  const link = d.ads_manager_url ? adsManagerHref(d.ads_manager_url) : null;
  const working = busy !== "";
  const remote: [string, string | null][] = [
    [c.remoteCampaign, d.remote.campaign_id],
    [c.remoteAdset, d.remote.adset_id],
    [c.remoteCreative, d.remote.creative_id],
    [c.remoteAd, d.remote.ad_id],
  ];
  return (
    <section
      className="ads-detail"
      aria-label={`${c.detailTitle} ${d.id}`}
      data-testid="ads-detail"
      data-status={d.status}
    >
      <div className="growth-card">
        <p>{c.templates[d.template]}</p>
        <dl className="growth-facts"><div><dt>{c.detailTitle}</dt><dd>{d.id}</dd></div><div><dt>{c.source}</dt><dd>{d.source_ref}</dd></div><div><dt>{c.budget}</dt><dd>{formatMinor(locale, d.currency, d.lifetime_budget_minor)}</dd></div><div><dt>{c.starts}</dt><dd>{when(locale, d.starts_at, "—")}</dd></div><div><dt>{c.ends}</dt><dd>{when(locale, d.ends_at, "—")}</dd></div><div><dt>{c.countries}</dt><dd>{d.countries.join(", ")}</dd></div><div><dt>{c.ages}</dt><dd>{d.age_min}–{d.age_max}</dd></div><div><dt>{c.fIdentity}</dt><dd>{identity || "—"}</dd></div><div><dt>{c.fAdAccount}</dt><dd>{account || "—"}</dd></div><div><dt>{c.revision}</dt><dd>{d.revision}</dd></div></dl>
        <small>{c.reportTimezone}: Asia/Taipei · {c.fPlacement}</small>
        <Badge c={c} status={d.status} />
        {canEdit(d) && <button type="button" data-testid="ads-edit" disabled={working || formOpen} onClick={onEdit}>{c.edit}</button>}
      </div>
      <details data-testid="ads-advanced"><summary>{growthCopy[locale].advanced}</summary>
      <div className="ads-detail-head">
        <h3>
          {c.templates[d.template]} <Badge c={c} status={d.status} />
        </h3>
        {link && (
          <a
            href={link}
            target="_blank"
            rel="noopener noreferrer"
            data-testid="ads-manager-link"
          >
            {c.adsManager}
          </a>
        )}
      </div>
      {d.status === "UNKNOWN" && (
        <p className="ads-bad" role="status" data-testid="ads-unknown-help">
          {c.checkAdsManager}
        </p>
      )}
      <dl className="ads-facts">
        <div>
          <dt>{c.budget}</dt>
          <dd>{formatMinor(locale, d.currency, d.lifetime_budget_minor)}</dd>
        </div>
        <div>
          <dt>{c.starts}</dt>
          <dd>{when(locale, d.starts_at, "—")}</dd>
        </div>
        <div>
          <dt>{c.ends}</dt>
          <dd>{when(locale, d.ends_at, "—")}</dd>
        </div>
        <div>
          <dt>{c.countries}</dt>
          <dd>{d.countries.join(", ")}</dd>
        </div>
        <div>
          <dt>{c.ages}</dt>
          <dd>
            {d.age_min}–{d.age_max}
          </dd>
        </div>
        <div>
          <dt>{c.source}</dt>
          <dd className="ads-mono">{d.source_ref}</dd>
        </div>
        <div>
          <dt>{c.revision}</dt>
          <dd>{d.revision}</dd>
        </div>
        <div>
          <dt>{c.attempt}</dt>
          <dd>{d.publish_attempt}</dd>
        </div>
      </dl>
      <div className="ads-actions" data-testid="ads-actions">
        {canEdit(d) && (
          <button
            type="button"
            onClick={onEdit}
            disabled={working || formOpen}
            data-testid="ads-edit-advanced"
          >
            {c.edit}
          </button>
        )}
        {canApprove(d) && (
          <button
            type="button"
            className="primary"
            onClick={() => act(d, "approve")}
            disabled={working || allowanceOff}
            data-testid="ads-approve"
          >
            {busy === `approve:${d.id}` ? c.sending : c.approve}
          </button>
        )}
        {canPublish(d) && (
          <button
            type="button"
            className="primary"
            onClick={() => act(d, "publish")}
            disabled={working || allowanceOff}
            data-testid="ads-publish"
          >
            {busy === `publish:${d.id}`
              ? c.sending
              : d.status === "FAILED"
                ? c.republish
                : c.publish}
          </button>
        )}
        {canPause(d) && (
          <button
            type="button"
            onClick={() => act(d, "pause")}
            disabled={working}
            data-testid="ads-pause"
          >
            {busy === `pause:${d.id}` ? c.sending : c.pause}
          </button>
        )}
        {canEnd(d) && !confirmEnd && (
          <button
            type="button"
            onClick={() => setConfirmEnd(true)}
            disabled={working}
            data-testid="ads-end"
          >
            {c.end}
          </button>
        )}
        {canCopy(d) && (
          <button
            type="button"
            onClick={onCopy}
            disabled={working || formOpen}
            data-testid="ads-copy"
          >
            {c.copy}
          </button>
        )}
      </div>
      {allowanceOff && canApprove(d) && (
        <p className="ads-hint">{c.approveOffHint}</p>
      )}
      {canCopy(d) && <p className="ads-hint">{c.copyHint}</p>}
      {confirmEnd && (
        <div
          className="ads-confirm"
          role="alertdialog"
          aria-labelledby="ads-end-h"
          data-testid="ads-end-confirm"
        >
          <h4 id="ads-end-h">{c.endConfirmTitle}</h4>
          <p>{c.endConfirmBody}</p>
          <div className="ads-actions">
            <button
              type="button"
              className="ads-danger"
              onClick={() => act(d, "end")}
              disabled={working}
              data-testid="ads-end-yes"
            >
              {c.endConfirm}
            </button>
            <button type="button" onClick={() => setConfirmEnd(false)}>
              {c.cancel}
            </button>
          </div>
        </div>
      )}
      <h4>{c.remote}</h4>
      {hasRemote(d) ? (
        <dl className="ads-facts" data-testid="ads-remote">
          {remote
            .filter(([, v]) => v)
            .map(([label, v]) => (
              <div key={label}>
                <dt>{label}</dt>
                <dd className="ads-mono">{v}</dd>
              </div>
            ))}
        </dl>
      ) : (
        <p className="ads-empty">{c.remoteNone}</p>
      )}
      <h4>{c.opsTitle}</h4>
      {d.ops.length === 0 ? (
        <p className="ads-empty">{c.opsEmpty}</p>
      ) : (
        <TableFrame
          className="ads-table-scroll"
          label={c.opsTitle}
          scrollHint={attributionCopy[locale].scrollHint}
        >
          <table className="ads-table ads-ops" data-testid="ads-ops">
            <thead>
              <tr>
                <th>{c.opKind}</th>
                <th className="ads-number">{c.opAttempt}</th>
                <th>{c.opState}</th>
                <th>{c.opUpdated}</th>
              </tr>
            </thead>
            <tbody>
              {d.ops.map((o) => (
                <tr key={`${o.kind}:${o.attempt}:${o.seq}`}>
                  <td data-label={c.opKind}>
                    <span className="ads-mono">{o.kind}</span>
                    {o.seq > 1 ? ` #${o.seq}` : ""}
                  </td>
                  <td className="ads-number" data-label={c.opAttempt}>
                    {o.attempt}
                  </td>
                  <td data-label={c.opState}>
                    {c.opStates[o.state]}
                    {o.code ? (
                      <small className="ads-mono"> {o.code}</small>
                    ) : null}
                    {o.error_user_msg && (
                      <p
                        className="ads-meta-message"
                        data-testid="ads-meta-message"
                      >
                        {o.error_user_msg}
                      </p>
                    )}
                  </td>
                  <td data-label={c.opUpdated}>
                    {when(locale, o.updated_at, "—")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableFrame>
      )}
      </details>
    </section>
  );
}
