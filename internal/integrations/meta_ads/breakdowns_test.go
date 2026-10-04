package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/integrations/core"
)

func TestInsightsBreakdownTransport(t *testing.T) {
	c, f := newFake(t, func(c call, w http.ResponseWriter) {
		q, _ := url.ParseQuery(c.query)
		switch {
		case strings.Contains(c.path, "act_"):
			io.WriteString(w, `{"currency":"TWD","timezone_name":"Asia/Taipei"}`)
		case !strings.HasSuffix(c.path, "/insights"):
			io.WriteString(w, `{"effective_status":"ACTIVE"}`)
		case q.Get("breakdowns") == "":
			io.WriteString(w, `{"data":[{"spend":"12.30","impressions":"9","clicks":"2"}]}`)
		case q.Get("breakdowns") == "age,gender":
			if q.Get("after") == "" {
				io.WriteString(w, `{"data":[{"age":"25-34","gender":"female","spend":"1.23","reach":"5","impressions":"9","clicks":"2","actions":[{"action_type":"post_engagement","value":"3"},{"action_type":"comment","value":"1"},{"action_type":"omni_purchase","value":""}],"action_values":[{"action_type":"omni_purchase","value":""}]}],"paging":{"next":"https://untrusted.invalid/?access_token=do-not-follow","cursors":{"after":"page2"}}}`)
			} else {
				io.WriteString(w, `{"data":[{"age":"35-44","gender":"male","actions":[{"action_type":"omni_purchase","value":"0"}],"action_values":[{"action_type":"omni_purchase","value":"0"}]}]}`)
			}
		case q.Get("breakdowns") == "region":
			io.WriteString(w, `{"data":[{"region":"Taipei"}]}`)
		case q.Get("breakdowns") == "publisher_platform,platform_position":
			io.WriteString(w, `{"data":[{"publisher_platform":"facebook","platform_position":"feed"}]}`)
		case q.Get("breakdowns") == "device_platform":
			io.WriteString(w, `{"data":[{"device_platform":"mobile_app"}]}`)
		default:
			io.WriteString(w, `{"data":[{"date_start":"2026-10-02","hourly_stats_aggregated_by_advertiser_time_zone":"13:00:00 - 13:59:59"}]}`)
		}
	})
	out := c.readInsights(context.Background(), asset, "555", "2026-10-02", []byte(fakeToken))
	if out.State != "SUCCEEDED" || out.ProviderReference != "v1;es=ACTIVE;sp=12.30;im=9;cl=2;pu=na;pv=na;cur=TWD;tz=Asia/Taipei" {
		t.Fatalf("daily = %+v", out)
	}
	raw, _ := json.Marshal(out.Detail)
	var detail struct {
		Timezone string `json:"timezone_name"`
		Currency string `json:"currency"`
		Rows     []struct {
			Dimension   string  `json:"dimension"`
			Bucket      string  `json:"bucket"`
			Hour        *string `json:"hour_start"`
			Spend       int64   `json:"spend_minor"`
			Engagements int64   `json:"engagements"`
			Comments    int64   `json:"comments"`
			Purchases   *int64  `json:"purchases"`
			Value       *int64  `json:"purchase_value_minor"`
		} `json:"rows"`
	}
	if json.Unmarshal(raw, &detail) != nil || detail.Timezone != "Asia/Taipei" || detail.Currency != "TWD" || len(detail.Rows) != 6 {
		t.Fatalf("detail = %s", raw)
	}
	first, zero, hour := detail.Rows[0], detail.Rows[1], detail.Rows[5]
	if first.Dimension != "age_gender" || first.Bucket != "25-34/female" || first.Spend != 123 || first.Engagements != 3 || first.Comments != 1 || first.Purchases != nil || first.Value != nil || first.Hour != nil {
		t.Fatalf("first = %+v", first)
	}
	if zero.Purchases == nil || *zero.Purchases != 0 || zero.Value == nil || *zero.Value != 0 || hour.Hour == nil || *hour.Hour != "2026-10-02T05:00:00Z" {
		t.Fatalf("zero/hour = %+v/%+v", zero, hour)
	}
	calls := f.log()
	if len(calls) != 9 {
		t.Fatalf("calls = %d", len(calls))
	}
	for _, call := range calls[3:] {
		q, _ := url.ParseQuery(call.query)
		if call.method != http.MethodGet || call.path != "/v26.0/555/insights" || q.Get("fields") != "spend,reach,impressions,clicks,actions,action_values" || q.Get("time_increment") != "1" {
			t.Fatalf("breakdown call = %+v", call)
		}
	}
	// Comparing outcomes must remain safe even with route-private detail.
	_ = out == out
}

