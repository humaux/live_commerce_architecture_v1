// Purpose: signed manual-order, after-sales and public CVS-tool seller copy.
// Depends on: Locale type only; callers supply catalog, RMA and public directory facts.
// Used by: admin ManualOrder, ReturnsList and CvsLookup.
// Invariants: all three locales have the same keys; no invented amounts or provider status.
import type { Locale } from './index.ts';
export const orderOpsCopy = {
  'zh-TW': {
    manual: '建立訂單', items: '1 · 商品', recipient: '2 · 收件人與配送', add: '加入商品', picker: '加入商品至本單',
    apply: '套用至本單', cancel: '取消', select: '選取', name: '收件人', phone: '聯絡電話', review: '檢查訂單',
    retained: '取消不會變更原訂單。', selected: '本單保留', advanced: '進階', backEdit: '返回修改',
    quote: '金額以下單時的正式報價為準；目前尚未建立訂單或收款。',
    returns: '退款與售後', requests: '售後申請', view: '查看售後申請', completed: '已完成的售後紀錄',
    aftersales: '退款、退貨收件與庫存處置分開記錄。請到原訂單處理退款。',
    lookup: '查門市編號', query: '地址、門市名稱或編號', change: '更換', code: '複製編號', all: '複製全部',
    copied: '已複製編號', copiedAll: '已複製門市資料', copyFailed: '未能複製，請選取門市資料手動複製。', back: '返回',
    pickup: '取貨門市', choose: '選擇取貨門市', returnOrder: '返回訂單', family: '全家',
    missing: '此編號不在門市清單：', mismatch: '請先選擇相符的超商品牌。', empty: '輸入地址、店名或編號查詢。',
    noMatches: '找不到門市，請確認品牌及資料。', loading: '正在讀取門市清單…', unavailable: '門市清單暫時無法讀取。', retry: '重試',
    source: '資料：綠界門市清單 · 更新', brand: '超商品牌', other: '其他門市資料',
  },
  'zh-CN': {
    manual: '创建订单', items: '1 · 商品', recipient: '2 · 收件人与配送', add: '添加商品', picker: '添加商品到本单',
    apply: '应用到本单', cancel: '取消', select: '选择', name: '收件人', phone: '联系电话', review: '检查订单',
    retained: '取消不会更改原订单。', selected: '本单保留', advanced: '高级选项', backEdit: '返回修改',
    quote: '金额以下单时的正式报价为准；目前尚未创建订单或收款。',
    returns: '退款与售后', requests: '售后申请', view: '查看售后申请', completed: '已完成的售后记录',
    aftersales: '退款、退货签收与库存处理分别记录。请到原订单处理退款。',
    lookup: '查门店编号', query: '地址、门店名称或编号', change: '更换', code: '复制编号', all: '复制全部信息',
    copied: '编号已复制', copiedAll: '门店信息已复制', copyFailed: '复制失败，请选中门店信息后手动复制。', back: '返回',
    pickup: '自提门店', choose: '选择自提门店', returnOrder: '返回订单', family: '全家',
    missing: '该编号未收录在门店清单中：', mismatch: '请先选择对应的便利店品牌。', empty: '输入地址、店名或编号查询。',
    noMatches: '找不到门店，请核对品牌和信息。', loading: '正在读取门店清单…', unavailable: '暂时无法读取门店清单。', retry: '重试',
    source: '数据：绿界门店清单 · 更新', brand: '便利店品牌', other: '其他门店信息',
  },
  en: {
    manual: 'Create order', items: '1 · Items', recipient: '2 · Recipient and delivery', add: 'Add products', picker: 'Add products to this order',
    apply: 'Apply to this order', cancel: 'Cancel', select: 'Select', name: 'Recipient', phone: 'Phone', review: 'Review order',
    retained: 'Cancel keeps the original order unchanged.', selected: 'Kept in this order', advanced: 'Advanced', backEdit: 'Back to editing',
    quote: 'The final quote is confirmed when you place the order. No order or payment has been created yet.',
    returns: 'Refunds and after-sales', requests: 'After-sales requests', view: 'View after-sales requests', completed: 'Completed after-sales record',
    aftersales: 'Refunds, returned goods and stock decisions are recorded separately. Process refunds from the original order.',
    lookup: 'Find store code', query: 'Address, store name or code', change: 'Change', code: 'Copy code', all: 'Copy all details',
    copied: 'Code copied', copiedAll: 'Store details copied', copyFailed: 'Could not copy. Select the store details and copy them manually.', back: 'Back',
    pickup: 'Pickup store', choose: 'Choose pickup store', returnOrder: 'Return to order', family: 'FamilyMart',
    missing: 'Code not in store list: ', mismatch: 'Choose the matching convenience-store brand first.', empty: 'Enter an address, store name or code.',
    noMatches: 'No store found. Check the brand and details.', loading: 'Loading store directory…', unavailable: 'The store directory is unavailable.', retry: 'Retry',
    source: 'Source: ECPay store directory · Updated', brand: 'Store brand', other: 'Other store details',
  },
} satisfies Record<Locale, Record<string, string>>;
