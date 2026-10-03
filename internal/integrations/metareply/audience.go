package metareply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/platform"
)

// LiveAudienceSnapshot is D9's route-private aggregate; a pointer keeps
// core.Outcome comparable. Peak/total time stay nil because no verified wire
// metrics provide them. Demographic amounts are view time, never people.
type LiveAudienceSnapshot struct {
	Status          string               `json:"status"`
	Views           *int64               `json:"views"`
	PeakConcurrent  *int64               `json:"peak_concurrent"`
	TotalViewTimeMS *int64               `json:"total_view_time_ms"`
	AgeGender       []LiveAudienceBucket `json:"age_gender"`
	Regions         []LiveAudienceBucket `json:"regions"`
}

type LiveAudienceBucket struct {
	Bucket     string `json:"bucket"`
	ViewTimeMS int64  `json:"view_time_ms"`
}

// D9 verified these wire metrics in the read-only 2026-10-03 probe:
// https://developers.facebook.com/docs/graph-api/reference/video/video_insights/
// Live listing: https://developers.facebook.com/docs/graph-api/reference/page/live_videos/
// Permissions: https://developers.facebook.com/docs/permissions/reference/read_insights/
const audienceMetrics = "total_video_views,total_video_view_time_by_age_bucket_and_gender,total_video_view_time_by_region_id"
const audienceCallTimeout = 15 * time.Second
const audiencePageLimit = 10

var audienceCursorPattern = regexp.MustCompile(`^[A-Za-z0-9_=+/-]{1,512}$`)
var audienceCountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var audienceCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var audiencePostPattern = regexp.MustCompile(`^[0-9]{1,40}(_[0-9]{1,40})?$`)

// AudienceRoutes registers one read-only Page route with the claims worker's
// existing AES/HPKE private custody; no messaging scope or new credential path.
func AudienceRoutes(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) ([]core.DispatchRoute, error) {
	if pool == nil {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), pool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	check := func(ctx context.Context, op string) (code string, err error) {
		// integration.check_meta_audience: frozen source/session/Page authorization.
		err = pool.QueryRow(ctx, `SELECT integration.check_meta_audience($1::uuid)`, op).Scan(&code)
		return code, err
	}
	route, err := newAudienceRoute(check, keys, v2, cfg)
	if err != nil {
		return nil, err
	}
	return []core.DispatchRoute{route}, nil
}

func newAudienceRoute(check func(context.Context, string) (string, error), keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) (core.DispatchRoute, error) {
	if check == nil || keys == nil || cfg.Validate() != nil {
		return core.DispatchRoute{}, ErrConfig
	}
	graph, err := metaoauth.NewGraph(cfg.GraphBaseURL, cfg.GraphVersion, cfg.HTTPClient)
	if err != nil {
		return core.DispatchRoute{}, ErrConfig
	}
	a := &audienceAdapter{check: check, graph: graph}
	return core.DispatchRoute{
		Provider: "facebook", Action: "meta.live_insights", Purpose: "service",
		Check: a.checkRoute,
		// integration.load_meta_audience_token: scopes and lease are independent
		// of private-reply eligibility; reuse only the credential custody.
		LoadSecret: pageSecretLoader(keys, v2, "facebook", []string{"read_insights", "pages_read_engagement"},
			`SELECT tenant_id::text,store_id::text,binding_id::text,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested FROM integration.load_meta_audience_token($1::uuid,$2::bigint,$3::bytea)`),
		DispatchWithSecret: a.dispatch,
		// I06: UNKNOWN never triggers automatic Graph replay in reconciliation.
		Reconcile: func(context.Context, core.DispatchRequest) (core.Outcome, error) {
			return audienceUnknown(codeUnproven), nil
		},
		Finish: finishAudience,
	}, nil
}

type audienceAdapter struct {
	check func(context.Context, string) (string, error)
	graph *metaoauth.Graph
}
type audienceRequestBody struct {
	V         int    `json:"v"`
	SourceID  string `json:"source_id"`
	SessionID string `json:"session_id"`
	PostID    string `json:"post_id"`
	AssetID   string `json:"asset_id"`
}

