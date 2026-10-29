// Purpose: Shared admin empty, error and loading display built from the DS state and action primitives.
// Depends on: React props, @live-commerce/i18n locale type and @live-commerce/ui/ds; no transport, ACL or routing authority.
// Used by: localized admin route boundaries, Customers and MerchantOrders; global-error uses it outside the locale layout.
// Invariants: errors never render upstream internals; loading is announced once; empty states do not invent write actions.
"use client";

import type { Locale } from "@live-commerce/i18n";
import { ButtonLink, EmptyState, ErrorState } from "@live-commerce/ui/ds";
import "./admin-state-block.css";

export const adminStateCopy: Record<Locale, {
  errorTitle: string;
  errorDescription: string;
  notFoundTitle: string;
  notFoundDescription: string;
  loading: string;
  retry: string;
  home: string;
}> = {
  en: {
    errorTitle: "We couldn’t load this page",
    errorDescription: "The page could not be loaded. Try again, or return to the dashboard.",
    notFoundTitle: "We couldn’t find this page",
    notFoundDescription: "The page may have moved or no longer exists. Return to the dashboard to continue.",
    loading: "Loading…",
    retry: "Try again",
    home: "Back to dashboard",
  },
  "zh-CN": {
    errorTitle: "无法加载这个页面",
    errorDescription: "页面暂时无法加载，请重试或返回仪表盘。",
    notFoundTitle: "找不到这个页面",
    notFoundDescription: "页面可能已移动或不存在，请返回仪表盘继续操作。",
    loading: "正在加载…",
    retry: "重试",
    home: "返回仪表盘",
  },
  "zh-TW": {
    errorTitle: "無法載入這個頁面",
    errorDescription: "頁面暫時無法載入，請重試或返回儀表板。",
    notFoundTitle: "找不到這個頁面",
    notFoundDescription: "頁面可能已移動或不存在，請返回儀表板繼續操作。",
    loading: "正在載入…",
    retry: "重試",
    home: "返回儀表板",
  },
};

type StateProps = {
  title: string;
  description?: string;
  actionHref?: string;
  actionLabel?: string;
};

export type AdminStateBlockProps = StateProps & {
  variant: "empty" | "loading" | "error";
  onRetry?: () => void;
  retryLabel?: string;
};

function StateAction({ actionHref, actionLabel }: Pick<StateProps, "actionHref" | "actionLabel">) {
  if (!actionHref || !actionLabel) return null;
  return <ButtonLink href={actionHref} variant="secondary">{actionLabel}</ButtonLink>;
}

/** Display only: callers retain their existing read, retry and navigation behavior. */
export function AdminStateBlock(props: AdminStateBlockProps) {
  const { title, description, actionHref, actionLabel } = props;
  if (props.variant === "loading") {
    return (
      <div className="admin-state-loading" role="status" aria-live="polite">
        <div className="admin-state-skeleton" aria-hidden="true">
          <span className="admin-state-skeleton-mark" />
          <span className="admin-state-skeleton-line admin-state-skeleton-line-title" />
          <span className="admin-state-skeleton-line" />
        </div>
        <h3 className="admin-state-loading-title">{title}</h3>
        {description && <p className="admin-state-loading-description">{description}</p>}
      </div>
    );
  }
  return (
    <div className="admin-state-block">
      {props.variant === "empty" ? (
        <EmptyState
          mode="full"
          title={title}
          description={description}
          action={<StateAction actionHref={actionHref} actionLabel={actionLabel} />}
        />
      ) : (
        <ErrorState
          mode="full"
          title={title}
          detail={description}
          onRetry={props.onRetry}
          retryLabel={props.retryLabel}
          actions={<StateAction actionHref={actionHref} actionLabel={actionLabel} />}
        />
      )}
    </div>
  );
}
