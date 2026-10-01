package notify

// render.go is the pure renderer: claimed Payload in, one mail.Message per recipient out. No clock, no network, no database. Copy
// exists for zh-TW (default), zh-CN and en; every dynamic value is HTML-escaped in the HTML part and control characters are stripped
// before it reaches a header (the mail adapter refuses CR/LF in a Subject rather than sanitising it).

import (
	"html"
	"strconv"
	"strings"
	"time"

	"livecommerce/internal/mail"
)

// Kinds are the notify.outbox.kind values (migration 0090).
const (
	KindPlaced      = "placed"
	KindPaid        = "paid"
	KindShipped     = "shipped"
	KindCancelled   = "cancelled"
	KindRefunded    = "refunded"
	KindMerchantNew = "merchant_new"
)

// DefaultLocale is used when the order carries no known locale (today: always, checkout does not capture it yet).
const DefaultLocale = "zh-TW"

// Payload is the jsonb notify.claim_batch returns for one send (one buyer mail, or one merchant batch). Pointer fields are NULL in SQL.
type Payload struct {
	BatchID     string    `json:"batch_id"`
	Kind        string    `json:"kind"`
	OrderID     string    `json:"order_id"`
	To          []string  `json:"to"`
	StoreName   string    `json:"store_name"`
	Locale      *string   `json:"locale"`
	Origin      *string   `json:"origin"`
	TotalMinor  int64     `json:"total_minor"`
	Currency    string    `json:"currency"`
	PaymentMode string    `json:"payment_mode"`
	ExpiresAt   time.Time `json:"expires_at"`
	Bank        *Bank     `json:"bank"`
	Pickup      *Pickup   `json:"pickup"`
	Shipment    *Shipment `json:"shipment"`
	CVS         *CVSInfo  `json:"cvs"`
	Count       int       `json:"count"`
	OrderIDs    []string  `json:"order_ids"`
}

// Bank is the order's own bank snapshot (checkout.bank_transfers), shown only in the buyer's own placed mail.
type Bank struct {
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
}

// Pickup is the CVS store of the order's destination.
type Pickup struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Shipment is the merchant-arranged shipment head (carrier + tracking).
type Shipment struct {
	CarrierCode    string  `json:"carrier_code"`
	CarrierName    *string `json:"carrier_name"`
	TrackingNumber string  `json:"tracking_number"`
	TrackingURL    *string `json:"tracking_url"`
}

// CVSInfo is the ECPay CVS shipment's shipping number.
type CVSInfo struct {
	ShippingNo *string `json:"shipping_no"`
}

type copyText struct {
	subject map[string]string // kind -> subject format; %s = store name
	intro   map[string]string // kind -> first line
	lbl     struct{ order, total, bank, branch, account, number, deadline, pickup, carrier, tracking, link, view, auto, newOrders, orders string }
}

var carrierNames = map[string]map[string]string{
	"zh-TW": {"seven_eleven_cvs": "7-ELEVEN 超商", "familymart_cvs": "全家超商", "hilife_cvs": "萊爾富超商", "okmart_cvs": "OK 超商", "sf_express": "順豐速運", "chunghwa_post": "中華郵政"},
	"zh-CN": {"seven_eleven_cvs": "7-ELEVEN 便利店", "familymart_cvs": "全家便利店", "hilife_cvs": "莱尔富便利店", "okmart_cvs": "OK 便利店", "sf_express": "顺丰速运", "chunghwa_post": "中华邮政"},
	"en":    {"seven_eleven_cvs": "7-ELEVEN", "familymart_cvs": "FamilyMart", "hilife_cvs": "Hi-Life", "okmart_cvs": "OK Mart", "sf_express": "SF Express", "chunghwa_post": "Chunghwa Post"},
}

