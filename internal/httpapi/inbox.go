// Purpose: the merchant inbox read/write HTTP adapter (contracts/live-console-v1.md §3.2/§3.6/§3.7, §11
// A8-A11/A13/A14) under /v1/admin/stores/{store_id}/inbox: the conversation list, the decrypted thread, the
// read/takeover/release/customer-link writes and the buyer panel. It decides no rule (internal/inbox and the
// SECURITY DEFINER functions do), never returns a driver message, never logs a body, and keeps the private
// no-store response boundary. Plaintext message bodies exist only in the A9 response.
// Depends on: livecommerce/internal/inbox, livecommerce/internal/platform (WithScope), livecommerce/internal/command
// (ValidID), and the SECURITY DEFINER functions of migration 0119.
// Used by: internal/httpapi/handler.go (registerInboxRoutes, gated on Options.Inbox); cmd/api builds the service.

package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

// inboxQueryError is a query parse failure with a specific status/code (A8's invalid_filter is 400, not 422).
type inboxQueryError struct {
	status int
	code   string
}

func (e *inboxQueryError) Error() string { return e.code }

// registerInboxRoutes mounts A8–A11/A13/A14 when svc is non-nil; nil leaves them unmounted (cmd/api builds the
// service only when the payload keyring is configured, exactly like every other nil-able service in Options).
func registerInboxRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *inbox.Service) {
	if svc == nil {
		return
	}
	const base = "/v1/admin/stores/{store_id}/inbox/conversations"
	const panelBase = "/v1/admin/stores/{store_id}/inbox/buyer-panel"

	// A8 conversation list. session_id is accepted (valid uuid) but matches nothing in 0119: comment read-through
	// (LC-B2) has not projected any comment conversations yet, so the live_comment filter returns empty.
	mux.HandleFunc("GET "+base, inboxRoute(http.MethodGet, false, true, func(w http.ResponseWriter, r *http.Request) {
		req, err := parseInboxListRequest(r.URL)
		if err != nil {
			if qe, ok := err.(*inboxQueryError); ok {
				respondError(w, qe.status, qe.code)
				return
			}
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		inboxScoped(pool, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			out, err := svc.ListConversations(ctx, tx, req)
			if err != nil {
				return nil, err
			}
			// Bundle-only items (Amendment 1 P2-2) follow the conversations and never carry the keyset cursor.
			conversations := 0
			for _, it := range out.Items {
				if !it.BundleOnly {
					conversations++
				}
			}
			if conversations == req.Limit && conversations > 0 {
				last := out.Items[conversations-1]
				out.NextCursor = encodeInboxCursor(last.LastAt, last.ConversationID)
			}
			return out, nil
		})(w, r)
	}))

	// A9 thread: the header + the opened/decrypted inbound messages (plaintext only here; no-store).
	mux.HandleFunc("GET "+base+"/{conversation_id}/messages", inboxRoute(http.MethodGet, false, true, func(w http.ResponseWriter, r *http.Request) {
		beforeSeq, limit, err := parseInboxThreadQuery(r.URL)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		inboxScoped(pool, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.ReadThread(ctx, tx, r.PathValue("conversation_id"), beforeSeq, limit)
		})(w, r)
	}))

	// A10 read. Keyed; effective only for inbox:reply holders (the definer answers with the unchanged value otherwise).
	mux.HandleFunc("POST "+base+"/{conversation_id}/read", inboxRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[inbox.ReadInput](w, r, []string{"read_seq"}, nil)
		if !ok {
			return
		}
		inboxScoped(pool, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.MarkRead(ctx, tx, r.PathValue("conversation_id"), in)
		})(w, r)
	}))

	// A11 takeover / release (CAS on expected_generation).
	mux.HandleFunc("POST "+base+"/{conversation_id}/takeover", inboxRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[inbox.TakeoverInput](w, r, []string{"expected_generation"}, nil)
		if !ok {
			return
		}
		inboxScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.Takeover(ctx, tx, r.PathValue("conversation_id"), in)
		})(w, r)
	}))
	mux.HandleFunc("POST "+base+"/{conversation_id}/release", inboxRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[inbox.TakeoverInput](w, r, []string{"expected_generation"}, nil)
		if !ok {
			return
		}
		inboxScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.Release(ctx, tx, r.PathValue("conversation_id"), in)
		})(w, r)
	}))

	// A14 customer link (customer_id string or null; CAS on expected_version). The null-vs-absent distinction is a
	// transport rule, so this one uses its own decoder instead of claimsBody (which rejects top-level null).
	mux.HandleFunc("POST "+base+"/{conversation_id}/customer-link", inboxRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := inboxCustomerLinkBody(w, r)
		if !ok {
			return
		}
		inboxScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.CustomerLink(ctx, tx, r.PathValue("conversation_id"), in)
		})(w, r)
	}))

	// A13 buyer panel: conversation-scoped fields, or the bundle-scoped panel (platform + link_pending_manual, LC-B4).
	mux.HandleFunc("GET "+panelBase, inboxRoute(http.MethodGet, false, true, func(w http.ResponseWriter, r *http.Request) {
		conversationID, bundleID, err := parseInboxBuyerPanel(r.URL)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		inboxScoped(pool, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			if bundleID != "" {
				return svc.BuyerPanelByBundle(ctx, tx, bundleID)
			}
			return svc.BuyerPanel(ctx, tx, conversationID)
		})(w, r)
	}))

	// Methodless fallbacks keep 405 inside the same private no-store boundary.
	for _, path := range []string{base, base + "/{conversation_id}/messages", base + "/{conversation_id}/read",
		base + "/{conversation_id}/takeover", base + "/{conversation_id}/release", base + "/{conversation_id}/customer-link",
		panelBase} {
		mux.HandleFunc(path, studioRoute("", false, nil))
	}
}

