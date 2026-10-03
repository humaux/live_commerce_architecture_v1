import type { PlatformLocale, PlatformPage } from "./company";

type Copy = {
  pages: Record<PlatformPage, string>;
  login: string;
  signup: string;
  language: string;
  skip: string;
  title: [string, string];
  intro: string;
  features: [string, string][];
  flowTitle: string;
  disclaimer: string;
  steps: [string, string][];
  example: string;
  comment: string;
  product: string;
  selected: string;
  order: string;
  pending: string;
  company: string;
  cr: string;
  br: string;
  incorporated: string;
  validity: string;
  email: string;
};
export const platformCopy: Record<PlatformLocale, Copy> = {
  "zh-TW": {
    pages: {
      home: "首頁",
      privacy: "隱私政策",
      terms: "服務條款",
      "data-deletion": "資料刪除",
      contact: "聯絡我們",
    },
    login: "登入",
    signup: "建立商家帳戶",
    language: "語言",
    skip: "跳至主要內容",
    title: ["把直播留言，", "接到你的生意。"],
    intro: "Facebook 留言收單、商家網店與訂單管理，在同一個工作流程。",
    features: [
      ["留言連結購物流程", "整理已授權專頁的留言，讓買家確認商品與數量。"],
      ["自己的商家網店", "管理商品、庫存與價格，提供買家選購入口。"],
      ["訂單進度有紀錄", "集中查看訂單、付款與手工出貨狀態。"],
    ],
    flowTitle: "從留言，到一筆有紀錄的訂單",
    disclaimer:
      "產品示意，並非真實交易。各項功能須由商家完成帳戶連接、授權及設定；不代表每家商戶都已接通。",
    steps: [
      ["Facebook 留言", "買家在商家已連接的專頁留下商品留言。"],
      ["商家網店", "進入網店，確認款式、數量及結帳資訊。"],
      ["訂單管理", "商家查看訂單並跟進付款與出貨。"],
    ],
    example: "產品示意",
    comment: "+1 黑色 L",
    product: "經典棉質 T 恤",
    selected: "黑色 · L · 1 件",
    order: "示例訂單",
    pending: "待付款",
    company: "營運公司",
    cr: "公司編號",
    br: "商業登記證號碼",
    incorporated: "成立日期",
    validity: "商業登記有效期",
    email: "聯絡電郵",
  },
  "zh-CN": {
    pages: {
      home: "首页",
      privacy: "隐私政策",
      terms: "服务条款",
      "data-deletion": "资料删除",
      contact: "联系我们",
    },
    login: "登录",
    signup: "创建商家账号",
    language: "语言",
    skip: "跳至主要内容",
    title: ["把直播留言，", "接到你的生意。"],
    intro: "Facebook 留言收单、商家网店与订单管理，在同一个工作流程。",
    features: [
      ["留言连接购物流程", "整理已授权主页的留言，让买家确认商品与数量。"],
      ["自己的商家网店", "管理商品、库存与价格，提供买家选购入口。"],
      ["订单进度有记录", "集中查看订单、付款与手工发货状态。"],
    ],
    flowTitle: "从留言，到一笔有记录的订单",
    disclaimer:
      "产品示意，并非真实交易。各项功能须由商家完成账户连接、授权及设置；不代表每家商户都已接通。",
    steps: [
      ["Facebook 留言", "买家在商家已连接的主页留下商品留言。"],
      ["商家网店", "进入网店，确认款式、数量及结账信息。"],
      ["订单管理", "商家查看订单并跟进付款与发货。"],
    ],
    example: "产品示意",
    comment: "+1 黑色 L",
    product: "经典棉质 T 恤",
    selected: "黑色 · L · 1 件",
    order: "示例订单",
    pending: "待付款",
    company: "运营公司",
    cr: "公司编号",
    br: "商业登记证号码",
    incorporated: "成立日期",
    validity: "商业登记有效期",
    email: "联系邮箱",
  },
  en: {
    pages: {
      home: "Home",
      privacy: "Privacy policy",
      terms: "Terms of service",
      "data-deletion": "Data deletion",
      contact: "Contact",
    },
    login: "Sign in",
    signup: "Create merchant account",
    language: "Language",
    skip: "Skip to content",
    title: ["From live comments", "to your next order."],
    intro:
      "Facebook comment ordering, your storefront and order management — in one workflow.",
    features: [
      [
        "Comments meet commerce",
        "Organise authorised Page comments so buyers can confirm items and quantities.",
      ],
      [
        "Your own storefront",
        "Manage products, stock and prices with a place for buyers to shop.",
      ],
      [
        "An order trail you can follow",
        "Review orders, payments and manual shipment status together.",
      ],
    ],
    flowTitle: "From a comment to a recorded order",
    disclaimer:
      "Product illustration, not a real transaction. Features require each merchant’s account connections, permissions and setup; this does not mean every merchant is already connected.",
    steps: [
      [
        "Facebook comment",
        "A buyer leaves a product comment on a connected merchant Page.",
      ],
      [
        "Merchant storefront",
        "The buyer confirms options, quantity and checkout information.",
      ],
      [
        "Order management",
        "The merchant reviews the order and follows up payment and shipping.",
      ],
    ],
    example: "Product illustration",
    comment: "+1 Black L",
    product: "Classic cotton T-shirt",
    selected: "Black · L · 1 item",
    order: "Example order",
    pending: "Unpaid",
    company: "Operator",
    cr: "Company number",
    br: "Business registration number",
    incorporated: "Incorporation date",
    validity: "Business registration validity",
    email: "Contact email",
  },
};
