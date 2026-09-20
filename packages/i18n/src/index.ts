export const locales = ['zh-CN', 'zh-TW', 'en'] as const;
export type Locale = (typeof locales)[number];

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