var copies = map[string]copyText{
	"zh-TW": func() copyText {
		c := copyText{
			subject: map[string]string{KindPlaced: "【%s】訂單已成立", KindPaid: "【%s】已確認收到付款", KindShipped: "【%s】您的訂單已出貨", KindCancelled: "【%s】訂單已取消", KindRefunded: "【%s】訂單已退款"},
			intro: map[string]string{KindPlaced: "感謝您的訂購，我們已收到您的訂單。", KindPaid: "我們已確認收到您的付款，將盡快為您安排出貨。",
				KindShipped: "您的訂單已出貨。", KindCancelled: "您的訂單已取消（含逾期未付款），保留的商品已釋出。", KindRefunded: "您的訂單已完成退款，實際入帳時間依發卡行或銀行而定。"},
		}
		c.lbl.order, c.lbl.total, c.lbl.bank, c.lbl.branch, c.lbl.account, c.lbl.number = "訂單編號", "訂單金額", "匯款銀行", "分行", "戶名", "帳號"
		c.lbl.deadline, c.lbl.pickup, c.lbl.carrier, c.lbl.tracking, c.lbl.link, c.lbl.view = "請於此時間前完成匯款", "取貨門市", "物流", "追蹤編號", "追蹤連結", "查看訂單"
		c.lbl.auto, c.lbl.newOrders, c.lbl.orders = "這是系統自動寄出的訂單通知，請勿直接回覆。", "新訂單通知", "訂單編號"
		return c
	}(),
	"zh-CN": func() copyText {
		c := copyText{
			subject: map[string]string{KindPlaced: "【%s】订单已成立", KindPaid: "【%s】已确认收到付款", KindShipped: "【%s】您的订单已发货", KindCancelled: "【%s】订单已取消", KindRefunded: "【%s】订单已退款"},
			intro: map[string]string{KindPlaced: "感谢您的订购，我们已收到您的订单。", KindPaid: "我们已确认收到您的付款，将尽快为您安排发货。",
				KindShipped: "您的订单已发货。", KindCancelled: "您的订单已取消（含逾期未付款），保留的商品已释放。", KindRefunded: "您的订单已完成退款，实际到账时间取决于发卡行或银行。"},
		}
		c.lbl.order, c.lbl.total, c.lbl.bank, c.lbl.branch, c.lbl.account, c.lbl.number = "订单编号", "订单金额", "汇款银行", "分行", "户名", "账号"
		c.lbl.deadline, c.lbl.pickup, c.lbl.carrier, c.lbl.tracking, c.lbl.link, c.lbl.view = "请在此时间前完成汇款", "取货门店", "物流", "追踪编号", "追踪链接", "查看订单"
		c.lbl.auto, c.lbl.newOrders, c.lbl.orders = "这是系统自动发送的订单通知，请勿直接回复。", "新订单通知", "订单编号"
		return c
	}(),
	"en": func() copyText {
		c := copyText{
			subject: map[string]string{KindPlaced: "[%s] Your order is placed", KindPaid: "[%s] Payment confirmed", KindShipped: "[%s] Your order has shipped", KindCancelled: "[%s] Your order was cancelled", KindRefunded: "[%s] Your order was refunded"},
			intro: map[string]string{KindPlaced: "Thank you for your order. We have received it.", KindPaid: "We have confirmed your payment and will arrange shipping soon.",
				KindShipped: "Your order has shipped.", KindCancelled: "Your order was cancelled (including a missed payment deadline) and the reserved items were released.",
				KindRefunded: "Your order has been refunded. When the money arrives depends on your card issuer or bank."},
		}
		c.lbl.order, c.lbl.total, c.lbl.bank, c.lbl.branch, c.lbl.account, c.lbl.number = "Order number", "Order total", "Bank", "Branch", "Account name", "Account number"
		c.lbl.deadline, c.lbl.pickup, c.lbl.carrier, c.lbl.tracking, c.lbl.link, c.lbl.view = "Please transfer before", "Pickup store", "Carrier", "Tracking number", "Tracking link", "View your order"
		c.lbl.auto, c.lbl.newOrders, c.lbl.orders = "This is an automatic order notification; please do not reply.", "New orders", "Order numbers"
		return c
	}(),
}

// OrderNumber is the buyer-facing order number: the first 12 hex digits of the order id, uppercase, as XXXX-XXXX-XXXX (contract §E4).
// It returns "" for an id that is not a canonical uuid text.
func OrderNumber(id string) string {
	h := strings.ReplaceAll(id, "-", "")
	if len(h) != 32 || strings.Trim(h, "0123456789abcdefABCDEF") != "" {
		return ""
	}
	h = strings.ToUpper(h[:12])
	return h[0:4] + "-" + h[4:8] + "-" + h[8:12]
}

var zeroDecimal = map[string]bool{"JPY": true, "KRW": true, "VND": true, "CLP": true, "ISK": true, "UGX": true, "PYG": true, "XAF": true, "XOF": true}

// Money formats minor units like the storefront does (the UI divides by 10^fraction digits; TWD has two): "TWD 1,234.50".
func Money(minor int64, currency string) string {
	exp := 2
	if zeroDecimal[currency] {
		exp = 0
	}
	neg := minor < 0
	if neg {
		minor = -minor
	}
	div := int64(1)
	for i := 0; i < exp; i++ {
		div *= 10
	}
	whole, frac := minor/div, minor%div
	digits := strconv.FormatInt(whole, 10)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if exp > 0 {
		f := strconv.FormatInt(frac, 10)
		out += "." + strings.Repeat("0", exp-len(f)) + f
	}
	if neg {
		out = "-" + out
	}
	return currency + " " + out
}

// clean strips control characters (header and line-break safety) and caps the length of a merchant-typed value.
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return strings.TrimSpace(s)
}

func localeOf(p Payload) string {
	if p.Locale != nil {
		if _, ok := copies[*p.Locale]; ok {
			return *p.Locale
		}
	}
	return DefaultLocale
}

// orderLink is the buyer's order page on the store's own published origin; "" when the store has none (the mail still goes).
func orderLink(p Payload, locale string) string {
	if p.Origin == nil || !strings.HasPrefix(*p.Origin, "https://") || strings.ContainsAny(*p.Origin, " \r\n\"<>") {
		return ""
	}
	return *p.Origin + "/" + locale + "/orders/" + p.OrderID
}

