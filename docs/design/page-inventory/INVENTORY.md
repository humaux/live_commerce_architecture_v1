# 页面功能清单（现状）

来源：7 个 Antigravity 只读会话逐页读代码，每条功能带 file:line；脚本核对 51/51 页覆盖、0 条无效位置；integrator 抽查 6/6 属实。
证据级 E1（读代码），未在浏览器实跑。生成：2026-10-15，基线 r3/integration 92d73661。

## 商品与库存

### P01 `/[locale]/collections` — Manage catalog collections, metadata, cover images, and ordered product memberships.
文件：`apps/admin/app/[locale]/collections/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Select collection | view | Select collection from list to load and edit its details. | GET /api/stores/{store}/collections/{id} | `apps/admin/components/CollectionManager.tsx:141` |
| Create new collection trigger | action | Switch editor to new collection creation mode. | none | `apps/admin/components/CollectionManager.tsx:107` |
| Create collection | form | Create collection with title, optional slug, description, sort mode, and status. | POST /api/stores/{store}/collections | `apps/admin/components/CollectionManager.tsx:290` |
| Update collection metadata | form | Submit changed collection metadata with expected_version. | PATCH /api/stores/{store}/collections/{id} | `apps/admin/components/CollectionManager.tsx:305` |
| Delete collection | action | Delete current collection after confirmation with expected_version. | POST /api/stores/{store}/collections/{id}/delete | `apps/admin/components/CollectionManager.tsx:436` |
| Upload collection image | upload | Upload JPEG, PNG, or WebP cover image up to 2MB. | POST /api/stores/{store}/collections/{id}/image | `apps/admin/components/CollectionManager.tsx:504` |
| Remove collection image | action | Delete current collection cover image. | POST /api/stores/{store}/collections/{id}/image/delete | `apps/admin/components/CollectionManager.tsx:521` |
| Search member products | filter | Debounced search for products to add to current collection. | GET /api/stores/{store}/catalog-products | `apps/admin/components/CollectionManager.tsx:594` |
| Add product to collection | action | Append selected product to collection member list up to limit. | none | `apps/admin/components/CollectionManager.tsx:674` |
| Move member product up | action | Move product one position up in ordered member list. | none | `apps/admin/components/CollectionManager.tsx:628` |
| Move member product down | action | Move product one position down in ordered member list. | none | `apps/admin/components/CollectionManager.tsx:634` |
| Remove member product | action | Remove product from staged collection member list. | none | `apps/admin/components/CollectionManager.tsx:641` |
| Save collection members | action | Save complete ordered product ID array with expected_version. | PUT /api/stores/{store}/collections/{id}/products | `apps/admin/components/CollectionManager.tsx:727` |
| Retry failed write | action | Retry uncertain or failed collection mutation command. | none | `apps/admin/components/CollectionManager.tsx:384` |

现存问题：
- **P1** readCollections hardcodes limit=100 and ignores pagination cursor; stores exceeding 100 collections cannot view or select remaining collections. (`apps/admin/lib/catalog-v2-client.ts:30`)
- **P2** readProducts search in Members picker hardcodes limit=25 with no pagination, capping candidate pool to first 25 matching products. (`apps/admin/components/CollectionManager.tsx:594`)
- **P2** Save button silently no-ops when form fields have not changed, giving user no visual feedback. (`apps/admin/components/CollectionManager.tsx:304`)

### P02 `/[locale]/inventory` — Inspect multi-warehouse SKU stock ledger, adjust warehouse inventory levels with audit reasons, and test storefront purchase links.
文件：`apps/admin/app/[locale]/inventory/page.tsx` · 角色：owner, admin, live_operator, fulfilment, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Search inventory | filter | Filter inventory ledger by text search query matching product name or SKU code. | GET /api/stores/{store}/catalog-ledger | `apps/admin/components/Ledger.tsx:487` |
| Select warehouse filter | filter | Filter inventory ledger by selected warehouse ID. | GET /api/stores/{store}/catalog-ledger | `apps/admin/components/Ledger.tsx:512` |
| Load more warehouses | action | Fetch next page of warehouse options using cursor. | GET /api/stores/{store}/warehouses | `apps/admin/components/Ledger.tsx:404` |
| Filter by status | filter | Filter inventory rows by all, active, or archived status. | GET /api/stores/{store}/catalog-ledger | `apps/admin/components/Ledger.tsx:541` |
| Reset ledger filters | filter | Clear search query and status filters back to defaults. | none | `apps/admin/components/Ledger.tsx:551` |
| Refresh ledger | action | Trigger server-side revalidation of current inventory query. | none | `apps/admin/components/Ledger.tsx:562` |
| Paginate ledger rows | navigation | Navigate to next page of ledger rows using keyset cursor or reset to beginning. | GET /api/stores/{store}/catalog-ledger | `apps/admin/components/Ledger.tsx:586` |
| Inspect SKU | view | Select SKU row to open inspector tray and fetch storefront purchase entry. | GET /api/stores/{store}/products/{id}/purchase-entry | `apps/admin/components/Ledger.tsx:577` |
| Close inspector | action | Close selected SKU inspector tray and purchase entry card. | none | `apps/admin/components/Ledger.tsx:610` |
| Submit stock adjustment | form | Submit journaled inventory adjustment with delta quantity, reason, and balance_version. | POST /api/stores/{store}/inventory/adjustments | `apps/admin/components/Ledger.tsx:378` |
| Retry pending journal command | action | Re-submit uncertain command from local journal storage. | POST /api/stores/{store}/inventory/adjustments | `apps/admin/components/Ledger.tsx:481` |
| Refresh purchase entry | action | Re-read storefront purchase entry URL and readiness configuration state. | GET /api/stores/{store}/products/{id}/purchase-entry | `apps/admin/components/Ledger.tsx:735` |
| Copy purchase link | action | Fetch fresh purchase entry and copy validated storefront URL to clipboard. | GET /api/stores/{store}/products/{id}/purchase-entry | `apps/admin/components/Ledger.tsx:744` |
| Open purchase page | navigation | Fetch fresh purchase entry and open storefront product URL in browser. | GET /api/stores/{store}/products/{id}/purchase-entry | `apps/admin/components/Ledger.tsx:751` |
| Navigate to edit product | navigation | Navigate to full product editor for the inspected SKU's product. | none | `apps/admin/components/Ledger.tsx:639` |
| Navigate to create product | navigation | Navigate to product creation form from header action. | none | `apps/admin/components/Ledger.tsx:443` |

现存问题：
- **P2** Section bar displays section switch label but section state cannot be toggled from inventory in WorkspaceFrame, rendering 'products' branch dead code. (`apps/admin/components/Ledger.tsx:456`)
- **P2** Adjustment fieldset disables completely when inspected SKU is archived without displaying an explicit disabled explanation. (`apps/admin/components/Ledger.tsx:661`)

### P03 `/[locale]/products/[product]` — Full catalog product authoring and editing form covering metadata, single/multi-variant option matrix, media uploads, collections, shipping, and SEO.
文件：`apps/admin/app/[locale]/products/[product]/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Navigate document section | navigation | Focus specific product document section using tab strip. | none | `apps/admin/components/ProductDocumentForm.tsx:276` |
| Toggle mobile readiness | action | Expand or collapse required/recommended completion readiness checklist on narrow screens. | none | `apps/admin/components/ProductDocumentForm.tsx:297` |
| Stage create media | upload | Stage local image files for upload during product document creation. | none | `apps/admin/components/ProductDocumentMedia.tsx:130` |
| Upload product image | upload | Upload image file directly to existing product via multipart POST. | POST /api/stores/{store}/products/{id}/images | `apps/admin/components/ProductMediaManager.tsx:71` |
| Reorder product images | action | Update sequence order of main or detail product images. | POST /api/stores/{store}/products/{id}/images/order | `apps/admin/components/ProductMediaManager.tsx:136` |
| Move image role | action | Change image classification between main and detail role. | POST /api/stores/{store}/products/{id}/images/{image}/move | `apps/admin/components/ProductMediaManager.tsx:164` |
| Delete product image | action | Delete committed image from product. | POST /api/stores/{store}/products/{id}/images/{image}/delete | `apps/admin/components/ProductMediaManager.tsx:100` |
| Assign option image | action | Bind uploaded image to specific variant option value. | POST /api/stores/{store}/products/{id}/option-images | `apps/admin/components/ProductMediaManager.tsx:189` |
| Set image option axis | action | Configure which variant option axis determines variant photo switching. | POST /api/stores/{store}/products/{id}/image-axis | `apps/admin/components/ProductMediaManager.tsx:217` |
| Save draft document | form | Save product document updates as draft with idempotency and expected_version. | POST /api/stores/{store}/products/document or PUT /api/stores/{store}/products/{id}/document | `apps/admin/components/ProductDocumentForm.tsx:764` |
| Publish product | action | Save product document and transition status to active after validating main image. | POST /api/stores/{store}/products/bulk-status or PUT /api/stores/{store}/products/{id}/document | `apps/admin/components/ProductDocumentForm.tsx:781` |
| Unpublish product | action | Demote active product to draft status with user confirmation. | PUT /api/stores/{store}/products/{id}/document | `apps/admin/components/ProductDocumentForm.tsx:771` |
| Configure option axes | action | Add, modify, or remove option dimensions and synchronize variant SKU matrix. | none | `apps/admin/components/ProductDocumentVariants.tsx:133` |
| Bulk fill variants | bulk | Apply pricing or stock values across all or selected variant rows simultaneously. | none | `apps/admin/components/ProductBulkFill.tsx:40` |
| Filter collections search | filter | Search available store collections within collections checklist. | none | `apps/admin/components/ProductDocumentForm.tsx:545` |
| Toggle collection assignment | action | Select or deselect collection membership for current product. | none | `apps/admin/components/ProductDocumentForm.tsx:562` |
| Copy generated keywords | action | Copy SKU claim keywords to system clipboard following successful creation. | none | `apps/admin/components/ProductDocumentForm.tsx:738` |
| Create another product | action | Reset form to blank state to author another product without page reload. | none | `apps/admin/components/ProductDocumentForm.tsx:721` |
| Retry document save | action | Replay staged document command from session fence after network error. | POST /api/stores/{store}/products/document or PUT /api/stores/{store}/products/{id}/document | `apps/admin/components/ProductDocumentForm.tsx:691` |

现存问题：
- **P1** If initial readCollections or readWarehouses fetch fails on mount, referencesReady stays false and disables entire form permanently with no retry control. (`apps/admin/components/ProductDocumentForm.tsx:154`)
- **P2** Status dropdown renders archived option as permanently disabled, preventing merchants from archiving products directly within the editor. (`apps/admin/components/ProductDocumentForm.tsx:382`)
- **P2** Tracked single-variant product without a warehouse disables quantity input and links to inventory, but provides no inline warehouse assignment in edit mode. (`apps/admin/components/ProductDocumentForm.tsx:479`)

### P04 `/[locale]/products/import` — Import and export catalog products and variants via CSV, featuring pre-commit dry-run preview and idempotent atomic execution.
文件：`apps/admin/app/[locale]/products/import/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch store | navigation | Switch active merchant store to import/export catalog products. | none | `apps/admin/components/ProductImport.tsx:154` |
| Export products CSV | export | Download complete product and SKU catalog as a CSV spreadsheet. | GET /api/stores/{store}/tools/products/export.csv | `apps/admin/components/ProductImport.tsx:97` |
| Select CSV file | upload | Pick local CSV file and validate file type, non-emptiness, and size cap. | none | `apps/admin/components/ProductImport.tsx:77` |
| Preview CSV import | action | Send CSV to server for dry-run simulation and row-level error reporting. | POST /api/stores/{store}/tools/products/import/preview | `apps/admin/components/ProductImport.tsx:64` |
| Commit CSV import | action | Atomically apply CSV product updates with server-side file hash idempotency. | POST /api/stores/{store}/tools/products/import/commit | `apps/admin/components/ProductImport.tsx:64` |

现存问题：
- **P2** Store switcher uses window.location.assign hard navigation instead of Next router, causing full page reloads. (`apps/admin/components/ProductImport.tsx:155`)
- **P2** In-page store selector duplicates the store switcher already provided globally in WorkspaceFrame topbar. (`apps/admin/components/ProductImport.tsx:151`)
- **P1** Session boundary is read once on mount; if session expires or boundary fails, preview/commit remains disabled without a retry mechanism. (`apps/admin/components/ProductImport.tsx:49`)

### P05 `/[locale]/products` — Browse, search, and filter catalog products, execute bulk status changes and collection assignments, and perform quick price/stock edits.
文件：`apps/admin/app/[locale]/products/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter by status | filter | Filter product list by status tab (all, active, draft, archived) updating search params. | GET /api/stores/{store}/catalog-products | `apps/admin/components/ProductList.tsx:280` |
| Search products | filter | Search products by keyword query matching name, slug, or SKU code. | GET /api/stores/{store}/catalog-products | `apps/admin/components/ProductList.tsx:123` |
| Clear search filters | filter | Reset search query and status tab back to default view. | GET /api/stores/{store}/catalog-products | `apps/admin/components/ProductList.tsx:320` |
| Toggle row selection | action | Select or deselect individual product row for bulk operations. | none | `apps/admin/components/ProductList.tsx:506` |
| Select all rows | bulk | Select or deselect all visible products on current page up to 100 items. | none | `apps/admin/components/ProductList.tsx:478` |
| Bulk publish products | bulk | Publish selected products to active status after verifying cover image. | POST /api/stores/{store}/products/bulk-status | `apps/admin/components/ProductList.tsx:160` |
| Bulk unpublish products | bulk | Revert selected active products back to draft status. | POST /api/stores/{store}/products/bulk-status | `apps/admin/components/ProductList.tsx:160` |
| Bulk archive products | bulk | Transition selected products to archived status. | POST /api/stores/{store}/products/bulk-status | `apps/admin/components/ProductList.tsx:160` |
| Load collections for assignment | action | Fetch store collections to populate bulk collection assignment selector. | GET /api/stores/{store}/collections | `apps/admin/components/ProductList.tsx:185` |
| Bulk assign to collection | bulk | Append selected products to chosen collection's ordered member list. | PUT /api/stores/{store}/collections/{id}/products | `apps/admin/components/ProductList.tsx:196` |
| Duplicate product | action | Copy product and its SKUs with expected_version creating a new draft copy. | POST /api/stores/{store}/products/{id}/copy | `apps/admin/components/ProductList.tsx:225` |
| Open quick edit price | action | Open inline or modal editor to adjust product SKU prices. | none | `apps/admin/components/ProductList.tsx:691` |
| Open quick edit stock | action | Open inline or modal editor to modify inventory tracking and quantities. | none | `apps/admin/components/ProductList.tsx:721` |
| Save quick edit | form | Submit updated price or inventory delta via document PUT endpoint. | PUT /api/stores/{store}/products/{id}/document | `apps/admin/components/ProductQuickEdit.tsx:84` |
| Paginate products next | navigation | Navigate to next page of products using keyset cursor. | GET /api/stores/{store}/catalog-products | `apps/admin/components/ProductList.tsx:567` |
| Paginate products previous | navigation | Navigate back to previous cursor position stored in memory. | GET /api/stores/{store}/catalog-products | `apps/admin/components/ProductList.tsx:557` |
| Retry uncertain bulk write | action | Replay pending uncertain bulk or copy mutation command. | none | `apps/admin/components/ProductList.tsx:416` |

