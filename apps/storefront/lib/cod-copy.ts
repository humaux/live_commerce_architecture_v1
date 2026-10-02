// Buyer-facing copy (zh-CN / zh-TW / en) for the cash-on-delivery payment mode (home-cod R5, migration 0107):
// the checkout choice (amount + surcharge) and the order page collection status. Rendered by components/OrderFlow.tsx and
// components/CodOrderStatus.tsx; no BFF route of its own. Rules encoded here: the order is never "paid" online — the buyer
// pays the carrier in cash on delivery; the surcharge is the fee shown at checkout and is never folded into the order total.
import type { Locale } from "@live-commerce/i18n";
import type { CollectionState } from "./cvs-contract";

const en = {
  due: "Due on delivery",
  includesFee: (fee: string) => `(includes ${fee} cash-on-delivery fee)`,
  recorded: "Original collection amount",
  homeOnly: "Cash on delivery · Home delivery only",
  capReached: "Cash on delivery is unavailable: the amount including the fee exceeds this store's limit.",
  wholeOnly: "Cash on delivery requires a whole-dollar NT$ total. Choose another payment method.",
  changed: "The delivery fee changed. Review the updated amount and confirm your address again before placing the order.",
  unavailable: "Cash on delivery is no longer available. Choose another payment method.",
  limit: "You have reached the limit for unpaid delivery orders. Contact the seller before placing another order.",
  manualCarrier: (carrier: string) => `Manual shipping · ${carrier}`,
  orderedCarrier: (carrier: string) => `Carrier at checkout · ${carrier} (manual shipping)`,
  carriers: { black_cat: "Black Cat", hsinchu: "Hsinchu" },
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
  due: "到货需付",
  includesFee: (fee) => `（含货到付款手续费 ${fee}）`,
  recorded: "原订单代收金额",
  homeOnly: "货到付款 · 仅限宅配",
  capReached: "含手续费的代收金额超过店铺上限，无法选择货到付款。",
  wholeOnly: "货到付款仅接受整数台币金额，请选择其他付款方式。",
  changed: "货到付款手续费已变更。请核对新金额，并重新确认收货地址后下单。",
  unavailable: "此店铺已暂停货到付款，请选择其他付款方式。",
  limit: "未完成的货到付款订单已达上限，请联系商家后再下单。",
  manualCarrier: (carrier) => `手工出货 · ${carrier}`,
  orderedCarrier: (carrier) => `下单时的物流商 · ${carrier}（手工出货）`,
  carriers: { black_cat: "黑猫", hsinchu: "新竹" },
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
  due: "到貨需付",
  includesFee: (fee) => `（含貨到付款手續費 ${fee}）`,
  recorded: "原訂單代收金額",
  homeOnly: "貨到付款 · 僅限宅配",
  capReached: "含手續費的代收金額超過商店上限，無法選擇貨到付款。",
  wholeOnly: "貨到付款僅接受整數台幣金額，請選擇其他付款方式。",
  changed: "貨到付款手續費已變更。請核對新金額，並重新確認收貨地址後下單。",
  unavailable: "此商店已暫停貨到付款，請選擇其他付款方式。",
  limit: "未完成的貨到付款訂單已達上限，請聯絡商家後再下單。",
  manualCarrier: (carrier) => `手工出貨 · ${carrier}`,
  orderedCarrier: (carrier) => `下單時的物流商 · ${carrier}（手工出貨）`,
  carriers: { black_cat: "黑貓", hsinchu: "新竹" },
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
