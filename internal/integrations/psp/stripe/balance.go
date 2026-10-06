// balance.go: the balance-transaction READ for the platform settlement ledger (contracts/stripe-platform-account-v1.md §6.5, W4-S2).
// Purpose: GET /v1/balance_transactions?created[gte]&created[lt]&limit=100&expand[]=data.source, projected to the exact keys the SQL
//   definer payments.record_settlement_lines accepts. It is a read: no idempotency key, never a refund or payment authority.
// Depends on: client.go call (opList classification), strictjson.go. No stripe-go.
// Used by: internal/payments/stripeadmin/settlement.go (settlement-sync) only.
// Invariants: >50 pages or a window >8 days is ErrUncertain/ErrInvalid and NOTHING is returned (the caller writes nothing); only the
//   listed ids/amounts of the expanded source are parsed: no billing details, email or card fields are ever decoded into the projection.
// Status: MOCK (wire shape verified against docs.stripe.com 2026-10-06, PF-F5); SANDBOX/LIVE need the owner's key (NOT_RUN here).
//
// Verified fields (https://docs.stripe.com/api/balance_transactions/object, 2026-10-06): id, amount, fee, net, currency (lowercase),
// exchange_rate (number, nullable), created (unix), source (string id, nullable, expandable), type, reporting_category. Card charges and
// refunds are type charge|refund (reporting_category charge|refund); disputes and dispute reversals are type=adjustment whose source is a
// dispute; payout and stripe_fee are their own types. An expanded charge/refund/dispute carries payment_intent, amount and currency.

package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxBalancePages  = 50               // §6.5: more pages is ErrUncertain, nothing returned
	maxBalanceWindow = 8 * 24 * 60 * 60 // §6.5: the window is at most 8 days (seconds)
)

