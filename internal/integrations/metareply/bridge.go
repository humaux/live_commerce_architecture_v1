// Purpose: live-console-v1 §2.3 — the internal API↔claims-worker bridge. The worker side is the
// Console HTTP handler (POST /internal/v1/comment-page and /comment-facts) served only on the backend
// Docker network with a shared 32-byte bearer token; the API side is BridgeClient. Comment text and
// names exist only inside these responses and are never logged or persisted (the bodies are never
// logged, and every error is a fixed safe code).
// Depends on: comment_poll.go (Console), live.console_source (0123), metaoauth.Graph, core.Secret.
// Used by: cmd/claims-worker (Console.Handler), internal/live/stream.go (BridgeClient).
package metareply

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"livecommerce/internal/command"
)

// Bridge cursor scope vocabulary and the fixed safe error codes of the bridge (§2.3).
var (
	errBridgeAuth        = errors.New("metareply: bridge auth failed")
	ErrBridgeNotFound    = errors.New("metareply: bridge source not found") // 404 → API no_source
	ErrBridgeNotOwner    = errors.New("metareply: bridge not poller owner") // 421 → API stream_unavailable
	ErrBridgeInvalidCur  = errors.New("metareply: bridge invalid cursor")   // 400 → API invalid_cursor
	ErrBridgeUnavailable = errors.New("metareply: bridge unavailable")      // 503/transport → API stream_unavailable

	// commentRefShape is the plain Meta comment id shape (numeric), matching the 0123 CHECK
	// `comment_ref ~ '^[0-9_]{1,80}$'` on live.comment_prints and live.console_marks.
	commentRefShape = regexp.MustCompile(`^[0-9_]{1,80}$`)
	// Go regexp caps repeat counts at 1000; the 1024 bound is enforced in openCursor by len().
	validCursorB64 = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,1000}$`)
	validGraphCur  = regexp.MustCompile(`^[A-Za-z0-9_=+./-]{1,512}$`)
)

// Cursor is one buffer position {epoch, seq}; the bridge owns both values (the API only echoes them).
type Cursor struct {
	Epoch int64 `json:"epoch"`
	Seq   int64 `json:"seq"`
}

// BridgeComment is one buffered comment (§2.6 minus marks). Text/author_name exist only in memory
// and in this response; from.id is never returned (is_page is the collapsed fact).
type BridgeComment struct {
	Ref           string    `json:"ref"`
	ParentRef     *string   `json:"parent_ref"`
	CreatedAt     time.Time `json:"created_at"`
	AuthorName    *string   `json:"author_name"`
	Text          string    `json:"text"`
	IsPage        bool      `json:"is_page"`
	HasAttachment bool      `json:"has_attachment"`
}

// BridgeStreamState is §2.4 StreamState; Reason carries the fixed code for
// throttled/reauth_required/unavailable (e.g. poller_cap) and is omitted when live.
type BridgeStreamState struct {
	State           string     `json:"state"`
	PollIntervalMs  int        `json:"poll_interval_ms"`
	LastOKAt        *time.Time `json:"last_ok_at"`
	LagMs           *int64     `json:"lag_ms"`
	SourcePlatform  string     `json:"source_platform"`
	VideoEmbeddable bool       `json:"video_embeddable"`
	Reason          string     `json:"reason,omitempty"`
}

// BridgePageRequest is POST /internal/v1/comment-page's exact body (§2.3).
type BridgePageRequest struct {
	TenantID     string  `json:"tenant_id"`
	StoreID      string  `json:"store_id"`
	SessionID    string  `json:"session_id"`
	SourceID     string  `json:"source_id"`
	After        *Cursor `json:"after"`
	BeforeCursor *string `json:"before_cursor"`
	Limit        int     `json:"limit"`
}

// BridgePage is the comment-page response (§2.3): the buffer epoch, the items (newest first),
// next_seq (the seq after the last returned item, same epoch) and an HMAC-signed older_cursor.
type BridgePage struct {
	Epoch       int64             `json:"epoch"`
	Items       []BridgeComment   `json:"items"`
	NextSeq     int64             `json:"next_seq"`
	OlderCursor *string           `json:"older_cursor"`
	Stream      BridgeStreamState `json:"stream"`
}

// BridgeFactsRequest is POST /internal/v1/comment-facts' exact body (§2.3).
type BridgeFactsRequest struct {
	TenantID   string `json:"tenant_id"`
	StoreID    string `json:"store_id"`
	SessionID  string `json:"session_id"`
	SourceID   string `json:"source_id"`
	CommentRef string `json:"comment_ref"`
}

// CommentFacts are the facts the manual private-reply rules need (§3.3); from.id itself never leaves the worker.
type CommentFacts struct {
	Found     bool       `json:"found"`
	CreatedAt *time.Time `json:"created_at"`
	IsPage    bool       `json:"is_page"`
	IsReply   bool       `json:"is_reply"`
}

// BridgeClient is the API-side bridge caller: a fixed bearer token, a backend-network base URL, no
// redirects, fixed safe errors. Bodies are never logged; a 421 from a non-holder replica is
// ErrBridgeNotOwner (the API maps it to stream_unavailable without retrying across replicas).
type BridgeClient struct {
	baseURL string
	token   []byte
	hc      *http.Client
}

// NewBridgeClient validates the base URL (http://host[:port], no path) and the 32-byte token.
func NewBridgeClient(baseURL string, token []byte) (*BridgeClient, error) {
	if token == nil || len(token) != 32 {
		return nil, ErrConfig
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrConfig
	}
	hc := &http.Client{}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &BridgeClient{baseURL: strings.TrimRight(baseURL, "/"), token: append([]byte(nil), token...), hc: hc}, nil
}

func (c *BridgeClient) do(ctx context.Context, path string, body any, out any) error {
	if c == nil {
		return ErrBridgeUnavailable
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return ErrBridgeUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return ErrBridgeUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// The bearer is 32 arbitrary bytes; the header carries its std-base64 form (the header value must
	// stay ASCII — raw bytes would trip Go's header validation, so this is the wire encoding).
	req.Header.Set("Authorization", "Bearer "+base64.StdEncoding.EncodeToString(c.token))
	resp, err := c.hc.Do(req)
	if err != nil {
		return ErrBridgeUnavailable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ErrBridgeUnavailable
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrBridgeNotFound
	case http.StatusMisdirectedRequest: // 421: this process is not the poll-lease holder
		return ErrBridgeNotOwner
	case http.StatusBadRequest:
		return ErrBridgeInvalidCur
	case http.StatusUnauthorized:
		return errBridgeAuth
	default:
		return ErrBridgeUnavailable
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return ErrBridgeUnavailable
	}
	return nil
}

// CommentPage reads one comment page from the bridge.
func (c *BridgeClient) CommentPage(ctx context.Context, req BridgePageRequest) (BridgePage, error) {
	var out BridgePage
	if !command.ValidID(req.TenantID) || !command.ValidID(req.StoreID) || !command.ValidID(req.SessionID) ||
		!command.ValidID(req.SourceID) || req.Limit < 1 || req.Limit > 100 {
		return out, ErrBridgeInvalidCur
	}
	return out, c.do(ctx, "/internal/v1/comment-page", req, &out)
}

// CommentFacts resolves one comment's facts through the bridge (ring buffer, else one Graph read).
func (c *BridgeClient) CommentFacts(ctx context.Context, req BridgeFactsRequest) (CommentFacts, error) {
	var out CommentFacts
	if !command.ValidID(req.TenantID) || !command.ValidID(req.StoreID) || !command.ValidID(req.SessionID) ||
		!command.ValidID(req.SourceID) || !commentRefShape.MatchString(req.CommentRef) {
		return out, ErrBridgeInvalidCur
	}
	return out, c.do(ctx, "/internal/v1/comment-facts", req, &out)
}

// sealCursor builds older_cursor = base64url(JSON{tenant,store,session,source,graph_cursor,exp}‖HMAC) (§2.4).
func (c *Console) sealCursor(tenant, store, session, source, graphCursor string, exp time.Time) string {
	raw, _ := json.Marshal([]string{tenant, store, session, source, graphCursor, exp.UTC().Format(time.RFC3339Nano)})
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, c.cursorKey)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// openCursor verifies and decodes older_cursor; any scope/format/tamper/expiry problem is ErrBridgeInvalidCur.
func (c *Console) openCursor(cursor, tenant, store, session, source string, now time.Time) (graphCursor string, err error) {
	if len(cursor) > 1024 || !validCursorB64.MatchString(cursor) {
		return "", ErrBridgeInvalidCur
	}
	payload, sig, ok := strings.Cut(cursor, ".")
	if !ok {
		return "", ErrBridgeInvalidCur
	}
	mac := hmac.New(sha256.New, c.cursorKey)
	mac.Write([]byte(payload))
	want := mac.Sum(nil)
	got, decErr := base64.RawURLEncoding.DecodeString(sig)
	if decErr != nil || !hmac.Equal(got, want) {
		return "", ErrBridgeInvalidCur
	}
	raw, decErr := base64.RawURLEncoding.DecodeString(payload)
	if decErr != nil {
		return "", ErrBridgeInvalidCur
	}
	var parts []string
	if json.Unmarshal(raw, &parts) != nil || len(parts) != 6 ||
		parts[0] != tenant || parts[1] != store || parts[2] != session || parts[3] != source {
		return "", ErrBridgeInvalidCur
	}
	exp, err := time.Parse(time.RFC3339Nano, parts[5])
	if err != nil || !now.Before(exp) || !validGraphCur.MatchString(parts[4]) {
		return "", ErrBridgeInvalidCur
	}
	return parts[4], nil
}

// constantTokenEqual is the §2.3 constant-time bearer compare.
func constantTokenEqual(a, b []byte) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}
