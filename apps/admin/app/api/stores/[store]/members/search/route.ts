// Purpose: expose the frozen LP-1 search member operation.
// Depends on: members/proxy server authority and Go/PG member policy.
// Used by: admin members clients; no automatic retries.
import { prepare, finish } from "@/app/api/stores/[store]/members/proxy";
import { callBackend } from "@/lib/backend";
type Context = { params: Promise<{ store: string; member?: string }> };
/** Forward the exact POST operation through server authority. */
export async function POST(request: Request, context: Context) {
  const prepared = await prepare(request, context, "search");
  if (prepared instanceof Response) return prepared;
  const { token, store, member, headers, body } = prepared;
  const response = await callBackend(`members/search${new URL(request.url).search}`, { method: "POST", headers, body }, token, store);
  return finish(response, member, "search");
}
