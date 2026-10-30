// Purpose: Owner-signed three-row settings, direct edit/review and optional three-step quick setup.
// Depends on: react, @live-commerce/i18n, ./WorkspaceFrame, @/lib/settings-v2, @/lib/settings-client, @/lib/catalog-v2-model, @/lib/settings-model and SettingsWizard's settings.css.
// Used by: SettingsWizard; localized authenticated settings route.
// Invariants: server baselines, one keyed command at a time, durable initial-stock intent; only owner AD-04 delivery defaults.
"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { settingsV2Copy, type Locale } from "@live-commerce/i18n";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { readSettings, sessionBoundary } from "@/lib/settings-client";
import { parseProduct, toMinor } from "@/lib/catalog-v2-model";
import type { Market, Policy, Service, SettingsInitial } from "@/lib/settings-model";
import { deliveryKinds, draftOf, execute, parseProfile, readSnapshot, reconciled, SaveFailure, serviceInput,
  type Journal, type Profile, type Quickstart, type SimpleDraft, type Snapshot } from "@/lib/settings-v2";

type Screen = "overview" | "editor" | "review" | "saved" | "uncertain" | "failed" | "step1" | "step2" | "step3";
const empty: SimpleDraft = { name: "", contact_email: "", contact_phone: "", enabled: {}, productName: "", price: "", stock: "" };

