// Purpose: Three-locale signed recovery copy for admin and buyer StatePanel/InlineError boundaries.
// Depends on: Locale from index; no diagnostics or resource identifiers enter displayed copy.
// Used by: UiGalleryRecovery, shared admin boundaries and storefront state branches.
// Invariants: uncertainty offers a read of the same operation, never another submission.
import type { Locale } from "./index";

const en = {
  title: "Recovery actions", empty: "No products yet", emptyBody: "Add your first product.",
  forbidden: "This page cannot be opened", forbiddenBody: "Return to your store to continue.",
  forbiddenNote: "Ask the store owner if you need access.",
  offline: "The list could not be loaded", offlineBody: "Check your connection and try loading it again.",
  expired: "This link is no longer available", expiredBody: "Return to your store to continue.",
  unknown: "The result is not confirmed", unknownBody: "Check the same operation before trying again.",
  pending: "Waiting for confirmation", checked: "Status checked. Still waiting for confirmation.",
  failed: "The latest status could not be loaded", loaded: "The list has been loaded.",
  retry: "Reload", check: "Check status", back: "Return to store",
};
type RecoveryCopy = typeof en;
export const stateRecoveryCopy: Record<Locale, RecoveryCopy> = {
  en,
  "zh-TW": {
    title: "恢復操作", empty: "還沒有商品", emptyBody: "先新增第一件商品。",
    forbidden: "目前無法開啟此頁", forbiddenBody: "請返回店鋪繼續操作。",
    forbiddenNote: "如需存取，請洽店鋪擁有者。",
    offline: "暫時無法載入列表", offlineBody: "請檢查網路連線，再重新載入。",
    expired: "此連結已無法使用", expiredBody: "請返回店鋪繼續操作。",
    unknown: "操作結果尚未確認", unknownBody: "請先查詢同一筆操作的狀態，確認前請勿再次送出。",
    pending: "等待確認", checked: "已查詢狀態，仍在等待確認。",
    failed: "暫時無法載入最新狀態", loaded: "已重新載入列表。",
    retry: "重新載入", check: "查詢狀態", back: "返回店鋪",
  },
  "zh-CN": {
    title: "恢复操作", empty: "还没有商品", emptyBody: "先添加第一件商品。",
    forbidden: "暂时无法打开此页面", forbiddenBody: "请返回店铺继续操作。",
    forbiddenNote: "如需访问，请联系店铺所有者。",
    offline: "暂时无法加载列表", offlineBody: "请检查网络连接，再重新加载。",
    expired: "此链接已无法使用", expiredBody: "请返回店铺继续操作。",
    unknown: "操作结果尚未确认", unknownBody: "请先查询同一笔操作的状态，确认前请勿再次提交。",
    pending: "等待确认", checked: "已查询状态，仍在等待确认。",
    failed: "暂时无法加载最新状态", loaded: "已重新加载列表。",
    retry: "重新加载", check: "查询状态", back: "返回店铺",
  },
};

// Signed ux-states recovery panels; these are product copy, never diagnostic text.
export const adminSignedStateCopy = {
    "zh-TW": { title: "目前無法開啟此頁", body: "此連結無法開啟，或目前帳號沒有此操作權限。請返回可用的功能。", note: "若需這項功能，請向店鋪擁有者確認權限。", home: "返回工作台", retry: "重試", expired: "登入已過期，請重新登入後繼續。", signIn: "重新登入" },
    "zh-CN": { title: "暂时无法打开此页面", body: "此链接无法打开，或当前账号没有操作权限。请返回可用的功能。", note: "如需使用此功能，请向店铺所有者确认权限。", home: "返回工作台", retry: "重试", expired: "登录已过期，请重新登录后继续。", signIn: "重新登录" },
    en: { title: "This page cannot be opened", body: "This link cannot be opened, or your account cannot use this feature. Return to an available feature.", note: "Ask the store owner if you need access to this feature.", home: "Back to workspace", retry: "Try again", expired: "Your session has expired. Sign in again to continue.", signIn: "Sign in again" },
  };
