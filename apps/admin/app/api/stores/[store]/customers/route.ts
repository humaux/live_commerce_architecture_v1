// Purpose: route tag-filtered customer lists through W6's closed grammar while preserving the legacy list.
// Depends on: customer-tags-bff server authority and the existing scoped catch-all GET.
// Used by: Customers list search/filter/pagination; detail and privacy routes retain their own handlers.
import { customerTagsBFF } from "@/lib/customer-tags-bff";
import { GET as legacyGET } from "../[...resource]/route";

/** A tag query selects the exact W6 list grammar; all other list requests retain existing behavior. */
export async function GET(request: Request, context: { params: Promise<{ store: string }> }): Promise<Response> {
  const { store } = await context.params;
  if (new URL(request.url).searchParams.has("tag")) return customerTagsBFF(request, store, "customers");
  return legacyGET(request, { params: Promise.resolve({ store, resource: ["customers"] }) });
}
