// Buyer-facing copy (zh-CN / zh-TW / en) for the cash-on-delivery payment mode (home-cod R5, migration 0107):
// the checkout choice (amount + surcharge) and the order page collection status. Rendered by components/OrderFlow.tsx and
// components/CodOrderStatus.tsx; no BFF route of its own. Rules encoded here: the order is never "paid" online — the buyer
// pays the carrier in cash on delivery; the surcharge is the fee shown at checkout and is never folded into the order total.
import type { Locale } from "@live-commerce/i18n";
import type { CollectionState } from "./cvs-contract";

const en = {
  // checkout choice: {total} is the order total, {surcharge} the whole-TWD fee (null = none).
  codLabel: (total: string, surcharge: string | null) =>
    surcharge ? `Cash on delivery ${total} + ${surcharge} fee` : `Cash on delivery ${total}`,
  createCod: "Place order (cash on delivery)",
  codNote: "Pay the carrier in cash when the parcel arrives. Nothing is charged online.",
  // order page
  orderTitle: "Cash on delivery",
  orderStates: {
    PENDING: "Pay the carrier when the parcel arrives",
    COLLECTED: "Paid on delivery",
    RETURNED: "Not collected — returned",
    REFUNDED_OFFLINE: "Refunded by the seller outside this site",
    CANCELLED: "The seller canceled this order",
    RESTOCKED: "Not collected — returned to the seller",
  } as Record<CollectionState, string>,
  orderNote: "Pay the carrier in cash when the parcel arrives. The seller records the result; refunds are made outside this site.",
};

const zhCN: typeof en = {
  codLabel: (total, surcharge) =>
    surcharge ? `货到付款 ${total} + ${surcharge} 手续费` : `货到付款 ${total}`,
  createCod: "提交订单（货到付款）",
  codNote: "包裹送达时向货运公司支付现金，不会在线扣款。",
  orderTitle: "货到付款",
  orderStates: {
    PENDING: "包裹送达时向货运公司付款",
    COLLECTED: "已货到收款",
    RETURNED: "未取货 — 已退回",
    REFUNDED_OFFLINE: "商家已在本站之外退款",
    CANCELLED: "商家已取消此订单",
    RESTOCKED: "未取货 — 已退回商家",
  },
  orderNote: "包裹送达时向货运公司支付现金。商家记录收款结果；退款在本站之外进行。",
};

const zhTW: typeof en = {
  codLabel: (total, surcharge) =>
    surcharge ? `貨到付款 ${total} + ${surcharge} 手續費` : `貨到付款 ${total}`,
  createCod: "送出訂單（貨到付款）",
  codNote: "包裹送達時向貨運公司支付現金，不會線上扣款。",
  orderTitle: "貨到付款",
  orderStates: {
    PENDING: "包裹送達時向貨運公司付款",
    COLLECTED: "已貨到收款",
    RETURNED: "未取貨 — 已退回",
    REFUNDED_OFFLINE: "商家已在本站之外退款",
    CANCELLED: "商家已取消此訂單",
    RESTOCKED: "未取貨 — 已退回商家",
  },
  orderNote: "包裹送達時向貨運公司支付現金。商家記錄收款結果；退款在本站之外進行。",
};

export const codCopy: Record<Locale, typeof en> = { en, "zh-CN": zhCN, "zh-TW": zhTW };
