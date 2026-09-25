import { createHash } from "node:crypto";

// Fixed central return is NOT a webhook. Deliberately accept no Request argument:
// neither provider body/query nor a tenant cookie can become payment authority,
// an echoed value or a redirect. Deploy the configured absolute ReturnURL here.
const style = `:root{color-scheme:light;font:16px Arial,"PingFang SC","Microsoft YaHei",sans-serif;color:#142942;background:#fff}*{box-sizing:border-box}body{margin:0}main{max-width:720px;margin:0 auto;padding:48px 24px}h1{font-size:28px;color:#193c61;line-height:1.35;margin:0 0 24px}section{border-top:1px solid #dde5ee;padding:20px 0}h2{font-size:20px;margin:0 0 12px}p{line-height:1.6;margin:0;overflow-wrap:anywhere}::selection{background:#c9e6de;color:#142942}@media(max-width:480px){main{padding:32px 20px}h1{font-size:26px}}`;
const body = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Return to your store</title><style>${style}</style></head><body><!--
THESIS: A neutral return that asks the buyer to check the store, not a payment receipt.
OWN-WORLD: Inherited white surface, navy headings, system type and fine gray rules.
STORY: Close this provider-return tab and refresh the original order for its actual result.
FIRST VIEWPORT: One heading and three short language sections in a single column; no payment claim or fabricated store link.
FORM: Local extension of approved B, seed baaadec1, not a replacement world.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
--><main data-testid="payment-return"><h1>Return to your store</h1><section lang="en"><h2>Check your order</h2><p>This page does not confirm payment. Return to your original store tab and select “Refresh order” to check the result. You may close this tab. If you lost the store tab, reopen your store; this page cannot identify it for you.</p></section><section lang="zh-CN"><h2>返回商店查询订单</h2><p>此页面不代表付款成功。请回到原来的商店标签页，点击“刷新订单”查询结果。你可以关闭本页。若原标签页已关闭，请自行重新打开商店；此页面无法判断你所属的商店。</p></section><section lang="zh-TW"><h2>返回商店查詢訂單</h2><p>此頁面不代表付款成功。請回到原本的商店分頁，點選「重新整理訂單」查詢結果。你可以關閉本頁。若原分頁已關閉，請自行重新開啟商店；此頁面無法判斷你所屬的商店。</p></section></main></body></html>`;
const styleHash = createHash("sha256").update(style).digest("base64");

export function paymentReturn(): Response {
  return new Response(body, {
    status: 200,
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-store",
      "Referrer-Policy": "no-referrer",
      "X-Content-Type-Options": "nosniff",
      "X-Frame-Options": "DENY",
      "X-Robots-Tag": "noindex, nofollow",
      "Content-Security-Policy": `default-src 'none'; style-src 'sha256-${styleHash}'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'`,
    },
  });
}
