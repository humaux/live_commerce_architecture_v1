import type { NextConfig } from "next";

// Transport-only package. There is deliberately no buyer visual page or test
// route until the storefront composition is approved. DNS/TLS/ingress and final
// standalone asset packaging remain deployment gates, not BFF auth shortcuts.
const config: NextConfig = {
  agentRules: false,
  devIndicators: false,
  poweredByHeader: false,
  reactStrictMode: true,
  async headers() {
    return [{
      source: "/:path*",
      headers: [
        { key: "X-Content-Type-Options", value: "nosniff" },
        { key: "X-Frame-Options", value: "DENY" },
        { key: "Referrer-Policy", value: "same-origin" },
        { key: "Content-Security-Policy", value: "frame-ancestors 'none'; object-src 'none'; base-uri 'self'" },
      ],
    }];
  },
};
export default config;
