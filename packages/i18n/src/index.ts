// Purpose: Supported locales, shared seller messages and validated locale-aware URL helpers.
// Depends on: state-recovery, people, catalog, editor, shipping and signed inbox copy; no runtime, network or storage dependency.
// Used by: admin/storefront routes, shared UI callers and locale tests.
// Invariants: only supported locales and validated local paths can enter generated URLs.
export const locales = ['zh-CN', 'zh-TW', 'en'] as const;
export { shippingCopy } from './shipping.ts';
export { stateRecoveryCopy } from './state-recovery.ts';
export { cvsPickerCopy } from './cvs-picker.ts';
export { editorCopy } from './editor.ts';
export type Locale = (typeof locales)[number];
export { inboxMessages, type InboxMessages } from './inbox.ts';

// Signed ux-settings polish/simplify copy. Values and currency are supplied by the server.
export const settingsV2Copy = {
  'zh-TW': {
    title: '店鋪設定', edit: '編輯設定', view: '檢視設定', complete: '店舖資料 · 已填寫', incomplete: '店舖資料 · 未填寫',
    delivery: '配送方式 · 已開啟 {n} 種', payment: '收款 · Stripe（平台代收）', start: '開始三步設定',
    step1: '1 / 3 · 店名', step2: '2 / 3 · 配送', step3: '3 / 3 · 第一個商品', name: '店名', email: '信箱', phone: '電話（選填）',
    selectAll: '全選', home: '宅配', seven: '7-ELEVEN', family: '全家', hilife: '萊爾富', ok: 'OK', on: '開放', off: '關閉',
    shipping: '自行出貨；寄出後登記承運商與單號。', shippingReview: '自行出貨；登記承運商與單號，不自動建單或購買面單。',
    paymentReview: 'Stripe 收款待確認；付款時間依買家方式，不含貨到付款。',
    checkout: '配送以下單確認為準。', publicURL: '商店網址', unavailable: '未填寫',
    prerequisite: '請在編輯設定確認市場與配送費用。', review: '核對設定', save: '儲存設定',
    back: '返回總覽，保留草稿', backEdit: '返回修改', nextDelivery: '下一步：配送', nextProduct: '下一步：商品',
    backName: '上一步：店名', backDelivery: '上一步：配送', skip: '略過', product: '商品名稱',
    price: '售價（{currency}）', stock: '庫存（件）', draft: '商品儲存為草稿，尚未上架。', productDraft: '商品草稿',
    saved: '設定已儲存', eligibility: '收款資格仍待確認。', failed: '設定未儲存', retained: '原設定不變，草稿已保留。',
    partial: '部分設定已儲存；請重新讀取後核對。', uncertain: '尚未取得最新狀態', reloadHelp: '請重新讀取；原設定保留。',
    reload: '重新讀取', advanced: '進階', loading: '正在讀取設定…', session: '登入狀態已變更，請重新登入。',
    pending: '上次儲存尚待確認；請核對並儲存原草稿，或重新讀取最新狀態。',
  },
  'zh-CN': {
    title: '店铺设置', edit: '编辑设置', view: '查看设置', complete: '店铺资料 · 已填写', incomplete: '店铺资料 · 未填写',
    delivery: '配送方式 · 已启用 {n} 种', payment: '收款 · Stripe（平台代收）', start: '开始三步设置',
    step1: '1 / 3 · 店名', step2: '2 / 3 · 配送', step3: '3 / 3 · 首件商品', name: '店名', email: '邮箱', phone: '电话（选填）',
    selectAll: '全选', home: '送货上门', seven: '7-ELEVEN', family: '全家', hilife: '莱尔富', ok: 'OK', on: '已开启', off: '已关闭',
    shipping: '自行发货；寄出后登记承运商和运单号。', shippingReview: '自行发货；登记承运商和运单号，系统不自动建单或购买面单。',
    paymentReview: 'Stripe 收款资格待确认；付款时间取决于买家选择的方式，不含货到付款。',
    checkout: '能否配送以下单时确认为准。', publicURL: '店铺网址', unavailable: '未填写',
    prerequisite: '请在编辑设置中核对市场和配送费用。', review: '核对设置', save: '保存设置',
    back: '返回概览，保留草稿', backEdit: '返回修改', nextDelivery: '下一步：配送', nextProduct: '下一步：商品',
    backName: '上一步：店名', backDelivery: '上一步：配送', skip: '跳过', product: '商品名称',
    price: '售价（{currency}）', stock: '库存（件）', draft: '商品将保存为草稿，尚未上架。', productDraft: '商品草稿',
    saved: '设置已保存', eligibility: '收款资格仍待确认。', failed: '设置未保存', retained: '原设置未变，草稿已保留。',
    partial: '部分设置已保存；请重新加载并核对。', uncertain: '暂时无法获取最新状态', reloadHelp: '请重新加载；原设置保留。',
    reload: '重新加载', advanced: '进阶', loading: '正在加载设置…', session: '登录状态已更改，请重新登录。',
    pending: '上次保存仍待确认；请核对并保存原草稿，或重新加载最新状态。',
  },
  en: {
    title: 'Store settings', edit: 'Edit settings', view: 'View settings', complete: 'Store details · Complete', incomplete: 'Store details · Incomplete',
    delivery: 'Delivery methods · {n} enabled', payment: 'Payments · Stripe (collected by platform)', start: 'Start quick setup',
    step1: '1 / 3 · Store name', step2: '2 / 3 · Delivery', step3: '3 / 3 · First product', name: 'Store name', email: 'Email', phone: 'Phone (optional)',
    selectAll: 'Select all', home: 'Home delivery', seven: '7-ELEVEN', family: 'FamilyMart', hilife: 'Hi-Life', ok: 'OK', on: 'on', off: 'off',
    shipping: 'Arrange shipping; record the carrier and tracking number after dispatch.',
    shippingReview: 'Arrange shipping and record the carrier and tracking number. No automatic shipment creation or label purchase.',
    paymentReview: 'Stripe payment eligibility pending; payment timing depends on the buyer’s method. No cash on delivery.',
    checkout: 'Delivery availability is confirmed at checkout.', publicURL: 'Store URL', unavailable: 'Not provided',
    prerequisite: 'Check the market and delivery pricing in Edit settings.', review: 'Review settings', save: 'Save settings',
    back: 'Back to overview, keep draft', backEdit: 'Back to edit', nextDelivery: 'Next: delivery', nextProduct: 'Next: product',
    backName: 'Back: store name', backDelivery: 'Back: delivery', skip: 'Skip', product: 'Product name',
    price: 'Price ({currency})', stock: 'Stock (units)', draft: 'Product is saved as a draft, not published.', productDraft: 'Product draft',
    saved: 'Settings saved', eligibility: 'Payment eligibility is still pending.', failed: 'Settings not saved', retained: 'Saved settings are unchanged; your draft is retained.',
    partial: 'Some settings were saved. Reload and review them.', uncertain: 'Latest status unavailable', reloadHelp: 'Reload; your saved settings are retained.',
    reload: 'Reload', advanced: 'Advanced', loading: 'Loading settings…', session: 'Your session changed. Sign in again.',
    pending: 'Your last save still needs confirmation. Review and save the original draft, or reload the latest status.',
  },
} satisfies Record<Locale, Record<string, string>>;

