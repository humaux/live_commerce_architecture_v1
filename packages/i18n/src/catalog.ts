// Purpose: Seller-facing copy for the signed catalog task screens in all supported locales.
// Depends on: the owner-signed catalog simplify inventory (2026-10-28).
// Used by: admin product list, simplified editor and compact media controls.
// Invariants: locale never changes market/currency; interpolate only server facts.
const en = {
  operations: 'Product tasks', copyEntry: 'Copy product', copyTitle: 'Copy as a new draft?', copyKeep: 'Keep variants and prices. Product codes are generated again.', copyExclude: 'Images, keywords and stock are not copied.', copyConfirm: 'Confirm copy', copyCancel: 'Cancel', copySaved: 'Draft created', copyFacts: 'Draft · New product code · No images yet', copyEdit: 'Edit new draft',
  title: 'Products', editorTitle: 'Edit product', edit: 'Edit', all: 'All', active: 'Active', draft: 'Draft', archived: 'Archived',
  empty: 'No archived products', select: 'Select {n}', selected: '{n} selected', archive: 'Archive products', archiveTitle: 'Archive {n} products',
  archivePolicy: 'Archiving stops new orders. Existing orders remain.', confirmArchive: 'Confirm archive', backSelection: 'Back to selection',
  result: '{saved} archived, {failed} unchanged', live: 'Archive after live orders close.', view: 'View products',
  name: 'Product name', desc: 'Description', price: 'Both prices · {currency}', onePrice: 'Price · {currency}', multiPrice: '{n} prices · {currency}',
  variants: 'Variants and stock', stock: 'Adjust stock', total: 'New on-hand quantity', delta: 'Quantity change',
  checkStock: 'Check stock', keep: 'Keep current stock', backVariants: 'Back to variants', backEditor: 'Back to product editor',
  review: 'Review changes', reviewTitle: 'Review changes', confirm: 'Confirm save', continueEdit: 'Continue editing', saved: 'Product saved',
  mediaNone: 'Product images · None', media: 'Product images', choose: 'Choose images', chooseVariant: 'Choose {value} image',
  main: 'Main {n}/4 · Required to publish', detail: 'Detail {n}/20', variantImages: 'Variant images',
  stockError: 'Not saved: {target} is below {committed} committed. No changes submitted.',
  cancel: 'Cancel changes', soldOut: 'Sold out', available: 'Available {n}', onHand: 'On hand {n}', committed: 'Committed {n}',
  advanced: 'Advanced', standard: 'Back to product editor', reason: 'Reason: Admin edit', tracked: 'Track stock',
  imagesSummary: 'Main {main}/4 · Variant {sku}/{variants} · Detail {detail}/20',
  uploadHelp: 'JPG, PNG, WebP; max 2 MB each; detail height up to six times width.',
  stockFloor: 'Count at least {n} units.',
  noPriceChange: 'Prices differ; edit each variant.', unchangedOrders: 'Existing orders keep their price.',
};
type Copy = { [K in keyof typeof en]: string };
const tw: Copy = {
  operations:'商品作業',copyEntry:'複製商品',copyTitle:'複製為新草稿？',copyKeep:'保留規格及售價；商品編號重新產生。',copyExclude:'不複製圖片、關鍵字及庫存。',copyConfirm:'確認複製',copyCancel:'取消',copySaved:'草稿已建立',copyFacts:'草稿 · 新商品編號 · 尚無圖片',copyEdit:'編輯新草稿',
  title:'商品',editorTitle:'編輯商品',edit:'編輯',all:'全部',active:'在售',draft:'草稿',archived:'已歸檔',empty:'尚無已歸檔商品',select:'選取 {n} 件',selected:'已選 {n} 件',archive:'歸檔商品',archiveTitle:'確認歸檔 {n} 件',archivePolicy:'歸檔後停止接單；舊訂單保留。',confirmArchive:'確認歸檔',backSelection:'返回選取',result:'{saved} 件已歸檔，{failed} 件未變更',live:'直播接單中，結束後再歸檔。',view:'查看商品列表',name:'商品名稱',desc:'商品說明',price:'兩款售價 · {currency}',onePrice:'售價 · {currency}',multiPrice:'{n} 款售價 · {currency}',variants:'規格與庫存',stock:'調整庫存',total:'調整後現有',delta:'調整數量',checkStock:'核對庫存',keep:'保留目前庫存',backVariants:'返回規格',backEditor:'返回商品編輯',review:'檢查變更',reviewTitle:'核對變更',confirm:'確認儲存',continueEdit:'繼續編輯',saved:'商品已儲存',mediaNone:'商品圖片 · 未加入',media:'商品圖片',choose:'選擇圖片',chooseVariant:'選擇{value}圖片',main:'主圖 {n}/4 · 上架必填',detail:'詳情圖 {n}/20',variantImages:'規格圖',stockError:'未儲存：{target} 件少於占用的 {committed} 件；全部變更未送出。',cancel:'取消變更',soldOut:'售罄',available:'可售 {n}',onHand:'現有 {n}',committed:'已占用 {n}',advanced:'進階',standard:'返回商品編輯',reason:'調整原因：後台編輯',tracked:'追蹤庫存',imagesSummary:'主圖 {main}/4 · 規格圖 {sku}/{variants} · 詳情圖 {detail}/20',uploadHelp:'JPG、PNG、WebP；每張最多 2 MB；詳情圖高度最多寬度六倍。',stockFloor:'盤點總量至少 {n} 件。',noPriceChange:'各規格售價不同，請分別編輯。',unchangedOrders:'舊訂單售價不變。',
};
const cn: Copy = {
  operations:'商品操作',copyEntry:'复制商品',copyTitle:'复制为新草稿？',copyKeep:'保留规格和售价；重新生成商品编号。',copyExclude:'不复制图片、关键词和库存。',copyConfirm:'确认复制',copyCancel:'取消',copySaved:'草稿已创建',copyFacts:'草稿 · 新商品编号 · 暂无图片',copyEdit:'编辑新草稿',
  title:'商品',editorTitle:'编辑商品',edit:'编辑',all:'全部',active:'在售',draft:'草稿',archived:'已归档',empty:'暂无已归档商品',select:'选择 {n} 件',selected:'已选 {n} 件',archive:'归档商品',archiveTitle:'确认归档 {n} 件',archivePolicy:'归档后停止接单；保留原订单。',confirmArchive:'确认归档',backSelection:'返回选择',result:'{saved} 件已归档，{failed} 件未更改',live:'直播接单中，结束后再归档。',view:'查看商品列表',name:'商品名称',desc:'商品说明',price:'两款售价 · {currency}',onePrice:'售价 · {currency}',multiPrice:'{n} 款售价 · {currency}',variants:'规格与库存',stock:'调整库存',total:'调整后现有',delta:'调整数量',checkStock:'核对库存',keep:'保留当前库存',backVariants:'返回规格',backEditor:'返回商品编辑',review:'检查更改',reviewTitle:'核对更改',confirm:'确认保存',continueEdit:'继续编辑',saved:'商品已保存',mediaNone:'商品图片 · 未添加',media:'商品图片',choose:'选择图片',chooseVariant:'选择{value}图片',main:'主图 {n}/4 · 上架必填',detail:'详情图 {n}/20',variantImages:'规格图',stockError:'未保存：{target} 件少于占用的 {committed} 件；全部更改未提交。',cancel:'取消更改',soldOut:'售罄',available:'可售 {n}',onHand:'现有 {n}',committed:'已占用 {n}',advanced:'进阶',standard:'返回商品编辑',reason:'调整原因：后台编辑',tracked:'跟踪库存',imagesSummary:'主图 {main}/4 · 规格图 {sku}/{variants} · 详情图 {detail}/20',uploadHelp:'JPG、PNG、WebP；每张最多 2 MB；详情图高度不超过宽度六倍。',stockFloor:'盘点总量至少 {n} 件。',noPriceChange:'各规格售价不同，请分别编辑。',unchangedOrders:'原订单价格不变。',
};
export const catalogTaskCopy = { 'zh-TW': tw, 'zh-CN': cn, en } as const;
/** Fill a localized template with values, without HTML interpretation. */
export const catalogText = (template: string, values: Record<string, string | number>) =>
  template.replace(/\{(\w+)\}/g, (match, key: string) => String(values[key] ?? match));
