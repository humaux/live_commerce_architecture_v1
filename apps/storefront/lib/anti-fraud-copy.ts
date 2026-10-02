// General safety guidance, NOT a merchant policy, refund guarantee or payment instruction.
// Taiwan contact verified 2026-10-02: https://165.npa.gov.tw/ ; original policy approval gates stay unchanged.
import type { Locale } from "@live-commerce/i18n";

export const antiFraudCopy: Record<Locale, { intro: string; source: string; sections: { heading: string; text: string }[] }> = {
  en: {
    intro: "Pause and verify if a payment request does not match your order.", source: "Taiwan National Police Agency — 165 anti-fraud service",
    sections: [
      { heading: "Check before you pay", text: "Open the shop from a bookmark or a known address. Check the order number, amount and payee in your order before transferring. If a message asks you to pay a different account, stop and contact the shop through its published contact details." },
      { heading: "Protect your account", text: "Do not give anyone your password, full card details or one-time verification codes. Do not install remote-control software or operate an ATM because a caller claims your order, instalment or refund needs fixing." },
      { heading: "If something looks wrong", text: "Stop the payment and keep the message and transaction records. Contact the shop using its published contact details. If you have already paid, contact your bank or payment provider promptly. In Taiwan, call 165 for anti-fraud assistance." },
    ],
  },
  "zh-TW": {
    intro: "付款要求與訂單不符時，請先停下來核實。", source: "內政部警政署 — 165 全民防騙網",
    sections: [
      { heading: "付款前先核對", text: "請從書籤或已確認的網址進入商店。轉帳前，在訂單中核對訂單編號、金額及收款人。如收到要求改付其他帳戶的訊息，請先停止付款，透過商店公開的聯絡方式核實。" },
      { heading: "保護帳戶資訊", text: "請勿向他人提供密碼、完整信用卡資料或一次性驗證碼。不要因來電聲稱訂單、分期或退款需要處理，就安裝遠端控制軟體或操作 ATM。" },
      { heading: "遇到可疑情況", text: "先停止付款，保留訊息與交易紀錄，透過商店公開的聯絡方式求證。若已付款，請儘速聯絡銀行或付款服務商。在台灣可撥打 165 反詐騙諮詢專線。" },
    ],
  },
  "zh-CN": {
    intro: "付款要求与订单不符时，请先停下来核实。", source: "台湾警政署 — 165 反诈骗服务",
    sections: [
      { heading: "付款前先核对", text: "请从书签或已确认的网址进入商店。转账前，在订单中核对订单编号、金额及收款人。如收到要求改付其他账户的消息，请先停止付款，通过商店公开的联系方式核实。" },
      { heading: "保护账户信息", text: "请勿向他人提供密码、完整信用卡资料或一次性验证码。不要因来电声称订单、分期或退款需要处理，就安装远程控制软件或操作 ATM。" },
      { heading: "遇到可疑情况", text: "先停止付款，保留消息与交易记录，通过商店公开的联系方式求证。若已付款，请尽快联系银行或付款服务商。在台湾可拨打 165 反诈骗咨询专线。" },
    ],
  },
};