type line struct{ label, value string }

// Render builds one message per recipient of p (a buyer mail has one; a merchant batch one per owner address). It returns nil for an
// unknown kind or a payload the renderer cannot make a coherent mail from (the worker records that as FAILED once and moves on).
func Render(p Payload) []mail.Message {
	loc := localeOf(p)
	c := copies[loc]
	store := clean(p.StoreName, 120)
	var subject, intro string
	var lines []line
	var link, linkLabel string
	if p.Kind == KindMerchantNew {
		if p.Count < 1 || len(p.To) == 0 {
			return nil
		}
		subject = "【" + store + "】" + c.lbl.newOrders + " (" + strconv.Itoa(p.Count) + ")"
		if loc == "en" {
			subject = "[" + store + "] " + c.lbl.newOrders + " (" + strconv.Itoa(p.Count) + ")"
		}
		intro = c.lbl.newOrders + ": " + strconv.Itoa(p.Count)
		nos := make([]string, 0, len(p.OrderIDs))
		for _, id := range p.OrderIDs {
			if n := OrderNumber(id); n != "" {
				nos = append(nos, n)
			}
		}
		if len(nos) > 0 {
			lines = append(lines, line{c.lbl.orders, strings.Join(nos, ", ")})
		}
	} else {
		format, ok := c.subject[p.Kind]
		no := OrderNumber(p.OrderID)
		if !ok || no == "" || len(p.To) == 0 {
			return nil
		}
		subject, intro = strings.Replace(format, "%s", store, 1), c.intro[p.Kind]
		lines = append(lines, line{c.lbl.order, no}, line{c.lbl.total, Money(p.TotalMinor, p.Currency)})
		if p.Kind == KindPlaced && p.Bank != nil {
			lines = append(lines, line{c.lbl.bank, clean(p.Bank.BankName, 60)})
			if b := clean(p.Bank.Branch, 60); b != "" {
				lines = append(lines, line{c.lbl.branch, b})
			}
			lines = append(lines, line{c.lbl.account, clean(p.Bank.AccountName, 60)}, line{c.lbl.number, clean(p.Bank.AccountNumber, 40)},
				line{c.lbl.deadline, p.ExpiresAt.In(taipei).Format("2006-01-02 15:04") + " (UTC+8)"})
		}
		if (p.Kind == KindPlaced || p.Kind == KindShipped) && p.Pickup != nil {
			lines = append(lines, line{c.lbl.pickup, clean(p.Pickup.Name, 120) + " " + clean(p.Pickup.Address, 200)})
		}
		if p.Kind == KindShipped {
			if s := p.Shipment; s != nil {
				name := carrierNames[loc][s.CarrierCode]
				if s.CarrierName != nil && clean(*s.CarrierName, 80) != "" {
					name = clean(*s.CarrierName, 80)
				}
				if name != "" {
					lines = append(lines, line{c.lbl.carrier, name})
				}
				lines = append(lines, line{c.lbl.tracking, clean(s.TrackingNumber, 64)})
				// https only (the SQL CHECK says so too); never rendered as an image or a redirect.
				if s.TrackingURL != nil && strings.HasPrefix(*s.TrackingURL, "https://") && !strings.ContainsAny(*s.TrackingURL, " \r\n\"<>") {
					lines = append(lines, line{c.lbl.link, *s.TrackingURL})
				}
			}
			if p.CVS != nil && p.CVS.ShippingNo != nil && clean(*p.CVS.ShippingNo, 40) != "" {
				lines = append(lines, line{c.lbl.tracking, clean(*p.CVS.ShippingNo, 40)})
			}
		}
		if l := orderLink(p, loc); l != "" {
			link, linkLabel = l, c.lbl.view
		}
	}
	var text, page strings.Builder
	text.WriteString(intro + "\n\n")
	page.WriteString("<p>" + html.EscapeString(intro) + "</p><table cellpadding=\"4\">")
	for _, l := range lines {
		text.WriteString(l.label + ": " + l.value + "\n")
		page.WriteString("<tr><td>" + html.EscapeString(l.label) + "</td><td><strong>" + html.EscapeString(l.value) + "</strong></td></tr>")
	}
	page.WriteString("</table>")
	if link != "" {
		text.WriteString("\n" + linkLabel + ": " + link + "\n")
		page.WriteString("<p><a href=\"" + html.EscapeString(link) + "\">" + html.EscapeString(linkLabel) + "</a></p>")
	}
	text.WriteString("\n" + c.lbl.auto + "\n")
	page.WriteString("<p>" + html.EscapeString(c.lbl.auto) + "</p>")
	out := make([]mail.Message, 0, len(p.To))
	for _, to := range p.To {
		out = append(out, mail.Message{To: to, Subject: subject, Text: text.String(), HTML: page.String()})
	}
	return out
}

// taipei is the merchant market's clock (UTC+8, no DST) for the transfer deadline; a fixed zone avoids a tzdata dependency in the worker image.
var taipei = time.FixedZone("UTC+8", 8*3600)
