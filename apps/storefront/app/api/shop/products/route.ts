// GET /api/shop/products?collection=&q=&sort=&min=&max=&after=&limit= -> Go GET /v1/buyer/catalog/v2/products
// (internal/buyerhttp/catalogv2.go). Public, cookie-less: used by the browser "Load more" button (components/ProductGrid.tsx)
// because the browser cannot hold the BFF key. Only the whitelisted query keys are rebuilt and forwarded (lib/shop-query.ts);
// the store comes from the Host-derived origin inside lib/shop-upstream.ts, never from the request. no-store: stock hints.
import { NextResponse } from "next/server";
import { listProducts } from "../../../../lib/shop-upstream";
import { parseListQuery } from "../../../../lib/shop-query";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const result = await listProducts(parseListQuery((k) => params.get(k)));
  const headers = { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" };
  if (result.state === "ok") return NextResponse.json(result.value, { headers });
  return NextResponse.json({ code: result.state === "missing" ? "not_found" : "unavailable" }, { status: result.state === "missing" ? 404 : 503, headers });
}
