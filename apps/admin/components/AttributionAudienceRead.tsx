"use client";
// POST /api/stores/{store}/ads/sessions/{session}/audience-read -> Go same resource under /v1/admin/stores.
// A read receipt may replay an existing operation; only SUCCEEDED proves completion. Uncertainty never creates a fresh-key retry.
import { useEffect, useRef, useState } from "react";
import { requestAudienceRead } from "@/lib/attribution-client";
import {
  audienceStorageKey,
  audienceAckPhase,
  parseAudienceJournal,
  type AudienceJournal,
} from "@/lib/attribution-audience";
import { sessionBoundary } from "@/lib/settings-client";
import type { AttributionCopy } from "@/lib/attribution-copy";

export function AttributionAudienceRead({
  store,
  session,
  c,
}: {
  store: string;
  session: string;
  c: AttributionCopy;
}) {
  const storageKey = audienceStorageKey(store, session),
    mounted = useRef(false),
    busy = useRef(false);
  const [journal, setJournal] = useState<AudienceJournal | null>(null),
    [verified, setVerified] = useState(false),
    [sending, setSending] = useState(false);
  const [problem, setProblem] = useState<
    "forbidden" | "signed-out" | "failed" | "storage" | null
  >(null);
  useEffect(() => {
    mounted.current = true;
    let current = true;
    void sessionBoundary()
      .then((boundary) => {
        if (!current) return;
        try {
          const raw = sessionStorage.getItem(storageKey);
          if (raw) {
            const stored = parseAudienceJournal(raw);
            if (stored.boundary === boundary) setJournal(stored);
            else sessionStorage.removeItem(storageKey); // A new session cannot replay a prior session's intention.
          }
          setVerified(true);
        } catch {
          setProblem("storage");
        }
      })
      .catch(() => {
        if (current) setProblem("signed-out");
      });
    return () => {
      mounted.current = false;
      current = false;
    };
  }, [storageKey]);

  async function queue(retry: boolean) {
    if (
      !verified ||
      busy.current ||
      (retry &&
        journal?.phase !== "unknown" &&
        journal?.phase !== "unconfirmed")
    )
      return;
    busy.current = true;
    setSending(true);
    setProblem(null);
    let pending: AudienceJournal;
    try {
      const boundary = await sessionBoundary();
      if (retry && journal && journal.boundary !== boundary)
        throw new Error("session_changed");
      pending =
        retry && journal
          ? journal
          : {
              key: `audience-read-${crypto.randomUUID()}`,
              boundary,
              phase: "unknown",
              operation_id: null,
            };
    } catch {
      setProblem("signed-out");
      setSending(false);
      busy.current = false;
      return;
    }
    try {
      // Persist before sending: reload, selection changes or dropped acknowledgements must not create another key.
      sessionStorage.setItem(storageKey, JSON.stringify(pending));
      if (mounted.current) setJournal(pending);
    } catch {
      setProblem("storage");
      setSending(false);
      busy.current = false;
      return;
    }
    const result = await requestAudienceRead(
      store,
      session,
      pending.key,
      pending.boundary,
    );
    try {
      if (result.kind === "acknowledged") {
        const acknowledged: AudienceJournal = {
          ...pending,
          phase: audienceAckPhase(result.ack),
          operation_id: result.ack.operation_id,
          state: result.ack.state,
        };
        sessionStorage.setItem(storageKey, JSON.stringify(acknowledged));
        if (mounted.current) setJournal(acknowledged);
      } else if (result.kind !== "unknown") {
        sessionStorage.removeItem(storageKey);
        if (mounted.current) {
          setJournal(null);
          setProblem(result.kind);
        }
      }
    } catch {
      if (mounted.current) setProblem("storage");
    }
    if (mounted.current) setSending(false);
    busy.current = false;
  }
  const locked =
    sending || !verified || problem === "storage" || problem === "signed-out";
  return (
    <div
      className="attribution-audience-read"
      data-testid="attribution-audience-read"
    >
      <button
        type="button"
        data-testid="attribution-audience-refresh"
        disabled={
          locked ||
          journal?.phase === "unknown" ||
          journal?.phase === "unconfirmed"
        }
        onClick={() => void queue(false)}
      >
        {sending ? c.audienceReading : c.audienceRefresh}
      </button>
      {!sending && journal?.phase === "unknown" && (
        <>
          <p role="status" data-testid="attribution-audience-unknown">
            {c.audienceUnknown}
          </p>
          <button
            type="button"
            data-testid="attribution-audience-retry"
            disabled={locked}
            onClick={() => void queue(true)}
          >
            {c.audienceRetry}
          </button>
        </>
      )}
      {!sending && journal?.phase === "queued" && (
        <p role="status" data-testid="attribution-audience-queued">
          {c.audienceQueued}
        </p>
      )}
      {!sending && journal?.phase === "inflight" && (
        <p
          role="status"
          data-testid="attribution-audience-inflight"
          data-state={journal.state}
        >
          {c.audienceInflight}
        </p>
      )}
      {!sending && journal?.phase === "completed" && (
        <p role="status" data-testid="attribution-audience-completed">
          {c.audienceCompleted}
        </p>
      )}
      {!sending && journal?.phase === "unconfirmed" && (
        <>
          <p role="status" data-testid="attribution-audience-unconfirmed">
            {c.audienceUnconfirmed}
          </p>
          <button
            type="button"
            data-testid="attribution-audience-retry"
            disabled={locked}
            onClick={() => void queue(true)}
          >
            {c.audienceCheck}
          </button>
        </>
      )}
      {!sending && journal?.phase === "failed" && (
        <p
          role="alert"
          data-testid="attribution-audience-failed"
          data-state={journal.state}
        >
          {journal.state === "BLOCKED_POLICY"
            ? c.audienceBlocked
            : journal.state === "STALE_BINDING"
              ? c.audienceStale
              : journal.state === "CANCELLED"
                ? c.audienceCancelled
                : c.audienceReadFailed}
        </p>
      )}
      {problem && (
        <p role="alert" data-testid="attribution-audience-problem">
          {problem === "forbidden"
            ? c.audienceForbidden
            : problem === "signed-out"
              ? c.signedOut
              : problem === "storage"
                ? c.audienceStorage
                : c.audienceFailed}
        </p>
      )}
    </div>
  );
}
