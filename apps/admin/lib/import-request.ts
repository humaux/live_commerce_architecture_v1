// Purpose: exact migration-import request grammar and header-only mapping guesses.
// Depends on: import-model fields/limits and canonical UUID; backend remains the field/data authority.
// Used by: import BFF and import-client/ImportWizard; no generic forwarding.
import { canonicalUUID } from "./orders-model.ts";
import { validImportMapping, type ImportKind, type ImportMapping, importFields, maxImportRows } from "./import-model.ts";

/** Closed request kinds for the shared [param]/[action] leaf. */
export type ImportRoute = { kind: ImportKind; action: "preview" | "commit" } | { batch: string; action: "results.csv" };
/** Match exactly four POSTs and failed-only result GET, avoiding Next sibling parameter conflicts. */
export function importRoute(method: string, param: string, action: string): ImportRoute | null {
  if (method === "POST" && (param === "customers" || param === "orders") && (action === "preview" || action === "commit")) return {kind:param, action};
  if (method === "GET" && canonicalUUID.test(param) && action === "results.csv") return {batch:param, action};
  return null;
}
/** Validate canonical query/body declarations before trusted authorization/forwarding. */
export function validImportRequest(request: Request, route: ImportRoute): boolean {
  if (request.headers.has("idempotency-key") || request.headers.has("transfer-encoding")) return false;
  const raw = new URL(request.url).search.slice(1);
  if (new TextEncoder().encode(raw).length > 4096 || /%(?![a-fA-F0-9]{2})/.test(raw) || raw.includes(";") || request.url.endsWith("?")) return false;
  const query = new URLSearchParams(raw), seen = new Set<string>();
  for (const [key,value] of query) {
    if (!value || seen.has(key)) return false;
    seen.add(key);
    if (route.action === "results.csv") { if (key !== "only" || value !== "failed") return false; }
    else if (key === "mapping") { try { if (!validImportMapping(JSON.parse(value), route.kind)) return false; } catch {return false;} }
    else if (key !== "expected_apply_rows" || route.action !== "commit" || !/^(?:0|[1-9][0-9]{0,3})$/.test(value) || Number(value) > maxImportRows) return false;
  }
  if (route.action === "results.csv") return raw === "only=failed" && request.body === null && (!request.headers.has("content-length") || request.headers.get("content-length") === "0");
  return (route.action !== "commit" || seen.has("expected_apply_rows")) && /^text\/csv(?:\s*;\s*charset=utf-8)?$/i.test(request.headers.get("content-type") ?? "");
}

function folded(header: string) {
  return header.trim().replace(/[！-～]/g, (ch) => String.fromCharCode(ch.charCodeAt(0)-0xfee0)).replaceAll("　", "_").toLowerCase().replace(/[ -]/g, "_");
}
// Backend aliases are copied below, as column metadata only; no sample customer or order rows.
const aliases: Record<ImportKind, Record<string, readonly string[]>> = {"customers":{"external_id":["customer_id","id","member_id","customer_number","顧客編號","顾客编号","顧客id","顾客id","客戶編號","客户编号","會員編號","会员编号","顧客代碼"],"name":["name","full_name","customer_name","姓名","顧客姓名","顾客姓名","顧客名稱","顾客名称","客戶名稱","客户名称","會員姓名","名稱","名字"],"phone":["phone","mobile","phone_number","mobile_phone","手機","手机","手機號碼","手机号码","行動電話","電話","电话","聯絡電話","联系电话"],"email":["email","e_mail","email_address","電子郵件","电子邮件","電子信箱","电子邮箱","電郵","信箱"],"consent":["accepts_marketing","marketing_consent","marketing","subscribed","consent","同意行銷","同意行销","同意行銷訊息","接受行銷","接受行销","同意接收行銷","訂閱電子報","订阅电子报"]},"orders":{"order_id":["order_id","order_number","order_no","訂單編號","订单编号","訂單號碼","订单号码","訂單號","订单号"],"customer_id":["customer_id","member_id","customer_number","顧客編號","顾客编号","顧客id","顾客id","客戶編號","客户编号","會員編號","会员编号","顧客代碼"],"ordered_at":["ordered_at","order_date","order_time","created_at","訂單日期","订单日期","訂單時間","订单时间","下單時間","下单时间","下單日期"],"status":["status","order_status","訂單狀態","订单状态","狀態","状态"],"total":["total","order_total","total_amount","grand_total","訂單總金額","订单总金额","訂單金額","订单金额","總金額","总金额","訂單總額","合計"],"item_name":["item_name","product_name","product","商品名稱","商品名称","商品","品項","品名"],"item_qty":["item_qty","quantity","qty","數量","数量","商品數量","商品数量"],"city":["city","shipping_city","delivery_city","城市","縣市","县市","配送城市","收件城市","收件縣市"]}};

/** Guess only unambiguous aliases; every empty value explicitly remains unmapped for manual review. */
export function guessImportMapping(kind: ImportKind, headers: string[]): ImportMapping {
  const result: ImportMapping = {}, used = new Set<string>();
  for (const field of importFields[kind]) {
    const header = headers.find((h) => !used.has(h) && aliases[kind][field]?.includes(folded(h)) && headers.filter((v) => folded(v) === folded(h)).length === 1);
    result[field] = header ?? "";
    if (header) used.add(header);
  }
  return result;
}
