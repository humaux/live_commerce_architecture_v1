// GET /[locale]/signup (server). Renders PasswordAuth mode="signup", which calls POST /api/auth/password/signup and
// /api/auth/password/verify → Go /v1/identity/password/signup and /complete (internal/identityhttp/password.go).
// 404 unless COMMERCE_PASSWORD_LOGIN_ENABLED=1; a valid session redirects to /<locale>/ (U6).
import { notFound, redirect } from "next/navigation";
import { headers } from "next/headers";
import { isLocale } from "@live-commerce/i18n";
import {
  SESSION_COOKIE,
  authConfig,
  authenticatedStores,
  exactCookieHeader,
  isBase64URL32,
} from "@/lib/auth";
import { onboardingPolicy } from "@/lib/backend";
import { inviteNextPath } from "@/lib/invite-next";
import { Entry } from "@/components/Entry";

export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  if (!authConfig?.passwordLogin) notFound();
  const query = await searchParams;
  const next = inviteNextPath(typeof query.next === "string" ? query.next : null);
  const token =
    exactCookieHeader((await headers()).get("cookie"), SESSION_COOKIE) ?? "";
  // Any authenticated store list (even empty) proves the session; workspace pages own the rest.
  // invite-next: an already-signed-in invitee goes straight back to the invite page, not the dashboard.
  if (isBase64URL32(token) && (await authenticatedStores(token)).stores)
    redirect(next ?? `/${locale}/`);
  const policy = onboardingPolicy();
  return (
    <Entry
      locale={locale}
      status="signed-out"
      authResult=""
      onboardingEnabled={policy.enabled}
      currencies={policy.currencies}
      passwordMode="signup"
      oidc={!!authConfig.issuer}
      path="signup"
      next={next}
    />
  );
}