func parseAudienceRequest(req core.DispatchRequest) (r audienceRequestBody, ok bool) {
	decoder := json.NewDecoder(bytes.NewReader(req.Request))
	decoder.DisallowUnknownFields()
	var extra json.RawMessage
	if decoder.Decode(&r) != nil || decoder.Decode(&extra) != io.EOF || r.V != 1 || !command.ValidID(r.SourceID) || !command.ValidID(r.SessionID) || !assetPattern.MatchString(r.AssetID) || r.AssetID != req.ExternalAssetID || !audiencePostPattern.MatchString(r.PostID) || req.Provider != "facebook" || req.Action != "meta.live_insights" || req.Purpose != "service" {
		return audienceRequestBody{}, false
	}
	if strings.Contains(r.PostID, "_") && strings.SplitN(r.PostID, "_", 2)[0] != r.AssetID {
		return audienceRequestBody{}, false
	}
	return r, true
}

func (a *audienceAdapter) checkRoute(ctx context.Context, req core.DispatchRequest) error {
	if _, ok := parseAudienceRequest(req); !ok {
		return core.DenyPolicy("bad_request")
	}
	code, err := a.check(ctx, req.OperationID)
	if err != nil {
		return errors.New("metareply: audience policy check failed")
	}
	if code == "" {
		return nil
	}
	if !audienceCodePattern.MatchString(code) {
		code = "policy_denied"
	}
	return core.DenyPolicy(code)
}

func audienceUnknown(code string) core.Outcome { return core.Outcome{State: "UNKNOWN", Code: code} }
func audienceBadResult() core.Outcome          { return core.Outcome{State: "FAILED_FINAL", Code: "bad_result"} }

func audienceFailure(rep metaoauth.Reply, err error) core.Outcome {
	// I06: transport/5xx/redirect uncertainty stops the snapshot, never retries.
	if err != nil {
		return audienceUnknown(codeUnconfirmed)
	}
	if rep.Status == 403 || graphErrorCode(rep.Body) == 190 {
		return core.Outcome{State: "FAILED_FINAL", Code: "permission_denied"}
	}
	if rep.Status >= 400 && rep.Status <= 499 {
		return core.Outcome{State: "FAILED_FINAL", Code: "graph_refused"}
	}
	return audienceUnknown(codeUnconfirmed)
}

