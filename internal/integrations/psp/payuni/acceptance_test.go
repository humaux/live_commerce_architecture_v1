package payuni

// Root-owned acceptance: fixtures are encoded independently of Client.seal/open.
// Public PAYUNi example keys only; all HTTP is intercepted, never a provider call.
import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

const wireKey = "12345678901234567890123456789012"
const wireIV = "1234567890123456"

func wireConfig() Config {
	return Config{Environment: "SANDBOX", MerchantID: "AAA", HashKey: wireKey, HashIV: wireIV,
		ReturnURL: "https://checkout.example.com/payment/return", NotifyURL: "https://api.example.com/payment/notify"}
}

func wireClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(wireConfig())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func wireRequest() HostedRequest {
	return HostedRequest{MerTradeNo: "attempt_1-A", AmountTWD: 300, Timestamp: 1790269200,
		Description: "商品描述 / Hearing aid", Method: "payuni_credit", PageExpirySeconds: 600, Language: "zh-tw"}
}

func wireExpected() ExpectedTrade {
	return ExpectedTrade{MerTradeNo: "attempt_1-A", AmountTWD: 300, Currency: "TWD", Method: "payuni_credit"}
}

func wireRow() url.Values {
	return url.Values{"Status": {"SUCCESS"}, "MerID": {"AAA"}, "MerTradeNo": {"attempt_1-A"},
		"TradeNo": {"provider_123"}, "TradeAmt": {"300"}, "TradeStatus": {"1"}, "PaymentType": {"1"},
		"Gateway": {"2"}, "AuthType": {"1"}, "CardInst": {"0"}}
}

