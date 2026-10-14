// Purpose: Owns the merchant setup sequence and composes integration settings cards.
// Depends on: react, @live-commerce/i18n, ./AdminPageHeader, ./WorkspaceFrame, ./LogisticsSettings, ./BankTransferSettings, ./CodSettings, ./NotifySettings, ./StorefrontSettings, ./MetaConnect, @/lib/operations-copy, @/lib/operations-model, @/lib/settings-copy, @/lib/catalog-v2-model, @/lib/client, @/lib/orders-model, @/lib/settings-client, @/lib/settings-model, @/lib/logistics-client, @/lib/logistics-model, @/lib/logistics-copy, @/lib/model, @/lib/settings-draft, ./settings.css
// Used by: apps/admin/app/[locale]/settings/page.tsx
"use client";

// Purpose: merchant setup and provider/Meta settings, forwarding the authenticated store permissions.
// Depends on: settings read/write clients, MetaConnect and the store list authority.
// Used by: localized admin settings page.
// Merchant settings wizard (approved A four-step sequence): BFF /api/stores/{store}/markets/...
// -> Go internal/httpapi settings routes. Card payments are the platform's (settings/payments/card); this wizard has no
// payment-provider credentials. Step 2 hosts the logistics cards
// (<LogisticsSettings>: BFF logistics/ecpay, logistics/ecpay/enabled, logistics/cvs-settings -> Go
// internal/httpapi/cvs.go, taiwan-cvs-logistics-v1 §8/§16.5) in step 2, and its delivery-service editor offers
// mode "API (ECPay)" for CVS kinds only while the ECPay connection is enabled and checked (§4.1 predicate).
// <StorefrontSettings> (below the steps) is the storefront publish/unpublish card: BFF storefront, storefront/publication
// -> Go internal/httpapi/storefront.go (published-storefront-resolver-v1 "Writer (R3)").
// <MetaConnect> (below it) is the merchant Facebook Page / Instagram connect card: BFF /api/meta/{connect,callback} and
// /api/stores/{store}/meta-connect/* -> Go internal/httpapi/meta_connect.go (meta-claims-intake-v1 "Merchant connect (R4)").
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import type { Locale } from "@live-commerce/i18n";
import { operationsCopy } from "@/lib/operations-copy";
import { canReadOperations } from "@/lib/operations-model";
import { AdminPageHeader } from "./AdminPageHeader";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { LogisticsSettings } from "./LogisticsSettings";
import { BankTransferSettings } from "./BankTransferSettings";
import { CodSettings } from "./CodSettings";
import { NotifySettings } from "./NotifySettings";
import { StorefrontSettings } from "./StorefrontSettings";
import { MetaConnect } from "./MetaConnect";
import { settingsCopy } from "@/lib/settings-copy";
import { cardPaymentsCopy } from "@/lib/card-payments-copy.ts";
import { fromMinor, toMinor } from "@/lib/catalog-v2-model";
import { currencySign } from "@/lib/client";
import { wholeOnly } from "@/lib/orders-model";
import {
  integer,
  readSettings,
  safeError,
  sessionBoundary,
  validCode,
  validCountry,
  writeSettings,
  type Pending,
} from "@/lib/settings-client";
import {
  type Market,
  type Policy,
  type Service,
  type SettingsInitial,
} from "@/lib/settings-model";
import { readEcpay } from "@/lib/logistics-client";
import { ecpayQualified } from "@/lib/logistics-model";
import { logisticsCopy } from "@/lib/logistics-copy";
import type { APIError, Page } from "@/lib/model";
import {
  emptyDraft,
  unknown,
  type Draft,
  type Observation,
} from "@/lib/settings-draft";
import "./settings.css";

// A missing/unresolved read is not a version conflict. Do not offer Save until
// the target's baseline is known; retain baseline()'s real stale-version guard.
function hasBaseline(observed: Observation | null, target: string) {
  return !!observed && !!target && observed.target === target && observed.version >= 0;
}

