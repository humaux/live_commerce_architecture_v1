// Purpose: render public buyer live replay list route on /{locale}/replays.
// Depends on: gate() and listReplays() via lib/shop-upstream; shopCopy and replayPath.
// Used by: storefront buyer navigation to browse past live sessions.
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import type { Locale } from "@live-commerce/i18n";
import { alternates, noindex } from "../../../lib/shop-page";
import { gate, must } from "../../../lib/shop-page";
import { listReplays } from "../../../lib/shop-upstream";
import { formatReplayEnded, shopCopy } from "../../../lib/shop-copy";
import { replayPath } from "../../../lib/routes";
import styles from "./replay.module.css";

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale: raw } = await params;
  if (!isLocale(raw)) return { robots: noindex };
  const locale = raw as Locale;
  const { shop } = await gate();
  const copy = shopCopy[locale];
  return {
    title: `${copy.replaysTitle} | ${shop.design.profile.name}`,
    description: copy.replaysTitle,
    alternates: alternates(locale, "/replays"),
  };
}

export default async function ReplaysPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale: raw } = await params;
  if (!isLocale(raw)) notFound();
  const locale = raw as Locale;
  const { shop } = await gate();
  const copy = shopCopy[locale];
  const replays = must(await listReplays());
  const storeHome = `/${locale}`;

  return (
    <div className={`${styles.root} replay-root`}>
      <header className={styles.appBar} data-name="Buyer app bar">
        <p className={styles.storeBrand}>{shop.design.profile.name}</p>
        <Link href={storeHome} className={styles.appBarBackBtn} data-name="Control · 返回">
          {copy.back}
        </Link>
      </header>

      <main className={styles.contentColumn}>
        <h1 className={styles.pageTitle}>{copy.replaysTitle}</h1>

        <div className={styles.taskViewport} data-name="Task viewport · original request stays in this state">
          {replays.items.length === 0 ? (
            <div className={styles.taskCard} data-name="Task content">
              <p className={styles.cardTitle}>{copy.noReplaysTitle}</p>
              <p className={styles.cardHint}>{copy.noReplaysHint}</p>
            </div>
          ) : (
            replays.items.map((item) => (
              <div key={item.session_id} className={styles.taskCard} data-name="Task content">
                <p className={styles.cardTitle}>{item.title}</p>
                <p className={styles.cardTime}>{formatReplayEnded(item.ended_at, locale)}</p>
                <a
                  href={replayPath(locale, item.session_id)}
                  className={styles.actionBtn}
                  data-name="Control · 觀看回放"
                >
                  {copy.watchReplay}
                </a>
              </div>
            ))
          )}
        </div>

        <div className={styles.bottomActions} data-name="Fixed actions · outside scroll viewport">
          <Link href={storeHome} className={styles.actionBtn} data-name="Control · 返回商店">
            {copy.backToShop}
          </Link>
        </div>
      </main>
    </div>
  );
}
