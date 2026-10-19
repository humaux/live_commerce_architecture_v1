// Purpose: The design-system gallery page body: mounts AppShellV2 around every component state (see UiGallerySections for the static ones) and owns the list demo, the confirm dialogs and the URL-held demo state.
// Depends on: react, @live-commerce/ui/ds, @live-commerce/i18n, ../src/ui-gallery-copy, ../src/ui-gallery-nav, ./UiGallerySections, ./UiGallery.module.css; sample data only, no API or session.
// Used by: apps/admin/app/[locale]/ui-gallery/page.tsx (rendered only when LC_UI_GALLERY=1).
"use client";
import { useEffect, useMemo, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { UiGalleryFrame } from "./UiGalleryFrame";
import { TemplateLinks } from "./UiGalleryTemplates";
import {
  Alert,
  BulkMenu,
  Button,
  Card,
  ConfirmDialog,
  DataTable,
  EmptyState,
  ErrorState,
  FilterBar,
  FilterDivider,
  FilterSelect,
  MenuButton,
  PageHeader,
  Pagination,
  SearchBox,
  Badge,
  ThumbCell,
  bulkNeedsConfirm,
  bulkTarget,
  clearSelection,
  emptySelection,
  selectionCount,
  withPage,
  type Column,
  type RowAction,
  type SelectionState,
} from "@live-commerce/ui/ds";
import { galleryCopy, type GalleryCopy } from "../src/ui-gallery-copy";
import {
  AlertsSection,
  BadgesSection,
  ButtonsSection,
  DropzoneSection,
  FormSectionDemo,
  KpiSection,
  SettingsSection,
  StatesSection,
  SwitchTabsSection,
  TokensSection,
  type GalleryParams,
  type SetParam,
} from "./UiGallerySections";
import s from "./UiGallery.module.css";

const PAGE_SIZE = 5;
const TOTAL_PRODUCTS = 47;
type Product = { n: number; published: boolean; price: number; stock: number; thumb?: string };
const thumb = `data:image/svg+xml;utf8,${encodeURIComponent("<svg xmlns='http://www.w3.org/2000/svg' width='64' height='64'><rect width='64' height='64' fill='rgb(232,241,250)'/><circle cx='32' cy='32' r='16' fill='rgb(68,146,209)'/></svg>")}`;
const PRODUCTS: Product[] = Array.from({ length: TOTAL_PRODUCTS }, (_, i) => {
  const n = i + 1;
  return { n, published: n % 3 !== 0, price: 100 + n * 7, stock: (n * 5) % 40, thumb: n % 4 === 0 ? undefined : thumb };
});

type Dialog = { kind: "plain" | "typed"; title: string; description: string; done: () => void };

/** List conditions live in the URL (`pf` = status filter; search is memory-only) so paging links, refresh and back keep the same result set. */
const FILTERS = ["all", "published", "draft"];
/** Replaces the list conditions in the URL (undefined removes a key) and returns to page 1, the page of a different result set. */
type SetList = (conditions: { pf: string | undefined }) => void;

function ProductTable({ c, params, setParam, setList, search }: { c: GalleryCopy; params: GalleryParams; setParam: SetParam; setList: SetList; search: string }) {
  const [filter, setFilter] = useState(() => {
    const raw = new URLSearchParams(search).get("pf");
    return raw !== null && FILTERS.includes(raw) ? raw : "all";
  });
  const initialQuery = ""; // Search may contain personal data: never read/write it through the URL.
  const [draft, setDraft] = useState(initialQuery);
  const [query, setQuery] = useState(initialQuery);
  const [selection, setSelection] = useState<SelectionState>(emptySelection);
  const [note, setNote] = useState("");
  const [bulkDone, setBulkDone] = useState("");
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const filtered = useMemo(
    () =>
      PRODUCTS.filter((p) => (filter === "all" ? true : filter === "published" ? p.published : !p.published)).filter((p) =>
        query ? c.productName(p.n).toLowerCase().includes(query.toLowerCase()) : true,
      ),
    [filter, query, c],
  );
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const page = Math.min(Math.max(1, params.page), pageCount);
  const status = params.tstate === "loading" || params.tstate === "error" ? params.tstate : "ready";
  const rows = params.tstate === "empty" ? [] : filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);
  const total = params.tstate === "empty" ? 0 : filtered.length;
  const filterKey = `${filter}|${query}`;
  const pageIds = rows.map((p) => String(p.n));
  const count = selectionCount(selection, total, pageIds);

  const act = (label: string, p: Product) => setNote(c.lastAction(label, c.productName(p.n)));
  const columns: Column<Product>[] = [
    { id: "thumb", header: c.colThumb, cell: (p) => <ThumbCell src={p.thumb} alt={c.productName(p.n)} emptyLabel={c.thumbEmpty} />, width: "88px" },
    { id: "name", header: c.colName, cell: (p) => c.productName(p.n), wrap: true },
    { id: "price", header: c.colPrice, cell: (p) => `NT$ ${p.price}`, align: "end" },
    { id: "stock", header: c.colStock, cell: (p) => p.stock, align: "end" },
    { id: "status", header: c.colStatus, cell: (p) => <Badge tone={p.published ? "success" : "neutral"}>{p.published ? c.published : c.draft}</Badge> },
  ];
  const actions = (p: Product): RowAction[] => [
    { id: "edit", label: c.edit, href: `?edit=${p.n}#sec-table` },
    p.published
      ? { id: "unpublish", label: c.unpublish, variant: "secondary", onSelect: () => act(c.unpublish, p) }
      : { id: "publish", label: c.publish, variant: "primary", onSelect: () => act(c.publish, p) },
    { id: "dup", label: c.duplicate, more: true, onSelect: () => act(c.duplicate, p) },
    {
      id: "del",
      label: c.del,
      more: true,
      variant: "danger",
      onSelect: () => setDialog({ kind: "plain", title: c.dlgTitle, description: c.dlgDesc, done: () => act(c.del, p) }),
    },
  ];
  const deleteSelected = () =>
    setDialog({
      kind: bulkNeedsConfirm(selection) ? "typed" : "plain",
      title: bulkNeedsConfirm(selection) ? c.dlgTypedTitle : c.dlgTitle,
      description: bulkNeedsConfirm(selection) ? c.dlgTypedDesc : c.dlgDesc,
      done: () => {
        const target = bulkTarget(selection, filterKey, pageIds);
        setSelection(clearSelection());
        if (target.scope === "stale_selection") return;
        setBulkDone(c.bulkDone(count, target.scope === "filter" ? c.scopeFilter : c.scopeIds));
      },
    });
  const dialogs = (
    <>
      <ConfirmDialog
        open={dialog !== null}
        title={dialog?.title ?? ""}
        description={dialog?.description}
        confirmLabel={c.dlgConfirm}
        cancelLabel={c.cancel}
        typed={dialog?.kind === "typed" ? { phrase: c.dlgPhrase, label: c.dlgTypedLabel, mismatchReason: c.dlgMismatch } : undefined}
        onCancel={() => setDialog(null)}
        onConfirm={() => {
          dialog?.done();
          setDialog(null);
        }}
      />
    </>
  );
  return (
    <>
      <Card id="sec-table" title={c.sTable} flush>
        <div className={s.pad}>
          <FilterBar label={c.filterGroup}>
            <FilterSelect
              label={c.filterShow}
              value={filter}
              onChange={(v) => {
                setFilter(v);
                setList({ pf: v === "all" ? undefined : v });
                setSelection(clearSelection());
              }}
              options={[{ value: "all", label: c.filterAll }, { value: "published", label: c.filterPublished }, { value: "draft", label: c.filterDraft }]}
            />
            <SearchBox
              label={c.searchLabel}
              value={draft}
              onChange={setDraft}
              onSubmit={(q) => {
                setQuery(q);
                setList({ pf: filter === "all" ? undefined : filter });
                setSelection(clearSelection());
                setNote(c.searchResult(q));
              }}
              buttonLabel={c.searchButton}
              placeholder={c.searchPlaceholder}
            />
            <FilterDivider />
            <BulkMenu
              label={c.bulk}
              disabledReason={count === 0 ? c.bulkNone : undefined}
              items={[
                { id: "unpublish", label: c.bulkUnpublish, onSelect: () => setNote(c.lastAction(c.bulkUnpublish, c.selected(count))) },
                { id: "delete", label: c.bulkDelete, variant: "danger", onSelect: deleteSelected },
              ]}
            />
            <FilterSelect
              label={c.stateLabel}
              value={params.tstate in { loading: 1, error: 1, empty: 1 } ? params.tstate : "ready"}
              onChange={(v) => setParam("tstate", v)}
              options={[{ value: "ready", label: c.stateReady }, { value: "loading", label: c.stateLoading }, { value: "error", label: c.stateError }, { value: "empty", label: c.stateEmpty }]}
            />
          </FilterBar>
          {note && <p className={s.result} role="status">{note}</p>}
          {bulkDone && <Alert tone="success">{bulkDone}</Alert>}
        </div>
        <DataTable
          scrollHint={c.tableScroll}
          caption={c.tblCaption}
          columns={columns}
          rows={rows}
          rowId={(p) => String(p.n)}
          status={status}
          loadingLabel={c.loadingLabel}
          empty={<EmptyState title={c.emptyTitle} description={c.emptyDesc} action={<Button variant="primary" onClick={() => setNote(c.clicked(c.emptyAction))}>{c.emptyAction}</Button>} />}
          error={<ErrorState title={c.errorTitle} detail={c.errorDetail} retryLabel={c.retry} onRetry={() => setParam("tstate", "ready")} />}
          actions={actions}
          actionsHeader={c.actionsHeader}
          moreLabel={c.more}
          selection={{
            state: selection,
            onChange: setSelection,
            total,
            filterKey,
            labels: { selectPage: c.selectPage, selectRow: c.selectRow, selected: c.selected, pageOnly: c.pageOnly, allResults: c.allResults, selectAllResults: c.selectAll, clear: c.clear },
          }}
        />
        <Pagination
          page={page}
          pageCount={pageCount}
          hrefFor={(n) => `${withPage(search, n)}#sec-table`}
          labels={{ nav: c.pagerNav, previous: c.prev, next: c.next, page: c.pageN, summary: c.pagerSummary(page, pageCount, total) }}
        />
      </Card>
      {dialogs}
      <DialogDemo c={c} />
    </>
  );
}

