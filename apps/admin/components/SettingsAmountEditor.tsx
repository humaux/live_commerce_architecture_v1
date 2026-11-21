// Purpose: Edit one CVS pickup cap or one Taiwanese delivery service's flat fee.
// Depends on: existing versioned settings APIs, logistics model, money format and settings-v2 command receipts.
// Used by: SettingsV2. An explicit save owns its original amounts, versions and keys through reload and partial delivery.
"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { settingsV2Copy, type Locale } from "@live-commerce/i18n";
import { fromMinor, toMinor } from "@/lib/catalog-v2-model";
import { cvsSettingsBody, parseCvsSettings, type CvsSettings } from "@/lib/logistics-model";
import { logisticsCopy } from "@/lib/logistics-copy";
import type { Page } from "@/lib/model";
import { readSettings, sessionBoundary } from "@/lib/settings-client";
import type { Market, Policy, Service } from "@/lib/settings-model";
import { execute, validReceipt, SaveFailure, serviceInput, type Journal } from "@/lib/settings-v2";

export type AmountItem = "pickupLimit" | "shippingFee";
type Fee = { service: Service; policy: Policy };
type Intent = { amount: string; commands: Journal[]; index: number; bindingRejected?: boolean };
const pickup = "logistics/cvs-settings";

async function list<T>(store: string, resource: string) {
  const rows: T[] = [], cursors = new Set<string>();
  let cursor = "";
  do {
    if (cursors.has(cursor)) throw new Error("repeated_cursor");
    cursors.add(cursor);
    const page = await readSettings<Page<T>>(store, cursor ? `${resource}?cursor=${encodeURIComponent(cursor)}` : resource);
    rows.push(...page.items);
    cursor = page.next_cursor;
  } while (cursor);
  return rows;
}

function command(resource: string, body: unknown): Journal {
  return { key: crypto.randomUUID(), resource, method: "PUT", body: JSON.stringify(body) };
}

