// Purpose: W3-U2 manual reminders, sold-out settings and restricted buyers within the existing W0 ledger layout.
// Depends on: live-settings controller/client -> scoped Go routes, shared privacy fences and W0 presentation tokens.
// Used by: /[locale]/studio/settings and BuyerPanel; private fields remain in visible component memory only.
"use client";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import type { StudioErrorCode } from "@/lib/studio-client";
import { liveSettingsError } from "@/lib/live-settings-copy";
import {
  merchantTemplate,
  settingsUUID,
  validSoldOutText,
} from "@/lib/live-settings-model";
import { useLiveSettingsController } from "@/lib/live-settings-controller";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "@/src/features/live/workspace.css";
/** Render scoped operator controls; the controller fences reads and preserves unknown receipts. */
export function LiveSettings({
  locale,
  store,
  scene,
  initialError,
}: {
  locale: Locale;
  store: Store | null;
  scene: string;
  initialError: StudioErrorCode | null;
}) {
  const {
    c,
    router,
    storeID,
    sessions,
    settings,
    enabled,
    setEnabled,
    draft,
    setDraft,
    report,
    result,
    blocked,
    history,
    remove,
    setRemove,
    busy,
    loading,
    error,
    notice,
    unknown,
    receipt,
    canManage,
    canReport,
    canRemind,
    privacy,
    setRevision,
    run,
    page,
    save,
    copy,
    unavailable,
    locked,
  } = useLiveSettingsController({ locale, store, scene, initialError });
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? ""}
      active="live"
      onBeforeNavigate={() => {
        privacy.suspend();
        return true;
      }}
    >
      <section
        className="live-workspace"
        data-testid="live-settings"
        style={{ overflowWrap: "anywhere" }}
        aria-busy={loading || busy}
      >
        <AdminPageHeader locale={locale} title={c.title} />
        {unavailable ? (
          <p role="alert">{initialError ? c.denied : c.noRead}</p>
        ) : !privacy.visible ? (
          <p role="status">{privacy.blocked.current ? c.denied : c.hidden}</p>
        ) : (
          <>
            <div className="live-workspace-toolbar">
              <label>
                {c.choose}
                <select
                  data-testid="scene-select"
                  value={scene}
                  disabled={locked || !sessions}
                  onChange={(event) => {
                    const id = event.target.value;
                    if (
                      id !== scene &&
                      settingsUUID.test(id) &&
                      sessions?.items.some((x) => x.session_id === id)
                    ) {
                      privacy.suspend();
                      router.push(
                        `/${locale}/studio/settings?store=${storeID}&scene=${id}`,
                      );
                    }
                  }}
                >
                  <option value="">{c.choose}</option>
                  {sessions?.items.map((item) => (
                    <option key={item.session_id} value={item.session_id}>
                      {item.title}
                    </option>
                  ))}
                </select>
              </label>
              <button
                type="button"
                data-testid="live-settings-refresh"
                disabled={locked}
                onClick={() => setRevision((v) => v + 1)}
              >
                {c.refresh}
              </button>
            </div>
            {loading && <p role="status">{c.loading}</p>}
            {error && (
              <p
                role="alert"
                data-testid={
                  [
                    "conflict",
                    "version_conflict",
                    "idempotency_conflict",
                  ].includes(error)
                    ? "sold-out-conflict"
                    : undefined
                }
              >
                {liveSettingsError(locale, error)}
              </p>
            )}
            {notice && <p role="status">{notice}</p>}
            {unknown && (
              <div role="alert" data-testid="live-settings-unknown">
                <p>{c.unknown}</p>
                <button
                  type="button"
                  data-testid="live-settings-retry"
                  disabled={busy}
                  onClick={() => {
                    if (receipt.current) void run(receipt.current);
                  }}
                >
                  {c.retry}
                </button>
              </div>
            )}
            <section className="live-offer" aria-labelledby="reminder-heading">
              <h2 id="reminder-heading">{c.manual}</h2>
              <p>{c.rule}</p>
              <button
                type="button"
                className="primary"
                data-testid="reminder-trigger"
                disabled={locked || !scene || !canRemind}
                onClick={() =>
                  void run({
                    key: crypto.randomUUID(),
                    method: "POST",
                    resource: `live-sessions/${scene}/reminders`,
                  })
                }
              >
                {busy ? c.saving : c.trigger}
              </button>
              {!scene && <p>{c.chooseFirst}</p>}
              {!canRemind && <p>{c.noReply}</p>}
              {canReport ? (
                report && (
                  <div data-testid="reminder-report">
                    <p role="status">
                      {c.queued} {report.queued} · {c.followup}{" "}
                      {result?.followup ??
                        report.followup.filter((x) => x.reason !== "restricted")
                          .length}{" "}
                      · {c.restricted}{" "}
                      {result?.restricted ??
                        report.followup.filter((x) => x.reason === "restricted")
                          .length}{" "}
                      · {c.sent} {report.sent.length} · {c.failed}{" "}
                      {report.failed.length}
                    </p>
                    {result && (
                      <p role="status">
                        {c.result}: {c.queued} {result.queued} · {c.already}{" "}
                        {result.already_reminded} · {c.failed} {result.refused}
                      </p>
                    )}
                    {result?.truncated && <p role="status">{c.more}</p>}
                    {report.followup.length ? (
                      <ul className="comment-rows">
                        {report.followup.map((row) => (
                          <li
                            key={row.bundle_id}
                            data-testid="followup-row"
                            data-bundle-id={row.bundle_id}
                          >
                            <strong>{row.display_name ?? row.bundle_id}</strong>
                            <p>
                              {row.reason === "restricted"
                                ? c.restricted
                                : (c[row.reason as "window_closed"] ??
                                  c.followup)}
                            </p>
                            {row.reason !== "restricted" &&
                              row.link_copy_allowed &&
                              report.link && (
                                <button
                                  type="button"
                                  data-testid="copy"
                                  disabled={locked}
                                  onClick={() => void copy(report.link!)}
                                >
                                  {c.copy}
                                </button>
                              )}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p>{c.empty}</p>
                    )}
                  </div>
                )
              ) : (
                <p>{c.noRead}</p>
              )}
            </section>
            <section
              className="live-offer comment-reply"
              aria-labelledby="soldout-heading"
            >
              <h2 id="soldout-heading">{c.soldout}</h2>
              {settings && (
                <p className="live-helper">
                  {c.currentTemplate}: {settings.template_id} · v
                  {settings.template_version}
                </p>
              )}
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  save();
                }}
              >
                <fieldset disabled={locked || !canManage || !settings}>
                  <label className="live-offer-toggle">
                    <input
                      type="checkbox"
                      role="switch"
                      data-testid="sold-out-enabled"
                      checked={enabled}
                      onChange={(e) => setEnabled(e.target.checked)}
                    />
                    {c.enabled}
                  </label>
                  <label>
                    {c.template}
                    <textarea
                      data-testid="sold-out-template"
                      value={draft}
                      maxLength={560}
                      placeholder={c.placeholder}
                      onChange={(e) => setDraft(e.target.value)}
                      aria-describedby="template-help"
                    />
                  </label>
                  <p id="template-help" className="live-helper">
                    {c.draftHint}
                  </p>
                  <div data-testid="sold-out-preview">
                    <h3>{c.preview}</h3>
                    <p>
                      {draft
                        ? draft.replace("{{product.name}}", c.product)
                        : c.placeholder.replace("{{product.name}}", c.product)}
                    </p>
                    <p>{c.quota}</p>
                  </div>
                  {settings &&
                    !merchantTemplate(settings.template_id) &&
                    !draft && <p>{c.templateRequired}</p>}
                  <button
                    type="submit"
                    className="primary"
                    data-testid="sold-out-save"
                    disabled={
                      !settings ||
                      (!draft && !merchantTemplate(settings.template_id)) ||
                      (!!draft && !validSoldOutText(draft))
                    }
                  >
                    {busy ? c.saving : c.save}
                  </button>
                </fieldset>
              </form>
              {!canManage && <p>{c.noManage}</p>}
            </section>
            <section className="live-offer" aria-labelledby="blocklist-heading">
              <h2 id="blocklist-heading">{c.list}</h2>
              <p>{c.blockHint}</p>
              {!scene ? (
                <p>{c.chooseFirst}</p>
              ) : (
                blocked && (
                  <>
                    {blocked.items.length ? (
                      <ul className="comment-rows">
                        {blocked.items.map((row) => (
                          <li
                            key={row.id}
                            data-testid="blocklist-row"
                            data-entry-id={row.id}
                          >
                            <dl>
                              <dt>{c.platform}</dt>
                              <dd>{row.platform}</dd>
                              <dt>{c.note}</dt>
                              <dd>{row.note ?? "—"}</dd>
                              <dt>{c.source}</dt>
                              <dd>{row.source_bundle_id ?? "—"}</dd>
                              <dt>{c.added}</dt>
                              <dd>{displayTime(locale, row.created_at)}</dd>
                            </dl>
                            {remove === row.id ? (
                              <div
                                role="alertdialog"
                                aria-label={c.removeConfirm}
                              >
                                <p>{c.removeConfirm}</p>
                                <button
                                  type="button"
                                  data-testid="blocklist-confirm"
                                  disabled={locked}
                                  onClick={() =>
                                    void run({
                                      key: crypto.randomUUID(),
                                      method: "DELETE",
                                      resource: `live-sessions/${scene}/claims/blocklist/entries/${row.id}`,
                                    })
                                  }
                                >
                                  {c.remove}
                                </button>
                                <button
                                  type="button"
                                  data-testid="blocklist-cancel"
                                  disabled={locked}
                                  onClick={() => setRemove(null)}
                                >
                                  {c.cancel}
                                </button>
                              </div>
                            ) : (
                              <button
                                type="button"
                                data-testid="blocklist-remove"
                                disabled={locked || !canManage}
                                onClick={() => setRemove(row.id)}
                              >
                                {c.remove}
                              </button>
                            )}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p>{c.empty}</p>
                    )}
                    <div className="live-workspace-toolbar">
                      <button
                        type="button"
                        data-testid="blocklist-previous"
                        disabled={locked || !history.length}
                        onClick={() => void page(history.at(-1) ?? "", true)}
                      >
                        {c.previous}
                      </button>
                      <button
                        type="button"
                        data-testid="blocklist-next"
                        disabled={locked || !blocked.next_cursor}
                        onClick={() => void page(blocked.next_cursor)}
                      >
                        {c.next}
                      </button>
                    </div>
                  </>
                )
              )}
            </section>
          </>
        )}
      </section>
    </WorkspaceFrame>
  );
}