function DialogDemo({ c }: { c: GalleryCopy }) {
  const [open, setOpen] = useState<"plain" | "typed" | "busy" | null>(null);
  const [result, setResult] = useState("");
  // The "busy" demo models a request in flight with NO busyLabel: Cancel and Escape must stay blocked until it settles (5 s).
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!busy) return;
    const timer = setTimeout(() => setBusy(false), 5000);
    return () => clearTimeout(timer);
  }, [busy]);
  return (
    <Card id="sec-dialog" title={c.sDialog}>
      <div className={s.row}>
        <Button onClick={() => setOpen("plain")}>{c.dlgOpenPlain}</Button>
        <Button variant="danger" onClick={() => setOpen("typed")}>{c.dlgOpenTyped}</Button>
        <Button
          onClick={() => {
            setBusy(true);
            setOpen("busy");
          }}
        >
          {c.dlgOpenBusy}
        </Button>
        <span role="status">{result}</span>
      </div>
      <ConfirmDialog
        open={open !== null}
        title={open === "typed" ? c.dlgTypedTitle : c.dlgTitle}
        description={open === "typed" ? c.dlgTypedDesc : c.dlgDesc}
        confirmLabel={c.dlgConfirm}
        cancelLabel={c.cancel}
        busy={open === "busy" && busy}
        typed={open === "typed" ? { phrase: c.dlgPhrase, label: c.dlgTypedLabel, mismatchReason: c.dlgMismatch } : undefined}
        onCancel={() => {
          setOpen(null);
          setResult(c.dlgCancelled);
        }}
        onConfirm={() => {
          setOpen(null);
          setResult(c.dlgDone);
        }}
      />
    </Card>
  );
}

