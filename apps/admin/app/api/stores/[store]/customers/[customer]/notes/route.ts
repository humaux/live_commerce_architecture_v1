// Purpose: exact W6-01B customer tag/note BFF leaf -> Go internal/httpapi/customer_tags.go.
// Depends on: customer-tags-bff authenticated closed grammar and session/store/CSRF guards.
// Used by: CustomerTags browser controls; private no-store responses.
import { customerTagsBFF } from "@/lib/customer-tags-bff";

type Context = { params: Promise<{ store: string; customer: string }> };
async function route(request: Request, context: Context) {
  const { store, customer } = await context.params;
  return customerTagsBFF(request, store, `customers/${customer}/notes`);
}
/** Forward only the frozen GET operation through server authority; no automatic retries. */
export const GET = route;
/** Forward only the frozen POST operation through server authority; no automatic retries. */
export const POST = route;
