// Purpose: configure the admin build and response security headers.
// Depends on: Next configuration phases and deliberate operator company configuration.
// Used by: Next build/start; sensitive route overrides must follow the global header defaults.
import type { NextConfig } from "next";
import { PHASE_PRODUCTION_BUILD } from "next/constants";
import { companyConfig } from "./lib/company";

// React's development build rebuilds call stacks with eval() (Next documents 'unsafe-eval' for `next dev`): without it the dev overlay opens a
// permanent issue badge. Only the development server gets it; the production policy (next build/start) stays eval-free (R4S-04).
const scriptSrc =
  process.env.NODE_ENV === "development"
    ? "script-src 'self' 'unsafe-inline' 'unsafe-eval'"
    : "script-src 'self' 'unsafe-inline'";

const config: NextConfig = {
  agentRules: false, // The repository owns its instruction hierarchy.
  devIndicators: false, // Do not cover merchant controls in local visual acceptance.
  poweredByHeader: false,
  reactStrictMode: true,
  // The orders proxy must see raw '?' and percent escapes before Next rewrites URLs.
  skipProxyUrlNormalize: true,
  output: "standalone",
  transpilePackages: [
    "@live-commerce/i18n",
    "@live-commerce/ui",
    "@live-commerce/format",
  ],
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "same-origin" },
          {
            key: "Content-Security-Policy",
            // R4S-04: same-origin sources only (no form-action: OAuth starts redirect off-site); nonce script-src is post-pilot.
            value: `default-src 'self'; ${scriptSrc}; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'`,
          },
        ],
      },
      // M7 returns a one-time claim credential. Preserve the BFF's policy through Next's global header layer.
      {
        source: "/api/stores/:store/live-sessions/:session/claims/bundles/:bundle/link",
        headers: [{ key: "Referrer-Policy", value: "no-referrer" }],
      },
      // staff-team D3: the invitation token is in this path, so neither the page's own subresources nor anything else may receive
      // it as a Referer, and the page is never cached. A real header (not only the <meta>) covers the stylesheet/script requests
      // the browser starts before it parses <head>.
      {
        source: "/:locale/invite/:path*",
        headers: [
          { key: "Referrer-Policy", value: "no-referrer" },
          { key: "Cache-Control", value: "no-store" },
        ],
      },
      // ads-ui U2: the Meta return URL carries the one-time `code`/`state`; nothing may receive it as a Referer.
      // Later entries win for the same key, so this overrides the global same-origin policy for this path only.
      {
        source: "/api/ads/meta/callback",
        headers: [{ key: "Referrer-Policy", value: "no-referrer" }],
      },
      // meta-connect: the merchant Page connect return carries the same one-time `code`/`state`.
      {
        source: "/api/meta/callback",
        headers: [{ key: "Referrer-Policy", value: "no-referrer" }],
      },
    ];
  },
};
export default function nextConfig(phase: string): NextConfig {
  // Public operator facts must be supplied deliberately to production builds.
  // No NEXT_PUBLIC binding: runtime env remains authoritative after a domain move.
  if (phase === PHASE_PRODUCTION_BUILD) companyConfig();
  return config;
}
