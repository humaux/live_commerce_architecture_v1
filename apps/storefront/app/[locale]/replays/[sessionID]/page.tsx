// Purpose: render public buyer live replay detail route on /{locale}/replays/{sessionID}.
// Depends on: gate(), getReplay() and storeCurrency() via lib/shop-upstream; official Meta video embed slot.
// Used by: storefront buyer navigation to view past live replay session and featured products.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import type { Locale } from "@live-commerce/i18n";
import { alternates, noindex } from "../../../../lib/shop-page";
import { gate, must } from "../../../../lib/shop-page";
import { getReplay, storeCurrency } from "../../../../lib/shop-upstream";
import { formatReplayEnded, shopCopy } from "../../../../lib/shop-copy";
import { productPath, replaysPath } from "../../../../lib/routes";
import { formatMoney } from "../../../../lib/money";
import styles from "../replay.module.css";

export const dynamic = "force-dynamic";

const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string; sessionID: string }>;
}): Promise<Metadata> {
  const { locale: raw, sessionID } = await params;
  if (!isLocale(raw) || !UUID.test(sessionID)) return { robots: noindex };
  const locale = raw as Locale;
  const { shop } = await gate();
  const replayResult = await getReplay(sessionID);
  if (replayResult.state !== "ok") return { robots: noindex };
  return {
    title: `${replayResult.value.title} | ${shop.design.profile.name}`,
    description: replayResult.value.title,
    alternates: alternates(locale, `/replays/${sessionID}`),
  };
}

export default async function ReplayDetailPage({
  params,
}: {
  params: Promise<{ locale: string; sessionID: string }>;
}) {
  const { locale: raw, sessionID } = await params;
  if (!isLocale(raw) || !UUID.test(sessionID)) notFound();
  const locale = raw as Locale;
  const { shop } = await gate();
  const copy = shopCopy[locale];
  const replay = must(await getReplay(sessionID));
  const currency = must(await storeCurrency());
  const listHref = replaysPath(locale);

  return (
    <div className={`${styles.root} replay-root`}>
      <header className={styles.appBar} data-name="Buyer app bar">
        <p className={styles.storeBrand}>{shop.design.profile.name}</p>
        <a href={listHref} className={styles.appBarBackBtn} data-name="Control · 返回">
          {copy.back}
        </a>
      </header>

      <main className={styles.contentColumn}>
        <h1 className={styles.pageTitle}>{replay.title}</h1>

        <div className={styles.taskViewport} data-name="Task viewport · original request stays in this state">
          <div className={styles.taskCard} data-name="Task content">
            <p className={styles.cardTime}>{formatReplayEnded(replay.ended_at, locale)}</p>

            <div className={styles.videoSlot} data-name="Official Meta embed slot · no fabricated playback">
              <div className={styles.videoHeader}>
                <p className={styles.videoLabel}>{copy.replayVideo}</p>
              </div>
              <iframe
                src={replay.embed_url}
                title={copy.replayVideo}
                className={styles.videoIframe}
                scrolling="no"
                allowFullScreen={true}
              />
            </div>

            {replay.offers.map((offer) => {
              const variant = offer.product.variants.find((v) => v.sku_id === offer.sku_id);
              if (!variant) return null;
              const formattedPrice = formatMoney(locale, variant.price_minor, currency);
              const isSoldOut = variant.stock === "out";

              return (
                <div key={offer.offer_id} style={{ display: "flex", flexDirection: "column", gap: "var(--space-3)", width: "100%" }}>
                  <p className={styles.productSectionTitle}>{copy.replayProducts}</p>
                  <div className={styles.productDetails}>
                    <p>{`${offer.product.title} · ${formattedPrice}`}</p>
                    <p>{isSoldOut ? copy.replaySoldOut : copy.replayInStock}</p>
                  </div>
                  <p className={styles.priceNotice}>{copy.replayPriceNotice}</p>
                  <a
                    href={productPath(locale, offer.product.slug || offer.product.id)}
                    className={styles.actionBtn}
                    data-name="Control · 查看商品"
                  >
                    {copy.viewProduct}
                  </a>
                </div>
              );
            })}
          </div>
        </div>

        <div className={styles.bottomActions} data-name="Fixed actions · outside scroll viewport">
          <a href={listHref} className={styles.actionBtn} data-name="Control · 返回回放列表">
            {copy.backToReplays}
          </a>
        </div>
      </main>
    </div>
  );
}
