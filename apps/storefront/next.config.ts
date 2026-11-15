import type { NextConfig } from "next";

// React's development build rebuilds call stacks with eval() (Next documents 'unsafe-eval' for `next dev`): without it the dev overlay opens a
// permanent issue badge. Only the development server gets it; the production policy (next build/start) stays eval-free (R4S-04).
const scriptSrc = process.env.NODE_ENV === "development" ? "script-src 'self' 'unsafe-inline' 'unsafe-eval'" : "script-src 'self' 'unsafe-inline'";

// Base shopping Content-Security-Policy (R4S-04): same-origin fetch/img/font/script sources only.
const shoppingCSP = `default-src 'self'; ${scriptSrc}; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self' https://logistics-stage.ecpay.com.tw/Express/map https://logistics.ecpay.com.tw/Express/map`;

// Approved B product surface shares the verified buyer BFF. There are no public
// fixture routes. DNS/TLS/ingress remain independent deployment gates.
const config: NextConfig = {
  agentRules: false,
  devIndicators: false,
  poweredByHeader: false,
  reactStrictMode: true,
  // Canonical-host 301 owns the first redirect; keep the buyer's path/query intact.
  skipTrailingSlashRedirect: true,
  skipProxyUrlNormalize: true,
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
        ],
      },
      {
        // Next configuration headers override Route Handler headers. The fixed
        // neutral return supplies its own default-src/form-action none + style
        // hash and no-referrer, so the shopping policy must not match that path.
        source: "/((?!payment/return/?$).*)",
        headers: [
          { key: "Referrer-Policy", value: "same-origin" },
          {
            key: "Content-Security-Policy",
            value: shoppingCSP,
          },
        ],
      },
      {
        // Replay detail pages frame Facebook's official embedded video player (contracts/storefront-v2.md §G6).
        // Scoped strictly to valid UUID session IDs: later rule overrides the base shopping Content-Security-Policy
        // with the addition of frame-src for Facebook's video plugin path only.
        source:
          "/:locale(zh-CN|zh-TW|en)/replays/:sessionID([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})",
        headers: [
          {
            key: "Content-Security-Policy",
            value: `${shoppingCSP}; frame-src https://www.facebook.com/plugins/video.php`,
          },
        ],
      },
      {
        // The claim page and the manual-order link page (storefront-v2 G3) hold a bearer link token in memory (live-keyword-claims-v1
        // §11.1): no Referer at all, and no scripts or connections beyond this origin.
        // Listed last so it overrides the shopping Referrer-Policy for this path only.
        source: "/:locale(zh-CN|zh-TW|en)/:page(claim|order-link)",
        headers: [
          { key: "Referrer-Policy", value: "no-referrer" },
          {
            key: "Content-Security-Policy",
            value:
              "frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'none'; connect-src 'self'",
          },
        ],
      },
    ];
  },
};
export default config;
