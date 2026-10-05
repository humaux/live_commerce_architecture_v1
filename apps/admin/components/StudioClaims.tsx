"use client";

// Owns the Studio › Claims panel (contracts/live-keyword-claims-v1.md §11.1, T10b): claim
// window badge and Open/Close, quantity-rule selector (CLOSED only), offers table and
// form, per-reason counters with the frozen host prompt, MOCK manual comment entry,
// bundle list, and the masked one-time link dialog (issue / replace / release+replace).
// It also owns the "Comment source" section: the one pasted link/ID that binds this scene
// to a Facebook post/live video or Instagram media, its save (version CAS, Idempotency-Key
// reused only by "retry same request") and its status line. Go parses and validates the
// pasted text; the UI never decides what a valid source is.
// Non-goals: no claim rule (Go decides every outcome; the UI shows results, never
// predicts them), no comment reading of its own (Go's Meta intake reads the bound post;
// the banner only reports whether a source is active), no storage of any
// response, label or token (browser memory only; the token is dropped when the dialog
// closes, the page hides or the session ends), no link delivery (the merchant copies it).
// BFF routes: /api/stores/{store}/live-sessions/{scene}/claims… (M1–M7, Go claims.go) and
// …/claim-source (GET read + enabled Meta platforms, PUT bind with the optional platform hint;
// Go live.put_claim_source, live:manage + integration:execute).
// Depends on: claims-client.ts (M1–M7 and catalog reads), studio-client.ts (scene detail
// for its title and can_manage), settings-client.ts (session boundary), claims-copy.ts
// plus studio-copy.ts (shared Studio words), WorkspaceFrame (shared admin shell) and
// claims.css (surface-local layout only).

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { locales, localeNames, type Locale } from "@live-commerce/i18n";
import { Badge, Field as SharedField, FormRow, TableFrame } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { sessionBoundary } from "@/lib/settings-client";
import { readStudioDetail, readStudioPage, StudioError, type StudioErrorCode } from "@/lib/studio-client";
import type { StudioDetail } from "@/lib/studio-model";
import { money } from "@/lib/client";
import {
  createClaimOffer, importClaimOffers, issueClaimLink, putClaimSource, readClaimBundles, readClaimProducts, readClaimSKUs,
  readClaimsBoard, readClaimSource, readKeywordLibrary, readStorefrontOrigin, recordManualClaim, setClaimWindow,
  setLibraryKeyword, updateClaimOffer,
} from "@/lib/claims-client";
import type { ClaimSource, SourcePlatform } from "@/lib/claim-source-model";
import { claimSourceBody, claimSourceInputMax, validClaimSourceInput, type ClaimSourceForm } from "@/lib/claims-request";
import {
  parseLivePrice, persistedReasons, type Board, type Bundle, type CatalogProduct, type CatalogSKU,
  type ImportResult, type LibraryEntry, type ManualResult, type MatchMode, type Offer,
} from "@/lib/claims-model";
import { fromMinor } from "@/lib/catalog-v2-model";
import { displayClock, displayTime } from "@/lib/orders-model";
import { claimLinkMessage, claimsCopy, hostPrompt } from "@/lib/claims-copy";
import { studioCopy } from "@/lib/studio-copy";
import { metaConnectCopy } from "@/lib/meta-connect-copy";
import { readStatus as readMetaStatus, type ConnectStatus } from "@/lib/meta-connect-client";
import { OrderReadError } from "@/lib/orders-client";
import { pageSourceInput } from "@/lib/meta-page-source";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "./claims.css";

type Status = "loading" | "ready" | StudioErrorCode;
type Action = "window" | "offer" | "update" | "manual" | "link" | "source" | "import" | "library";
type ActionError = { action: Action; code: StudioErrorCode | "no-origin" | "form"; message?: string; api?: string };
type Pending = { action: Action; key: string; run: (key: string, boundary: string) => Promise<void> };
// source is read on its own: a failing source read (sourceError) never hides the rest of the panel.
// library is read on its own too (null = it could not be read); scenes feed "copy offers from".
type Facts = { detail: StudioDetail; board: Board; bundles: Bundle[]; next: string; source: ClaimSource | null; platforms: SourcePlatform[]; sourceError: StudioErrorCode | null; library: LibraryEntry[] | null; meta: ConnectStatus | null };
type Issued = { ref: string; origin: string; token: string | null; generation: number; expiresAt: string; released: boolean; replayed: boolean };

// A label beside (not around) its control keeps the accessible name exactly the label
// text, so a select's option text never leaks into it.
function Field({ id, label, hint, children }: { id: string; label: string; hint?: string; children: ReactNode }) {
  return <SharedField id={id} label={label} hint={hint} width="full" className="claims-field">{children}</SharedField>;
}
const codeOf = (error: unknown): StudioErrorCode => error instanceof StudioError ? error.code : "unavailable";
// The form the saved source (or the defaults of a first bind: collecting, reply off) would produce.
// Private reply defaults to off: it messages real buyers, so the merchant opts in (ruling t).
// Re-saving keeps the saved source's platform (ruling p), so its own id never becomes ambiguous.
const sourceFormOf = (source: ClaimSource | null, locale: Locale): ClaimSourceForm => source
  ? { input: source.source_object_id, private_reply: source.private_reply, reply_locale: source.reply_locale, active: source.active, platform: source.platform }
  : { input: "", private_reply: false, reply_locale: locale, active: true, platform: "" };
const utf8Bytes = (value: string) => new TextEncoder().encode(value).length;

