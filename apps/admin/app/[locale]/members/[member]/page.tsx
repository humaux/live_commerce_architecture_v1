// Purpose: resolve member UUID and authorized store before rendering read-only detail.
// Depends on: existing scoped page loader and canonical UUID guard.
// Used by: /[locale]/members/[member]; query contains only store UUID.
import { Suspense } from "react";
import { notFound } from "next/navigation";
import AdminLoading from "@/components/AdminLoading";
import { MemberDetail } from "@/components/members/MemberDetail";
import { canonicalUUID } from "@/lib/orders-model";
import { loadPage } from "../../customers/page-data";
export default async function MemberPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string; member: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { member } = await params;
  if (!canonicalUUID.test(member)) notFound();
  const page = await loadPage(params, searchParams, ["store"]);
  return (
    <Suspense fallback={<AdminLoading />}>
      <MemberDetail {...page} member={member} />
    </Suspense>
  );
}