// Separate test encoder/decryptor prevents producer/consumer bugs cancelling out.
func wireAEAD(t *testing.T) cipher.AEAD {
	t.Helper()
	b, err := aes.NewCipher([]byte(wireKey))
	if err != nil {
		t.Fatal(err)
	}
	a, err := cipher.NewGCMWithNonceSize(b, len(wireIV))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func wireHash(value string) string {
	s := sha256.Sum256([]byte(wireKey + value + wireIV))
	return strings.ToUpper(hex.EncodeToString(s[:]))
}

func wireEnvelope(t *testing.T, plain string) url.Values {
	t.Helper()
	sealed := wireAEAD(t).Seal(nil, []byte(wireIV), []byte(plain), nil)
	n := len(sealed) - 16
	w := base64.StdEncoding.EncodeToString(sealed[:n]) + ":::" + base64.StdEncoding.EncodeToString(sealed[n:])
	e := hex.EncodeToString([]byte(w))
	return url.Values{"MerID": {"AAA"}, "Version": {"2.0"}, "Status": {"SUCCESS"}, "EncryptInfo": {e}, "HashInfo": {wireHash(e)}}
}

func wireDecode(t *testing.T, form url.Values) url.Values {
	t.Helper()
	e := form.Get("EncryptInfo")
	if form.Get("HashInfo") != wireHash(e) {
		t.Fatal("independent request digest mismatch")
	}
	w, err := hex.DecodeString(e)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(w), ":::")
	if len(parts) != 2 {
		t.Fatal("bad wire parts")
	}
	b, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	tag, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	p, err := wireAEAD(t).Open(nil, []byte(wireIV), append(b, tag...), nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := url.ParseQuery(string(p))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func wireJSON(t *testing.T, form url.Values) string {
	t.Helper()
	m := map[string]string{}
	for k, v := range form {
		m[k] = v[0]
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wireQueryBody(t *testing.T, row url.Values) string {
	t.Helper()
	inner := url.Values{"Status": {"SUCCESS"}, "Message": {"query succeeded"}}
	for k, v := range row {
		if k != "Status" {
			inner["Result[0]["+k+"]"] = v
		}
	}
	return wireJSON(t, wireEnvelope(t, inner.Encode()))
}

func TestWireOfficialGoldenAndIndependentCodec(t *testing.T) {
	c := wireClient(t)
	plain := "MerID=AAA&MerTradeNO=BBB&Prod=%E5%95%86%E5%93%81%E8%AA%AA%E6%98%8E"
	want := "47396636346f66735853533167396942344f587a3775696b34732b596e70452b675270564f73536b7753446c6a4d77526d4e374256514173672b6c78616d4533504d475152642b362f4530626f446e4f6356533969756c743a3a3a4b5961342f4635456965743069385a784b6277704a413d3d"
	wantHash := "E97180D78C8378D64A188D292938B9D2717034F292B626019B01DF160AEFC0B7"
	e, h, err := c.seal(plain)
	if err != nil || e != want || h != wantHash {
		t.Fatal("official page312 golden mismatch", err)
	}
	p, err := c.open(want, wantHash)
	if err != nil || p != plain {
		t.Fatal("official golden decryption mismatch", err)
	}
	fixture := wireEnvelope(t, plain)
	if fixture.Get("EncryptInfo") != want || fixture.Get("HashInfo") != wantHash {
		t.Fatal("independent fixture is not official compatible")
	}
}

func TestWireHostedNodeInteroperability(t *testing.T) {
	// Node is already the frontend development dependency. This test fails, not skips,
	// if unavailable: the protocol gate promises an independent OpenSSL comparison.
	for _, env := range []string{"SANDBOX", "LIVE"} {
		for _, method := range []string{"payuni_credit", "payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay"} {
			t.Run(env+"/"+method, func(t *testing.T) {
				cfg := wireConfig()
				cfg.Environment = env
				c, err := New(cfg)
				if err != nil {
					t.Fatal(err)
				}
				in := wireRequest()
				in.Method = method
				if method == "payuni_installment" {
					in.Installments = []int{3, 6, 12}
				}
				if method == "payuni_atm" || method == "payuni_cvs" {
					in.ExpireDate = "2026-09-26"
				}
				form, err := c.BuildHosted(in)
				if err != nil {
					t.Fatal(err)
				}
				host := "https://sandbox-api.payuni.com.tw"
				if env == "LIVE" {
					host = "https://api.payuni.com.tw"
				}
				if form.Action != host+"/api/upp" || len(form.Fields) != 4 || form.Fields.Get("Version") != "2.0" || form.Fields.Get("MerID") != "AAA" {
					t.Fatal("wrong hosted endpoint or envelope")
				}
				b, err := json.Marshal(form.Fields)
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("node", "../../../../scripts/dev/payuni-wire-check.mjs", "--form")
				cmd.Stdin = bytes.NewReader(b)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("independent Node check: %v: %s", err, out)
				}
				var got map[string]string
				if err = json.Unmarshal(out, &got); err != nil {
					t.Fatal(err)
				}
				want := map[string]string{"MerID": "AAA", "MerTradeNo": in.MerTradeNo, "TradeAmt": "300", "Timestamp": "1790269200",
					"ProdDesc": in.Description, "ReturnURL": cfg.ReturnURL, "NotifyURL": cfg.NotifyURL, "Lang": "zh-tw", "TradeLExpireSec": "600"}
				selector := map[string]string{"payuni_credit": "Credit", "payuni_installment": "CreditInst", "payuni_atm": "ATM", "payuni_cvs": "CVS", "payuni_linepay": "LinePay"}[method]
				want[selector] = "1"
				if method == "payuni_installment" {
					want[selector] = "3,6,12"
				}
				if in.ExpireDate != "" {
					want["ExpireDate"] = in.ExpireDate
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("hosted allowlist mismatch: got %v want %v", got, want)
				}
			})
		}
	}
}

func TestWireRejectsUnsafeInputs(t *testing.T) {
	for _, cb := range []string{"http://example.com/a", "https://127.0.0.1/a", "https://[::1]/a", "https://localhost/a", "https://foo.localhost/a", "https://user@example.com/a", "https://example.com:444/a", "https://example.com/a#fragment", "https://example.com/\r\n", "https://"} {
		cfg := wireConfig()
		cfg.NotifyURL = cb
		if _, err := New(cfg); !errors.Is(err, ErrInvalid) {
			t.Errorf("invalid callback accepted: %q", cb)
		}
	}
	for _, edge := range []struct{ key, iv string }{{" " + wireKey[1:], wireIV}, {wireKey[:31] + " ", wireIV}, {wireKey, " " + wireIV[1:]}, {wireKey, wireIV[:15] + " "}, {wireKey[:31], wireIV}, {wireKey, wireIV[:15]}} {
		cfg := wireConfig()
		cfg.HashKey, cfg.HashIV = edge.key, edge.iv
		if _, err := New(cfg); !errors.Is(err, ErrInvalid) {
			t.Error("padded/wrong-length key accepted")
		}
	}
	c := wireClient(t)
	changes := map[string]func(*HostedRequest){
		"blank trade": func(x *HostedRequest) { x.MerTradeNo = "" }, "long trade": func(x *HostedRequest) { x.MerTradeNo = strings.Repeat("x", 26) },
		"injected trade": func(x *HostedRequest) { x.MerTradeNo = "x&Credit=1" }, "zero time": func(x *HostedRequest) { x.Timestamp = 0 },
		"overflow time": func(x *HostedRequest) { x.Timestamp = 253402300800 }, "zero amount": func(x *HostedRequest) { x.AmountTWD = 0 },
		"large amount": func(x *HostedRequest) { x.AmountTWD = 200000 }, "unknown method": func(x *HostedRequest) { x.Method = "payuni_any" },
		"unknown lang": func(x *HostedRequest) { x.Language = "zh-CN" }, "short page": func(x *HostedRequest) { x.PageExpirySeconds = 59 },
		"long page": func(x *HostedRequest) { x.PageExpirySeconds = 601 }, "blank description": func(x *HostedRequest) { x.Description = "" },
		"long description": func(x *HostedRequest) { x.Description = strings.Repeat("a", 551) }, "bad utf8": func(x *HostedRequest) { x.Description = string([]byte{255}) },
		"control description": func(x *HostedRequest) { x.Description = "a\nb" }, "extra tenor": func(x *HostedRequest) { x.Installments = []int{3} },
		"extra expiry": func(x *HostedRequest) { x.ExpireDate = "2026-09-26" }, "missing tenor": func(x *HostedRequest) { x.Method = "payuni_installment" },
		"duplicate tenor": func(x *HostedRequest) { x.Method = "payuni_installment"; x.Installments = []int{3, 3} }, "unsorted tenor": func(x *HostedRequest) { x.Method = "payuni_installment"; x.Installments = []int{6, 3} },
		"invalid tenor": func(x *HostedRequest) { x.Method = "payuni_installment"; x.Installments = []int{4} }, "missing ATM expiry": func(x *HostedRequest) { x.Method = "payuni_atm" },
		"ATM amount": func(x *HostedRequest) { x.Method = "payuni_atm"; x.AmountTWD = 50000; x.ExpireDate = "2026-09-26" },
		"CVS amount": func(x *HostedRequest) { x.Method = "payuni_cvs"; x.AmountTWD = 29; x.ExpireDate = "2026-09-26" },
		"CVS date":   func(x *HostedRequest) { x.Method = "payuni_cvs"; x.ExpireDate = "2027-01-01" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			x := wireRequest()
			change(&x)
			if _, err := c.BuildHosted(x); !errors.Is(err, ErrInvalid) {
				t.Fatal("unsafe input accepted", err)
			}
		})
	}
	// 16:00 UTC is the next Taiwan calendar day: expiry must be tomorrow in Taiwan.
	for _, method := range []string{"payuni_atm", "payuni_cvs"} {
		x := wireRequest()
		x.Method = method
		x.Timestamp = time.Date(2026, 9, 24, 16, 0, 0, 0, time.UTC).Unix()
		x.ExpireDate = "2026-09-25"
		if _, err := c.BuildHosted(x); !errors.Is(err, ErrInvalid) {
			t.Fatal("same Taiwan date accepted")
		}
		x.ExpireDate = "2026-09-26"
		if _, err := c.BuildHosted(x); err != nil {
			t.Fatal("tomorrow rejected", err)
		}
	}
	expected := wireExpected()
	expected.Currency = "USD"
	if _, err := c.VerifyNotification([]byte("x=y"), expected); !errors.Is(err, ErrInvalid) {
		t.Fatal("non-TWD expected accepted")
	}
}

func TestWireNotificationBindingAndPending(t *testing.T) {
	c := wireClient(t)
	for _, status := range []string{"SUCCESS", "UNKNOWN", "UNAPPROVED"} {
		row := wireRow()
		row.Set("Status", status)
		row.Set("TradeStatus", "8")
		form := wireEnvelope(t, row.Encode())
		form.Set("Status", status)
		if status == "UNAPPROVED" {
			form.Set("Status", "Unapproved")
		}
		obs, err := c.VerifyNotification([]byte(form.Encode()), wireExpected())
		if err != nil || obs.Status != status || obs.TradeStatus != "8" {
			t.Fatalf("pending observation mismatch: %+v %v", obs, err)
		}
	}
	for _, method := range []string{"payuni_atm", "payuni_cvs", "payuni_linepay"} {
		row := wireRow()
		row.Del("AuthType")
		row.Del("CardInst")
		row.Set("PaymentType", map[string]string{"payuni_atm": "2", "payuni_cvs": "3", "payuni_linepay": "9"}[method])
		row.Set("TradeStatus", "0")
		expected := wireExpected()
		expected.Method = method
		obs, err := c.VerifyNotification([]byte(wireEnvelope(t, row.Encode()).Encode()), expected)
		if err != nil || obs.TradeStatus != "0" {
			t.Fatal("number issued not preserved", err)
		}
	}
	row := wireRow()
	row.Set("AuthType", "2")
	row.Set("CardInst", "6")
	expected := wireExpected()
	expected.Method = "payuni_installment"
	expected.Installments = []int{3, 6}
	obs, err := c.VerifyNotification([]byte(wireEnvelope(t, row.Encode()).Encode()), expected)
	if err != nil || obs.CardInst != 6 {
		t.Fatal("installment mismatch", err)
	}
	for key, value := range map[string]string{"MerID": "OTHER", "MerTradeNo": "other_attempt", "TradeAmt": "301", "PaymentType": "2", "Gateway": "1", "AuthType": "4", "CardInst": "12", "TradeStatus": "999", "Status": "PAYMENT_FAILED"} {
		t.Run(key, func(t *testing.T) {
			row := wireRow()
			row.Set(key, value)
			form := wireEnvelope(t, row.Encode())
			if key == "Status" {
				form.Set("Status", value)
			}
			if _, err := c.VerifyNotification([]byte(form.Encode()), wireExpected()); err == nil {
				t.Fatal("signed wrong facts accepted")
			}
		})
	}
	expected = wireExpected()
	expected.TradeNo = "another_provider_id"
	if _, err = c.VerifyNotification([]byte(wireEnvelope(t, wireRow().Encode()).Encode()), expected); !errors.Is(err, ErrMismatch) {
		t.Fatal("known provider ID ignored", err)
	}
	row = wireRow()
	row.Set("Status", "UNKNOWN")
	form := wireEnvelope(t, row.Encode()) // forged unsigned SUCCESS
	if _, err = c.VerifyNotification([]byte(form.Encode()), wireExpected()); !errors.Is(err, ErrUncertain) {
		t.Fatal("unsigned outer status upgraded inner UNKNOWN", err)
	}
}

func TestWireMalformedAuthenticatedPayloads(t *testing.T) {
	c := wireClient(t)
	base := wireRow().Encode()
	badPlain := []string{base + "&%4derTradeNo=attempt_1-A", base + "&x=%ZZ", base + "&x=%FF", base + "&", base + "&a[b]=x", base + "&" + strings.Repeat("k", 121) + "=x", base + "&x=" + strings.Repeat("a", 49<<10), base + "&x=" + string([]byte{255})}
	for i, raw := range badPlain {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := c.VerifyNotification([]byte(wireEnvelope(t, raw).Encode()), wireExpected()); err == nil {
				t.Fatal("malformed signed plaintext accepted")
			}
		})
	}
	var many strings.Builder
	many.WriteString(base)
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&many, "&extra%d=x", i)
	}
	if _, err := c.VerifyNotification([]byte(wireEnvelope(t, many.String()).Encode()), wireExpected()); !errors.Is(err, ErrProtocol) {
		t.Fatal("field cap not enforced", err)
	}
	form := wireEnvelope(t, base)
	for name, raw := range map[string]string{"duplicate outer": form.Encode() + "&%4DerID=AAA", "malformed outer": form.Encode() + "&x=%ZZ", "large body": strings.Repeat("a", (128<<10)+1), "unsigned error": "Status=ERROR&Message=secret"} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.VerifyNotification([]byte(raw), wireExpected()); err == nil {
				t.Fatal("untrusted outer accepted")
			}
		})
	}
	for _, mutation := range []func(url.Values){
		func(f url.Values) { f.Set("HashInfo", strings.Repeat("0", 64)) },
		func(f url.Values) { f.Set("EncryptInfo", "zz"); f.Set("HashInfo", wireHash("zz")) },
		func(f url.Values) {
			e := hex.EncodeToString([]byte("AAAA:::AAAA:::AAAA"))
			f.Set("EncryptInfo", e)
			f.Set("HashInfo", wireHash(e))
		},
		func(f url.Values) {
			w, _ := hex.DecodeString(f.Get("EncryptInfo"))
			parts := strings.Split(string(w), ":::")
			tag, _ := base64.StdEncoding.DecodeString(parts[1])
			tag[0] ^= 1
			e := hex.EncodeToString([]byte(parts[0] + ":::" + base64.StdEncoding.EncodeToString(tag)))
			f.Set("EncryptInfo", e)
			f.Set("HashInfo", wireHash(e))
		},
		func(f url.Values) { f.Set("MerID", "OTHER") }, func(f url.Values) { f.Set("Version", "1.0") },
	} {
		f := wireEnvelope(t, base)
		mutation(f)
		if _, err := c.VerifyNotification([]byte(f.Encode()), wireExpected()); err == nil {
			t.Fatal("bad crypto/envelope accepted")
		}
	}
}