export function StudioClaims({ locale, store, scene, initialError }: {
  locale: Locale; store: Store | null; scene: string; initialError: StudioErrorCode | null;
}) {
  const c = claimsCopy[locale];
  const shared = studioCopy[locale];
  const storeID = store?.id ?? "";
  const [status, setStatus] = useState<Status>(initialError ?? "loading");
  const [facts, setFacts] = useState<Facts | null>(null);
  const [products, setProducts] = useState<CatalogProduct[] | null>(null);
  const [catalogError, setCatalogError] = useState(false);
  const [skus, setSKUs] = useState<CatalogSKU[]>([]);
  const [offerForm, setOfferForm] = useState({ keyword: "", product: "", sku: "", max: "3", live: "" });
  const [limits, setLimits] = useState<Record<string, string>>({});
  // Live tools (R4): live-price edits per offer, library keyword edits per SKU, copy-from scene, last import outcome.
  const [prices, setPrices] = useState<Record<string, string>>({});
  const [libraryEdits, setLibraryEdits] = useState<Record<string, string>>({});
  const [scenes, setScenes] = useState<{ id: string; title: string }[]>([]);
  const [copyFrom, setCopyFrom] = useState("");
  const [imported, setImported] = useState<{ result: ImportResult; names: Record<string, string> } | null>(null);
  const [manual, setManual] = useState({ bundle: "", label: "", text: "" });
  const [result, setResult] = useState<ManualResult | null>(null);
  const [modeDraft, setModeDraft] = useState<MatchMode>("EXACT");
  const [promptLanguage, setPromptLanguage] = useState<Locale>(locale);
  const [promptKeyword, setPromptKeyword] = useState("");
  const [copied, setCopied] = useState("");
  const [busy, setBusy] = useState<Action | null>(null);
  const [refreshing, setRefreshing] = useState(false); // "Refresh facts" is re-reading; refreshedAt = when the last user refresh finished (D03)
  const [refreshedAt, setRefreshedAt] = useState<number | null>(null);
  const [actionError, setActionError] = useState<ActionError | null>(null);
  const [issued, setIssued] = useState<Issued | null>(null);
  const [buyerLocale, setBuyerLocale] = useState<Locale>(locale);
  const [revealed, setRevealed] = useState(false);
  const [sourceForm, setSourceForm] = useState<ClaimSourceForm>(() => sourceFormOf(null, locale));
  const [sourcePage, setSourcePage] = useState("");
  const sourceDirty = useRef(false);
  const boundary = useRef("");
  const pending = useRef<Pending | null>(null);
  const origin = useRef<string | null>(null);
  const epoch = useRef(0);
  const controller = useRef<AbortController | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const canManage = facts?.detail.can_manage === true;

  const signOut = useCallback(() => {
    epoch.current++;
    controller.current?.abort();
    boundary.current = "";
    pending.current = null;
    setIssued(null);
    setFacts(null);
    setStatus("signed-out");
  }, []);

  // Resolves true once the facts of this call were applied (false: nothing to read, superseded by a newer read, or failed).
  const load = useCallback(async (): Promise<boolean> => {
    if (initialError || !storeID || !scene) return false;
    const current = ++epoch.current;
    controller.current?.abort();
    const abort = new AbortController();
    controller.current = abort;
    setStatus((value) => value === "ready" ? value : "loading");
    try {
      const before = await sessionBoundary().catch(() => { throw new StudioError("signed-out"); });
      const [detail, board, page, library, source, meta] = await Promise.all([readStudioDetail(storeID, scene, abort.signal),
        readClaimsBoard(storeID, scene, abort.signal), readClaimBundles(storeID, scene, "", abort.signal),
        readKeywordLibrary(storeID, scene, abort.signal).catch((error) => { if (codeOf(error) === "signed-out") throw error; return null; }),
        readClaimSource(storeID, scene, abort.signal).then((value) => ({ value, error: null as StudioErrorCode | null }),
          (error) => { if (codeOf(error) === "signed-out") throw error; return { value: { source: null, platforms: [] as SourcePlatform[] }, error: codeOf(error) as StudioErrorCode | null }; }),
        readMetaStatus(storeID, abort.signal).catch((error) => {
          if (error instanceof OrderReadError && error.code === "signed-out") throw new StudioError("signed-out");
          return null;
        })]);
      if (current !== epoch.current || abort.signal.aborted) return false;
      if ((await sessionBoundary().catch(() => "")) !== before) throw new StudioError("signed-out");
      boundary.current = before;
      setFacts({ detail, board, bundles: page.items, next: page.next_cursor, source: source.value.source, platforms: source.value.platforms, sourceError: source.error, library, meta });
      // Other saves re-read the facts too; never overwrite what the merchant is still typing.
      if (!sourceDirty.current) {
        setSourceForm(sourceFormOf(source.value.source, locale));
        setSourcePage(source.value.source?.platform === "facebook" && meta?.pages.some((p) => p.id === source.value.source?.asset_id) ? source.value.source.asset_id : "");
      }
      setModeDraft(board.window.match_mode);
      setStatus("ready");
      return true;
    } catch (error) {
      if (current !== epoch.current || abort.signal.aborted) return false;
      const code = codeOf(error);
      if (code === "signed-out") signOut();
      else setStatus(code);
      return false;
    }
  }, [initialError, storeID, scene, signOut, locale]);

  useEffect(() => {
    void load();
    return () => { epoch.current++; controller.current?.abort(); };
  }, [load]);

  useEffect(() => {
    if (initialError || !storeID) return;
    const abort = new AbortController();
    readClaimProducts(storeID, abort.signal).then(setProducts).catch(() => { if (!abort.signal.aborted) setCatalogError(true); });
    return () => abort.abort();
  }, [initialError, storeID]);

  // Other scenes of the store (first page) for "copy offers from".
  useEffect(() => {
    if (initialError || !storeID) return;
    const abort = new AbortController();
    readStudioPage(storeID, "", abort.signal)
      .then((page) => setScenes(page.items.filter((item) => item.session_id !== scene).map((item) => ({ id: item.session_id, title: item.title }))))
      .catch(() => { /* the copy control just stays empty; the rest of the panel is unaffected */ });
    return () => abort.abort();
  }, [initialError, storeID, scene]);

  useEffect(() => {
    setSKUs([]);
    if (!offerForm.product || !storeID) return;
    const abort = new AbortController();
    readClaimSKUs(storeID, offerForm.product, abort.signal).then(setSKUs).catch(() => { if (!abort.signal.aborted) setCatalogError(true); });
    return () => abort.abort();
  }, [offerForm.product, storeID]);

  // A logout anywhere, a changed session cookie, or leaving the page ends every secret.
  useEffect(() => {
    const onMessage = (event: MessageEvent) => { if (event.data?.type === "logout") signOut(); };
    const onStorage = (event: StorageEvent) => { if (event.key === "commerce-session-logout") signOut(); };
    const onFocus = () => {
      if (!boundary.current) return;
      void sessionBoundary().then((value) => { if (value !== boundary.current) signOut(); }).catch(signOut);
    };
    const onVisibility = () => { if (document.visibilityState === "hidden") setRevealed(false); };
    const onPageHide = () => setIssued(null);
    let channel: BroadcastChannel | null = null;
    try { channel = new BroadcastChannel("commerce-session"); channel.addEventListener("message", onMessage); }
    catch { /* storage and focus fences remain */ }
    window.addEventListener("storage", onStorage);
    window.addEventListener("commerce-session-logout", signOut);
    window.addEventListener("focus", onFocus);
    window.addEventListener("pagehide", onPageHide);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      channel?.close();
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("commerce-session-logout", signOut);
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("pagehide", onPageHide);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [signOut]);

  useEffect(() => {
    if (issued && dialog.current && !dialog.current.open) dialog.current.showModal();
    if (!issued) {
      setRevealed(false);
      if (dialog.current?.open) dialog.current.close();
    }
  }, [issued]);

  // One fresh Idempotency-Key per submit; only "retry same request" after an unknown
  // result reuses the pending key and body.
  async function perform(action: Action, run: Pending["run"], reuse?: Pending) {
    if (busy || !boundary.current) return;
    const request = reuse ?? { action, key: crypto.randomUUID(), run };
    pending.current = request;
    setBusy(action);
    setActionError(null);
    try {
      await request.run(request.key, boundary.current);
      pending.current = null;
      await load();
    } catch (error) {
      const code = codeOf(error);
      if (code !== "uncertain") pending.current = null;
      if (code === "signed-out") signOut();
      else setActionError({ action, code, api: error instanceof StudioError ? error.api : "" });
    } finally { setBusy(null); }
  }

  const statusText = (value: StudioErrorCode | "loading") => value === "signed-out" ? c.signedOut
    : value === "forbidden" ? c.forbidden : value === "not-found" ? c.notFound : value === "loading" ? c.loading
    : value === "invalid" ? c.invalid : c.unavailable;
  function errorText(error: ActionError) {
    if (error.message) return error.message;
    if (error.action === "source") {
      const known = (c.sourceErrors as Record<string, string>)[error.api ?? ""];
      if (known) return known;
      if (error.code === "forbidden") return c.sourceForbidden;
    }
    if (error.code === "no-origin") return c.noOrigin;
    if (error.code === "conflict") return c.conflict[error.action];
    if (error.code === "uncertain") return c.uncertain;
    return statusText(error.code === "form" ? "invalid" : error.code);
  }
  // U6 (customers-billing-v1 §5): a window open refused by billing (Go PT412 -> 402 `billing_restricted`, api code kept by
  // studio-client) says why and links to billing; nothing else in Studio is affected by standing.
  const restricted = (error: ActionError) => error.action === "window" && error.api === "billing_restricted";
  const alert = (action: Action) => actionError?.action === action && <div role="alert" className="claims-alert">
    <p>{restricted(actionError) ? c.billingRestricted : errorText(actionError)}</p>
    {restricted(actionError) && <p><a href={`/${locale}/billing${storeID ? `?store=${storeID}` : ""}`} data-testid="claims-billing-link">{c.billingLink}</a></p>}
    {actionError.code === "uncertain" && pending.current?.action === action && <button type="button" disabled={!!busy}
      onClick={() => { const value = pending.current; if (value) void perform(value.action, value.run, value); }}>{shared.retrySame}</button>}
  </div>;

  const board = facts?.board;
  const claimWindow = board?.window;
  const activeOffers = board?.offers.filter((offer) => offer.active) ?? [];
  const keywordForPrompt = activeOffers.some((offer) => offer.keyword === promptKeyword) ? promptKeyword : activeOffers[0]?.keyword ?? "";
  const prompt = keywordForPrompt && claimWindow ? hostPrompt(promptLanguage, claimWindow.match_mode, keywordForPrompt) : "";
  const blocked = !canManage || !!busy || pending.current !== null;

  const source = facts?.source ?? null;
  const sourceSaved = sourceFormOf(source, locale);
  const sourceChanged = !source || sourceForm.input.trim() !== sourceSaved.input || sourceForm.private_reply !== sourceSaved.private_reply ||
    sourceForm.reply_locale !== sourceSaved.reply_locale || sourceForm.active !== sourceSaved.active || sourceForm.platform !== sourceSaved.platform ||
    (!!sourcePage && sourcePage !== source?.asset_id);
  // Ruling p: the platform select exists only when the store has both an enabled Facebook and Instagram binding.
  const platformChoice = facts?.platforms.length === 2;
  // Ruling t: the banner says whether comments are read automatically (active source) or how to start.
  const feed = facts && !facts.sourceError
    ? source?.active ? (source.verified ? c.feedBound : `${c.feedBound} · ${c.sourceUnverified}`) : c.feedNone
    : null;
  const sourceLocked = blocked || !!facts?.sourceError;
  const editSource = (patch: Partial<ClaimSourceForm>) => {
    sourceDirty.current = true;
    setSourceForm((form) => ({ ...form, ...patch }));
    if (actionError?.action === "source" && actionError.code === "form") setActionError(null);
  };
  function saveSource() {
    if (!validClaimSourceInput(sourceForm.input)) return setActionError({ action: "source", code: "form", message: c.sourceInputInvalid });
    // expected_version 0 = first bind; a later save must match the version this panel read.
    // Without the select, only the saved source's own id carries its platform; a new paste lets Go decide.
    const page = sourcePage ? facts?.meta?.pages.find((p) => p.id === sourcePage && p.status === "active") : null;
    const input = sourcePage ? page && pageSourceInput(sourceForm.input, page.id) : sourceForm.input;
    if (!input) return setActionError({ action: "source", code: "form", message: metaConnectCopy[locale].studioPageMismatch });
    const platform = sourcePage ? "facebook" : platformChoice ? sourceForm.platform : source && sourceForm.input.trim() === source.source_object_id ? source.platform : "";
    const body = claimSourceBody({ ...sourceForm, input, platform }, source?.version ?? 0);
    void perform("source", async (key, current) => { await putClaimSource(storeID, scene, body, key, current); sourceDirty.current = false; });
  }
  function toggleWindow() {
    if (!claimWindow) return;
    const open = claimWindow.state === "CLOSED";
    const body = { expected_version: claimWindow.version, state: open ? "OPEN" as const : "CLOSED" as const,
      match_mode: claimWindow.version === 0 ? modeDraft : claimWindow.match_mode };
    void perform("window", async (key, current) => { await setClaimWindow(storeID, scene, body, key, current); });
  }
  function changeMode(mode: MatchMode) {
    setModeDraft(mode);
    if (!claimWindow || claimWindow.state !== "CLOSED" || claimWindow.version === 0 || mode === claimWindow.match_mode) return;
    const body = { expected_version: claimWindow.version, state: "CLOSED" as const, match_mode: mode };
    void perform("window", async (key, current) => { await setClaimWindow(storeID, scene, body, key, current); });
  }
  function addOffer() {
    const keyword = offerForm.keyword.trim();
    const max = Number(offerForm.max);
    if (!/^[A-Za-z0-9０-９Ａ-Ｚａ-ｚ]{1,16}$/.test(keyword))
      return setActionError({ action: "offer", code: "form", message: c.keywordInvalid });
    if (!/^[1-9][0-9]{0,2}$/.test(offerForm.max)) return setActionError({ action: "offer", code: "form", message: c.maxInvalid });
    if (!offerForm.sku) return setActionError({ action: "offer", code: "form", message: c.skuRequired });
    // Live price is optional; empty = none. Parsed in the SKU's own currency decimals (Go re-validates).
    const sku = skus.find((item) => item.id === offerForm.sku);
    const live = offerForm.live.trim() === "" ? null : parseLivePrice(offerForm.live, sku?.currency ?? "USD");
    if (offerForm.live.trim() !== "" && live === null) return setActionError({ action: "offer", code: "form", message: c.live.livePriceInvalid });
    const body = { keyword, sku_id: offerForm.sku, max_quantity_per_claim: max, ...(live === null ? {} : { live_price_minor: live }) };
    void perform("offer", async (key, current) => {
      await createClaimOffer(storeID, scene, body, key, current);
      setOfferForm((form) => ({ ...form, keyword: "", live: "" }));
    });
  }
  function updateOffer(offer: Offer, active: boolean, limit: string) {
    if (!/^[1-9][0-9]{0,2}$/.test(limit)) return setActionError({ action: "update", code: "form", message: c.maxInvalid });
    const body = { expected_version: offer.version, max_quantity_per_claim: Number(limit), active };
    void perform("update", async (key, current) => {
      await updateClaimOffer(storeID, scene, offer.offer_id, body, key, current);
      setLimits((values) => { const next = { ...values }; delete next[offer.offer_id]; return next; });
    });
  }
  // Live price save/clear: same PATCH with an explicit live_price_minor (0 = clear); the limit and active flag
  // are re-sent unchanged from the loaded offer (the version CAS guards against a concurrent edit).
  function saveLivePrice(offer: Offer, text: string) {
    const clear = text.trim() === "";
    const minor = clear ? 0 : parseLivePrice(text, offer.currency);
    if (minor === null) return setActionError({ action: "update", code: "form", message: c.live.livePriceInvalid });
    const body = { expected_version: offer.version, max_quantity_per_claim: offer.max_quantity_per_claim, active: offer.active, live_price_minor: minor };
    void perform("update", async (key, current) => {
      await updateClaimOffer(storeID, scene, offer.offer_id, body, key, current);
      setPrices((values) => { const next = { ...values }; delete next[offer.offer_id]; return next; });
    });
  }
  // Library: the offer form's keyword + SKU pickers also add a library row (version 0 = new).
  function addLibrary() {
    const keyword = offerForm.keyword.trim();
    if (!/^[A-Za-z0-9０-９Ａ-Ｚａ-ｚ]{1,16}$/.test(keyword)) return setActionError({ action: "library", code: "form", message: c.keywordInvalid });
    if (!offerForm.sku) return setActionError({ action: "library", code: "form", message: c.skuRequired });
    const existing = facts?.library?.find((entry) => entry.sku_id === offerForm.sku);
    const body = { keyword, expected_version: existing?.version ?? 0 };
    void perform("library", async (key, current) => {
      await setLibraryKeyword(storeID, scene, offerForm.sku, body, key, current);
      setOfferForm((form) => ({ ...form, keyword: "" }));
    });
  }
  function saveLibrary(entry: LibraryEntry, keyword: string) {
    const text = keyword.trim();
    if (!/^[A-Za-z0-9０-９Ａ-Ｚａ-ｚ]{1,16}$/.test(text)) return setActionError({ action: "library", code: "form", message: c.keywordInvalid });
    void perform("library", async (key, current) => {
      await setLibraryKeyword(storeID, scene, entry.sku_id, { keyword: text, expected_version: entry.version }, key, current);
      setLibraryEdits((values) => { const next = { ...values }; delete next[entry.sku_id]; return next; });
    });
  }
  function removeLibrary(entry: LibraryEntry) {
    if (!globalThis.confirm(c.live.libraryRemoveConfirm(entry.keyword))) return;
    void perform("library", async (key, current) => { await setLibraryKeyword(storeID, scene, entry.sku_id, { keyword: "", expected_version: entry.version }, key, current); });
  }
  // One action seeds this scene's offers; refusals come back as data and are listed, never silently dropped.
  function importOffers(body: { source: "library" } | { source: "session"; from_session_id: string }) {
    const names = Object.fromEntries((facts?.library ?? []).map((entry) => [entry.sku_id, entry.sku_code]));
    void perform("import", async (key, current) => {
      const result = await importClaimOffers(storeID, scene, body, key, current);
      setImported({ result, names });
    });
  }
  function record() {
    const text = manual.text;
    if (!text || utf8Bytes(text) > 256) return setActionError({ action: "manual", code: "form", message: c.textInvalid });
    const label = manual.label.trim();
    if (!manual.bundle && (!label || /\p{Cc}/u.test(label)))
      return setActionError({ action: "manual", code: "form", message: c.labelInvalid });
    const body = manual.bundle ? { bundle_id: manual.bundle, text } : { actor_label: manual.label, text };
    void perform("manual", async (key, current) => {
      const outcome = await recordManualClaim(storeID, scene, body, key, current);
      setResult(outcome);
      setManual((form) => ({ bundle: outcome.bundle_id || form.bundle, label: outcome.bundle_id ? "" : form.label, text: "" }));
    });
  }
  async function issue(bundle: Bundle, release: boolean) {
    if (blocked || (release && !globalThis.confirm(c.releaseConfirm))) return;
    setActionError(null);
    if (!origin.current) {
      try {
        const abort = new AbortController();
        origin.current = await readStorefrontOrigin(storeID, products ?? await readClaimProducts(storeID, abort.signal), abort.signal);
      } catch (error) { return setActionError({ action: "link", code: codeOf(error) }); }
      if (!origin.current) return setActionError({ action: "link", code: "no-origin" });
    }
    const storefront = origin.current;
    const body = { expected_generation: bundle.link.generation, release_binding: release };
    void perform("link", async (key, current) => {
      const link = await issueClaimLink(storeID, scene, bundle.bundle_id, body, key, current);
      setBuyerLocale(locale);
      setIssued({ ref: bundle.ref, origin: storefront, token: link.token, generation: link.generation,
        expiresAt: link.expires_at, released: link.released, replayed: link.replayed });
    });
  }
  async function loadMore() {
    if (!facts?.next) return;
    try {
      const page = await readClaimBundles(storeID, scene, facts.next, new AbortController().signal);
      setFacts((value) => value && { ...value, bundles: [...value.bundles, ...page.items], next: page.next_cursor });
    } catch (error) { setStatus(codeOf(error)); }
  }
  async function copy(value: string, what: string) {
    try { await navigator.clipboard.writeText(value); setCopied(what); }
    catch { setCopied(""); }
  }
  async function refreshFacts() {
    if (refreshing) return;
    // Every claims write is CAS- or absolute-quantity safe, so after reading the facts again a fresh submit may replace an unknown earlier one.
    pending.current = null;
    sourceDirty.current = false;
    setActionError(null);
    setRefreshing(true);
    const applied = await load();
    setRefreshing(false);
    if (applied) setRefreshedAt(Date.now());
  }
  function closeDialog() {
    dialog.current?.close();
    setIssued(null);
    setCopied("");
  }

  const link = issued?.token ? `${issued.origin}/${buyerLocale}/claim#t=${issued.token}` : "";
  const masked = issued ? `${issued.origin}/${buyerLocale}/claim#t=••••••••` : "";
  const expiresText = issued ? displayTime(locale, issued.expiresAt) : "";

  return <WorkspaceFrame locale={locale} storeName={store?.name ?? shared.noStore} active="live" onBeforeNavigate={() => { setIssued(null); return true; }}>
    <div className="studio-page claims-page" data-testid="merchant-claims">
      <AdminPageHeader locale={locale} description={facts ? `${c.scene}: ${facts.detail.draft.title}` : undefined} actions={<>
          {refreshedAt !== null && !refreshing && <span className="claims-refreshed" role="status">{c.refreshedAt(displayClock(locale, refreshedAt))}</span>}
          <button type="button" className="studio-refresh" disabled={!!busy || refreshing} aria-busy={refreshing} onClick={() => void refreshFacts()}>
            {refreshing ? c.refreshing : shared.refresh}
          </button>
        </>} />
      <div className="claims-mock" role="note">{feed && <strong data-testid="claims-feed">{feed}</strong>}<p>{c.mockDetail}</p></div>
      {status !== "ready" || !facts || !board || !claimWindow ? <p className="claims-status" role={status === "loading" ? "status" : "alert"}>{statusText(status === "ready" ? "loading" : status)}</p> : <>
        {!canManage && <p className="studio-note">{c.readOnly}</p>}
        <div className="claims-surface">
          <aside className="claims-rail" aria-labelledby="claims-window-title">
            <div className="claims-window-head">
              <h2 id="claims-window-title">{c.window}</h2>
              <Badge tone={claimWindow.state === "OPEN" ? "success" : "neutral"} data-testid="claims-window-state">
                {claimWindow.state === "OPEN" ? c.open : c.closed}</Badge>
            </div>
            <Field id="claims-mode" label={c.mode}>
              <select id="claims-mode" value={claimWindow.state === "OPEN" ? claimWindow.match_mode : modeDraft} disabled={blocked || claimWindow.state === "OPEN"}
                onChange={(event) => changeMode(event.target.value as MatchMode)}>
                <option value="EXACT">{c.modeExact}</option><option value="KEYWORD_QTY_ONLY">{c.modeQty}</option>
              </select>
            </Field>
            {claimWindow.state === "OPEN" && <p className="claims-muted">{c.modeLocked}</p>}
            <button type="button" className="primary claims-window-action" disabled={blocked} onClick={toggleWindow}>
              {busy === "window" ? c.working : claimWindow.state === "OPEN" ? c.closeWindow : c.openWindow}</button>
            <dl className="claims-facts">
              <div><dt>{c.round}</dt><dd>{claimWindow.generation}</dd></div>
              {claimWindow.state === "OPEN" && claimWindow.opened_at && <div><dt>{c.openedAt}</dt><dd>{displayTime(locale, claimWindow.opened_at)}</dd></div>}
            </dl>
            {alert("window")}
            <details className="claims-stats" open={board.stats.accepted > 0 || persistedReasons.some((reason) => board.stats.rejected[reason] > 0)}>
              <summary id="claims-stats-title">{c.stats}</summary>
              <dl className="claims-facts">
                <div><dt>{c.accepted}</dt><dd data-testid="claims-accepted">{board.stats.accepted}</dd></div>
                {persistedReasons.map((reason) => <div key={reason}><dt>{c.reasons[reason]}</dt>
                  <dd data-testid={`claims-rejected-${reason}`}>{board.stats.rejected[reason]}</dd></div>)}
              </dl>
              {board.stats.rejected.NO_MATCH > 0 && <p className="claims-hint" role="status">{c.notUnderstood(board.stats.rejected.NO_MATCH)}</p>}
            </details>
            {prompt && <section className="claims-prompt" aria-labelledby="claims-prompt-title">
              <h3 id="claims-prompt-title">{c.prompt}</h3>
              {prompt ? <>
                <div className="claims-prompt-controls">
                  <Field id="claims-prompt-language" label={c.promptLanguage}>
                    <select id="claims-prompt-language" value={promptLanguage} onChange={(event) => setPromptLanguage(event.target.value as Locale)}>
                      {locales.map((item) => <option key={item} value={item}>{localeNames[item]}</option>)}</select></Field>
                  <Field id="claims-prompt-keyword" label={c.promptKeyword}>
                    <select id="claims-prompt-keyword" value={keywordForPrompt} onChange={(event) => setPromptKeyword(event.target.value)}>
                      {activeOffers.map((offer) => <option key={offer.offer_id} value={offer.keyword}>{offer.keyword}</option>)}</select></Field>
                </div>
                <p className="claims-prompt-text" data-testid="host-prompt" lang={promptLanguage}>{prompt}</p>
                <button type="button" onClick={() => void copy(prompt, "prompt")}>{copied === "prompt" ? c.copied : c.copyPrompt}</button>
              </> : <p className="claims-muted">{c.promptNone}</p>}
            </section>}
          </aside>
          <div className="claims-work">
            <section className="claims-section" aria-labelledby="claims-source-title" data-testid="claims-source">
              <h2 id="claims-source-title">{c.source}</h2>
              <p className="claims-muted">{c.sourceIntro}</p>
              {facts.sourceError ? <p className="claims-muted" role="status">{c.sourceUnavailable}</p> : <>
                {source ? <dl className="claims-facts claims-source-status" data-testid="claims-source-status">
                  <div><dt>{c.sourceBoundTitle}</dt><dd>{c.sourcePlatform[source.platform]}</dd></div>
                  <div><dt>{c.sourceObject}</dt><dd data-testid="claims-source-object">{source.source_object_id}</dd></div>
                  <div><dt>{c.sourceState}</dt><dd>{source.active ? c.sourceOn : c.sourceOff}</dd></div>
                  <div><dt>{c.sourceVerification}</dt><dd data-testid="claims-source-verified">{source.verified ? c.sourceVerified : c.sourceUnverified}</dd></div>
                  <div><dt>{c.sourceCount}</dt><dd data-testid="claims-source-count">{source.intake_count}</dd></div>
                  <div><dt>{c.sourceCapped}</dt><dd data-testid="claims-source-capped">{source.intake_capped}</dd></div>
                  <div><dt>{c.sourceUpdated}</dt><dd>{displayTime(locale, source.updated_at)}</dd></div>
                </dl> : <p className="claims-muted" role="status" data-testid="claims-source-none">{c.sourceNone}</p>}
                {/* No Meta binding on this store (platforms empty): point at the Settings connect card instead of a dead end. */}
                {facts.platforms.length === 0 && <p className="claims-muted" role="status" data-testid="claims-source-connect">
                  {metaConnectCopy[locale].studioNone} <a href={`/${locale}/settings?store=${storeID}`}>{metaConnectCopy[locale].studioLink}</a></p>}
                <form className="claims-form claims-source-form" onSubmit={(event) => { event.preventDefault(); saveSource(); }}>
                  {facts.meta === null && <p role="status" className="claims-muted">{metaConnectCopy[locale].unavailable}</p>}
                  {!!facts.meta?.pages.length && <FormRow><Field id="claims-source-page" label={metaConnectCopy[locale].studioPage} hint={metaConnectCopy[locale].studioPageHint}>
                    <select id="claims-source-page" value={sourcePage} disabled={sourceLocked} aria-describedby="claims-source-page-hint"
                      onChange={(event) => { setSourcePage(event.target.value); editSource({ platform: event.target.value ? "facebook" : "" }); }}>
                      <option value="">{metaConnectCopy[locale].studioPageAuto}</option>
                      {facts.meta.pages.map((page) => <optgroup key={page.id} label={page.name || page.id}>
                        <option value={page.id} disabled={page.status !== "active"}>{page.name || page.id} · {page.id}{page.status !== "active" ? ` · ${metaConnectCopy[locale].reconnect}` : ""}</option>
                      </optgroup>)}
                    </select>
                  </Field></FormRow>}
                  <FormRow>
                  <Field id="claims-source-input" label={c.sourceInput} hint={c.sourceInputHint}>
                    <input id="claims-source-input" value={sourceForm.input} maxLength={claimSourceInputMax} autoComplete="off" spellCheck={false}
                      disabled={sourceLocked} aria-describedby="claims-source-input-hint"
                      aria-invalid={actionError?.action === "source" && actionError.code === "form" ? true : undefined}
                      onChange={(event) => editSource({ input: event.target.value })} /></Field>
                  {platformChoice && <Field id="claims-source-platform" label={c.sourcePlatformLabel} hint={c.sourcePlatformHint}>
                    <select id="claims-source-platform" value={sourceForm.platform} disabled={sourceLocked} aria-describedby="claims-source-platform-hint"
                      onChange={(event) => { setSourcePage(""); editSource({ platform: event.target.value as ClaimSourceForm["platform"] }); }}>
                      <option value="">{c.sourcePlatformAuto}</option>
                      {(["facebook", "instagram"] as const).map((item) => <option key={item} value={item}>{c.sourcePlatform[item]}</option>)}</select></Field>}
                  <Field id="claims-source-locale" label={c.sourceReplyLocale}>
                    <select id="claims-source-locale" value={sourceForm.reply_locale} disabled={sourceLocked}
                      onChange={(event) => editSource({ reply_locale: event.target.value as Locale })}>
                      {locales.map((item) => <option key={item} value={item}>{localeNames[item]}</option>)}</select></Field>
                  </FormRow>
                  <div className="claims-check-row">
                  <div className="claims-check">
                    <label><input type="checkbox" id="claims-source-reply" checked={sourceForm.private_reply} disabled={sourceLocked}
                      aria-describedby="claims-source-reply-hint" onChange={(event) => editSource({ private_reply: event.target.checked })} />{c.sourcePrivateReply}</label>
                    <small id="claims-source-reply-hint" className="claims-muted">{c.sourcePrivateReplyHint}</small>
                  </div>
                  <div className="claims-check">
                    <label><input type="checkbox" id="claims-source-active" checked={sourceForm.active} disabled={sourceLocked}
                      onChange={(event) => editSource({ active: event.target.checked })} />{c.sourceActive}</label>
                  </div>
                  </div>
                  <div className="claims-form-actions">
                    <button type="submit" className="primary" disabled={sourceLocked || !sourceChanged}>{busy === "source" ? c.working : c.sourceSave}</button>
                    {!sourceLocked && !sourceChanged && <p className="claims-muted">{c.sourceUnchanged}</p>}
                  </div>
                </form>
              </>}
              {alert("source")}
            </section>
            <section className="claims-section" aria-labelledby="claims-offers-title">
              <h2 id="claims-offers-title">{c.offers}</h2>
              <div className="claims-toolbar" data-testid="claims-import-tools">
                <FormRow><Field id="claims-copy-from" label={c.live.copyFrom} hint={c.live.copyHint}>
                  <select id="claims-copy-from" value={copyFrom} disabled={blocked || !scenes.length} onChange={(event) => setCopyFrom(event.target.value)}>
                    <option value="">{scenes.length ? c.live.chooseScene : c.live.noOtherScenes}</option>
                    {scenes.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></Field></FormRow>
                <div className="claims-import-actions">
                  <div><button type="button" disabled={blocked || !facts.library?.length} onClick={() => importOffers({ source: "library" })}>{c.live.importLibrary}</button>
                    {!facts.library?.length && <p className="claims-muted">{c.importEmpty}</p>}</div>
                  <div><button type="button" disabled={blocked || !copyFrom} onClick={() => importOffers({ source: "session", from_session_id: copyFrom })}>{c.live.copyOffers}</button>
                    {!copyFrom && <p className="claims-muted">{scenes.length ? c.live.chooseScene : c.live.noOtherScenes}</p>}</div>
                </div>
              </div>
              {busy === "import" && <p className="claims-muted" role="status">{c.working}</p>}
              {imported && <div role="status" className="claims-import-result" data-testid="claims-import-result">
                <p>{c.live.importDone(imported.result.created.length, imported.result.conflicts.length)}</p>
                {imported.result.conflicts.length > 0 && <>
                  <strong>{c.live.conflictsTitle}</strong>
                  <ul>{imported.result.conflicts.map((item) => <li key={`${item.keyword}-${item.sku_id}`}>
                    <span className="claims-keyword">{item.keyword}</span> — {c.live.conflictReason[item.reason]}</li>)}</ul></>}
              </div>}
              {alert("import")}
              {board.offers.length ? <TableFrame className="claims-table-scroll" label={c.offers} scrollHint={c.scrollHint}>
                <table className="claims-table claims-offers-table">
                  <thead><tr><th scope="col">{c.keyword}</th><th scope="col">{c.product}</th><th scope="col">{c.maxPerClaim}</th><th scope="col">{c.live.livePrice}</th><th scope="col">{c.status}</th><th scope="col">{c.actions}</th></tr></thead>
                  <tbody>{board.offers.map((offer) => {
                    const limit = limits[offer.offer_id] ?? String(offer.max_quantity_per_claim);
                    const savedPrice = offer.live_price_minor === null ? "" : fromMinor(offer.live_price_minor, offer.currency);
                    const price = prices[offer.offer_id] ?? savedPrice;
                    return <tr key={offer.offer_id} data-testid={`offer-${offer.keyword}`}>
                      <th scope="row" className="claims-keyword">{offer.keyword}</th>
                      <td><span className="claims-product">{offer.product_name}</span><small>{offer.sku_code}</small></td>
                      <td><input type="number" min={1} max={999} step={1} inputMode="numeric" aria-label={`${c.maxPerClaim} ${offer.keyword}`}
                        value={limit} disabled={blocked} onChange={(event) => setLimits({ ...limits, [offer.offer_id]: event.target.value })} /></td>
                      <td data-testid={`offer-price-${offer.keyword}`}>
                        <input type="text" inputMode="decimal" autoComplete="off" aria-label={`${c.live.livePrice} ${offer.keyword}`} placeholder={c.live.livePriceNone}
                          value={price} disabled={blocked} onChange={(event) => setPrices({ ...prices, [offer.offer_id]: event.target.value })} />
                        <small>{c.live.normalPrice(money(locale, offer.currency, offer.sku_price_minor))}</small>
                        {offer.live_price_minor !== null && offer.live_price_minor > offer.sku_price_minor && <small role="status" className="claims-warn">
                          {c.live.higherWarning(money(locale, offer.currency, offer.live_price_minor), money(locale, offer.currency, offer.sku_price_minor))}</small>}
                        {price !== savedPrice && <button type="button" disabled={blocked} onClick={() => saveLivePrice(offer, price)}>
                          {price.trim() === "" ? c.live.clearLivePrice : c.live.saveLivePrice}</button>}
                      </td>
                      <td><span className={`claims-badge ${offer.active ? "open" : "closed"}`}>{offer.active ? c.active : c.paused}</span></td>
                      <td className="claims-row-actions">
                        {limit !== String(offer.max_quantity_per_claim) && <button type="button" disabled={blocked}
                          onClick={() => updateOffer(offer, offer.active, limit)}>{c.saveMax}</button>}
                        <button type="button" disabled={blocked} onClick={() => updateOffer(offer, !offer.active, String(offer.max_quantity_per_claim))}>
                          {offer.active ? c.pause : c.resume}</button>
                      </td>
                    </tr>;
                  })}</tbody>
                </table>
              </TableFrame> : <p className="claims-muted">{c.noOffers}</p>}
              {alert("update")}
              <form className="claims-form claims-offer-form" onSubmit={(event) => { event.preventDefault(); addOffer(); }}>
                <FormRow className="claims-offer-fields">
                  <Field id="claims-offer-keyword" label={c.keyword}>
                    <input id="claims-offer-keyword" value={offerForm.keyword} maxLength={32} autoComplete="off" disabled={blocked}
                      aria-describedby="claims-keyword-hint" onChange={(event) => setOfferForm({ ...offerForm, keyword: event.target.value })} /></Field>
                  <Field id="claims-offer-product" label={c.product}>
                    <select id="claims-offer-product" value={offerForm.product} disabled={blocked || !products?.length}
                      onChange={(event) => setOfferForm({ ...offerForm, product: event.target.value, sku: "" })}>
                      <option value="">{c.chooseProduct}</option>
                      {products?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field>
                  <Field id="claims-offer-sku" label={c.sku}>
                    <select id="claims-offer-sku" value={offerForm.sku} disabled={blocked || !skus.length}
                      onChange={(event) => setOfferForm({ ...offerForm, sku: event.target.value })}>
                      <option value="">{c.chooseSKU}</option>
                      {skus.map((item) => <option key={item.id} value={item.id}>{item.code}</option>)}</select></Field>
                  <Field id="claims-offer-max" label={c.maxPerClaim}>
                    <input id="claims-offer-max" type="number" min={1} max={999} step={1} inputMode="numeric" value={offerForm.max} disabled={blocked}
                      onChange={(event) => setOfferForm({ ...offerForm, max: event.target.value })} /></Field>
                  <Field id="claims-offer-live" label={c.live.livePrice} hint={c.live.livePriceHint}>
                    <input id="claims-offer-live" type="text" inputMode="decimal" autoComplete="off" value={offerForm.live} disabled={blocked}
                      aria-describedby="claims-offer-live-hint" onChange={(event) => setOfferForm({ ...offerForm, live: event.target.value })} /></Field>
                </FormRow>
                <div className="claims-offer-actions">
                  <button type="submit" className="primary" disabled={blocked}>{busy === "offer" ? c.working : c.addOffer}</button>
                  <button type="button" disabled={blocked} data-testid="claims-add-library" onClick={addLibrary}>{busy === "library" ? c.working : c.live.libraryAdd}</button>
                </div>
                <p id="claims-keyword-hint" className="claims-muted claims-form-hint">{c.keywordHint}</p>
                {catalogError ? <p className="claims-muted" role="status">{c.catalogUnavailable}</p>
                  : products && !products.length && <p className="claims-muted" role="status">{c.noProducts}</p>}
              </form>
              {alert("offer")}
            </section>
            <section className="claims-section" aria-labelledby="claims-library-title" data-testid="claims-library">
              <h2 id="claims-library-title">{c.live.library}</h2>
              <p className="claims-muted">{c.live.libraryHint}</p>
              {facts.library && (facts.library.length ? <TableFrame className="claims-table-scroll" label={c.live.library} scrollHint={c.scrollHint}>
                <table className="claims-table claims-library-table">
                  <thead><tr><th scope="col">{c.live.libraryKeyword}</th><th scope="col">{c.product}</th><th scope="col">{c.actions}</th></tr></thead>
                  <tbody>{facts.library.map((entry) => {
                    const edit = libraryEdits[entry.sku_id] ?? entry.keyword;
                    return <tr key={entry.sku_id} data-testid={`library-${entry.keyword}`}>
                      <td><input type="text" maxLength={32} autoComplete="off" aria-label={`${c.live.libraryKeyword} ${entry.sku_code}`} value={edit} disabled={blocked}
                        onChange={(event) => setLibraryEdits({ ...libraryEdits, [entry.sku_id]: event.target.value })} /></td>
                      <td><span className="claims-product">{entry.product_name}</span><small>{entry.sku_code}</small></td>
                      <td className="claims-row-actions">
                        {edit !== entry.keyword && <button type="button" disabled={blocked} onClick={() => saveLibrary(entry, edit)}>{c.live.librarySave}</button>}
                        <button type="button" disabled={blocked} onClick={() => removeLibrary(entry)}>{c.live.libraryRemove}</button>
                      </td>
                    </tr>;
                  })}</tbody>
                </table>
              </TableFrame> : <p className="claims-muted">{c.live.libraryEmpty}</p>)}
              {alert("library")}
            </section>
            <section className="claims-section" aria-labelledby="claims-manual-title">
              <h2 id="claims-manual-title">{c.manual}</h2>
              <form className="claims-form claims-manual-form" onSubmit={(event) => { event.preventDefault(); record(); }}>
                <FormRow>
                <Field id="claims-manual-buyer" label={c.buyer}>
                  <select id="claims-manual-buyer" value={manual.bundle} disabled={blocked} onChange={(event) => setManual({ ...manual, bundle: event.target.value })}>
                    <option value="">{c.newBuyer}</option>
                    {/* Go adds manual claims only to manual bundles (claims.manualActor). */}
                    {facts.bundles.filter((bundle) => bundle.platform === "manual").map((bundle) => <option key={bundle.bundle_id} value={bundle.bundle_id}>{bundle.label} · {bundle.ref}</option>)}
                  </select></Field>
                {!manual.bundle && <Field id="claims-manual-label" label={c.label} hint={c.labelHint}>
                  <input id="claims-manual-label" value={manual.label} maxLength={120} autoComplete="off" disabled={blocked}
                    aria-describedby="claims-manual-label-hint" onChange={(event) => setManual({ ...manual, label: event.target.value })} /></Field>}
                <Field id="claims-manual-text" label={c.comment}>
                  <input id="claims-manual-text" value={manual.text} maxLength={256} autoComplete="off" disabled={blocked}
                    onChange={(event) => setManual({ ...manual, text: event.target.value })} /></Field>
                </FormRow>
                <div className="claims-form-actions"><button type="submit" className="primary" disabled={blocked}>{busy === "manual" ? c.working : c.record}</button></div>
              </form>
              {result && <p className={`claims-result ${result.outcome === "ACCEPTED" ? "accepted" : "rejected"}`} role="status" data-testid="claims-manual-result">
                {result.outcome === "ACCEPTED" ? c.resultAccepted(result.keyword, result.quantity, result.previous_quantity)
                  : c.resultRejected(result.reason ? c.reasons[result.reason] : c.invalid)}</p>}
              {alert("manual")}
            </section>
          </div>
        </div>
        <section className="claims-section claims-bundles" aria-labelledby="claims-bundles-title">
          <h2 id="claims-bundles-title">{c.bundles}</h2>
          {alert("link")}
          {facts.bundles.length ? <TableFrame className="claims-table-scroll" label={c.bundles} scrollHint={c.scrollHint}>
            <table className="claims-table claims-bundles-table">
              <thead><tr><th scope="col">{c.ref}</th><th scope="col">{c.buyer}</th><th scope="col">{c.items}</th><th scope="col">{c.link}</th><th scope="col">{c.actions}</th></tr></thead>
              <tbody>{facts.bundles.map((bundle) => <tr key={bundle.bundle_id} data-testid={`bundle-${bundle.label}`}>
                <th scope="row" className="claims-ref">{bundle.ref}</th>
                <td><span className="claims-product">{bundle.platform === "manual" ? bundle.label : c.sourcePlatform[bundle.platform]}</span><small>{bundle.bound ? c.bound : c.unbound}</small></td>
                <td><ul className="claims-lines">{bundle.lines.map((line) => <li key={line.offer_id}>
                  <strong>{line.keyword} × {line.quantity}</strong><small>{line.applied ? c.inCart : c.notInCart}</small></li>)}</ul></td>
                <td>{bundle.link.state === "ACTIVE" && bundle.link.expires_at ? c.linkActive(displayTime(locale, bundle.link.expires_at))
                  : bundle.link.state === "EXPIRED" ? c.linkExpired : c.linkNone}</td>
                <td className="claims-row-actions">
                  <button type="button" disabled={blocked} onClick={() => void issue(bundle, false)}>
                    {bundle.link.state === "NONE" ? c.issue : c.rotate}</button>
                  {bundle.bound && <button type="button" disabled={blocked} onClick={() => void issue(bundle, true)}>{c.release}</button>}
                </td>
              </tr>)}</tbody>
            </table>
          </TableFrame> : <p className="claims-muted">{c.noBundles}</p>}
          {facts.next && <button type="button" className="claims-more" onClick={() => void loadMore()}>{c.more}</button>}
        </section>
      </>}
      <dialog ref={dialog} className="claims-dialog" aria-labelledby="claims-dialog-title" onCancel={closeDialog}>
        {issued && <div className="claims-dialog-body">
          <h2 id="claims-dialog-title">{c.dialog} · {issued.ref}</h2>
          {feed && <p className="claims-mock-inline">{feed}</p>}
          {issued.replayed || !issued.token ? <p role="alert" className="claims-alert-text">{c.replayed}</p> : <>
            <p className="claims-muted">{c.dialogHint}</p>
            {issued.released && <p role="status">{c.released}</p>}
            <Field id="claims-buyer-locale" label={c.buyerLanguage}>
              <select id="claims-buyer-locale" value={buyerLocale} onChange={(event) => setBuyerLocale(event.target.value as Locale)}>
                {locales.map((item) => <option key={item} value={item}>{localeNames[item]}</option>)}</select></Field>
            <output className="claims-link-value" data-testid="claims-link-value">{revealed ? link : masked}</output>
            <p className="claims-muted">{c.expires(expiresText)}</p>
            <div className="claims-dialog-actions">
              <button type="button" onClick={() => setRevealed(!revealed)}>{revealed ? c.hide : c.reveal}</button>
              <button type="button" onClick={() => void copy(link, "link")}>{copied === "link" ? c.copied : c.copyLink}</button>
              <button type="button" onClick={() => void copy(claimLinkMessage(buyerLocale, link, expiresText), "message")}>
                {copied === "message" ? c.copied : c.copyMessage}</button>
            </div>
          </>}
          <button type="button" className="primary claims-dialog-close" onClick={closeDialog}>{c.close}</button>
        </div>}
      </dialog>
    </div>
  </WorkspaceFrame>;
}