/** Gallery of every design-system component and state inside the v2 shell. Demo state that has a URL form (tabs, switch, settings item, KPI list, table state, page) survives a reload. */
export function UiGallery({ locale, initial, search: initialSearch }: { locale: Locale; initial: GalleryParams; search: string }) {
  const c = galleryCopy[locale];
  const [params, setParams] = useState(initial);
  // The query string is state too, so the page links rendered on the server and on the client agree.
  const [search, setSearch] = useState(initialSearch);
  const setParam: SetParam = (key, value) => {
    setParams((p) => ({ ...p, [key]: key === "page" ? Number(value) : value }));
    const query = new URLSearchParams(search);
    query.set(key, value);
    setSearch(`?${query.toString()}`);
    window.history.replaceState(null, "", `${window.location.pathname}?${query.toString()}${window.location.hash}`);
  };
  const setList: SetList = (conditions) => {
    setParams((p) => ({ ...p, page: 1 }));
    const query = new URLSearchParams(search);
    for (const [key, value] of Object.entries(conditions)) {
      if (value === undefined) query.delete(key);
      else query.set(key, value);
    }
    query.delete("page");
    const text = query.toString();
    setSearch(text ? `?${text}` : "?");
    window.history.replaceState(null, "", `${window.location.pathname}${text ? `?${text}` : ""}${window.location.hash}`);
  };
  return (
    <UiGalleryFrame locale={locale} currentId={params.nav || "products"}>
      <PageHeader breadcrumbLabel={c.crumbLabel} title={c.title} description={c.description} />
      <div className={s.page}>
        <TemplateLinks locale={locale} />
        <TokensSection c={c} />
        <ButtonsSection c={c} />
        <SwitchTabsSection c={c} params={params} setParam={setParam} />
        <BadgesSection c={c} />
        <AlertsSection c={c} />
        <StatesSection c={c} />
        <DropzoneSection c={c} />
        <FormSectionDemo c={c} />
        <SettingsSection c={c} params={params} />
        <KpiSection c={c} params={params} />
        <ProductTable c={c} params={params} setParam={setParam} setList={setList} search={search} />
      </div>
    </UiGalleryFrame>
  );
}
