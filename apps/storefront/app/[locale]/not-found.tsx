// Branded 404 for every /{locale}/... page. Two meanings share it: "this page/product does not exist" and "this host is not a
// published store" (Go answers both as 404, lib/shop-page.ts gate() calls notFound() for either). It tells them apart with the
// design read (closed vs ok). The locale comes from the x-shop-locale request header that proxy.ts derives from the URL, because
// not-found files receive no route params. BFF/Go: design/published through lib/shop-upstream.ts. HTTP status stays 404.
import Link from "next/link";
import { headers } from "next/headers";
import { isLocale } from "@live-commerce/i18n";
import { loadShop, previewToken } from "../../lib/shop-upstream";
import { shopCopy } from "../../lib/shop-copy";

export default async function NotFound() {
  const raw = (await headers()).get("x-shop-locale") ?? "zh-TW";
  const locale = isLocale(raw) ? raw : "zh-TW";
  const copy = shopCopy[locale];
  const shop = await loadShop(await previewToken());
  const closed = shop.state === "closed";
  return (
    <main className="sf-wrap">
      <div className="sf-empty sf-empty--page" data-testid={closed ? "store-closed" : "not-found"}>
        <p className="sf-empty__code" aria-hidden="true">
          {closed ? "·" : "404"}
        </p>
        <h1 className="sf-empty__title">{closed ? copy.closedTitle : copy.notFoundTitle}</h1>
        <p className="sf-muted">{closed ? copy.closedBody : copy.notFoundBody}</p>
        {!closed && (
          <Link className="sf-btn sf-btn--primary" href={`/${locale}`}>
            {copy.backHome}
          </Link>
        )}
      </div>
    </main>
  );
}
