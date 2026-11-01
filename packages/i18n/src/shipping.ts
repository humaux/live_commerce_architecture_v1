// Purpose: Signed shipping-center and packing-slip seller copy in all supported locales.
// Depends on: Locale type only; current ux-orders DELIVERY copy table, 2026-10-29.
// Used by: apps/admin/components/shipping/ShippingCenter.tsx and ShippingShell.tsx.
// Invariants: order, recipient, store and delivery facts come from scoped server reads, never this copy table.
import type { Locale } from './index.ts';

const en = {
  center: 'Shipping center', backOrders: 'Back to orders', note: 'Printing does not buy a label or record shipment.',
  review: 'Review selected orders', back: 'Back to selection', edit: 'Edit selection', confirm: 'Confirm and preview',
  preview: 'Print preview', print: 'Print', done: 'Complete printing', view: 'View', rowView: 'View', cancel: 'Cancel',
  backPreview: 'Back to print preview', backCenter: 'Back to shipping center', recipient: 'Recipient', delivery: 'Delivery method',
  manual: 'Seller-arranged delivery', order: 'Order', empty: 'No orders selected',
  boundary: 'Printing does not prove carrier handover, record shipment or buy a label.',
  outcome: 'Canceling print keeps shipment status; printing does not prove handover.',
  mobile: 'Use desktop for batch printing.', ready: 'Ready to ship', needsReview: 'Review needed',
  selected: '{n} selected', source: 'Address and phone come from the order',
  address: 'Destination', phone: 'Phone', seven: '7-ELEVEN pickup', family: 'FamilyMart pickup', hilife: 'Hi-Life pickup', ok: 'OK Mart pickup',
  loading: 'Loading orders…', unavailable: 'Orders could not be loaded. Try again.', retry: 'Retry',
  signedOut: 'Your session has expired. Sign in again.', signIn: 'Sign in', forbidden: 'You do not have access to these orders.',
  noOrders: 'No orders to prepare.', selectionUnavailable: 'Some selected orders are no longer available. Review your selection.',
  firstPage: 'Back to first page', nextPage: 'Next page', printFailed: 'Printing could not open. Try again.',
  demo: 'Demo store', language: 'Language', localeShort: 'EN',
};
type ShippingCopy = { [K in keyof typeof en]: string };
export const shippingCopy: Record<Locale, ShippingCopy> = {
  'zh-TW': {
    center: '出貨中心', backOrders: '返回訂單', note: '列印不購面單、不登記出貨。',
    review: '核對選取訂單', back: '返回選取', edit: '修改選取', confirm: '確認並預覽',
    preview: '列印預覽', print: '列印', done: '完成列印', view: '檢視', rowView: '查看', cancel: '取消',
    backPreview: '返回列印預覽', backCenter: '返回出貨中心', recipient: '收件人', delivery: '配送方式',
    manual: '商家自行安排配送', order: '訂單', empty: '尚未選取訂單',
    boundary: '列印不證明交寄、不登記出貨、不購面單。', outcome: '取消列印不改出貨狀態；列印不證明交寄。',
    mobile: '批次列印請用桌面。', ready: '待出貨', needsReview: '需覆核', selected: '已選 {n} 筆', source: '地址與電話取自訂單',
    address: '配送目的地', phone: '電話', seven: '7-ELEVEN 取貨', family: '全家取貨', hilife: '萊爾富取貨', ok: 'OK 超商取貨',
    loading: '正在載入訂單…', unavailable: '暫時無法載入訂單，請重試。', retry: '重試',
    signedOut: '登入已到期，請重新登入。', signIn: '登入', forbidden: '你沒有查看這些訂單的權限。',
    noOrders: '目前沒有待處理訂單。', selectionUnavailable: '部分選取訂單已無法查看，請重新核對選取。',
    firstPage: '返回第一頁', nextPage: '下一頁', printFailed: '無法開啟列印，請重試。', demo: '示範店', language: '語言', localeShort: '繁中',
  },
  'zh-CN': {
    center: '发货中心', backOrders: '返回订单', note: '打印不购买面单，也不登记发货。',
    review: '核对所选订单', back: '返回选择订单', edit: '修改所选订单', confirm: '确认并预览',
    preview: '打印预览', print: '打印', done: '完成打印', view: '查看', rowView: '查看', cancel: '取消',
    backPreview: '返回打印预览', backCenter: '返回发货中心', recipient: '收件人', delivery: '配送方式',
    manual: '商家自行安排配送', order: '订单', empty: '尚未选择订单',
    boundary: '打印不代表物流已揽收，不登记发货，也不购买面单。', outcome: '取消打印不改变发货状态；打印不代表物流已揽收。',
    mobile: '批量打印请使用电脑。', ready: '待发货', needsReview: '需复核', selected: '已选 {n} 笔', source: '地址和电话取自订单',
    address: '配送目的地', phone: '电话', seven: '7-ELEVEN 取货', family: '全家取货', hilife: '莱尔富取货', ok: 'OK 便利店取货',
    loading: '正在加载订单…', unavailable: '暂时无法加载订单，请重试。', retry: '重试',
    signedOut: '登录已过期，请重新登录。', signIn: '登录', forbidden: '你没有查看这些订单的权限。',
    noOrders: '目前没有待处理订单。', selectionUnavailable: '部分所选订单已无法查看，请重新核对所选订单。',
    firstPage: '返回第一页', nextPage: '下一页', printFailed: '无法打开打印窗口，请重试。', demo: '示例店铺', language: '语言', localeShort: '简中',
  },
  en,
};
