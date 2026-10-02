// GET /[locale]/invite/[token] (server). The page behind the link in the staff invitation email. It only learns whether this
// browser holds a valid session (the store list call, like /signup) and hands the token to TeamInvite.tsx, whose Accept button
// calls POST /api/team/accept -> Go /v1/identity/staff/accept (migration 0089: single use, 72 h, email must equal the account's).
// Nothing about the invitation (store, role, inviter) is rendered before accepting, so a leaked link tells a stranger nothing.
// Referrer-Policy no-referrer + noindex keep the token out of outbound requests and search results. 404 unless auth is configured.
import type { Metadata } from "next";
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { routeMetadata } from "@/src/route-metadata";
export const generateMetadata = ({ params }: { params: Promise<{ locale: string }> }) => routeMetadata("/invite/[token]", params);
import { SESSION_COOKIE, authConfig, authenticatedStores, exactCookieHeader, isBase64URL32 } from "@/lib/auth";
import { TeamInvite } from "@/components/TeamInvite";

export const metadata: Metadata = { referrer: "no-referrer", robots: { index: false, follow: false } };

export default async function InvitePage({ params }: { params: Promise<{ locale: string; token: string }> }) {
  const { locale, token } = await params;
  if (!isLocale(locale) || !authConfig || !/^[A-Za-z0-9_-]{43}$/.test(token)) notFound();
  const session = exactCookieHeader((await headers()).get("cookie"), SESSION_COOKIE) ?? "";
  // Any authenticated store list (even empty) proves the session; the Go definer re-verifies it on accept.
  const signedIn = isBase64URL32(session) && !!(await authenticatedStores(session)).stores;
  return <TeamInvite locale={locale} token={token} signedIn={signedIn} passwordLogin={!!authConfig.passwordLogin} />;
}