现存问题：
- **P1** Previous pagination cursor stack is held only in component ref; page reload or navigation clears history and disables previous button permanently. (`apps/admin/components/ProductList.tsx:97`)
- **P1** Bulk add to collection fetches collections via readCollections with hardcoded limit=100 and no cursor paging, truncating stores with >100 collections. (`apps/admin/components/ProductList.tsx:188`)
- **P2** bulkStatus client-side validation aborts entire batch without specifying which product is missing a cover image. (`apps/admin/components/ProductList.tsx:163`)

## 订单与客户

### P06 `/[locale]/customers/[customer]` — Customer detail view showing profile facts, purchase history, live claims, consent records, privacy controls, tags, and staff notes
文件：`apps/admin/app/[locale]/customers/[customer]/page.tsx` · 角色：owner, admin, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Reload customer detail | action | Reloads customer details and PII when clicking retry or after data mutation. | GET /api/stores/{store}/customers/{customer} | `apps/admin/components/CustomerDetail.tsx:86` |
| Navigate back to customers list | navigation | Navigates back to the store customers directory. | none | `apps/admin/components/CustomerDetail.tsx:79` |
| Open order from customer history | navigation | Navigates to merchant orders page focused on selected order. | none | `apps/admin/components/CustomerDetail.tsx:206` |
| Withdraw customer consent | action | Records merchant-submitted customer consent withdrawal for marketing or ad personalization. | POST /api/stores/{store}/customers/{customer}/consent-withdrawals | `apps/admin/components/CustomerDetail.tsx:157` |
| Export customer data | export | Requests and downloads customer data export JSON blob under privacy rights. | POST /api/stores/{store}/customers/{customer}/exports | `apps/admin/components/CustomerDetail.tsx:164` |
| Erase customer profile | action | Executes permanent customer data erasure after typing ERASE confirmation in dialog. | POST /api/stores/{store}/customers/{customer}/erasure | `apps/admin/components/CustomerDetail.tsx:388` |
| Retry pending privacy command | action | Replays pending export, withdrawal, or erasure with identical idempotency key. | POST /api/stores/{store}/customers/{customer}/[exports|consent-withdrawals|erasure] | `apps/admin/components/CustomerDetail.tsx:315` |
| Save assigned customer tags | form | Updates assigned tags on customer profile with revision check. | PUT /api/stores/{store}/customers/{customer}/tags | `apps/admin/components/CustomerTags.tsx:151` |
| Create tag in catalog inline | form | Creates a new tag name and color from customer tag editor. | POST /api/stores/{store}/customers/tags | `apps/admin/components/CustomerTags.tsx:157` |
| Create customer note | form | Adds internal staff note up to 1000 characters. | POST /api/stores/{store}/customers/{customer}/notes | `apps/admin/components/CustomerTagsNotes.tsx:103` |
| Edit customer note | form | Updates existing staff note text with optimistic concurrency version check. | PATCH /api/stores/{store}/customers/{customer}/notes/{id} | `apps/admin/components/CustomerTagsNotes.tsx:103` |
| Delete customer note | action | Deletes internal staff note after confirmation. | DELETE /api/stores/{store}/customers/{customer}/notes/{id} | `apps/admin/components/CustomerTagsNotes.tsx:129` |
| Load more customer notes | action | Fetches next 50-item keyset page of internal notes. | GET /api/stores/{store}/customers/{customer}/notes?limit=50&after={after} | `apps/admin/components/CustomerTagsNotes.tsx:74` |
| Paginate historical orders | navigation | Pages through archived SHOPLINE orders using cursor pagination. | GET /api/stores/{store}/customers/{customer}/historical-orders?limit=50&after={after} | `apps/admin/components/CustomerHistoricalOrders.tsx:52` |

现存问题：
- **P2** CustomerDetail receives stores prop but renders no store selector, locking store to query parameter. (`apps/admin/components/CustomerDetail.tsx:53`)
- **P2** TableFrame accessible label for consent history uses consentSection label instead of consentHistory label. (`apps/admin/components/CustomerDetail.tsx:272`)
- **P2** Historical orders pagination trail is stored in React useState and resets to first page on remount. (`apps/admin/components/CustomerHistoricalOrders.tsx:21`)

### P07 `/[locale]/customers/import` — Multi-step CSV import wizard for customer profiles and archived historical orders with schema mapping, dry-run validation, and commit
文件：`apps/admin/app/[locale]/customers/import/page.tsx` · 角色：owner, admin · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch import type | filter | Toggles between customers import and historical orders import workflows. | none | `apps/admin/components/ImportWizard.tsx:199` |
| Select CSV file | upload | Uploads CSV file and inspects header row without reading row values into memory. | none | `apps/admin/components/ImportWizard.tsx:204` |
| Map column headers | form | Maps required and optional schema fields to uploaded CSV header names. | none | `apps/admin/components/ImportWizardSteps.tsx:21` |
| Auto-guess column mapping | action | Guesses mapping between target fields and CSV headers using predefined aliases. | none | `apps/admin/components/ImportWizard.tsx:210` |
| Generate import preview | action | Submits CSV and mapping to backend to generate dry-run validation preview. | POST /api/stores/{store}/imports/{kind}/preview | `apps/admin/components/ImportWizard.tsx:126` |
| Paginate verdict rows | navigation | Navigates 50-row pages of server dry-run validation verdicts. | none | `apps/admin/components/ImportWizardSteps.tsx:60` |
| Advance to commit confirmation | navigation | Advances wizard from preview verdict inspection to final commit confirmation. | none | `apps/admin/components/ImportWizard.tsx:219` |
| Commit import batch | action | Applies import batch with expected apply row count guard. | POST /api/stores/{store}/imports/{kind}/commit | `apps/admin/components/ImportWizard.tsx:126` |
| Retry unconfirmed commit | action | Replays commit with identical immutable file and mapping after network timeout. | POST /api/stores/{store}/imports/{kind}/commit | `apps/admin/components/ImportWizard.tsx:231` |
| Download import failure CSV | export | Downloads CSV of failed row numbers and error codes from batch or preview. | GET /api/stores/{store}/imports/{batch}/results.csv?only=failed | `apps/admin/components/ImportWizard.tsx:171` |
| Step back in wizard | navigation | Returns to previous wizard stage preserving file and mapping. | none | `apps/admin/components/ImportWizard.tsx:234` |
| Restart import wizard | action | Resets wizard to start and clears file and mapping from memory. | none | `apps/admin/components/ImportWizard.tsx:239` |

现存问题：
- **P1** Orders import tab is disabled unless a customer import finishes in the same React session. (`apps/admin/components/ImportWizard.tsx:200`)
- **P2** ImportWizard page lacks a back navigation link to return to customer list. (`apps/admin/app/[locale]/customers/import/page.tsx:13`)
- **P2** ImportWizard receives stores prop from server loader but renders no store selector. (`apps/admin/components/ImportWizard.tsx:29`)

### P08 `/[locale]/customers` — Merchant customer directory with text search, tag filtering, cursor pagination, tag catalogue management, and export/report links
文件：`apps/admin/app/[locale]/customers/page.tsx` · 角色：owner, admin, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Search customers by keyword | form | Filters customer list by name, phone last 3 digits, or email keyword. | GET /api/stores/{store}/customers?limit=25&q={q} | `apps/admin/components/Customers.tsx:76` |
| Clear customer search | action | Clears search input and resets customer list to unfiltered view. | GET /api/stores/{store}/customers?limit=25 | `apps/admin/components/Customers.tsx:113` |
| Refresh customer list | action | Reloads customer directory data and boundary from the server. | GET /api/stores/{store}/customers | `apps/admin/components/Customers.tsx:124` |
| Filter customers by tag | filter | Filters customer directory by selected customer tag ID. | GET /api/stores/{store}/customers?limit=25&tag={tag} | `apps/admin/components/Customers.tsx:129` |
| Retry tag catalog load | action | Retries loading store tag catalog when initial read fails. | GET /api/stores/{store}/customers/tags | `apps/admin/components/Customers.tsx:136` |
| Open tag manager dialog | view | Opens modal dialog to create, rename, recolor, and delete store tags. | none | `apps/admin/components/CustomerTags.tsx:59` |
| Create tag in catalog | form | Adds a new customer tag with specified name and color palette. | POST /api/stores/{store}/customers/tags | `apps/admin/components/CustomerTags.tsx:103` |
| Edit tag in catalog | form | Updates name or color of an existing customer tag. | PATCH /api/stores/{store}/customers/tags/{id} | `apps/admin/components/CustomerTags.tsx:103` |
| Delete tag from catalog | action | Deletes customer tag from catalog and detaches it from customers. | DELETE /api/stores/{store}/customers/tags/{id} | `apps/admin/components/CustomerTags.tsx:97` |
| Navigate to customer detail | navigation | Navigates to individual customer profile page. | none | `apps/admin/components/Customers.tsx:217` |
| Navigate to finance reports | navigation | Navigates to financial reports page for the active store. | none | `apps/admin/components/Customers.tsx:143` |
| Navigate to customer import | navigation | Navigates to CSV import wizard for customers and orders. | none | `apps/admin/components/Customers.tsx:144` |
| Paginate next customer page | navigation | Fetches next keyset page using server opaque cursor. | GET /api/stores/{store}/customers?limit=25&after={cursor} | `apps/admin/components/Customers.tsx:193` |
| Paginate previous customer page | navigation | Pops cursor history stack to return to previous customer page. | GET /api/stores/{store}/customers?limit=25&after={cursor} | `apps/admin/components/Customers.tsx:185` |

现存问题：
- **P2** Pagination previous cursor stack is stored in memory ref and disabled on direct link access or refresh. (`apps/admin/components/Customers.tsx:66`)
- **P2** Customers component receives stores array in props but renders no store selector dropdown. (`apps/admin/components/Customers.tsx:46`)
- **P2** Tag filter select is placed inside search form, triggering router.push onChange while inside a form element. (`apps/admin/components/Customers.tsx:129`)

### P09 `/[locale]/orders/cvs-print` — Standalone new-tab page that requests an ECPay CVS shipping label print form and posts it to ECPay
文件：`apps/admin/app/[locale]/orders/cvs-print/page.tsx` · 角色：owner, admin, fulfilment · 状态：空=n/a 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Fetch CVS print form | action | Requests signed ECPay print form parameters using order ID and thermal mode. | POST /api/stores/{store}/orders/{order}/cvs-shipment/print-form | `apps/admin/app/[locale]/orders/cvs-print/CvsPrint.tsx:35` |
| Auto-submit print form | action | Automatically submits the signed form to ECPay via requestSubmit. | none | `apps/admin/app/[locale]/orders/cvs-print/CvsPrint.tsx:42` |
| Manual print form submit | form | Fallback manual submit button to post signed form to ECPay when auto-submit is blocked. | POST {form.action} | `apps/admin/app/[locale]/orders/cvs-print/CvsPrint.tsx:59` |

现存问题：
- **P2** WorkspaceFrame wraps the print tab, rendering full admin navigation inside a single-action redirect tab. (`apps/admin/app/[locale]/orders/cvs-print/page.tsx:40`)
- **P2** Error states have no retry button, forcing manual browser reload if request fails. (`apps/admin/app/[locale]/orders/cvs-print/CvsPrint.tsx:51`)
- **P2** WorkspaceFrame receives empty storeName string because server page does not resolve store name. (`apps/admin/app/[locale]/orders/cvs-print/page.tsx:40`)

