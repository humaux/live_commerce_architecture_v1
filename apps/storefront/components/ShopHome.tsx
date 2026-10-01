"use client";

// Storefront home /{locale} (catalog-media CM6): the published store name and a grid of its active products.
// BFF: GET /api/buyer/catalog?limit=100[&cursor=] -> Go /v1/buyer/catalog (internal/buyerhttp, storefront.ListCatalog:
// active products and active SKUs only, with `images` and `store_name`); the session comes from the same browser
// coordinator as the product page (lib/buyer-client.ts: HttpOnly cookie, never read here). Photo bytes load from
// /media/p/{product}/{image} (app/media/p/[productID]/[imageID]/route.ts). Cards link to the existing product page
// /{locale}/products/{id}, which owns cart, quote and checkout. Non-goals: no search, no categories, no cart here.
// ponytail: at most 5 catalog pages (500 SKUs) are read; add "load more" when a store passes that.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import {
  BuyerClientError,
  initializeBuyerSession,
  readBuyerSession,
  resetBuyerSession,
} from "../lib/buyer-client";
import { readPurchase, orderRecoveryRequired } from "../lib/purchase";
import type { Product } from "../lib/purchase";
import { groupProducts, mediaPath, validHomePage } from "../lib/home";
import type { HomeProduct } from "../lib/home";
import { homeCopy } from "../lib/home-copy";
import { purchaseCopy } from "../lib/purchase-copy";
import styles from "./ShopHome.module.css";

const MAX_PAGES = 5;

export default function ShopHome({
  locale: initialLocale,
  demonstration = false,
}: {
  locale: Locale;
  demonstration?: boolean;
}) {
  const [locale, setLocale] = useState(initialLocale);
  const copy = purchaseCopy[locale];
  const home = homeCopy[locale];
  const [products, setProducts] = useState<HomeProduct[]>([]);
  const [storeName, setStoreName] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<"failed" | "session" | null>(null);
  const epoch = useRef(0);

  const money = (amount: number, currency: string) => {
    const formatter = new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
      currencyDisplay: locale === "zh-TW" && currency === "TWD" ? "code" : "symbol",
    });
    return formatter.format(amount / 10 ** (formatter.resolvedOptions().maximumFractionDigits ?? 2));
  };

  async function load() {
    const version = ++epoch.current;
    setLoading(true);
    setError(null);
    try {
      const session = await initializeBuyerSession();
      if (!session.context) throw new BuyerClientError("requires_reset");
      const rows: Product[] = [];
      let cursor = "";
      let name = "";
      for (let page = 0; page < MAX_PAGES; page++) {
        const result = await readPurchase(
          `catalog?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
          session.context,
          validHomePage,
        );
        rows.push(...result.items);
        name = result.store_name;
        cursor = result.next_cursor;
        if (!cursor) break;
      }
      if (version !== epoch.current) return;
      setStoreName(name);
      setProducts(groupProducts(rows));
    } catch (reason) {
      if (version !== epoch.current) return;
      const session =
        reason instanceof BuyerClientError &&
        (["context_changed", "requires_reset"].includes(reason.code) || reason.status === 401);
      setError(session ? "session" : "failed");
    } finally {
      if (version === epoch.current) setLoading(false);
    }
  }

  // Renewal guard identical to the product page: an expired session that still owns an unresolved order is never
  // reset from here (the order page recovers it).
  async function retry() {
    if (error === "session") {
      try {
        const current = await readBuyerSession();
        if (current.context && current.state !== "active" && orderRecoveryRequired(current.context)) {
          setError("failed");
          return;
        }
        if (current.context && current.state !== "active") await resetBuyerSession(current.context);
      } catch {
        setError("failed");
        return;
      }
    }
    await load();
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <>
      <header className="shop-header">
        <span>{storeName || copy.store}</span>
        <label className="locale">
          <span className="sr-only">{copy.language}</span>
          <select
            value={locale}
            onChange={(event) => {
              const next = event.target.value as Locale;
              window.history.replaceState(window.history.state, "", `/${next}`);
              document.documentElement.lang = next;
              setLocale(next);
            }}
          >
            <option value="zh-CN">简体中文</option>
            <option value="zh-TW">繁體中文</option>
            <option value="en">English</option>
          </select>
        </label>
      </header>
      {demonstration && <p className="demonstration">{copy.demonstration}</p>}
      <main className={styles.home} aria-busy={loading}>
        {error && (
          <div role="alert" className="purchase-error">
            <p>{error === "session" ? copy.session : copy.failed}</p>
            <button data-testid="home-retry" onClick={() => void retry()}>
              {error === "session" ? copy.renew : copy.retry}
            </button>
          </div>
        )}
        {loading ? (
          <p role="status">{copy.loading}</p>
        ) : error ? null : (
          <>
            <h1>{storeName || copy.store}</h1>
            {products.length === 0 ? (
              <div className={styles.empty} data-testid="home-empty">
                <h2>{home.empty}</h2>
                <p>{home.emptyHint}</p>
              </div>
            ) : (
              <ul className={styles.grid} aria-label={home.products}>
                {products.map((product) => (
                  <li key={product.product_id}>
                    <a
                      className={styles.card}
                      href={`/${locale}/products/${product.product_id}`}
                      data-testid="home-product"
                    >
                      {product.cover ? (
                        <img
                          className={styles.photo}
                          src={mediaPath(product.product_id, product.cover.id)}
                          alt=""
                          loading="lazy"
                        />
                      ) : (
                        <div className={styles.noPhoto}>{home.noPhoto}</div>
                      )}
                      <div className={styles.body}>
                        <span className={styles.name}>{product.name}</span>
                        <span className={styles.price}>
                          {locale === "en" ? `${home.from} ` : ""}
                          {money(product.price_minor, product.currency)}
                          {locale === "en" ? "" : ` ${home.from}`}
                        </span>
                      </div>
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </main>
    </>
  );
}
