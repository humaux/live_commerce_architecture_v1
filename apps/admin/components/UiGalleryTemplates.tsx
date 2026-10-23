// @ds-page: built only from packages/ui DS components; design-spec rules R1–R7 apply (docs/design/DESIGN-SPEC.md §6).
// Purpose: Four complete SHOPLINE-style page compositions for design review; synthetic data and local demo state only.
// Depends on: React, @live-commerce/format (money), DS components, UiGalleryFrame, typed three-locale copy and gallery state helper.
// Used by: /{locale}/ui-gallery/templates/{list,detail,form,settings}; no live routes adopt these components.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { money } from "@live-commerce/format";
import {
  Alert, Badge, BulkMenu, Button, ButtonLink, Card, Checkbox, ConfirmDialog, DataTable, DetailLayout,
  Dropzone, EmptyState, ErrorState, Field, FilterBar, FilterSelect, FormActionBar, FormGrid, Icon,
  Inline, MenuButton, NumberInput, PageHeader, Pagination, RadioGroup, SearchBox, SegmentedSwitch,
  Select, SettingsLayout, Stack, StatusPill, TabPanel, Tabs, Textarea, TextInput, ThumbCell, Timeline,
  UploadGrid, bulkTarget, clearSelection, type SelectionState, type UploadItem,
} from "@live-commerce/ui/ds";
import { UiGalleryFrame } from "./UiGalleryFrame";
import { templateCopy, templateNames, type TemplateCopy, type TemplateName } from "../src/ui-gallery-template-copy";
import { useGalleryRecord } from "../src/ui-gallery-state";

export function TemplateLinks({ locale }: { locale: Locale }) {
  const t = templateCopy[locale];
  return <Card title={t.templates}><Inline>{templateNames.map(name => <ButtonLink key={name} href={`/${locale}/ui-gallery/templates/${name}`}>{t[name]}</ButtonLink>)}</Inline></Card>;
}
export type TemplateParams = { payment: string; state: string; page: number; section: string; setting: string; nav: string };
/** Only explicit non-sensitive display state is serializable; never copy arbitrary URL keys. */
function queryFor(params: TemplateParams) {
  const q = new URLSearchParams();
  if (params.payment !== "all") q.set("payment", params.payment);
  if (params.state !== "ready") q.set("state", params.state);
  if (params.page > 1) q.set("page", String(params.page));
  if (params.section !== "general") q.set("section", params.section);
  if (params.setting !== "profile") q.set("setting", params.setting);
  if (params.nav) q.set("nav", params.nav);
  return q.size ? `?${q}` : "";
}
export function UiGalleryTemplate({ locale, name, initial }: { locale: Locale; name: TemplateName; initial: TemplateParams }) {
  const t = templateCopy[locale];
  const [params, setParams] = useState(initial);
  const update = (patch: Partial<TemplateParams>) => {
    const next = { ...params, ...patch };
    setParams(next);
    window.history.replaceState(null, "", `${window.location.pathname}${queryFor(next)}`);
  };
  const root = `/${locale}/ui-gallery`;
  const [notice, persistNotice] = useGalleryRecord<{ text: string; created: string }>(`ds2-${name}-notice`, { text: "", created: "0" });
  const [create, setCreate] = useState(false);
  const created = /^\d{1,2}$/.test(notice.created) ? Number(notice.created) : 0;
  const setNotice = (text: string) => persistNotice({ ...notice, text });
  return <UiGalleryFrame locale={locale} template={name} currentId={params.nav || (name === "settings" ? "store-profile" : name === "form" ? "products" : "order-list")}>
    <div data-template={name}>
      <Stack>
        <PageHeader breadcrumbLabel={t.breadcrumb} title={name === "detail" ? `${t.detail} #DEMO-1001` : t[name]} description={name === "detail" ? `${t.synthetic} ${t.date}` : t.synthetic}
          status={name === "detail" ? <><StatusPill domain="payment" status="paid" label={t.paid} /><Badge tone="warning">{t.pending}</Badge></> : undefined}
          actions={name === "list" ? <><MenuButton label={t.export} items={[{ id: "export", label: t.export, onSelect: () => setNotice(t.exportDone) }]} /><Button variant="primary" disabledReason={created >= 99 ? t.noChanges : undefined} onClick={() => setCreate(true)}>{t.create}<Icon name="plus" size={16} /></Button></> : name === "detail" ? <><ButtonLink href={`${root}/templates/list`}>{t.back}</ButtonLink><MenuButton label={t.more} items={[{ id: "receipt", label: t.receipt, onSelect: () => setNotice(t.receiptDone) }, { id: "archive", label: t.archive, onSelect: () => setNotice(t.archived) }]} /></> : <ButtonLink href={root}>{t.returnGallery}</ButtonLink>} />
        {notice.text && <Alert tone="success">{notice.text}</Alert>}
        {name === "list" && <ListTemplate {...{ locale, t, params, update, setNotice, created }} />}
        {name === "detail" && <DetailTemplate {...{ locale, t }} />}
        {name === "form" && <ProductForm {...{ t, params, update }} />}
        {name === "settings" && <SettingsTemplate {...{ t, params }} />}
        <ConfirmDialog open={create} title={t.create} description={t.synthetic} confirmLabel={t.confirm} cancelLabel={t.cancel} tone="primary" onCancel={() => setCreate(false)} onConfirm={() => { persistNotice({ text: t.created, created: String(created + 1) }); setCreate(false); }} />
        <TemplateLinks locale={locale} />
      </Stack>
    </div>
  </UiGalleryFrame>;
}

