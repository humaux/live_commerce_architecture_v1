// Purpose: The static-state sections of the design-system gallery: tokens, buttons, the DS-3 Figma specimen card (one control per Figma master variant, data-figma = node id), switch and tabs, badges, alerts, state blocks, upload area, form with save bars, settings layout and KPI cards.
// Depends on: react, @live-commerce/ui/ds, ../src/ui-gallery-copy, ./UiGallery.module.css; sample data only, no API.
// Used by: apps/admin/components/UiGallery.tsx.
"use client";
import { useState } from "react";
import {
  FormActionBar,
  Field,
  TextInput,
  Icon,
  Alert,
  Badge,
  BlockedState,
  Button,
  Card,
  Dropzone,
  EmptyState,
  ErrorState,
  FormSection,
  KpiCard,
  PermissionState,
  SegmentedSwitch,
  SettingsLayout,
  StatusPill,
  STATUS_FAMILIES,
  TabPanel,
  Tabs,
  type StatusDomain,
  type Tone,
} from "@live-commerce/ui/ds";
import type { GalleryCopy } from "../src/ui-gallery-copy";
import s from "./UiGallery.module.css";

export type GalleryParams = { tab: string; stab: string; sw: string; setting: string; list: string; nav: string; page: number; tstate: string };
export type SetParam = (key: keyof GalleryParams, value: string) => void;

const SWATCHES = ["--ds-nav-bg", "--ds-nav-active", "--ds-page", "--ds-surface", "--ds-primary", "--ds-link", "--ds-success-text", "--ds-warn-bg", "--ds-danger", "--ds-text-1", "--ds-text-2", "--ds-text-3", "--ds-border", "--ds-border-input"];
const SIZES = ["xs", "sm", "md", "lg", "xl", "2xl", "3xl"];
const TONES: Tone[] = ["neutral", "info", "success", "warning", "danger"];

export function TokensSection({ c }: { c: GalleryCopy }) {
  return (
    <Card id="sec-tokens" title={c.sTokens}>
      <p className={s.note}>{c.tokenNames}</p>
      <ul className={s.swatches}>
        {SWATCHES.map((name) => (
          <li key={name}>
            <span className={s.swatch} style={{ background: `var(${name})` }} aria-hidden="true" />
            <code>{name}</code>
          </li>
        ))}
      </ul>
      {SIZES.map((size) => (
        <p key={size} className={s.typeRow} style={{ fontSize: `var(--ds-text-${size})` }}>
          <code>--ds-text-{size}</code> {c.title}
        </p>
      ))}
    </Card>
  );
}

export function ButtonsSection({ c }: { c: GalleryCopy }) {
  const [clicked, setClicked] = useState("");
  return (
    <Card id="sec-buttons" title={c.sButtons}>
      <div className={s.row}>
        <Button variant="primary" onClick={() => setClicked(c.btnPrimary)}>{c.btnPrimary}</Button>
        <Button variant="secondary" onClick={() => setClicked(c.btnSecondary)}>{c.btnSecondary}</Button>
        <Button variant="danger" onClick={() => setClicked(c.btnDanger)}>{c.btnDanger}</Button>
        <Button variant="link" onClick={() => setClicked(c.btnLink)}>{c.btnLink}</Button>
      </div>
      <div className={s.row}>
        <Button size="sm" variant="secondary" onClick={() => setClicked(c.btnSmall)}>{c.btnSmall}</Button>
      </div>
      <div className={s.row}>
        <Button size="lg" variant="secondary" onClick={() => setClicked(c.btnLarge)}>{c.btnLarge}</Button>
      </div>
      <div className={s.row}>
        <Button variant="primary" loading loadingLabel={c.btnLoadingLabel}>{c.btnLoading}</Button>
      </div>
      <div className={s.row}>
        <Button variant="primary" disabledReason={c.btnDisabledReason}>{c.btnDisabled}</Button>
      </div>
      <p className={s.result} role="status">{clicked}</p>
    </Card>
  );
}

/** DS-3: one specimen per Figma Button / Input master variant (data-figma = Figma node id); tests/ui/ds-figma-parity.mjs reads their computed style. */
export function FigmaSection({ c }: { c: GalleryCopy }) {
  const [clicked, setClicked] = useState("");
  const kinds = [["primary", c.btnPrimary, "4:12712", "1394:103520"], ["secondary", c.btnSecondary, "4:12714", "1394:103522"], ["danger", c.btnDanger, "1394:103516", "1394:103526"], ["ghost", c.btnLink, "1394:103518", "1394:103528"], ["outline", c.btnLink, "4:12716", "1394:103524"]] as const;
  return (
    <Card id="sec-figma" title={`${c.sButtons} · Figma`}>
      <div className={s.row}>
        {kinds.map(([variant, label, id]) => <Button key={id} variant={variant} data-figma={id} onClick={() => setClicked(label)}>{label}</Button>)}
      </div>
      <div className={s.row}>
        {kinds.map(([variant, label, , id]) => <Button key={id} variant={variant} data-figma={id} disabledReason={c.btnDisabledReason}>{label}</Button>)}
      </div>
      <div className={s.row}>
        <TextInput data-figma="1401:38450" aria-label={c.nameLabel} placeholder={c.nameLabel} />
        <TextInput data-figma="1401:38452" aria-label={c.nameLabel} defaultValue={c.storeName} />
        <TextInput data-figma="1401:38454" aria-label={c.nameLabel} defaultValue={c.storeName} aria-invalid />
      </div>
      <p className={s.result} role="status">{clicked}</p>
    </Card>
  );
}

