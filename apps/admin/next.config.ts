import type { NextConfig } from "next";

const config: NextConfig = {
  agentRules: false, // The repository owns its instruction hierarchy.
  devIndicators: false, // Do not cover merchant controls in local visual acceptance.
  poweredByHeader: false,
  reactStrictMode: true,
  // The orders proxy must see raw '?' and percent escapes before Next rewrites URLs.
  skipProxyUrlNormalize: true,
  output: "standalone",
  transpilePackages: ["@live-commerce/i18n"],
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
            value: "frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
          },
        ],
      },
    ];
  },
};
export default config;