export function SettingsAmountEditor({ store, locale, item, onBack, onBusy }: {
  store: string; locale: Locale; item: AmountItem; onBack: () => void; onBusy: (busy: boolean) => void;
}) {
  const c = settingsV2Copy[locale];
  const [choices, setChoices] = useState<{ path: string; name: string }[]>([]);
  const [path, setPath] = useState(item === "pickupLimit" ? pickup : "");
  const [cvs, setCvs] = useState<CvsSettings | null>(null), [fee, setFee] = useState<Fee | null>(null);
  const [amount, setAmount] = useState(""), [intent, setIntent] = useState<Intent | null>(null);
  const [scope, setScope] = useState(""), [notice, setNotice] = useState(""), [problem, setProblem] = useState("");
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [reload, setReload] = useState(0);
  const writing = useRef(false), mounted = useRef(false);
  const key = `commerce-settings-amount:${store}:${scope}:${path}`;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);

  useEffect(() => {
    let active = true;
    setLoading(true); setProblem(""); setIntent(null); setCvs(null); setFee(null);
    async function load() {
      const boundary = await sessionBoundary();
      if (item === "shippingFee" && !path) {
        const markets = (await list<Market>(store, "markets")).filter(m => m.active);
        const groups = await Promise.all(markets.map(async market => {
          if (!/^[0-9a-f-]{36}$/.test(market.id)) throw new Error("market");
          const base = `markets/${market.id}/countries/TW/delivery-services`;
          return (await list<Service>(store, base)).map(s => {
            if (!/^[a-z][a-z0-9_-]{0,39}$/.test(s.code) || s.market_id !== market.id || s.country !== "TW") throw new Error("service");
            const name = locale === "en" ? s.name_en : locale === "zh-TW" ? s.name_hant : s.name_hans;
            return { path: `${base}/${s.code}`, name: `${market.name} · ${name} (${s.code})` };
          });
        }));
        if (boundary !== await sessionBoundary()) throw new Error("session");
        if (!active) return;
        const options = groups.flat();
        setChoices(options);
        if (options.length === 1) setPath(options[0].path);
        return;
      }
      const cached = localStorage.getItem(`commerce-settings-amount:${store}:${boundary}:${path}`);
      const plan: Intent | null = cached ? JSON.parse(cached) : null;
      if (plan) {
        const paths = item === "pickupLimit" ? [pickup] : [path + "/policy", path];
        if (plan.commands.length !== paths.length || !Number.isInteger(plan.index) || !paths[plan.index] ||
            typeof plan.amount !== "string" || !plan.commands.every((cmd, i) => cmd.resource === paths[i] && cmd.method === "PUT" && typeof cmd.body === "string" && /^[0-9a-f-]{36}$/.test(cmd.key))) throw new Error("intent");
      }
      if (item === "pickupLimit") {
        const value = parseCvsSettings(await readSettings(store, path));
        if (boundary !== await sessionBoundary()) throw new Error("session");
        if (!active) return;
        setCvs(value); setAmount(plan?.amount ?? String(value.pay_at_pickup_max_twd ?? ""));
      } else {
        const [service, policy] = await Promise.all([readSettings<Service>(store, path), readSettings<Policy>(store, path + "/policy")]);
        if (path !== `markets/${service.market_id}/countries/${service.country}/delivery-services/${service.code}` ||
            policy.market_id !== service.market_id || policy.country !== service.country || policy.currency !== service.currency ||
            policy.method !== service.policy_method || policy.method !== `delivery:${service.code}` || policy.shipping_mode !== "country_flat" ||
            !Number.isSafeInteger(policy.version) || policy.version < 1 || !Number.isSafeInteger(policy.shipping_minor) || policy.shipping_minor < 0 ||
            !Number.isSafeInteger(policy.tax_rate_bps) || !Number.isSafeInteger(policy.quote_ttl_seconds) ||
            !["none", "exclusive", "inclusive"].includes(policy.tax_mode) || !["goods", "goods_and_shipping"].includes(policy.tax_basis) ||
            (policy.free_shipping_threshold_minor != null && !Number.isSafeInteger(policy.free_shipping_threshold_minor)) ||
            policy.enabled !== true || !Number.isSafeInteger(service.version) || service.version < 1 || typeof service.enabled !== "boolean" ||
            typeof service.visible !== "boolean" || !Number.isSafeInteger(service.sort_order) ||
            ![service.name_hans, service.name_hant, service.name_en].every(n => typeof n === "string" && n.trim()) ||
            (!plan && policy.version !== service.policy_version)) throw new Error("fee_baseline");
        if (boundary !== await sessionBoundary()) throw new Error("session");
        if (!active) return;
        setFee({ service, policy }); setAmount(plan?.amount ?? fromMinor(policy.shipping_minor, policy.currency));
      }
      setIntent(plan); setScope(boundary);
    }
    void load().catch(() => { if (active) setProblem(c.reloadHelp); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [store, locale, item, path, reload, c.reloadHelp]);

  async function save(event: FormEvent) {
    event.preventDefault();
    if (writing.current || (!cvs && !fee)) return;
    writing.current = true; setBusy(true); onBusy(true); setNotice(""); setProblem("");
    let plan = intent;
    try {
      if (!navigator.locks) throw new Error("lock");
      await navigator.locks.request(`settings-v2:${store}:${scope}`, { ifAvailable: true }, async lock => {
        if (!lock || scope !== await sessionBoundary() || sessionStorage.getItem(`commerce-settings-v2:${store}:${scope}:command`) || localStorage.getItem(key) !== (intent && JSON.stringify(intent))) throw new Error("stale");
        if (!plan) {
          let commands: Journal[];
          if (cvs) {
            const body = cvsSettingsBody({ expectedVersion: cvs.version, chains: cvs.enabled_chains, payAtPickup: cvs.pay_at_pickup_enabled, maxTWD: amount, maxOpen: String(cvs.pay_at_pickup_max_open) });
            if (!body) { setProblem(logisticsCopy[locale].cvsInvalid); return; }
            commands = [command(pickup, { ...body, enabled_chains: cvs.enabled_chains })];
          } else if (fee) {
            const { service, policy } = fee, minor = toMinor(amount, policy.currency);
            if (minor === null) { setProblem(c.invalidFee); return; }
            const { version, free_shipping_threshold_minor } = policy;
            commands = [command(path + "/policy", { market_id: policy.market_id, country: policy.country, method: policy.method, currency: policy.currency,
              shipping_mode: policy.shipping_mode, tax_basis: policy.tax_basis, tax_mode: policy.tax_mode, tax_rate_bps: policy.tax_rate_bps,
              quote_ttl_seconds: policy.quote_ttl_seconds, enabled: policy.enabled, ...(free_shipping_threshold_minor == null ? {} : { free_shipping_threshold_minor }),
              shipping_minor: minor, expected_version: version, configuration_ref: c.shippingFee }),
              command(path, { ...serviceInput(service, service.enabled), policy_version: version + 1 })];
          } else return;
          plan = { amount, commands, index: 0 };
        }
        if (plan.bindingRejected && fee) {
          // The service rejected the old CAS. On explicit confirmation, preserve the freshly read service's other fields.
          const intended = JSON.parse(plan.commands[0].body);
          if (!validReceipt(path + "/policy", intended, fee.policy)) throw new Error("policy_changed");
          plan = { ...plan, bindingRejected: false, commands: [plan.commands[0],
            command(path, { ...serviceInput(fee.service, fee.service.enabled), policy_version: fee.policy.version })] };
        }
        // Persist the complete two-command intent *before* the policy PUT. A lost response or a tab close cannot lose the required service binding.
        localStorage.setItem(key, JSON.stringify(plan)); setIntent(plan);
        try {
          while (plan.index < plan.commands.length) {
            if (!mounted.current || scope !== await sessionBoundary()) throw new Error("session");
            const cmd = plan.commands[plan.index];
            await execute(store, scope, cmd.resource, "PUT", JSON.parse(cmd.body), cmd, () => {});
            if (plan.index + 1 === plan.commands.length) break;
            plan = { ...plan, index: plan.index + 1 };
            localStorage.setItem(key, JSON.stringify(plan)); if (mounted.current) setIntent(plan);
          }
        } catch (error) {
          if (!(error instanceof SaveFailure) || error.uncertain || error.session) throw error;
          if (plan.index === 0) localStorage.removeItem(key);
          else localStorage.setItem(key, JSON.stringify({ ...plan, bindingRejected: true }));
          if (mounted.current) { setNotice(plan.index ? c.partial : c.failed); setReload(n => n + 1); }
          return;
        }
        localStorage.removeItem(key);
        if (mounted.current && scope === await sessionBoundary()) { setIntent(null); setNotice(c.saved); setReload(n => n + 1); }
      });
    } catch {
      if (mounted.current) setProblem(plan?.index ? c.partial : plan ? c.pending : c.reloadHelp);
    } finally { writing.current = false; if (mounted.current) { setBusy(false); onBusy(false); } }
  }

  const unchanged = cvs ? Number(amount) === cvs.pay_at_pickup_max_twd : fee ? toMinor(amount, fee.policy.currency) === fee.policy.shipping_minor : true;
  return <section>
    {item === "shippingFee" && choices.length > 0 && <label>{c.deliveryTitle}
      <select value={path} disabled={busy} onChange={e => { setPath(e.target.value); setNotice(""); }}>
        <option value="">{c.selectService}</option>{choices.map(o => <option key={o.path} value={o.path}>{o.name}</option>)}
      </select></label>}
    {loading ? <p role="status">{c.loading}</p> : item === "shippingFee" && !choices.length && !problem && <p>{c.noService}</p>}
    {!loading && (cvs || fee) && <form onSubmit={save}>
      <label>{cvs ? logisticsCopy[locale].cvsMaxTWD : `${c.shippingFee} (${fee?.policy.currency})`}
        <input required inputMode={cvs || fee?.policy.currency === "TWD" ? "numeric" : "decimal"} value={amount}
          disabled={busy || !!intent} onChange={e => { setAmount(e.target.value); setNotice(""); }} /></label>
      <button className="settings-v2-primary" disabled={busy || (!intent && unchanged)}>
        {intent && !intent.bindingRejected ? logisticsCopy[locale].retrySame : c.save}</button>
    </form>}
    {problem && <p role="alert">{problem} <button type="button" disabled={busy} onClick={() => setReload(n => n + 1)}>{c.reload}</button></p>}
    {notice && <p role="status">{notice}</p>}
    <button type="button" disabled={busy} className="settings-v2-secondary" onClick={onBack}>{c.backOverview}</button>
  </section>;
}
