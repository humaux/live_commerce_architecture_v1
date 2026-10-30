// Purpose: Read the signed settings surface from existing profile, delivery, catalog and quick-start responses.
// Depends on: settings-client, settings-model, catalog-v2-model and storefront-model; existing authenticated BFF only.
// Used by: components/SettingsV2.tsx.
// Invariants: AD-04 defaults apply only to explicit delivery selections; existing policies remain intact; keyed/CAS commands.
import { readSettings, writeSettings, type Pending } from "./settings-client";
import type { Market, Policy, Service } from "./settings-model";
import type { Page } from "./model";
import { fromMinor, parseProduct, parseSummaryPage, parseWarehouses, type ProductDetail } from "./catalog-v2-model";
import { parseStorefront } from "./storefront-model";

export type Profile = { name: string; contact_email: string; contact_phone: string; version: number };
export type Quickstart = { completed: boolean; completed_at: string | null; steps: { store_name: boolean; shipping: boolean; first_product: boolean } };
export const deliveryKinds = ["home", "cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"] as const;
export type DeliveryKind = typeof deliveryKinds[number];
export type Delivery = { kind: DeliveryKind; service: Service | null; createCode: string | null; policy: Policy | null };
export type Snapshot = {
  profile: Profile; marker: Quickstart; delivery: Delivery[]; product: ProductDetail | null;
  market: Market | null; createMarket: boolean;
  warehouses: { id: string; name: string }[]; publicURL: string | null;
};
export type SimpleDraft = {
  name: string; contact_email: string; contact_phone: string;
  enabled: Partial<Record<DeliveryKind, boolean>>;
  productName: string; price: string; stock: string;
};
export type Journal = Pending & { body: string };
export class SaveFailure extends Error {
  constructor(public uncertain: boolean, public partial: boolean, public session = false) { super("settings_save"); }
}

export function parseProfile(value: unknown): Profile {
  const p = value as Profile;
  if (!p || typeof p.name !== "string" || typeof p.contact_email !== "string" || typeof p.contact_phone !== "string" ||
      !Number.isSafeInteger(p.version) || p.version < 0) throw new Error("profile_shape");
  return p;
}