### P10 `/[locale]/orders/new` — Manual order creation interface for staff to pick SKUs, configure delivery/payment modes, and generate shareable buyer links
文件：`apps/admin/app/[locale]/orders/new/page.tsx` · 角色：owner, admin · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch active store | navigation | Switches selected store and reloads manual order form for that store. | none | `apps/admin/components/ManualOrder.tsx:143` |
| Search active products | form | Searches store active catalog products to add to manual order draft. | GET /api/stores/{store}/catalog-products?q={q}&state=active | `apps/admin/components/ManualOrder.tsx:276` |
| Expand product variants | view | Fetches product detail to show purchasable SKU options and prices. | GET /api/stores/{store}/products/{id} | `apps/admin/components/ManualOrder.tsx:288` |
| Add SKU to order | action | Appends selected product variant SKU line item to order draft. | none | `apps/admin/components/ManualOrder.tsx:294` |
| Update item quantity | form | Updates quantity of selected SKU line item in order draft. | none | `apps/admin/components/ManualOrder.tsx:338` |
| Remove item from order | action | Removes selected SKU line item from order draft. | none | `apps/admin/components/ManualOrder.tsx:339` |
| Select delivery option | form | Sets delivery option and auto-selects payment mode if only one available. | none | `apps/admin/components/ManualOrder.tsx:82` |
| Select payment mode | form | Selects payment mode among bank transfer, cash on delivery, or pickup. | none | `apps/admin/components/ManualOrder.tsx:228` |
| Submit manual order | action | Creates manual order with selected items, recipient, delivery, and payment. | POST /api/stores/{store}/tools/orders/manual | `apps/admin/components/ManualOrder.tsx:96` |
| Copy buyer link | action | Copies generated buyer checkout link to clipboard. | none | `apps/admin/components/ManualOrder.tsx:167` |
| Regenerate buyer link | action | Issues a new single-use buyer checkout link for the created manual order. | POST /api/stores/{store}/tools/orders/manual/regenerate-link | `apps/admin/components/ManualOrder.tsx:120` |
| Create another order | action | Resets manual order form to blank state for placing another order. | none | `apps/admin/components/ManualOrder.tsx:107` |
| Navigate to orders list | navigation | Navigates back to merchant orders list for the active store. | none | `apps/admin/components/ManualOrder.tsx:179` |

现存问题：
- **P2** Store selector uses window.location.assign causing full page reload instead of Next.js router navigation. (`apps/admin/components/ManualOrder.tsx:143`)
- **P1** Delivery options requiring interactive map selection disable submission with warning text but provide no map picker. (`apps/admin/components/ManualOrder.tsx:78`)
- **P2** Product detail fetch failures in ItemPicker.expand are swallowed silently without user notification. (`apps/admin/components/ManualOrder.tsx:291`)

### P11 `/[locale]/orders` — Comprehensive merchant order management ledger with status bucket tabs, advanced filtering, inline detail expansion, bulk fulfillment, and export
文件：`apps/admin/app/[locale]/orders/page.tsx` · 角色：owner, admin, live_operator, fulfilment, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter orders by status | filter | Filters order list by commercial or fulfillment state. | GET /api/stores/{store}/orders?limit=25&state={state} | `apps/admin/components/MerchantOrders.tsx:568` |
| Select order bucket tab | filter | Switches active order lifecycle bucket tab. | GET /api/stores/{store}/orders?limit=25&bucket={bucket} | `apps/admin/components/MerchantOrders.tsx:660` |
| Apply search and filters | form | Filters orders by search query, payment mode, delivery kind, session, and date range. | GET /api/stores/{store}/orders?limit=25&... | `apps/admin/components/OrderListFilters.tsx:27` |
| Toggle advanced filters drawer | view | Expands or collapses secondary order filter controls and state dropdown. | none | `apps/admin/components/OrderListFilters.tsx:38` |
| Refresh order list | action | Manually triggers full refresh of orders list, actions probe, and counts. | GET /api/stores/{store}/orders | `apps/admin/components/MerchantOrders.tsx:589` |
| Export unshipped orders CSV | export | Directly downloads CSV of unshipped orders for warehouse fulfillment. | GET /api/stores/{store}/orders/unshipped.csv | `apps/admin/components/MerchantOrders.tsx:599` |
| Import tracking numbers CSV | upload | Opens modal wizard to upload shipment tracking numbers CSV and batch fulfill. | POST /api/stores/{store}/shipments/tracking-import/preview|commit | `apps/admin/components/MerchantOrders.tsx:607` |
| Select orders in bulk | bulk | Selects individual rows or all page orders for batch picking and CVS export. | none | `apps/admin/components/MerchantOrders.tsx:668` |
| Batch CVS shipping labels | bulk | Generates batch pickup shipping documents via PickList. | POST /api/stores/{store}/shipments/cvs-batch | `apps/admin/components/PickList.tsx:150` |
| Merge parcel groups | action | Merges compatible orders into shared parcel shipment group. | POST /api/stores/{store}/parcel-groups | `apps/admin/components/ParcelGroup.tsx:80` |
| Expand inline order detail | view | Fetches and expands full order details and action forms below row. | GET /api/stores/{store}/orders/{id} | `apps/admin/components/MerchantOrders.tsx:800` |
| Confirm or reject bank transfer | action | Staff confirms receipt or rejects buyer bank transfer submission. | POST /api/stores/{store}/orders/{id}/bank-transfer/{confirm|reject} | `apps/admin/components/OrderBankTransfer.tsx:95` |
| Submit Stripe payment refund | action | Processes partial or full Stripe refund with idempotency key and reason. | POST /api/stores/{store}/orders/{id}/refunds | `apps/admin/components/OrderRefunds.tsx:115` |
| Cancel order with reason | action | Cancels unfulfilled order and releases reserved stock inventory. | POST /api/stores/{store}/orders/{id}/cancel | `apps/admin/components/OrderDetailPanel.tsx:124` |
| Create return RMA | form | Registers return request for items and quantities on shipped order. | POST /api/stores/{store}/orders/{id}/returns | `apps/admin/components/OrderReturns.tsx:105` |
| Create CVS shipping order | action | Requests ECPay store pickup logistics creation and payment number. | POST /api/stores/{store}/orders/{id}/cvs-shipment | `apps/admin/components/OrderCvsShipment.tsx:120` |
| Open CVS print tab | navigation | Opens new tab to generate signed ECPay thermal or standard label. | none | `apps/admin/components/OrderCvsShipment.tsx:318` |
| Record COD collected or offline | action | Updates cash on delivery collection state to collected or returned. | POST /api/stores/{store}/orders/{id}/collection | `apps/admin/components/OrderCodCollection.tsx:90` |
| Record manual shipment tracking | form | Submits carrier name and tracking number to fulfill order lines. | POST /api/stores/{store}/orders/{id}/shipment | `apps/admin/components/OrderShipment.tsx:105` |
| Poll order list updates | realtime | Polls first page every 20 seconds while visible to detect new incoming orders. | GET /api/stores/{store}/orders?limit=25&... | `apps/admin/components/MerchantOrders.tsx:395` |
| Paginate next order page | navigation | Fetches next keyset page using server opaque cursor. | GET /api/stores/{store}/orders?limit=25&cursor={cursor} | `apps/admin/components/MerchantOrders.tsx:748` |
| Paginate previous order page | navigation | Pops cursor history stack to return to previous order page. | GET /api/stores/{store}/orders?limit=25&cursor={cursor} | `apps/admin/components/MerchantOrders.tsx:740` |

现存问题：
- **P1** Orders page route rejects query param q with 404 notFound, breaking shareable search URLs. (`apps/admin/app/[locale]/orders/page.tsx:28`)
- **P2** State filter dropdown is hidden by CSS when data-expanded=false until clicking more filters toggle. (`apps/admin/components/orders-v2.css:16`)
- **P2** MerchantOrders component receives no stores prop, preventing store switching from the orders page UI. (`apps/admin/components/MerchantOrders.tsx:93`)
- **P2** Pagination previous cursor stack is kept in memory ref and disabled on direct link access or refresh. (`apps/admin/components/MerchantOrders.tsx:159`)

### P12 `/[locale]/returns` — Overview of customer returns and RMAs by processing state, plus tracking of failed cancel refunds requiring merchant recovery
文件：`apps/admin/app/[locale]/returns/page.tsx` · 角色：owner, admin, live_operator, fulfilment, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter RMAs by state | filter | Filters RMA list by lifecycle state (registered, received, inspected, closed, cancelled). | GET /api/stores/{store}/returns?state={state} | `apps/admin/components/ReturnsList.tsx:78` |
| Retry loading returns | action | Retries fetching RMA list when guarded read fails. | GET /api/stores/{store}/returns | `apps/admin/components/ReturnsList.tsx:253` |
| Open order from RMA | navigation | Navigates to orders list with state=all and expands corresponding order detail. | none | `apps/admin/components/ReturnsList.tsx:128` |
| Navigate to gap refund | navigation | Navigates to orders list focusing on cancelled order with pending refund gap. | none | `apps/admin/components/ReturnsList.tsx:193` |

现存问题：
- **P2** Navigation links and filter tabs use raw a href tags instead of Next.js Link, forcing full page reloads. (`apps/admin/components/ReturnsList.tsx:74`)
- **P2** ReturnsList receives stores array in props but renders no store selector dropdown. (`apps/admin/components/ReturnsList.tsx:220`)
- **P2** Retry button in error block calls only list.reload, leaving failed refund gaps un-retried. (`apps/admin/components/ReturnsList.tsx:253`)

## 直播与消息

### P13 `/[locale]/messages` — Omnichannel merchant inbox for buyer messaging, human takeover, and customer order resolution.
文件：`apps/admin/app/[locale]/messages/page.tsx` · 角色：owner, admin, live_operator · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter conversation channels | filter | Filter conversation stream by channel platform or pending reply state. | GET /api/stores/{store}/inbox/conversations | `apps/admin/components/Inbox.tsx:166` |
| Refresh conversations list | action | Re-fetch conversations page and update unread counter. | GET /api/stores/{store}/inbox/conversations | `apps/admin/components/Inbox.tsx:190` |
| Select active conversation | view | Load selected conversation message thread and linked buyer details. | none | `apps/admin/components/Inbox.tsx:230` |
| Page next conversations | navigation | Load next batch of conversations using cursor pagination. | GET /api/stores/{store}/inbox/conversations | `apps/admin/components/Inbox.tsx:258` |
| Take over conversation | action | Transition conversation handling mode from automated bot to human operator. | POST /api/stores/{store}/inbox/conversations/{id}/takeover | `apps/admin/components/InboxThread.tsx:346` |
| Release conversation takeover | action | Release human operator control back to automated response handling. | POST /api/stores/{store}/inbox/conversations/{id}/release | `apps/admin/components/InboxThread.tsx:354` |
| Refresh thread messages | action | Reload thread message history and channel capability health. | GET /api/stores/{store}/inbox/conversations/{id}/messages | `apps/admin/components/InboxThread.tsx:362` |
| Load older messages | navigation | Fetch earlier messages before the lowest message sequence. | GET /api/stores/{store}/inbox/conversations/{id}/messages | `apps/admin/components/InboxThread.tsx:373` |
| Send thread reply | form | Submit text message or canned template reply to buyer. | POST /api/stores/{store}/inbox/conversations/{id}/messages | `apps/admin/components/InboxThread.tsx:440` |
| Retry message send | action | Resubmit uncertain outbound message with identical idempotency key. | POST /api/stores/{store}/inbox/conversations/{id}/messages | `apps/admin/components/InboxThread.tsx:480` |
| Link customer record | form | Associate buyer conversation identity with registered customer profile. | POST /api/stores/{store}/inbox/conversations/{id}/customer-link | `apps/admin/components/BuyerPanel.tsx:266` |
| Unlink customer record | action | Dissociate customer profile from buyer conversation. | POST /api/stores/{store}/inbox/conversations/{id}/customer-link | `apps/admin/components/BuyerPanel.tsx:267` |
| Page customer candidates | navigation | Navigate pages of customer records eligible for manual linking. | GET /api/stores/{store}/customers | `apps/admin/components/BuyerPanel.tsx:261` |
| Block buyer claims | form | Add buyer bundle to session restriction blocklist. | POST /api/stores/{store}/live-sessions/{session}/claims/blocklist | `apps/admin/components/BuyerPanel.tsx:441` |
| Copy recovery claim link | action | Issue one-time checkout link for manual bundle and copy URL. | POST /api/stores/{store}/live-sessions/{session}/claims/bundles/{bundle}/link | `apps/admin/src/features/messages/bundle-recovery.tsx:135` |

现存问题：
- **P2** Any query parameter beyond store triggers notFound without handling deep-links. (`apps/admin/app/[locale]/messages/page.tsx:29`)
- **P2** Missing onOrder prop leaves buyer order numbers rendered as unclickable plain text. (`apps/admin/components/BuyerPanel.tsx:246`)
- **P2** Clipboard copy failure leaves issued recovery link invisible with no manual copy fallback. (`apps/admin/src/features/messages/bundle-recovery.tsx:115`)

