package metaads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"livecommerce/internal/integrations/core"
)

// InsightsBreakdowns is the non-PII D9/R7 projection passed only to the fenced
// ads.finish_insights_breakdowns definer. A pointer keeps core.Outcome comparable.
type InsightsBreakdowns struct {
	TimezoneName string                 `json:"timezone_name"`
	Currency     string                 `json:"currency"`
	Rows         []InsightsBreakdownRow `json:"rows"`
	Unavailable  []string               `json:"unavailable"`
}

type InsightsBreakdownRow struct {
	Dimension          string     `json:"dimension"`
	Bucket             string     `json:"bucket"`
	HourStart          *time.Time `json:"hour_start"`
	SpendMinor         *int64     `json:"spend_minor"`
	Reach              *int64     `json:"reach"`
	Impressions        *int64     `json:"impressions"`
	Clicks             *int64     `json:"clicks"`
	Engagements        *int64     `json:"engagements"`
	Comments           *int64     `json:"comments"`
	Purchases          *int64     `json:"purchases"`
	PurchaseValueMinor *int64     `json:"purchase_value_minor"`
}

// Official wire names, frozen by D9's read-only 2026-10-03 probe:
// https://developers.facebook.com/docs/marketing-api/insights/breakdowns/
// https://developers.facebook.com/docs/marketing-api/reference/ad-campaign-group/insights/
var breakdownDimensions = []struct{ dimension, wire string }{
	{"age_gender", "age,gender"}, {"region", "region"},
	{"placement", "publisher_platform,platform_position"}, {"device", "device_platform"},
	{"hourly", "hourly_stats_aggregated_by_advertiser_time_zone"},
}

type breakdownWireRow struct {
	Age               string        `json:"age"`
	Gender            string        `json:"gender"`
	Region            string        `json:"region"`
	PublisherPlatform string        `json:"publisher_platform"`
	PlatformPosition  string        `json:"platform_position"`
	DevicePlatform    string        `json:"device_platform"`
	Hour              string        `json:"hourly_stats_aggregated_by_advertiser_time_zone"`
	DateStart         string        `json:"date_start"`
	DateStop          string        `json:"date_stop"`
	Spend             string        `json:"spend"`
	Reach             string        `json:"reach"`
	Impressions       string        `json:"impressions"`
	Clicks            string        `json:"clicks"`
	Actions           []actionValue `json:"actions"`
	ActionValues      []actionValue `json:"action_values"`
}

func (c *Client) readBreakdowns(ctx context.Context, campaign, day, currency, timezone string, location *time.Location, token []byte) *InsightsBreakdowns {
	detail := &InsightsBreakdowns{TimezoneName: timezone, Currency: currency, Rows: make([]InsightsBreakdownRow, 0), Unavailable: make([]string, 0)}
	rng, _ := json.Marshal(map[string]string{"since": day, "until": day})
	for _, dimension := range breakdownDimensions {
		rows, failure := c.readBreakdownDimension(ctx, campaign, day, currency, location, token, rng, dimension.dimension, dimension.wire)
		if failure.State != "" {
			// R11: optional D9 evidence cannot veto the D7 daily result. Discard
			// the entire failed dimension; the next scheduled sweep reads it again.
			detail.Unavailable = append(detail.Unavailable, dimension.dimension)
			continue
		}
		detail.Rows = append(detail.Rows, rows...)
	}
	return detail
}