export const buyerSignedStateCopy = {
    "zh-TW": {
      empty: "購物車還沒有商品", emptyBody: "先挑選想買的商品，再回到購物車確認數量與配送方式。", emptyNote: "選好商品後，可在購物車確認數量。", browse: "瀏覽商品",
      forbidden: "目前無法開啟此頁", forbiddenBody: "此連結無法開啟，或目前購物連線無法使用這項功能。請從商店重新開始。", forbiddenNote: "如需查看訂單，請使用商店的訂單查詢。", home: "返回商店",
      offline: "暫時無法載入購物車", offlineBody: "請檢查網路連線，再重新載入。這次只讀取購物車，不會送出訂單。", offlineNote: "商品與數量以原購物連線重新讀取的結果為準。", retry: "重新載入", failed: "目前仍無法連線。請保留原頁，稍後重新載入。",
      expired: "無法開啟這個連結", expiredBody: "請從訂單查詢重新開始，並準備訂單編號與電子郵件。", expiredNote: "連結不適用時，請勿再次送出原訂單。", lookup: "前往訂單查詢",
      unknown: "付款結果尚未確認", unknownBody: "請先查詢這筆訂單的付款狀態。確認前請勿再次付款。", check: "查詢狀態", unknownFailed: "目前無法讀取最新狀態。請稍後查詢同一筆訂單。",
      lookupSubmit: "查詢", lookupFailed: "暫時無法查詢，請稍後再試。", lookupNoMatch: "找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。",
    },
    "zh-CN": {
      empty: "购物车还没有商品", emptyBody: "先挑选想买的商品，再回到购物车确认数量与配送方式。", emptyNote: "选好商品后，可在购物车确认数量。", browse: "浏览商品",
      forbidden: "暂时无法打开此页面", forbiddenBody: "此链接无法打开，或当前购物会话无法使用此功能。请从商店重新开始。", forbiddenNote: "如需查看订单，请使用商店的订单查询。", home: "返回商店",
      offline: "暂时无法加载购物车", offlineBody: "请检查网络连接，再重新加载。本次只读取购物车，不会提交订单。", offlineNote: "商品与数量以当前购物会话重新读取的结果为准。", retry: "重新加载", failed: "暂时仍无法连接。请保留当前页面，稍后重新加载。",
      expired: "无法打开此链接", expiredBody: "请从订单查询重新开始，并准备订单编号与邮箱。", expiredNote: "链接不可用时，请勿重新提交原订单。", lookup: "前往订单查询",
      unknown: "付款结果尚未确认", unknownBody: "请先查询这笔订单的付款状态。确认前请勿再次付款。", check: "查询状态", unknownFailed: "暂时无法读取最新状态。请稍后查询同一笔订单。",
      lookupSubmit: "查询", lookupFailed: "暂时无法查询，请稍后再试。", lookupNoMatch: "找不到匹配的订单，请确认订单编号与邮箱或手机号码。",
    },
    en: {
      empty: "Your cart is empty", emptyBody: "Choose products first, then return to your cart to confirm quantities and delivery.", emptyNote: "You can check quantities in your cart after choosing products.", browse: "Browse products",
      forbidden: "This page cannot be opened", forbiddenBody: "This link cannot be opened, or your shopping session cannot use this feature. Start again from the store.", forbiddenNote: "Use the store’s order lookup to view an order.", home: "Return to store",
      offline: "Your cart could not be loaded", offlineBody: "Check your connection and reload. This only reads your cart and does not place an order.", offlineNote: "Products and quantities come from re-reading your original shopping session.", retry: "Reload", failed: "We still cannot connect. Keep this page open and reload later.",
      expired: "This link cannot be opened", expiredBody: "Start again from order lookup, with your order number and email ready.", expiredNote: "Do not place the original order again if the link cannot be used.", lookup: "Go to order lookup",
      unknown: "Your payment result is not confirmed", unknownBody: "Check this order’s payment status first. Do not pay again until it is confirmed.", check: "Check status", unknownFailed: "The latest status could not be loaded. Check the same order again later.",
      lookupSubmit: "Find order", lookupFailed: "Lookup is unavailable right now. Please try again later.", lookupNoMatch: "We could not find a matching order. Check the order number and the email or phone number.",
    },
  };
