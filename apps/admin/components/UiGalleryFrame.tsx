// @ds-page: built only from packages/ui DS components; design-spec rules R1–R7 apply (docs/design/DESIGN-SPEC.md §6).
// Purpose: Shared gallery-only frame; every template uses the same shell and working topbar.
// Depends on: React, DS, localized gallery copy/navigation; synthetic demo data only.
// Used by: UiGallery and UiGalleryTemplates, behind LC_UI_GALLERY=1; no live-page adoption.
"use client";
import { useMemo, type ReactNode } from "react";
import { localeNames, locales, type Locale } from "@live-commerce/i18n";
import { AppShellV2, Breadcrumb, Button, Icon, MenuButton } from "@live-commerce/ui/ds";
import { galleryNav } from "../src/ui-gallery-nav";
import { galleryCopy } from "../src/ui-gallery-copy";
import { templateCopy, type TemplateName } from "../src/ui-gallery-template-copy";
import s from "./UiGallery.module.css";
export function UiGalleryFrame({ locale, currentId, template, children }: { locale: Locale; currentId?: string; template?: TemplateName; children: ReactNode }) {
  const c = galleryCopy[locale];
  const nav = useMemo(() => galleryNav(locale), [locale]);
  const root = `/${locale}/ui-gallery`;
  const t = templateCopy[locale];
  const groupId = template === "settings" ? "settings" : template === "form" ? "catalog" : "orders";
  const group = nav.find(item => item.id === groupId)!;
  const crumbs = template ? [
    { label: group.label, href: template === "list" || template === "detail" ? `${root}/templates/list` : `${root}?nav=${template === "form" ? "products" : "store-profile"}` },
    ...(template === "detail" ? [{ label: t.list, href: `${root}/templates/list` }] : []),
    { label: t[template] },
  ] : [{ label: c.crumbHome, href: "#sec-tokens" }, { label: c.crumbHere }];
  return <AppShellV2 nav={nav} currentId={currentId ?? "products"} persistenceKey="ds2-gallery-nav" navLabel={c.navLabel} brand={c.brand}
    store={{ name: c.storeName, region: c.storeRegion, logo: <Icon name="store" size={24} /> }} menuLabel={c.menu} closeLabel={c.close} skipLabel={c.skip}
    breadcrumb={<Breadcrumb label={c.crumbLabel} crumbs={crumbs} />}
    topbar={{
      locale: <nav aria-label={c.localeLabel} data-figma="1397:105153"><MenuButton label={localeNames[locale]} caret={false} items={locales.map(l => ({ id: l, label: localeNames[l], href: `/${l}/ui-gallery${template ? `/templates/${template}` : ""}` }))} /></nav>,
      help: <Button iconOnly variant="ghost" onClick={() => { window.location.href = `${root}#sec-alerts`; }}><Icon name="help" size={16} /><span className={s.sr}>{c.help}</span></Button>,
      notifications: <MenuButton icon="bell" variant="ghost" label={c.notifications} items={[{ id: "unread", label: c.kpiUnread, onSelect: () => { window.location.href = `${root}#sec-kpi`; } }]} />,
      account: <MenuButton icon="user" variant="ghost" label={c.account} items={[{ id: "profile", label: c.setItemProfile, onSelect: () => { window.location.href = `${root}?setting=profile#sec-settings`; } }]} />,
      mobileAccount: <MenuButton icon="user" variant="ghost" label={c.account} items={[
        { id: "profile", label: c.setItemProfile, href: `${root}?setting=profile#sec-settings` },
        ...locales.map(l => ({ id: l, label: localeNames[l], href: `/${l}/ui-gallery${template ? `/templates/${template}` : ""}` })),
        { id: "help", label: c.help, href: `${root}#sec-alerts` },
        { id: "store", label: c.enterStore, href: `${root}#sec-tokens`, target: "_blank" as const },
      ]} />,
      enterStore: { label: c.enterStore, href: `${root}#sec-tokens` },
    }}>
    {children}
  </AppShellV2>;
}
