// Purpose: Owns storefront design draft editing, publication and rollback controls.
// Depends on: @live-commerce/format (Taipei store timestamps), react, @live-commerce/i18n, @live-commerce/ui, @/lib/catalog-v2-copy, @/lib/model, @/lib/customers-client, @/lib/settings-client, @/lib/storefront-client, @/lib/design-client, @/lib/design-copy, @/lib/design-model, @/lib/use-design-load, ./WorkspaceFrame, ./AdminPageHeader, ./DesignProfile, ./DesignNav, ./DesignSections, ./DesignPages, ./DesignVersions, ./DesignSummary, ./DesignDomains, ./DesignMedia, ./orders.css, ./design.css
// Used by: apps/admin/app/[locale]/design/page.tsx
"use client";

// Merchant store-design page (/{locale}/design): store profile, header/footer navigation, home sections, information
// pages, Save draft / Preview / Publish and published versions with Rollback (unit store-design,
// contracts/storefront-v2.md section B).
// BFF (browser, via lib/design-client.ts): GET|PUT /api/stores/{store}/design/draft, POST .../design/publish, .../rollback,
// .../preview-token, GET .../design/versions, GET|POST .../design/media, GET .../design/media/{id}, POST
// .../design/media/{id}/delete, plus GET .../storefront (the active address, to open the preview) -> Go
// /v1/admin/stores/{id}/design/* (internal/httpapi/design.go; integration:read / integration:manage).
// Invariants: pending publication is armed durably before send and resolved only by its original key/body.
// Owns: the editable copy of the document, the dirty/unsaved guard, the local "what is still missing" hints and the
// request choreography (save before publish/preview, compare-and-set versions). It never validates what the server
// owns (the server's 422 path is shown on the field) and never keeps the preview token (it only opens the link).
// Unlike the PII pages it does NOT drop its state when the tab is hidden: unsaved edits must survive a tab switch; every
// write is still fenced by the session boundary.
import { useEffect, useMemo, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import { Badge } from "@live-commerce/ui";
import { catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import type { Store } from "@/lib/model";
import type { ReadCode } from "@/lib/customers-client";
import { useDesignLoad, type Loaded } from "@/lib/use-design-load";
import { sessionBoundary } from "@/lib/settings-client";
import { readStorefront } from "@/lib/storefront-client";
import {
  deleteMedia,
  issuePreview,
  mediaFileProblem,
  publish,
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
  type VersionList,
} from "@/lib/design-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { DesignProfile } from "./DesignProfile";
import { DesignNav } from "./DesignNav";
import { DesignSections } from "./DesignSections";
import { DesignPages } from "./DesignPages";
import { Versions } from "./DesignVersions";
import { DesignSummary } from "./DesignSummary";
import { DesignDomains } from "./DesignDomains";
import type { MediaOps } from "./DesignMedia";
import "./orders.css";
import "./design.css";

type Tab = "hub" | "profile" | "nav" | "home" | "pages" | "versions" | "domains" | "review" | "result" | "published";
type Note = { kind: "ok" | "error"; text: string } | null;

type Pending = { version: number; key: string; boundary: string; retries: number };
const pendingKey = (store: string) => `lc.design.publishPending.${store}`;
function loadPending(store: string): Pending | null {
  try {
    const v = JSON.parse(sessionStorage.getItem(pendingKey(store)) ?? "null") as Partial<Pending> | null;
    return v && Number.isSafeInteger(v.version) && Number(v.version) > 0 && typeof v.key === "string" && /^[A-Za-z0-9_.:-]{8,128}$/.test(v.key) && typeof v.boundary === "string" && Number.isInteger(v.retries) && Number(v.retries) >= 0 && Number(v.retries) <= 3 ? (v as Pending) : null;
  } catch {
    return null; // storage can be blocked: the page then simply behaves as before (in-memory only)
  }
}
function storePending(store: string, next: Pending | null) {
  try {
    if (next) sessionStorage.setItem(pendingKey(store), JSON.stringify(next));
    else sessionStorage.removeItem(pendingKey(store));
  } catch {
    /* in-memory state still holds */
  }
}

/** Owns storefront design draft editing, publication and rollback controls. User actions submit draft, publication, rollback and media commands through design-client. */
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
  const read = useDesignLoad(store?.id ?? null, initialError, renderKey);
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
        <AdminPageHeader locale={locale} title={c.editor.title} />
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
  const [tab, setTab] = useState<Tab>("hub");
  const [address, setAddress] = useState('');
  const [receipt, setReceipt] = useState<VersionList['items'][number] | null>(null);
  const [publishedDoc, setPublishedDoc] = useState<DesignDocument | null>(null);
  const [resultError, setResultError] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    let current = true;
    void readStorefront(store, controller.signal).then(async state => {
      if (current && await sessionBoundary() === boundary) setAddress(state.domains.find(d => d.serving)?.origin ?? '');
    }).catch(() => { /* domain readiness never guesses or blocks document editing */ });
    return () => { current = false; controller.abort(); };
  }, [store,boundary]);
  const [busy, setBusy] = useState<
    "" | "save" | "publish" | "preview" | "rollback"
  >("");
  const [note, setNote] = useState<Note>(null);
  // R3-R5: persist before sending; only an identical keyed POST can resolve this attempt.
  const [pending, setPendingState] = useState<Pending | null>(null);
  const pendingRef = useRef<Pending | null>(null);
  const resolving = useRef(false);
  const mounted = useRef(true);
  const [rechecking, setRechecking] = useState(false);
  const [unconfirmed, setUnconfirmed] = useState(false);
  const [integrity, setIntegrity] = useState(false);
  const [superseded, setSuperseded] = useState(false);
  const setPending = (next: Pending | null) => {
    pendingRef.current = next;
    setPendingState(next);
    storePending(store, next);
  };
  useEffect(() => {
    mounted.current = true;
    const restored = loadPending(store);
    if (restored) {
      setPending(restored);
      // A session fence mismatch must never replay a previous merchant's request.
      if (restored.boundary === boundary) void recheckPublish(true);
    }
    return () => { mounted.current = false; };
  }, [store, boundary]);
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
    if (!dirty && !pending) return; // an unknown publish outcome also warns on leave (the browser's own generic prompt)
    const warn = (event: BeforeUnloadEvent) => event.preventDefault(); // the browser shows its own generic prompt
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty, pending, dirtyRef]);
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
      const list = await readVersions(store, new AbortController().signal);
      setVersions(list);
    } catch {
      /* the next page load shows it; the publish itself already succeeded */
    }
  }

  async function openReview() {
    if (busy || pendingRef.current) return;
    if (dirty || base === 0) {
      if ((await save()) === null) return;
    }
    try {
      const state = await readStorefront(store, AbortSignal.timeout(5000));
      setAddress(state.domains.find(d => d.serving)?.origin ?? '');
    } catch { setAddress(''); }
    setTab('review');
    setNote(null);
  }

  async function doPublish() {
    if (busy || pendingRef.current || dirty) return;
    const version = base;
    setBusy("publish");
    setReceipt(null);
    setPublishedDoc(null);
    setResultError('');
    const attempt: Pending = { version, key: crypto.randomUUID(), boundary, retries: 0 };
    setPending(attempt); // synchronous sessionStorage write BEFORE the first POST
    setUnconfirmed(false);
    setIntegrity(false);
    setSuperseded(false);
    setNote(null);
    const r = await publish(store, version, boundary, attempt.key);
    if (!mounted.current) return;
    setBusy("");
    if (await applyPublish(r, attempt, true)) return;
    setTab("hub");
    void recheckPublish(true);
  }

  async function applyPublish(r: Outcome<VersionList["items"][number]>, attempt: Pending, initialSend = false): Promise<boolean> {
    if (!mounted.current || pendingRef.current?.key !== attempt.key) return true;
    if (!r.ok) {
      if (r.code === "idempotency_conflict") {
        setIntegrity(true);
        return true; // report integrity failure, retain the original attempt; never mint a replacement key
      }
      if (r.uncertain || r.code !== "conflict") {
        // A definite rejection of the INITIAL send has no ambiguous prior attempt.
        // Recovery refusals still retain their identity: they cannot disprove an earlier commit.
        if (!r.uncertain && initialSend) {
          setPending(null);
          setResultError(c.errors[r.code] ?? c.errors.default);
          setTab('result');
          return true;
        }
        setUnconfirmed(true);
        return !r.uncertain; // definite auth/validation refusals keep the unresolved identity without automatic replay
      }
      setPending(null);
      setNote({ kind: "error", text: c.publishChanged });
      setStale(true);
      return true;
    }
    setPending(null);
    setUnconfirmed(false);
    setIntegrity(false);
    setLive(r.value.version);
    setReceipt(r.value);
    setPublishedDoc(attempt.version === base ? JSON.parse(saved) as DesignDocument : null);
    setVersions(previous => ({ live_version: r.value.version, items: [r.value, ...previous.items.filter(v => v.version !== r.value.version)] }));
    setTab('result');
    setNote({ kind: "ok", text: fill(c.okPublished, { n: r.value.version }) });
    // The receipt establishes success. The list only supplies current display/supersession, never confirmation.
    try {
      const list = await readVersions(store, AbortSignal.timeout(5000));
      if (mounted.current) {
        setVersions(list);
        setLive(list.live_version ?? r.value.version);
        setSuperseded((list.live_version ?? r.value.version) > r.value.version);
      }
    } catch { /* the authoritative receipt remains successful even if this display refresh fails */ }
    return true;
  }

  async function recheckPublish(automatic = false) {
    let attempt = pendingRef.current;
    if (!attempt || resolving.current) return;
    if (attempt.boundary !== boundary) {
      // A new authenticated session must explicitly authorize the original intent.
      // Keep its old fence: an ambiguous answer/reload still cannot auto-replay it.
      if (automatic || !window.confirm(fill(c.publishRecoveryConfirm, { n: attempt.version }))) return;
    }
    resolving.current = true;
    setRechecking(true);
    try {
      // The automatic budget is durable too: reload cannot reset the three-resend limit. A click sends once.
      for (let i = 0; i < (automatic ? 3 : 1); i++) {
        if (automatic && attempt.retries >= 3) return;
        await new Promise<void>((resolve) => window.setTimeout(resolve, 250 * 2 ** i));
        if (!mounted.current || pendingRef.current?.key !== attempt.key) return;
        if (automatic) {
          attempt = { ...attempt, retries: attempt.retries + 1 };
          setPending(attempt);
        }
        const r = await publish(store, attempt.version, boundary, attempt.key);
        if (await applyPublish(r, attempt)) return;
      }
    } finally {
      resolving.current = false;
      if (mounted.current) setRechecking(false);
    }
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
      busy || pendingRef.current ||
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
  const publishReason = busy ? p.busy : pending ? c.publishPending : liveIsDraft ? p.alreadyPublished : "";
  return (
    <div className="design-editor" data-testid="design-screen">
      {tab !== 'hub' && <button type="button" data-testid="design-back" disabled={!!busy} onClick={() => setTab('hub')}>{c.editor.back}</button>}
      <div className="design-toolbar" data-testid="design-toolbar" hidden={['review','result','published'].includes(tab)}>
        <div
          className="design-status"
          role="status"
          data-testid="design-status"
        >
          <Badge className={!dirty && base !== 0 ? 'design-saved-badge' : undefined} tone={dirty ? "warning" : base === 0 ? "neutral" : "success"}>
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
            hidden={!['hub','profile','home','nav','pages'].includes(tab) || (tab === 'hub' && !dirty && base !== 0)}
          >
            {busy === "save" ? c.saving : c.save}
          </button>
          <button type="button" data-testid="design-cancel" hidden={!['profile','home','nav','pages'].includes(tab)} disabled={!!busy} onClick={() => { setDoc(JSON.parse(saved) as DesignDocument); setNote(null); setTab('hub'); }}>{c.editor.cancel}</button>
          <button
            type="button"
            onClick={() => void doPreview()}
            disabled={!!busy}
            aria-describedby={busy ? "design-save-reason" : undefined}
            data-testid="design-preview"
            hidden={tab !== 'hub'}
          >
            {c.preview}
          </button>
          <button
            type="button"
            className="design-primary"
            onClick={() => void openReview()}
            disabled={!!busy || liveIsDraft || !!pending}
            aria-describedby={
              publishReason
                ? publishReason === saveReason
                  ? "design-save-reason"
                  : "design-publish-reason"
                : undefined
            }
            data-testid="design-publish"
            hidden={tab !== 'hub' || !!pending || liveIsDraft}
          >
            {busy === "publish" ? c.publishing : c.publish}
          </button>
        </div>
      </div>
      {(saveReason || publishReason) && (
        <div className="design-disabled-reasons" hidden>
          {saveReason && <p id="design-save-reason">{saveReason}</p>}
          {publishReason && publishReason !== saveReason && (
            <p id="design-publish-reason">{publishReason}</p>
          )}
        </div>
      )}
      {pending && busy !== "publish" && (
        <div className="design-note warning" role="alert" data-testid="design-publish-pending">
          <p>
            <strong>{c.editor.pending}</strong>
          </p>
          <p>{fill(c.publishPendingBody, { n: pending.version })}</p>
          <p>{live === null ? c.neverPublished : fill(c.liveVersion, { n: live })}</p>
          <p>{c.editor.scope}</p><p>{c.editor.unchanged}</p>
          <p>{address || c.editor.noAddress}{address && ` · ${c.editor.available}`}</p>
          {unconfirmed && <p data-testid="design-publish-unconfirmed">{c.publishUnconfirmed}</p>}
          {integrity && <p data-testid="design-publish-integrity">{c.publishIntegrity}</p>}
          <button type="button" onClick={() => void recheckPublish()} disabled={rechecking} data-testid="design-publish-recheck">
            {rechecking ? c.publishRechecking : c.editor.query}
          </button>
          <details><summary>{c.editor.details}</summary><DesignSummary store={store} doc={JSON.parse(saved) as DesignDocument} c={c} /></details>
        </div>
      )}
      {superseded && <p className="design-note" data-testid="design-publish-superseded">{c.publishSuperseded}</p>}
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
      {tab === 'hub' && <nav className="design-task-list" data-testid="design-hub" aria-label={c.editor.title}>
        {(['pages','home','nav','profile'] as const).map(id => <button key={id} type="button" data-testid={`design-task-${id}`} onClick={() => setTab(id)}>{c.editor[id]}</button>)}
        <button type="button" data-testid="design-domains-open" onClick={() => setTab('domains')}>{c.editor.domains}</button>
        <button type="button" data-testid="design-tab-versions" onClick={() => setTab('versions')}>{c.editor.versions}</button>
      </nav>}
      {tab === 'domains' && <DesignDomains store={store} boundary={boundary} locale={locale} c={c} />}
      {tab === 'review' && <section className="design-panel" data-testid="design-publish-review">
        <h2>{fill(c.publishConfirm, { n: base })}</h2>
        <p>{live === null ? c.neverPublished : fill(c.liveVersion, { n: live })}</p>
        <p>{c.editor.scope}</p><p>{c.editor.unchanged}</p>
        <p>{address || c.editor.noAddress}{address && ` · ${c.editor.available}`}</p>
        <details><summary>{c.editor.details}</summary><DesignSummary store={store} doc={JSON.parse(saved) as DesignDocument} c={c} /></details>
        <div className="design-actions">
          <button type="button" data-testid="design-publish-continue" disabled={!!busy} onClick={() => setTab('hub')}>{c.editor.continue}</button>
          <button type="button" className="design-primary" data-testid="design-publish-confirm" disabled={!!busy || !!pending} onClick={() => void doPublish()}>{c.editor.confirm}</button>
        </div>
      </section>}
      {tab === 'result' && !pending && <section className="design-panel" data-testid="design-publish-result">
        <h2>{resultError ? c.editor.failed : c.editor.published}</h2>
        <p>{fill(c.draftVersion, { n: receipt?.source_version ?? base })} · {live === null ? c.neverPublished : fill(c.liveVersion, { n: live })}</p>
        {resultError ? <p role="alert">{resultError}</p> : receipt && <p>{displayTime(locale, receipt.published_at)}</p>}
        <p>{c.editor.scope}</p><p>{c.editor.unchanged}</p>
        <p>{address || c.editor.noAddress}{address && ` · ${c.editor.available}`}</p>
        <details><summary>{c.editor.details}</summary><DesignSummary store={store} doc={publishedDoc ?? JSON.parse(saved) as DesignDocument} c={c} /></details>
        {!resultError && <button type="button" data-testid="design-view-published" onClick={() => setTab('published')}>{c.editor.view}</button>}
      </section>}
      {tab === 'published' && <section className="design-panel" data-testid="design-published-content">
        <h2>{live === null ? c.neverPublished : fill(c.liveVersion, { n: live })}</h2>
        {publishedDoc && !superseded && <DesignSummary store={store} doc={publishedDoc} c={c} />}
        {address && <a href={address} target="_blank" rel="noopener noreferrer">{c.editor.visit}</a>}
      </section>}
      <fieldset
        className="design-body"
        id="design-tabpanel"
        hidden={!["profile","home","nav","pages","versions"].includes(tab)}
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
            busy={busy === "rollback" || !!pending}
            c={c}
            locale={locale}
            onRollback={(v) => void doRollback(v)}
          />
        )}
      </fieldset>
    </div>
  );
}
