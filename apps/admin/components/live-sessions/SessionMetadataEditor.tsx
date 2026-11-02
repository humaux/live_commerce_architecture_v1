// Purpose: Renders the controlled Studio metadata editor and its same-session departure confirmation.
// Depends on: React types, shared DS fields/dialog, locale copy and read-only Studio DTO types; no tables/API writes.
// Used by: Studio.tsx; the parent exclusively owns form, save/CAS, request and recovery state.
// Invariants: preserves the original fields/disabled rules; every exit and confirmation delegates to the parent's current guards.
"use client";
import type { ReactNode } from "react";
import { liveSessionsCopy, type Locale } from "@live-commerce/i18n";
import { ConfirmDialog, DateControl, Field, FormRow } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import { studioCopy } from "@/lib/studio-copy";
import type { AspectRatio, StudioDetail } from "@/lib/studio-model";
import type { StudioErrorCode } from "@/lib/studio-client";
import { displayTime } from "@/lib/orders-model";
import s from "./session-tasks.module.css";

export type MetadataForm = { id: string; title: string; scheduled: string; aspect: AspectRatio };
export function SessionMetadataEditor({ locale, c, form, shown, canEdit, busy, newMode, recoveryBlocked,
  formDirty, formError, actionError, detailMessage, showExit, confirmExit, onChange, onSave, onLeave,
  onKeep, onDiscard, onFieldFocus, children }: {
  locale: Locale; c: (typeof studioCopy)[Locale]; form: MetadataForm; shown: StudioDetail | null;
  canEdit: boolean; busy: boolean; newMode: boolean; recoveryBlocked: boolean; formDirty: boolean;
  formError: string; actionError: StudioErrorCode | null; detailMessage: string; showExit: boolean; confirmExit: boolean;
  onChange: (form: MetadataForm, clearError: boolean) => void; onSave: () => void; onLeave: () => void;
  onKeep: () => void; onDiscard: () => void; onFieldFocus: (field: HTMLElement) => void; children: ReactNode;
}) {
  const p = liveSessionsCopy[locale], unavailable = p.unavailable;
  return <section className="studio-editor" aria-label={c.editor} onFocusCapture={(event) => {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement) onFieldFocus(event.target);
    }}>
      {showExit && <div className={s.row}><button type="button" onClick={onLeave}>‹ {p.prepare}</button></div>}
      <h2>{c.editor}</h2><p className="studio-muted">{c.editorHint}</p>
          {newMode || shown ? <>
            <FormRow className="studio-fields">
              <Field id="studio-name" label={c.name}><input id="studio-name" value={form.title} maxLength={400} disabled={!canEdit || busy}
                onChange={(event) => onChange({ ...form, title: event.target.value }, true)} /></Field>
              {locale === "en" ? <Field id="studio-schedule-entry" label={c.schedule}>
                <div className="studio-schedule-control">
                  <input id="studio-schedule-entry" type="text" placeholder="YYYY-MM-DDTHH:mm" maxLength={16}
                    value={form.scheduled} disabled={!canEdit || busy}
                    onChange={(event) => onChange({ ...form, scheduled: event.target.value }, true)} />
                  <span className="studio-schedule-picker">
                    <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M7 3v4M17 3v4M3 10h18" /></svg>
                    <input type="datetime-local" aria-label={c.pickSchedule} title={c.pickSchedule}
                      value={form.scheduled} disabled={!canEdit || busy}
                      min="2000-01-01T00:00" max="2199-12-31T23:59"
                      onClick={(event) => event.currentTarget.showPicker?.()}
                      onChange={(event) => onChange({ ...form, scheduled: event.target.value }, true)} />
                  </span>
                </div>
              </Field> : <Field id="studio-schedule-entry" label={c.schedule}><DateControl emptyLabel={presentationCopy[locale].dateTime} id="studio-schedule-entry" type="datetime-local" value={form.scheduled} disabled={!canEdit || busy}
                min="2000-01-01T00:00" max="2199-12-31T23:59"
                onChange={(event) => onChange({ ...form, scheduled: event.target.value }, true)} /></Field>}
              <Field id="studio-aspect" label={c.aspect} width="short"><select id="studio-aspect" value={form.aspect} disabled={!canEdit || busy}
                onChange={(event) => onChange({ ...form, aspect: event.target.value as AspectRatio }, false)}>
                <option value="9:16">{c.tall}</option><option value="16:9">{c.wide}</option>
              </select></Field>
            </FormRow>
            <div className="studio-version">
              <h3>{c.version}</h3>
              {shown ? <div><strong>{c.state[shown.draft.state as keyof typeof c.state] ?? unavailable} / {c.version} {shown.draft.version}</strong>
                <span>{c.savedAt}: {displayTime(locale, shown.draft.updated_at)}</span></div> : <p>{c.newDraft}</p>}
            </div>
            {!canEdit && shown && !recoveryBlocked && <p className="studio-note">{shown.can_manage ? c.notDraftEditable : c.readOnly}</p>}
            {formDirty && <p className="studio-dirty">{c.unsaved}</p>}
            {formError && <p role="alert" className="studio-error">{formError}</p>}
            <button type="button" className="primary studio-save" disabled={!canEdit || busy || (!newMode && !formDirty) || actionError === "uncertain" || actionError === "conflict"}
              onClick={onSave}>{busy ? c.saving : newMode ? c.create : c.save}</button>
            {showExit && <button type="button" onClick={onLeave}>{p.cancel}</button>}
            {canEdit && !busy && !newMode && !formDirty && !actionError && <p className="studio-note">{c.noChanges}</p>}
          </> : <p className="studio-panel-message" role="status">{detailMessage}</p>}
          {children}
      {confirmExit && <ConfirmDialog open title={c.dirty} description={p.metadataDirtyHelp}
        cancelLabel={p.metadataKeep} confirmLabel={p.metadataDiscard} onCancel={onKeep} onConfirm={onDiscard} />}
    </section>;
}
