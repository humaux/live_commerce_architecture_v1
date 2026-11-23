import { finish, prepare } from "@/app/api/stores/[store]/members/proxy";
import { localError } from "@/lib/auth";
import { callBackend } from "@/lib/backend";
import { canonicalUUID } from "@/lib/orders-model";

type Context = { params: Promise<{ store: string; order: string }> };

export async function GET(request: Request, context: Context) {
  const { order } = await context.params;
  if (!canonicalUUID.test(order)) return localError(404, "not_found");
  const prepared = await prepare(request, context, "award-recovery");
  if (prepared instanceof Response) return prepared;
  const { headers, token, store } = prepared;
  return finish(await callBackend(`orders/${order}/member-award-recovery`, { method: "GET", headers }, token, store));
}

export async function POST(request: Request, context: Context) {
  const { order } = await context.params;
  if (!canonicalUUID.test(order)) return localError(404, "not_found");
  const prepared = await prepare(request, context, "award-recovery");
  if (prepared instanceof Response) return prepared;
  const { headers, body, token, store } = prepared;
  if (!body || !/^\s*\{\s*\}\s*$/.test(body)) return localError(400, "invalid_json");
  return finish(await callBackend(`orders/${order}/member-award-recovery`, { method: "POST", headers, body }, token, store));
}