func (c *Client) readBreakdownDimension(ctx context.Context, campaign, day, currency string, location *time.Location, token, rng []byte, dimension, wireDimension string) ([]InsightsBreakdownRow, core.Outcome) {
	rows := make([]InsightsBreakdownRow, 0)
	q := url.Values{"fields": {"spend,reach,impressions,clicks,actions,action_values"},
		"time_range": {string(rng)}, "time_increment": {"1"}, "limit": {pageLimit}, "breakdowns": {wireDimension}}
	cursors, buckets := map[string]bool{}, map[string]bool{}
	for page := 0; page < maxPages; page++ {
		rep, err := c.g.do(ctx, http.MethodGet, campaign+"/insights", q, token, nil)
		if err != nil || !rep.ok() {
			// I06: no immediate retry and no partial dimension on transport uncertainty.
			return nil, failureOutcome(rep, err)
		}
		var doc struct {
			Data   []breakdownWireRow `json:"data"`
			Paging struct {
				Next    string `json:"next"`
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"paging"`
		}
		if json.Unmarshal(rep.body, &doc) != nil || doc.Data == nil || len(doc.Data) > 100 {
			return nil, unconfirmed()
		}
		for _, wire := range doc.Data {
			row, failure := normalizeBreakdown(wire, dimension, day, currency, location)
			if failure.State != "" {
				return nil, failure
			}
			if buckets[row.Bucket] {
				return nil, failedFinal("bad_result")
			}
			buckets[row.Bucket] = true
			rows = append(rows, row)
		}
		if doc.Paging.Next == "" {
			break
		}
		// I11: paging.next is untrusted and can contain credentials; only use its
		// presence, then rebuild a pinned path with a validated, non-repeating cursor.
		after := doc.Paging.Cursors.After
		if !cursorPattern.MatchString(after) || cursors[after] || page == maxPages-1 {
			return nil, unconfirmed()
		}
		cursors[after] = true
		q.Set("after", after)
	}
	return rows, core.Outcome{}
}

func normalizeBreakdown(w breakdownWireRow, dimension, day, currency string, location *time.Location) (InsightsBreakdownRow, core.Outcome) {
	r := InsightsBreakdownRow{Dimension: dimension}
	if (w.DateStart != "" && w.DateStart != day) || (w.DateStop != "" && w.DateStop != day) {
		return r, failedFinal("bad_result")
	}
	var parts []string
	switch dimension {
	case "age_gender":
		parts = []string{w.Age, w.Gender}
	case "region":
		parts = []string{w.Region}
	case "placement":
		parts = []string{w.PublisherPlatform, w.PlatformPosition}
	case "device":
		parts = []string{w.DevicePlatform}
	case "hourly":
		parts = []string{w.Hour}
		hour, err := absoluteInsightHour(day, w.Hour, location)
		if err != nil {
			return r, failedFinal("bad_result")
		}
		r.HourStart = &hour
	default:
		return r, failedFinal("bad_result")
	}
	for _, part := range parts {
		if !plainBucket(part) {
			return r, failedFinal("bad_result")
		}
	}
	r.Bucket = strings.Join(parts, "/")
	if !plainBucket(r.Bucket) {
		return r, failedFinal("bad_result")
	}
	// I05: minor units use the existing exact currency conversion, never rounding.
	// I12: preserve omitted metrics as unknown, while explicit zero stays zero.
	if w.Spend != "" {
		n, err := SpendMinor(currency, w.Spend)
		if err != nil {
			return r, spendFailure(err)
		}
		r.SpendMinor = &n
	}
	for _, metric := range []struct {
		wire   string
		target **int64
	}{{w.Reach, &r.Reach}, {w.Impressions, &r.Impressions}, {w.Clicks, &r.Clicks}} {
		if metric.wire == "" {
			continue
		}
		n, err := count(metric.wire)
		if err != nil {
			return r, failedFinal("bad_result")
		}
		*metric.target = &n
	}
	seen := map[string]bool{}
	for _, a := range w.Actions {
		if a.Type != "post_engagement" && a.Type != "comment" && a.Type != purchaseAction {
			continue
		}
		if seen[a.Type] {
			return r, failedFinal("bad_result")
		}
		seen[a.Type] = true
		// I12: empty purchase metrics mean withheld/unknown, not a measured zero.
		if a.Value == "" {
			continue
		}
		n, err := count(a.Value)
		if err != nil {
			return r, failedFinal("bad_result")
		}
		switch a.Type {
		case "post_engagement":
			r.Engagements = &n
		case "comment":
			r.Comments = &n
		case purchaseAction:
			r.Purchases = &n
		}
	}
	foundValue := false
	for _, a := range w.ActionValues {
		if a.Type != purchaseAction {
			continue
		}
		if foundValue {
			return r, failedFinal("bad_result")
		}
		foundValue = true
		if a.Value == "" {
			continue
		}
		n, err := SpendMinor(currency, a.Value)
		if err != nil {
			return r, spendFailure(err)
		}
		r.PurchaseValueMinor = &n
	}
	return r, core.Outcome{}
}

func plainBucket(s string) bool {
	if s == "" || strings.TrimSpace(s) != s || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 160 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || !unicode.IsPrint(r) || r == '<' || r == '>' {
			return false
		}
	}
	return true
}

var hourBucketPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):00:00 - ([01][0-9]|2[0-3]):59:59$`)

func insightLocation(name string) (*time.Location, error) {
	// "Local" would depend on the worker's host instead of the account's IANA zone.
	if name == "" || name == "Local" {
		return nil, ErrGrammar
	}
	return time.LoadLocation(name)
}

func absoluteInsightHour(day, bucket string, location *time.Location) (time.Time, error) {
	m := hourBucketPattern.FindStringSubmatch(bucket)
	if m == nil || m[1] != m[2] || location == nil {
		return time.Time{}, ErrGrammar
	}
	wall, err := time.Parse("2006-01-02 15", day+" "+m[1])
	if err != nil {
		return time.Time{}, ErrGrammar
	}
	// Enumerate adjacent zone periods, including half-hour DST changes, then
	// round-trip every possible offset. time.Date alone silently chooses an
	// arbitrary side of an overlap and normalizes nonexistent local times.
	offsets := map[int]bool{}
	end := wall.Add(48 * time.Hour)
	for probe := wall.Add(-48 * time.Hour); !probe.After(end); {
		local := probe.In(location)
		_, offset := local.Zone()
		offsets[offset] = true
		_, until := local.ZoneBounds()
		if until.IsZero() || until.After(end) {
			break
		}
		if !until.After(probe) {
			return time.Time{}, ErrGrammar
		}
		probe = until
	}
	var found, foundEnd time.Time
	wallEnd := wall.Add(time.Hour - time.Second)
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		if candidate.In(location).Format("2006-01-02 15:04:05") == wall.Format("2006-01-02 15:04:05") {
			if !found.IsZero() {
				return time.Time{}, ErrGrammar
			}
			found = candidate.UTC()
		}
		candidateEnd := wallEnd.Add(-time.Duration(offset) * time.Second)
		if candidateEnd.In(location).Format("2006-01-02 15:04:05") == wallEnd.Format("2006-01-02 15:04:05") {
			if !foundEnd.IsZero() {
				return time.Time{}, ErrGrammar
			}
			foundEnd = candidateEnd.UTC()
		}
	}
	// Refuse a partially repeated hour too (Lord Howe has half-hour DST).
	if found.IsZero() || foundEnd.IsZero() || foundEnd.Sub(found) != time.Hour-time.Second {
		return time.Time{}, ErrGrammar
	}
	return found, nil
}
