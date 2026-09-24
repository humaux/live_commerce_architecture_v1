import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import {
  authConfig,
  authenticatedStores,
  exactCookieHeader,
  isBase64URL32,
  safeError,
  SESSION_COOKIE,
} from "@/lib/auth";
import type { APIError } from "@/lib/model";
import { SettingsWizard } from "@/components/SettingsWizard";

export default async function SettingsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  const requested = query.store;
  if (
    requested !== undefined &&
    (typeof requested !== "string" || !/^[0-9a-f-]{36}$/.test(requested))
  )
    notFound();
  let error: APIError | null = null;
  let store = null;
  if (!authConfig) {
    error = {
      code: "unauthorized",
      message: "",
      request_id: "",
      retryable: false,
      details: {},
    };
  } else {
    const token = exactCookieHeader(
      (await headers()).get("cookie"),
      SESSION_COOKIE,
    );
    if (!token || !isBase64URL32(token)) {
      error = {
        code: "unauthorized",
        message: "",
        request_id: "",
        retryable: false,
        details: {},
      };
    } else {
      const listed = await authenticatedStores(token);
      if (!listed.stores)
        error = (await (await safeError(listed.response)).json()) as APIError;
      else {
        store = requested
          ? (listed.stores.find((item) => item.id === requested) ?? null)
          : ([...listed.stores].sort((a, b) => a.id.localeCompare(b.id))[0] ??
            null);
        if (requested && !store) notFound();
      }
    }
  }
  return (
    <SettingsWizard
      key={`${locale}:${store?.id ?? "no-store"}`}
      locale={locale}
      initial={{ store, error }}
    />
  );
}
