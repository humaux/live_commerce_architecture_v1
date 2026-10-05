"use client";

// Merchant store-design page (/{locale}/design): store profile, header/footer navigation, home sections, information
// pages, Save draft / Preview / Publish and published versions with Rollback (unit store-design,
// contracts/storefront-v2.md section B).
// BFF (browser, via lib/design-client.ts): GET|PUT /api/stores/{store}/design/draft, POST .../design/publish, .../rollback,
// .../preview-token, GET .../design/versions, GET|POST .../design/media, GET .../design/media/{id}, POST
// .../design/media/{id}/delete, plus GET .../storefront (the active address, to open the preview) -> Go
// /v1/admin/stores/{id}/design/* (internal/httpapi/design.go; integration:read / integration:manage).
// Owns: the editable copy of the document, the dirty/unsaved guard, the local "what is still missing" hints and the
// request choreography (save before publish/preview, compare-and-set versions). It never validates what the server
// owns (the server's 422 path is shown on the field) and never keeps the preview token (it only opens the link).
// Unlike the PII pages it does NOT drop its state when the tab is hidden: unsaved edits must survive a tab switch; every
// write is still fenced by the session boundary.
import { useEffect, useMemo, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Badge, TabStrip, TableFrame } from "@live-commerce/ui";
import { catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import type { Store } from "@/lib/model";
import { ReadError, type ReadCode } from "@/lib/customers-client";
import { sessionBoundary } from "@/lib/settings-client";
import { readStorefront } from "@/lib/storefront-client";
import {
  deleteMedia,
  issuePreview,
  mediaFileProblem,
  publish,
  readDraft,
  readMedia,
  readVersions,
  rollback,
  saveDraft,
  uploadMedia,
  type Outcome,
} from "@/lib/design-client";
import { designCopy, fill, type DesignCopy } from "@/lib/design-copy";
import {
  cleanForSave,
  localIssues,
  type DesignDocument,
  type Draft,
  type MediaItem,
  type VersionList,
} from "@/lib/design-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { DesignProfile } from "./DesignProfile";
import { DesignNav } from "./DesignNav";
import { DesignSections } from "./DesignSections";
import { DesignPages } from "./DesignPages";
import type { MediaOps } from "./DesignMedia";
import "./orders.css";
import "./design.css";

type Loaded = { draft: Draft; media: MediaItem[]; versions: VersionList };
type Tab = "profile" | "nav" | "home" | "pages" | "versions";
const TABS: Tab[] = ["profile", "nav", "home", "pages", "versions"];
type Note = { kind: "ok" | "error"; text: string } | null;

async function loadAll(store: string, signal: AbortSignal): Promise<Loaded> {
  const [draft, media, versions] = await Promise.all([
    readDraft(store, signal),
    readMedia(store, signal),
    readVersions(store, signal),
  ]);
  return { draft, media, versions };
}

// One read per (store, reload). The boundary is the session fence write requests must still match.
function useLoad(
  storeID: string | null,
  initialError: ReadCode | null,
  renderKey: string,
) {
  const [tick, setTick] = useState(0);
  const key = `${renderKey}|${storeID ?? ""}|${tick}`;
  const [view, setView] = useState<{
    key: string;
    status: "loading" | "ready" | ReadCode;
    data: Loaded | null;
    boundary: string;
  }>({ key: "", status: "loading", data: null, boundary: "" });
  useEffect(() => {
    if (initialError || !storeID) {
      setView({
        key,
        status: initialError ?? "not-found",
        data: null,
        boundary: "",
      });
      return;
    }
    let live = true;
    const controller = new AbortController();
    void (async () => {
      try {
        const boundary = await sessionBoundary();
        const data = await loadAll(storeID, controller.signal);
        if (!live) return;
        if ((await sessionBoundary()) !== boundary)
          throw new ReadError("signed-out");
        setView({ key, status: "ready", data, boundary });
      } catch (error) {
        if (live)
          setView({
            key,
            status: error instanceof ReadError ? error.code : "unavailable",
            data: null,
            boundary: "",
          });
      }
    })();
    return () => {
      live = false;
      controller.abort();
    };
  }, [key, storeID, initialError]);
  const current =
    view.key === key
      ? view
      : { key, status: "loading" as const, data: null, boundary: "" };
  return { ...current, reload: () => setTick((value) => value + 1) };
}

export function Design({
  locale,
  stores,
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
  const c = designCopy[locale];
  const read = useLoad(store?.id ?? null, initialError, renderKey);
  const dirtyRef = useRef(false);
  const guard = () => !dirtyRef.current || window.confirm(c.guard);
  const failure =
    read.status === "signed-out"
      ? c.signedOut
      : read.status === "forbidden"
        ? c.forbidden
        : read.status === "not-found"
          ? store
            ? c.notFound
            : c.noStore
          : read.status === "unavailable"
            ? c.unavailable
            : "";
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="design"
      onBeforeNavigate={guard}
    >
      <div className="orders-page design-page" data-testid="design-page">
        <AdminPageHeader locale={locale} description={c.subtitle} />
        {stores.length > 1 && (
          <div className="orders-controls">
            <label>
              {c.store}
              <select
                data-testid="store-selector"
                value={store?.id ?? ""}
                onChange={(event) => {
                  if (guard())
                    window.location.assign(
                      `/${locale}/design?store=${event.target.value}`,
                    );
                }}
              >
                {stores.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
        )}
        {read.status === "loading" && (
          <p className="orders-message" role="status">
            {c.loading}
          </p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>
              {c.retry}
            </button>
          </div>
        )}
        {read.status === "ready" && read.data && store && (
          <Editor
            key={read.boundary + read.data.draft.version}
            store={store.id}
            locale={locale}
            initial={read.data}
            boundary={read.boundary}
            dirtyRef={dirtyRef}
            c={c}
            reload={read.reload}
          />
        )}
      </div>
    </WorkspaceFrame>
  );
}

const tabOf = (path: string): Tab =>
  path.startsWith("nav")
    ? "nav"
    : path.startsWith("home")
      ? "home"
      : path.startsWith("pages")
        ? "pages"
        : "profile";

function Editor({
  store,
  locale,
  initial,
  boundary,
  dirtyRef,
  c,
  reload,
}: {
  store: string;
  locale: Locale;
  initial: Loaded;
  boundary: string;
  dirtyRef: { current: boolean };
  c: DesignCopy;
  reload: () => void;
}) {
  const [doc, setDocState] = useState<DesignDocument>(initial.draft.document);
  const [saved, setSaved] = useState(() =>
    JSON.stringify(cleanForSave(initial.draft.document)),
  );
  const [base, setBase] = useState(initial.draft.version);
  const [live, setLive] = useState(initial.draft.published_version);
  const [media, setMedia] = useState(initial.media);
  const [versions, setVersions] = useState(initial.versions);
  const [tab, setTab] = useState<Tab>("profile");
  const [busy, setBusy] = useState<
    "" | "save" | "publish" | "preview" | "rollback"
  >("");
  const [note, setNote] = useState<Note>(null);
  const [stale, setStale] = useState(false);
  const [server, setServer] = useState<{ path: string; reason: string } | null>(
    null,
  );

  const snapshot = useMemo(() => JSON.stringify(cleanForSave(doc)), [doc]);
  const dirty = snapshot !== saved;
  const issues = useMemo(() => localIssues(doc), [doc]);
  const local = useMemo(
    () => new Map(issues.map((i) => [i.path, c.issue[i.reason]])),
    [issues, c],
  );
  const issue = (path: string) =>
    local.get(path) ?? (server?.path === path ? server.reason : "");
  const setDoc = (next: DesignDocument) => {
    setServer(null);
    setDocState(next);
  };

  useEffect(() => {
    dirtyRef.current = dirty;
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault(); // the browser shows its own generic prompt
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty, dirtyRef]);
  useEffect(
    () => () => {
      dirtyRef.current = false;
    },
    [dirtyRef],
  );

  function fail(r: Extract<Outcome<unknown>, { ok: false }>) {
    if (r.path) {
      setServer({ path: r.path, reason: r.reason });
      setTab(tabOf(r.path));
      setNote({
        kind: "error",
        text: fill(c.invalidAt, { path: r.path, reason: r.reason }),
      });
    } else
      setNote({ kind: "error", text: c.errors[r.code] ?? c.errors.default });
    if (r.code === "conflict") setStale(true);
  }

  async function save(): Promise<number | null> {
    if (issues.length) {
      setNote({ kind: "error", text: c.needsSave });
      return null;
    }
    setBusy("save");
    const r = await saveDraft(store, base, cleanForSave(doc), boundary);
    setBusy("");
    if (!r.ok) {
      fail(r);
      return null;
    }
    setDocState(r.value.document);
    setSaved(JSON.stringify(cleanForSave(r.value.document)));
    setBase(r.value.version);
    setLive(r.value.published_version);
    setStale(false);
    setNote({ kind: "ok", text: c.okSaved });
    return r.value.version;
  }

  async function refreshVersions() {
    try {
      setVersions(await readVersions(store, new AbortController().signal));
    } catch {
      /* the next page load shows it; the publish itself already succeeded */
    }
  }

  async function doPublish() {
    if (busy) return;
    let version = base;
    if (dirty || base === 0) {
      const saved = await save();
      if (saved === null) return;
      version = saved;
    }
    if (!window.confirm(fill(c.publishConfirm, { n: version }))) return;
    setBusy("publish");
    const r = await publish(store, version, boundary);
    setBusy("");
    if (!r.ok) return fail(r);
    setLive(r.value.version);
    setNote({ kind: "ok", text: fill(c.okPublished, { n: r.value.version }) });
    await refreshVersions();
  }

  async function doPreview() {
    if (busy) return;
    // The tab must be opened inside the click; the address is filled in after the saves/requests below.
    const tabWindow = window.open("", "_blank");
    if (!tabWindow) return setNote({ kind: "error", text: c.previewBlocked });
    const abandon = (text: string) => {
      tabWindow.close();
      setNote({ kind: "error", text });
    };
    if (dirty || base === 0) {
      if ((await save()) === null) return tabWindow.close();
    }
    setBusy("preview");
    try {
      // GET storefront (integration:read): the active origin(s) of this store; the token is bound to the SAVED draft.
      const state = await readStorefront(store, new AbortController().signal);
      const origin = (state.domains.find((d) => d.serving) ?? state.domains[0])
        ?.origin;
      if (!origin) return abandon(c.previewNoOrigin);
      const token = await issuePreview(store, boundary);
      if (!token.ok) {
        tabWindow.close();
        return fail(token);
      }
      tabWindow.opener = null;
      tabWindow.location.href = `${origin}/?preview=${encodeURIComponent(token.value.token)}`;
      setNote({ kind: "ok", text: c.previewOpened });
    } catch {
      abandon(c.errors.retry_later);
    } finally {
      setBusy("");
    }
  }

  async function doRollback(version: number) {
    if (
      busy ||
      !window.confirm(fill(c.versions.rollbackConfirm, { n: version }))
    )
      return;
    setBusy("rollback");
    const r = await rollback(store, version, boundary);
    setBusy("");
    if (!r.ok) return fail(r);
    setLive(r.value.version);
    setNote({ kind: "ok", text: fill(c.okRolledBack, { n: r.value.version }) });
    await refreshVersions();
  }

  const ops: MediaOps = {
    async upload(file) {
      const problem = mediaFileProblem(file);
      if (problem) {
        setNote({
          kind: "error",
          text: problem === "type" ? c.media.badType : c.media.badSize,
        });
        return null;
      }
      const r = await uploadMedia(store, file, boundary);
      if (!r.ok) {
        setNote({
          kind: "error",
          text:
            r.code === "conflict"
              ? c.media.full
              : r.code === "invalid_request"
                ? c.media.badType
                : (c.errors[r.code] ?? c.errors.default),
        });
        return null;
      }
      setMedia((list) =>
        list.some((m) => m.id === r.value.id) ? list : [r.value, ...list],
      );
      return r.value.id;
    },
    async remove(id) {
      // The server only knows the SAVED draft and live version; an unsaved reference must also block the delete.
      if (snapshot.includes(id))
        return setNote({ kind: "error", text: c.media.inUse });
      const r = await deleteMedia(store, id, boundary);
      if (r.ok) setMedia(r.value);
      else
        setNote({
          kind: "error",
          text:
            r.code === "conflict"
              ? c.media.inUse
              : (c.errors[r.code] ?? c.errors.default),
        });
    },
  };

  // The newest published row is a plain publish of exactly this saved draft: nothing new to publish (Go answers 409 too).
  const liveIsDraft =
    !dirty &&
    base !== 0 &&
    versions.items[0]?.kind === "publish" &&
    versions.items[0].source_version === base;
  const status = dirty ? c.unsaved : base === 0 ? c.noDraft : c.saved;
  const p = catalogPresentationCopy[locale];
  const saveReason = busy ? p.busy : !dirty && base !== 0 ? p.noChanges : "";
  const publishReason = busy ? p.busy : liveIsDraft ? p.alreadyPublished : "";
  return (
    <div className="design-editor">
      <div className="design-toolbar" data-testid="design-toolbar">
        <div
          className="design-status"
          role="status"
          data-testid="design-status"
        >
          <Badge tone={dirty ? "warning" : base === 0 ? "neutral" : "success"}>
            {status}
          </Badge>
          <span>
            {base === 0 ? c.noDraft : fill(c.draftVersion, { n: base })}
          </span>
          <span>
            {live === null
              ? c.neverPublished
              : fill(c.liveVersion, { n: live })}
          </span>
        </div>
        <div className="design-actions">
          <button
            type="button"
            onClick={() => void save()}
            disabled={!!busy || (!dirty && base !== 0)}
            aria-describedby={saveReason ? "design-save-reason" : undefined}
            data-testid="design-save"
          >
            {busy === "save" ? c.saving : c.save}
          </button>
          <button
            type="button"
            onClick={() => void doPreview()}
            disabled={!!busy}
            aria-describedby={busy ? "design-save-reason" : undefined}
            data-testid="design-preview"
          >
            {c.preview}
          </button>
          <button
            type="button"
            className="design-primary"
            onClick={() => void doPublish()}
            disabled={!!busy || liveIsDraft}
            aria-describedby={
              publishReason
                ? publishReason === saveReason
                  ? "design-save-reason"
                  : "design-publish-reason"
                : undefined
            }
            data-testid="design-publish"
          >
            {busy === "publish" ? c.publishing : c.publish}
          </button>
        </div>
      </div>
      {(saveReason || publishReason) && (
        <div className="design-disabled-reasons">
          {saveReason && <p id="design-save-reason">{saveReason}</p>}
          {publishReason && publishReason !== saveReason && (
            <p id="design-publish-reason">{publishReason}</p>
          )}
        </div>
      )}
      {note && (
        <p
          className={`design-note ${note.kind}`}
          role={note.kind === "error" ? "alert" : "status"}
          data-testid="design-note"
        >
          {note.text}
        </p>
      )}
      {stale && (
        <p className="design-note error">
          <button type="button" onClick={reload} data-testid="design-reload">
            {c.reload}
          </button>
        </p>
      )}
      <TabStrip
        label={c.title}
        previousLabel={p.previousTabs}
        nextLabel={p.nextTabs}
      >
        <div className="design-tabs" role="tablist" aria-label={c.title}>
          {TABS.map((id) => {
            const count =
              id === "versions"
                ? 0
                : issues.filter((i) => tabOf(i.path) === id).length;
            return (
              <button
                key={id}
                type="button"
                role="tab"
                id={`design-tab-${id}`}
                aria-selected={tab === id}
                aria-controls="design-tabpanel"
                className={tab === id ? "active" : undefined}
                onClick={() => setTab(id)}
                data-testid={`design-tab-${id}`}
              >
                {c.tabs[id]}
                {count ? (
                  <Badge
                    tone="danger"
                    className="design-badge"
                    aria-label={String(count)}
                  >
                    {count}
                  </Badge>
                ) : null}
              </button>
            );
          })}
        </div>
      </TabStrip>
      <fieldset
        className="design-body"
        id="design-tabpanel"
        role="tabpanel"
        aria-labelledby={`design-tab-${tab}`}
        disabled={busy === "save" || busy === "preview"}
      >
        {tab === "profile" && (
          <DesignProfile
            store={store}
            profile={doc.profile}
            media={media}
            ops={ops}
            c={c}
            issue={issue}
            onChange={(profile) => setDoc({ ...doc, profile })}
          />
        )}
        {tab === "nav" && (
          <DesignNav
            nav={doc.nav}
            pages={doc.pages}
            c={c}
            issue={issue}
            onChange={(nav) => setDoc({ ...doc, nav })}
          />
        )}
        {tab === "home" && (
          <DesignSections
            store={store}
            sections={doc.home.sections}
            pages={doc.pages}
            media={media}
            ops={ops}
            c={c}
            issue={issue}
            onChange={(sections) => setDoc({ ...doc, home: { sections } })}
          />
        )}
        {tab === "pages" && (
          <DesignPages
            pages={doc.pages}
            c={c}
            issue={issue}
            onChange={(pages) => setDoc({ ...doc, pages })}
          />
        )}
        {tab === "versions" && (
          <Versions
            list={versions}
            live={live}
            busy={busy === "rollback"}
            c={c}
            locale={locale}
            onRollback={(v) => void doRollback(v)}
          />
        )}
      </fieldset>
    </div>
  );
}

function Versions({
  list,
  live,
  busy,
  c,
  locale,
  onRollback,
}: {
  list: VersionList;
  live: number | null;
  busy: boolean;
  c: DesignCopy;
  locale: Locale;
  onRollback: (version: number) => void;
}) {
  const when = new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  });
  return (
    <div className="design-panel" data-testid="design-versions">
      <p className="design-muted">{c.versions.help}</p>
      {list.items.length === 0 && <p>{c.versions.none}</p>}
      {list.items.length > 0 && (
        <TableFrame
          label={c.tabs.versions}
          scrollHint={catalogPresentationCopy[locale].tableScroll}
        >
          <table className="design-table">
            <thead>
              <tr>
                <th>{c.versions.version}</th>
                <th>{c.versions.kind.publish}</th>
                <th>{c.versions.at}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {list.items.map((v) => (
                <tr key={v.version} data-testid="design-version-row">
                  <td>
                    v{v.version}
                    {v.version === live && (
                      <Badge tone="info" className="design-live">
                        {c.versions.live}
                      </Badge>
                    )}
                  </td>
                  <td>
                    {c.versions.kind[v.kind]}
                    {v.kind === "rollback"
                      ? ` · ${fill(c.versions.source, { n: v.source_version })}`
                      : ""}
                  </td>
                  <td>{when.format(new Date(v.published_at))}</td>
                  <td>
                    {v.version !== live && (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => onRollback(v.version)}
                        data-testid="design-rollback"
                      >
                        {c.versions.rollback}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableFrame>
      )}
    </div>
  );
}