type wireTransport func(*http.Request) (*http.Response, error)

func (f wireTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type wireBody struct {
	io.Reader
	closed bool
	read   int
}

func (b *wireBody) Read(p []byte) (int, error) { n, e := b.Reader.Read(p); b.read += n; return n, e }
func (b *wireBody) Close() error               { b.closed = true; return nil }

func TestWireQueryRequestAndObservation(t *testing.T) {
	for _, env := range []string{"SANDBOX", "LIVE"} {
		for _, known := range []bool{false, true} {
			for _, source := range []string{"A", "B"} {
				t.Run(fmt.Sprint(env, "/", known, "/", source), func(t *testing.T) {
					cfg := wireConfig()
					cfg.Environment = env
					c, err := New(cfg)
					if err != nil {
						t.Fatal(err)
					}
					expected := wireExpected()
					if known {
						expected.TradeNo = "provider_123"
					}
					row := wireRow()
					row.Set("DataSource", source)
					row.Set("CloseStatus", "9")
					body := &wireBody{Reader: strings.NewReader(wireQueryBody(t, row))}
					calls := 0
					c.httpClient.Transport = wireTransport(func(r *http.Request) (*http.Response, error) {
						calls++
						host := "sandbox-api.payuni.com.tw"
						if env == "LIVE" {
							host = "api.payuni.com.tw"
						}
						if r.Method != "POST" || r.URL.Scheme != "https" || r.URL.Host != host || r.URL.Path != "/api/trade/query" || r.URL.RawQuery != "" || r.Header.Get("User-Agent") != "payuni" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
							t.Fatal("unsafe query request")
						}
						if err := r.ParseForm(); err != nil {
							t.Fatal(err)
						}
						if len(r.PostForm) != 4 || r.PostForm.Get("Version") != "2.0" {
							t.Fatal("query envelope mismatch")
						}
						inner := wireDecode(t, r.PostForm)
						want := url.Values{"MerID": {"AAA"}, "Timestamp": {"1790269200"}}
						if known {
							want.Set("TradeNo", expected.TradeNo)
						} else {
							want.Set("MerTradeNo", expected.MerTradeNo)
						}
						if !reflect.DeepEqual(inner, want) {
							t.Fatal("query identity not exclusive", inner)
						}
						return &http.Response{StatusCode: 200, Body: body, Header: http.Header{}, Request: r}, nil
					})
					obs, err := c.Query(context.Background(), expected, 1790269200)
					if err != nil || calls != 1 || !body.closed || obs.DataSource != source || obs.CloseStatus != "9" || obs.TradeStatus != "1" {
						t.Fatalf("query observation: %+v %v calls=%d closed=%v", obs, err, calls, body.closed)
					}
				})
			}
		}
	}
}

