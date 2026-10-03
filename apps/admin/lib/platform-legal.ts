import type { PlatformLocale } from "./company";

// Describes implemented data flows, not a compliance certification or deletion SLA.
// Before launch: owner/legal review; evidence and primary references in the unit SUMMARY.
export type LegalSection = readonly [title: string, text: string];
type Documents = Record<
  "privacy" | "terms" | "data-deletion" | "contact",
  readonly LegalSection[]
>;
export const platformLegal: Record<PlatformLocale, Documents> = {
  "zh-TW": {
    privacy: [
      [
        "我們處理的資料",
        "商家註冊及使用服務時，我們處理帳戶電郵、登入驗證、店鋪設定、員工權限及操作紀錄，以提供和保護服務。買家向商家購物時，系統代商家處理姓名、聯絡方式、配送資料、訂單及付款狀態；商家負責其交易及買家資料的使用目的。",
      ],
      [
        "Meta 連接與使用目的",
        "商家明確授權並連接後，我們按已授予的權限處理專頁、留言、直播影片、廣告帳戶及洞察資料，用於留言收單、訂單跟進和商家啟用的廣告功能。我們不要求 Facebook 或 Instagram 密碼，不出售 Meta 資料。轉換事件僅按商家啟用的功能及買家同意處理。",
      ],
      [
        "服務提供者與跨境處理",
        "履行商家交易所需的資料可能交由其選用的支付、物流及 Meta 等服務提供者處理。託管及服務提供者可能在香港以外處理資料；第三方各自的隱私政策亦適用。平台不儲存完整信用卡資料。",
      ],
      [
        "Cookie 與安全",
        "登入、請求驗證及語言選擇使用必要的 cookie 或本機儲存。此平台官网不安裝 Meta Pixel 或分析追蹤器。我們以店鋪權限隔離、存取控制及加密憑據保護資料；沒有任何系統能保證零風險。",
      ],
      [
        "保留與刪除",
        "資料按提供服務、交易對帳、安全稽核及適用法定要求保留。斷開 Meta 不等於刪除已建立的訂單或所有歷史紀錄。刪除請求經核實後按資料類型處理；尚待對帳、爭議、依法須保留的紀錄或備份可能需要保留。我們會說明處理範圍及預計時間，不承諾即時或固定天數刪除。",
      ],
      [
        "你的選擇與聯絡",
        "你可使用下方電郵要求查閱、更正或刪除與你有關的資料，或撤回適用的同意。買家交易資料請先聯絡相關商家；我們會協助確認責任方。停止連接及刪除請求的步驟見「資料刪除」。",
      ],
    ],
    terms: [
      [
        "服務與營運者",
        "本服務提供商家網店、商品與庫存、Facebook 留言收單、訂單管理及按設定啟用的收款、物流和廣告工作流程。營運公司及登記資料列於本頁。功能受帳戶權限、商家設定及第三方審核限制；本網站的流程圖僅為產品示意。",
      ],
      [
        "商家責任",
        "商家須有權使用所連接的帳戶、專頁、商品及內容，保護員工帳戶，提供準確的商品、價格、配送與退貨資訊，並依法處理買家資料和取得必要同意。不得以本服務詐騙、發送未經同意的行銷或存取其他商家的資料。",
      ],
      [
        "交易與第三方",
        "買賣交易由商家與買家訂立，商家負責商品、履約、售後及退款。第三方支付、物流和 Meta 的條款、資格及費用由各方另行確定；在介面啟用功能不等於第三方已核准或款項已到帳。手工出貨紀錄不代表平台已向物流商下單。",
      ],
      [
        "可用性與變更",
        "外部服務可能拒絕、延遲或中斷。我們不保證銷售成效、廣告審核結果或不中斷運作。對未確認的付款或外部操作，商家應先核對紀錄，不應重複建立。帳戶停用及資料請求請透過本頁電郵聯絡；交易及法定保留事項可能在停止使用後繼續處理。",
      ],
      [
        "聯絡及爭議",
        "如對服務、帳戶或條款有疑問，請聯絡下方營運公司。這些條款不排除適用法律下不得排除的權利或責任。",
      ],
    ],
    "data-deletion": [
      [
        "1. 停止 Meta 連接",
        "商家可登入後台，在「設定 → Facebook」找到已連接的專頁，按「斷開」並確認。這會停止該專頁的新留言收單；第三方取消訂閱仍可能需要處理。你也可在 Meta 的應用程式或商務整合設定撤銷授權。斷開不會自動刪除既有訂單。",
      ],
      [
        "2. 提交刪除請求",
        "寄信至本頁聯絡電郵，主旨註明「資料刪除請求」，提供帳戶電郵、店鋪編號或相關專頁識別資訊，以及希望刪除的資料範圍。不要提供密碼、API 密鑰或完整信用卡號。我們可能要求必要的身分及權限證明。買家可先向交易商家提出請求。",
      ],
      [
        "3. 核實與處理",
        "核實後，我們會告知可刪除的帳戶或連接資料、處理方式及預計時間。交易、對帳、爭議、安全稽核或依法須保留的資料可能不會立即刪除；備份按其生命週期處理。本頁提供人工申請流程，不代表已有自動資料刪除回呼或固定處理時限。",
      ],
    ],
    contact: [
      [
        "服務與資料請求",
        "商家帳戶、服務問題、隱私或資料刪除請求，請使用下方電郵聯絡營運公司。請描述問題及店鋪編號，不要寄送密碼、API 密鑰或完整付款資料。",
      ],
      [
        "買家訂單",
        "商品、配送、退貨及退款問題，請優先聯絡你購物的商家。此處為平台營運公司聯絡方式，並非所有商家的客服或退貨地址。",
      ],
    ],
  },
  "zh-CN": {
    privacy: [
      [
        "我们处理的资料",
        "商家注册及使用服务时，我们处理账户邮箱、登录验证、店铺设置、员工权限及操作记录，以提供和保护服务。买家向商家购物时，系统代商家处理姓名、联系方式、配送资料、订单及付款状态；商家负责其交易及买家资料的使用目的。",
      ],
      [
        "Meta 连接与使用目的",
        "商家明确授权并连接后，我们按已授予的权限处理主页、留言、直播视频、广告账户及洞察资料，用于留言收单、订单跟进和商家启用的广告功能。我们不要求 Facebook 或 Instagram 密码，不出售 Meta 资料。转化事件仅按商家启用的功能及买家同意处理。",
      ],
      [
        "服务提供者与跨境处理",
        "履行商家交易所需的资料可能交由其选用的支付、物流及 Meta 等服务提供者处理。托管及服务提供者可能在香港以外处理资料；第三方各自的隐私政策亦适用。平台不存储完整信用卡资料。",
      ],
      [
        "Cookie 与安全",
        "登录、请求验证及语言选择使用必要的 cookie 或本地存储。此平台官网不安装 Meta Pixel 或分析追踪器。我们以店铺权限隔离、访问控制及加密凭据保护资料；没有任何系统能保证零风险。",
      ],
      [
        "保留与删除",
        "资料按提供服务、交易对账、安全审计及适用法定要求保留。断开 Meta 不等于删除已创建的订单或所有历史记录。删除请求经核实后按资料类型处理；尚待对账、争议、依法须保留的记录或备份可能需要保留。我们会说明处理范围及预计时间，不承诺即时或固定天数删除。",
      ],
      [
        "你的选择与联系",
        "你可使用下方邮箱要求查阅、更正或删除与你有关的资料，或撤回适用的同意。买家交易资料请先联系相关商家；我们会协助确认责任方。停止连接及删除请求的步骤见「资料删除」。",
      ],
    ],
    terms: [
      [
        "服务与运营者",
        "本服务提供商家网店、商品与库存、Facebook 留言收单、订单管理及按设置启用的收款、物流和广告工作流程。运营公司及登记资料列于本页。功能受账户权限、商家设置及第三方审核限制；本网站的流程图仅为产品示意。",
      ],
      [
        "商家责任",
        "商家须有权使用所连接的账户、主页、商品及内容，保护员工账户，提供准确的商品、价格、配送与退货信息，并依法处理买家资料和取得必要同意。不得以本服务诈骗、发送未经同意的营销或访问其他商家的资料。",
      ],
      [
        "交易与第三方",
        "买卖交易由商家与买家订立，商家负责商品、履约、售后及退款。第三方支付、物流和 Meta 的条款、资格及费用由各方另行确定；在界面启用功能不等于第三方已批准或款项已到账。手工发货记录不代表平台已向物流商下单。",
      ],
      [
        "可用性与变更",
        "外部服务可能拒绝、延迟或中断。我们不保证销售成效、广告审核结果或不中断运作。对未确认的付款或外部操作，商家应先核对记录，不应重复创建。账户停用及资料请求请通过本页邮箱联系；交易及法定保留事项可能在停止使用后继续处理。",
      ],
      [
        "联系及争议",
        "如对服务、账户或条款有疑问，请联系下方运营公司。这些条款不排除适用法律下不得排除的权利或责任。",
      ],
    ],
    "data-deletion": [
      [
        "1. 停止 Meta 连接",
        "商家可登录后台，在「设置 → Facebook」找到已连接的主页，按「断开」并确认。这会停止该主页的新留言收单；第三方取消订阅仍可能需要处理。你也可在 Meta 的应用或商务集成设置撤销授权。断开不会自动删除现有订单。",
      ],
      [
        "2. 提交删除请求",
        "发信至本页联系邮箱，主题注明「资料删除请求」，提供账户邮箱、店铺编号或相关主页识别信息，以及希望删除的资料范围。不要提供密码、API 密钥或完整信用卡号。我们可能要求必要的身份及权限证明。买家可先向交易商家提出请求。",
      ],
      [
        "3. 核实与处理",
        "核实后，我们会告知可删除的账户或连接资料、处理方式及预计时间。交易、对账、争议、安全审计或依法须保留的资料可能不会立即删除；备份按其生命周期处理。本页提供人工申请流程，不代表已有自动资料删除回调或固定处理时限。",
      ],
    ],
    contact: [
      [
        "服务与资料请求",
        "商家账户、服务问题、隐私或资料删除请求，请使用下方邮箱联系运营公司。请描述问题及店铺编号，不要发送密码、API 密钥或完整付款资料。",
      ],
      [
        "买家订单",
        "商品、配送、退货及退款问题，请优先联系你购物的商家。此处为平台运营公司联系方式，并非所有商家的客服或退货地址。",
      ],
    ],
  },
  en: {
    privacy: [
      [
        "Information we process",
        "We process merchant account email, sign-in verification, store settings, staff permissions and operational records to provide and protect the service. For merchant purchases, we process buyer names, contact and delivery details, orders and payment status on the merchant’s behalf. Merchants determine the purposes of their transactions and buyer-data use.",
      ],
      [
        "Meta connections and purposes",
        "After a merchant explicitly connects and authorises an account, we process the permitted Pages, comments, live videos, ad accounts and insights for comment ordering, order follow-up and enabled advertising features. We do not request Facebook or Instagram passwords or sell Meta data. Conversion events are processed only for enabled features and with buyer consent.",
      ],
      [
        "Providers and cross-border processing",
        "Information needed to fulfil a merchant transaction may be processed by its selected payment, logistics and Meta service providers. Hosting and service providers may process information outside Hong Kong; their own privacy policies also apply. The platform does not store complete credit-card details.",
      ],
      [
        "Cookies and security",
        "Sign-in, request verification and language preferences use necessary cookies or local storage. This public platform website does not install Meta Pixel or analytics trackers. Store-level permissions, access controls and encrypted credentials protect information; no system can guarantee zero risk.",
      ],
      [
        "Retention and deletion",
        "We retain records for service delivery, transaction reconciliation, security audits and applicable legal requirements. Disconnecting Meta does not delete existing orders or all historical records. Verified deletion requests are handled by data type; unresolved transactions, disputes, required records or backups may need to be retained. We explain the scope and expected timing, rather than promising immediate deletion or a fixed number of days.",
      ],
      [
        "Your choices and contact",
        "Use the email below to request access, correction or deletion of your information, or withdraw applicable consent. For buyer transaction information, contact the merchant first; we can help identify the responsible party. See Data deletion for disconnection and request instructions.",
      ],
    ],
    terms: [
      [
        "Service and operator",
        "The service provides merchant storefronts, products and inventory, Facebook comment ordering, order management and configured payment, logistics and advertising workflows. The operator’s registered details appear on this page. Features depend on account permissions, merchant setup and third-party approval. Website workflow diagrams are product illustrations only.",
      ],
      [
        "Merchant responsibilities",
        "Merchants must be authorised to use connected accounts, Pages, products and content, protect staff accounts, provide accurate product, pricing, shipping and returns information, and obtain necessary consent for lawful buyer-data processing. Fraud, unsolicited marketing and accessing other merchants’ information are prohibited.",
      ],
      [
        "Transactions and third parties",
        "Sales are between merchants and buyers. Merchants are responsible for products, fulfilment, after-sales service and refunds. Payment, logistics and Meta providers set their own terms, eligibility and fees. Enabling a feature does not establish third-party approval or receipt of funds. A manual shipment record does not mean the platform has placed a carrier order.",
      ],
      [
        "Availability and changes",
        "External services may reject, delay or interrupt operations. We do not guarantee sales results, ad approval or uninterrupted service. Merchants should reconcile unconfirmed payments or external actions before repeating them. Contact us about account closure or data requests; transaction and legally required retention matters may continue after use ends.",
      ],
      [
        "Questions and disputes",
        "Contact the operator below about the service, accounts or these terms. These terms do not exclude rights or liabilities that cannot be excluded under applicable law.",
      ],
    ],
    "data-deletion": [
      [
        "1. Disconnect Meta",
        "Sign in to the merchant admin and open Settings → Facebook. Find the connected Page, select Disconnect and confirm. This stops new comment intake for that Page; third-party unsubscription may still need processing. You can also revoke access in Meta’s apps or business integrations settings. Disconnecting does not automatically delete existing orders.",
      ],
      [
        "2. Request deletion",
        "Email the contact below with the subject ‘Data deletion request’. Include your account email, store number or relevant Page identifier and the data you want deleted. Do not send passwords, API secrets or complete card numbers. We may request necessary proof of identity and authority. Buyers can first submit their request to the merchant.",
      ],
      [
        "3. Verification and handling",
        "After verification, we explain which account or connection data can be deleted, how and the expected timing. Transaction, reconciliation, dispute, security-audit or legally required records may not be deleted immediately; backups follow their lifecycle. This is a manual request process, not an automated deletion callback or a fixed processing deadline.",
      ],
    ],
    contact: [
      [
        "Service and data requests",
        "Contact the operator using the email below for merchant account, service, privacy or data-deletion requests. Describe the issue and store number; do not send passwords, API secrets or complete payment information.",
      ],
      [
        "Buyer orders",
        "For products, delivery, returns or refunds, contact the merchant you purchased from first. This is the platform operator’s contact information, not every merchant’s customer service or returns address.",
      ],
    ],
  },
};