### P14 `/[locale]/studio/claims` — Live session claims setup, keyword offers, comment source binding, manual claim intake, and checkout links.
文件：`apps/admin/app/[locale]/studio/claims/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Select session scene | navigation | Select live session to configure claims rules and keyword offers. | none | `apps/admin/components/LiveSettingsEntry.tsx:25` |
| Refresh claims facts | action | Re-read claim window, board, offers, bundles, and source status. | GET /api/stores/{store}/live-sessions/{scene}/claims | `apps/admin/components/StudioClaims.tsx:472` |
| Toggle claim window | action | Open or close keyword intake window for current session round. | POST /api/stores/{store}/live-sessions/{scene}/claims/window | `apps/admin/components/StudioClaims.tsx:496` |
| Change parsing match mode | filter | Switch comment parsing grammar between exact, quantity-only, and contains. | POST /api/stores/{store}/live-sessions/{scene}/claims/window | `apps/admin/components/StudioClaims.tsx:487` |
| Copy host prompt | action | Copy formatted host teleprompter script to system clipboard. | none | `apps/admin/components/StudioClaims.tsx:524` |
| Bind comment source | form | Save Facebook post link or Instagram media object binding. | PUT /api/stores/{store}/live-sessions/{scene}/claim-source | `apps/admin/components/StudioClaims.tsx:545` |
| Import offers from library | action | Batch import pre-configured default keyword offers from library. | POST /api/stores/{store}/live-sessions/{scene}/claims/offers/import | `apps/admin/components/StudioClaims.tsx:599` |
| Copy offers from session | form | Duplicate active keyword offers from another completed live session. | POST /api/stores/{store}/live-sessions/{scene}/claims/offers/import | `apps/admin/components/StudioClaims.tsx:601` |
| Save offer live price | action | Set promotional live stream price or clear back to SKU price. | PATCH /api/stores/{store}/live-sessions/{scene}/claims/offers/{offer_id} | `apps/admin/components/StudioClaims.tsx:632` |
| Update offer limit or status | action | Save updated per-claim quantity limit or pause/resume claim offer. | PATCH /api/stores/{store}/live-sessions/{scene}/claims/offers/{offer_id} | `apps/admin/components/StudioClaims.tsx:637` |
| Add claim offer | form | Create new keyword claim offer mapped to product SKU. | POST /api/stores/{store}/live-sessions/{scene}/claims/offers | `apps/admin/components/StudioClaims.tsx:647` |
| Add keyword to library | action | Persist keyword and SKU pairing into store-wide keyword library. | PUT /api/stores/{store}/live-sessions/{scene}/claims/library/{sku_id} | `apps/admin/components/StudioClaims.tsx:672` |
| Save library keyword | action | Update mapped keyword string for catalog SKU in library. | PUT /api/stores/{store}/live-sessions/{scene}/claims/library/{sku_id} | `apps/admin/components/StudioClaims.tsx:693` |
| Remove library keyword | action | Delete keyword mapping entry from store keyword library. | PUT /api/stores/{store}/live-sessions/{scene}/claims/library/{sku_id} | `apps/admin/components/StudioClaims.tsx:694` |
| Record manual claim | form | Submit manual customer claim comment for bundle or new buyer. | POST /api/stores/{store}/live-sessions/{scene}/claims/manual | `apps/admin/components/StudioClaims.tsx:704` |
| Issue or rotate link | action | Generate new one-time checkout link for buyer claim bundle. | POST /api/stores/{store}/live-sessions/{scene}/claims/bundles/{bundle_id}/link | `apps/admin/components/StudioClaims.tsx:742` |
| Release and rotate link | action | Release social identity binding and issue fresh cart link. | POST /api/stores/{store}/live-sessions/{scene}/claims/bundles/{bundle_id}/link | `apps/admin/components/StudioClaims.tsx:744` |
| Page next bundles | navigation | Fetch next page of buyer claim bundles using cursor. | GET /api/stores/{store}/live-sessions/{scene}/claims/bundles | `apps/admin/components/StudioClaims.tsx:749` |

现存问题：
- **P2** Providing scene parameter without store causes liveSettingsSelection to return null (404). (`apps/admin/app/[locale]/studio/claims/page.tsx:35`)
- **P1** Claim link issuance fails with no-origin if store has no products or storefront origin. (`apps/admin/components/StudioClaims.tsx:425`)

### P15 `/[locale]/studio/console` — Real-time live streaming operations console with lifecycle controls, metrics, inventory adjustment, comments, and printing.
文件：`apps/admin/app/[locale]/studio/console/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Select live session | navigation | Switch current console view to different live streaming session. | none | `apps/admin/components/LiveWorkspace.tsx:37` |
| Refresh console metrics | realtime | Manually re-fetch console snapshot, session details, and offer controls. | GET /api/stores/{store}/live-sessions/{session}/console | `apps/admin/components/LiveConsole.tsx:71` |
| Transition lifecycle state | action | Start live session (draft to live) or end session (live to ended). | POST /api/stores/{store}/live-sessions/{session}/lifecycle | `apps/admin/components/LiveConsole.tsx:79` |
| Copy ended session | form | Duplicate ended session title, offers, and settings into new draft. | POST /api/stores/{store}/live-sessions/{session}/copy | `apps/admin/components/LiveConsole.tsx:94` |
| Toggle offer claim active | action | Pause or resume keyword claims intake for specific offer. | PATCH /api/stores/{store}/live-sessions/{session}/claims/offers/{offer_id} | `apps/admin/components/LiveConsole.tsx:136` |
| Adjust offer inventory | form | Submit inventory balance delta adjustment for offer SKU. | POST /api/stores/{store}/live-sessions/{session}/inventory/adjustments | `apps/admin/components/LiveConsole.tsx:139` |
| Acknowledge stock change | action | Accept updated sellable stock baseline following background change. | none | `apps/admin/components/LiveConsole.tsx:147` |
| Recommend offer | action | Highlight offer as currently featured promotion in live stream. | POST /api/stores/{store}/live-sessions/{session}/claims/offers/{offer_id}/recommend | `apps/admin/components/LiveConsole.tsx:143` |
| Filter stream comments | filter | Filter comment stream by all, keyword claims, private DMs, or unreplied. | none | `apps/admin/components/CommentStream.tsx:170` |
| Refresh comments stream | action | Poll incoming stream comments and update active buffer. | none | `apps/admin/components/CommentStream.tsx:161` |
| Load older comments | navigation | Fetch earlier comments before oldest buffered comment. | none | `apps/admin/components/CommentStream.tsx:293` |
| Select comment for reply | view | Select comment to open reply composer and load buyer panel. | none | `apps/admin/components/CommentStream.tsx:281` |
| Send comment reply | form | Submit private message or public comment reply to buyer. | POST /api/stores/{store}/live-sessions/{session}/comments/{ref}/{mode}-reply | `apps/admin/components/CommentReply.tsx:260` |
| Print comment labels | action | Preview keyword claim labels and trigger native browser print dialog. | POST /api/stores/{store}/live-sessions/{session}/comments/{ref}/print | `apps/admin/components/CommentLabelPrint.tsx:58` |
| Select labels for print | bulk | Toggle comment rows for batch claim label printing. | none | `apps/admin/components/CommentLabelPrint.tsx:129` |

现存问题：
- **P1** Failure of secondary readOfferControls leaves all offer active switches disabled without specific error. (`apps/admin/components/LiveConsole.tsx:110`)
- **P2** Recommended offer has no un-recommend or toggle-off action once set. (`apps/admin/components/LiveConsole.tsx:143`)

