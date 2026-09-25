# PAYUNi UPP and query wire v1

2026-09-24; frozen implementation contract. PROTOCOL_MOCK only until real merchant
sandbox acceptance. Does not enable payment methods, create attempts or book money.

## Official source basis

- UPP v2.0: https://docs.payuni.com.tw/web/#/7/34
- AES-GCM/hash: https://docs.payuni.com.tw/web/#/7/29
- Official Node crypto golden vector: https://docs.payuni.com.tw/web/#/7/312
- Query v2.0: https://docs.payuni.com.tw/web/#/7/164
- Amount limits: https://docs.payuni.com.tw/web/#/7/170
- Official PHP SDK wire reference (not imported/copied):
  https://github.com/payuni/PHP_SDK/blob/bba8ddd831321aebc0123478b34fecfbc11ed70a/src/PayuniApi.php

SPA source text was read via official read-only page/info API. Requests and decrypted
responses are URL-form encoded; HTTP query envelope is JSON. The local
VerifyNotification parser supports a bounded form envelope. The 2026-09-25
official-source recheck did not establish the background NotifyURL transport,
Content-Type or ACK/retry contract: explicit Form Post wording covers the UPP
submission and foreground ReturnURL, not proof of background HTTP delivery.
Do not expose this helper as a production callback receiver without that gate.
Never copy SDK disabled TLS verification or treat its `success=true` ERROR response as
a payment success. Provider protocol uses a fixed merchant IV; this is vendor wire
compatibility only, NEVER reuse this construction for storage (accounts uses random
nonce/AAD). Protocol limitations and merchant secret lifecycle require production review.

## Small standalone package

`internal/integrations/psp/payuni` uses stdlib only, no SQL/River dependencies. Functions:

- `AmountTWDFromMinor(currency string, amountMinor int64) (int64,error)` is the
  explicit boundary from the repository's TWD two-decimal minor units to PAYUNi
  integer yuan. Require currency=TWD, amount>0 and amount%100=0; divide exactly,
  never round/truncate/re-price the order. It is not method-limit/admission validation;
  BuildHosted and the payment orchestrator must still validate those independently.
- `New(Config) (*Client,error)`; Config Environment SANDBOX/LIVE, MerchantID
  `[A-Za-z0-9_-]{1,64}`, HashKey, HashIV, ReturnURL, NotifyURL. This initial supported
  interoperability profile requires exactly 32 printable ASCII key bytes and 16 IV
  bytes, neither padded/trimmed. This is a local supported profile, not a claim that
  PAYUNi formally disallows every other format. Config and Client formatting and
  JSON redact secrets; client owns its copied immutable configuration.
- `(*Client).BuildHosted(HostedRequest) (HostedForm,error)`; no network call.
  HostedRequest: MerTradeNo string (1..25 ASCII alphanumeric/underscore/hyphen), AmountTWD int64, Timestamp int64 (>0), Description
  string (nonempty printable UTF8 <=550 bytes, fail rather than silently truncate),
  Method string (five payuni_* codes), Installments []int (only installment: explicit
  unique ascending subset of 3,6,9,12; others require empty), ExpireDate string
  (ATM/CVS explicit YYYY-MM-DD; reject for others), PageExpirySeconds int (60..600),
  Language string (`zh-tw` or `en`; frontend zh-Hans intentionally falls back zh-tw).
  Callers supply persisted server-owned trade, clock and deadline, never buyer values.
  No card numbers, buyer token, logistics, COD, coupons or tracking parameters.
  Amount bounds TWD integer units: credit/installment1..199999, ATM15..49999,
  CVS30..20000; LINE Pay1..199999 is conservative local cap pending merchant limits.
  ATM/CVS dates compare Taiwan date of Timestamp: tomorrow..+7days for CVS,
  tomorrow..+180days for ATM. Excluding same-day avoids undocumented edge timing.
  Do not create long-lived bank/CVS requests until inventory/payment lifetime policy
  is implemented; building a form alone is not inventory authorization.
- HostedForm: Action string, Fields url.Values. Fixed HTTPS official UPP host by env,
  fields only MerID,Version=2.0,EncryptInfo,HashInfo. Inner fields include callbacks,
  exact server trade and ONE selected payment selector (`Credit=1`, `ATM=1`, `CVS=1`,
  `LinePay=1`, or `CreditInst=...`). Never omit all selectors and inherit store defaults.
- `(*Client).VerifyNotification(body []byte, expected ExpectedTrade) (Observation,error)`;
  bounded form body, reject duplicate fields/invalid encoding before authentication.
- `(*Client).Query(ctx context.Context, expected ExpectedTrade, timestamp int64)
  (Observation,error)`; one POST to official `/api/trade/query`, no automatic retry.
  Request uses TradeNo if expected supplies one, otherwise MerTradeNo, not both.

