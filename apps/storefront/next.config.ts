import type { NextConfig } from "next";

// Approved B product surface shares the verified buyer BFF. There are no public
// fixture routes. DNS/TLS/ingress remain independent deployment gates.
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
