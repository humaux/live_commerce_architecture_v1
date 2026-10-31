// Purpose: Signed orders list and single-order production copy in the three supported locales.
// Depends on: @live-commerce/i18n locale type; UX-orders DELIVERY polish 2026-10-29.
// Used by: MerchantOrders and OrderDetailView.
// Invariants: Seller words only; values and shipment facts always come from the scoped server.
import type { Locale } from "@live-commerce/i18n";
const en = {
  detail: "Order details", search: "Search", view: "View", ship: "Shipping center", back: "Back",
  carrier: "Choose carrier", tracking: "Tracking number", trackingHint: "Enter tracking number", record: "Record shipment",
  review: "Confirm shipment record", confirm: "Confirm", edit: "Edit", saved: "Shipment recorded",
  blocked: "Payment needs review; shipping is blocked", note: "No label purchase or proof of carrier handover",
  paid:"Paid", reviewState:"Review needed", deliveryUnchanged:"Delivery method and pickup store stay the same",handover:"Carrier receipt is unconfirmed",returnOrders:"Back to orders",
  advanced: "Advanced", facts: "Order information and actions", filters: "Advanced filters", other: "Other carrier name",
  carriers: {black_cat:"T-Cat",hsinchu:"HCT Logistics",chunghwa_post:"Chunghwa Post",sf_express:"SF Express",seven_eleven_cvs:"7-ELEVEN",familymart_cvs:"FamilyMart",hilife_cvs:"Hi-Life",okmart_cvs:"OK mart",other:"Other carrier"},
  tabs: {all:"All",unpaid:"Unpaid",transfer_review:"Review transfer",ready_to_ship:"Ready to ship",ready_to_consign:"Awaiting handover",shipped:"Shipped",completed:"Completed",cancelled:"Cancelled"},
  moreItems: (count: number) => `${count} items`,
};
export const parityCopy: Record<Locale, typeof en> = {
  en,
  "zh-TW": {
    detail:"訂單詳情",search:"搜尋",view:"查看",ship:"出貨中心",back:"返回",carrier:"選擇物流",tracking:"單號",trackingHint:"輸入單號",record:"登記出貨",review:"出貨確認",confirm:"確認",edit:"修改",saved:"出貨已登記",blocked:"付款待確認，暫不可出貨",note:"不買面單，不代表交寄",paid:"已付款",reviewState:"需覆核",deliveryUnchanged:"不改配送或門市",handover:"未證明物流收件",returnOrders:"返回訂單",advanced:"進階",facts:"訂單資料與其他操作",filters:"進階篩選",other:"其他承運商名稱",
    carriers:{black_cat:"黑貓宅急便",hsinchu:"新竹物流",chunghwa_post:"中華郵政",sf_express:"順豐速運",seven_eleven_cvs:"7-ELEVEN",familymart_cvs:"全家",hilife_cvs:"萊爾富",okmart_cvs:"OK mart",other:"其他承運商"},
    tabs:{all:"全部",unpaid:"待付款",transfer_review:"待核對轉帳",ready_to_ship:"待出貨",ready_to_consign:"待交寄",shipped:"已出貨",completed:"已完成",cancelled:"已取消"},moreItems:count=>`共 ${count} 件`,
  },
  "zh-CN": {
    detail:"订单详情",search:"搜索",view:"查看",ship:"发货中心",back:"返回",carrier:"选择物流公司",tracking:"物流单号",trackingHint:"输入物流单号",record:"登记发货",review:"确认发货记录",confirm:"确认",edit:"修改",saved:"已记录发货",blocked:"付款待核实，暂不能发货",note:"不购买面单，也不代表物流已揽收",paid:"已付款",reviewState:"需复核",deliveryUnchanged:"不更改配送方式或自提门店",handover:"物流揽收尚未确认",returnOrders:"返回订单",advanced:"高级选项",facts:"订单资料与其他操作",filters:"高级筛选",other:"其他物流公司名称",
    carriers:{black_cat:"黑猫宅急便",hsinchu:"新竹物流",chunghwa_post:"中华邮政",sf_express:"顺丰速运",seven_eleven_cvs:"7-ELEVEN",familymart_cvs:"全家",hilife_cvs:"莱尔富",okmart_cvs:"OK mart",other:"其他物流公司"},
    tabs:{all:"全部",unpaid:"待付款",transfer_review:"待核实转账",ready_to_ship:"待发货",ready_to_consign:"待交物流",shipped:"已发货",completed:"已完成",cancelled:"已取消"},moreItems:count=>`共 ${count} 件`,
  },
};
/** Display identity only: the complete server UUID remains the routing/command key. */
export const shortOrderNumber = (id: string) => `#${id.replaceAll("-", "").slice(-5).toUpperCase()}`;