func TestProductTrafficAttributionLink(t *testing.T) {
	body := `{"v":1,"draft_id":"` + draftID + `","attempt":1,"name":"lc-` + opID + `","template":"PRODUCT_TRAFFIC","page_id":"777","link_url":"https://shop.example.test/p?a=1&a=2&lc_ad=untrusted"}`
	c, f := newFake(t, reply200(`{"id":"88"}`))
	out, err := c.dispatch(context.Background(), dreq(ActionCreateCreative, body), core.NewSecret([]byte(fakeToken)))
	if err != nil || out.State != "SUCCEEDED" {
		t.Fatalf("out = %+v, %v", out, err)
	}
	link := f.log()[0].body["object_story_spec"].(map[string]any)["link_data"].(map[string]any)["link"].(string)
	u, _ := url.Parse(link)
	if got := u.Query(); got.Get("lc_ad") != draftID || len(got["lc_ad"]) != 1 || strings.Join(got["a"], ",") != "1,2" {
		t.Fatalf("link = %s", link)
	}
}

func TestInsightHourBoundaries(t *testing.T) {
	for _, tc := range []struct{ zone, day, hour, want string }{
		{"Asia/Taipei", "2026-10-02", "00:00:00 - 00:59:59", "2026-10-01T16:00:00Z"},
		{"America/New_York", "2026-10-02", "13:00:00 - 13:59:59", "2026-10-02T17:00:00Z"},
		{"America/New_York", "2026-11-01", "01:00:00 - 01:59:59", ""},
		{"America/New_York", "2026-03-08", "02:00:00 - 02:59:59", ""},
		{"Australia/Lord_Howe", "2026-04-05", "01:00:00 - 01:59:59", ""},
		{"Asia/Taipei", "2026-10-02", "24:00:00 - 24:59:59", ""},
		{"Asia/Taipei", "2026-10-02", "01:00:00 - 02:59:59", ""},
		{"Asia/Taipei", "2026-02-30", "01:00:00 - 01:59:59", ""},
	} {
		t.Run(tc.zone+tc.day+tc.hour, func(t *testing.T) {
			loc, err := insightLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			hour, err := absoluteInsightHour(tc.day, tc.hour, loc)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("accepted ambiguous/invalid time: %s", hour)
				}
				return
			}
			if err != nil || hour.Format(time.RFC3339) != tc.want {
				t.Fatalf("got %s,%v want %s", hour, err, tc.want)
			}
		})
	}
	for _, zone := range []string{"", "Local", "Invented/Zone"} {
		if _, err := insightLocation(zone); err == nil {
			t.Fatalf("accepted zone %q", zone)
		}
	}
}

