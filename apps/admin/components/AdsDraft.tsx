// Purpose: Renders ads draft forms, details and status badges.
// Depends on: @live-commerce/ui, @/lib/presentation-copy, react, @live-commerce/i18n, @/lib/ads-model, @/lib/ads-copy, @/lib/attribution-copy
// Used by: apps/admin/components/Ads.tsx
"use client";
import { DateControl } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
// Extracted ads panel: existing BFF ads-client calls -> Go /v1/admin/stores/{store}/ads; no command or DTO changes.
import { useEffect, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
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

/** Renders draft fields and delegates edits to the supplied callbacks. */
export function DraftFormPanel({
  c,
  locale,
  settings,
  mode,
  problems,
  busy,
  uncertain,
  onSubmit,
  onCancel,
  onRetry,
}: {
  c: AdsCopy;
  locale: Locale;
  settings: Settings;
  mode: Exclude<Mode, { kind: "none" }>;
  problems: AdsCode[];
  busy: boolean;
  uncertain: boolean;
  onSubmit: (form: FormState) => void;
  onCancel: () => void;
  onRetry: () => void;
}) {
  const [form, setForm] = useState<FormState>(mode.form);
  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((f) => ({ ...f, [key]: value }));
  const accounts = settings.connections.filter(
    (x) => x.provider === "meta_ads" && x.enabled,
  );
  const identities = settings.identities.filter((x) =>
    form.template === "PRODUCT_TRAFFIC"
      ? x.provider === "facebook"
      : x.provider === "facebook" || x.provider === "instagram",
  );
  useEffect(() => {
    // Preselect the only choice; drop a choice the template no longer allows.
    setForm((f) => {
      const ad = accounts.some((a) => a.binding_id === f.ad_binding_id)
        ? f.ad_binding_id
        : accounts.length === 1
          ? accounts[0].binding_id
          : "";
      const id = identities.some((a) => a.binding_id === f.identity_binding_id)
        ? f.identity_binding_id
        : identities.length === 1
          ? identities[0].binding_id
          : "";
      return ad === f.ad_binding_id && id === f.identity_binding_id
        ? f
        : { ...f, ad_binding_id: ad, identity_binding_id: id };
    });
  }, [form.template]); // eslint-disable-line react-hooks/exhaustive-deps
  const title =
    mode.kind === "edit"
      ? c.formEdit
      : mode.kind === "copy"
        ? c.formCopy
        : c.formNew;
  const boost = form.template === "BOOST_POST";
  return (
    <form
      className="ads-form ads-draft-form"
      data-testid="ads-form"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(form);
      }}
      noValidate
    >
      <fieldset disabled={busy || uncertain}>
        <legend>{title}</legend>
        {mode.kind === "copy" && <p className="ads-note">{c.copyHint}</p>}
        <div
          role="radiogroup"
          aria-label={c.fTemplate}
          className="ads-radios ads-radios-row"
        >
          {(["BOOST_POST", "PRODUCT_TRAFFIC"] as Template[]).map((t) => (
            <label key={t} className="ads-radio">
              <input
                type="radio"
                name="template"
                value={t}
                checked={form.template === t}
                data-testid={`ads-template-${t}`}
                onChange={() => set("template", t)}
              />
              <span>{c.templates[t]}</span>
            </label>
          ))}
        </div>
        <label>
          {c.fAdAccount}
          <select
            value={form.ad_binding_id}
            onChange={(e) => set("ad_binding_id", e.target.value)}
            data-testid="ads-f-account"
          >
            <option value="">—</option>
            {accounts.map((a) => (
              <option key={a.binding_id} value={a.binding_id}>
                {a.asset_id}
              </option>
            ))}
          </select>
          {accounts.length === 0 && (
            <small className="ads-bad">{c.fNoAccount}</small>
          )}
        </label>
        <label>
          {boost ? c.fIdentity : c.fIdentityFacebookOnly}
          <select
            value={form.identity_binding_id}
            onChange={(e) => set("identity_binding_id", e.target.value)}
            data-testid="ads-f-identity"
          >
            <option value="">—</option>
            {identities.map((a) => (
              <option key={a.binding_id} value={a.binding_id}>
                {c.providers[a.provider] ?? a.provider} · {a.asset_id}
              </option>
            ))}
          </select>
          {identities.length === 0 && (
            <small className="ads-bad">{c.fNoIdentity}</small>
          )}
        </label>
        <label>
          {c.fSource}
          <input
            value={form.source_ref}
            onChange={(e) => set("source_ref", e.target.value)}
            maxLength={128}
            autoComplete="off"
            inputMode="text"
            data-testid="ads-f-source"
          />
          <small>{boost ? c.fSourceBoost : c.fSourceProduct}</small>
        </label>
        <label>
          {c.fBudget(form.currency)}
          <input
            value={form.budget}
            onChange={(e) => set("budget", e.target.value)}
            inputMode="decimal"
            autoComplete="off"
            maxLength={14}
            data-testid="ads-f-budget"
          />
          <small>{c.fBudgetHint(form.currency)}</small>
        </label>
        <FormRow>
          <Field id="ads-f-starts" label={c.fStarts}>
            <DateControl emptyLabel={presentationCopy[locale].dateTime}
              id="ads-f-starts"
              lang={locale}
              type="datetime-local"
              value={form.starts_local}
              onChange={(e) => set("starts_local", e.target.value)}
              data-testid="ads-f-starts"
            />
          </Field>
          <Field id="ads-f-ends" label={c.fEnds}>
            <DateControl emptyLabel={presentationCopy[locale].dateTime}
              id="ads-f-ends"
              lang={locale}
              type="datetime-local"
              value={form.ends_local}
              onChange={(e) => set("ends_local", e.target.value)}
              data-testid="ads-f-ends"
            />
          </Field>
        </FormRow>
        <small className="ads-hint">{c.fDatesHint}</small>
        <label>
          {c.fCountries}
          <input
            value={form.countries}
            onChange={(e) => set("countries", e.target.value)}
            maxLength={80}
            autoComplete="off"
            data-testid="ads-f-countries"
          />
          <small>{c.fCountriesHint}</small>
        </label>
        <div className="ads-grid2">
          <label>
            {c.fAgeMin}
            <input
              value={form.age_min}
              onChange={(e) => set("age_min", e.target.value)}
              inputMode="numeric"
              maxLength={2}
              data-testid="ads-f-agemin"
            />
          </label>
          <label>
            {c.fAgeMax}
            <input
              value={form.age_max}
              onChange={(e) => set("age_max", e.target.value)}
              inputMode="numeric"
              maxLength={2}
              data-testid="ads-f-agemax"
            />
          </label>
        </div>
        <small className="ads-hint">{c.fPlacement}</small>
        {problems.length > 0 && (
          <div className="ads-bad" role="alert" data-testid="ads-form-problems">
            <p>{c.fixFields}</p>
            <ul>
              {problems.map((p) => (
                <li key={p}>{errorText(c, p)}</li>
              ))}
            </ul>
          </div>
        )}
        <div className="ads-actions">
          <button type="submit" className="primary" data-testid="ads-f-save">
            {busy ? c.saving : c.save}
          </button>
          <button type="button" onClick={onCancel} data-testid="ads-f-cancel">
            {c.cancel}
          </button>
        </div>
      </fieldset>
      {uncertain && (
        <p className="ads-actions">
          <button type="button" onClick={onRetry}>
            {c.retrySame}
          </button>
        </p>
      )}
    </form>
  );
}

// ---------- draft detail ----------
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
  when,
}: {
  when: (locale: Locale, iso: string | null, empty: string) => string;
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
            data-testid="ads-edit"
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
    </section>
  );
}