func TestWireQueryRejectsAmbiguousAndMalformedResponses(t *testing.T) {
	row := wireRow()
	row.Set("DataSource", "A")
	valid := wireQueryBody(t, row)
	form, _ := url.ParseQuery(wireEnvelope(t, "Status=SUCCESS&Result[0][TradeAmt]=300&Result[1][TradeAmt]=300").Encode())
	responses := map[string]string{
		"duplicate JSON":         strings.TrimSuffix(valid, "}") + `,"MerID":"AAA"}`,
		"escaped duplicate JSON": strings.TrimSuffix(valid, "}") + `,"\u004derID":"AAA"}`,
		"trailing JSON":          valid + `{}`, "nested JSON": `{"MerID":{"x":"AAA"}}`, "nonstring JSON": `{"Status":true}`,
		"unsigned error": `{"Status":"ERROR","Message":"card-secret"}`, "multirow": wireJSON(t, form),
		"flat row": wireJSON(t, wireEnvelope(t, row.Encode())), "scalar result": wireJSON(t, wireEnvelope(t, "Status=SUCCESS&Result=x")),
		"invalid utf8": valid + string([]byte{255}),
	}
	for key, value := range map[string]string{"DataSource": "Z", "MerID": "OTHER", "TradeAmt": "299", "MerTradeNo": "other", "Gateway": "1", "PaymentType": "9", "AuthType": "4", "CloseStatus": "999"} {
		r := wireRow()
		r.Set("DataSource", "A")
		r.Set(key, value)
		responses["row/"+key] = wireQueryBody(t, r)
	}
	for name, response := range responses {
		t.Run(name, func(t *testing.T) {
			c := wireClient(t)
			calls := 0
			body := &wireBody{Reader: strings.NewReader(response)}
			c.httpClient.Transport = wireTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Body: body, Header: http.Header{}, Request: r}, nil
			})
			if _, err := c.Query(context.Background(), wireExpected(), 1790269200); err == nil {
				t.Fatal("untrusted response accepted")
			}
			if calls != 1 || !body.closed {
				t.Fatal("query retried or leaked body")
			}
		})
	}
}