/** Bounded discovery: incomplete/ambiguous collections cannot enable a switch. */
export async function readSnapshot(store: string, currency: string, recoveryProductId: string | null = null): Promise<Snapshot> {
  if (recoveryProductId !== null && !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(recoveryProductId)) throw new Error("recovery_product_shape");
  const [profileRaw, marker, markets, productsRaw, warehousesRaw, storefront] = await Promise.all([
    readSettings<unknown>(store, "profile"), readSettings<Quickstart>(store, "quickstart"),
    readSettings<Page<Market>>(store, "markets").catch(() => null),
    readSettings<unknown>(store, "catalog-products?status=draft&limit=1").catch(() => null),
    readSettings<unknown>(store, "warehouses?limit=100").catch(() => null),
    readSettings<unknown>(store, "storefront").then(parseStorefront).catch(() => null),
  ]);
  if (typeof marker?.completed !== "boolean" || !marker.steps ||
      [marker.steps.store_name, marker.steps.shipping, marker.steps.first_product].some(v => typeof v !== "boolean")) throw new Error("marker_shape");
  // Catalog/inventory permissions are not prerequisites for viewing store settings.
  // Failed discovery disables product creation rather than treating an unread list
  // as proof that the store has no products (which could create a duplicate).
  const products = productsRaw === null ? null : parseSummaryPage(productsRaw);
  const warehouses = productsRaw === null || warehousesRaw === null ? [] : parseWarehouses(warehousesRaw);
  // Discovery is only a default. An unfinished operation owns its original
  // product even when another operator creates a newer draft in the meantime.
  const productId = recoveryProductId ?? products?.items[0]?.id;
  const product = productId ? parseProduct(await readSettings(store, `products/${productId}`)) : null;
  if (recoveryProductId !== null && product?.id !== recoveryProductId) throw new Error("recovery_product_mismatch");
  // No arbitrary first-market choice: multiple markets require the existing advanced editor.
  const candidates = markets?.items.filter(m => m.active && m.currency === currency) ?? [];
  const market = markets && !markets.next_cursor && candidates.length === 1 ? candidates[0] : null;
  const createMarket = !!markets && !markets.next_cursor && candidates.length === 0 && !markets.items.some(m => m.code === "tw");
  let services: Service[] = [];
  let completeServices = createMarket;
  if (market) {
    const page = await readSettings<Page<Service>>(store, `markets/${market.id}/countries/TW/delivery-services`);
    if (!page.next_cursor) { services = page.items; completeServices = true; }
  }
  const delivery = await Promise.all(deliveryKinds.map(async kind => {
    const matches = services.filter(s => s.delivery_kind === kind && s.mode === "MANUAL");
    const unavailable = { kind, service: null, createCode: null, policy: null };
    if (!completeServices || matches.length > 1) return unavailable;
    if (!matches.length) {
      const code = `quickstart_${kind}`;
      if (services.some(s => s.code === code)) return unavailable;
      if (!market) return { ...unavailable, createCode: createMarket ? code : null };
      // An orphan policy may be the acknowledged first half of a previous save.
      // Never mistake a failed read for absence or overwrite an existing policy.
      try {
        const policy = await readSettings<Policy>(store, `markets/${market.id}/countries/TW/delivery-services/${code}/policy`);
        if (policy.market_id !== market.id || policy.country !== "TW" || policy.method !== `delivery:${code}` ||
            policy.currency !== currency || !Number.isSafeInteger(policy.version) || policy.version < 1 ||
            !policy.enabled || policy.shipping_mode !== "country_flat" || policy.shipping_minor !== 0 ||
            policy.tax_mode !== "inclusive" || policy.tax_basis !== "goods" || policy.tax_rate_bps !== 0) return unavailable;
        return { ...unavailable, createCode: code, policy };
      } catch (error) {
        return error && typeof error === "object" && "code" in error && error.code === "not_found"
          ? { ...unavailable, createCode: code } : unavailable;
      }
    }
    if (!market) return unavailable;
    const code = matches[0].code;
    if (!/^[a-z][a-z0-9_-]{0,39}$/.test(code)) return unavailable;
    try {
      const path = `markets/${market.id}/countries/TW/delivery-services/${code}`;
      const [service, policy] = await Promise.all([
        readSettings<Service>(store, path), readSettings<Policy>(store, `${path}/policy`),
      ]);
      if (service.market_id !== market.id || service.country !== "TW" || service.code !== code ||
          service.delivery_kind !== kind || service.mode !== "MANUAL" || service.currency !== currency ||
          !Number.isSafeInteger(service.version) || service.version < 1 || !policy.enabled ||
          policy.market_id !== service.market_id || policy.country !== service.country || policy.method !== service.policy_method ||
          policy.version !== service.policy_version || policy.currency !== currency) return unavailable;
      return { kind, service, createCode: null, policy };
    } catch { return unavailable; }
  }));
  const domains = storefront?.published ? storefront.domains.filter(d => d.serving && Date.parse(d.valid_until) > Date.now()) : [];
  return { profile: parseProfile(profileRaw), marker, delivery, product, warehouses, market, createMarket,
    publicURL: domains.length === 1 ? domains[0].origin : null };
}

export function draftOf(data: Snapshot): SimpleDraft {
  const sku = data.product?.skus.length === 1 ? data.product.skus[0] : null;
  return { name: data.profile.name, contact_email: data.profile.contact_email, contact_phone: data.profile.contact_phone,
    enabled: Object.fromEntries(data.delivery.map(d => [d.kind, d.service?.enabled ?? false])),
    productName: data.product?.name ?? "", price: sku ? fromMinor(sku.price_minor, sku.currency) : "", stock: sku ? String(sku.on_hand ?? sku.available) : "" };
}

