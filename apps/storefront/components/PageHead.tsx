// Title block of the inner pages: breadcrumb trail + the page's single h1 + optional intro. Server component, no BFF/Go.
// Breadcrumb items are internal links already carrying the preview token (built by the caller with lib/design.ts withPreview).
import Link from "next/link";
import type { ReactNode } from "react";
import { ChevronRightIcon } from "./icons";

export default function PageHead({
  crumbs,
  label,
  title,
  heading = true,
  children,
}: {
  crumbs: { href: string; label: string }[];
  label: string;
  title: string;
  // false: only the trail (the product page puts its h1 next to the price instead).
  heading?: boolean;
  children?: ReactNode;
}) {
  return (
    <header className="sf-pagehead">
      {crumbs.length > 0 && (
        <nav aria-label={label} className="sf-crumbs">
          <ol>
            {crumbs.map((c) => (
              <li key={c.href}>
                <Link href={c.href}>{c.label}</Link>
                <ChevronRightIcon />
              </li>
            ))}
            <li aria-current="page">{title}</li>
          </ol>
        </nav>
      )}
      {heading && <h1>{title}</h1>}
      {children}
    </header>
  );
}
