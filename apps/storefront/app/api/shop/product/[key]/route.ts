// GET /api/shop/product/{slug_or_id} -> Go GET /v1/buyer/catalog/v2/products/{slug_or_id} (internal/buyerhttp/catalogv2.go).
// Public, cookie-less: the cart drawer/page asks it for the variant title, compare-at price and stock hint of the products in
// the buyer's cart (components/CartLines.tsx). Same trust shape as app/api/shop/products/route.ts; 404 is identical for
// unknown, draft, archived and foreign products.
import { NextResponse } from "next/server";
import { getProduct } from "../../../../../lib/shop-upstream";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(_request: Request, { params }: { params: Promise<{ key: string }> }): Promise<Response> {
  const { key } = await params;
  const headers = { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" };
  if (!/^[a-z0-9-]{1,80}$/.test(key)) return NextResponse.json({ code: "not_found" }, { status: 404, headers });
  const result = await getProduct(key);
  if (result.state === "ok") return NextResponse.json(result.value, { headers });
  return NextResponse.json({ code: result.state === "missing" ? "not_found" : "unavailable" }, { status: result.state === "missing" ? 404 : 503, headers });
}
