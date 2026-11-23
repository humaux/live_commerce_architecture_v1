// Purpose: keep automatic shop navigation prefetch out of the buyer order-link handoff.
// Depends on: the locale layout segment and Next Link.
// Used by: ShopChrome and MobileMenu.
"use client";

import type { ComponentProps } from "react";
import Link from "next/link";
import { useSelectedLayoutSegment } from "next/navigation";

export default function ShopChromeLink(props: ComponentProps<typeof Link>) {
  const segment = useSelectedLayoutSegment();
  // The exchange leaves this document as soon as the buyer cookie is set, cancelling in-flight prefetches.
  return <Link {...props} prefetch={segment === "order-link" ? false : props.prefetch} />;
}
