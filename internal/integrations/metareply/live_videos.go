// Purpose: A5-3 Page "live videos" picker read (MOCK) — one read-only Graph GET /{page}/live_videos with pages_read_engagement only, normalized into a bounded (≤25, LIVE-first) snapshot. No Meta mutation, no viewer/buyer data; the Page token stays in the worker process (the API plan never loads it).
// Depends on: core.DispatchRoute/Outcome/Secret, metaoauth.Graph, pageSecretLoader (AES/HPKE custody), integration.check/load/finish_meta_live_videos (0114), routes.go (codeUnconfirmed/codeUnproven).
// Used by: cmd/claims-worker (LiveVideoRoutes); internal/integrations/metareply/live_videos_test.go.
package metareply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
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

const liveVideosCallTimeout = 15 * time.Second
const liveVideosPageLimit = 25

// liveVideosFields is the single-page, no-pagination field list (R5: status is unverified; when it is
// absent every video is listed and sorted by creation_time descending).
const liveVideosFields = "id,post_id,title,status,creation_time"

// liveVideoItem is the normalized snapshot item: post_id is <page_id>_<post>, title is stripped of
// control characters and bounded to 255 runes, started_at is the raw creation_time (bounded, no
// parsing — Meta's +0000 offset is not RFC 3339).
type liveVideoItem struct {
	VideoID   string `json:"video_id"`
	PostID    string `json:"post_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
}

// LiveVideoRoutes registers one read-only Page route with the claims worker's existing AES/HPKE
// private custody; no new credential path, no messaging scope.
func LiveVideoRoutes(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) ([]core.DispatchRoute, error) {
	if pool == nil {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), pool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	check := func(ctx context.Context, op string) (code string, err error) {
		// integration.check_meta_live_videos: principal + exact bound Page recheck.
		err = pool.QueryRow(ctx, `SELECT integration.check_meta_live_videos($1::uuid)`, op).Scan(&code)
		return code, err
	}
	route, err := newLiveVideoRoute(check, keys, v2, cfg)
	if err != nil {
		return nil, err
	}
	return []core.DispatchRoute{route}, nil
}

func newLiveVideoRoute(check func(context.Context, string) (string, error), keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) (core.DispatchRoute, error) {
	if check == nil || keys == nil || cfg.Validate() != nil {
		return core.DispatchRoute{}, ErrConfig
	}
	graph, err := metaoauth.NewGraph(cfg.GraphBaseURL, cfg.GraphVersion, cfg.HTTPClient)
	if err != nil {
		return core.DispatchRoute{}, ErrConfig
	}
	a := &liveVideosAdapter{check: check, graph: graph}
	return core.DispatchRoute{
		Provider: "facebook", Action: "meta.live_videos", Purpose: "service",
		Check: a.checkRoute,
		// integration.load_meta_live_videos_token: exact dispatch lease and pages_read_engagement only.
		LoadSecret: pageSecretLoader(keys, v2, "facebook", []string{"pages_read_engagement"},
			`SELECT tenant_id::text,store_id::text,binding_id::text,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested FROM integration.load_meta_live_videos_token($1::uuid,$2::bigint,$3::bytea)`),
		DispatchWithSecret: a.dispatch,
		// I06: UNKNOWN never triggers automatic Graph replay in reconciliation.
		Reconcile: func(context.Context, core.DispatchRequest) (core.Outcome, error) {
			return liveVideosUnknown(codeUnproven), nil
		},
		Finish: finishLiveVideos,
	}, nil
}

type liveVideosAdapter struct {
	check func(context.Context, string) (string, error)
	graph *metaoauth.Graph
}

// liveVideosRequest is the frozen operation request the plan SQL wrote (v=1, binding_id, asset_id).
type liveVideosRequest struct {
	V         int    `json:"v"`
	BindingID string `json:"binding_id"`
	AssetID   string `json:"asset_id"`
}

func parseLiveVideosRequest(req core.DispatchRequest) (r liveVideosRequest, ok bool) {
	decoder := json.NewDecoder(bytes.NewReader(req.Request))
	decoder.DisallowUnknownFields()
	var extra json.RawMessage
	if decoder.Decode(&r) != nil || decoder.Decode(&extra) != io.EOF || r.V != 1 ||
		!command.ValidID(r.BindingID) || r.BindingID != req.BindingID ||
		!assetPattern.MatchString(r.AssetID) || r.AssetID != req.ExternalAssetID ||
		req.Provider != "facebook" || req.Action != "meta.live_videos" || req.Purpose != "service" {
		return liveVideosRequest{}, false
	}
	return r, true
}

func (a *liveVideosAdapter) checkRoute(ctx context.Context, req core.DispatchRequest) error {
	if _, ok := parseLiveVideosRequest(req); !ok {
		return core.DenyPolicy("bad_request")
	}
	code, err := a.check(ctx, req.OperationID)
	if err != nil {
		return errors.New("metareply: live videos policy check failed")
	}
	if code == "" {
		return nil
	}
	if !audienceCodePattern.MatchString(code) {
		code = "policy_denied"
	}
	return core.DenyPolicy(code)
}

func liveVideosUnknown(code string) core.Outcome { return core.Outcome{State: "UNKNOWN", Code: code} }

func liveVideosFailure(rep metaoauth.Reply, err error) core.Outcome {
	// I06: transport/5xx/redirect uncertainty stops the snapshot, never retries the Graph read.
	if err != nil {
		return liveVideosUnknown(codeUnconfirmed)
	}
	if rep.Status == 403 || graphErrorCode(rep.Body) == 190 {
		return core.Outcome{State: "FAILED_FINAL", Code: "permission_denied"}
	}
	if rep.Status >= 400 && rep.Status <= 499 {
		return core.Outcome{State: "FAILED_FINAL", Code: "graph_refused"}
	}
	return liveVideosUnknown(codeUnconfirmed)
}

func (a *liveVideosAdapter) dispatch(ctx context.Context, req core.DispatchRequest, secret core.Secret) (core.Outcome, error) {
	r, ok := parseLiveVideosRequest(req)
	if !ok || len(secret.Reveal()) == 0 {
		return core.Outcome{State: "FAILED_FINAL", Code: "bad_request"}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, liveVideosCallTimeout)
	defer cancel()
	// Single page, never paginated (the snapshot is bounded to 25 rows).
	rep, err := a.graph.Do(ctx, http.MethodGet, r.AssetID+"/live_videos",
		url.Values{"fields": {liveVideosFields}, "limit": {"25"}}, secret.Reveal(), nil)
	if err != nil || !rep.OK() {
		return liveVideosFailure(rep, err), nil
	}
	var doc struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(rep.Body, &doc) != nil || doc.Data == nil || len(doc.Data) > liveVideosPageLimit {
		return liveVideosUnknown(codeUnconfirmed), nil
	}
	items, ok := normalizeLiveVideos(doc.Data, r.AssetID)
	if !ok {
		return core.Outcome{State: "FAILED_FINAL", Code: "bad_result"}, nil
	}
	return core.Outcome{State: "SUCCEEDED", Code: "graph_read", Detail: items}, nil
}

// normalizeLiveVideos validates and normalizes the rows: numeric video ids, post_id prefixed to
// <page_id>_<post> (a mismatched prefix fails the whole result), title/status/started_at bounded and
// stripped of control characters, then LIVE first and creation_time descending.
func normalizeLiveVideos(rows []json.RawMessage, assetID string) ([]liveVideoItem, bool) {
	items := make([]liveVideoItem, 0, len(rows))
	for _, raw := range rows {
		var row struct {
			ID           string `json:"id"`
			PostID       string `json:"post_id"`
			Title        string `json:"title"`
			Status       string `json:"status"`
			CreationTime string `json:"creation_time"`
		}
		if json.Unmarshal(raw, &row) != nil || !assetPattern.MatchString(row.ID) || !audiencePostPattern.MatchString(row.PostID) {
			return nil, false
		}
		postID := row.PostID
		if strings.Contains(postID, "_") {
			if strings.SplitN(postID, "_", 2)[0] != assetID {
				return nil, false
			}
		} else {
			postID = assetID + "_" + postID
		}
		items = append(items, liveVideoItem{
			VideoID:   row.ID,
			PostID:    postID,
			Title:     sanitizeLiveToken(row.Title, 255),
			Status:    sanitizeLiveToken(row.Status, 40),
			StartedAt: sanitizeLiveToken(row.CreationTime, 64),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		liveI, liveJ := items[i].Status == "LIVE", items[j].Status == "LIVE"
		if liveI != liveJ {
			return liveI
		}
		return items[i].StartedAt > items[j].StartedAt
	})
	return items, true
}

// sanitizeLiveToken bounds one untrusted Graph string: valid UTF-8, control characters removed,
// at most max runes. An invalid string is the empty string, never an error.
func sanitizeLiveToken(s string, max int) string {
	if !utf8.ValidString(s) {
		return ""
	}
	cap := len(s)
	if cap > max {
		cap = max
	}
	out := make([]rune, 0, cap)
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		out = append(out, r)
		if len(out) >= max {
			break
		}
	}
	return string(out)
}

// finishLiveVideos writes the bounded snapshot on a terminal SUCCEEDED/FAILED_FINAL completion; any
// UNKNOWN/BLOCKED_POLICY/ACKNOWLEDGED outcome leaves the snapshot absent so the GET read falls back
// to the operation state (pending/unknown/failed).
func finishLiveVideos(ctx context.Context, tx pgx.Tx, claim core.SecretClaim, out core.Outcome) error {
	var result struct {
		State string          `json:"state"`
		Code  *string         `json:"code"`
		Items json.RawMessage `json:"items"`
	}
	switch out.State {
	case "SUCCEEDED":
		items, ok := out.Detail.([]liveVideoItem)
		if !ok {
			return nil
		}
		raw, err := json.Marshal(items)
		if err != nil {
			return err
		}
		result.State, result.Items = "succeeded", raw
	case "FAILED_FINAL":
		code := out.Code
		if code == "" {
			code = "graph_refused"
		}
		result.State, result.Code, result.Items = "failed", &code, json.RawMessage("[]")
	default:
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	// integration.finish_meta_live_videos: lease-fenced, latest-wins bounded snapshot in the completion transaction.
	_, err = tx.Exec(ctx, `SELECT integration.finish_meta_live_videos($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb)`,
		claim.OperationID, claim.Generation, claim.LeaseToken, claim.Mode, raw)
	return err
}
