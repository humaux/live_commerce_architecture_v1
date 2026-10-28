// Purpose: Shared neutral admin forbidden display from the approved Figma state.
// Depends on: @live-commerce/i18n locale and @live-commerce/ui/ds PermissionState/ButtonLink; no role or permission mapping.
// Used by: Customers and MerchantOrders after their existing server-authorized reads return forbidden.
// Invariants: display only; never names a required role, changes ACL or reveals resource authorization details.
import type { Locale } from "@live-commerce/i18n";
import { ButtonLink, PermissionState } from "@live-commerce/ui/ds";
import "./admin-state-block.css";

const copy: Record<Locale, { title: string; body: string; home: string }> = {
  en: {
    title: "You can’t use this feature",
    body: "Your current role can’t use this feature. Contact the store owner.",
    home: "Back to dashboard",
  },
  "zh-CN": {
    title: "你目前无法使用这个功能",
    body: "你目前的角色无法使用此功能，请联系店铺所有者。",
    home: "返回仪表盘",
  },
  "zh-TW": {
    title: "你目前無法使用這個功能",
    body: "你目前的角色無法使用此功能，請聯絡店鋪擁有者。",
    home: "返回儀表板",
  },
};

/** The read result already established denial; this component adds no authorization decision. */
export function AdminForbidden({ locale }: { locale: Locale }) {
  const c = copy[locale];
  return (
    <div className="admin-state-block">
      <PermissionState
        mode="full"
        title={c.title}
        reason={c.body}
        action={<ButtonLink href={`/${locale}`} variant="secondary">{c.home}</ButtonLink>}
      />
    </div>
  );
}
