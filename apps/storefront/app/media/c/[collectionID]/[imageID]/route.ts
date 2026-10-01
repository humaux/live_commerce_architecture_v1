// GET /media/c/{collectionID}/{imageID} -> Go GET /v1/buyer/media/c/{collection}/{image} (internal/buyerhttp/catalogv2.go ->
// catalog.buyer_collection_image, migrations/0086; contracts/storefront-v2.md section A). Collection photo bytes; 404 unless
// the collection is active in the published store. Behaviour in lib/media-proxy.ts.
import { UUID, notFound, proxyImage } from "../../../../../lib/media-proxy.ts";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request, { params }: { params: Promise<{ collectionID: string; imageID: string }> }): Promise<Response> {
  const { collectionID, imageID } = await params;
  if (!UUID.test(collectionID) || !UUID.test(imageID)) return notFound();
  return proxyImage(request, `/v1/buyer/media/c/${collectionID}/${imageID}`);
}
