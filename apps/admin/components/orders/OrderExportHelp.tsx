// Figma local links: https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=808-5618 and https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=808-5619
// Purpose: signed help-only frame 808:5618/5619; depends on authenticated page context and shared states.
// Used by: orders/export/page. Invariants: orders:read; no export/CSV/write transport; return retains authenticated store.
"use client";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import type { ReadCode } from "@/lib/customers-client";
import { permitted } from "@/src/features/messages/privacy";
import { AdminForbidden } from "../AdminForbidden";
import { AdminStateBlock, adminStateCopy } from "../AdminStateBlock";
import { orderExportHelpCopy } from "./OrderExportHelp.copy";
import s from "./OrderExportHelp.module.css";
/** Information only: help access never grants permission to export recipients. */
export function OrderExportHelp({
  locale,
  store,
  initialError,
}: {
  locale: Locale;
  store: Store | null;
  initialError: ReadCode | null;
}) {
  const router = useRouter(),
    c = orderExportHelpCopy[locale];
  if (initialError)
    return initialError === "forbidden" ? (
      <AdminForbidden locale={locale} />
    ) : (
      <AdminStateBlock
        variant="error"
        title={adminStateCopy[locale].errorTitle}
      />
    );
  if (!permitted(store, "orders:read"))
    return <AdminForbidden locale={locale} />;
  return (
    <section className={s.page} data-figma-node="808:5618 808:5619">
      <h1>{c.title}</h1>
      <h2>{c.warning}</h2>
      <p>{c.body}</p>
      <button
        type="button"
        onClick={() =>
          router.push(
            `/${locale}/orders?${new URLSearchParams({ store: store!.id })}`,
          )
        }
      >
        {c.back}
      </button>
    </section>
  );
}