var (
	balanceTxnPattern = regexp.MustCompile(`^txn_[A-Za-z0-9]{1,255}$`)
	balanceTypeWord   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	sourceIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_]{1,255}$`)
	currencyLower     = regexp.MustCompile(`^[a-z]{3}$`)
	rateNumber        = regexp.MustCompile(`^[0-9]{1,6}(\.[0-9]{1,12})?$`)
)

// BalanceTransaction is the exact projection (JSON keys = the keys of payments.record_settlement_lines, always all 21 present; a value
// Stripe did not send is JSON null so SQL fails the row closed). Amounts are minor units; Currency fields are upper-case ISO codes.
// Contract §6.5 lists the first 16 projected fields; ReportingCategory and the Charge*/Refund* presentment pairs are additive (a card
// dispute is type=adjustment, and a charge's presentment amount is only on its expanded source), flagged for the integrator.
type BalanceTransaction struct {
	ID                   string       `json:"id"`
	Type                 string       `json:"type"`
	ReportingCategory    *string      `json:"reporting_category"`
	Amount               int64        `json:"amount"` // settlement currency, signed
	Fee                  *int64       `json:"fee"`
	Net                  int64        `json:"net"`
	Currency             string       `json:"currency"` // settlement currency
	ExchangeRate         *json.Number `json:"exchange_rate"`
	Created              int64        `json:"created"`
	SourceID             *string      `json:"source_id"`
	SourceObject         *string      `json:"source_object"`
	ChargePaymentIntent  *string      `json:"charge_payment_intent"`
	ChargeAmount         *int64       `json:"charge_amount"` // presentment (store currency) amount of the expanded charge
	ChargeCurrency       *string      `json:"charge_currency"`
	RefundID             *string      `json:"refund_id"`
	RefundAmount         *int64       `json:"refund_amount"`
	RefundCurrency       *string      `json:"refund_currency"`
	DisputeID            *string      `json:"dispute_id"`
	DisputePaymentIntent *string      `json:"dispute_payment_intent"`
	DisputeAmount        *int64       `json:"dispute_amount"`
	DisputeCurrency      *string      `json:"dispute_currency"`
}

// ListBalanceTransactions lists every balance transaction created in [createdGTE, createdLT) (unix seconds, window at most 8 days),
// newest first, following has_more by starting_after for at most 50 pages of 100. More pages, a malformed list or a wrong JSON type for
// a listed field is ErrUncertain with nothing returned. It issues GET only and sends no key material in the URL (Bearer header).
// https://docs.stripe.com/api/balance_transactions/list (retrieved 2026-10-06, PF-F5).
func (c *Client) ListBalanceTransactions(ctx context.Context, createdGTE, createdLT int64) ([]BalanceTransaction, CallMeta, error) {
	if c == nil || createdGTE <= 0 || createdLT <= createdGTE || createdLT > maxUnixSeconds || createdLT-createdGTE > maxBalanceWindow {
		return nil, CallMeta{}, ErrInvalid
	}
	out := []BalanceTransaction{}
	var meta CallMeta
	after := ""
	for page := 0; page < maxBalancePages; page++ {
		q := "created%5Bgte%5D=" + strconv.FormatInt(createdGTE, 10) + "&created%5Blt%5D=" + strconv.FormatInt(createdLT, 10) +
			"&limit=" + strconv.Itoa(listPageLimit) + "&expand%5B%5D=data.source"
		if after != "" {
			q += "&starting_after=" + url.QueryEscape(after)
		}
		raw, m, err := c.call(ctx, opList, http.MethodGet, "/v1/balance_transactions?"+q, nil, "")
		meta = m
		if err != nil {
			return nil, meta, err
		}
		txns, hasMore, err := decodeBalanceList(raw)
		if err != nil {
			return nil, meta, ErrUncertain
		}
		out = append(out, txns...)
		if !hasMore {
			return out, meta, nil
		}
		if len(txns) == 0 {
			return nil, meta, ErrUncertain // has_more with an empty page cannot advance
		}
		after = txns[len(txns)-1].ID
	}
	return nil, meta, ErrUncertain
}

func decodeBalanceList(raw []byte) ([]BalanceTransaction, bool, error) {
	root, err := decodeStrict(raw)
	if err != nil {
		return nil, false, ErrUncertain
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil, false, ErrUncertain
	}
	kind, _, okKind := jsonObject(obj).str("object")
	hasMore, hasMorePresent, okMore := jsonObject(obj).boolean("has_more")
	data, okData := obj["data"].([]any)
	if !okKind || kind != "list" || !okMore || !hasMorePresent || !okData || len(data) > listPageLimit {
		return nil, false, ErrUncertain
	}
	out := make([]BalanceTransaction, 0, len(data))
	for _, item := range data {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false, ErrUncertain
		}
		t, ok := projectBalance(jsonObject(m))
		if !ok {
			return nil, false, ErrUncertain
		}
		out = append(out, t)
	}
	return out, hasMore, nil
}

// projectBalance reads only the listed paths. ok=false means a listed field had the wrong JSON type or shape (hostile or drifted body).
// Absent or null optional fields stay nil, which SQL turns into a fail-closed row (contract §6.3).
func projectBalance(o jsonObject) (BalanceTransaction, bool) {
	var t BalanceTransaction
	id, _, okID := o.str("id")
	kind, _, okKind := o.str("object")
	typ, _, okType := o.str("type")
	cur, _, okCur := o.str("currency")
	if !okID || !okKind || !okType || !okCur || kind != "balance_transaction" || !balanceTxnPattern.MatchString(id) ||
		!balanceTypeWord.MatchString(typ) || !currencyLower.MatchString(cur) {
		return t, false
	}
	t.ID, t.Type, t.Currency = id, typ, strings.ToUpper(cur)
	amount, okA := o.integer("amount")
	net, okN := o.integer("net")
	created, okC := o.integer("created")
	fee, okF := o.integer("fee")
	if !okA || !okN || !okC || !okF || amount == nil || net == nil || created == nil || *created <= 0 {
		return t, false // amount, net and created are required; fee may be absent (nil => the row fails closed in SQL)
	}
	t.Amount, t.Net, t.Created, t.Fee = *amount, *net, *created, fee
	if rc, present, ok := o.str("reporting_category"); !ok {
		return t, false
	} else if present && balanceTypeWord.MatchString(rc) {
		t.ReportingCategory = &rc
	}
	if raw, exists := o["exchange_rate"]; exists && raw != nil {
		n, isNum := raw.(json.Number)
		if !isNum {
			return t, false
		}
		// an unusual notation (exponent, >12 decimals) is not guessed at: the rate stays nil and SQL fails that row closed
		if rateNumber.MatchString(n.String()) {
			t.ExchangeRate = &n
		}
	}
	return projectSource(o, &t)
}

// projectSource reads the expanded (object) or plain (string id) source. Only ids, payment intents and presentment amounts are kept.
func projectSource(o jsonObject, t *BalanceTransaction) (BalanceTransaction, bool) {
	raw, exists := o["source"]
	if !exists || raw == nil {
		return *t, true
	}
	if s, isStr := raw.(string); isStr { // not expanded: id only, so SQL cannot attribute it (unmapped_source) except by refund id
		if !sourceIDPattern.MatchString(s) {
			return *t, false
		}
		t.SourceID = &s
		if obj := sourceObjectFromID(s); obj != "" {
			t.SourceObject = &obj
			if obj == "refund" {
				t.RefundID = &s
			}
			if obj == "dispute" {
				t.DisputeID = &s
			}
		}
		return *t, true
	}
	src, isObj := raw.(map[string]any)
	if !isObj {
		return *t, false
	}
	so := jsonObject(src)
	id, _, okID := so.str("id")
	obj, _, okObj := so.str("object")
	if !okID || !okObj || !sourceIDPattern.MatchString(id) || !balanceTypeWord.MatchString(obj) {
		return *t, false
	}
	t.SourceID, t.SourceObject = &id, &obj
	amount, okA := so.integer("amount")
	cur, curPresent, okCur := so.str("currency")
	pi, piPresent, okPI := so.str("payment_intent")
	if !okA || !okCur || !okPI {
		return *t, false
	}
	var curUpper *string
	if curPresent {
		if !currencyLower.MatchString(cur) {
			return *t, false
		}
		u := strings.ToUpper(cur)
		curUpper = &u
	}
	var piPtr *string
	if piPresent {
		if !sourceIDPattern.MatchString(pi) {
			return *t, false
		}
		piPtr = &pi
	}
	switch obj {
	case "charge":
		t.ChargePaymentIntent, t.ChargeAmount, t.ChargeCurrency = piPtr, amount, curUpper
	case "refund":
		t.RefundID, t.RefundAmount, t.RefundCurrency = &id, amount, curUpper
	case "dispute":
		t.DisputeID, t.DisputePaymentIntent, t.DisputeAmount, t.DisputeCurrency = &id, piPtr, amount, curUpper
	}
	return *t, true
}

// sourceObjectFromID names the object of an unexpanded source id by its Stripe prefix ("" when unknown).
func sourceObjectFromID(id string) string {
	switch {
	case strings.HasPrefix(id, "ch_"), strings.HasPrefix(id, "py_"):
		return "charge"
	case strings.HasPrefix(id, "re_"), strings.HasPrefix(id, "pyr_"):
		return "refund"
	case strings.HasPrefix(id, "dp_"), strings.HasPrefix(id, "du_"):
		return "dispute"
	case strings.HasPrefix(id, "po_"):
		return "payout"
	}
	return ""
}