// inboxRoute is studioRoute (private no-store, method, query and GET-body rules) plus the inbox transport rules:
// Idempotency-Key exactly once (receipt grammar) on the keyed writes and forbidden elsewhere, and a canonical
// conversation_id path id (422 before any transaction).
func inboxRoute(method string, keyed, query bool, next http.HandlerFunc) http.HandlerFunc {
	return studioRoute(method, query, func(w http.ResponseWriter, r *http.Request) {
		keys := r.Header.Values("Idempotency-Key")
		if (keyed && (len(keys) != 1 || !claimsKey.MatchString(keys[0]))) || (!keyed && len(keys) != 0) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		if value := r.PathValue("conversation_id"); value != "" && !command.ValidID(value) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	})
}

func inboxScoped(pool *pgxpool.Pool, permission string, fn action) http.HandlerFunc {
	return scopedAs(pool, permission, inboxClassify, fn)
}

// inboxClassify maps the fixed definer codes to their frozen transport codes and falls back to the shared table.
// PT409 distinguishes takeover_changed (A11) from version_conflict (A14) by the definer's raise message.
func inboxClassify(err error) (int, string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return http.StatusBadRequest, "invalid_filter"
		case "PT403":
			return http.StatusForbidden, "forbidden"
		case "PT404":
			return http.StatusNotFound, "not_found"
		case "PT409":
			if pg.Message == "version_conflict" {
				return http.StatusConflict, "version_conflict"
			}
			return http.StatusConflict, "takeover_changed"
		case "PT422":
			return http.StatusUnprocessableEntity, "invalid_request"
		}
	}
	if inbox.IsNotFound(err) {
		return http.StatusNotFound, "not_found"
	}
	return classify(err)
}

// parseInboxListRequest parses A8's filter/session_id/cursor/limit. Defaults filter=all and limit=50. An unknown
// filter is a 400 invalid_filter (the one query error that is not a 422).
func parseInboxListRequest(u *url.URL) (inbox.ListRequest, error) {
	req := inbox.ListRequest{Filter: "all", Limit: 50}
	if u.ForceQuery || len(u.RawQuery) > 4096 {
		return req, command.ErrInvalid
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return req, command.ErrInvalid
	}
	for key, list := range values {
		if len(list) != 1 || list[0] == "" {
			return req, command.ErrInvalid
		}
		switch key {
		case "filter":
			req.Filter = list[0]
		case "session_id":
			if !command.ValidID(list[0]) {
				return req, command.ErrInvalid
			}
			// LC-B2 boundary: no comment conversations exist in 0119, so a session scope matches nothing.
		case "cursor":
			if len(list[0]) > 1024 {
				return req, command.ErrInvalid
			}
			at, id, err := decodeInboxCursor(list[0])
			if err != nil {
				return req, command.ErrInvalid
			}
			req.LastAt = &at
			req.CursorID = &id
		case "limit":
			n, err := strconv.Atoi(list[0])
			if err != nil || n < 1 || n > 50 || strconv.Itoa(n) != list[0] {
				return req, command.ErrInvalid
			}
			req.Limit = n
		default:
			return req, command.ErrInvalid
		}
	}
	switch req.Filter {
	case "all", "unreplied", "messenger", "instagram", "live_comment":
	default:
		return req, &inboxQueryError{http.StatusBadRequest, "invalid_filter"}
	}
	return req, nil
}