const ORDERS = Array.from({ length: 24 }, (_, i) => ({ id: i + 1001, paid: i % 3 !== 0, amount: 360 + i * 120 }));
// Explicit synthetic pictogram, never a real product photograph.
const THUMB = "/demo-assets/gallery-product-thumb.svg"; // synthetic product photo stand-in (static asset, not UI chrome)
function ListTemplate({ locale, t, params, update, setNotice, created }: { created: number; locale: Locale; t: TemplateCopy; params: TemplateParams; update: (patch: Partial<TemplateParams>) => void; setNotice: (text: string) => void }) {
  const [query, setQuery] = useState("");
  const [draft, setDraft] = useState("");
  const [selection, setSelection] = useState<SelectionState>(clearSelection);
  const [confirm, setConfirm] = useState(false);
  const orders = [...Array.from({ length: created }, (_, i) => ({ id: 1025 + i, paid: false, amount: 360 })), ...ORDERS];
  const filtered = orders.filter(o => (params.payment === "paid" ? o.paid : params.payment === "unpaid" ? !o.paid : true) && (!query || `DEMO-${o.id}`.toLowerCase().includes(query.toLowerCase())));
  const total = params.state === "empty" ? 0 : filtered.length;
  const pageCount = Math.max(1, Math.ceil(total / 5));
  const page = Math.min(params.page, pageCount);
  const rows = params.state === "empty" ? [] : filtered.slice((page - 1) * 5, page * 5);
  const reset = () => { setQuery(""); setDraft(""); setSelection(clearSelection()); update({ payment: "all", state: "ready", page: 1 }); };
  const apply = () => {
    const target = bulkTarget(selection, `${params.payment}|${query}`, rows.map(o => String(o.id)));
    if (target.scope !== "stale_selection") setNotice(t.bulkDone);
    setSelection(clearSelection()); setConfirm(false);
  };
  return <Stack>
    <FilterBar label={t.previewControls}>
      <FilterSelect label={t.state} value={params.state} onChange={state => { update({ state, page: 1 }); setSelection(clearSelection()); }} options={[{ value: "ready", label: t.ready }, { value: "loading", label: t.loading }, { value: "error", label: t.error }, { value: "empty", label: t.empty }]} />
    </FilterBar>
    <Card title={t.orders}>
    <FilterBar label={t.filters}>
      <FilterSelect label={t.payment} value={params.payment} onChange={payment => { update({ payment, page: 1 }); setSelection(clearSelection()); }} options={[{ value: "all", label: t.all }, { value: "paid", label: t.paid }, { value: "unpaid", label: t.unpaid }]} />
      <SearchBox label={t.search} value={draft} onChange={setDraft} onSubmit={value => { setQuery(value); update({ page: 1 }); setSelection(clearSelection()); }} buttonLabel={t.searchButton} placeholder={t.searchPlaceholder} />
    </FilterBar>
    <DataTable caption={t.orders} scrollHint={t.tableScroll} rows={rows} rowId={o => String(o.id)} status={params.state === "loading" || params.state === "error" ? params.state : "ready"} loadingLabel={t.loading}
      empty={<EmptyState title={t.emptyTitle} description={t.emptyDesc} action={<Button variant="primary" onClick={reset}>{t.reset}</Button>} />}
      error={<ErrorState title={t.errorTitle} detail={t.errorDesc} retryLabel={t.retry} onRetry={() => update({ state: "ready" })} />}
      columns={[
        { id: "image", header: t.thumbnail, cell: () => <ThumbCell src={THUMB} alt={t.item} emptyLabel={t.thumbnail} /> },
        { id: "order", header: t.order, cell: o => <ButtonLink variant="link" size="sm" href={`/${locale}/ui-gallery/templates/detail`}>DEMO-{o.id}</ButtonLink>, wrap: true },
        { id: "buyer", header: t.buyer, cell: o => `${t.demoBuyer} ${o.id - 1000}`, wrap: true },
        { id: "payment", header: t.payment, cell: o => <StatusPill domain="payment" status={o.paid ? "paid" : "unpaid"} label={o.paid ? t.paid : t.unpaid} /> },
        { id: "amount", header: t.amount, cell: o => money(locale, "TWD", o.amount * 100), align: "end" },
      ]}
      actions={() => [{ id: "view", label: t.view, href: `/${locale}/ui-gallery/templates/detail` }, { id: "tag", label: t.tag, more: true, onSelect: () => setNotice(t.tagged) }]}
      actionsHeader={t.action} moreLabel={t.more}
      selection={{ state: selection, onChange: setSelection, total, filterKey: `${params.payment}|${query}`,
        actions: <BulkMenu label={t.action} items={[{ id: "tag", label: t.tag, onSelect: () => selection.scope === "all" ? setConfirm(true) : apply() }]} />,
        labels: {
        selectPage: t.selectPage, selectRow: n => `${t.row} ${n}`, selected: n => `${n} ${t.selected}`, pageOnly: t.pageOnly,
        allResults: n => `${t.allResults} (${n})`, selectAllResults: n => `${t.selectAll} (${n})`, clear: t.clear,
      } }} />
    {params.state === "ready" && <Pagination page={page} pageCount={pageCount} hrefFor={n => queryFor({ ...params, page: n }) || "?"}
      // A plain click pages in place so the in-memory search term (never put in the URL) survives; modified clicks keep the link (#87).
      onNavigate={(n, event) => { if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return; event.preventDefault(); update({ page: n }); setSelection(clearSelection()); }}
      labels={{ nav: t.paging, previous: t.previous, next: t.next, page: n => `${t.page} ${n}`, summary: `${total} ${t.filterSummary} · ${page} / ${pageCount}` }} />}
    <ConfirmDialog open={confirm} title={t.bulkConfirm} description={t.bulkDesc} confirmLabel={t.confirm} cancelLabel={t.cancel} tone="primary" onConfirm={apply} onCancel={() => setConfirm(false)} />
    </Card>
  </Stack>;
}
function DetailTemplate({ locale, t }: { locale: Locale; t: TemplateCopy }) {
  const [saved, persist, ready] = useGalleryRecord<{ note: string; result: string }>("ds2-detail-note", { note: "", result: "" });
  const [draft, setNote] = useState<string | null>(null);
  const note = draft ?? saved.note;
  // Do not expose an empty server-seeded field while the saved note is still being restored.
  if (!ready) return <p role="status">{t.loading}</p>;
  return <Stack>
    <DetailLayout main={<>
      <Card title={t.thumbnail}><Inline><ThumbCell src={THUMB} alt={t.item} emptyLabel={t.thumbnail} /><div><strong>{t.item}</strong><p>{t.quantity}: 2 · {money(locale, "TWD", 36000)}</p></div></Inline></Card>
      <Card title={t.timeline}><Timeline label={t.timeline} items={[
        { id: "pack", title: t.packed, time: "2026-10-15 10:30 UTC+8", dateTime: "2026-10-15T10:30:00+08:00" },
        { id: "pay", title: t.paidEvent, time: "2026-10-15 10:05 UTC+8", dateTime: "2026-10-15T10:05:00+08:00", tone: "success" },
        { id: "create", title: t.received, time: "2026-10-15 10:00 UTC+8", dateTime: "2026-10-15T10:00:00+08:00", tone: "success" },
      ]} /></Card>
      <Card title={t.note}><Stack><Field label={t.noteContent} width="full" counter={{ value: note, max: 500, format: (n, max) => `${n} / ${max}` }}>{control => <Textarea {...control} value={note} onChange={event => setNote(event.target.value)} />}</Field>
        <Inline><Button variant="primary" disabledReason={note.trim() && note !== saved.note ? undefined : t.noChanges} onClick={() => persist({ note, result: t.noteSaved })}>{t.addNote}</Button></Inline>
        {saved.result && <Alert tone="success">{saved.result}</Alert>}</Stack></Card>
    </>} summary={<Card title={t.summary}><Stack><div><strong>{t.buyer}</strong><p>{t.demoBuyer} A</p></div><div><strong>{t.shipping}</strong><p>{t.delivery}</p></div><div><strong>{t.total}</strong><p>{money(locale, "TWD", 72000)}</p></div></Stack></Card>} />
  </Stack>;
}

