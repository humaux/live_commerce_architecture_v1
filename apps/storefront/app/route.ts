// GET / -> 308 /zh-TW. BFF: none (no Go call, no data read).
// Why: a buyer opening the bare shop address (ads, shared link, typed host) hit a 404 because the storefront only serves
// /{locale}/...; the home page is /{locale} (app/[locale]/page.tsx). Same default and same relative-Location rule as
// app/products/[productID]/route.ts: zh-TW is the pilot market, the redirect never depends on a caller-supplied Host.
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(): Promise<Response> {
  return new Response(null, { status: 308, headers: { Location: "/zh-TW", "Cache-Control": "no-store" } });
}