func TestBreakdownRefusesPartialOrUnboundedRead(t *testing.T) {
	for _, tc := range []struct {
		name, body    string
		status, calls int
		want          core.Outcome
	}{
		{"bad cursor", `{"data":[],"paging":{"next":"https://untrusted.invalid","cursors":{"after":"x?access_token=private"}}}`, 200, 4, unconfirmed()},
		{"missing cursor", `{"data":[],"paging":{"next":"https://untrusted.invalid"}}`, 200, 4, unconfirmed()},
		{"repeated cursor", `{"data":[],"paging":{"next":"https://untrusted.invalid","cursors":{"after":"same"}}}`, 200, 5, unconfirmed()},
		{"unknown page", `{"unknown":[]}`, 200, 4, unconfirmed()},
		{"invalid label", `{"data":[{"age":"<script>","gender":"female"}]}`, 200, 4, failedFinal("bad_result")},
		{"oversize label", `{"data":[{"age":"` + strings.Repeat("x", 161) + `","gender":"female"}]}`, 200, 4, failedFinal("bad_result")},
		{"wrong day", `{"data":[{"age":"25-34","gender":"female","date_start":"2026-10-01"}]}`, 200, 4, failedFinal("bad_result")},
		{"duplicate row", `{"data":[{"age":"25-34","gender":"female"},{"age":"25-34","gender":"female"}]}`, 200, 4, failedFinal("bad_result")},
		{"bad money", `{"data":[{"age":"25-34","gender":"female","spend":"1.234"}]}`, 200, 4, failedFinal("bad_spend")},
		{"overflow count", `{"data":[{"age":"25-34","gender":"female","reach":"9223372036854775808"}]}`, 200, 4, failedFinal("bad_result")},
		{"duplicate metric", `{"data":[{"age":"25-34","gender":"female","actions":[{"action_type":"omni_purchase","value":"2"},{"action_type":"omni_purchase","value":"3"}]}]}`, 200, 4, failedFinal("bad_result")},
		{"refusal", `{"error":{"code":100,"error_user_msg":"Public refusal"}}`, 400, 4, core.Outcome{State: "FAILED_FINAL", Code: "graph_100", Detail: GraphRefusal{UserMessage: "Public refusal"}}},
		{"uncertain server", `{"error":{"code":100}}`, 503, 4, unknown("graph_100")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newFake(t, func(c call, w http.ResponseWriter) {
				q, _ := url.ParseQuery(c.query)
				if strings.Contains(c.path, "act_") {
					io.WriteString(w, `{"currency":"TWD","timezone_name":"Asia/Taipei"}`)
				} else if q.Get("breakdowns") != "" {
					w.WriteHeader(tc.status)
					io.WriteString(w, tc.body)
				} else if strings.HasSuffix(c.path, "/insights") {
					io.WriteString(w, `{"data":[]}`)
				} else {
					io.WriteString(w, `{"effective_status":"ACTIVE"}`)
				}
			})
			// R11 keeps these refusals at the dimension boundary. No rows from
			// any failed or incomplete dimension may reach the daily detail.
			loc, _ := insightLocation("Asia/Taipei")
			rows, out := c.readBreakdownDimension(context.Background(), "555", "2026-10-02", "TWD", loc, []byte(fakeToken), []byte(`{"since":"2026-10-02","until":"2026-10-02"}`), "age_gender", "age,gender")
			if rows != nil || out != tc.want || len(f.log()) != tc.calls-3 {
				t.Fatalf("rows=%+v out=%+v calls=%d want=%+v/%d", rows, out, len(f.log()), tc.want, tc.calls-3)
			}
		})
	}
	page := 0
	c, f := newFake(t, func(c call, w http.ResponseWriter) {
		q, _ := url.ParseQuery(c.query)
		if strings.Contains(c.path, "act_") {
			io.WriteString(w, `{"currency":"TWD","timezone_name":"Asia/Taipei"}`)
		} else if q.Get("breakdowns") != "" {
			page++
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "paging": map[string]any{"next": "https://untrusted.invalid", "cursors": map[string]string{"after": itoa(page)}}})
		} else if strings.HasSuffix(c.path, "/insights") {
			io.WriteString(w, `{"data":[]}`)
		} else {
			io.WriteString(w, `{"effective_status":"ACTIVE"}`)
		}
	})
	loc, _ := insightLocation("Asia/Taipei")
	if rows, out := c.readBreakdownDimension(context.Background(), "555", "2026-10-02", "TWD", loc, []byte(fakeToken), []byte(`{"since":"2026-10-02","until":"2026-10-02"}`), "age_gender", "age,gender"); rows != nil || out != unconfirmed() || len(f.log()) != 10 {
		t.Fatalf("unbounded pagination rows=%+v out=%+v calls=%d", rows, out, len(f.log()))
	}
}

type breakdownTx struct {
	pgx.Tx
	sql    []string
	args   [][]any
	failAt int
}