type FormData = { name: string; category: string; description: string; price: string; stock: string; visible: boolean; channel: string; enabled: boolean; images: string };
function ProductForm({ t, params, update }: { t: TemplateCopy; params: TemplateParams; update: (patch: Partial<TemplateParams>) => void }) {
  const [saved, persist, ready] = useGalleryRecord<FormData>("ds2-product", { name: t.sampleName, category: "bags", description: "", price: "360", stock: "12", visible: true, channel: "online", enabled: true, images: "[]" });
  const [data, setData] = useState(saved);
  useEffect(() => { setData(saved); }, [saved]);
  const [saving, setSaving] = useState(false);
  const [done, setDone] = useState(false);
  const [cancel, setCancel] = useState(false);
  const change = <K extends keyof FormData>(key: K, value: FormData[K]) => { setData(current => ({ ...current, [key]: value })); setDone(false); };
  const dirty = JSON.stringify(data) !== JSON.stringify(saved);
  const nameError = data.name.trim() ? undefined : t.required;
  const priceError = data.price.trim() !== "" && Number.isFinite(Number(data.price)) && Number(data.price) >= 0 ? undefined : t.priceError;
  const stockError = /^\d+$/.test(data.stock) && Number.isSafeInteger(Number(data.stock)) ? undefined : t.inventoryError;
  const invalid = Boolean(nameError || priceError || stockError);
  // Saving mid-transfer would persist "uploading" and the transfer's completion would re-dirty the saved form (#87).
  const uploading = (() => { try { const items: unknown = JSON.parse(data.images); return Array.isArray(items) && items.some(item => item?.status === "uploading"); } catch { return false; } })();
  useEffect(() => {
    if (!saving) return;
    const timer = setTimeout(() => { persist(data); setSaving(false); setDone(true); }, 450);
    return () => clearTimeout(timer);
    // The inputs are read-only while this simulated save is in flight.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [saving]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  const state = saving ? "saving" : dirty ? "dirty" : done ? "saved" : "clean";
  return <FormActionBar label={t.action} state={state} dirty={dirty} note={t.clean} dirtyNote={t.dirty} savingNote={t.saving} savedNote={t.saved}
    actions={<><Button disabledReason={!ready ? t.loading : saving ? t.saving : undefined} onClick={() => { if (dirty) setCancel(true); }}>{t.cancel}</Button><Button variant="primary" loading={saving} loadingLabel={t.saving} disabledReason={!ready ? t.loading : invalid ? t.invalid : !dirty ? t.noChanges : uploading ? t.uploadPending : undefined} onClick={() => setSaving(true)}>{t.save}</Button></>}>
    <Stack>
      <Tabs idPrefix="product-sections" variant="section" value={params.section} onChange={section => update({ section })} ariaLabel={t.form} items={[{ id: "general", label: t.general }, { id: "media", label: t.media }, { id: "pricing", label: t.pricing }]} />
      <TabPanel idPrefix="product-sections" id={params.section} flush>
        {params.section === "general" && <Stack><FormGrid>
          <Field label={t.name} required help={t.nameHelp} error={nameError} counter={{ value: data.name, max: 80, format: (n, max) => `${n} / ${max}` }}>{control => <TextInput {...control} value={data.name} readOnly={saving || !ready} onChange={event => change("name", event.target.value)} />}</Field>
          <Field label={t.category}>{control => <Select {...control} value={data.category} disabled={saving || !ready} onChange={event => change("category", event.target.value)}><option value="bags">{t.bags}</option><option value="accessories">{t.accessories}</option></Select>}</Field>
        </FormGrid><Field label={t.description} help={t.descriptionHelp} counter={{ value: data.description, max: 500, format: (n, max) => `${n} / ${max}` }}>{control => <Textarea {...control} value={data.description} readOnly={saving || !ready} onChange={event => change("description", event.target.value)} />}</Field>
          <Checkbox label={t.visible} checked={data.visible} disabled={saving || !ready} onChange={event => change("visible", event.target.checked)} />
          <RadioGroup disabled={saving || !ready} label={t.channel} value={data.channel} onChange={value => { if (!saving) change("channel", value); }} options={[{ value: "online", label: t.online }, { value: "live", label: t.live }]} />
          <Inline><span>{t.enabled}</span><SegmentedSwitch label={t.enabled} checked={data.enabled} onChange={value => change("enabled", value)} onLabel={t.yes} offLabel={t.no} disabledReason={!ready ? t.loading : saving ? t.saving : undefined} /></Inline>
        </Stack>}
        {params.section === "media" && <GalleryUpload t={t} value={data.images} onChange={value => change("images", value)} disabled={saving || !ready} />}
        {params.section === "pricing" && <FormGrid>
          <Field label={t.price} required error={priceError} width="full">{control => <NumberInput {...control} unit="NT$" min="0" step="0.01" value={data.price} readOnly={saving || !ready} onChange={event => change("price", event.target.value)} />}</Field>
          <Field label={t.inventory} required error={stockError} width="full">{control => <TextInput {...control} type="number" min="0" step="1" value={data.stock} readOnly={saving || !ready} onChange={event => change("stock", event.target.value)} />}</Field>
        </FormGrid>}
      </TabPanel>
      <ConfirmDialog open={cancel} title={t.cancelConfirm} confirmLabel={t.discard} cancelLabel={t.cancel} onCancel={() => setCancel(false)} onConfirm={() => { setData(saved); setCancel(false); }} />
    </Stack>
  </FormActionBar>;
}
/** A deliberately labelled simulator: no upload endpoint is contacted and no transport success is implied. */
export function GalleryUpload({ t, value, onChange, disabled = false }: { t: TemplateCopy; value?: string; onChange?: (value: string) => void; disabled?: boolean }) {
  const [own, setOwn] = useState("[]");
  const raw = value ?? own;
  const previews = useRef(new Map<string, string>());
  const items: UploadItem[] = (() => {
    try {
      const parsed: unknown = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      return parsed.filter(item => item && typeof item.id === "string" && typeof item.name === "string" && ["uploading", "error", "done"].includes(item.status) && typeof item.progress === "number").slice(0, 12).map(item => ({ ...item, src: previews.current.get(item.id), error: item.status === "error" ? t.transferError : undefined }));
    } catch { return []; }
  })();
  const write = (next: UploadItem[]) => {
    const serialized = JSON.stringify(next.map(({ id, name, status, progress }) => ({ id, name, status, progress })));
    if (onChange) onChange(serialized); else setOwn(serialized);
  };
  const [fail, setFail] = useState(false);
  useEffect(() => {
    const urls = previews.current;
    return () => { for (const url of urls.values()) URL.revokeObjectURL(url); };
  }, []);
  useEffect(() => {
    if (disabled || !items.some(item => item.status === "uploading")) return;
    const finish = () => write(items.map(item => item.status === "uploading" ? { ...item, progress: 100, status: "done" } : item));
    // Gallery test seam (#147 TH-13): only a gallery build with NEXT_PUBLIC_LC_GALLERY_TEST=1 keeps this branch. Next inlines the
    // literal at `next build`, so a production build folds it to false and drops the hold/release hook entirely. While
    // window.__lcHoldUpload is set the simulated transfer waits for lc-gallery-upload-release instead of its timer.
    if (process.env.NEXT_PUBLIC_LC_GALLERY_TEST === "1") {
      if ((window as unknown as { __lcHoldUpload?: boolean }).__lcHoldUpload) {
        window.addEventListener("lc-gallery-upload-release", finish, { once: true });
        return () => window.removeEventListener("lc-gallery-upload-release", finish);
      }
    }
    const timer = setTimeout(finish, 700);
    return () => clearTimeout(timer);
    // The serialized queue is the complete dependency; controls remain disabled while the form saves.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [raw, disabled]);
  return <Stack>
    <Dropzone title={t.upload} hint={t.uploadHint} rule={{ extensions: ["jpg", "jpeg", "png", "webp"], maxBytes: 5 * 1024 * 1024 }} errors={{ type: t.uploadHint, size: t.uploadHint }} browseLabel={t.browse} inputLabel={t.uploadLabel} disabledReason={disabled ? t.saving : undefined}
      onFiles={files => {
        if (disabled) return;
        const added = files.slice(0, Math.max(0, 12 - items.length)).map((file): UploadItem => {
          const id = crypto.randomUUID();
          previews.current.set(id, URL.createObjectURL(file));
          return { id, name: file.name.slice(0, 60), status: fail ? "error" : "uploading", progress: fail ? 0 : 35 };
        });
        write([...items, ...added]);
      }} />
    <Checkbox label={t.transferFail} checked={fail} disabled={disabled} onChange={event => setFail(event.target.checked)} />
    <UploadGrid disabledReason={disabled ? t.saving : undefined} items={items} label={t.uploadQueue} doneLabel={t.uploaded} removeLabel={name => `${t.remove} ${name}`} retryLabel={name => `${t.retry} ${name}`} progressLabel={n => `${t.progress} ${n}%`}
      onRemove={id => { if (disabled) return; const url = previews.current.get(id); if (url) URL.revokeObjectURL(url); previews.current.delete(id); write(items.filter(item => item.id !== id)); }} onRetry={id => { if (!disabled) write(items.map(item => item.id === id ? { ...item, status: "uploading", progress: 35 } : item)); }} />
  </Stack>;
}
function SettingsTemplate({ t, params }: { t: TemplateCopy; params: TemplateParams }) {
  const [saved, persist, ready] = useGalleryRecord<{ name: string; currency: string; timezone: string; notices: boolean }>("ds2-settings", { name: "Demo store", currency: "TWD", timezone: "Asia/Taipei", notices: false });
  // No editable draft until the first edit: restored values never need a second effect to reach the fields.
  const [draft, setData] = useState<typeof saved | null>(null);
  const data = draft ?? saved;
  const [done, setDone] = useState(false);
  const [saving, setSaving] = useState(false);
  const dirty = JSON.stringify(data) !== JSON.stringify(saved);
  useEffect(() => {
    if (!saving) return;
    const timer = setTimeout(() => { persist(data); setDone(true); setSaving(false); }, 450);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [saving]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  const current = params.setting === "notices" ? "notices" : "profile";
  // A reload must expose restored preferences, never the server's synthetic defaults as a usable form.
  if (!ready) return <p role="status">{t.loading}</p>;
  return <SettingsLayout navLabel={t.nav} current={current} groups={[{ id: "store", title: t.config, items: [{ id: "profile", label: t.storeProfile, href: "?setting=profile" }, { id: "notices", label: t.notices, href: "?setting=notices" }] }]}>
    <FormActionBar label={t.action} dirty={dirty} state={saving ? "saving" : dirty ? "dirty" : done ? "saved" : "clean"} note={t.clean} dirtyNote={t.dirty} savingNote={t.saving} savedNote={t.saved}
      actions={<><Button disabledReason={saving ? t.saving : undefined} onClick={() => { setData(null); setDone(false); }}>{t.cancel}</Button><Button variant="primary" loading={saving} disabledReason={!ready ? t.loading : !data.name.trim() ? t.invalid : !dirty ? t.noChanges : undefined} onClick={() => setSaving(true)}>{t.save}</Button></>}>
      <Stack><h2>{current === "profile" ? t.storeProfile : t.notices}</h2>
        {current === "profile" ? <Stack><Field label={t.storeName} required width="full" error={data.name.trim() ? undefined : t.required} counter={{ value: data.name, max: 80, format: (n, max) => `${n} / ${max}` }}>{control => <TextInput {...control} value={data.name} readOnly={saving || !ready} onChange={event => { setData({ ...data, name: event.target.value }); setDone(false); }} />}</Field>
          <FormGrid><Field label={t.currency} help={t.currencyHelp}>{control => <Select {...control} value={data.currency} disabled={saving || !ready} onChange={event => { setData({ ...data, currency: event.target.value }); setDone(false); }}><option value="TWD">TWD · NT$</option><option value="USD">USD · US$</option></Select>}</Field>
            <Field label={t.timezone} help={t.tzHelp}>{control => <Select {...control} value={data.timezone} disabled={saving || !ready} onChange={event => { setData({ ...data, timezone: event.target.value }); setDone(false); }}><option value="Asia/Taipei">Asia/Taipei · UTC+8</option><option value="UTC">UTC</option></Select>}</Field></FormGrid></Stack>
          : <><Checkbox label={t.emailNotices} checked={data.notices} disabled={saving || !ready} onChange={event => { setData({ ...data, notices: event.target.checked }); setDone(false); }} /><p>{t.noticesHelp}</p></>}
      </Stack>
    </FormActionBar>
  </SettingsLayout>;
}