export function serviceInput(service: Service, enabled: boolean) {
  const binding = service as Service & { binding_id?: string; binding_version?: number };
  return { market_id: service.market_id, country: service.country, code: service.code,
    expected_version: service.version, policy_version: service.policy_version,
    name_hans: service.name_hans, name_hant: service.name_hant, name_en: service.name_en,
    delivery_kind: service.delivery_kind, mode: service.mode, enabled, visible: service.visible, sort_order: service.sort_order,
    ...(binding.binding_id === undefined ? {} : { binding_id: binding.binding_id }),
    ...(binding.binding_version === undefined ? {} : { binding_version: binding.binding_version }),
  };
}

/** A command is journaled before sending; an uncertain command never receives a fresh key. */
export async function execute<T>(store: string, boundary: string, resource: string, method: "POST" | "PUT", body: unknown,
  current: Journal | null, persist: (p: Journal | null) => void): Promise<T> {
  const bytes = JSON.stringify(body);
  if (current && (current.resource !== resource || current.method !== method || current.body !== bytes)) throw new SaveFailure(true, false);
  const pending: Journal = current ?? { resource, method, body: bytes, key: crypto.randomUUID() };
  persist(pending); // storage failure sends zero writes
  let result: Awaited<ReturnType<typeof writeSettings<T>>>;
  try { result = await writeSettings<T>(store, pending, pending.body, boundary); }
  catch (error) { throw new SaveFailure(false, false, error instanceof Error && error.message === "session_changed"); }
  if (result.uncertain) throw new SaveFailure(true, false);
  if (result.error) { persist(null); throw new SaveFailure(false, false); }
  // Keep the original journal on an unreadable or mismatched 200 just as on a
  // lost answer. A success-shaped object alone never permits another command.
  if (result.value === null || !validReceipt(resource, JSON.parse(bytes), result.value)) throw new SaveFailure(true, false);
  persist(null);
  return result.value;
}

function validReceipt(resource: string, sent: Record<string, unknown>, value: unknown) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const saved = value as Record<string, unknown>;
  const uuid = (v: unknown) => typeof v === "string" && /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(v);
  if (resource === "quickstart") {
    const steps = saved.steps as Record<string, unknown> | undefined;
    return saved.completed === true && typeof saved.completed_at === "string" && Number.isFinite(Date.parse(saved.completed_at)) &&
      !!steps && [steps.store_name, steps.shipping, steps.first_product].every(v => typeof v === "boolean");
  }
  if (resource === "products") return uuid(saved.id) && saved.name === sent.name && saved.status === "draft";
  if (resource === "markets") return uuid(saved.id) && saved.active === true && saved.version === 1 &&
    ["code", "name", "currency"].every(k => saved[k] === sent[k]);
  if (resource === "skus") return uuid(saved.id) && ["product_id", "code", "price_minor"].every(k => saved[k] === sent[k]);
  if (resource === "inventory/adjustments") return saved.sku_id === sent.sku_id && saved.warehouse_id === sent.warehouse_id && saved.version === Number(sent.expected_version) + 1;
  return saved.version === Number(sent.expected_version) + 1 &&
    Object.entries(sent).every(([k, v]) => k === "expected_version" || (resource.endsWith("/policy") && k === "configuration_ref") || saved[k] === v);
}

/** Reads reconcile committed receipts and the append-only completion marker, never infer an absent write. */
export function reconciled(pending: Journal, data: Snapshot) {
  const body = JSON.parse(pending.body) as Record<string, unknown>;
  if (pending.resource === "profile") return validReceipt("profile", body, data.profile);
  // OC-05 completion is permanent per store, including dismissal; GET proves it
  // independently of the POST key without claiming the derived steps are done.
  if (pending.resource === "quickstart") return pending.method === "POST" && Object.keys(body).length === 0 && validReceipt("quickstart", body, data.marker);
  const service = data.delivery.map(d => d.service).find(s => s && pending.resource === `markets/${s.market_id}/countries/TW/delivery-services/${s.code}`);
  return service ? validReceipt(pending.resource, body, service) : false;
}
