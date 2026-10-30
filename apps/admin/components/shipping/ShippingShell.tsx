// Purpose: Compose the signed light admin shell around the shipping task without changing other modules.
// Depends on: React, i18n, shell-copy, routes, team-model, company, model and shipping.module.css.
// Used by: ShippingCenter.tsx.
// Invariants: navigation is filtered by authenticated store permissions; language changes preserve only the non-private URL.
'use client';
import type { ReactNode } from 'react';
import { locales, shippingCopy, type Locale } from '@live-commerce/i18n';
import type { Store } from '@/lib/model';
import { navAccessFrom } from '@/lib/team-model';
import { visibleGroups } from '@/src/routes';
import { shellCopy } from '@/src/shell-copy';
import { company } from '@/lib/company';
import s from './shipping.module.css';

export function ShippingShell({ locale, store, demo, children }: {
  locale: Locale; store: Store | null; demo: boolean; children: ReactNode;
}) {
  const c = shippingCopy[locale], nav = shellCopy[locale];
  const groups = visibleGroups(navAccessFrom({ items: store ? [store] : [] }, store?.id ?? null));
  const href = (path: string) => `/${locale}${path}${store ? `?store=${store.id}` : ''}`;
  return <div className={s.shell}>
    <aside className={s.sidebar}>
      <div className={s.brand}>{company.productName}</div>
      <div className={s.store}>{store?.name}{demo ? ` · ${c.demo}` : ''}</div>
      <nav aria-label={nav.navigation}>
        {groups.map(group => group.routes.length === 1 ?
          <a key={group.id} href={href(group.routes[0].path)} aria-current={group.id === 'orders' ? 'page' : undefined}>
            {nav[group.id]}
          </a> : <details key={group.id}>
            <summary>{nav[group.id]}</summary>
            {group.routes.map(route => <a key={route.id} href={href(route.path)}>{nav[route.labelKey]}</a>)}
          </details>)}
      </nav>
    </aside>
    <header className={s.topbar}>
      <span className={s.mobileStore}>{demo ? c.demo : store?.name}</span>
      <nav className={s.breadcrumb} aria-label={nav.navigation}>
        <a href={href('/')}>{nav.overview}</a><span>/</span><span>{nav.orders}</span><span>/</span>
        <a href={href('/orders')}>{nav.orderList}</a>
      </nav>
      <select className={s.language} aria-label={c.language} value={locale} onChange={event => {
        const url = new URL(window.location.href);
        url.pathname = url.pathname.replace(/^\/(zh-TW|zh-CN|en)/, `/${event.currentTarget.value}`);
        window.location.assign(`${url.pathname}${url.search}`);
      }}>
        {locales.map(value => <option key={value} value={value}>{shippingCopy[value].localeShort}</option>)}
      </select>
    </header>
    {children}
  </div>;
}