export function SwitchTabsSection({ c, params, setParam }: { c: GalleryCopy; params: GalleryParams; setParam: SetParam }) {
  const tabs = [{ id: "a", label: c.tabA }, { id: "b", label: c.tabB, count: 12 }, { id: "c", label: c.tabC }];
  const stabs = [{ id: "a", label: c.stabA }, { id: "b", label: c.stabB }, { id: "c", label: c.stabC }];
  const label = (list: { id: string; label: string }[], id: string) => list.find((t) => t.id === id)?.label ?? "";
  const on = params.sw !== "0";
  return (
    <Card id="sec-switch" title={c.sSwitch}>
      <div className={s.row}>
        <SegmentedSwitch checked={on} onChange={(next) => setParam("sw", next ? "1" : "0")} label={c.swLabel} onLabel={c.swOn} offLabel={c.swOff} />
        <span role="status">{c.swResult(on)}</span>
      </div>
      <Tabs idPrefix="gt" items={tabs} value={params.tab} onChange={(id) => setParam("tab", id)} ariaLabel={c.tabsLabel} />
      <TabPanel idPrefix="gt" id={params.tab}>{c.panelText(label(tabs, params.tab))}</TabPanel>
      <div className={s.gap} />
      <Tabs idPrefix="gs" variant="section" items={stabs} value={params.stab} onChange={(id) => setParam("stab", id)} ariaLabel={c.stabsLabel} />
      <TabPanel idPrefix="gs" id={params.stab}>{c.panelText(label(stabs, params.stab))}</TabPanel>
    </Card>
  );
}

export function BadgesSection({ c }: { c: GalleryCopy }) {
  return (
    <Card id="sec-badges" title={c.sBadges}>
      <div className={s.row}>
        {TONES.map((tone) => (
          <Badge key={tone} tone={tone}>{c.badgeTones[tone]}</Badge>
        ))}
      </div>
      {(Object.keys(STATUS_FAMILIES) as StatusDomain[]).map((domain) => (
        <div key={domain} className={s.row}>
          <strong className={s.rowTitle}>{c.domains[domain]}</strong>
          {Object.keys(STATUS_FAMILIES[domain]).map((status) => (
            // The key set of each family is closed in logic.ts; the cast only bridges Object.keys' string type.
            <StatusPill key={status} domain={domain} status={status as never} label={c.status[status] ?? status} />
          ))}
        </div>
      ))}
    </Card>
  );
}

export function AlertsSection({ c }: { c: GalleryCopy }) {
  const [shown, setShown] = useState(true);
  const [done, setDone] = useState("");
  return (
    <Card id="sec-alerts" title={c.sAlerts}>
      <div className={s.stack}>
        {TONES.filter((t) => t !== "neutral").map((tone) => (
          <Alert key={tone} tone={tone} title={c.badgeTones[tone]} action={tone === "warning" ? <Button size="sm" onClick={() => setDone(c.clicked(c.alertAction))}>{c.alertAction}</Button> : undefined}>
            {c.alertBody}
          </Alert>
        ))}
        {shown && (
          <Alert tone="info" onDismiss={() => setShown(false)} dismissLabel={c.alertDismiss}>
            {c.alertBody}
          </Alert>
        )}
      </div>
      <p className={s.result} role="status">{done}</p>
    </Card>
  );
}

export function StatesSection({ c }: { c: GalleryCopy }) {
  const [retried, setRetried] = useState(false);
  const [added, setAdded] = useState(false);
  return (
    <Card id="sec-states" title={c.sStates}>
      <div className={s.grid2}>
        <EmptyState title={c.emptyTitle} description={c.emptyDesc} action={<Button variant="primary" onClick={() => setAdded(true)}>{c.emptyAction}</Button>} />
        <ErrorState title={c.errorTitle} detail={c.errorDetail} retryLabel={c.retry} onRetry={() => setRetried(true)} />
        <PermissionState title={c.permTitle} reason={c.permReason} nextLabel={c.permNextLabel} nextStep={c.permNext} />
        <BlockedState title={c.blockedTitle} reason={c.blockedReason} nextLabel={c.blockedNextLabel} nextStep={c.blockedNext} />
      </div>
      {/* one line per control: retry must not hide the empty-state result (and vice versa) */}
      <p className={s.result} role="status">{retried ? c.retryDone : ""}</p>
      <p className={s.result} role="status">{added ? c.clicked(c.emptyAction) : ""}</p>
    </Card>
  );
}