### P16 `/[locale]/studio` — Live streaming session planning list, draft creation and editing, aspect ratio selection, and rehearsal controls.
文件：`apps/admin/app/[locale]/studio/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch active store | navigation | Switch store scope and navigate to its studio sessions list. | none | `apps/admin/components/Studio.tsx:625` |
| Open session claims | navigation | Navigate to keyword claims configuration for selected session. | none | `apps/admin/components/Studio.tsx:628` |
| Open session console | navigation | Navigate to real-time operations console for selected session. | none | `apps/admin/components/Studio.tsx:631` |
| Refresh sessions list | action | Re-fetch sessions list, selected draft details, and sales results. | GET /api/stores/{store}/live-sessions | `apps/admin/components/Studio.tsx:632` |
| Initialize new session | view | Clear editor to blank draft mode for creating a new session. | none | `apps/admin/components/Studio.tsx:644` |
| Select session draft | view | Load selected session draft details and rehearsal telemetry. | GET /api/stores/{store}/live-sessions/{session_id} | `apps/admin/components/Studio.tsx:652` |
| Page sessions list | navigation | Navigate to next or previous page of live sessions. | GET /api/stores/{store}/live-sessions | `apps/admin/components/Studio.tsx:667` |
| Save session draft | form | Create new session draft or edit existing title, schedule, and aspect ratio. | POST /api/stores/{store}/live-sessions or PATCH /api/stores/{store}/live-sessions/{session_id} | `apps/admin/components/Studio.tsx:716` |
| Start rehearsal attempt | action | Trigger media rehearsal execution for prepared draft session. | POST /api/stores/{store}/live-sessions/{session_id}/rehearsal/start | `apps/admin/components/Studio.tsx:759` |
| Stop rehearsal attempt | action | Halt active rehearsal attempt and initiate resource cleanup. | POST /api/stores/{store}/live-sessions/{session_id}/rehearsal/stop | `apps/admin/components/Studio.tsx:759` |
| Copy session draft | form | Duplicate session title and offers into a new live session draft. | POST /api/stores/{store}/live-sessions/{session_id}/copy | `apps/admin/src/features/live/SessionCopy.tsx:33` |

现存问题：
- **P2** Unsaved edits in draft form silently disable rehearsal start button without tooltip explanation. (`apps/admin/components/Studio.tsx:611`)
- **P2** When media is disabled (mediaOn false), rehearsal column is completely omitted from DOM. (`apps/admin/components/Studio.tsx:725`)

### P17 `/[locale]/studio/settings` — Live session operational settings for manual payment reminders, sold-out auto-reply template, and restricted buyers.
文件：`apps/admin/app/[locale]/studio/settings/page.tsx` · 角色：owner, admin, live_operator, viewer · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Select session scene | navigation | Switch session context to view its reminder report and restricted blocklist. | none | `apps/admin/components/LiveSettings.tsx:112` |
| Load more sessions | navigation | Fetch next page of store live sessions for session picker selector. | GET /api/stores/{store}/live-sessions | `apps/admin/components/LiveSettings.tsx:134` |
| Refresh settings data | action | Re-fetch sessions, sold-out settings, reminder reports, and blocklist entries. | GET /api/stores/{store}/live-settings/sold-out-reply | `apps/admin/components/LiveSettings.tsx:139` |
| Retry settings command | action | Resubmit uncertain reminder, template, or blocklist command from stored receipt. | POST, PUT, or DELETE /api/stores/{store}/... | `apps/admin/components/LiveSettings.tsx:167` |
| Trigger payment reminders | action | Dispatch payment reminder messages to buyers with uncompleted session claims. | POST /api/stores/{store}/live-sessions/{scene}/reminders | `apps/admin/components/LiveSettings.tsx:184` |
| Copy followup claim link | action | Copy unexpired checkout link for followup buyer to system clipboard. | none | `apps/admin/components/LiveSettings.tsx:238` |
| Toggle sold-out auto-reply | action | Enable or disable automated private DM response for sold-out comments. | none | `apps/admin/components/LiveSettings.tsx:278` |
| Save sold-out template | form | Publish message template and save store-wide sold-out auto-reply configuration. | POST /api/stores/{store}/message-templates or PUT /api/stores/{store}/live-settings/sold-out-reply | `apps/admin/components/LiveSettings.tsx:312` |
| Confirm unblock buyer | action | Remove buyer restriction entry from live session blocklist. | DELETE /api/stores/{store}/live-sessions/{scene}/claims/blocklist/entries/{entry} | `apps/admin/components/LiveSettings.tsx:360` |
| Page blocklist entries | navigation | Navigate forward or backward through paginated blocklist entries. | GET /api/stores/{store}/live-sessions/{scene}/claims/blocklist | `apps/admin/components/LiveSettings.tsx:398` |

现存问题：
- **P2** Providing scene parameter without store causes liveSettingsSelection to return null (404). (`apps/admin/app/[locale]/studio/settings/page.tsx:40`)
- **P2** Toggling sold-out switch with system template active disables save unless custom template is typed. (`apps/admin/components/LiveSettings.tsx:314`)

## 财务、设置与团队

### P18 `/[locale]/billing` — Manage merchant subscription standing, active subscriptions, plan choices, usage metrics, and Stripe checkout/portal redirects.
文件：`apps/admin/app/[locale]/billing/page.tsx` · 角色：owner, billing:manage · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| View billing status | view | Fetch merchant subscription standing, active subscriptions, available plans, and usage metrics | GET /api/stores/{store}/billing | `apps/admin/lib/billing-client.ts:11` |
| Retry billing fetch | action | Reload billing status after request failure | GET /api/stores/{store}/billing | `apps/admin/components/Billing.tsx:91` |
| Open Stripe customer portal | action | Request Stripe Customer Portal session URL and redirect browser to manage payment methods | POST /api/stores/{store}/billing/portal | `apps/admin/lib/billing-client.ts:35` |
| Select subscription plan | action | Initiate Stripe checkout session for selected plan and redirect browser to checkout | POST /api/stores/{store}/billing/checkout | `apps/admin/lib/billing-client.ts:32` |

现存问题：
- **P2** Subscriptions table renders nothing without an empty state placeholder when subscriptions list is empty (`apps/admin/components/Billing.tsx:153`)
- **P2** Stores array prop is accepted but completely unused by Billing component (`apps/admin/components/Billing.tsx:43`)

### P19 `/[locale]/design` — Edit storefront design drafts, manage profile, navigation, homepage sections and content pages, preview, publish, and rollback versions.
文件：`apps/admin/app/[locale]/design/page.tsx` · 角色：owner, integration:manage, integration:read · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch store | filter | Switch store draft via query parameter | none | `apps/admin/components/Design.tsx:175` |
| View design draft | view | Fetch design draft document and version | GET /api/stores/{store}/design/draft | `apps/admin/lib/design-client.ts:20` |
| View published versions | view | Fetch list of published design versions and history | GET /api/stores/{store}/design/versions | `apps/admin/lib/design-client.ts:22` |
| View media assets | view | Fetch uploaded media image items for store | GET /api/stores/{store}/design/media | `apps/admin/lib/design-client.ts:24` |
| Save design draft | action | Save design document changes with compare-and-set version check | PUT /api/stores/{store}/design/draft | `apps/admin/lib/design-client.ts:62` |
| Publish design draft | action | Publish saved draft version to live storefront | POST /api/stores/{store}/design/publish | `apps/admin/lib/design-client.ts:65` |
| Preview storefront draft | action | Issue 15-minute preview token and open storefront URL in new tab | POST /api/stores/{store}/design/preview-token | `apps/admin/lib/design-client.ts:71` |
| Rollback to previous version | action | Restore and republish a historical design version | POST /api/stores/{store}/design/rollback | `apps/admin/lib/design-client.ts:68` |
| Upload media image | upload | Upload JPEG, PNG, or WebP image under 2MB | POST /api/stores/{store}/design/media | `apps/admin/lib/design-client.ts:74` |
| Delete media asset | action | Delete unused uploaded design image asset | POST /api/stores/{store}/design/media/{id}/delete | `apps/admin/lib/design-client.ts:80` |
| Reload editor on conflict | action | Reload draft and version state after version conflict | GET /api/stores/{store}/design/draft | `apps/admin/components/Design.tsx:533` |

现存问题：
- **P2** In-page store selector duplicates WorkspaceFrame navigation and triggers hard window.location reload (`apps/admin/components/Design.tsx:177`)
- **P2** Preview button unconditionally opens blank window before saving, closing it if validation fails (`apps/admin/components/Design.tsx:356`)

### P20 `/[locale]/finance` — Review merchant financial summary across date ranges, inspect daily sales and collection metrics, and export audited CSV reports.
文件：`apps/admin/app/[locale]/finance/page.tsx` · 角色：owner, orders:read, orders:export · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter finance date range | filter | Update date range filter parameters up to 91 days | none | `apps/admin/components/Finance.tsx:87` |
| View finance summary | view | Fetch daily financial summary breakdown and totals for specified date range | GET /api/stores/{store}/finance/summary?from={from}&to={to} | `apps/admin/lib/customers-client.ts:68` |
| Export finance CSV | export | Stream financial summary CSV file for selected range | GET /api/stores/{store}/finance/summary.csv?from={from}&to={to} | `apps/admin/lib/customers-client.ts:71` |
| Retry finance fetch | action | Reload financial summary after request failure | GET /api/stores/{store}/finance/summary?from={from}&to={to} | `apps/admin/components/Finance.tsx:135` |
| Navigate to sales reports | navigation | Route user to detailed sales reports page | none | `apps/admin/components/Finance.tsx:123` |

现存问题：
- **P2** Stores array prop is accepted by the Finance component but never used (`apps/admin/components/Finance.tsx:51`)

### P21 `/[locale]/finance/reports` — Analyze multi-dimensional sales reports across products, channels, live stream conversion funnels, and manual orders with concurrency-locked CSV downloads.
文件：`apps/admin/app/[locale]/finance/reports/page.tsx` · 角色：owner, orders:read, live:read, orders:export · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter report date range | filter | Apply modified date range to sales report queries | none | `apps/admin/components/Reports.tsx:97` |
| Switch report tab | filter | Switch between products, channels, funnel, and manual-orders reports | none | `apps/admin/components/Reports.tsx:47` |
| Switch store | filter | Change selected store for sales report analysis | none | `apps/admin/components/Reports.tsx:93` |
| Sort product report | filter | Sort product sales table by name or numeric metrics | none | `apps/admin/components/Reports.tsx:163` |
| View sales report | view | Fetch report data for selected tab, date range, and store | GET /api/stores/{store}/reports/{report}?from={from}&to={to} | `apps/admin/lib/reports-client.ts:26` |
| Export report CSV | export | Download audited sales report CSV with concurrency session lock | GET /api/stores/{store}/reports/{report}.csv?from={from}&to={to} | `apps/admin/lib/reports-client.ts:116` |
| Retry report query | action | Reload active report after fetch error | GET /api/stores/{store}/reports/{report}?from={from}&to={to} | `apps/admin/components/Reports.tsx:159` |

现存问题：
- **P2** In-page store selector duplicates WorkspaceFrame navigation dropdown (`apps/admin/components/Reports.tsx:137`)
- **P2** Export CSV button is silently disabled while draft date inputs differ from applied dates (`apps/admin/components/Reports.tsx:111`)

### P22 `/[locale]/settings/operations` — Track asynchronous integration failures and UNKNOWN operations, review audit event timelines, and execute authorized query, retry, or cancel commands.
文件：`apps/admin/app/[locale]/settings/operations/page.tsx` · 角色：owner, integration:manage, integration:read · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter operations | filter | Filter operations by state category (attention, failed, pending, all) | none | `apps/admin/components/OperationsLedger.tsx:379` |
| Paginate operations | filter | Load next page of operations via cursor | none | `apps/admin/components/OperationsLedger.tsx:461` |
| View operations list | view | Fetch filtered page of asynchronous operations for store | GET /api/stores/{store}/operations?state={state}&limit=20&cursor={cursor} | `apps/admin/lib/operations-client.ts:17` |
| View operation detail | view | Fetch single operation facts, available action capabilities, and audit events | GET /api/stores/{store}/operations/{id} | `apps/admin/lib/operations-client.ts:31` |
| Refresh operations | action | Reload operations list and active detail from server | GET /api/stores/{store}/operations?state={state}&limit=20&cursor={cursor} | `apps/admin/components/OperationsLedger.tsx:393` |
| Open operation drawer | action | Open detail drawer dialog for selected operation | none | `apps/admin/components/OperationsLedger.tsx:448` |
| Query operation status | action | Dispatch query command to check operation status with external provider | POST /api/stores/{store}/operations/{id}/query | `apps/admin/lib/operations-client.ts:44` |
| Cancel operation | action | Cancel pending or timed-out operation | POST /api/stores/{store}/operations/{id}/cancel | `apps/admin/lib/operations-client.ts:44` |
| Retry operation | action | Retry failed operation with fresh idempotency key | POST /api/stores/{store}/operations/{id}/retry | `apps/admin/lib/operations-client.ts:44` |
| Resubmit uncertain command | action | Resubmit unconfirmed command using identical idempotency key and body | POST /api/stores/{store}/operations/{id}/{action} | `apps/admin/components/OperationsLedger.tsx:416` |
| Switch store | filter | Switch store query parameter for operations ledger | none | `apps/admin/components/OperationsLedger.tsx:354` |

现存问题：
- **P2** In-page navigation links to Settings and Ads use raw anchor tags triggering full page reloads (`apps/admin/components/OperationsLedger.tsx:338`)
- **P2** Store switcher duplicates WorkspaceFrame navigation dropdown (`apps/admin/components/OperationsLedger.tsx:348`)

### P23 `/[locale]/settings` — Configure store markets, delivery services, payment gateways, notification preferences, custom domains, and Meta Facebook/Instagram integrations.
文件：`apps/admin/app/[locale]/settings/page.tsx` · 角色：owner, integration:manage, integration:read · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch wizard step | navigation | Move forward or backward between wizard steps 1 through 4 | none | `apps/admin/components/SettingsWizard.tsx:1066` |
| Save market | form | Create or update sales market code, name, currency, and active status | POST /api/stores/{store}/markets or PUT /api/stores/{store}/markets/{id} | `apps/admin/components/SettingsWizard.tsx:550` |
| Save shipping policy | form | Update country shipping rates, free shipping threshold, and tax calculation rules | PUT /api/stores/{store}/markets/{id}/countries/{country}/policy | `apps/admin/components/SettingsWizard.tsx:615` |
| Save delivery service | form | Configure market delivery service kind, mode, visibility, and sorting index | PUT /api/stores/{store}/markets/{id}/countries/{country}/delivery-services/{code} | `apps/admin/components/SettingsWizard.tsx:710` |
| Paginate markets | filter | Fetch additional markets using cursor pagination | GET /api/stores/{store}/markets?limit=10&cursor={cursor} | `apps/admin/components/SettingsWizard.tsx:200` |
| Paginate delivery services | filter | Fetch additional delivery services using cursor pagination | GET /api/stores/{store}/markets/{id}/countries/{country}/delivery-services?limit=10&cursor={cursor} | `apps/admin/components/SettingsWizard.tsx:240` |
| Retry uncertain write | action | Resubmit unconfirmed settings modification command | POST /api/stores/{store}/... or PUT /api/stores/{store}/... | `apps/admin/components/SettingsWizard.tsx:1103` |
| Refresh settings status | action | Reload store markets, policies, and services from server | GET /api/stores/{store}/markets | `apps/admin/components/SettingsWizard.tsx:1661` |

现存问题：
- **P2** In-page navigation links to operations and card payments use raw anchor tags causing full page reloads (`apps/admin/components/SettingsWizard.tsx:1047`)
- **P2** Step 1 contains no configuration fields and acts solely as an informational intermediary screen (`apps/admin/components/SettingsWizard.tsx:1115`)
- **P2** Aside status column duplicates information already presented in Step 4 view (`apps/admin/components/SettingsWizard.tsx:1672`)

### P24 `/[locale]/settings/payments/card` — Activate or deactivate platform-collected credit card payments, review terms of service, and configure merchant statement descriptor suffixes.
文件：`apps/admin/app/[locale]/settings/payments/card/page.tsx` · 角色：owner, integration:read, billing:manage · 状态：空=n/a 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| View card payment summary | view | Fetch platform Stripe readiness, store activation state, limits, and descriptor preview | GET /api/stores/{store}/payments/card | `apps/admin/lib/card-payments-client.ts:17` |
| Open activation dialog | action | Open platform terms agreement and descriptor configuration dialog | none | `apps/admin/components/CardPayments.tsx:215` |
| Enable card payments | form | Activate platform credit card payments with accepted terms version and optional descriptor suffix | PUT /api/stores/{store}/payments/card | `apps/admin/lib/card-payments-client.ts:55` |
| Open deactivation dialog | action | Open confirmation dialog to disable card payments | none | `apps/admin/components/CardPayments.tsx:220` |
| Disable card payments | action | Deactivate platform credit card processing for store | PUT /api/stores/{store}/payments/card | `apps/admin/lib/card-payments-client.ts:55` |
| Retry summary fetch | action | Reload card payment summary after request failure | GET /api/stores/{store}/payments/card | `apps/admin/components/CardPayments.tsx:85` |

现存问题：
- **P2** Stores array prop is accepted by CardPayments but discarded and unused (`apps/admin/components/CardPayments.tsx:52`)
- **P2** Settlements link uses a raw anchor tag instead of Next.js Link causing full page reload (`apps/admin/components/CardPayments.tsx:229`)
- **P2** Activation and deactivation dialogs use section elements without modal focus trapping (`apps/admin/components/CardPayments.tsx:236`)

### P25 `/[locale]/settings/settlements` — Review weekly platform card-payment settlement statements, fee calculations, payout transfer references, and individual transaction line items.
文件：`apps/admin/app/[locale]/settings/settlements/page.tsx` · 角色：owner, billing:manage · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| View settlement statements | view | Fetch list of up to 52 closed weekly settlement statements | GET /api/stores/{store}/settlements | `apps/admin/lib/card-payments-client.ts:22` |
| View statement detail | view | Fetch single settlement statement breakdown and transaction lines | GET /api/stores/{store}/settlements/{id} | `apps/admin/lib/card-payments-client.ts:25` |
| Open statement lines | action | Select settlement statement to view line item details | none | `apps/admin/components/Settlements.tsx:146` |
| Back to statements list | action | Return from statement detail view back to statements overview table | none | `apps/admin/components/Settlements.tsx:76` |
| Retry statements list | action | Reload settlement statements list after fetch failure | GET /api/stores/{store}/settlements | `apps/admin/components/Settlements.tsx:69` |
| Retry statement detail | action | Reload single statement line items after fetch failure | GET /api/stores/{store}/settlements/{id} | `apps/admin/components/Settlements.tsx:86` |

现存问题：
- **P2** Stores array prop is accepted by Settlements but discarded and unused (`apps/admin/components/Settlements.tsx:30`)
- **P2** Selected statement state is kept in local React state only, resetting back to list view on page refresh (`apps/admin/components/Settlements.tsx:42`)

### P26 `/[locale]/team` — Manage merchant staff members, modify staff roles, send and revoke email invitations, and remove team member access.
文件：`apps/admin/app/[locale]/team/page.tsx` · 角色：owner · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch store | filter | Change active store for team management via hard reload | none | `apps/admin/components/Team.tsx:51` |
| View team and invitations | view | Fetch store staff members and pending email invitations | POST /api/team/list | `apps/admin/lib/team-client.ts:45` |
| Send team invitation | form | Send invitation email to new team member with assigned role | POST /api/team/invite | `apps/admin/lib/team-client.ts:59` |
| Resend invitation email | action | Resend invitation email to pending invitee | POST /api/team/invite | `apps/admin/lib/team-client.ts:59` |
| Revoke invitation | action | Cancel and invalidate an open pending invitation | POST /api/team/revoke-invite | `apps/admin/lib/team-client.ts:61` |
| Update member role | action | Change access role for existing team member | POST /api/team/set-role | `apps/admin/lib/team-client.ts:63` |
| Remove team member | action | Revoke store access for a team member | POST /api/team/remove | `apps/admin/lib/team-client.ts:65` |
| Retry team fetch | action | Reload team members and invitations after fetch failure | POST /api/team/list | `apps/admin/components/Team.tsx:61` |

现存问题：
- **P2** In-page store selector duplicates WorkspaceFrame navigation and triggers hard window.location reload (`apps/admin/components/Team.tsx:51`)
- **P2** Team members table lacks an empty state fallback if no members are returned (`apps/admin/components/Team.tsx:171`)

## 营销、账号与官网

### P27 `/(public)/site/[locale]/[page]` — Public platform marketing, company facts, and legal compliance pages.
文件：`apps/admin/app/(public)/site/[locale]/[page]/page.tsx` · 角色：public visitor · 状态：空=n/a 加载=missing 出错=missing 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Skip to main content | navigation | Jumps keyboard focus directly to main content landmark. | none | `apps/admin/app/(public)/site/[locale]/[page]/page.tsx:70` |
| Navigate to home | navigation | Navigates to platform home page via logo link. | none | `apps/admin/app/(public)/site/[locale]/[page]/page.tsx:75` |
| Navigate to about page | navigation | Navigates to platform about page. | none | `apps/admin/app/(public)/site/[locale]/[page]/page.tsx:84` |
| Switch platform locale | navigation | Switches language between zh-TW, zh-CN, and EN. | none | `apps/admin/app/(public)/site/[locale]/[page]/page.tsx:88` |
| Navigate to merchant login | navigation | Navigates to merchant admin login on admin origin. | none | `apps/admin/app/(public)/site/[locale]/[page]/page.tsx:98` |
| Navigate to merchant signup | navigation | Navigates to merchant registration on admin origin. | none | `apps/admin/components/platform/PlatformHome.tsx:55` |
| Contact via email | navigation | Opens email client via mailto link. | none | `apps/admin/components/platform/PlatformDocument.tsx:51` |
| Navigate back to home | navigation | Returns from legal document or about view to home. | none | `apps/admin/components/platform/PlatformDocument.tsx:118` |

现存问题：
- **P2** Unlisted page or locale param triggers notFound() 404 without helpful error guidance. (`apps/admin/app/(public)/site/[locale]/[page]/page.tsx:30`)
- **P2** Sample product price NT$590 is hardcoded instead of using locale currency formatting. (`apps/admin/components/platform/PlatformHome.tsx:118`)

### P28 `/[locale]/ads/attribution` — Read-only merchant attribution report linking ad drafts and live sessions with sales and audience metrics.
文件：`apps/admin/app/[locale]/ads/attribution/page.tsx` · 角色：merchant owner, staff · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Apply date range filter | filter | Filters attribution report by start and end dates. | GET /api/stores/[store]/ads/attribution | `apps/admin/components/Attribution.tsx:156` |
| Select ad draft | filter | Switches displayed draft performance and audience breakdown. | none | `apps/admin/components/Attribution.tsx:271` |
| Select live session | filter | Switches displayed live session funnel and timeline. | none | `apps/admin/components/Attribution.tsx:295` |
| Retry attribution fetch | action | Retries loading attribution report after failure. | GET /api/stores/[store]/ads/attribution | `apps/admin/components/Attribution.tsx:230` |
| Navigate back to ads | navigation | Navigates back to main ads management overview. | none | `apps/admin/components/Attribution.tsx:146` |
| Reconnect Meta in settings | navigation | Navigates to settings page when Meta auth is missing. | none | `apps/admin/components/AttributionPanels.tsx:448` |
| Refresh live audience metrics | action | Requests queued Meta audience read for live session. | POST /api/stores/[store]/ads/sessions/[session]/audience-read | `apps/admin/components/AttributionAudienceRead.tsx:134` |
| Retry audience read operation | action | Retries unconfirmed audience read using saved idempotency key. | POST /api/stores/[store]/ads/sessions/[session]/audience-read | `apps/admin/components/AttributionAudienceRead.tsx:147` |

现存问题：
- **P2** Unwhitelisted query parameters cause unexpected 404 notFound() response. (`apps/admin/app/[locale]/ads/attribution/page.tsx:34`)
- **P2** Duplicate id attribute rendered on Field wrapper and DateControl element. (`apps/admin/components/Attribution.tsx:173`)

### P29 `/[locale]/ads` — Manages Meta ads integration, asset bindings, catalog feed, ad campaign drafts, reports, and Conversions API.
文件：`apps/admin/app/[locale]/ads/page.tsx` · 角色：merchant owner, staff · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch active store | filter | Switches current store for ads management. | none | `apps/admin/components/Ads.tsx:330` |
| Refresh ads data | action | Reloads ads settings, connections, and campaign drafts. | none | `apps/admin/components/Ads.tsx:339` |
| Connect Meta account | action | Starts Meta OAuth connection flow. | POST /api/ads/meta/connect | `apps/admin/components/Ads.tsx:214` |
| Bind Meta assets | form | Binds selected Meta ad account and dataset. | POST /api/stores/[store]/ads/meta/bindings | `apps/admin/components/AdsConnection.tsx:317` |
| Unbind ad account | action | Disconnects Meta ad account from store. | POST /api/stores/[store]/ads/meta/unbind | `apps/admin/components/AdsConnection.tsx:83` |
| Copy catalog feed URL | action | Copies catalog feed URL to clipboard. | none | `apps/admin/components/AdsConnection.tsx:408` |
| Create ad draft | form | Creates new ad campaign draft specification. | POST /api/stores/[store]/ads/drafts | `apps/admin/components/Ads.tsx:298` |
| Update ad draft | form | Updates draft configuration with optimistic locking revision. | PUT /api/stores/[store]/ads/drafts/[id] | `apps/admin/components/Ads.tsx:283` |
| Approve ad draft | action | Marks draft approved for Meta publication. | POST /api/stores/[store]/ads/drafts/[id]/approve | `apps/admin/components/Ads.tsx:317` |
| Publish ad draft | action | Submits approved ad campaign to Meta. | POST /api/stores/[store]/ads/drafts/[id]/publish | `apps/admin/components/Ads.tsx:317` |
| Pause ad campaign | action | Pauses active running Meta ad campaign. | POST /api/stores/[store]/ads/drafts/[id]/pause | `apps/admin/components/Ads.tsx:317` |
| End ad campaign | action | Permanently stops running ad campaign on Meta. | POST /api/stores/[store]/ads/drafts/[id]/end | `apps/admin/components/Ads.tsx:317` |
| Load performance report | filter | Loads aggregated metrics for selected date window. | GET /api/stores/[store]/ads/report | `apps/admin/components/AdsResults.tsx:54` |
| Save CAPI settings | form | Configures Meta Conversions API and test code. | PUT /api/stores/[store]/ads/capi | `apps/admin/components/Ads.tsx:509` |

现存问题：
- **P2** Unwhitelisted query parameters cause unexpected 404 notFound() response. (`apps/admin/app/[locale]/ads/page.tsx:31`)
- **P2** Duplicate id attribute rendered on Field wrapper and DateControl element. (`apps/admin/components/AdsResults.tsx:106`)

### P30 `/[locale]/invite/[token]` — Staff member invitation acceptance page, joining authenticated staff account to invited store.
文件：`apps/admin/app/[locale]/invite/[token]/page.tsx` · 角色：invited staff, visitor · 状态：空=n/a 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Accept team invitation | action | Accepts staff invitation token to join store team. | POST /api/team/accept | `apps/admin/components/TeamInvite.tsx:40` |
| Navigate to sign in | navigation | Navigates to sign in with return path in hash. | none | `apps/admin/components/TeamInvite.tsx:77` |
| Navigate to sign up | navigation | Navigates to sign up with return path in hash. | none | `apps/admin/components/TeamInvite.tsx:78` |
| Open dashboard | navigation | Navigates to store admin root after accepting invite. | none | `apps/admin/components/TeamInvite.tsx:58` |

现存问题：
- **P2** Invalid token length or format immediately throws 404 without user-friendly explanation. (`apps/admin/app/[locale]/invite/[token]/page.tsx:20`)

### P31 `/[locale]` — Merchant dashboard for authenticated stores, initial store onboarding wizard, or sign-in entry.
文件：`apps/admin/app/[locale]/page.tsx` · 角色：merchant owner, staff, visitor · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Navigate to create order | navigation | Navigates to manual order creation page. | none | `apps/admin/components/Dashboard.tsx:62` |
| Navigate to import products | navigation | Navigates to product import wizard. | none | `apps/admin/components/Dashboard.tsx:63` |
| Navigate to inventory | navigation | Navigates to stock inventory ledger. | none | `apps/admin/components/Dashboard.tsx:64` |
| Drill down to filtered orders | navigation | Navigates to orders list filtered by todo state. | none | `apps/admin/components/Dashboard.tsx:80` |
| View order details | navigation | Navigates to order detail from latest orders list. | none | `apps/admin/components/Dashboard.tsx:131` |
| Retry dashboard read | action | Retries loading dashboard metrics after fetch failure. | GET /api/stores/[store]/tools/dashboard | `apps/admin/components/Dashboard.tsx:70` |
| Switch interface locale | navigation | Switches interface language in entry header. | none | `apps/admin/components/Entry.tsx:361` |
| Submit password login | form | Submits email and password to authenticate. | POST /api/auth/password/login | `apps/admin/components/PasswordAuth.tsx:154` |
| Verify login code | form | Submits emailed one-time code to complete login. | POST /api/auth/password/verify | `apps/admin/components/PasswordAuth.tsx:178` |
| Resend login verification code | action | Requests new verification code after cooldown. | POST /api/auth/password/login | `apps/admin/components/PasswordAuth.tsx:145` |
| Submit OIDC login | action | Submits OIDC authentication form post. | POST /api/auth/login | `apps/admin/components/Entry.tsx:414` |
| Submit initial store onboarding | form | Submits initial merchant, store, and warehouse setup. | POST /api/onboarding/initial-store | `apps/admin/components/Entry.tsx:244` |
| Sign out current session | action | Logs out merchant user session. | POST /api/auth/logout | `apps/admin/components/Entry.tsx:288` |

现存问题：
- **P2** Order ID link in dashboard slices string to 8 characters without showing full ID. (`apps/admin/components/Dashboard.tsx:131`)

### P32 `/[locale]/promotions` — Merchant discount promotion management for creating, editing, and toggling active status of coupon codes.
文件：`apps/admin/app/[locale]/promotions/page.tsx` · 角色：merchant owner, staff · 状态：空=present 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Create promotion code | form | Creates new discount promotion with limits and schedule. | POST /api/stores/[store]/promotions | `apps/admin/components/Promotions.tsx:123` |
| Update promotion code | form | Updates existing promotion details and limits with version check. | POST /api/stores/[store]/promotions/[id] | `apps/admin/components/Promotions.tsx:118` |
| Toggle promotion pause status | action | Pauses or resumes discount promotion code. | POST /api/stores/[store]/promotions/[id] | `apps/admin/components/Promotions.tsx:128` |
| Edit promotion in form | action | Populates form with promotion details for editing. | none | `apps/admin/components/Promotions.tsx:213` |
| Cancel promotion edit | action | Resets edit form back to blank create mode. | none | `apps/admin/components/Promotions.tsx:183` |
| Retry promotions read | action | Retries loading promotions list after network error. | GET /api/stores/[store]/promotions | `apps/admin/components/Promotions.tsx:56` |

现存问题：
- **P2** Promotion code input is disabled in edit mode, preventing code renaming. (`apps/admin/components/Promotions.tsx:143`)

### P33 `/[locale]/reset` — Password reset page for merchants and staff via email challenge and one-time verification code.
文件：`apps/admin/app/[locale]/reset/page.tsx` · 角色：merchant owner, staff, visitor · 状态：空=n/a 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Request password reset code | form | Submits email to request password reset code. | POST /api/auth/password/reset | `apps/admin/components/PasswordAuth.tsx:154` |
| Verify reset code and password | form | Submits 6-digit code and new password to complete reset. | POST /api/auth/password/verify | `apps/admin/components/PasswordAuth.tsx:178` |
| Resend reset code | action | Requests new verification code after cooldown timer. | POST /api/auth/password/reset | `apps/admin/components/PasswordAuth.tsx:336` |
| Switch back to email input | action | Returns from code step back to email step. | none | `apps/admin/components/PasswordAuth.tsx:353` |
| Navigate to sign in | navigation | Navigates back to merchant sign in page. | none | `apps/admin/components/PasswordAuth.tsx:384` |
| Switch interface locale | navigation | Switches interface language in reset header. | none | `apps/admin/components/Entry.tsx:361` |

现存问题：
- **P1** Throws 404 notFound() when password login is disabled instead of displaying an explanatory message. (`apps/admin/app/[locale]/reset/page.tsx:26`)

### P34 `/[locale]/signup` — Self-service merchant account signup flow with email verification code and password policy validation.
文件：`apps/admin/app/[locale]/signup/page.tsx` · 角色：prospective merchant, visitor · 状态：空=n/a 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Submit registration credentials | form | Submits email and password to initiate account registration. | POST /api/auth/password/signup | `apps/admin/components/PasswordAuth.tsx:154` |
| Verify registration code | form | Submits 6-digit code to complete registration and sign in. | POST /api/auth/password/verify | `apps/admin/components/PasswordAuth.tsx:178` |
| Resend registration code | action | Requests new verification code after cooldown countdown expires. | POST /api/auth/password/signup | `apps/admin/components/PasswordAuth.tsx:336` |
| Switch back to credentials form | action | Returns from code step back to credentials form. | none | `apps/admin/components/PasswordAuth.tsx:353` |
| Navigate to sign in | navigation | Navigates back to merchant sign in page. | none | `apps/admin/components/PasswordAuth.tsx:379` |
| Switch interface locale | navigation | Switches interface language in signup header. | none | `apps/admin/components/Entry.tsx:361` |

现存问题：
- **P1** Throws 404 notFound() when password login is disabled instead of displaying an explanatory message. (`apps/admin/app/[locale]/signup/page.tsx:26`)
- **P2** Account exists 409 resets form without pre-filled email or link to password reset. (`apps/admin/components/PasswordAuth.tsx:199`)

## 买家店铺 · 购物与法律

### P35 `/[locale]/cart` — Buyer cart page to review line items, adjust quantities, view shipping progress, and proceed to checkout.
文件：`apps/storefront/app/[locale]/cart/page.tsx` · 角色：buyer, guest · 状态：空=present 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Increase item quantity | action | Increments SKU quantity by one up to limit of 99. | PUT /api/buyer/cart | `apps/storefront/components/CartLines.tsx:155` |
| Decrease item quantity | action | Decrements SKU quantity by one down to limit of 1. | PUT /api/buyer/cart | `apps/storefront/components/CartLines.tsx:149` |
| Remove line item | action | Removes line item by updating its quantity to 0. | PUT /api/buyer/cart | `apps/storefront/components/CartLines.tsx:160` |
| Proceed to checkout | navigation | Navigates to checkout page unless blocked or busy. | none | `apps/storefront/components/CartLines.tsx:213` |
| Continue shopping (empty cart) | navigation | Navigates to product catalog from empty cart state. | none | `apps/storefront/components/CartView.tsx:24` |
| Continue shopping (summary) | navigation | Navigates to product catalog from summary sidebar. | none | `apps/storefront/components/CartView.tsx:34` |
| View product detail | navigation | Navigates to product detail page if slug is available. | none | `apps/storefront/components/CartLines.tsx:123` |
| Retry cart write | action | Resumes the same journalled idempotent cart mutation on uncertain outcome. | PUT /api/buyer/cart | `apps/storefront/components/CartLines.tsx:81` |
| Dismiss cart problem notice | action | Dismisses transient cart notification banner. | none | `apps/storefront/components/CartLines.tsx:90` |
| View existing order | navigation | Navigates to checkout order view when order is locked. | none | `apps/storefront/components/CartLines.tsx:58` |
| Start new cart | action | Resets buyer purchase context to begin a new cart. | none | `apps/storefront/components/CartLines.tsx:61` |

现存问题：
- **P1** Checkout Link has is-disabled class but lacks pointer-events:none, allowing click navigation when blocked or busy. (`apps/storefront/components/CartLines.tsx:213`)

### P36 `/[locale]/checkout` — Multi-step checkout for selecting delivery, obtaining quote, entering address or CVS pickup, applying discount, and paying.
文件：`apps/storefront/app/[locale]/checkout/page.tsx` · 角色：buyer, guest · 状态：空=present 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Toggle order history drawer | navigation | Opens or closes buyer order history panel. | none | `apps/storefront/components/CheckoutFlow.tsx:371` |
| Select delivery method | form | Selects shipping option and clears obsolete quote. | none | `apps/storefront/components/CheckoutFlow.tsx:537` |
| Load more delivery options | action | Fetches additional shipping options using pagination cursor. | GET /api/buyer/checkout-options | `apps/storefront/components/CheckoutFlow.tsx:572` |
| Request delivery quotation | action | Requests authoritative server quote for chosen delivery. | POST /api/buyer/quotes | `apps/storefront/components/CheckoutFlow.tsx:722` |
| Change delivery method | action | Resets current quote and reopens delivery selector. | none | `apps/storefront/components/CheckoutFlow.tsx:641` |
| Apply promo code | form | Submits discount code and recalculates quotation. | POST /api/buyer/quotes | `apps/storefront/components/PromoCode.tsx:73` |
| Remove promo code | action | Removes active promo code and re-quotes cart. | POST /api/buyer/quotes | `apps/storefront/components/PromoCode.tsx:85` |
| Confirm destination address | form | Validates and saves recipient address or CVS store. | PUT /api/buyer/destination | `apps/storefront/components/OrderFlow.tsx:380` |
| Place order | action | Creates new order with confirmed quotation and destination. | POST /api/buyer/checkout | `apps/storefront/components/OrderFlow.tsx:570` |
| Record checkout consents | form | Saves marketing and advertising consent preferences. | POST /api/buyer/consents | `apps/storefront/components/ConsentChoices.tsx:33` |
| Pay order via Stripe | action | Prepares Stripe payment attempt and handles redirect/popup. | POST /api/buyer/orders/{id}/payment/prepare | `apps/storefront/components/OrderPayment.tsx:213` |
| Cancel Stripe payment attempt | action | Cancels active Stripe payment attempt. | POST /api/buyer/orders/{id}/payment/cancel | `apps/storefront/components/OrderPayment.tsx:279` |
| Submit bank transfer proof | form | Submits bank transfer account last 5 digits and transferred amount. | PUT /api/buyer/orders/{id}/bank-transfer/proof | `apps/storefront/components/BankTransfer.tsx:156` |
| Open ECPay store map | action | Opens ECPay convenience store selection map. | POST /api/buyer/cvs-selections | `apps/storefront/components/CvsPickup.tsx:216` |
| Save buyer-entered CVS store | form | Submits manual convenience store code, name, and address. | POST /api/buyer/cvs-stores | `apps/storefront/components/CvsPickup.tsx:301` |
| Recover pending purchase | action | Resumes in-flight journalled checkout transaction. | POST /api/buyer/checkout | `apps/storefront/components/CheckoutFlow.tsx:392` |
| Retry failed load or reset session | action | Retries failed load or resets expired buyer session. | POST /api/buyer/session/reset | `apps/storefront/components/CheckoutFlow.tsx:427` |
| Continue shopping (post-order) | action | Clears active order locator and prepares new cart. | none | `apps/storefront/components/CheckoutFlow.tsx:487` |
| Refresh order status | action | Re-reads order status and fires payment refresh signal. | GET /api/buyer/orders/{id} | `apps/storefront/components/OrderFlow.tsx:830` |

现存问题：
- **P2** Disabled COD radio row is rendered as an unclickable disabled input with no action when CVS is selected. (`apps/storefront/components/OrderFlow.tsx:428`)
- **P2** Privacy policy notice link opens external target without rel='noreferrer'. (`apps/storefront/components/ConsentChoices.tsx:64`)

### P37 `/[locale]/claim` — Buyer keyword claim page to preview claimed live-stream items from one-time URL fragment and merge into cart for checkout.
文件：`apps/storefront/app/[locale]/claim/page.tsx` · 角色：buyer, guest · 状态：空=present 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch claim page language | filter | Changes locale while retaining in-memory token and path. | none | `apps/storefront/components/ClaimLink.tsx:271` |
| Preview claimed bundle | view | Fetches preview of claimed items via fragment token. | GET /api/buyer/claim-link | `apps/storefront/components/ClaimLink.tsx:95` |
| Fetch buyer cart | view | Reads current buyer cart to compare existing items with claim. | GET /api/buyer/cart | `apps/storefront/components/ClaimLink.tsx:97` |
| Redeem claim link | action | Merges claimed items into cart and redirects to checkout. | POST /api/buyer/claim-link/redeem | `apps/storefront/components/ClaimLink.tsx:170` |
| Remove cart item | action | Removes an item from cart to resolve cart line limits or conflict. | PUT /api/buyer/cart | `apps/storefront/components/ClaimLink.tsx:217` |
| Reload claim preview | action | Retries loading claim preview and cart on failure. | GET /api/buyer/claim-link | `apps/storefront/components/ClaimLink.tsx:296` |
| Renew expired buyer session | action | Resets buyer session and reloads claim link. | POST /api/buyer/session/reset | `apps/storefront/components/ClaimLink.tsx:232` |
| Review cart link | navigation | Navigates to cart page upon conflict or unavailable state. | none | `apps/storefront/components/ClaimLink.tsx:309` |

现存问题：
- **P1** Cart link uses raw anchor tag causing full page reload instead of Next.js Link. (`apps/storefront/components/ClaimLink.tsx:309`)
- **P1** Page refresh strips URL fragment and leaves token null, causing permanent not-found error without recovery. (`apps/storefront/components/ClaimLink.tsx:105`)
- **P2** Navigation to checkout uses window.location.assign instead of Next.js router. (`apps/storefront/components/ClaimLink.tsx:182`)

### P38 `/[locale]/collections/(list)` — Directory of active store collections displaying cover tiles, product counts, and navigation.
文件：`apps/storefront/app/[locale]/collections/(list)/page.tsx` · 角色：buyer, guest · 状态：空=present 加载=missing 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| List published collections | view | Renders active store collections list with product counts. | GET /v1/buyer/catalog/v2/collections | `apps/storefront/app/[locale]/collections/(list)/page.tsx:26` |
| Navigate to collection detail | navigation | Opens selected collection products page. | none | `apps/storefront/app/[locale]/collections/(list)/page.tsx:41` |
| Navigate to all products | navigation | Navigates to all products catalog from empty state. | none | `apps/storefront/app/[locale]/collections/(list)/page.tsx:33` |
| Navigate home | navigation | Navigates back to storefront home page via breadcrumb. | none | `apps/storefront/app/[locale]/collections/(list)/page.tsx:29` |

现存问题：
- **P2** Missing loading.tsx skeleton in collections/(list) route directory during SSR data fetching. (`apps/storefront/app/[locale]/collections/(list)/page.tsx:21`)

### P39 `/[locale]/collections/[slug]` — Collection product catalog view with collection chips, price and sort filters, product grid, and cursor pagination.
文件：`apps/storefront/app/[locale]/collections/[slug]/page.tsx` · 角色：buyer, guest · 状态：空=present 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Get collection detail | view | Fetches collection metadata and title by slug. | GET /v1/buyer/catalog/v2/collections/{slug} | `apps/storefront/app/[locale]/collections/[slug]/page.tsx:37` |
| Fetch collection products | view | Loads initial filtered product cards in collection. | GET /v1/buyer/catalog/v2/products | `apps/storefront/app/[locale]/collections/[slug]/page.tsx:38` |
| Filter by price range | form | Submits GET form with min and max price bounds. | none | `apps/storefront/components/ListControls.tsx:29` |
| Sort collection products | filter | Selects sorting order and auto-submits filter form. | none | `apps/storefront/components/ListControls.tsx:66` |
| Clear price filter | action | Clears price filter parameters and resets list. | none | `apps/storefront/components/ListControls.tsx:57` |
| Switch collection chip | navigation | Navigates to another collection via chip pills. | none | `apps/storefront/components/CollectionChips.tsx:20` |
| Load more products | action | Fetches next page of product cards via cursor. | GET /api/shop/products | `apps/storefront/components/ProductGrid.tsx:40` |
| Navigate to product detail | navigation | Navigates to product detail page from product card. | none | `apps/storefront/components/ProductCard.tsx:42` |
| Clear filters (empty state) | action | Resets active filters when no products match. | none | `apps/storefront/components/ListBody.tsx:63` |

现存问题：
- **P2** Clear filter action uses raw anchor tag causing full page reload instead of client navigation. (`apps/storefront/components/ListControls.tsx:57`)
- **P2** Missing loading.tsx skeleton in collections/[slug] route directory. (`apps/storefront/app/[locale]/collections/[slug]/page.tsx:31`)

### P40 `/[locale]/data-deletion` — Public buyer data deletion instruction page compliant with Meta platform App Review requirements.
文件：`apps/storefront/app/[locale]/data-deletion/page.tsx` · 角色：buyer, guest · 状态：空=missing 加载=missing 出错=missing 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch page language | filter | Navigates to data deletion page in selected locale. | none | `apps/storefront/components/LocaleSelect.tsx:16` |
| View data deletion instructions | view | Displays statutory and Meta app data deletion procedures. | none | `apps/storefront/app/[locale]/data-deletion/page.tsx:15` |
| Navigate to privacy page | navigation | Navigates to buyer self-service privacy portal. | none | `apps/storefront/app/[locale]/data-deletion/page.tsx:41` |

现存问题：
- **P1** Privacy link uses raw anchor tag causing full page reload instead of Next.js Link. (`apps/storefront/app/[locale]/data-deletion/page.tsx:41`)
- **P2** Language change uses window.location.assign instead of Next.js router navigation. (`apps/storefront/components/LocaleSelect.tsx:19`)

### P41 `/[locale]/legal/[slug]` — Static legal policy pages (privacy, terms, refunds, shipping, contact) displaying merchant compliance terms or unpublished notice.
文件：`apps/storefront/app/[locale]/legal/[slug]/page.tsx` · 角色：buyer, guest · 状态：空=present 加载=missing 出错=missing 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| View legal policy terms | view | Displays published legal policy clauses or contact address. | none | `apps/storefront/app/[locale]/legal/[slug]/page.tsx:48` |
| Return to home page | navigation | Navigates back to storefront home page from unpublished placeholder or breadcrumb. | none | `apps/storefront/app/[locale]/legal/[slug]/page.tsx:40` |

现存问题：
- **P2** Draft or pending policy terms render legalUnpublished fallback to buyers hiding policy content. (`apps/storefront/app/[locale]/legal/[slug]/page.tsx:34`)

### P42 `/[locale]/legal/anti-fraud` — Anti-fraud and consumer safety guidance page for Taiwan buyers referencing ATM precautions and 165 helpline.
文件：`apps/storefront/app/[locale]/legal/anti-fraud/page.tsx` · 角色：buyer, guest · 状态：空=missing 加载=missing 出错=missing 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| View anti-fraud guidance | view | Displays Taiwan consumer payment verification and fraud prevention guidance. | none | `apps/storefront/app/[locale]/legal/anti-fraud/page.tsx:17` |
| Navigate home | navigation | Navigates back to home page via breadcrumb link. | none | `apps/storefront/app/[locale]/legal/anti-fraud/page.tsx:18` |
| Open 165 anti-fraud portal | navigation | Navigates to external Taiwan National Police Agency 165 anti-fraud website. | none | `apps/storefront/app/[locale]/legal/anti-fraud/page.tsx:21` |

现存问题：
- **P2** External link to 165 portal opens in same tab without target blank, navigating buyer away from storefront. (`apps/storefront/app/[locale]/legal/anti-fraud/page.tsx:21`)

### P43 `/[locale]/order-link` — Manual order token redemption gateway that extracts URL hash fragment token, exchanges it for buyer capability, and navigates to order.
文件：`apps/storefront/app/[locale]/order-link/page.tsx` · 角色：buyer, guest · 状态：空=missing 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Exchange order link token | action | Exchanges in-memory token and proof for session cookie and redirects to order. | POST /api/buyer/orders/link | `apps/storefront/components/OrderLink.tsx:38` |
| Retry token exchange | action | Retries exchanging in-memory token following network failure. | POST /api/buyer/orders/link | `apps/storefront/components/OrderLink.tsx:89` |
| Navigate to order lookup | navigation | Navigates to manual order lookup page when link is invalid or expired. | none | `apps/storefront/components/OrderLink.tsx:83` |

现存问题：
- **P1** Order lookup link uses raw anchor tag instead of Next.js Link causing full page reload. (`apps/storefront/components/OrderLink.tsx:83`)
- **P1** Browser refresh clears fragment and leaves token null, permanently showing refused error without recovery. (`apps/storefront/components/OrderLink.tsx:35`)
- **P2** Redirect to order uses window.location.replace instead of Next.js router replace. (`apps/storefront/components/OrderLink.tsx:49`)

## 买家店铺 · 首页、商品与订单

### P44 `/[locale]/orders/[orderID]` — Buyer order details, status tracking, payment completion, and bank transfer proof submission
文件：`apps/storefront/app/[locale]/orders/[orderID]/page.tsx` · 角色：buyer, guest buyer · 状态：空=missing 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Reload order | action | Retries loading order after initial capability or network failure | GET /api/buyer/orders/{id} | `apps/storefront/app/[locale]/orders/[orderID]/OrderPage.tsx:54` |
| Navigate to guest lookup | navigation | Navigates visitor to order lookup page when order cannot load | none | `apps/storefront/app/[locale]/orders/[orderID]/OrderPage.tsx:55` |
| Refresh order status | action | Refreshes current order status, shipment tracking, and payment signals | GET /api/buyer/orders/{id} | `apps/storefront/components/OrderFlow.tsx:830` |
| Pay order | action | Initiates digital payment handoff for draft or recovering payment attempt | POST /api/buyer/orders/{id}/payment/prepare | `apps/storefront/components/OrderPayment.tsx:483` |
| Cancel payment attempt | action | Cancels active hosted payment attempt to unlock different payment actions | POST /api/buyer/orders/{id}/payment/cancel | `apps/storefront/components/OrderPayment.tsx:492` |
| Submit bank transfer proof | form | Submits last 5 account digits, amount, and paid-at timestamp for verification | PUT /api/buyer/orders/{id}/bank-transfer/proof | `apps/storefront/components/BankTransfer.tsx:142` |

现存问题：
- **P2** Lookup fallback link uses native anchor tag causing full page reload instead of router navigation (`apps/storefront/app/[locale]/orders/[orderID]/OrderPage.tsx:55`)
- **P2** Bank transfer section nests h3 heading directly inside dl description list element violating HTML specification (`apps/storefront/components/BankTransfer.tsx:194`)

### P45 `/[locale]/orders/lookup` — Guest order lookup by order number and buyer contact email or phone
文件：`apps/storefront/app/[locale]/orders/lookup/page.tsx` · 角色：buyer, guest buyer · 状态：空=missing 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Submit order lookup | form | Validates inputs and submits order reference and contact to locate order | POST /api/buyer/orders/lookup | `apps/storefront/app/[locale]/orders/lookup/LookupForm.tsx:28` |
| Navigate to found order | navigation | Redirects browser to order detail page upon successful order lookup | none | `apps/storefront/app/[locale]/orders/lookup/LookupForm.tsx:38` |

现存问题：
- **P2** Successful lookup uses window.location.assign triggering full page refresh instead of Next.js router push (`apps/storefront/app/[locale]/orders/lookup/LookupForm.tsx:38`)
- **P2** Page lacks navigation link to return to store home or product catalog (`apps/storefront/app/[locale]/orders/lookup/LookupForm.tsx:52`)

### P46 `/[locale]` — Storefront landing home page rendering published custom design sections
文件：`apps/storefront/app/[locale]/page.tsx` · 角色：buyer, guest buyer · 状态：空=present 加载=missing 出错=missing 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Click hero call to action | navigation | Navigates to configured target page, collection, or product from hero banner | none | `apps/storefront/components/HomeSections.tsx:34` |
| View all grid products | navigation | Navigates to full product catalog with matching sort query | none | `apps/storefront/components/HomeSections.tsx:66` |
| View all collection products | navigation | Navigates to dedicated collection page for featured collection section | none | `apps/storefront/components/HomeSections.tsx:88` |
| Scroll product rail left | action | Scrolls featured collection horizontal product rail to the left | none | `apps/storefront/components/Rail.tsx:40` |
| Scroll product rail right | action | Scrolls featured collection horizontal product rail to the right | none | `apps/storefront/components/Rail.tsx:43` |
| Navigate to product detail | navigation | Navigates to selected product detail page from product card link | none | `apps/storefront/components/ProductCard.tsx:35` |

现存问题：
- **P1** Upstream service failure throws unhandled error without localized in-page recovery UI (`apps/storefront/components/HomeSections.tsx:59`)
- **P2** Featured collection section silently drops out without fallback indicator when collection is missing or empty (`apps/storefront/components/HomeSections.tsx:83`)

### P47 `/[locale]/pages/[slug]` — Merchant static content page displaying markdown prose from store design document
文件：`apps/storefront/app/[locale]/pages/[slug]/page.tsx` · 角色：buyer, guest buyer · 状态：空=missing 加载=missing 出错=missing 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Navigate to home via breadcrumb | navigation | Navigates back to storefront home page from breadcrumb trail | none | `apps/storefront/components/PageHead.tsx:28` |
| Open markdown external link | navigation | Opens merchant-provided external HTTPS markdown link in browser | none | `apps/storefront/components/HomeSections.tsx:21` |

现存问题：
- **P2** Meta description generation strips markdown formatting with naive regex leaving whitespace and punctuation artifacts (`apps/storefront/app/[locale]/pages/[slug]/page.tsx:21`)
- **P2** Page with empty markdown body displays blank container without empty state fallback (`apps/storefront/app/[locale]/pages/[slug]/page.tsx:35`)

### P48 `/[locale]/privacy` — Buyer privacy self-service center for consents, data download, erasure, and policy notices
文件：`apps/storefront/app/[locale]/privacy/page.tsx` · 角色：buyer, guest buyer · 状态：空=missing 加载=present 出错=present 权限=present

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Switch privacy page language | action | Updates client locale and history URL state without reloading page | none | `apps/storefront/components/PrivacyCenter.tsx:248` |
| Reload privacy profile | action | Retries fetching buyer privacy record after request failure | GET /api/buyer/privacy | `apps/storefront/components/PrivacyCenter.tsx:260` |
| Renew buyer session | action | Resets buyer capability cookie and re-initializes privacy session | POST /api/buyer/session/reset | `apps/storefront/components/PrivacyCenter.tsx:266` |
| Toggle marketing or ad consent | action | Updates consent grant status for marketing messages or personalized ads | PUT /api/buyer/consents | `apps/storefront/components/PrivacyCenter.tsx:141` |
| Download personal data export | export | Exports buyer personal data JSON file and triggers browser download | POST /api/buyer/privacy/export | `apps/storefront/components/PrivacyCenter.tsx:162` |
| Open erasure modal dialog | action | Displays confirmation dialog requiring user to type ERASE | none | `apps/storefront/components/PrivacyCenter.tsx:195` |
| Cancel erasure modal | action | Dismisses account erasure confirmation dialog without action | none | `apps/storefront/components/PrivacyCenter.tsx:360` |
| Confirm buyer data erasure | action | Submits validated erasure command with idempotency key to delete personal data | POST /api/buyer/privacy/erasure | `apps/storefront/components/PrivacyCenter.tsx:207` |

现存问题：
- **P2** Language switcher uses window.history.replaceState leaving Next.js App Router route context out of sync (`apps/storefront/components/PrivacyCenter.tsx:248`)
- **P2** Erasure dialog applies hardcoded inline CSS style object instead of stylesheet utility classes (`apps/storefront/components/PrivacyCenter.tsx:339`)

### P49 `/[locale]/products/(list)` — Catalog all products listing with collection filters, price range, sorting, and pagination
文件：`apps/storefront/app/[locale]/products/(list)/page.tsx` · 角色：buyer, guest buyer · 状态：空=present 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Filter by collection chip | navigation | Navigates to selected collection catalog or back to all products | none | `apps/storefront/components/CollectionChips.tsx:20` |
| Apply price range filter | filter | Submits min and max price criteria to filter product list | none | `apps/storefront/components/ListControls.tsx:53` |
| Clear price filter | filter | Clears price filter inputs and reloads full product listing | none | `apps/storefront/components/ListControls.tsx:57` |
| Change sort order | filter | Auto-submits filter form to reorder products by chosen sort option | none | `apps/storefront/components/ListControls.tsx:66` |
| Load more products | action | Fetches next page of product cards by cursor and appends to grid | GET /api/shop/products | `apps/storefront/components/ProductGrid.tsx:40` |
| Navigate to product detail | navigation | Navigates to clicked product detail page from product card | none | `apps/storefront/components/ProductCard.tsx:35` |

现存问题：
- **P2** Clear filter action uses native anchor tag instead of Next.js Link causing full page refresh (`apps/storefront/components/ListControls.tsx:57`)
- **P2** Price filter input fields lack numeric step and min constraints allowing negative or malformed values (`apps/storefront/components/ListControls.tsx:44`)

### P50 `/[locale]/products/[slug]` — Product detail page with photo gallery, variant selection, price/stock display, cart actions, and assurance
文件：`apps/storefront/app/[locale]/products/[slug]/page.tsx` · 角色：buyer, guest buyer · 状态：空=missing 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Select product option value | action | Updates option selection to resolve corresponding SKU variant and gallery photo | none | `apps/storefront/components/ProductBuy.tsx:59` |
| Step quantity down | action | Decrements product order quantity down to minimum of 1 | none | `apps/storefront/components/ProductBuy.tsx:130` |
| Step quantity up | action | Increments product order quantity up to maximum of 99 | none | `apps/storefront/components/ProductBuy.tsx:134` |
| Add product to cart | action | Merges selected variant and quantity into buyer cart and opens cart drawer | PUT /api/buyer/cart | `apps/storefront/components/ProductBuy.tsx:71` |
| Buy product now | action | Adds selected variant to cart and navigates directly to checkout page | PUT /api/buyer/cart | `apps/storefront/components/ProductBuy.tsx:73` |
| Open photo zoom modal | action | Opens high-resolution photo viewer dialog for detailed image inspection | none | `apps/storefront/components/ProductGallery.tsx:115` |
| Navigate gallery photos | action | Switches active gallery photo via thumbnail click or keyboard arrow navigation | none | `apps/storefront/components/ProductGallery.tsx:70` |
| Scroll related products rail | action | Scrolls related products horizontal rail left or right | none | `apps/storefront/components/Rail.tsx:34` |

现存问题：
- **P2** Option pick silently ignores clicks on sold out combinations without presenting user feedback (`apps/storefront/components/ProductBuy.tsx:62`)
- **P2** Quantity manual text entry allows entering arbitrary strings before blur event clamps value (`apps/storefront/components/ProductBuy.tsx:133`)

### P51 `/[locale]/search` — Catalog search page with text query search bar, price filters, sort selector, and product grid
文件：`apps/storefront/app/[locale]/search/page.tsx` · 角色：buyer, guest buyer · 状态：空=present 加载=present 出错=present 权限=n/a

| 功能 | 类型 | 作用 | 接口 | 位置 |
|---|---|---|---|---|
| Submit catalog search | form | Submits keyword query to filter catalog products by title, description, or SKU | none | `apps/storefront/app/[locale]/search/page.tsx:33` |
| Apply search price filter | filter | Submits min and max price constraints on searched products | none | `apps/storefront/components/ListControls.tsx:53` |
| Clear search price filter | filter | Resets price filter while preserving active search query string | none | `apps/storefront/components/ListControls.tsx:57` |
| Change search sort order | filter | Auto-submits search form to change result ordering | none | `apps/storefront/components/ListControls.tsx:66` |
| Load more search results | action | Fetches next page of matching products by cursor pagination and appends to grid | GET /api/shop/products | `apps/storefront/components/ProductGrid.tsx:40` |
| Navigate to product detail | navigation | Navigates to clicked product detail page from search card | none | `apps/storefront/components/ProductCard.tsx:35` |

现存问题：
- **P2** Search input uses autoFocus when empty which can unexpectedly displace mobile keyboard focus (`apps/storefront/app/[locale]/search/page.tsx:37`)
- **P2** Search query input hardcodes maxLength 60 without displaying character limit feedback to user (`apps/storefront/app/[locale]/search/page.tsx:37`)
