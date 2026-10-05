"use client";
import { usePathname } from "next/navigation";
import type { ComponentProps } from "react";
import type { Locale } from "@live-commerce/i18n";
import { PageHeader } from "@live-commerce/ui";
import { pageTitle } from "@/src/page-title";

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