export function DropzoneSection({ c }: { c: GalleryCopy }) {
  const [names, setNames] = useState<string[]>([]);
  const [library, setLibrary] = useState(false);
  return (
    <Card id="sec-dropzone" title={c.sDropzone}>
      <Dropzone
        title={c.dzTitle}
        hint={c.dzHint}
        rule={{ extensions: ["jpg", "jpeg", "png", "webp"], maxBytes: 5 * 1024 * 1024 }}
        errors={{ type: c.dzErrType, size: c.dzErrSize }}
        browseLabel={c.dzBrowse}
        libraryLabel={c.dzLibrary}
        onLibrary={() => setLibrary(true)}
        inputLabel={c.dzInput}
        onFiles={(files) => setNames(files.map((f) => f.name))}
      />
      <p className={s.result} role="status">{names.length ? c.dzAccepted(names.join(", ")) : ""}</p>
      <p className={s.result} role="status">{library ? c.dzLibraryClicked : ""}</p>
    </Card>
  );
}

export function FormSectionDemo({ c }: { c: GalleryCopy }) {
  const [saved, setSaved] = useState("");
  const [name, setName] = useState("");
  const [done, setDone] = useState(false);
  const dirty = name !== saved;
  return (
    <div id="sec-form">
      <FormActionBar
        label={c.barLabel}
        note={c.formNote}
        dirty={dirty}
        dirtyNote={c.formDirty}
        actions={
          <>
            <Button onClick={() => setName(saved)}>{c.cancel}</Button>
            <Button
              variant="primary"
              disabledReason={dirty ? undefined : c.saveReason}
              onClick={() => {
                setSaved(name);
                setDone(true);
              }}
            >
              {c.save}
            </Button>
          </>
        }
      >
        <FormSection title={c.formTitle} help={{ label: c.formHelpLabel, content: c.formHelp }}>
          <Field label={c.nameLabel} width="lg" counter={{ value: name, max: 12, format: (n) => c.formCounter(n) }}>
            {control => <TextInput {...control} value={name} onChange={event => { setName(event.target.value); setDone(false); }} />}
          </Field>
          <p className={s.result} role="status">{done && !dirty ? c.saved : ""}</p>
        </FormSection>
      </FormActionBar>
    </div>
  );
}

export function SettingsSection({ c, params }: { c: GalleryCopy; params: GalleryParams }) {
  const items = { profile: c.setItemProfile, shipping: c.setItemShipping, payments: c.setItemPayments, notices: c.setItemNotices } as Record<string, string>;
  const href = (id: string) => `?setting=${id}#sec-settings`;
  const current = params.setting in items ? params.setting : "profile";
  return (
    <div id="sec-settings">
      <SettingsLayout
        navLabel={c.setNavLabel}
        current={current}
        groups={[
          { id: "a", title: c.setGroupA, items: [{ id: "profile", label: c.setItemProfile, href: href("profile") }, { id: "shipping", label: c.setItemShipping, href: href("shipping") }] },
          { id: "b", title: c.setGroupB, items: [{ id: "payments", label: c.setItemPayments, href: href("payments") }, { id: "notices", label: c.setItemNotices, href: href("notices") }] },
        ]}
      >
        <h2 className={s.h2}>{c.setBodyTitle}</h2>
        <p>{c.setBodyText(items[current] ?? "")}</p>
      </SettingsLayout>
    </div>
  );
}

export function KpiSection({ c, params }: { c: GalleryCopy; params: GalleryParams }) {
  const names: Record<string, string> = { unread: c.kpiUnread, unshipped: c.kpiUnshipped, unpaid: c.kpiUnpaid };
  const href = (id: string) => `?list=${id}#sec-kpi`;
  return (
    <Card id="sec-kpi" title={c.sKpi}>
      <div className={s.grid3}>
        <KpiCard value="3" label={c.kpiUnread} href={href("unread")} icon={<Icon name="messages" size={24} />} />
        <KpiCard value="12" label={c.kpiUnshipped} href={href("unshipped")} icon={<Icon name="orders" size={24} />} />
        <KpiCard value="5" label={c.kpiUnpaid} href={href("unpaid")} icon={<Icon name="analytics" size={24} />} trend={{ text: c.kpiTrend, tone: "success" }} />
      </div>
      <p className={s.result} role="status">{names[params.list] ? c.kpiOpened(names[params.list] as string) : c.kpiNone}</p>
    </Card>
  );
}
