// Purpose: Seller copy for the simplified store editor, review and publication states.
// Depends on: supported Locale type in index.ts.
// Used by: admin Design component family through design-copy.ts.
// Invariants: every visible label has zh-TW, zh-CN and en; merchant content and server facts are never translated.
import type { Locale } from './index.ts';

const en = {
  subtitle: 'Saves first', pendingHelp: 'Check the result before publishing again',
  previewLabel: 'Draft {n} preview · Valid for 15 minutes', previewLive: 'Unpublished · Buyers see version {n}',
  title: 'Edit store', home: 'Home sections', profile: 'Store details', pages: 'Pages', nav: 'Menus',
  domains: 'Domains', back: 'Back', cancel: 'Cancel changes', more: 'More actions',
  intro: 'Logo & intro', contact: 'Contact', social: 'Social', details: 'View details',
  scope: 'Publishes store details, menus, home and every page', unchanged: 'Prices, stock and domains stay unchanged',
  immediate: 'Buyers see the new content immediately', confirm: 'Publish draft {n}', continue: 'Continue editing',
  published: 'Published as version {n}', failed: 'Publication did not complete', pending: 'Publication awaiting confirmation',
  query: 'Check publication status', view: 'View published content', visit: 'Visit store',
  contentUnavailable: "This publication's content is unavailable. Visit your store to check it.",
  empty: 'page content is empty', platform: 'Store address', custom: 'Custom domain',
  active: 'Ready', waiting: 'Verification in progress', unavailable: 'Address unavailable', settings: 'Manage domains',
  available: 'Current address is active', noAddress: 'No address is ready yet. Check domain settings.',
  versions: 'Publication history', summary: 'Store content', newPage: 'New page',
};
type EditorCopy = typeof en;
export const editorCopy: Record<Locale, EditorCopy> = {
  en,
  'zh-TW': {
    subtitle: '預覽與發布前會先儲存', pendingHelp: '先查詢結果，請勿再次發布',
    previewLabel: '草稿 {n} 預覽 · 15 分鐘有效', previewLive: '尚未發布 · 買家看到版本 {n}',
    title: '編輯網店', home: '首頁區塊', profile: '店鋪資料', pages: '資訊頁', nav: '導覽選單',
    domains: '網店網址', back: '返回', cancel: '取消變更', more: '更多操作',
    intro: '圖片與介紹', contact: '聯絡資料', social: '社群連結', details: '查看內容',
    scope: '發布店鋪資料、導覽、首頁及全部資訊頁', unchanged: '不更改價格、庫存與網址',
    immediate: '發布後，買家立即看到新內容', confirm: '確認發布草稿 {n}', continue: '繼續編輯',
    published: '已發布為版本 {n}', failed: '這次未能發布', pending: '發布結果待確認',
    query: '查詢發布狀態', view: '查看已發布內容', visit: '查看網店',
    contentUnavailable: '無法顯示這次發布的內容，請前往網店查看。',
    empty: '頁面內容留白', platform: '系統網址', custom: '自有域名',
    active: '可使用', waiting: '等待驗證', unavailable: '網址暫不可用', settings: '管理網址',
    available: '目前網址可使用', noAddress: '尚無可用網址，請查看網址設定。',
    versions: '發布記錄', summary: '網店內容', newPage: '新增資訊頁',
  },
  'zh-CN': {
    subtitle: '预览和发布前会先保存', pendingHelp: '请先查询结果，勿重复发布',
    previewLabel: '草稿 {n} 预览 · 15 分钟有效', previewLive: '尚未发布 · 买家看到版本 {n}',
    title: '编辑网店', home: '首页区块', profile: '店铺资料', pages: '信息页', nav: '导航菜单',
    domains: '网店网址', back: '返回', cancel: '取消更改', more: '更多操作',
    intro: '图片和介绍', contact: '联系资料', social: '社交链接', details: '查看内容',
    scope: '发布店铺资料、导航、首页和全部信息页', unchanged: '不修改价格、库存和网址',
    immediate: '发布后，买家立即看到新内容', confirm: '确认发布草稿 {n}', continue: '继续编辑',
    published: '已发布为版本 {n}', failed: '本次发布失败', pending: '发布结果待确认',
    query: '查询发布状态', view: '查看已发布内容', visit: '查看网店',
    contentUnavailable: '无法显示本次发布的内容，请前往网店查看。',
    empty: '页面内容留空', platform: '系统网址', custom: '自有域名',
    active: '可使用', waiting: '等待验证', unavailable: '网址暂不可用', settings: '管理网址',
    available: '当前网址可用', noAddress: '暂无可用网址，请查看网址设置。',
    versions: '发布记录', summary: '网店内容', newPage: '添加信息页',
  },
};
