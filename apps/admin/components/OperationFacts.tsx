// Purpose: read-only operation metadata and recent event rendering, separated from command/session state.
// Depends on: shared Taipei format; operations-model and localized copy; React node type.
// Used by: OperationsLedger's accessible detail drawer; no network or writes.
import type { ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import type { OperationDetail } from "@/lib/operations-model";
import { operationReasonText, operationsCopy } from "@/lib/operations-copy";
/** Render only the server's closed public metadata and an already-validated object link. */
export function OperationFacts({
  detail,
  locale,
  object,
}: {
  detail: OperationDetail;
  locale: Locale;
  object: ReactNode;
}) {
  const c = operationsCopy[locale];
  const stateText = (v: string) => c.states[v as keyof typeof c.states] ?? v;
  return (
    <>
      {" "}
      <p className="operations-id">{detail.operation_id}</p>
      <dl>
        <dt>{c.state}</dt>
        <dd>{stateText(detail.state)}</dd>
        <dt>{c.action}</dt>
        <dd>{detail.action}</dd>
        <dt>{c.provider}</dt>
        <dd>{detail.provider}</dd>
        <dt>{c.attempts}</dt>
        <dd>{detail.attempts}</dd>
        <dt>{c.created}</dt>
        <dd>{displayTime(locale, detail.created_at)}</dd>
        <dt>{c.updated}</dt>
        <dd>{displayTime(locale, detail.updated_at)}</dd>
        <dt>{c.reason}</dt>
        <dd>{operationReasonText(locale, detail.reason_code)}</dd>
        <dt>{c.object}</dt>
        <dd>{object}</dd>
      </dl>
    </>
  );
}
/** Render the newest audit events with an explicit Asia/Taipei timestamp. */
export function OperationEvents({
  detail,
  locale,
}: {
  detail: OperationDetail;
  locale: Locale;
}) {
  const c = operationsCopy[locale];
  const stateText = (v: string) => c.states[v as keyof typeof c.states] ?? v;
  return (
    <>
      {" "}
      <h3>{c.events}</h3>
      <ol>
        {detail.events.map((e, i) => (
          <li key={`${e.created_at}-${i}`}>
            <span>
              {stateText(e.state)} · {c.attempts}: {e.generation}
            </span>
            <p>{operationReasonText(locale, e.reason_code)}</p>
            <time dateTime={e.created_at}>
              {displayTime(locale, e.created_at)}
            </time>
          </li>
        ))}
      </ol>
    </>
  );
}
