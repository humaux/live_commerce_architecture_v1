// Purpose: authorized merchant public CVS-directory lookup route; no buyer data or writes.
// Depends on: customers page-data session/store loader, WorkspaceFrame and CvsLookup.
// Used by: Next app router at /[locale]/orders/cvs-match.
// Invariants: only locale/store query is accepted; directory selection grants no shipping eligibility.
import { CvsLookup } from '@/components/CvsLookup';
import { WorkspaceFrame } from '@/components/WorkspaceFrame';
import { loadPage } from '../../customers/page-data';

export default async function CvsMatchPage({ params, searchParams }: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ['store']);
  return <WorkspaceFrame locale={page.locale} storeName={page.store?.name ?? ''} active="orders"><CvsLookup locale={page.locale} /></WorkspaceFrame>;
}
