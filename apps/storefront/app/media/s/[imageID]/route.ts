// GET /media/s/{imageID} -> Go GET /v1/buyer/media/s/{image} (internal/buyerhttp/design.go -> design.buyer_media, migrations/0087).
// Store design media (logo, favicon, hero and image_text pictures); 404 unless the image belongs to the published store.
// Behaviour in lib/media-proxy.ts.
import { UUID, notFound, proxyImage } from "../../../../lib/media-proxy.ts";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request, { params }: { params: Promise<{ imageID: string }> }): Promise<Response> {
  const { imageID } = await params;
  if (!UUID.test(imageID)) return notFound();
  return proxyImage(request, `/v1/buyer/media/s/${imageID}`);
}
