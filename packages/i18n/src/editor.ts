// Purpose: Seller copy for the simplified store editor, review and publication states.
// Depends on: supported Locale type in index.ts.
// Used by: admin Design component family through design-copy.ts.
// Invariants: every visible label has zh-TW, zh-CN and en; merchant content and server facts are never translated.
import type { Locale } from './index.ts';

const en = {
  title: 'Edit store', home: 'Home sections', profile: 'Store details', pages: 'Pages', nav: 'Menus',
  domains: 'Domains', back: 'Back', cancel: 'Cancel changes', more: 'More actions',
  intro: 'Logo & intro', contact: 'Contact', social: 'Social', details: 'View details',
  scope: 'Publishes store details, menus, home and every page', unchanged: 'Prices, stock and domains stay unchanged',
  immediate: 'Buyers see this version immediately', confirm: 'Confirm publication', continue: 'Continue editing',
  published: 'Published', failed: 'Publication did not complete', pending: 'Publication awaiting confirmation',
  query: 'Check publication status', view: 'View published content', visit: 'Visit store',
  empty: 'page content is empty', platform: 'Store address', custom: 'Custom domain',
  active: 'Ready', waiting: 'Verification in progress', unavailable: 'Address unavailable', settings: 'Manage domains',
  available: 'This address is ready to use', noAddress: 'No address is ready yet. Check domain settings.',
  versions: 'Publication history', summary: 'Store content', newPage: 'New page',
};
type EditorCopy = typeof en;
export const editorCopy: Record<Locale, EditorCopy> = {
  en,
  'zh-TW': {
    title: '編輯網店', home: '首頁區塊', profile: '店鋪資料', pages: '資訊頁', nav: '導覽選單',
    domains: '網店網址', back: '返回', cancel: '取消變更', more: '更多操作',
    intro: '圖片與介紹', contact: '聯絡資料', social: '社群連結', details: '查看內容',
    scope: '發布店鋪資料、導覽、首頁及全部資訊頁', unchanged: '不更改價格、庫存與網址',
    immediate: '發布後買家會立即看到', confirm: '確認發布', continue: '繼續編輯',
    published: '發布成功', failed: '這次未能發布', pending: '發布結果待確認',
    query: '查詢發布狀態', view: '查看已發布內容', visit: '查看網店',
    empty: '頁面內容留白', platform: '系統網址', custom: '自有域名',
    active: '可使用', waiting: '等待驗證', unavailable: '網址暫不可用', settings: '管理網址',
    available: '目前網址可使用', noAddress: '尚無可用網址，請查看網址設定。',
    versions: '發布記錄', summary: '網店內容', newPage: '新增資訊頁',
  },
  'zh-CN': {
    title: '编辑网店', home: '首页区块', profile: '店铺资料', pages: '信息页', nav: '导航菜单',
    domains: '网店网址', back: '返回', cancel: '取消更改', more: '更多操作',
    intro: '图片和介绍', contact: '联系资料', social: '社交链接', details: '查看内容',
    scope: '发布店铺资料、导航、首页和全部信息页', unchanged: '不修改价格、库存和网址',
    immediate: '发布后买家会立即看到', confirm: '确认发布', continue: '继续编辑',
    published: '发布成功', failed: '本次发布失败', pending: '发布结果待确认',
    query: '查询发布状态', view: '查看已发布内容', visit: '查看网店',
    empty: '页面内容留空', platform: '系统网址', custom: '自有域名',
    active: '可使用', waiting: '等待验证', unavailable: '网址暂不可用', settings: '管理网址',
    available: '当前网址可使用', noAddress: '暂无可用网址，请查看网址设置。',
    versions: '发布记录', summary: '网店内容', newPage: '添加信息页',
  },
};