// parseInboxThreadQuery parses A9's before_seq (positive) and limit (1..50, default 50).
func parseInboxThreadQuery(u *url.URL) (*int64, int, error) {
	var beforeSeq *int64
	limit := 50
	if u.ForceQuery || len(u.RawQuery) > 4096 {
		return nil, 0, command.ErrInvalid
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, 0, command.ErrInvalid
	}
	for key, list := range values {
		if len(list) != 1 || list[0] == "" {
			return nil, 0, command.ErrInvalid
		}
		switch key {
		case "before_seq":
			n, perr := strconv.ParseInt(list[0], 10, 64)
			if perr != nil || n < 1 || strconv.FormatInt(n, 10) != list[0] {
				return nil, 0, command.ErrInvalid
			}
			beforeSeq = &n
		case "limit":
			n, perr := strconv.Atoi(list[0])
			if perr != nil || n < 1 || n > 50 || strconv.Itoa(n) != list[0] {
				return nil, 0, command.ErrInvalid
			}
			limit = n
		default:
			return nil, 0, command.ErrInvalid
		}
	}
	return beforeSeq, limit, nil
}

// parseInboxBuyerPanel parses A13's exactly-one-of conversation_id | bundle_id (both canonical uuids). Query shape
// errors map to 400 per the contract.
func parseInboxBuyerPanel(u *url.URL) (conversationID, bundleID string, err error) {
	if u.ForceQuery || len(u.RawQuery) > 4096 {
		return "", "", command.ErrInvalid
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) == 0 {
		return "", "", command.ErrInvalid
	}
	for key, list := range values {
		if len(list) != 1 || list[0] == "" {
			return "", "", command.ErrInvalid
		}
		switch key {
		case "conversation_id":
			if conversationID != "" || bundleID != "" || !command.ValidID(list[0]) {
				return "", "", command.ErrInvalid
			}
			conversationID = list[0]
		case "bundle_id":
			if conversationID != "" || bundleID != "" || !command.ValidID(list[0]) {
				return "", "", command.ErrInvalid
			}
			bundleID = list[0]
		default:
			return "", "", command.ErrInvalid
		}
	}
	if conversationID == "" && bundleID == "" {
		return "", "", command.ErrInvalid
	}
	return conversationID, bundleID, nil
}

// encodeInboxCursor is the opaque A8 keyset cursor: base64url of "<RFC3339Nano last_at>|<conversation_id>".
func encodeInboxCursor(lastAt time.Time, conversationID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(lastAt.UTC().Format(time.RFC3339Nano) + "|" + conversationID))
}

func decodeInboxCursor(c string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", command.ErrInvalid
	}
	at, id, ok := strings.Cut(string(raw), "|")
	if !ok || !command.ValidID(id) {
		return time.Time{}, "", command.ErrInvalid
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return time.Time{}, "", command.ErrInvalid
	}
	return t, id, nil
}

// inboxCustomerLinkBody is A14's strict decoder, which (unlike claimsBody) accepts customer_id: null as the unlink
// and requires it present either way.
func inboxCustomerLinkBody(w http.ResponseWriter, r *http.Request) (inbox.CustomerLinkInput, bool) {
	var in inbox.CustomerLinkInput
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		respondError(w, http.StatusUnsupportedMediaType, "json_required")
		return in, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(raw) || !studioUniqueJSON(raw, []string{"customer_id", "expected_version"}) {
		respondError(w, http.StatusBadRequest, "invalid_json")
		return in, false
	}
	var body struct {
		CustomerID      json.RawMessage `json:"customer_id"`
		ExpectedVersion json.RawMessage `json:"expected_version"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json")
		return in, false
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, "invalid_json")
		return in, false
	}
	if len(body.CustomerID) == 0 || len(body.ExpectedVersion) == 0 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return in, false
	}
	var version int64
	if err := json.Unmarshal(body.ExpectedVersion, &version); err != nil || version < 0 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return in, false
	}
	in.ExpectedVersion = version
	if bytes.Equal(bytes.TrimSpace(body.CustomerID), []byte("null")) {
		return in, true
	}
	var id string
	if err := json.Unmarshal(body.CustomerID, &id); err != nil || !command.ValidID(id) {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return in, false
	}
	in.CustomerID = &id
	return in, true
}
