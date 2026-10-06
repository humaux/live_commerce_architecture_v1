// Purpose: Renders localized route titles through the shared page header.
// Depends on: next/navigation, react, @live-commerce/i18n, @live-commerce/ui, @/src/page-title
// Used by: apps/admin/components/Customers.tsx, apps/admin/components/StudioClaims.tsx, apps/admin/components/MerchantOrders.tsx, apps/admin/components/Finance.tsx, apps/admin/components/Ledger.tsx, apps/admin/components/Billing.tsx, apps/admin/components/Promotions.tsx, apps/admin/components/CustomerDetail.tsx, apps/admin/components/SettingsWizard.tsx, apps/admin/components/Attribution.tsx, apps/admin/components/Studio.tsx, apps/admin/components/ProductImport.tsx, apps/admin/components/ManualOrder.tsx, apps/admin/components/Dashboard.tsx, apps/admin/components/Ads.tsx, apps/admin/components/Team.tsx, apps/admin/components/Design.tsx, apps/admin/components/ProductList.tsx, apps/admin/components/CollectionManager.tsx, apps/admin/components/ProductEditor.tsx
"use client";
import { usePathname } from "next/navigation";
import type { ComponentProps } from "react";
import type { Locale } from "@live-commerce/i18n";
import { PageHeader } from "@live-commerce/ui";
import { pageTitle } from "@/src/page-title";

/** Renders localized route titles through the shared page header. */
export function AdminPageHeader({
  locale,
  title,
  ...props
}: Omit<ComponentProps<typeof PageHeader>, "title"> & {
  locale: Locale;
  title?: string;
}) {
  const pathname = usePathname();
  return <PageHeader {...props} title={title ?? pageTitle(locale, pathname)} />;
}
