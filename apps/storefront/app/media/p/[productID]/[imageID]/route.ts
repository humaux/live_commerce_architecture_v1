// GET /media/p/{productID}/{imageID} on the verified storefront host -> Go GET /v1/buyer/media/p/{product}/{image}
// (internal/buyerhttp/media.go -> catalog.buyer_media_image, migrations/0082). Public product photo bytes for the home, the
// product page gallery, cards and the Meta feed image_link. All behaviour (origin from Host, type/size limits, 404 collapse,
// immutable cache) lives in lib/media-proxy.ts; this file only validates the two path ids.
import { UUID, notFound, proxyImage } from "../../../../../lib/media-proxy.ts";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request, { params }: { params: Promise<{ productID: string; imageID: string }> }): Promise<Response> {
  const { productID, imageID } = await params;
  if (!UUID.test(productID) || !UUID.test(imageID)) return notFound();
  return proxyImage(request, `/v1/buyer/media/p/${productID}/${imageID}`);
}
