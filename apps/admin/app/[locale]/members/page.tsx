// Purpose: resolve authorized store/locale for the private member list.
// Depends on: existing customers page loader; MemberList owns browser reads.
// Used by: native /[locale]/members route; only store UUID is allowed in query.
import { Suspense } from "react";
import AdminLoading from "@/components/AdminLoading";
import { MemberList } from "@/components/members/MemberList";
import { loadPage } from "../customers/page-data";
export default async function MembersPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return (
    <Suspense fallback={<AdminLoading />}>
      <MemberList {...page} />
    </Suspense>
  );
}
