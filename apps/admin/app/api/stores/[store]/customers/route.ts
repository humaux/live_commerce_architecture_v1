// Purpose: route tag-filtered customer lists through W6's closed grammar while preserving the legacy list.
// Depends on: customer-tags-request closed grammar, customer-tags-bff server authority and the existing scoped catch-all GET.
// Used by: Customers list search/filter/pagination; detail and privacy routes retain their own handlers.
import { customerTagsBFF } from "@/lib/customer-tags-bff";
import { customerTagsRoute } from "@/lib/customer-tags-request";
import { GET as legacyGET } from "../[...resource]/route";

/** The shared raw tag grammar selects W6; other requests retain the legacy closed list grammar. */
export async function GET(request: Request, context: { params: Promise<{ store: string }> }): Promise<Response> {
  const { store } = await context.params;
  if (customerTagsRoute(request.method, "customers", new URL(request.url).search) === "customer-list") return customerTagsBFF(request, store, "customers");
  return legacyGET(request, { params: Promise.resolve({ store, resource: ["customers"] }) });
}