func (a *audienceAdapter) edge(ctx context.Context, path string, query url.Values, token []byte) ([]json.RawMessage, core.Outcome) {
	rows := make([]json.RawMessage, 0)
	seen := map[string]bool{}
	query.Set("limit", "100")
	for page := 0; page < audiencePageLimit; page++ {
		rep, err := a.graph.Do(ctx, http.MethodGet, path, query, token, nil)
		if err != nil || !rep.OK() {
			return nil, audienceFailure(rep, err)
		}
		var doc struct {
			Data   []json.RawMessage `json:"data"`
			Paging struct {
				Next    string `json:"next"`
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"paging"`
		}
		if json.Unmarshal(rep.Body, &doc) != nil || doc.Data == nil || len(doc.Data) > 100 {
			return nil, audienceUnknown(codeUnconfirmed)
		}
		rows = append(rows, doc.Data...)
		if doc.Paging.Next == "" {
			return rows, core.Outcome{}
		}
		// I11: never follow response URLs or move the Page token into a URL.
		after := doc.Paging.Cursors.After
		if !audienceCursorPattern.MatchString(after) || seen[after] || page == audiencePageLimit-1 {
			return nil, audienceUnknown(codeUnproven)
		}
		seen[after] = true
		query.Set("after", after)
	}
	return nil, audienceUnknown(codeUnproven)
}

func (a *audienceAdapter) dispatch(ctx context.Context, req core.DispatchRequest, secret core.Secret) (core.Outcome, error) {
	r, ok := parseAudienceRequest(req)
	if !ok || len(secret.Reveal()) == 0 {
		return core.Outcome{State: "FAILED_FINAL", Code: "bad_request"}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, audienceCallTimeout)
	defer cancel()
	rows, failure := a.edge(ctx, r.AssetID+"/live_videos", url.Values{"fields": {"id,post_id"}}, secret.Reveal())
	if failure.State != "" {
		return failure, nil
	}
	video := ""
	for _, raw := range rows {
		var row struct {
			ID     string `json:"id"`
			PostID string `json:"post_id"`
		}
		if json.Unmarshal(raw, &row) != nil || !assetPattern.MatchString(row.ID) || !audiencePostPattern.MatchString(row.PostID) {
			return audienceBadResult(), nil
		}
		if row.PostID == r.PostID {
			if video != "" && video != row.ID {
				return audienceUnknown(codeUnproven), nil
			}
			video = row.ID
		}
	}
	if video == "" {
		return audienceUnknown(codeUnproven), nil
	}
	rows, failure = a.edge(ctx, video+"/video_insights", url.Values{"metric": {audienceMetrics}}, secret.Reveal())
	if failure.State != "" {
		return failure, nil
	}
	snapshot, ok := parseAudienceMetrics(rows)
	if !ok {
		return audienceBadResult(), nil
	}
	return core.Outcome{State: "SUCCEEDED", Code: "graph_read", Detail: snapshot}, nil
}

func parseAudienceMetrics(rows []json.RawMessage) (*LiveAudienceSnapshot, bool) {
	snapshot := &LiveAudienceSnapshot{Status: "insufficient", AgeGender: make([]LiveAudienceBucket, 0), Regions: make([]LiveAudienceBucket, 0)}
	seen := map[string]bool{}
	for _, raw := range rows {
		var metric struct {
			Name   string `json:"name"`
			Period string `json:"period"`
			Values []struct {
				Value json.RawMessage `json:"value"`
			} `json:"values"`
		}
		if json.Unmarshal(raw, &metric) != nil || (metric.Period != "" && metric.Period != "lifetime") || seen[metric.Name] || len(metric.Values) > 1 {
			return nil, false
		}
		if metric.Name != "total_video_views" && metric.Name != "total_video_view_time_by_age_bucket_and_gender" && metric.Name != "total_video_view_time_by_region_id" {
			return nil, false
		}
		seen[metric.Name] = true
		if len(metric.Values) == 0 || len(metric.Values[0].Value) == 0 || bytes.Equal(metric.Values[0].Value, []byte("null")) {
			continue
		}
		value := metric.Values[0].Value
		if metric.Name == "total_video_views" {
			n, ok := audienceInteger(value)
			if !ok {
				return nil, false
			}
			snapshot.Views = &n
		} else {
			buckets, ok := audienceBuckets(value)
			if !ok {
				return nil, false
			}
			if metric.Name == "total_video_view_time_by_age_bucket_and_gender" {
				snapshot.AgeGender = buckets
			} else {
				snapshot.Regions = buckets
			}
		}
	}
	// I12: withholding/empty demographics remain insufficient even when views are known.
	if len(snapshot.AgeGender) > 0 || len(snapshot.Regions) > 0 {
		snapshot.Status = "available"
	}
	return snapshot, true
}

func audienceInteger(raw json.RawMessage) (int64, bool) {
	s := string(raw)
	if len(s) > 0 && s[0] == '"' {
		if json.Unmarshal(raw, &s) != nil {
			return 0, false
		}
	}
	if !audienceCountPattern.MatchString(s) {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

func audienceBuckets(raw json.RawMessage) ([]LiveAudienceBucket, bool) {
	buckets := make([]LiveAudienceBucket, 0)
	var empty []json.RawMessage
	if json.Unmarshal(raw, &empty) == nil && empty != nil && len(empty) == 0 {
		return buckets, true
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil || len(values) > 1000 {
		return nil, false
	}
	labels := make([]string, 0, len(values))
	for label := range values {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		if !audienceLabel(label) {
			return nil, false
		}
		n, ok := audienceInteger(values[label])
		if !ok {
			return nil, false
		}
		buckets = append(buckets, LiveAudienceBucket{Bucket: label, ViewTimeMS: n})
	}
	return buckets, true
}

func audienceLabel(s string) bool {
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

func finishAudience(ctx context.Context, tx pgx.Tx, claim core.SecretClaim, out core.Outcome) error {
	snapshot, ok := out.Detail.(*LiveAudienceSnapshot)
	if out.State != "SUCCEEDED" || out.Code != "graph_read" || !ok || snapshot == nil {
		return nil
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	// integration.finish_meta_audience: root-owned source/session and lease-fenced
	// aggregate writer, in the same transaction as dispatcher completion.
	_, err = tx.Exec(ctx, `SELECT integration.finish_meta_audience($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb)`, claim.OperationID, claim.Generation, claim.LeaseToken, claim.Mode, raw)
	return err
}
