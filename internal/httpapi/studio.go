package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

func registerStudioRoutes(mux *http.ServeMux, pool *pgxpool.Pool, planner *live.MediaPlanner) {
	if planner == nil {
		return
	}
	const base = "/v1/admin/stores/{store_id}/live-sessions"
	mux.HandleFunc("GET "+base, studioRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		page, err := studioPage(r.URL)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return live.ListDrafts(ctx, tx, s, bearerToken(r), page)
		})(w, r)
	}))
	mux.HandleFunc("POST "+base, studioRoute(http.MethodPost, false, studioBodyRoute(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in live.DraftInput) (any, error) {
		return live.CreateDraft(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
	})))
	mux.HandleFunc("GET "+base+"/{session_id}", studioRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(r.PathValue("session_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return live.GetStudio(ctx, tx, s, bearerToken(r), r.PathValue("session_id"))
		})(w, r)
	}))
	type edit struct {
		live.DraftInput
		ExpectedVersion int64 `json:"expected_version"`
	}
	mux.HandleFunc("PATCH "+base+"/{session_id}", studioRoute(http.MethodPatch, false, studioBodyRoute(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in edit) (any, error) {
		return live.UpdateDraft(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), in.ExpectedVersion, in.DraftInput)
	})))
	type start struct {
		AuthorizationID        string `json:"authorization_id"`
		ExpectedSessionVersion int64  `json:"expected_session_version"`
	}
	mux.HandleFunc("POST "+base+"/{session_id}/rehearsal/start", studioRoute(http.MethodPost, false, studioBodyRoute(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in start) (any, error) {
		out, err := planner.PlanStart(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), live.MediaStartInput{SessionID: r.PathValue("session_id"), AuthorizationID: in.AuthorizationID, ExpectedSessionVersion: in.ExpectedSessionVersion})
		if err != nil {
			return nil, err
		}
		return studioReceipt{SessionID: out.SessionID, AttemptID: out.AttemptID, State: out.State}, nil
	})))
	type stop struct {
		AttemptID string `json:"attempt_id"`
	}
	mux.HandleFunc("POST "+base+"/{session_id}/rehearsal/stop", studioRoute(http.MethodPost, false, studioBodyRoute(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in stop) (any, error) {
		out, err := planner.RequestStop(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), live.MediaStopInput{SessionID: r.PathValue("session_id"), AttemptID: in.AttemptID})
		if err != nil {
			return nil, err
		}
		return studioReceipt{SessionID: out.SessionID, AttemptID: out.AttemptID, State: out.State}, nil
	})))
}

type studioReceipt struct {
	SessionID string `json:"session_id"`
	AttemptID string `json:"attempt_id"`
	State     string `json:"state"`
}

func studioRoute(method string, query bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if r.Method != method {
			respondError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if !query && (r.URL.RawQuery != "" || r.URL.ForceQuery) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		if method == http.MethodGet && (r.Body != nil && r.Body != http.NoBody || r.ContentLength != 0 || len(r.TransferEncoding) != 0 || r.Header.Get("Idempotency-Key") != "") {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	}
}

func studioPage(u *url.URL) (pagination.Request, error) {
	var out pagination.Request
	if u.ForceQuery || len(u.RawQuery) > 4096 {
		return out, command.ErrInvalid
	}
	if u.RawQuery != "" {
		for _, field := range strings.Split(u.RawQuery, "&") {
			if field == "" || !strings.Contains(field, "=") {
				return out, command.ErrInvalid
			}
		}
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return out, command.ErrInvalid
	}
	for name, list := range values {
		if len(list) != 1 || list[0] == "" {
			return out, command.ErrInvalid
		}
		switch name {
		case "limit":
			n, err := strconv.Atoi(list[0])
			if err != nil || n < 1 || n > 100 || strconv.Itoa(n) != list[0] {
				return out, command.ErrInvalid
			}
			out.Limit = n
		case "cursor":
			if len(list[0]) > 1024 {
				return out, command.ErrInvalid
			}
			out.Cursor = list[0]
		default:
			return out, command.ErrInvalid
		}
	}
	return out, nil
}

// The generic bodyRoute does not reject duplicate JSON keys; Studio does.
func studioBodyRoute[T any](pool *pgxpool.Pool, permission string, fn func(context.Context, pgx.Tx, platform.Scope, *http.Request, T) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			respondError(w, http.StatusUnsupportedMediaType, "json_required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		defer r.Body.Close()
		raw, err := io.ReadAll(r.Body)
		if err != nil || !utf8.Valid(raw) || !studioUniqueJSON(raw) {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		var in T
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		var extra any
		if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		scoped(pool, permission, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return fn(ctx, tx, s, r, in)
		})(w, r)
	}
}

func studioUniqueJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	if !studioValue(d, 0) {
		return false
	}
	_, err := d.Token()
	return errors.Is(err, io.EOF)
}

func studioValue(d *json.Decoder, depth int) bool {
	if depth > 16 {
		return false
	}
	token, err := d.Token()
	if err != nil {
		return false
	}
	switch token {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return false
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return false
			}
			seen[key] = true
			if !studioValue(d, depth+1) {
				return false
			}
		}
		end, err := d.Token()
		return err == nil && end == json.Delim('}')
	case json.Delim('['):
		for d.More() {
			if !studioValue(d, depth+1) {
				return false
			}
		}
		end, err := d.Token()
		return err == nil && end == json.Delim(']')
	case json.Delim('}'), json.Delim(']'):
		return false
	default:
		return true
	}
}