ExpectedTrade: MerTradeNo (same bounds as request), TradeNo(optional; local bounded
1..64 ASCII alphanumeric/underscore/hyphen profile), AmountTWD, Currency("TWD" only), Method,
Installments ([]int frozen allowed tenors, same validation as request). Query expected
must come from a durable attempt, not arbitrary caller/user-provided business facts.
The caller MUST select Client configuration from that attempt's frozen tenant/store,
merchant-account ID, environment and credential-version/key reference. Never pick the
current active credentials or treat unsigned outer MerID as account authority. An outer
identifier can at most narrow candidates before authenticating against persisted context.
The wire contains no environment/tenant/store proof; those boundaries belong to the
durable caller. Preserve historical credential references for outstanding attempts;
old-key notification behavior and credential rotation for queries need merchant sandbox
verification, not an assumed automatic fallback to new credentials.
Observation contains MerTradeNo, TradeNo, AmountTWD, PaymentType string, TradeStatus
string, Status string, AuthType string, CardInst int, DataSource string, CloseStatus
string. No Paid bool, capture claim, raw body, card data, account keys or generic map.
String fields only projected from known validated fields. This is an authenticated
provider observation, NOT a database state transition or evidence of settlement.

## Crypto, parser and binding checks

Go AES-256-GCM with 16-byte nonce and 16-byte tag, no AAD (wire spec). Request plaintext
is `url.Values.Encode()`. EncryptInfo=lowerhex(base64(ciphertext)+":::"+base64(tag));
HashInfo=upperhex(SHA256(HashKey+EncryptInfo+HashIV)). Constant-time hash comparison
before GCM decryption; fail closed on malformed hex/base64/tag/duplicate delimiter.
Private helpers `seal(plain string)` and `open(encryptInfo,hashInfo string)` allow root
independent official-vector tests; never export generic signing/decryption endpoints.

Bound raw response128KiB, decoded plaintext48KiB, field count256, key<=120 bytes;
no duplicate decoded keys, invalid UTF8, malformed escapes or partial parse accepted.
JSON envelope must be one flat object of string values; reject duplicate keys,
trailing data and nested values. Extra scalar fields may be discarded, never logged.
Outer MerID matches configured merchant, Version=2.0, signed inner payload is required;
unsigned ERROR/other envelopes fail uncertain, not a final financial failure.
Callback inner MerID must match too. Inner notification Status supports SUCCESS,
UNKNOWN,UNAPPROVED only; other outcomes return ErrUncertain for later query.
Official page34 uses outer `Unapproved` with inner `UNAPPROVED`; accept that exact
documented pair, not arbitrary case folding. All other supported outer/inner states
must agree; the unsigned outer status can never upgrade the authenticated result.
Only projection with exact MerTradeNo/amount/optional knownTradeNo, expected
PaymentType(credit/installment1,ATM2,CVS3,LINE9) and Gateway=2 is accepted.
Credit AuthType must be1 for single payment,2 for installment; installment CardInst
must be in frozen allowed list. Never infer paid just from Status=SUCCESS.
TradeStatus allowed 0,1,2,3,8 for callback; query additionally4,9. Preserve raw state.
No currency wire field is documented: only a previously validated TWD account/attempt
can call this profile; do not claim cryptographic currency verification from response.

Query encrypted payload Status must be SUCCESS; use exactly one Result row, otherwise
ErrUncertain. Accept PHP `Result[0][Field]` bracket encoding (official Result array +
parse_str SDK inference; exact literal raw provider sample still NOT_RUN). Do not
accept a flattened alternative or silently select first among multiple records.
Query merchant is outer verified MerID; if row MerID exists it must match. Require
DataSource=A or B and preserve it (B incomplete => no financial finality). Additional
rows/unrecognized structure => ErrUncertain, query again via durable orchestration.
Unknown nested row fields may be discarded only after syntax/duplicate checks.

## Network and errors

Owned HTTP client timeout10s, TLS>=1.2 with cert verification, no cookies, no redirects,
fixed provider endpoints; no externally supplied baseURL/transport/HTTP client option.
Test files in same package may replace private client transport only for fixtures.
POST application/x-www-form-urlencoded, User-Agent payuni; bounded read + body close.
Require HTTP200 and valid envelope. Context cancellation propagated via errors.Is;
otherwise sanitized ErrTransport, ErrInvalid, ErrAuthentication, ErrMismatch,
ErrProtocol, ErrUncertain; never embed provider body/URL query/keys in errors.
Callbacks: server-owned https URLs with no userinfo/fragment, host required, no IP or
localhost, port absent/443, <=2048 bytes. No data fetched from callbacks by this package.

## Acceptance and next boundary

Author unit tests + independent root official golden vector and Node cross-language
decrypt/re-encrypt evidence; no self-roundtrip-only acceptance. Root tests malicious
forms/JSON (duplicates, limits, bad crypto, wrong identity/amount/method), genuine
signed UNKNOWN/number-issued/incomplete records never becoming paid, HTTPS query
mock transport (one call, response bounded, redirects/cancel/error sanitized).
Full Go race/vet regression and independent payment review. No provider transactions.

This wire package does not own credentials lookup, durable attempts, production
worker assembly, replay ledger/inbox or a callback HTTP handler. The following
[query increment](payment-query-v1.md) composes a historical reader and internal
River worker outside this package; it does not settle funds. UPP front-channel
return must not commit money. Official
callback ACK/retry policy unresolved; do not invent ACK. StartPayment integration must
freeze the exact account/environment/credential version, attempt identity/deadlines,
and move stock to PAYMENT_PENDING atomically before
exposing hosted form; query/notification facts require separate idempotent reconciliation.
Method enabled guard stays until that integration and real supplier admission pass.
