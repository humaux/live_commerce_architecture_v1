package metaads

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestR11OptionalDimensionFailureKeepsDaily(t *testing.T) {
	for _, tc := range []struct {
		name, body, day, zone, dimension string
		status                           int
	}{
		{"refused", `{"error":{"code":100}}`, "2026-10-02", "Asia/Taipei", "region", 400},
		{"server", `{}`, "2026-10-02", "Asia/Taipei", "region", 503},
		{"truncated transport", `{`, "2026-10-02", "Asia/Taipei", "region", -1},
		{"malformed JSON", `{`, "2026-10-02", "Asia/Taipei", "region", 200},
		{"malformed label", `{"data":[{"region":"Taipei"},{"region":"<script>"}]}`, "2026-10-02", "Asia/Taipei", "region", 200},
		{"DST ambiguity", `{"data":[{"hourly_stats_aggregated_by_advertiser_time_zone":"01:00:00 - 01:59:59"}]}`, "2026-11-01", "America/New_York", "hourly", 200},
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
					io.WriteString(w, `{"data":[{"spend":"12.30","impressions":"9","clicks":"2"}]}`)
				case q.Get("breakdowns") == "age,gender":
					io.WriteString(w, `{"data":[{"age":"25-34","gender":"female","reach":"0"}]}`)
				case q.Get("breakdowns") == "region" && tc.dimension == "region", q.Get("breakdowns") == "hourly_stats_aggregated_by_advertiser_time_zone" && tc.dimension == "hourly":
					if tc.status == -1 {
						// A partial response produces a transport read error, without
						// scheduling a retry or waiting on wall-clock timeouts.
						w.Header().Set("Content-Length", "1000")
					} else {
						w.WriteHeader(tc.status)
					}
					io.WriteString(w, tc.body)
				default:
					io.WriteString(w, `{"data":[]}`)
				}
			})
			for sweep := 0; sweep < 2; sweep++ {
				out := c.readInsights(context.Background(), asset, "555", tc.day, []byte(fakeToken))
				day, err := ParseInsights(out.ProviderReference)
				if out.State != "SUCCEEDED" || err != nil || day.SpendMinor != 1230 {
					t.Fatalf("daily lost: %+v %v", out, err)
				}
				raw, _ := json.Marshal(out.Detail)
				var detail struct {
					Unavailable []string         `json:"unavailable"`
					Rows        []map[string]any `json:"rows"`
				}
				if err = json.Unmarshal(raw, &detail); err != nil || !reflect.DeepEqual(detail.Unavailable, []string{tc.dimension}) || len(detail.Rows) != 1 || detail.Rows[0]["dimension"] != "age_gender" {
					t.Fatalf("partial dimension leaked/healthy lost: %s %v", raw, err)
				}
				if len(f.log()) != 8*(sweep+1) {
					t.Fatalf("next sweep must repeat each dimension once: calls=%d", len(f.log()))
				}
			}
		})
	}
}

func TestR11AllDimensionsUnavailableKeepsDaily(t *testing.T) {
	c, f := newFake(t, func(c call, w http.ResponseWriter) {
		q, _ := url.ParseQuery(c.query)
		switch {
		case strings.Contains(c.path, "act_"):
			io.WriteString(w, `{"currency":"TWD","timezone_name":"Asia/Taipei"}`)
		case !strings.HasSuffix(c.path, "/insights"):
			io.WriteString(w, `{"effective_status":"ACTIVE"}`)
		case q.Get("breakdowns") == "":
			io.WriteString(w, `{"data":[{"spend":"1.23"}]}`)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{}`)
		}
	})
	out := c.readInsights(context.Background(), asset, "555", "2026-10-02", []byte(fakeToken))
	day, err := ParseInsights(out.ProviderReference)
	if out.State != "SUCCEEDED" || err != nil || day.SpendMinor != 123 {
		t.Fatalf("daily: %+v %v", out, err)
	}
	detail := out.Detail.(*InsightsBreakdowns)
	if !reflect.DeepEqual(detail.Unavailable, []string{"age_gender", "region", "placement", "device", "hourly"}) || detail.Rows == nil || len(detail.Rows) != 0 || len(f.log()) != 8 {
		t.Fatalf("all unavailable: %+v calls=%d", detail, len(f.log()))
	}
}

func TestR11SecondPageFailureDiscardsOnlyThatDimension(t *testing.T) {
	c, f := newFake(t, func(c call, w http.ResponseWriter) {
		q, _ := url.ParseQuery(c.query)
		switch {
		case strings.Contains(c.path, "act_"):
			io.WriteString(w, `{"currency":"TWD","timezone_name":"Asia/Taipei"}`)
		case !strings.HasSuffix(c.path, "/insights"):
			io.WriteString(w, `{"effective_status":"ACTIVE"}`)
		case q.Get("breakdowns") == "":
			io.WriteString(w, `{"data":[{"spend":"1.23"}]}`)
		case q.Get("breakdowns") == "age,gender" && q.Get("after") == "":
			io.WriteString(w, `{"data":[{"age":"25-34","gender":"female"}],"paging":{"next":"https://untrusted.invalid","cursors":{"after":"page2"}}}`)
		case q.Get("breakdowns") == "age,gender":
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{}`)
		case q.Get("breakdowns") == "region":
			io.WriteString(w, `{"data":[{"region":"Taipei","spend":"0"}]}`)
		default:
			io.WriteString(w, `{"data":[]}`)
		}
	})
	out := c.readInsights(context.Background(), asset, "555", "2026-10-02", []byte(fakeToken))
	if out.State != "SUCCEEDED" {
		t.Fatalf("daily: %+v", out)
	}
	detail := out.Detail.(*InsightsBreakdowns)
	if !reflect.DeepEqual(detail.Unavailable, []string{"age_gender"}) || len(detail.Rows) != 1 || detail.Rows[0].Dimension != "region" || detail.Rows[0].SpendMinor == nil || *detail.Rows[0].SpendMinor != 0 || len(f.log()) != 9 {
		t.Fatalf("dimension atomicity: %+v calls=%d", detail, len(f.log()))
	}
}

func TestR11OmittedBreakdownMetricsStayNull(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire breakdownWireRow
		zero bool
	}{
		{"omitted", breakdownWireRow{Region: "Taipei"}, false},
		{"empty actions", breakdownWireRow{Region: "Taipei", Actions: []actionValue{{Type: "comment", Value: ""}, {Type: "post_engagement", Value: ""}}}, false},
		{"explicit zero", breakdownWireRow{Region: "Taipei", Spend: "0", Reach: "0", Impressions: "0", Clicks: "0", Actions: []actionValue{{Type: "comment", Value: "0"}, {Type: "post_engagement", Value: "0"}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, failure := normalizeBreakdown(tc.wire, "region", "2026-10-02", "TWD", nil)
			if failure.State != "" {
				t.Fatal(failure)
			}
			raw, _ := json.Marshal(row)
			var values map[string]any
			json.Unmarshal(raw, &values)
			for _, key := range []string{"spend_minor", "reach", "impressions", "clicks", "engagements", "comments"} {
				v, ok := values[key]
				if !ok || (!tc.zero && v != nil) || (tc.zero && v != float64(0)) {
					t.Fatalf("%s: %s must preserve missing/explicit zero", key, raw)
				}
			}
		})
	}
}