export const localeNames: Record<Locale, string> = {
  'zh-CN': '简体中文',
  'zh-TW': '繁體中文',
  en: 'English',
};

export type Messages = {
  appName: string; products: string; inventory: string; warehouses: string; language: string;
  createProduct: string; name: string; description: string; sku: string; price: string; currency: string;
  available: string; reserved: string; onHand: string; allocated: string; unavailable: string;
  save: string; cancel: string; loading: string; emptyProducts: string; emptyInventory: string;
  failedRequest: string; sessionRequired: string; retry: string; archived: string; active: string;
  productCreated: string; adjustInventory: string; quantity: string; reason: string;
  expectedVersion: string; conflict: string; permissionDenied: string; notConnected: string;
  localEnvironment: string; storefront: string; signIn: string; signOut: string;
};

export type MessageKey = keyof Messages;

export const messages: Record<Locale, Messages> = {
  'zh-CN': {
    appName: '直播电商', products: '商品', inventory: '库存', warehouses: '仓库', language: '语言',
    createProduct: '创建商品', name: '名称', description: '描述', sku: 'SKU', price: '价格', currency: '货币',
    available: '可用', reserved: '已预留', onHand: '现有库存', allocated: '已分配', unavailable: '不可用',
    save: '保存', cancel: '取消', loading: '加载中', emptyProducts: '暂无商品', emptyInventory: '暂无库存',
    failedRequest: '请求失败', sessionRequired: '需要登录', retry: '重试', archived: '已归档', active: '启用',
    productCreated: '商品已创建', adjustInventory: '调整库存', quantity: '数量', reason: '原因',
    expectedVersion: '预期版本', conflict: '发生冲突', permissionDenied: '无权限', notConnected: '未连接',
    localEnvironment: '本地环境', storefront: '店铺前台', signIn: '登录', signOut: '退出登录',
  },
  'zh-TW': {
    appName: '直播電商', products: '商品', inventory: '庫存', warehouses: '倉庫', language: '語言',
    createProduct: '建立商品', name: '名稱', description: '描述', sku: 'SKU', price: '價格', currency: '貨幣',
    available: '可用', reserved: '已預留', onHand: '現有庫存', allocated: '已分配', unavailable: '不可用',
    save: '儲存', cancel: '取消', loading: '載入中', emptyProducts: '目前沒有商品', emptyInventory: '目前沒有庫存',
    failedRequest: '請求失敗', sessionRequired: '需要登入', retry: '重試', archived: '已封存', active: '啟用',
    productCreated: '商品已建立', adjustInventory: '調整庫存', quantity: '數量', reason: '原因',
    expectedVersion: '預期版本', conflict: '發生衝突', permissionDenied: '沒有權限', notConnected: '未連線',
    localEnvironment: '本機環境', storefront: '店面前台', signIn: '登入', signOut: '登出',
  },
  en: {
    appName: 'Live Commerce', products: 'Products', inventory: 'Inventory', warehouses: 'Warehouses', language: 'Language',
    createProduct: 'Create product', name: 'Name', description: 'Description', sku: 'SKU', price: 'Price', currency: 'Currency',
    available: 'Available', reserved: 'Reserved', onHand: 'On hand', allocated: 'Allocated', unavailable: 'Unavailable',
    save: 'Save', cancel: 'Cancel', loading: 'Loading', emptyProducts: 'No products yet', emptyInventory: 'No inventory yet',
    failedRequest: 'Request failed', sessionRequired: 'Sign-in required', retry: 'Retry', archived: 'Archived', active: 'Active',
    productCreated: 'Product created', adjustInventory: 'Adjust inventory', quantity: 'Quantity', reason: 'Reason',
    expectedVersion: 'Expected version', conflict: 'Conflict', permissionDenied: 'Permission denied', notConnected: 'Not connected',
    localEnvironment: 'Local environment', storefront: 'Storefront', signIn: 'Sign in', signOut: 'Sign out',
  },
};