/** A compact merchant surface over the incumbent authenticated commands. */
export function SettingsV2({ locale, initial, onAdvanced }: { locale: Locale; initial: SettingsInitial; onAdvanced: () => void }) {
  const c = settingsV2Copy[locale], store = initial.store;
  const [data, setData] = useState<Snapshot | null>(null), [draft, setDraft] = useState<SimpleDraft>(empty);
  const [screen, setScreen] = useState<Screen>("overview"), [fromQuickstart, setFromQuickstart] = useState(false);
  const [busy, setBusy] = useState(false), [loading, setLoading] = useState(true), [partial, setPartial] = useState(false);
  const [notice, setNotice] = useState("");
  const scope = useRef(""), key = useRef(""), pending = useRef<Journal | null>(null);
  const active = useRef(true), generation = useRef(0), writing = useRef(false), heading = useRef<HTMLHeadingElement>(null);
  const names = { home: c.home, cvs_711: c.seven, cvs_familymart: c.family, cvs_hilife: c.hilife, cvs_okmart: c.ok };
  const currency = store?.currency ?? "";
  const priceLabel = c.price.replace("{currency}", currency);
  const canManage = !!store && (store.role === "owner" || store.role === "admin" || !!store.permissions?.includes("integration:manage"));
  const canCatalog = !!store && (store.role === "owner" || store.role === "admin" || !!store.permissions?.includes("catalog:write"));

  function journal(value: Journal | null) {
    if (value) sessionStorage.setItem(`${key.current}:command`, JSON.stringify(value));
    else sessionStorage.removeItem(`${key.current}:command`);
    pending.current = value;
  }
  function change(patch: Partial<SimpleDraft>) {
    const next = { ...draft, ...patch };
    try { sessionStorage.setItem(key.current, JSON.stringify({ version: data?.profile.version, draft: next })); setDraft(next); }
    catch { setScreen("failed"); setNotice(c.retained); }
  }
  async function load(discardDraft = false) {
    if (!store) { setLoading(false); setScreen("uncertain"); return; }
    const at = ++generation.current;
    setLoading(true); setNotice("");
    try {
      const boundary = await sessionBoundary(), scopedKey = `commerce-settings-v2:${store.id}:${boundary}`;
      const rawPending = sessionStorage.getItem(`${scopedKey}:command`);
      let command: Journal | null = null;
      if (rawPending) {
        const value = JSON.parse(rawPending) as Journal;
        if (typeof value.key !== "string" || !/^[0-9a-f-]{36}$/.test(value.key) || typeof value.body !== "string" ||
            value.body.length > 8192 || !["POST", "PUT"].includes(value.method) ||
            !/^(?:profile|quickstart|products|skus|inventory\/adjustments|markets|markets\/[0-9a-f-]{36}\/countries\/TW\/delivery-services\/[a-z][a-z0-9_-]{0,39}(?:\/policy)?)$/.test(value.resource)) throw new Error("journal_shape");
        command = value;
      }
      const skuCommand = command?.resource === "skus" ? command : null;
      const deliveryCommand = command?.resource === "markets" || command?.resource.startsWith("markets/");
      const initialStock = sessionStorage.getItem(`${scopedKey}:initial-stock`);
      const skuProduct = skuCommand ? (JSON.parse(skuCommand.body) as Record<string, unknown>).product_id : null;
      const stockProduct = initialStock ? (JSON.parse(initialStock) as Record<string, unknown>).product_id : null;
      if (skuCommand && initialStock && skuProduct !== stockProduct) throw new Error("recovery_product_mismatch");
      const recoveryProduct = skuCommand ? skuProduct : initialStock ? stockProduct : null;
      if ((skuCommand || initialStock) && typeof recoveryProduct !== "string") throw new Error("recovery_product_shape");
      const fresh = await readSnapshot(store.id, currency, recoveryProduct as string | null);
      if (!active.current || at !== generation.current) return;
      if (boundary !== await sessionBoundary()) throw new Error("session_changed");
      scope.current = boundary; key.current = scopedKey;
      pending.current = command;
      if (command && reconciled(command, fresh)) journal(null);
      let next = draftOf(fresh);
      const cached = sessionStorage.getItem(key.current);
      if ((!discardDraft || skuCommand || initialStock || deliveryCommand) && cached) {
        const saved = JSON.parse(cached) as { version: number; draft: SimpleDraft };
        const d = saved.draft;
        if (saved.version === fresh.profile.version && d &&
            [d.name, d.contact_email, d.contact_phone, d.productName, d.price, d.stock].every(s => typeof s === "string" && s.length <= 254) &&
            d.enabled && Object.entries(d.enabled).every(([k, v]) => deliveryKinds.includes(k as typeof deliveryKinds[number]) && typeof v === "boolean")) {
          next = skuCommand || initialStock ? { ...next, productName: d.productName, price: d.price, stock: d.stock }
            : discardDraft && deliveryCommand ? { ...next, enabled: d.enabled } : d;
        } else if (skuCommand || initialStock) throw new Error("sku_draft_shape");
      }
      if (skuCommand) {
        const original = JSON.parse(skuCommand.body) as Record<string, unknown>;
        const product = fresh.product, price = toMinor(next.price, currency), stock = Number(next.stock);
        if (!cached || skuCommand.method !== "POST" || !product || product.status !== "draft" ||
            product.name !== next.productName || product.skus.length > 1 || original.product_id !== product.id ||
            original.code !== `setup-${product.id}` || price === null || original.price_minor !== price ||
            !next.stock.trim() || !Number.isSafeInteger(stock) || stock < 0 ||
            product.skus.some(sku => sku.code !== original.code || sku.price_minor !== price || sku.currency !== currency)) throw new Error("sku_draft_mismatch");
        // The SKU command has not sent initial stock yet. Its scoped draft is
        // required even on Reload; a read balance cannot replace that quantity.
      }
      if (initialStock) {
        const intent = JSON.parse(initialStock) as Record<string, unknown>;
        const product = fresh.product, price = toMinor(next.price, currency), stock = Number(next.stock);
        if (!cached || !product || product.status !== "draft" || product.name !== next.productName ||
            product.id !== intent.product_id || intent.code !== `setup-${product.id}` ||
            price === null || intent.price_minor !== price || intent.currency !== currency ||
            fresh.warehouses.length !== 1 || intent.warehouse_id !== fresh.warehouses[0].id ||
            !next.stock.trim() || !Number.isSafeInteger(stock) || stock < 0 || intent.quantity !== stock ||
            product.skus.length > 1 || product.skus.some(sku => sku.code !== intent.code || sku.price_minor !== price || sku.currency !== currency)) throw new Error("initial_stock_mismatch");
        // A successful SKU receipt may have cleared its journal before a later
        // read failed. This unsent stock still requires an explicit Save.
      }
      if (pending.current?.resource === "inventory/adjustments") {
        const original = JSON.parse(pending.current.body) as Record<string, unknown>;
        if (!Number.isSafeInteger(original.delta) || Number(original.delta) < 0 || original.expected_version !== 0 ||
            original.reason !== "quickstart_initial_stock" || original.warehouse_id !== fresh.warehouses[0]?.id ||
            original.sku_id !== fresh.product?.skus[0]?.id) throw new Error("inventory_journal_shape");
        // The initial adjustment is an operation, not the current balance. Keep
        // its quantity even when Reload discards edits; Save retains its key/body.
        next = { ...next, stock: String(original.delta) };
      }
      setData(fresh); setDraft(next); setScreen("overview");
      sessionStorage.setItem(key.current, JSON.stringify({ version: fresh.profile.version, draft: next }));
    } catch { if (active.current && at === generation.current) setScreen("uncertain"); }
    finally { if (active.current && at === generation.current) setLoading(false); }
  }
  useEffect(() => { active.current = true; void load(); return () => { active.current = false; generation.current++; }; }, [store?.id]);
  useEffect(() => { heading.current?.focus(); }, [screen]);

  async function save() {
    if (!store || !data || writing.current || !canManage) return;
    writing.current = true; setBusy(true); setNotice(""); setPartial(false);
    let changed = false;
    const send = async <T,>(resource: string, method: "POST" | "PUT", body: unknown): Promise<T> => {
      const result = await execute<T>(store.id, scope.current, resource, method, body, pending.current, journal);
      changed = true;
      if (!active.current || scope.current !== await sessionBoundary()) throw new SaveFailure(false, true, true);
      return result;
    };
    try {
      // The browser's per-store lock also covers other tabs. No lock/storage capability => no unsafe write.
      if (!navigator.locks) throw new SaveFailure(false, false);
      await navigator.locks.request(`settings-v2:${store.id}:${scope.current}`, { ifAvailable: true }, async lock => {
        if (!lock) throw new SaveFailure(false, false);
        // Resolve the original delivery command before considering any later
        // edits. Reads and reloads never retry a command or allocate a new key.
        if (pending.current?.resource === "markets" || pending.current?.resource.startsWith("markets/")) {
          const original = pending.current;
          await send(original.resource, original.method, JSON.parse(original.body));
          const fresh = await readSnapshot(store.id, currency);
          data.market = fresh.market; data.createMarket = fresh.createMarket; data.delivery = fresh.delivery;
        }
        let profile = { name: draft.name, contact_email: draft.contact_email, contact_phone: draft.contact_phone, expected_version: data.profile.version };
        if (pending.current?.resource === "profile") {
          const original = JSON.parse(pending.current.body) as typeof profile;
          if (original.name !== draft.name || original.contact_email !== draft.contact_email || original.contact_phone !== draft.contact_phone) throw new SaveFailure(true, false);
          profile = original; // durable replay, including its original version after another operator's save
        }
        if (pending.current?.resource === "profile" || draft.name !== data.profile.name || draft.contact_email !== data.profile.contact_email || draft.contact_phone !== data.profile.contact_phone) {
          const saved = parseProfile(await send<Profile>("profile", "PUT", profile));
          if (saved.name !== profile.name || saved.contact_email !== profile.contact_email || saved.contact_phone !== profile.contact_phone || saved.version !== profile.expected_version + 1) throw new SaveFailure(true, changed);
          data.profile = saved;
          sessionStorage.setItem(key.current, JSON.stringify({ version: saved.version, draft }));
        }
        if (data.createMarket && data.delivery.some(d => draft.enabled[d.kind] && d.createCode)) {
          data.market = await send<Market>("markets", "POST", { code: "tw", name: "台灣", currency });
          data.createMarket = false;
        }
        for (const [index, d] of data.delivery.entries()) {
          if (!d.service && draft.enabled[d.kind]) {
            if (!d.createCode || !data.market) throw new SaveFailure(false, changed);
            const path = `markets/${data.market.id}/countries/TW/delivery-services/${d.createCode}`;
            const policy = d.policy ?? await send<Policy>(`${path}/policy`, "PUT", {
              market_id: data.market.id, country: "TW", method: `delivery:${d.createCode}`, currency,
              shipping_mode: "country_flat", shipping_minor: 0, tax_mode: "inclusive", tax_basis: "goods",
              tax_rate_bps: 0, quote_ttl_seconds: 300, enabled: true, expected_version: 0,
              configuration_ref: "快速上手預設（owner 2026-10-30）",
            });
            d.policy = policy;
            const labelKey = { home: "home", cvs_711: "seven", cvs_familymart: "family", cvs_hilife: "hilife", cvs_okmart: "ok" } as const;
            const label = labelKey[d.kind];
            d.service = await send<Service>(path, "PUT", {
              market_id: data.market.id, country: "TW", code: d.createCode, expected_version: 0, policy_version: policy.version,
              name_hans: settingsV2Copy["zh-CN"][label], name_hant: settingsV2Copy["zh-TW"][label], name_en: settingsV2Copy.en[label],
              delivery_kind: d.kind, mode: "MANUAL", enabled: true, visible: true, sort_order: index,
            });
          }
          if (!d.service || d.service.enabled === draft.enabled[d.kind]) continue;
          const service = d.service, enabled = !!draft.enabled[d.kind];
          const saved = await send<typeof service>(`markets/${service.market_id}/countries/TW/delivery-services/${service.code}`, "PUT", serviceInput(service, enabled));
          if (saved.code !== service.code || saved.enabled !== enabled || saved.version !== service.version + 1) throw new SaveFailure(true, changed);
          d.service = saved;
        }
        if (draft.productName && fromQuickstart) {
          if (!canCatalog || data.warehouses.length !== 1) throw new SaveFailure(false, changed);
          const price = toMinor(draft.price, currency), stock = Number(draft.stock);
          if (price === null || !Number.isSafeInteger(stock) || stock < 0) throw new SaveFailure(false, changed);
          const retainInitialStock = (productID: string) => {
            sessionStorage.setItem(key.current, JSON.stringify({ version: data.profile.version, draft }));
            sessionStorage.setItem(`${key.current}:initial-stock`, JSON.stringify({ product_id: productID, code: `setup-${productID}`,
              price_minor: price, currency, warehouse_id: data.warehouses[0].id, quantity: stock }));
          };
          let product = data.product;
          if (!product || pending.current?.resource === "products") {
            const saved = await send<{ id: string }>("products", "POST", { name: draft.productName, description: "", status: "draft" });
            // The acknowledged product and unsent fields must survive even if
            // its first detail read fails before any SKU command is dispatched.
            retainInitialStock(saved.id);
            product = parseProduct(await readSettings(store.id, `products/${saved.id}`));
          }
          if (product.name !== draft.productName || product.status !== "draft" || product.skus.length > 1) throw new SaveFailure(false, changed);
          data.product = product; // retain accepted progress if a later command fails
          if (!product.skus.length || pending.current?.resource === "skus") {
            // Persist before dispatch: the SKU acknowledgement clears its own
            // journal, but cannot discard the next, still-unsent stock operation.
            retainInitialStock(product.id);
            await send("skus", "POST", { product_id: product.id, code: `setup-${product.id}`, price_minor: price });
            product = parseProduct(await readSettings(store.id, `products/${product.id}`));
            data.product = product;
          }
          const sku = product.skus[0];
          if (!sku || sku.price_minor !== price || sku.currency !== currency) throw new SaveFailure(false, changed);
          if (pending.current?.resource === "inventory/adjustments" || (sku.on_hand === null && stock > 0)) {
            await send("inventory/adjustments", "POST", { warehouse_id: data.warehouses[0].id, sku_id: sku.id, delta: stock, expected_version: 0, reason: "quickstart_initial_stock" });
          } else if ((sku.on_hand ?? sku.available) !== stock) throw new SaveFailure(false, changed);
          sessionStorage.removeItem(`${key.current}:initial-stock`);
        }
        if (pending.current) throw new SaveFailure(true, changed);
        const fresh = await readSnapshot(store.id, currency,
          sessionStorage.getItem(`${key.current}:initial-stock`) ? data.product?.id ?? null : null);
        // A partial quick start is not called complete. The server derives these steps.
        if (fromQuickstart && !fresh.marker.completed && Object.values(fresh.marker.steps).every(Boolean)) {
          fresh.marker = await send<Quickstart>("quickstart", "POST", {});
        }
        if (!active.current || scope.current !== await sessionBoundary()) throw new SaveFailure(false, changed, true);
        let next = draftOf(fresh);
        if (sessionStorage.getItem(`${key.current}:initial-stock`)) {
          // Ordinary settings saves do not submit initial stock. Keep its
          // quantity-bearing draft paired with the intent at the fresh version.
          next = { ...next, productName: draft.productName, price: draft.price, stock: draft.stock };
          sessionStorage.setItem(key.current, JSON.stringify({ version: fresh.profile.version, draft: next }));
        } else sessionStorage.removeItem(key.current);
        setData(fresh); setDraft(next); setScreen("saved");
      });
    } catch (error) {
      if (!active.current) return;
      const failure = error instanceof SaveFailure ? error : new SaveFailure(true, changed);
      setPartial(changed || failure.partial); setNotice(failure.session ? c.session : failure.uncertain && pending.current ? c.pending : "");
      setScreen(failure.uncertain ? "uncertain" : "failed");
    } finally { writing.current = false; if (active.current) setBusy(false); }
  }
  async function skip() {
    if (!store || !canManage || writing.current) return;
    writing.current = true; setBusy(true);
    try {
      await execute<Quickstart>(store.id, scope.current, "quickstart", "POST", {}, pending.current, journal);
      await load(true);
    } catch { setScreen("uncertain"); }
    finally { writing.current = false; if (active.current) setBusy(false); }
  }
  function review(event: FormEvent) { event.preventDefault(); setScreen("review"); }
  const button = (label: string, action: () => void, primary = false) => <button type="button" className={primary ? "settings-v2-primary" : "settings-v2-secondary"} disabled={busy || loading} onClick={action}>{label}</button>;
  const title = screen === "editor" ? c.edit : screen === "review" ? c.review : screen === "saved" ? c.saved : screen === "failed" ? c.failed : screen === "uncertain" ? c.uncertain : screen === "step1" ? c.step1 : screen === "step2" ? c.step2 : screen === "step3" ? c.step3 : c.title;
  const profileFields = <>
    <label>{c.name}<input required maxLength={120} value={draft.name} onChange={e => change({ name: e.target.value })} /></label>
    <label>{c.email}<input required type="email" maxLength={254} value={draft.contact_email} onChange={e => change({ contact_email: e.target.value })} /></label>
    <label>{c.phone}<input type="tel" maxLength={32} value={draft.contact_phone} onChange={e => change({ contact_phone: e.target.value })} /></label>
  </>;
  const deliveryFields = <>
    <label className="settings-v2-switch settings-v2-select-all">
      <input type="checkbox" checked={deliveryKinds.every(kind => draft.enabled[kind])}
        disabled={!canManage || busy || data?.delivery.some(d => !d.service && !d.createCode)}
        onChange={e => change({ enabled: Object.fromEntries(deliveryKinds.map(kind => [kind, e.target.checked])) })} />{c.selectAll}
    </label>
    {data?.delivery.map(d => <label className="settings-v2-switch" key={d.kind}>
      <input type="checkbox" checked={!!draft.enabled[d.kind]} disabled={!canManage || (!d.service && !d.createCode) || busy} aria-describedby={!d.service && !d.createCode ? "settings-v2-prerequisite" : undefined}
        onChange={e => change({ enabled: { ...draft.enabled, [d.kind]: e.target.checked } })} />{names[d.kind]}
    </label>)}
    {data?.delivery.some(d => !d.service && !d.createCode) && <p id="settings-v2-prerequisite" className="settings-v2-hint">{c.prerequisite}</p>}
  </>;
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? ""} active="settings" locked={busy}>
    <div className="settings-v2 settings-page" data-testid="settings-wizard" aria-busy={busy || loading}>
      <h1 ref={heading} tabIndex={-1}>{title}</h1>
      {loading ? <p role="status">{c.loading}</p> : <>
        {screen === "overview" && data && <section data-testid="settings-v2-overview">
          {canManage && <div className="settings-v2-actions settings-v2-overview-actions">
            <span data-testid="settings-v2-quickstart-prompt">{button(c.start, () => { setFromQuickstart(true); setScreen("step1"); }, true)}</span>
            {button(c.edit, onAdvanced)}
          </div>}
          {!canManage && button(c.view, onAdvanced)}
          <div className="settings-v2-rows">{[
            data.profile.name && data.profile.contact_email ? c.complete : c.incomplete,
            c.delivery.replace("{n}", String(data.delivery.filter(d => d.service?.enabled).length)), c.payment,
          ].map(label => <button type="button" key={label} disabled={!canManage} onClick={() => { setFromQuickstart(false); setScreen("editor"); }}>
            <span>{label}</span><svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none"><path d="m9 5 7 7-7 7" stroke="currentColor" strokeWidth="1.5" /></svg>
          </button>)}</div>
        </section>}
        {/* Remount between steps so a clicked navigation button cannot become the next step's submit button before its native default action. */}
        {(screen === "editor" || screen.startsWith("step")) && <form key={screen} data-testid={screen === "editor" ? "settings-v2-editor" : "settings-v2-quickstart"} onSubmit={review}>
          <fieldset disabled={busy || !canManage}>
            {screen.startsWith("step") && <p>{button(c.skip, () => void skip())}</p>}
            {(screen === "editor" || screen === "step2") && <p className="settings-v2-shipping">{c.shipping}</p>}
            {(screen === "editor" || screen === "step1") && profileFields}
            {(screen === "editor" || screen === "step2") && deliveryFields}
            {screen === "step3" && <>
              <p>{c.draft}</p>
              <label>{c.product}<input required maxLength={120} disabled={!canCatalog || !!data?.product} value={draft.productName} onChange={e => change({ productName: e.target.value })} /></label>
              <label>{priceLabel}<input required inputMode="decimal" disabled={!canCatalog || !!data?.product?.skus.length || pending.current?.resource === "skus"} value={draft.price} onChange={e => change({ price: e.target.value })} /></label>
              <label>{c.stock}<input required type="number" min="0" step="1" disabled={!canCatalog || !!data?.product?.skus.length || pending.current?.resource === "skus"} value={draft.stock} onChange={e => change({ stock: e.target.value })} /></label>
            </>}
            <div className="settings-v2-actions">
              {screen === "step1" ? button(c.nextDelivery, () => { if (draft.name.trim() && /^[^\s@]+@[^\s@]+$/.test(draft.contact_email)) setScreen("step2"); }, true)
                : screen === "step2" ? <>{button(c.backName, () => setScreen("step1"))}{button(c.nextProduct, () => setScreen("step3"), true)}</>
                : <>{screen === "step3" && button(c.backDelivery, () => setScreen("step2"))}<button className="settings-v2-primary" type="submit">{c.review}</button></>}
              {screen === "editor" && button(c.back, () => setScreen("overview"))}
            </div>
          </fieldset>
        </form>}
        {screen === "review" && <section data-testid="settings-v2-review">
          <div className="settings-v2-facts">
            <p>{`${c.name}: ${draft.name}`}</p><p>{`${c.email}: ${draft.contact_email}`}</p><p>{`${c.phone}: ${draft.contact_phone || c.unavailable}`}</p>
            {data?.publicURL && <p>{`${c.publicURL}: ${data.publicURL}`}</p>}
          </div>
          {deliveryKinds.map(kind => <p key={kind}>{`${names[kind]}: ${draft.enabled[kind] ? c.on : c.off}`}</p>)}
          {draft.productName && <div className="settings-v2-product"><p>{`${c.productDraft}: ${draft.productName}`}</p><p>{`${priceLabel}: ${draft.price}`}</p><p>{`${c.stock}: ${draft.stock}`}</p><p>{c.draft}</p></div>}
          <p>{c.paymentReview}</p><p>{c.shippingReview}</p><p>{c.checkout}</p>
          <div className="settings-v2-actions">{button(c.backEdit, () => setScreen(fromQuickstart ? "step3" : "editor"))}{button(c.save, () => void save(), true)}</div>
        </section>}
        {screen === "saved" && <section><p>{c.eligibility}</p>{button(c.back, () => { setFromQuickstart(false); setScreen("overview"); }, true)}</section>}
        {(screen === "uncertain" || screen === "failed") && <section role="alert">
          <p>{notice || (screen === "uncertain" ? c.reloadHelp : partial ? c.partial : c.retained)}</p>
          {screen === "uncertain" ? button(c.reload, () => void load(true), true) : <>{button(c.backEdit, () => setScreen(fromQuickstart ? "step3" : "editor"), true)}{partial && button(c.reload, () => void load(true))}</>}
        </section>}
      </>}
    </div>
  </WorkspaceFrame>;
}
