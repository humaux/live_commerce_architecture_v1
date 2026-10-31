// Purpose: Render fourteen signed domain settings states, DNS instructions and lifecycle actions on the design route.
// Depends on: react, @live-commerce/i18n, storefront-client, settings-client session fence.
// Used by: Design task hub (Design.tsx).
// Invariants: never expose raw verification tokens; only ACTIVE and serving is ready; reads remain store/session scoped; replay preserves exact idempotency key and payload.
import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { domainsParityCopy, type Locale } from '@live-commerce/i18n';
import type { DesignCopy } from '@/lib/design-copy';
import {
  readDomains,
  requestDomain,
  moveDomain,
  type StorefrontDomains,
  type StorefrontDomainRow,
  type DomainRequestResult,
} from '@/lib/storefront-client';
import { sessionBoundary } from '@/lib/settings-client';

type ViewMode =
  | 'main'
  | 'fullPlatform'
  | 'add'
  | 'receipt'
  | 'records'
  | 'fullValue'
  | 'suspend'
  | 'detach'
  | 'pending';

type JournalCommand =
  | {
      action: 'request';
      key: string;
      hostname: string;
      targetOrigin?: string;
    }
  | {
      action: 'suspend' | 'detach';
      key: string;
      origin: string;
    };

export function DesignDomains({
  store,
  boundary,
  locale,
  c,
}: {
  store: string;
  boundary: string;
  locale: Locale;
  c: DesignCopy;
}) {
  const cCopy = domainsParityCopy[locale];
  const journalKey = `storefront-domains-cmd:${store}`;

  const [domains, setDomains] = useState<StorefrontDomains | null>(null);
  const [readError, setReadError] = useState(false);
  const [viewMode, setViewMode] = useState<ViewMode>('main');
  const [selectedDomain, setSelectedDomain] = useState<StorefrontDomainRow | null>(null);
  const [receiptDomain, setReceiptDomain] = useState<string>('');
  const [receiptDns, setReceiptDns] = useState<DomainRequestResult['dns'] | null>(null);
  const [hostnameInput, setHostnameInput] = useState('');
  const [addError, setAddError] = useState<string | null>(null);
  const [fullValueText, setFullValueText] = useState('');
  const [pendingCommand, setPendingCommand] = useState<JournalCommand | null>(null);
  const [pendingOrigin, setPendingOrigin] = useState<string>('');

  const loadDomains = useCallback(async () => {
    const controller = new AbortController();
    try {
      const result = await readDomains(store, controller.signal);
      if ((await sessionBoundary()) === boundary) {
        setDomains(result);
        setReadError(false);
      }
    } catch {
      setReadError(true);
    }
  }, [store, boundary]);

  // Initial load and restore pending journal command if present
  useEffect(() => {
    let current = true;
    try {
      const item = sessionStorage.getItem(journalKey);
      if (item) {
        const parsed = JSON.parse(item) as JournalCommand;
        if (
          parsed &&
          (parsed.action === 'request' || parsed.action === 'suspend' || parsed.action === 'detach') &&
          parsed.key
        ) {
          if (current) {
            setPendingCommand(parsed);
            const orig =
              parsed.action === 'request'
                ? parsed.targetOrigin || `https://${parsed.hostname}`
                : parsed.origin;
            setPendingOrigin(orig);
            setViewMode('pending');
          }
        }
      }
    } catch {
      // ignore
    }
    void loadDomains();
    return () => {
      current = false;
    };
  }, [loadDomains, journalKey]);

  const platformDomain = domains?.domains.find((d) => d.kind === 'platform') ?? null;
  const customDomains = domains?.domains.filter((d) => d.kind === 'custom') ?? [];

  // Copy full value action handler
  const handleCopyValue = useCallback(async (value: string) => {
    setFullValueText(value);
    setViewMode('fullValue');
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
      }
    } catch {
      // ignore clipboard error in headless test environment if permission denied
    }
  }, []);

  // Add domain apply
  const handleApplyAdd = async (e: FormEvent) => {
    e.preventDefault();
    const raw = hostnameInput.trim().toLowerCase();
    // Validate domain format
    if (
      !raw ||
      raw.includes('https://') ||
      raw.includes('http://') ||
      raw.includes('/') ||
      raw.includes(' ') ||
      !raw.includes('.')
    ) {
      setAddError(cCopy.invalid);
      return;
    }
    setAddError(null);
    const key = `storefront-domain-${crypto.randomUUID()}`;
    const targetOrigin = `https://${raw}`;
    const cmd: JournalCommand = {
      action: 'request',
      key,
      hostname: raw,
      targetOrigin,
    };
    try {
      sessionStorage.setItem(journalKey, JSON.stringify(cmd));
    } catch {
      // ignore
    }
    setPendingCommand(cmd);
    setPendingOrigin(targetOrigin);

    const outcome = await requestDomain(store, key, raw, boundary);
    if (!outcome.ok && outcome.uncertain) {
      setViewMode('pending');
      return;
    }
    if (outcome.ok) {
      try {
        sessionStorage.removeItem(journalKey);
      } catch {
        // ignore
      }
      setPendingCommand(null);
      setReceiptDomain(targetOrigin);
      setReceiptDns(outcome.result.dns);
      setViewMode('receipt');
      void loadDomains();
    } else {
      setAddError(cCopy.invalid);
    }
  };

  // Reapply action on existing custom domain
  const handleReapply = async () => {
    const origin = receiptDomain || selectedDomain?.origin;
    if (!origin) return;
    const hostname = origin.replace(/^https?:\/\//, '');
    const key = `storefront-domain-${crypto.randomUUID()}`;
    const cmd: JournalCommand = {
      action: 'request',
      key,
      hostname,
      targetOrigin: origin,
    };
    try {
      sessionStorage.setItem(journalKey, JSON.stringify(cmd));
    } catch {
      // ignore
    }
    setPendingCommand(cmd);
    setPendingOrigin(origin);

    const outcome = await requestDomain(store, key, hostname, boundary);
    if (!outcome.ok && outcome.uncertain) {
      setViewMode('pending');
      return;
    }
    if (outcome.ok) {
      try {
        sessionStorage.removeItem(journalKey);
      } catch {
        // ignore
      }
      setPendingCommand(null);
      setReceiptDomain(origin);
      setReceiptDns(outcome.result.dns);
      setViewMode('receipt');
      void loadDomains();
    }
  };

  // Suspend confirm
  const handleConfirmSuspend = async () => {
    const origin = receiptDomain || selectedDomain?.origin;
    if (!origin) return;
    const key = `storefront-domain-${crypto.randomUUID()}`;
    const cmd: JournalCommand = {
      action: 'suspend',
      key,
      origin,
    };
    try {
      sessionStorage.setItem(journalKey, JSON.stringify(cmd));
    } catch {
      // ignore
    }
    setPendingCommand(cmd);
    setPendingOrigin(origin);

    const outcome = await moveDomain(store, key, origin, 'suspend', boundary);
    if (!outcome.ok && outcome.uncertain) {
      setViewMode('pending');
      return;
    }
    if (outcome.ok) {
      try {
        sessionStorage.removeItem(journalKey);
      } catch {
        // ignore
      }
      setPendingCommand(null);
      setReceiptDomain(origin);
      setReceiptDns(null);
      setViewMode('receipt');
      void loadDomains();
    }
  };

  // Detach confirm
  const handleConfirmDetach = async () => {
    const origin = receiptDomain || selectedDomain?.origin;
    if (!origin) return;
    const key = `storefront-domain-${crypto.randomUUID()}`;
    const cmd: JournalCommand = {
      action: 'detach',
      key,
      origin,
    };
    try {
      sessionStorage.setItem(journalKey, JSON.stringify(cmd));
    } catch {
      // ignore
    }
    setPendingCommand(cmd);
    setPendingOrigin(origin);

    const outcome = await moveDomain(store, key, origin, 'detach', boundary);
    if (!outcome.ok && outcome.uncertain) {
      setViewMode('pending');
      return;
    }
    if (outcome.ok) {
      try {
        sessionStorage.removeItem(journalKey);
      } catch {
        // ignore
      }
      setPendingCommand(null);
      setReceiptDomain(origin);
      setReceiptDns(null);
      setViewMode('receipt');
      void loadDomains();
    }
  };

  // Retry pending journal command
  const handleRetry = async () => {
    if (!pendingCommand) return;
    if (pendingCommand.action === 'request') {
      const outcome = await requestDomain(
        store,
        pendingCommand.key,
        pendingCommand.hostname,
        boundary,
      );
      if (!outcome.ok && outcome.uncertain) {
        setViewMode('pending');
        return;
      }
      if (outcome.ok) {
        try {
          sessionStorage.removeItem(journalKey);
        } catch {
          // ignore
        }
        const orig = pendingCommand.targetOrigin || `https://${pendingCommand.hostname}`;
        setPendingCommand(null);
        setReceiptDomain(orig);
        setReceiptDns(outcome.result.dns);
        setViewMode('receipt');
        void loadDomains();
      }
    } else {
      const outcome = await moveDomain(
        store,
        pendingCommand.key,
        pendingCommand.origin,
        pendingCommand.action,
        boundary,
      );
      if (!outcome.ok && outcome.uncertain) {
        setViewMode('pending');
        return;
      }
      if (outcome.ok) {
        try {
          sessionStorage.removeItem(journalKey);
        } catch {
          // ignore
        }
        const orig = pendingCommand.origin;
        setPendingCommand(null);
        setReceiptDomain(orig);
        setReceiptDns(null);
        setViewMode('receipt');
        void loadDomains();
      }
    }
  };

  return (
    <section className="design-panel" data-testid="design-domains">
      {/* 1. Pending view (uncertain write recovery) */}
      {viewMode === 'pending' && (
        <div className="design-block">
          <h2>{cCopy.pending}</h2>
          <p>{pendingOrigin}</p>
          <div className="design-actions">
            <button type="button" className="design-primary" onClick={() => void handleRetry()}>
              {cCopy.retry}
            </button>
          </div>
        </div>
      )}

      {/* 2. Full platform origin view */}
      {viewMode === 'fullPlatform' && (
        <div className="design-block">
          <h2>{cCopy.full}</h2>
          <p>{platformDomain?.origin}</p>
          <div className="design-actions">
            <button type="button" onClick={() => setViewMode('main')}>
              {cCopy.return}
            </button>
          </div>
        </div>
      )}

      {/* 3. Add domain view */}
      {viewMode === 'add' && (
        <form className="design-block" onSubmit={(e) => void handleApplyAdd(e)}>
          <h2>{cCopy.addTitle}</h2>
          <div className="design-field">
            <label htmlFor="domain-hostname-input">{cCopy.hostname}</label>
            <input
              id="domain-hostname-input"
              aria-label={cCopy.hostname}
              type="text"
              value={hostnameInput}
              onChange={(e) => {
                setHostnameInput(e.target.value);
                setAddError(null);
              }}
            />
          </div>
          {addError && (
            <p className="design-error" role="alert">
              {addError}
            </p>
          )}
          <div className="design-actions">
            <button
              type="button"
              onClick={() => {
                setAddError(null);
                setViewMode('main');
              }}
            >
              {cCopy.cancel}
            </button>
            <button type="submit" className="design-primary">
              {cCopy.apply}
            </button>
          </div>
        </form>
      )}

      {/* 4. Operation receipt view */}
      {viewMode === 'receipt' && (
        <div className="design-block">
          <h2>{cCopy.receipt}</h2>
          <p>{receiptDomain}</p>
          <div className="design-actions">
            {receiptDns && (
              <button
                type="button"
                className="design-primary"
                onClick={() => setViewMode('records')}
              >
                {cCopy.records}
              </button>
            )}
            <button type="button" onClick={() => setViewMode('main')}>
              {cCopy.home}
            </button>
            <button type="button" onClick={() => setViewMode('main')}>
              {cCopy.back}
            </button>
          </div>
        </div>
      )}

      {/* 5. DNS records view */}
      {viewMode === 'records' && (
        <div className="design-block">
          <h2>{cCopy.dns}</h2>
          <p>{receiptDomain || selectedDomain?.origin}</p>
          {receiptDns ? (
            <div className="design-records-table">
              <p>{receiptDns.txt_value}</p>
              <p>{receiptDns.txt_name}</p>
              {receiptDns.cname_target && <p>{receiptDns.cname_target}</p>}

              {/* TXT Record */}
              <div data-testid="domain-dns-TXT" className="design-row">
                <span>TXT</span>
                <span>{receiptDns.txt_name}</span>
                <span>{receiptDns.txt_value}</span>
                <div className="design-row-actions">
                  <button
                    type="button"
                    onClick={() => void handleCopyValue(receiptDns.txt_name)}
                  >
                    {cCopy.copyHost}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleCopyValue(receiptDns.txt_value)}
                  >
                    {cCopy.copyValue}
                  </button>
                </div>
              </div>

              {/* CNAME or Apex (A/AAAA) */}
              {receiptDns.apex ? (
                receiptDns.edge_addresses && receiptDns.edge_addresses.length >= 2 ? (
                  <>
                    <div data-testid="domain-dns-A" className="design-row">
                      <span>A</span>
                      <span>{receiptDns.edge_addresses[0]}</span>
                      <div className="design-row-actions">
                        <button
                          type="button"
                          onClick={() =>
                            void handleCopyValue(
                              (receiptDomain || selectedDomain?.origin || '').replace(
                                /^https?:\/\//,
                                '',
                              ),
                            )
                          }
                        >
                          {cCopy.copyHost}
                        </button>
                        <button
                          type="button"
                          onClick={() => void handleCopyValue(receiptDns.edge_addresses![0])}
                        >
                          {cCopy.copyValue}
                        </button>
                      </div>
                    </div>
                    <div data-testid="domain-dns-AAAA" className="design-row">
                      <span>AAAA</span>
                      <span>{receiptDns.edge_addresses[1]}</span>
                      <div className="design-row-actions">
                        <button
                          type="button"
                          onClick={() =>
                            void handleCopyValue(
                              (receiptDomain || selectedDomain?.origin || '').replace(
                                /^https?:\/\//,
                                '',
                              ),
                            )
                          }
                        >
                          {cCopy.copyHost}
                        </button>
                        <button
                          type="button"
                          onClick={() => void handleCopyValue(receiptDns.edge_addresses![1])}
                        >
                          {cCopy.copyValue}
                        </button>
                      </div>
                    </div>
                  </>
                ) : (
                  <p>{cCopy.missingIP}</p>
                )
              ) : (
                <div data-testid="domain-dns-CNAME" className="design-row">
                  <span>CNAME</span>
                  <span>{receiptDns.cname_target}</span>
                  <div className="design-row-actions">
                    <button
                      type="button"
                      onClick={() =>
                        void handleCopyValue(
                          (receiptDomain || selectedDomain?.origin || '').replace(
                            /^https?:\/\//,
                            '',
                          ),
                        )
                      }
                    >
                      {cCopy.copyHost}
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleCopyValue(receiptDns.cname_target)}
                    >
                      {cCopy.copyValue}
                    </button>
                  </div>
                </div>
              )}
            </div>
          ) : (
            <p>{cCopy.missing}</p>
          )}

          <div className="design-actions">
            <button type="button" onClick={() => setViewMode('suspend')}>
              {cCopy.suspend}
            </button>
            <button type="button" onClick={() => setViewMode('detach')}>
              {cCopy.detach}
            </button>
            <button type="button" onClick={() => void handleReapply()}>
              {cCopy.reapply}
            </button>
            <button type="button" onClick={() => setViewMode('main')}>
              {cCopy.back}
            </button>
          </div>
        </div>
      )}

      {/* 6. Full value view (from copying DNS host/value) */}
      {viewMode === 'fullValue' && (
        <div className="design-block">
          <h2>{cCopy.full}</h2>
          <p data-testid="domain-full-value">{fullValueText}</p>
          <p role="status">{cCopy.copied}</p>
          <div className="design-actions">
            <button type="button" onClick={() => setViewMode('records')}>
              {cCopy.return}
            </button>
          </div>
        </div>
      )}

      {/* 7. Suspend confirmation view */}
      {viewMode === 'suspend' && (
        <div className="design-block">
          <h2>{cCopy.suspendTitle}</h2>
          <p>{receiptDomain || selectedDomain?.origin}</p>
          <div className="design-actions">
            <button type="button" onClick={() => setViewMode('records')}>
              {cCopy.cancel}
            </button>
            <button
              type="button"
              className="design-primary"
              onClick={() => void handleConfirmSuspend()}
            >
              {cCopy.confirmSuspend}
            </button>
          </div>
        </div>
      )}

      {/* 8. Detach confirmation view */}
      {viewMode === 'detach' && (
        <div className="design-block">
          <h2>{cCopy.detachTitle}</h2>
          <p>{receiptDomain || selectedDomain?.origin}</p>
          <div className="design-actions">
            <button type="button" onClick={() => setViewMode('records')}>
              {cCopy.cancel}
            </button>
            <button
              type="button"
              className="design-primary"
              onClick={() => void handleConfirmDetach()}
            >
              {cCopy.confirmDetach}
            </button>
          </div>
        </div>
      )}

      {/* 9. Main view */}
      {viewMode === 'main' && (
        <>
          <h2>{cCopy.title}</h2>
          {readError && (
            <p className="design-error" role="status">
              {cCopy.readFailed}
            </p>
          )}

          {/* Platform domain card */}
          {platformDomain && (
            <article
              data-testid="design-domain-platform"
              data-state={platformDomain.state}
              className="design-block"
            >
              <h3>{cCopy.platform}</h3>
              <p>{platformDomain.origin}</p>
              <button type="button" onClick={() => setViewMode('fullPlatform')}>
                {cCopy.fullPlatform}
              </button>
            </article>
          )}

          {/* Toolbar with refresh and add domain */}
          <div className="design-actions">
            <button type="button" onClick={() => void loadDomains()}>
              {cCopy.refresh}
            </button>
            <button
              type="button"
              className="design-primary"
              onClick={() => {
                setHostnameInput('');
                setAddError(null);
                setViewMode('add');
              }}
            >
              {cCopy.add}
            </button>
          </div>

          {/* Custom domain rows */}
          {customDomains.map((d) => {
            const stateIndex =
              {
                REQUESTED: 0,
                OWNERSHIP_PENDING: 1,
                TLS_PENDING: 2,
                ACTIVE: 3,
                SUSPENDED: 4,
                DETACHED: 5,
              }[d.state] ?? 0;
            return (
              <div key={d.origin} data-testid="domain-custom-row" className="design-row">
                <article
                  data-testid="design-domain-custom"
                  data-state={d.state}
                  className="design-block"
                >
                  <p>{d.origin}</p>
                  <p>{cCopy.states[stateIndex]}</p>
                  {d.state === 'ACTIVE' && d.serving ? (
                    <p>{cCopy.serving}</p>
                  ) : (
                    <p>{cCopy.notServing}</p>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedDomain(d);
                      setReceiptDomain(d.origin);
                      setReceiptDns(null);
                      setViewMode('records');
                    }}
                  >
                    {cCopy.settings}
                  </button>
                </article>
              </div>
            );
          })}

          {domains?.domains.length === 0 && <p>{c.editor.noAddress}</p>}
        </>
      )}
    </section>
  );
}
