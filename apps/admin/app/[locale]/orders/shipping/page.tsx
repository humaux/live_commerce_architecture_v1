// Purpose: Validate the authenticated store and non-private shipping flow URL before rendering.
// Depends on: next/headers, next/navigation, React Suspense, i18n, auth, orders-model and ShippingCenter.
// Used by: Next.js /[locale]/orders/shipping.
// Invariants: store membership comes from the server session; URL contains only store/order IDs, cursor and step.
import { Suspense } from 'react';
import { headers } from 'next/headers';
import { notFound } from 'next/navigation';
import { isLocale } from '@live-commerce/i18n';
import { authConfig, authenticatedStores, exactCookieHeader, isBase64URL32, safeError, SESSION_COOKIE } from '@/lib/auth';
import { canonicalCursor, canonicalUUID } from '@/lib/orders-model';
import type { OrderReadCode } from '@/lib/orders-client';
import type { Store } from '@/lib/model';
import AdminLoading from '@/components/AdminLoading';
import { ShippingCenter, type ShippingStep } from '@/components/shipping/ShippingCenter';

export default async function ShippingPage({ params, searchParams }: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (Object.keys(query).some(key => !['store', 'selected', 'step', 'cursor'].includes(key))) notFound();
  const single = (key: string) => {
    const value = query[key];
    if (value !== undefined && (typeof value !== 'string' || !value)) notFound();
    return value ?? '';
  };
  const requested = single('store'), selected = single('selected'), cursor = single('cursor');
  const step = single('step') || 'select';
  const ids = selected ? selected.split(',') : [];
  if ((requested && !canonicalUUID.test(requested)) || ids.length > 100 ||
    ids.some(id => !canonicalUUID.test(id)) || new Set(ids).size !== ids.length ||
    (cursor && !canonicalCursor.test(cursor)) || !['select', 'review', 'preview', 'done'].includes(step)) notFound();
  let store: Store | null = null;
  let error: OrderReadCode | null = null;
  const token = authConfig ? exactCookieHeader((await headers()).get('cookie'), SESSION_COOKIE) : null;
  if (!token || !isBase64URL32(token)) error = 'signed-out';
  else {
    const listed = await authenticatedStores(token);
    if (!listed.stores) {
      const safe = await safeError(listed.response);
      error = safe.status === 401 ? 'signed-out' : safe.status === 403 ? 'forbidden' : 'unavailable';
    } else {
      store = requested ? listed.stores.find(s => s.id === requested) ?? null :
        [...listed.stores].sort((a, b) => a.id.localeCompare(b.id))[0] ?? null;
      if (requested && !store) notFound();
    }
  }
  return <Suspense fallback={<AdminLoading />}>
    <ShippingCenter key={`${store?.id ?? ''}:${locale}`} locale={locale} store={store}
      initialIDs={ids} initialStep={step as ShippingStep} initialCursor={cursor} initialError={error} />
  </Suspense>;
}