func TestWireQueryTransportFailsClosed(t *testing.T) {
	c := wireClient(t)
	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 || transport.TLSClientConfig.InsecureSkipVerify || c.httpClient.Timeout != 10*time.Second || c.httpClient.Jar != nil {
		t.Fatal("unsafe owned HTTP defaults")
	}
	for _, status := range []int{302, 307, 400, 500} {
		c := wireClient(t)
		calls := 0
		body := &wireBody{Reader: strings.NewReader("provider-sensitive-data")}
		c.httpClient.Transport = wireTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: status, Header: http.Header{"Location": {"https://attacker.example/collect"}}, Body: body, Request: r}, nil
		})
		_, err := c.Query(context.Background(), wireExpected(), 1790269200)
		if !errors.Is(err, ErrTransport) || calls != 1 || !body.closed || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("HTTP %d leaked/retried: %v", status, err)
		}
	}
	c = wireClient(t)
	body := &wireBody{Reader: strings.NewReader(strings.Repeat("x", 256<<10))}
	c.httpClient.Transport = wireTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: body, Header: http.Header{}, Request: r}, nil
	})
	if _, err := c.Query(context.Background(), wireExpected(), 1790269200); !errors.Is(err, ErrProtocol) || !body.closed || body.read > (128<<10)+1 {
		t.Fatal("response not bounded/closed", err, body.read)
	}
	c = wireClient(t)
	calls := 0
	c.httpClient.Transport = wireTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("private HashKey and customer payload")
	})
	if _, err := c.Query(context.Background(), wireExpected(), 1790269200); !errors.Is(err, ErrTransport) || strings.Contains(err.Error(), "private") || calls != 1 {
		t.Fatal("transport error not sanitized", err)
	}
	c = wireClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	c.httpClient.Transport = wireTransport(func(r *http.Request) (*http.Response, error) { calls++; cancel(); return nil, r.Context().Err() })
	if _, err := c.Query(ctx, wireExpected(), 1790269200); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("cancel lost", err)
	}
	c = wireClient(t)
	c.httpClient.Transport = wireTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid expected made network request")
		return nil, ErrTransport
	})
	expected := wireExpected()
	expected.TradeNo = "bad&value"
	if _, err := c.Query(context.Background(), expected, 1790269200); !errors.Is(err, ErrInvalid) {
		t.Fatal("unsafe provider trade accepted", err)
	}
}

func TestWireSensitiveFieldsStayOutOfProjectionAndFormatting(t *testing.T) {
	c := wireClient(t)
	row := wireRow()
	row.Set("CardNo", "private-card-data")
	row.Set("Message", "private-customer-data")
	row.Set("CreditHash", "private-token")
	obs, err := c.VerifyNotification([]byte(wireEnvelope(t, row.Encode()).Encode()), wireExpected())
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "private") {
		t.Fatal("sensitive provider field projected")
	}
	typ := reflect.TypeOf(obs)
	for _, field := range []string{"Paid", "Raw", "RawBody", "CardNo", "CreditHash", "HashKey", "HashIV"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatal("unsafe observation field", field)
		}
	}
	for _, value := range []any{wireConfig(), c, *c} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		all := fmt.Sprintf("%v %+v %#v %s", value, value, value, encoded)
		if strings.Contains(all, wireKey) || strings.Contains(all, wireIV) || strings.Contains(all, "payment/notify") {
			t.Fatal("config leaked")
		}
	}
}