export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (locales as readonly string[]).includes(value);
}

function firstPathSegment(pathname: string): string | undefined {
  if (!pathname.startsWith('/') || pathname.startsWith('//') || /[\\\u0000-\u001f\u007f]/.test(pathname)) return undefined;
  const cut = pathname.search(/[?#]/);
  const pathOnly = cut < 0 ? pathname : pathname.slice(0, cut);
  const segment = pathOnly.slice(1).split('/')[0];
  return segment || undefined;
}

export function localeFromPath(pathname: string): Locale | undefined {
  const segment = firstPathSegment(pathname);
  return isLocale(segment) ? segment : undefined;
}

function qualityRanges(header: string): Array<{ range: string; q: number; order: number }> {
  if (header.length > 4096) return [];
  return header.split(',').slice(0, 32).map((part, order) => {
    const [rawRange, ...params] = part.trim().split(';');
    const range = rawRange.trim().toLowerCase();
    if (!range) return { range, q: -1, order };
    if (!/^(?:\*|[a-z]{2,8}(?:-[a-z0-9]{1,8})*)$/i.test(range)) return { range, q: -1, order };
    const qParams = params.filter((p) => /^\s*q\s*=/i.test(p));
    if (qParams.length > 1) return { range, q: -1, order };
    // Consume the complete parameter: split('=')[1] accidentally accepts q=0.5=0.
    const qMatch = qParams[0]?.match(/^\s*q\s*=\s*(0(?:\.\d{0,3})?|1(?:\.0{0,3})?)\s*$/i);
    if (qParams.length && !qMatch) return { range, q: -1, order };
    return { range, q: qMatch ? Number(qMatch[1]) : 1, order };
  }).filter((item) => item.q > 0).sort((a, b) => b.q - a.q || a.order - b.order);
}

function languageToLocale(range: string): Locale | undefined {
  if (range === 'en' || range.startsWith('en-')) return 'en';
  const parts = range.split('-');
  if (parts[0] !== 'zh') return undefined;
  const subtags = parts.slice(1);
  const script = subtags.find((part) => /^(?:hans|hant)$/i.test(part));
  const region = subtags.find((part) => /^(?:[a-z]{2}|\d{3})$/i.test(part));
  if (script === 'hant') return 'zh-TW';
  if (script === 'hans') return 'zh-CN';
  if (['tw', 'hk', 'mo'].includes(region ?? '')) return 'zh-TW';
  if (['cn', 'sg'].includes(region ?? '') || parts.length === 1) return 'zh-CN';
  return undefined;
}

export function resolveLocale(input: { pathname: string; preference?: string; acceptLanguage?: string; defaultLocale: Locale }): Locale {
  const explicit = localeFromPath(input.pathname);
  if (explicit) return explicit;
  if (isLocale(input.preference)) return input.preference;
  if (input.acceptLanguage) {
    for (const item of qualityRanges(input.acceptLanguage)) {
      const locale = languageToLocale(item.range);
      if (locale) return locale;
    }
  }
  return input.defaultLocale;
}

function validatePath(path: string): { pathname: string; suffix: string } {
  if (path.length > 4096 || !path.startsWith('/') || path.startsWith('//') || /^[a-z][a-z\d+.-]*:/i.test(path)) throw new RangeError('Invalid localized path');
  if (/[\\\u0000-\u001f\u007f]/.test(path) || /%(?![0-9a-f]{2})/i.test(path)) throw new RangeError('Invalid localized path');
  const cut = path.search(/[?#]/);
  const pathname = cut < 0 ? path : path.slice(0, cut);
  const suffix = cut < 0 ? '' : path.slice(cut);
  if (/%2f|%5c/i.test(pathname)) throw new RangeError('Invalid localized path');
  let decoded = pathname;
  for (let i = 0; i < 4; i += 1) {
    let next: string;
    try {
      next = decodeURIComponent(decoded);
    } catch {
      throw new RangeError('Invalid localized path');
    }
    if (/%2f|%5c/i.test(next)) throw new RangeError('Invalid localized path');
    if (next === decoded) break;
    decoded = next;
  }
  if (/%[0-9a-f]{2}/i.test(decoded) || /[\\\u0000-\u001f\u007f]/.test(decoded) || decoded.split('/').some((part) => part === '.' || part === '..')) throw new RangeError('Invalid localized path');
  return { pathname, suffix };
}

export function localizedPath(locale: Locale, path: string): string {
  if (!isLocale(locale)) throw new RangeError('Invalid locale');
  const { pathname, suffix } = validatePath(path);
  if (pathname === '/') return `/${locale}${suffix}`;
  const segments = pathname.split('/');
  if (isLocale(segments[1])) segments[1] = locale;
  else segments.splice(1, 0, locale);
  return segments.join('/') + suffix;
}

export function translate(locale: Locale, key: MessageKey): string {
  return messages[locale][key];
}

export { peopleCopy } from "./people.ts";

export { catalogTaskCopy, catalogText } from "./catalog.ts";
