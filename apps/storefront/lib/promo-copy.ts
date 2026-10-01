// Buyer-facing copy of the discount-code field (components/PromoCode.tsx) and of the refusals of the quote request and of BeginCheckout;
// locales zh-CN / zh-TW / en (contracts/storefront-v2.md §F). Owns every string of PromoCode.tsx plus the text for the closed code list in
// lib/promo-contract.ts. Non-goals: no rule and no amount (the server's quote is the only price; this file only words its answers).
import type { Locale } from "@live-commerce/i18n";
import type { PromoErrorCode } from "./promo-contract.ts";

type Copy = {
  label: string;
  placeholder: string;
  apply: string;
  applying: string;
  remove: string;
  applied: (code: string, off: string) => string;
  hint: string;
  failed: string;
  errors: Record<PromoErrorCode, string>;
  atCheckout: string;
};

const en: Copy = {
  label: "Discount code",
  placeholder: "Enter a code",
  apply: "Apply",
  applying: "Applying…",
  remove: "Remove code",
  applied: (code, off) => `Code ${code} applied: ${off} off.`,
  hint: "A code takes money off the goods, not shipping. One code per order.",
  failed: "The code could not be checked right now. Try again.",
  errors: {
    promo_invalid: "This code is not valid.",
    promo_not_started: "This code is not active yet.",
    promo_expired: "This code has expired.",
    promo_min_subtotal: "Your order is below the minimum amount for this code.",
    promo_used_up: "This code has been fully used.",
    promo_buyer_limit: "You have already used this code the maximum number of times.",
    promo_changed: "This code changed while you were checking out. Remove it or apply it again.",
  },
  atCheckout: "The order was not placed. Remove the code or apply it again above, then place the order.",
};

const zhCN: Copy = {
  label: "优惠码",
  placeholder: "输入优惠码",
  apply: "使用",
  applying: "正在使用…",
  remove: "移除优惠码",
  applied: (code, off) => `已使用优惠码 ${code}：减 ${off}。`,
  hint: "优惠码只减商品金额，不减运费。每张订单限用一个。",
  failed: "暂时无法验证优惠码，请重试。",
  errors: {
    promo_invalid: "此优惠码无效。",
    promo_not_started: "此优惠码尚未生效。",
    promo_expired: "此优惠码已过期。",
    promo_min_subtotal: "订单金额未达此优惠码的最低消费。",
    promo_used_up: "此优惠码已被领完。",
    promo_buyer_limit: "您使用此优惠码的次数已达上限。",
    promo_changed: "结账期间此优惠码已变更，请移除后重新使用。",
  },
  atCheckout: "订单未成立。请在上方移除优惠码或重新使用，然后再下单。",
};

const zhTW: Copy = {
  label: "優惠碼",
  placeholder: "輸入優惠碼",
  apply: "使用",
  applying: "正在使用…",
  remove: "移除優惠碼",
  applied: (code, off) => `已使用優惠碼 ${code}：減 ${off}。`,
  hint: "優惠碼只減商品金額，不減運費。每張訂單限用一個。",
  failed: "暫時無法驗證優惠碼，請重試。",
  errors: {
    promo_invalid: "此優惠碼無效。",
    promo_not_started: "此優惠碼尚未生效。",
    promo_expired: "此優惠碼已過期。",
    promo_min_subtotal: "訂單金額未達此優惠碼的最低消費。",
    promo_used_up: "此優惠碼已被領完。",
    promo_buyer_limit: "您使用此優惠碼的次數已達上限。",
    promo_changed: "結帳期間此優惠碼已變更，請移除後重新使用。",
  },
  atCheckout: "訂單未成立。請在上方移除優惠碼或重新使用，然後再下單。",
};

export const promoCopy: Record<Locale, Copy> = { en, "zh-CN": zhCN, "zh-TW": zhTW };
