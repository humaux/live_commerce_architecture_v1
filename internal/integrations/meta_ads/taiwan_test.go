package metaads

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"livecommerce/internal/integrations/core"
)

// MOCK: these checks freeze the owner-observed Graph contract, not advertiser verification.
func TestTaiwanAdsetCategory(t *testing.T) {
	for _, template := range []string{"BOOST_POST", "PRODUCT_TRAFFIC"} {
		for _, countries := range []string{`["TW"]`, `["HK"]`, `["HK","TW"]`} {
			t.Run(template+countries, func(t *testing.T) {
				body := strings.ReplaceAll(strings.ReplaceAll(adsetBody, "BOOST_POST", template), `["TW"]`, countries)
				c, f := newFake(t, reply200(`{"id":"77"}`))
				out, err := c.dispatch(context.Background(), dreq(ActionCreateAdset, body), core.NewSecret([]byte(fakeToken)))
				if err != nil || out.State != "SUCCEEDED" || len(f.log()) != 1 {
					t.Fatalf("dispatch = %+v, %v; calls=%d", out, err, len(f.log()))
				}
				payload := f.log()[0].body
				category, present := payload["regional_regulated_categories"]
				if strings.Contains(countries, `"TW"`) {
					if !reflect.DeepEqual(category, []any{"TAIWAN_UNIVERSAL"}) {
						t.Errorf("TW category = %#v", category)
					}
				} else if present {
					t.Errorf("non-TW request must omit regional category: %#v", category)
				}
				if _, present := payload["regional_regulation_identities"]; present {
					t.Error("must use Ads Manager defaults, not send regulation identities")
				}
			})
		}
	}
}

func TestTaiwanAdvertiserRejection(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       core.Outcome
	}{
		{"unverified", `{"error":{"code":100,"error_subcode":3858495}}`, 400, failedFinal("tw_advertiser_unverified")},
		{"other subcode", `{"error":{"code":100,"error_subcode":3858498}}`, 400, failedFinal("graph_100")},
		{"other code", `{"error":{"code":190,"error_subcode":3858495}}`, 400, failedFinal("graph_190")},
		{"no subcode", `{"error":{"code":100}}`, 400, failedFinal("graph_100")},
		{"server uncertainty", `{"error":{"code":100,"error_subcode":3858495}}`, 500, unconfirmed()},
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

func TestTaiwanReadRejection(t *testing.T) {
	for _, tc := range []struct{ action, body string }{
		{ActionPreflight, `{"v":1,"draft_id":"` + draftID + `","attempt":1,"seq":1}`},
		{ActionReadInsights, `{"v":1,"draft_id":"` + draftID + `","campaign_id":"555","day":"2026-10-01"}`},
	} {
		c, f := newFake(t, status(400, `{"error":{"code":100,"error_subcode":3858495}}`))
		out, err := c.dispatch(context.Background(), dreq(tc.action, tc.body), core.NewSecret([]byte(fakeToken)))
		if err != nil || out != failedFinal("tw_advertiser_unverified") || len(f.log()) != 1 || f.log()[0].method != "GET" {
			t.Fatalf("%s = %+v, %v; want proven read refusal", tc.action, out, err)
		}
	}
}