func (tx *breakdownTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.sql = append(tx.sql, sql)
	tx.args = append(tx.args, args)
	if tx.failAt == len(tx.sql) {
		return pgconn.CommandTag{}, errors.New("fixture SQL refusal")
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func TestBreakdownFinishSeam(t *testing.T) {
	claim := core.SecretClaim{OperationID: opID, Generation: 7, LeaseToken: []byte("fixture-lease"), Mode: "dispatch"}
	ref, _ := EncodeInsights(InsightsDay{EffectiveStatus: "ACTIVE", Currency: "TWD", Timezone: "Asia/Taipei"})
	detail := &InsightsBreakdowns{TimezoneName: "Asia/Taipei", Currency: "TWD", Rows: []InsightsBreakdownRow{}}
	for _, tc := range []struct {
		name              string
		out               core.Outcome
		failAt, wantCalls int
		wantErr           bool
	}{
		{"typed success", core.Outcome{State: "SUCCEEDED", Code: "graph_read", ProviderReference: ref, Detail: detail}, 0, 2, false},
		{"plain success", core.Outcome{State: "SUCCEEDED", Code: "graph_created", ProviderReference: "88"}, 0, 1, false},
		{"preflight success", core.Outcome{State: "SUCCEEDED", Code: "graph_read", ProviderReference: "v1;as=1;cur=TWD;tz=Asia/Taipei;fs=1"}, 0, 1, false},
		{"refusal", core.Outcome{State: "FAILED_FINAL", Code: "graph_100", Detail: GraphRefusal{UserMessage: "public"}}, 0, 1, false},
		{"unknown", unknown("graph_unconfirmed"), 0, 0, false},
		{"old definer fails", core.Outcome{State: "SUCCEEDED", Code: "graph_read", ProviderReference: ref, Detail: detail}, 1, 1, true},
		{"new definer fails", core.Outcome{State: "SUCCEEDED", Code: "graph_read", ProviderReference: ref, Detail: detail}, 2, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &breakdownTx{failAt: tc.failAt}
			err := FinishRefusal(context.Background(), tx, claim, tc.out)
			if (err != nil) != tc.wantErr || len(tx.sql) != tc.wantCalls {
				t.Fatalf("err=%v calls=%v", err, tx.sql)
			}
			if len(tx.sql) > 0 && !strings.Contains(tx.sql[0], "ads.finish_operation_refusal") {
				t.Fatalf("lost 0112: %v", tx.sql)
			}
			if len(tx.sql) == 2 {
				if !strings.Contains(tx.sql[1], "ads.finish_insights_breakdowns") || tx.args[1][0] != opID || tx.args[1][1] != int64(7) || string(tx.args[1][2].([]byte)) != "fixture-lease" || tx.args[1][3] != "dispatch" {
					t.Fatalf("seam=%v", tx.sql)
				}
				var payload InsightsBreakdowns
				if json.Unmarshal(tx.args[1][4].([]byte), &payload) != nil || payload.TimezoneName != "Asia/Taipei" || payload.Rows == nil {
					t.Fatal("invalid typed payload")
				}
			}
		})
	}
}

func TestInsightsTimezoneRefusalAndEmptyPurchases(t *testing.T) {
	for _, tc := range []struct {
		name, zone, day, hour string
		calls                 int
		success               bool
	}{
		{"unknown zone", "Invented/Zone", "2026-10-02", "", 1, false},
		{"host zone", "Local", "2026-10-02", "", 1, false},
		{"overlapping hour", "America/New_York", "2026-11-01", "01:00:00 - 01:59:59", 8, true},
		{"missing hour", "America/New_York", "2026-03-08", "02:00:00 - 02:59:59", 8, true},
		{"empty purchases", "Asia/Taipei", "2026-10-02", "", 8, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newFake(t, func(c call, w http.ResponseWriter) {
				q, _ := url.ParseQuery(c.query)
				switch {
				case strings.Contains(c.path, "act_"):
					json.NewEncoder(w).Encode(map[string]string{"currency": "TWD", "timezone_name": tc.zone})
				case !strings.HasSuffix(c.path, "/insights"):
					io.WriteString(w, `{"effective_status":"ACTIVE"}`)
				case q.Get("breakdowns") == "":
					io.WriteString(w, `{"data":[{"actions":[{"action_type":"omni_purchase","value":""}],"action_values":[{"action_type":"omni_purchase","value":""}]}]}`)
				case q.Get("breakdowns") == "hourly_stats_aggregated_by_advertiser_time_zone" && tc.hour != "":
					json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"date_start": tc.day, "hourly_stats_aggregated_by_advertiser_time_zone": tc.hour}}})
				default:
					io.WriteString(w, `{"data":[]}`)
				}
			})
			out := c.readInsights(context.Background(), asset, "555", tc.day, []byte(fakeToken))
			if len(f.log()) != tc.calls {
				t.Fatalf("calls=%d want %d", len(f.log()), tc.calls)
			}
			if tc.success {
				parsed, err := ParseInsights(out.ProviderReference)
				if out.State != "SUCCEEDED" || err != nil || parsed.Purchases != nil || parsed.PurchaseValueMinor != nil {
					t.Fatalf("empty metrics=%+v/%+v,%v", out, parsed, err)
				}
				detail := out.Detail.(*InsightsBreakdowns)
				if tc.hour != "" && (len(detail.Unavailable) != 1 || detail.Unavailable[0] != "hourly" || len(detail.Rows) != 0) {
					t.Fatalf("ambiguous/missing hour must remain unavailable: %+v", detail)
				}
			} else if out != failedFinal("bad_result") {
				t.Fatalf("out=%+v", out)
			}
		})
	}
}