/** Owns the merchant setup sequence and composes integration settings cards. User actions submit setup changes through settings-client; nested cards own their commands. */
export function SettingsWizard({
  locale,
  initial,
}: {
  locale: Locale;
  initial: SettingsInitial;
}) {
  const c = settingsCopy[locale],
    lc = logisticsCopy[locale],
    store = initial.store;
  const [step, setStep] = useState<1 | 2 | 3 | 4>(1);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [markets, setMarkets] = useState<Market[]>([]),
    [marketCursor, setMarketCursor] = useState("");
  const [services, setServices] = useState<Service[]>([]);
  const [serviceCursor, setServiceCursor] = useState("");
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const [ready, setReady] = useState(false),
    [loading, setLoading] = useState(false),
    [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState(""),
    [error, setError] = useState<APIError | null>(initial.error);
  const boundary = useRef(""),
    draftKey = useRef(""),
    journalKey = useRef("");
  const epoch = useRef(0),
    mounted = useRef(true),
    busyRef = useRef(false);
  const marketRead = useRef(0),
    policyRead = useRef(0);
  const serviceRead = useRef(0),
    marketPageRead = useRef(0),
    servicePageRead = useRef(0);
  const draftRef = useRef(draft);
  draftRef.current = draft;
  // Currency the money fields of a market are typed in (a market is created in the store currency; hydration runs in async callbacks).
  const marketsRef = useRef(markets);
  marketsRef.current = markets;
  const currencyOf = (marketID: string) =>
    marketsRef.current.find((item) => item.id === marketID)?.currency ?? store?.currency ?? "TWD";
  const stepRef = useRef(step);
  stepRef.current = step;
  // Whether "API (ECPay)" may be chosen for a CVS service: the store's ECPay connection is enabled and checked.
  // Read-only probe (GET logistics/ecpay, integration:read); a 403/404/failure simply leaves API mode unavailable.
  const [ecpayReady, setEcpayReady] = useState(false);
  const activeMarket =
    markets.find((item) => item.id === draft.marketID) ?? null;
  const activeService =
    services.find((item) => item.code === draft.serviceCode) ?? null;
  const serviceTarget =
    draft.marketID &&
    validCountry(draft.country) &&
    validCode(draft.serviceCode)
      ? `${draft.marketID}:${draft.country}:${draft.serviceCode}`
      : "";

  const probeEcpay = store?.id ?? "";
  const probeStep = ready && step === 3;
  useEffect(() => {
    if (!probeEcpay || !probeStep) return;
    const active = new AbortController();
    readEcpay(probeEcpay, active.signal).then(
      (value) => setEcpayReady(ecpayQualified(value)),
      () => !active.signal.aborted && setEcpayReady(false),
    );
    return () => active.abort();
  }, [probeEcpay, probeStep]);

  function saveDraft(next: Draft, atStep = stepRef.current) {
    persistDraft(next, atStep);
    draftRef.current = next;
    setDraft(next);
  }
  function hydrateService(current: Draft, saved: Service | null): Draft {
    return {
      ...current,
      nameHans: saved?.name_hans ?? "",
      nameHant: saved?.name_hant ?? "",
      nameEN: saved?.name_en ?? "",
      serviceKind: saved?.delivery_kind ?? current.serviceKind,
      serviceMode: saved?.mode ?? "MANUAL",
      serviceEnabled: saved?.enabled ?? false,
      serviceVisible: saved?.visible ?? false,
      sort: String(saved?.sort_order ?? 10),
      serviceObservation: {
        target: `${current.marketID}:${current.country}:${current.serviceCode}`,
        version: saved?.version ?? 0,
        dirty: false,
      },
    };
  }
  function hydratePolicy(current: Draft, saved: Policy | null): Draft {
    return {
      ...current,
      shipping: fromMinor(saved?.shipping_minor ?? 0, currencyOf(current.marketID)),
      freeShipping:
        saved?.free_shipping_threshold_minor == null
          ? ""
          : fromMinor(saved.free_shipping_threshold_minor, currencyOf(current.marketID)),
      taxMode: saved?.tax_mode ?? "none",
      taxBasis: saved?.tax_basis ?? "goods",
      taxRate: String(saved?.tax_rate_bps ?? 0),
      ttl: String(saved?.quote_ttl_seconds ?? 300),
      policyEnabled: saved?.enabled ?? false,
      reference: "",
      policyObservation: {
        target: `${current.marketID}:${current.country}:${current.serviceCode}`,
        version: saved?.version ?? 0,
        dirty: false,
      },
    };
  }

  async function stillSession() {
    try {
      return (await sessionBoundary()) === boundary.current;
    } catch {
      return false;
    }
  }
  function displayError(value: APIError | null) {
    if (!value) return "";
    if (value.code === "command_storage_unavailable") return c.storage;
    if (value.code === "session_changed" || value.code === "unauthorized")
      return c.session;
    if (value.code === "conflict") return c.conflict;
    if (value.code === "forbidden") return c.forbidden;
    if (value.code === "invalid_money") return c.invalidMoney;
    if (value.code === "invalid_request" || value.code === "invalid_json")
      return c.invalid;
    return c.failed;
  }
  function storageError() {
    setError({
      ...unknown,
      code: "command_storage_unavailable",
      retryable: false,
    });
    setReady(false);
  }
  function persistDraft(next: Draft, nextStep: number) {
    if (!draftKey.current) throw new Error("storage");
    const encoded = JSON.stringify({ version: 2, draft: next, step: nextStep });
    sessionStorage.setItem(draftKey.current, encoded);
    if (sessionStorage.getItem(draftKey.current) !== encoded)
      throw new Error("storage");
  }
  function update<K extends keyof Draft>(field: K, value: Draft[K]) {
    const next = { ...draft, [field]: value };
    // API (ECPay) is a CVS-only mode: choosing home delivery puts the service back to manual.
    if (field === "serviceKind" && value === "home") next.serviceMode = "MANUAL";
    if (
      field === "serviceCode" ||
      field === "marketID" ||
      field === "country"
    ) {
      setPolicy(null);
      next.policyObservation = null;
      next.serviceObservation = null;
      policyRead.current += 1;
      serviceRead.current += 1;
      marketRead.current += 1;
    }
    if (
      [
        "nameHans",
        "nameHant",
        "nameEN",
        "serviceKind",
        "serviceMode",
        "serviceEnabled",
        "serviceVisible",
        "sort",
      ].includes(field)
    )
      next.serviceObservation = {
        target: serviceTarget,
        version: next.serviceObservation?.version ?? -1,
        dirty: true,
      };
    if (
      [
        "shipping",
        "freeShipping",
        "taxMode",
        "taxBasis",
        "taxRate",
        "ttl",
        "policyEnabled",
        "reference",
      ].includes(field)
    )
      next.policyObservation = {
        target: serviceTarget,
        version: next.policyObservation?.version ?? -1,
        dirty: true,
      };
    try {
      persistDraft(next, step);
      draftRef.current = next;
      setDraft(next);
      setError(null);
    } catch {
      storageError();
    }
  }
  function goStep(next: 1 | 2 | 3 | 4) {
    try {
      persistDraft(draft, next);
      setStep(next);
      setError(null);
      setNotice("");
    } catch {
      storageError();
    }
  }

  useEffect(() => {
    mounted.current = true;
    if (!store) return;
    let active = true;
    void (async () => {
      try {
        if (!navigator.locks) throw new Error("storage");
        const hash = await sessionBoundary();
        if (!active) return;
        boundary.current = hash;
        draftKey.current = `commerce-settings-draft:${store.id}:${hash}`;
        journalKey.current = `commerce-settings-command:${store.id}:${hash}`;
        const rawDraft = sessionStorage.getItem(draftKey.current);
        const rawPending = localStorage.getItem(journalKey.current);
        // A version-1 draft holds minor-unit money text ("6000" = NT$60 now): reading it as major units would multiply every amount by 100.
        const legacyDraft =
          !!rawDraft && (JSON.parse(rawDraft) as { version?: number }).version === 1;
        if (legacyDraft) sessionStorage.removeItem(draftKey.current);
        if (rawDraft && !legacyDraft) {
          const saved = JSON.parse(rawDraft) as {
            version?: number;
            draft?: Draft;
            step?: number;
          };
          if (
            saved.version !== 2 ||
            !saved.draft ||
            ![1, 2, 3, 4].includes(saved.step ?? 0)
          )
            throw new Error("storage");
          // A draft saved before serviceMode existed reads as MANUAL.
          setDraft({ ...emptyDraft, ...saved.draft });
          setStep(saved.step as 1 | 2 | 3 | 4);
        }
        if (rawPending) {
          const saved = JSON.parse(rawPending) as Pending;
          if (
            saved &&
            typeof saved.resource === "string" &&
            /^(provider-accounts|markets\/[0-9a-f-]{36}\/countries\/[A-Z]{2}\/payment-methods)/.test(
              saved.resource,
            )
          ) {
            // Journal of a removed payment-provider write: nothing left to retry, so discard it rather than block the wizard.
            localStorage.removeItem(journalKey.current);
          } else {
            if (
              !saved ||
              !/^[A-Za-z0-9_.:-]{8,128}$/.test(saved.key) ||
              !/^(markets(?:\/[0-9a-f-]{36}\/countries\/[A-Z]{2}\/delivery-services\/[a-z][a-z0-9_-]{0,39}(?:\/policy)?)?)$/.test(
                saved.resource,
              ) ||
              typeof saved.body !== "string"
            )
              throw new Error("storage");
            setPending(saved);
            setNotice(c.unknown);
          }
        }
        setReady(true);
      } catch {
        if (active) storageError();
      }
    })();
    return () => {
      active = false;
      mounted.current = false;
      epoch.current += 1;
    };
    // Locale changes remount but retain the session-scoped nonsecret draft.
  }, [store?.id]);

  const loadBase = useCallback(async () => {
    if (!store) return;
    const token = epoch.current;
    marketPageRead.current += 1;
    setLoading(true);
    try {
      const marketPage = await readSettings<Page<Market>>(store.id, "markets");
      if (
        !mounted.current ||
        token !== epoch.current ||
        (await sessionBoundary()) !== boundary.current
      )
        return;
      setMarkets(marketPage.items);
      setMarketCursor(marketPage.next_cursor);
    } catch (value) {
      if (mounted.current && token === epoch.current && (await stillSession()))
        setError(safeError(value));
    } finally {
      if (mounted.current && token === epoch.current) setLoading(false);
    }
  }, [store]);
  useEffect(() => {
    if (ready) void loadBase();
  }, [ready, loadBase]);

  const loadMarket = useCallback(
    async (marketID: string, country: string) => {
      if (!store || !marketID || !validCountry(country)) return;
      const token = epoch.current,
        request = ++marketRead.current;
      servicePageRead.current += 1;
      setLoading(true);
      try {
        const base = `markets/${marketID}/countries/${country}`;
        const servicePage = await readSettings<Page<Service>>(
          store.id,
          `${base}/delivery-services`,
        );
        if (
          !mounted.current ||
          token !== epoch.current ||
          request !== marketRead.current ||
          draftRef.current.marketID !== marketID ||
          draftRef.current.country !== country ||
          (await sessionBoundary()) !== boundary.current
        )
          return;
        setServices(servicePage.items);
        setServiceCursor(servicePage.next_cursor);
      } catch (value) {
        if (
          mounted.current &&
          token === epoch.current &&
          request === marketRead.current &&
          draftRef.current.marketID === marketID &&
          draftRef.current.country === country &&
          (await stillSession())
        )
          setError(safeError(value));
      } finally {
        if (
          mounted.current &&
          token === epoch.current &&
          request === marketRead.current
        )
          setLoading(false);
      }
    },
    [store],
  );
  useEffect(() => {
    if (ready && draft.marketID) void loadMarket(draft.marketID, draft.country);
  }, [ready, draft.marketID, draft.country, loadMarket]);

  async function moreMarkets() {
    if (!store || !ready || !marketCursor) return;
    const token = epoch.current,
      request = ++marketPageRead.current,
      cursor = marketCursor;
    try {
      const page = await readSettings<Page<Market>>(
        store.id,
        `markets?cursor=${encodeURIComponent(cursor)}`,
      );
      if (
        !mounted.current ||
        token !== epoch.current ||
        request !== marketPageRead.current ||
        (await sessionBoundary()) !== boundary.current
      )
        return;
      setMarkets((rows) => [
        ...rows,
        ...page.items.filter((item) => !rows.some((row) => row.id === item.id)),
      ]);
      setMarketCursor(page.next_cursor);
    } catch (issue) {
      if (
        mounted.current &&
        request === marketPageRead.current &&
        (await stillSession())
      )
        setError(safeError(issue));
    }
  }
  async function moreServices() {
    if (!store || !ready || !activeMarket || !serviceCursor) return;
    const token = epoch.current,
      request = ++servicePageRead.current;
    const marketID = activeMarket.id,
      country = draft.country,
      cursor = serviceCursor;
    try {
      const page = await readSettings<Page<Service>>(
        store.id,
        `markets/${marketID}/countries/${country}/delivery-services?cursor=${encodeURIComponent(cursor)}`,
      );
      if (
        !mounted.current ||
        token !== epoch.current ||
        request !== servicePageRead.current ||
        draftRef.current.marketID !== marketID ||
        draftRef.current.country !== country ||
        (await sessionBoundary()) !== boundary.current
      )
        return;
      setServices((rows) => [
        ...rows,
        ...page.items.filter(
          (item) => !rows.some((row) => row.code === item.code),
        ),
      ]);
      setServiceCursor(page.next_cursor);
    } catch (issue) {
      if (
        mounted.current &&
        request === servicePageRead.current &&
        draftRef.current.marketID === marketID &&
        draftRef.current.country === country &&
        (await stillSession())
      )
        setError(safeError(issue));
    }
  }

  const loadPolicy = useCallback(
    async (marketID: string, country: string, code: string) => {
      if (!store || !marketID || !validCountry(country) || !validCode(code)) {
        setPolicy(null);
        return;
      }
      const token = epoch.current,
        request = ++policyRead.current;
      const target = `${marketID}:${country}:${code}`;
      try {
        const result = await readSettings<Policy>(
          store.id,
          `markets/${marketID}/countries/${country}/delivery-services/${code}/policy`,
        );
        if (
          mounted.current &&
          token === epoch.current &&
          request === policyRead.current &&
          `${draftRef.current.marketID}:${draftRef.current.country}:${draftRef.current.serviceCode}` ===
            target &&
          (await sessionBoundary()) === boundary.current
        ) {
          setPolicy(result);
          const current = draftRef.current,
            observed = current.policyObservation;
          if (!observed || observed.target !== target || !observed.dirty)
            saveDraft(hydratePolicy(current, result));
          else if (observed.version !== result.version)
            setError({ ...unknown, code: "conflict" });
        }
      } catch (value) {
        const issue = safeError(value);
        if (
          mounted.current &&
          token === epoch.current &&
          request === policyRead.current &&
          `${draftRef.current.marketID}:${draftRef.current.country}:${draftRef.current.serviceCode}` ===
            target &&
          (await stillSession())
        ) {
          setPolicy(null);
          if (issue.code === "not_found") {
            const current = draftRef.current,
              observed = current.policyObservation;
            if (!observed || observed.target !== target || !observed.dirty)
              saveDraft(hydratePolicy(current, null));
            else if (observed.version === -1)
              saveDraft({
                ...current,
                policyObservation: { ...observed, version: 0 },
              });
            else if (observed.version !== 0)
              setError({ ...unknown, code: "conflict" });
          } else setError(issue);
        }
      }
    },
    [store],
  );
  useEffect(() => {
    if (ready) void loadPolicy(draft.marketID, draft.country, draft.serviceCode);
  }, [
    ready,
    draft.marketID,
    draft.country,
    draft.serviceCode,
    loadPolicy,
  ]);

  const loadService = useCallback(
    async (marketID: string, country: string, code: string) => {
      if (!store || !marketID || !validCountry(country) || !validCode(code))
        return;
      const token = epoch.current,
        request = ++serviceRead.current;
      const target = `${marketID}:${country}:${code}`;
      try {
        const result = await readSettings<Service>(
          store.id,
          `markets/${marketID}/countries/${country}/delivery-services/${code}`,
        );
        if (
          !mounted.current ||
          token !== epoch.current ||
          request !== serviceRead.current ||
          `${draftRef.current.marketID}:${draftRef.current.country}:${draftRef.current.serviceCode}` !==
            target ||
          (await sessionBoundary()) !== boundary.current
        )
          return;
        setServices((rows) => [
          ...rows.filter((row) => row.code !== result.code),
          result,
        ]);
        const current = draftRef.current,
          observed = current.serviceObservation;
        if (!observed || observed.target !== target || !observed.dirty)
          saveDraft(hydrateService(current, result));
        else if (observed.version !== result.version)
          setError({ ...unknown, code: "conflict" });
      } catch (value) {
        const issue = safeError(value);
        if (
          !mounted.current ||
          token !== epoch.current ||
          request !== serviceRead.current ||
          `${draftRef.current.marketID}:${draftRef.current.country}:${draftRef.current.serviceCode}` !==
            target ||
          !(await stillSession())
        )
          return;
        if (issue.code === "not_found") {
          setServices((rows) => rows.filter((row) => row.code !== code));
          const current = draftRef.current,
            observed = current.serviceObservation;
          if (!observed || observed.target !== target || !observed.dirty)
            saveDraft(hydrateService(current, null));
          else if (observed.version === -1)
            saveDraft({
              ...current,
              serviceObservation: { ...observed, version: 0 },
            });
          else if (observed.version !== 0)
            setError({ ...unknown, code: "conflict" });
        } else setError(issue);
      }
    },
    [store],
  );
  useEffect(() => {
    if (ready) void loadService(draft.marketID, draft.country, draft.serviceCode);
  }, [
    ready,
    draft.marketID,
    draft.country,
    draft.serviceCode,
    loadService,
  ]);

  async function savePending(
    command: Pending,
    body: string,
    after: (value: unknown) => void,
  ) {
    if (!store || busyRef.current || !ready) return;
    busyRef.current = true;
    setBusy(true);
    setError(null);
    setNotice("");
    const startEpoch = epoch.current;
    try {
      await navigator.locks.request(journalKey.current, async () => {
        if ((await sessionBoundary()) !== boundary.current)
          throw new Error("session_changed");
        const raw = localStorage.getItem(journalKey.current);
        if (raw) {
          const previous = JSON.parse(raw) as Pending;
          if (previous.key !== command.key) {
            setPending(previous);
            setNotice(c.unknown);
            return;
          }
          command = previous;
          body = command.body!;
        } else {
          const encoded = JSON.stringify(command);
          localStorage.setItem(journalKey.current, encoded);
          if (localStorage.getItem(journalKey.current) !== encoded)
            throw new Error("storage");
        }
        setPending(command);
        const result = await writeSettings<unknown>(
          store.id,
          command,
          body,
          boundary.current,
        );
        if ((await sessionBoundary()) !== boundary.current)
          throw new Error("session_changed");
        if (!mounted.current || startEpoch !== epoch.current) return;
        if (result.uncertain) {
          const retry = { ...command, uncertain: true };
          const encoded = JSON.stringify(retry);
          localStorage.setItem(journalKey.current, encoded);
          if (localStorage.getItem(journalKey.current) !== encoded)
            throw new Error("storage");
          setPending(retry);
          setNotice(c.unknown);
          setError(result.error);
          return;
        }
        localStorage.removeItem(journalKey.current);
        setPending(null);
        if (result.error) {
          setError(result.error);
          return;
        }
        after(result.value);
      });
    } catch (value) {
      if (value instanceof Error && value.message === "session_changed") {
        try {
          localStorage.removeItem(journalKey.current);
          sessionStorage.removeItem(draftKey.current);
        } catch {
          /* session is lost */
        }
        setPending(null);
        setDraft(emptyDraft);
        setReady(false);
        setError({ ...unknown, code: "session_changed", retryable: false });
      } else storageError();
    } finally {
      busyRef.current = false;
      if (mounted.current) setBusy(false);
    }
  }
  function command(
    method: "POST" | "PUT",
    resource: string,
    payload: unknown,
    after: (value: unknown) => void,
  ) {
    if (pending) {
      setNotice(c.unknown);
      return;
    }
    const body = JSON.stringify(payload);
    void savePending(
      { key: crypto.randomUUID(), method, resource, body },
      body,
      after,
    );
  }
  function retryPending() {
    if (!pending) return;
    void savePending(pending, pending.body ?? "", (value) => {
      const resource = pending.resource;
      if (resource === "markets") {
        const saved = value as Market;
        setMarkets((rows) => [
          ...rows.filter((row) => row.id !== saved.id),
          saved,
        ]);
        commitDraft(
          {
            ...draft,
            marketID: saved.id,
            policyObservation: null,
            serviceObservation: null,
          },
          3,
        );
        setNotice(c.saved);
      } else if (resource.endsWith("/policy")) {
        const saved = value as Policy;
        setPolicy(saved);
        saveDraft({
          ...draftRef.current,
          reference: "",
          policyObservation: {
            target: `${saved.market_id}:${saved.country}:${draft.serviceCode}`,
            version: saved.version,
            dirty: false,
          },
        });
        setNotice(c.policySaved);
      } else if (resource.includes("delivery-services")) {
        const saved = value as Service;
        setServices((rows) => [
          ...rows.filter((row) => row.code !== saved.code),
          saved,
        ]);
        commitDraft(
          {
            ...draftRef.current,
            serviceObservation: {
              target: `${saved.market_id}:${saved.country}:${saved.code}`,
              version: saved.version,
              dirty: false,
            },
          },
          4,
        );
        setNotice(c.serviceSaved);
      }
    });
  }
  function commitDraft(next: Draft, nextStep: 1 | 2 | 3 | 4) {
    try {
      persistDraft(next, nextStep);
      draftRef.current = next;
      setDraft(next);
      setStep(nextStep);
    } catch {
      storageError();
    }
  }
  function baseline(
    observed: Observation | null,
    target: string,
    currentVersion: number,
  ) {
    if (
      !hasBaseline(observed, target) ||
      !observed ||
      observed.version !== currentVersion
    ) {
      setError({ ...unknown, code: "conflict" });
      return null;
    }
    return observed.version;
  }
  async function reloadCurrent() {
    if (!store || !ready || pending || busyRef.current) return;
    const next = {
      ...draft,
      policyObservation: null,
      serviceObservation: null,
      reference: "",
    };
    try {
      saveDraft(next);
      setError(null);
      setNotice("");
    } catch {
      storageError();
      return;
    }
    await loadBase();
    if (next.marketID) await loadMarket(next.marketID, next.country);
    if (validCode(next.serviceCode))
      await Promise.all([
        loadPolicy(next.marketID, next.country, next.serviceCode),
        loadService(next.marketID, next.country, next.serviceCode),
      ]);
    if (mounted.current) setNotice(c.reloaded);
  }
  function marketSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!store || !validCode(draft.marketCode) || !draft.marketName.trim()) {
      setError({ ...unknown, code: "invalid_request" });
      return;
    }
    command(
      "POST",
      "markets",
      {
        code: draft.marketCode,
        name: draft.marketName.trim(),
        currency: store.currency,
      },
      (value) => {
        const saved = value as Market;
        setMarkets((rows) => [...rows, saved]);
        commitDraft(
          {
            ...draft,
            marketID: saved.id,
            policyObservation: null,
            serviceObservation: null,
          },
          3,
        );
        setNotice(c.saved);
      },
    );
  }
  function policySubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const currency = activeMarket?.currency ?? "TWD";
    const shipping = toMinor(draft.shipping, currency),
      // Empty = no threshold; anything else is major-unit text turned into minor units here (the Go quote re-validates).
      free =
        draft.freeShipping.trim() === ""
          ? null
          : toMinor(draft.freeShipping, currency),
      tax = integer(draft.taxRate, 0, 10000),
      ttl = integer(draft.ttl, 60, 1800);
    const ref = draft.reference.trim();
    if (wholeOnly(currency) && /[.,]/.test(draft.shipping + draft.freeShipping)) {
      setError({ ...unknown, code: "invalid_money" }); // D02: NT$ fees are whole dollars
      return;
    }
    if (
      !activeMarket ||
      !validCountry(draft.country) ||
      !validCode(draft.serviceCode) ||
      (draft.serviceKind !== "home" && draft.country !== "TW") ||
      shipping === null ||
      (draft.freeShipping.trim() !== "" && free === null) ||
      tax === null ||
      ttl === null ||
      (draft.taxMode === "none" && tax !== 0) ||
      !ref ||
      [...ref].length > 240 ||
      [...ref].some((char) => /[\x00-\x1f\x7f]/.test(char))
    ) {
      setError({ ...unknown, code: "invalid_request" });
      return;
    }
    const expected = baseline(
      draft.policyObservation,
      serviceTarget,
      policy?.version ?? 0,
    );
    if (expected === null) return;
    const resource = `markets/${activeMarket.id}/countries/${draft.country}/delivery-services/${draft.serviceCode}/policy`;
    command(
      "PUT",
      resource,
      {
        market_id: activeMarket.id,
        country: draft.country,
        method: `delivery:${draft.serviceCode}`,
        currency: activeMarket.currency,
        shipping_mode: "country_flat",
        shipping_minor: shipping,
        // Absent = no threshold (Go omitempty); every save writes a full new policy version, so omitting clears it.
        ...(free === null ? {} : { free_shipping_threshold_minor: free }),
        tax_mode: draft.taxMode,
        tax_basis: draft.taxBasis,
        tax_rate_bps: tax,
        quote_ttl_seconds: ttl,
        enabled: draft.policyEnabled,
        expected_version: expected,
        configuration_ref: ref,
      },
      (value) => {
        const saved = value as Policy;
        setPolicy(saved);
        // A sibling service GET may have completed during this PUT. Applying
        // the render-time draft would discard that baseline and fake a conflict.
        saveDraft({
          ...draftRef.current,
          reference: "",
          policyObservation: {
            target: serviceTarget,
            version: saved.version,
            dirty: false,
          },
        });
        setNotice(c.policySaved);
      },
    );
  }
  function serviceSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const sort = integer(draft.sort, 0, 1000);
    if (
      !activeMarket?.active ||
      !policy?.enabled ||
      !validCode(draft.serviceCode) ||
      !validCountry(draft.country) ||
      (draft.serviceKind !== "home" && draft.country !== "TW") ||
      // API (ECPay) exists only for a CVS kind while the ECPay connection is enabled and checked.
      (draft.serviceMode === "API" && (draft.serviceKind === "home" || !ecpayReady)) ||
      sort === null ||
      !draft.nameHans.trim() ||
      !draft.nameHant.trim() ||
      !draft.nameEN.trim()
    ) {
      setError({ ...unknown, code: "invalid_request" });
      return;
    }
    const expected = baseline(
      draft.serviceObservation,
      serviceTarget,
      activeService?.version ?? 0,
    );
    if (
      expected === null ||
      baseline(draft.policyObservation, serviceTarget, policy.version) === null
    )
      return;
    const resource = `markets/${activeMarket.id}/countries/${draft.country}/delivery-services/${draft.serviceCode}`;
    command(
      "PUT",
      resource,
      {
        market_id: activeMarket.id,
        country: draft.country,
        code: draft.serviceCode,
        expected_version: expected,
        policy_version: draft.policyObservation!.version,
        name_hans: draft.nameHans.trim(),
        name_hant: draft.nameHant.trim(),
        name_en: draft.nameEN.trim(),
        delivery_kind: draft.serviceKind,
        mode: draft.serviceMode,
        enabled: draft.serviceEnabled,
        visible: draft.serviceVisible,
        sort_order: sort,
      },
      (value) => {
        const saved = value as Service;
        setServices((rows) => [
          ...rows.filter((row) => row.code !== saved.code),
          saved,
        ]);
        commitDraft(
          {
            ...draftRef.current,
            serviceObservation: {
              target: serviceTarget,
              version: saved.version,
              dirty: false,
            },
          },
          4,
        );
        setNotice(c.serviceSaved);
      },
    );
  }
  const needsPolicySave =
    !policy ||
    draft.reference.trim() !== "" ||
    policy.shipping_minor !== toMinor(draft.shipping, currencyOf(draft.marketID)) ||
    (policy.free_shipping_threshold_minor ?? "") !== (draft.freeShipping.trim() === "" ? "" : toMinor(draft.freeShipping, currencyOf(draft.marketID))) ||
    policy.tax_mode !== draft.taxMode ||
    policy.tax_basis !== draft.taxBasis ||
    policy.tax_rate_bps !== Number(draft.taxRate) ||
    policy.quote_ttl_seconds !== Number(draft.ttl) ||
    policy.enabled !== draft.policyEnabled;
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="settings"
      locked={busy}
    >
      <div className="settings-page" data-testid="settings-wizard">
        <AdminPageHeader className="settings-heading" locale={locale} description={c.subtitle} />
        {store && canReadOperations(store) && <p><a data-testid="settings-operations-link" href={`/${locale}/settings/operations?store=${store.id}`}>{operationsCopy[locale].title}</a></p>}
        <ol className="settings-steps" aria-label={c.title}>
          {c.steps.map((label, index) => {
            const number = (index + 1) as 1 | 2 | 3 | 4;
            return (
              <li
                key={label}
                className={
                  number === step
                    ? "current"
                    : number < step
                      ? "complete"
                      : "future"
                }
              >
                <button
                  type="button"
                  disabled={!ready || busy || number > step}
                  aria-current={number === step ? "step" : undefined}
                  onClick={() => goStep(number)}
                >
                  <span className="step-number">{number}</span>
                  <span>{label}</span>
                </button>
              </li>
            );
          })}
        </ol>
        <div className="settings-surface">
          <section
            className="settings-form-area"
            aria-label={c.steps[step - 1]}
          >
            {loading && (
              <p className="message" role="status">
                {c.loading}
              </p>
            )}
            {error && (
              <div className="message error" role="alert">
                <strong>{displayError(error)}</strong>
                {error.request_id && (
                  <small>
                    {c.requestID}: {error.request_id}
                  </small>
                )}
              </div>
            )}
            {notice && (
              <p className="message pending" role="status">
                {notice}
              </p>
            )}
            {pending && !busy && (
              <div className="settings-pending" data-testid="settings-pending">
                <p>{c.unknown}</p>
                <button type="button" onClick={retryPending}>
                  {c.retry}
                </button>
              </div>
            )}
            {!store ? (
              <div className="settings-empty">
                <h2>{c.noStore}</h2>
                <p>{displayError(initial.error)}</p>
                <a href={`/${locale}/`}>{c.recoverWorkspace}</a>
              </div>
            ) : null}
            {store && step === 1 && (
              <>
                <h2 className="settings-section-title">{c.platformTitle}</h2>
                <p className="settings-intro">
                  <strong>{c.manual}</strong> · {c.manualHint}
                </p>
                {/* W4-U1: platform card payments live on their own page (nav:false route); entry only. */}
                <p className="settings-note">
                  <a
                    href={`/${locale}/settings/payments/card?store=${store.id}`}
                    data-testid="settings-card-payments-link"
                  >
                    {cardPaymentsCopy[locale].entry}
                  </a>
                </p>
                <div className="settings-actions">
                  <button
                    className="primary"
                    type="button"
                    disabled={!ready || !!pending || busy}
                    onClick={() => goStep(2)}
                  >
                    {c.next}
                  </button>
                </div>
              </>
            )}
            {store && step === 2 && (
                <>
                  <h2 className="settings-section-title">{c.manual}</h2>
                  <p className="settings-note">{c.manualHint}</p>
                  <LogisticsSettings store={store.id} locale={locale} />
                  {/* storefront-v2 §C: bank-transfer details and window (BFF bank-transfer-settings -> Go offline.go). */}
                  <BankTransferSettings store={store.id} locale={locale} />
                  {/* home-cod R5: cash-on-delivery switch, cap, surcharge and carrier (BFF cash-on-delivery-settings -> Go cod.go). */}
                  <CodSettings store={store.id} locale={locale} />
                  {/* storefront-v2 §E6: new-order email opt-out (BFF notification-settings -> Go notify.go). */}
                  <NotifySettings store={store.id} locale={locale} />
                  <div className="settings-actions">
                    <button type="button" onClick={() => goStep(1)}>
                      {c.back}
                    </button>
                    <button
                      className="primary"
                      type="button"
                      disabled={!ready || !!pending || busy}
                      onClick={() => goStep(3)}
                    >
                      {c.next}
                    </button>
                  </div>
                </>
            )}
            {store && step === 3 && (
              <>
                <h2 className="settings-section-title">{c.marketTitle}</h2>
                <div className="settings-fields">
                  <label>
                    {c.market}
                    <select
                      disabled={!ready || busy || !!pending}
                      value={draft.marketID}
                      onChange={(event) =>
                        update("marketID", event.target.value)
                      }
                    >
                      <option value="">{c.createMarket}</option>
                      {markets.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name} · {item.code} · {item.currency}
                          {!item.active ? ` · ${c.marketInactive}` : ""}
                        </option>
                      ))}
                    </select>
                  </label>
                  {marketCursor && (
                    <button
                      type="button"
                      disabled={busy || !!pending}
                      onClick={() => void moreMarkets()}
                    >
                      {c.loadMore}
                    </button>
                  )}
                  {activeMarket && (
                    <button
                      type="button"
                      data-testid="settings-reload-current"
                      disabled={!ready || busy || !!pending}
                      onClick={() => void reloadCurrent()}
                    >
                      {c.reloadCurrent}
                    </button>
                  )}
                </div>
                {!activeMarket && (
                  <form
                    data-testid="settings-market-form"
                    onSubmit={marketSubmit}
                    className="settings-fields"
                  >
                    <label>
                      {c.marketCode}
                      <input
                        required
                        pattern="[a-z][a-z0-9_-]{0,39}"
                        value={draft.marketCode}
                        onChange={(event) =>
                          update("marketCode", event.target.value)
                        }
                      />
                    </label>
                    <label>
                      {c.marketName}
                      <input
                        required
                        maxLength={120}
                        value={draft.marketName}
                        onChange={(event) =>
                          update("marketName", event.target.value)
                        }
                      />
                    </label>
                    <p className="settings-note">
                      {c.currency}: {store.currency}
                    </p>
                    <div className="settings-actions">
                      <button
                        className="primary"
                        disabled={!ready || !!pending || busy}
                        type="submit"
                      >
                        {c.createMarket}
                      </button>
                    </div>
                  </form>
                )}
                {activeMarket && (
                  <>
                    {!activeMarket.active && (
                      <p className="message error">{c.marketInactive}</p>
                    )}
                    <>
                        <h2 className="settings-section-title settings-subtitle">
                          {c.deliveryTitle}
                        </h2>
                        <p className="settings-note">{c.sequential}</p>
                        <div className="settings-fields">
                          <label>
                            {c.country}
                            <input
                              maxLength={2}
                              pattern="[A-Z]{2}"
                              value={draft.country}
                              onChange={(event) =>
                                update(
                                  "country",
                                  event.target.value.toUpperCase(),
                                )
                              }
                            />
                          </label>
                          <label>
                            {c.service}
                            <select
                              value={activeService?.code ?? ""}
                              onChange={(event) => {
                                const selected = services.find(
                                  (item) => item.code === event.target.value,
                                );
                                if (selected) {
                                  const next = {
                                    ...draft,
                                    serviceCode: selected.code,
                                    policyObservation: null,
                                  };
                                  try {
                                    setPolicy(null);
                                    saveDraft(hydrateService(next, selected));
                                  } catch {
                                    storageError();
                                  }
                                } else update("serviceCode", "");
                              }}
                            >
                              <option value="">{c.createService}</option>
                              {services.map((item) => (
                                <option key={item.code} value={item.code}>
                                  {item.code}
                                </option>
                              ))}
                            </select>
                          </label>
                          {serviceCursor && (
                            <button
                              type="button"
                              disabled={busy || !!pending}
                              onClick={() => void moreServices()}
                            >
                              {c.loadMore}
                            </button>
                          )}
                          <label>
                            {c.serviceCode}
                            <input
                              required
                              pattern="[a-z][a-z0-9_-]{0,39}"
                              maxLength={40}
                              value={draft.serviceCode}
                              onChange={(event) =>
                                update("serviceCode", event.target.value)
                              }
                            />
                          </label>
                          <label>
                            {c.deliveryKind}
                            <select
                              value={draft.serviceKind}
                              onChange={(event) =>
                                update(
                                  "serviceKind",
                                  event.target.value as Draft["serviceKind"],
                                )
                              }
                            >
                              <option value="home">{c.home}</option>
                              <option value="cvs_711">{c.cvs711}</option>
                              <option value="cvs_familymart">
                                {c.cvsFamily}
                              </option>
                              <option value="cvs_hilife">{lc.chains.cvs_hilife}</option>
                              <option value="cvs_okmart">{lc.chains.cvs_okmart}</option>
                            </select>
                          </label>
                          <label>
                            {lc.modeLabel}
                            <select
                              data-testid="settings-service-mode"
                              value={draft.serviceMode}
                              disabled={draft.serviceKind === "home"}
                              onChange={(event) =>
                                update(
                                  "serviceMode",
                                  event.target.value as Draft["serviceMode"],
                                )
                              }
                            >
                              <option value="MANUAL">{lc.modeManual}</option>
                              {(ecpayReady || draft.serviceMode === "API") &&
                                draft.serviceKind !== "home" && (
                                  <option value="API" disabled={!ecpayReady}>
                                    {lc.modeApi}
                                  </option>
                                )}
                            </select>
                            <small>{lc.modeApiHint}</small>
                          </label>
                        </div>
                        <form
                          data-testid="settings-policy-form"
                          onSubmit={policySubmit}
                          className="settings-fields settings-subform"
                        >
                          <div className="settings-field-grid">
                            <label>
                              {`${c.shipping} (${currencySign(activeMarket?.currency ?? "TWD")})`}
                              <input
                                type="text"
                                inputMode="decimal"
                                autoComplete="off"
                                required
                                value={draft.shipping}
                                onChange={(event) =>
                                  update("shipping", event.target.value)
                                }
                              />
                            </label>
                            <label>
                              {`${c.freeShipping} (${currencySign(activeMarket?.currency ?? "TWD")})`}
                              <input
                                type="text"
                                inputMode="decimal"
                                autoComplete="off"
                                data-testid="settings-free-shipping"
                                value={draft.freeShipping}
                                onChange={(event) =>
                                  update("freeShipping", event.target.value)
                                }
                              />
                              <small>{c.freeShippingHint}</small>
                            </label>
                            <label>
                              {c.taxMode}
                              <select
                                value={draft.taxMode}
                                onChange={(event) =>
                                  update(
                                    "taxMode",
                                    event.target.value as Draft["taxMode"],
                                  )
                                }
                              >
                                <option value="none">{c.taxNone}</option>
                                <option value="inclusive">
                                  {c.taxInclusive}
                                </option>
                                <option value="exclusive">
                                  {c.taxExclusive}
                                </option>
                              </select>
                            </label>
                            <label>
                              {c.taxBasis}
                              <select
                                value={draft.taxBasis}
                                onChange={(event) =>
                                  update(
                                    "taxBasis",
                                    event.target.value as Draft["taxBasis"],
                                  )
                                }
                              >
                                <option value="goods">{c.goods}</option>
                                <option value="goods_and_shipping">
                                  {c.goodsShipping}
                                </option>
                              </select>
                            </label>
                            <label>
                              {c.taxRate}
                              <input
                                type="number"
                                min="0"
                                max="10000"
                                required
                                value={draft.taxRate}
                                onChange={(event) =>
                                  update("taxRate", event.target.value)
                                }
                              />
                            </label>
                            <label>
                              {c.ttl}
                              <input
                                type="number"
                                min="60"
                                max="1800"
                                required
                                value={draft.ttl}
                                onChange={(event) =>
                                  update("ttl", event.target.value)
                                }
                              />
                            </label>
                          </div>
                          <label className="settings-check">
                            <input
                              type="checkbox"
                              checked={draft.policyEnabled}
                              onChange={(event) =>
                                update("policyEnabled", event.target.checked)
                              }
                            />
                            {c.policyEnabled}
                          </label>
                          <label>
                            {c.ref}
                            <input
                              required={needsPolicySave}
                              maxLength={240}
                              value={draft.reference}
                              onChange={(event) =>
                                update("reference", event.target.value)
                              }
                            />
                            <small>{c.refHint}</small>
                          </label>
                          {needsPolicySave && (
                            <div className="settings-actions">
                              {!hasBaseline(draft.policyObservation, serviceTarget) && (
                                <p role="status">{c.loading}</p>
                              )}
                              <button
                                className="primary"
                                disabled={
                                  !ready ||
                                  !!pending ||
                                  busy ||
                                  !hasBaseline(draft.policyObservation, serviceTarget) ||
                                  !activeMarket.active
                                }
                                type="submit"
                              >
                                {busy ? c.pending : c.savePolicy}
                              </button>
                            </div>
                          )}
                        </form>
                        {!needsPolicySave && (
                          <form
                            data-testid="settings-service-form"
                            onSubmit={serviceSubmit}
                            className="settings-fields settings-subform"
                          >
                            <div className="settings-field-grid">
                              <label>
                                {c.nameHans}
                                <input
                                  required
                                  maxLength={120}
                                  value={draft.nameHans}
                                  onChange={(event) =>
                                    update("nameHans", event.target.value)
                                  }
                                />
                              </label>
                              <label>
                                {c.nameHant}
                                <input
                                  required
                                  maxLength={120}
                                  value={draft.nameHant}
                                  onChange={(event) =>
                                    update("nameHant", event.target.value)
                                  }
                                />
                              </label>
                              <label>
                                {c.nameEN}
                                <input
                                  required
                                  maxLength={120}
                                  value={draft.nameEN}
                                  onChange={(event) =>
                                    update("nameEN", event.target.value)
                                  }
                                />
                              </label>
                              <label>
                                {c.sort}
                                <input
                                  type="number"
                                  min="0"
                                  max="1000"
                                  required
                                  value={draft.sort}
                                  onChange={(event) =>
                                    update("sort", event.target.value)
                                  }
                                />
                              </label>
                            </div>
                            <label className="settings-check">
                              <input
                                type="checkbox"
                                checked={draft.serviceEnabled}
                                onChange={(event) =>
                                  update("serviceEnabled", event.target.checked)
                                }
                              />
                              {c.serviceEnabled}
                            </label>
                            <label className="settings-check">
                              <input
                                type="checkbox"
                                checked={draft.serviceVisible}
                                onChange={(event) =>
                                  update("serviceVisible", event.target.checked)
                                }
                              />
                              {c.visible}
                            </label>
                            <p className="settings-note">{c.prereq}</p>
                            {!hasBaseline(draft.serviceObservation, serviceTarget) && (
                              <p role="status">{c.loading}</p>
                            )}
                            <div className="settings-actions">
                              <button type="button" onClick={() => goStep(2)}>
                                {c.back}
                              </button>
                              <button
                                className="primary"
                                type="submit"
                                disabled={
                                  !ready ||
                                  !!pending ||
                                  busy ||
                                  !hasBaseline(draft.serviceObservation, serviceTarget) ||
                                  !hasBaseline(draft.policyObservation, serviceTarget) ||
                                  !policy?.enabled ||
                                  !activeMarket.active
                                }
                              >
                                {busy ? c.pending : c.saveService}
                              </button>
                            </div>
                          </form>
                        )}
                    </>
                  </>
                )}
              </>
            )}
            {store && step === 4 && (
              <div data-testid="settings-status">
                <h2 className="settings-section-title">{c.statusTitle}</h2>
                <p className="settings-note">{c.manualHint}</p>
                <dl className="settings-status-list">
                  <div>
                    <dt>{c.platform}</dt>
                    <dd>{c.manual}</dd>
                  </div>
                  <div>
                    <dt>{c.market}</dt>
                    <dd>
                      {activeMarket
                        ? `${activeMarket.name} · ${activeMarket.currency}`
                        : c.notConfigured}
                    </dd>
                  </div>
                  <div>
                    <dt>{c.service}</dt>
                    <dd>
                      {activeService
                        ? `${activeService.code} · ${activeService.enabled ? c.enabled : c.notAvailable} · ${activeService.visible ? c.visible : c.notConfigured}`
                        : c.notConfigured}
                    </dd>
                  </div>
                  <div>
                    <dt>{c.policyEnabled}</dt>
                    <dd>
                      {policy
                        ? `v${policy.version} · ${policy.enabled ? c.enabled : c.notAvailable}`
                        : c.notConfigured}
                    </dd>
                  </div>
                </dl>
                <div className="settings-actions">
                  <button type="button" onClick={() => goStep(3)}>
                    {c.back}
                  </button>
                  <button
                    type="button"
                    className="primary"
                    disabled={busy}
                    onClick={() => {
                      void loadBase();
                      if (draft.marketID)
                        void loadMarket(draft.marketID, draft.country);
                    }}
                  >
                    {c.refresh}
                  </button>
                </div>
              </div>
            )}
          </section>
          <aside className="settings-status-column" aria-label={c.statusTitle}>
            <h2>{c.statusTitle}</h2>
            <dl>
              <div>
                <dt>{c.platform}</dt>
                <dd>{c.manual}</dd>
              </div>
              <div>
                <dt>{c.market}</dt>
                <dd>
                  {activeMarket
                    ? `${activeMarket.name} · ${activeMarket.currency}`
                    : c.notConfigured}
                </dd>
              </div>
              <div>
                <dt>{c.service}</dt>
                <dd>
                  {activeService
                    ? `${activeService.code} · ${activeService.enabled ? c.enabled : c.notAvailable} · ${activeService.visible ? c.visible : c.notConfigured}`
                    : c.notConfigured}
                </dd>
              </div>
              <div>
                <dt>{c.policyEnabled}</dt>
                <dd>
                  {policy
                    ? `v${policy.version} · ${policy.enabled ? c.enabled : c.notAvailable}`
                    : c.notConfigured}
                </dd>
              </div>
            </dl>
            <div className="settings-status-next">
              <h3>{c.next}</h3>
              <p>
                {step <= 2 ? c.manualHint : c.prereq}
              </p>
            </div>
          </aside>
        </div>
        {store && <StorefrontSettings store={store.id} locale={locale} />}
        {/* Facebook Page / Instagram connect: BFF /api/meta/*, /api/stores/{store}/meta-connect/* -> Go internal/httpapi/meta_connect.go */}
        {store && <MetaConnect key={store.id} store={store.id} locale={locale} canManage={store.permissions?.includes("integration:manage") === true} />}
      </div>
    </WorkspaceFrame>
  );
}
