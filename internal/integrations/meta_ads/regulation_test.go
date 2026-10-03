package metaads

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"livecommerce/internal/integrations/core"
)

// MOCK: these checks freeze the owner-observed Graph contract, not advertiser verification.
func TestRegionalAdsetDeclarations(t *testing.T) {
	for _, template := range []string{"BOOST_POST", "PRODUCT_TRAFFIC"} {
		for _, countries := range []string{`["TW"]`, `["SG"]`, `["HK"]`, `["HK","TW"]`, `["SG","TW"]`, `["TH"]`, `["DE"]`} {
			t.Run(template+countries, func(t *testing.T) {
				body := strings.ReplaceAll(strings.ReplaceAll(adsetBody, "BOOST_POST", template), `["TW"]`, countries)
				c, f := newFake(t, reply200(`{"id":"77"}`))
				out, err := c.dispatch(context.Background(), dreq(ActionCreateAdset, body), core.NewSecret([]byte(fakeToken)))
				if err != nil || out.State != "SUCCEEDED" || len(f.log()) != 1 {
					t.Fatalf("dispatch = %+v, %v; calls=%d", out, err, len(f.log()))
				}
				payload := f.log()[0].body
				category, present := payload["regional_regulated_categories"]
				var want []any
				if strings.Contains(countries, `"TW"`) {
					want = append(want, "TAIWAN_UNIVERSAL")
				}
				if strings.Contains(countries, `"SG"`) {
					want = append(want, "SINGAPORE_UNIVERSAL")
				}
				if len(want) > 0 && !reflect.DeepEqual(category, want) {
					t.Errorf("category = %#v; want %#v", category, want)
				} else if len(want) == 0 && present {
					t.Errorf("unlisted country must omit regional category: %#v", category)
				}
				if _, present := payload["regional_regulation_identities"]; present {
					t.Error("must use Ads Manager defaults, not send regulation identities")
				}
			})
		}
	}
}

func TestGraphRejectionOriginalMessage(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       core.Outcome
	}{
		{"unverified", `{"error":{"code":100,"error_subcode":3858495,"error_user_msg":"<b>Meta 原文</b> &amp; details"}}`, 400, core.Outcome{State: "FAILED_FINAL", Code: "graph_100", Detail: GraphRefusal{UserMessage: "Meta 原文 & details"}}},
		{"other subcode", `{"error":{"code":100,"error_subcode":3858498}}`, 400, failedFinal("graph_100")},
		{"other code", `{"error":{"code":190,"error_subcode":3858495}}`, 400, failedFinal("graph_190")},
		{"no subcode", `{"error":{"code":100}}`, 400, failedFinal("graph_100")},
		{"wrong message type keeps rejection", `{"error":{"code":100,"error_user_msg":{}}}`, 400, failedFinal("graph_100")},
		{"private message never used", `{"error":{"code":190,"message":"private diagnostic"}}`, 400, failedFinal("graph_190")},
		{"server uncertainty", `{"error":{"code":100,"error_subcode":3858495}}`, 500, unknown("graph_100")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newFake(t, status(tc.status, tc.body))
			out, err := c.dispatch(context.Background(), dreq(ActionCreateAdset, adsetBody), core.NewSecret([]byte(fakeToken)))
			if err != nil || out != tc.want || len(f.log()) != 1 {
				t.Fatalf("dispatch = %+v, %v; want %+v (one POST)", out, err, tc.want)
			}
		})
	}
	// A status write can have spent money: its existing UNKNOWN safety boundary is unchanged.
	for _, action := range []string{ActionActivate, ActionPause} {
		c, _ := newFake(t, status(400, `{"error":{"code":100,"error_subcode":3858495}}`))
		out, err := c.dispatch(context.Background(), dreq(action, statusBody), core.NewSecret([]byte(fakeToken)))
		if err != nil || out.State != "UNKNOWN" {
			t.Fatalf("%s = %+v, %v; must remain UNKNOWN", action, out, err)
		}
	}
}

func TestGraphReadRejection(t *testing.T) {
	for _, tc := range []struct{ action, body string }{
		{ActionPreflight, `{"v":1,"draft_id":"` + draftID + `","attempt":1,"seq":1}`},
		{ActionReadInsights, `{"v":1,"draft_id":"` + draftID + `","campaign_id":"555","day":"2026-10-01"}`},
	} {
		c, f := newFake(t, status(400, `{"error":{"code":100,"error_subcode":3858495}}`))
		out, err := c.dispatch(context.Background(), dreq(tc.action, tc.body), core.NewSecret([]byte(fakeToken)))
		if err != nil || out != failedFinal("graph_100") || len(f.log()) != 1 || f.log()[0].method != "GET" {
			t.Fatalf("%s = %+v, %v; want proven read refusal", tc.action, out, err)
		}
	}
}

func TestPlainUserMessage(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`<b title="x > y">原文 &amp; text</b><script>alert(1)</script><style>.x{}</style>!`, "原文 & text!"},
		{"第一行\n第二行 😀 👩‍💻", "第一行\n第二行 😀 👩‍💻"},
		{strings.Repeat("臺", 301), strings.Repeat("臺", 300)},
		{"a\x00b\x1bc", "abc"},
		{`<!--hidden--><img src=x onerror=evil()>Meta`, "Meta"},
	} {
		if got := PlainUserMessage(tc.raw); got != tc.want {
			t.Errorf("plain = %q; want %q", got, tc.want)
		}
	}
	// Cap the actual API envelope too, not only the standalone sanitizer.
	b, _ := json.Marshal(map[string]any{"error": map[string]any{"code": 100, "error_user_msg": strings.Repeat("😀", 301)}})
	o := classifyCreate(reply{status: 400, body: b}, nil)
	if o.Detail != (GraphRefusal{UserMessage: strings.Repeat("😀", 300)}) {
		t.Fatalf("bounded envelope = %+v", o)
	}
}
