// Purpose: the claims-worker dispatcher routes of the Meta private reply: the automatic claim-link reply (meta-claims-intake-v1 §6: Check =
// claims.check_meta_reply, link token re-derived in memory, one POST that is never repeated, query-only Reconcile) and, on the SAME route, the
// manual private reply of live-console-v1 §4.3 (message_type manual_private_reply), which the send adapter (send_dm.go) serves.
// Depends on: internal/integrations/core (DispatchRoute), internal/claims (ReplyLinkKey), pagetoken/pageopen (Page-token custody), SQL
// claims.check_meta_reply, integration.load_meta_page_token, integration.meta_connect_mark_reauth, inbox.* send definers (send_dm.go); Graph
// POST /{asset}/messages (MOCK against a loopback fake; LIVE only at the probe).
// Used by: cmd/claims-worker (RoutesV2), internal/integrations/metareply tests, tests/foundation (MCI07, LCN06-LCN13).
// Invariants: one private reply per comment (mpr: key, live-console-v1 §4.2); deny codes human_takeover / takeover_changed (§3.6).
// Status: MOCK.

package metareply

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/platform"
)

// Graph constants (meta-claims-intake-v1 §0 F1/F2, retrieved 2026-09-28):
//   - Facebook Page private reply: POST /{PAGE-ID}/messages, recipient.comment_id, needs
//     pages_messaging: https://developers.facebook.com/docs/messenger-platform/discovery/private-replies/
//   - Instagram private reply: POST /<IG_ID>/messages on graph.facebook.com (Facebook Login path),
//     needs instagram_manage_comments + pages_read_engagement:
//     https://developers.facebook.com/docs/instagram-platform/private-replies/
//
// The API version is required configuration (U5, no default) and the token travels in the JSON
// body unless AuthorizationHeader is set (U6 default false), never in the URL.
const (
	graphHost       = "https://graph.facebook.com"
	maxResponseBody = 64 << 10
	templateID      = "claim-link/v1"
	messageType     = "first_private_reply"

	codeSent        = "graph_sent"
	codeUnconfirmed = "graph_unconfirmed"
	codeUnproven    = "reconcile_unproven"
)

var (
	versionPattern  = regexp.MustCompile(`^v[0-9]{1,3}\.[0-9]{1,2}$`)
	loopbackPattern = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]{1,5}$`)

	// checkDenyCodes are the §6.3 fixed deny codes of claims.check_meta_reply.
	checkDenyCodes = map[string]bool{"deadline": true, "source_off": true, "principal_revoked": true, "link_invalid": true, "live_closed": true,
		"human_takeover": true, "takeover_changed": true} // the last two: live-console-v1 §3.6 / meta-claims-intake-v1 §14.1 clause 3

	// requiredScopes are the attested Page-token scopes per provider (§7; U2 open for Instagram).
	requiredScopes = map[string][]string{
		"facebook":  {"pages_messaging"},
		"instagram": {"instagram_manage_comments", "pages_read_engagement"},
	}
)

// Config is the Graph adapter configuration. GraphBaseURL is exactly https://graph.facebook.com or
// a loopback http://127.0.0.1:<port> (MOCK); GraphVersion is required (U5). HTTPClient is optional;
// redirects are never followed (a 307 would replay the POST body with the token).
type Config struct {
	GraphBaseURL        string
	GraphVersion        string
	AuthorizationHeader bool
	HTTPClient          *http.Client
}

// Validate is the configuration part of Routes' checks (no I/O), so a command can refuse a bad
// base URL or API version before opening any connection.
func (c Config) Validate() error {
	if !versionPattern.MatchString(c.GraphVersion) || !(c.GraphBaseURL == graphHost || loopbackPattern.MatchString(c.GraphBaseURL)) {
		return ErrConfig
	}
	return nil
}

// GraphHost is the only non-loopback base URL Config accepts.
const GraphHost = graphHost

// checkFunc runs claims.check_meta_reply; separated so unit tests need no database.
type checkFunc func(ctx context.Context, operationID string, linkHash []byte) (string, error)

// Routes returns the two dispatcher routes (facebook and instagram, meta.private_reply, service).
// checkPool must be the commerce_claims_worker pool (platform.ValidateWorkerPool) and is used for one
// STABLE statement per Check; no transaction is held across I/O.
func Routes(checkPool *pgxpool.Pool, linkKey claims.ReplyLinkKey, pageKeys *PageTokenKeyring, cfg Config) ([]core.DispatchRoute, error) {
	return RoutesV2(checkPool, linkKey, pageKeys, nil, cfg)
}

// RoutesV2 is Routes plus the HPKE private ring that opens meta-page-token-v2 credentials (sealed by the merchant connect in cmd/api,
// which holds only public keys). A stored credential is v1 (AES keyring, nonce 12 bytes) or v2 (HPKE, nonce = 32-byte encapsulated
// key); with v2 == nil a v2 row is a pre-dispatch denial, never an open attempt.
func RoutesV2(checkPool *pgxpool.Pool, linkKey claims.ReplyLinkKey, pageKeys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) ([]core.DispatchRoute, error) {
	if checkPool == nil {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), checkPool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	check := func(ctx context.Context, operationID string, linkHash []byte) (code string, err error) {
		// claims.check_meta_reply: definer commerce_claims_writer, lock-free read of the frozen operation.
		err = checkPool.QueryRow(ctx, `SELECT claims.check_meta_reply($1::uuid,$2::bytea)`, operationID, linkHash).Scan(&code)
		return code, err
	}
	// integration.meta_connect_mark_reauth (0095, commerce_claims_worker): a Graph 190 flips the merchant's connect card to "reconnect".
	reauth := func(ctx context.Context, operationID string) {
		_, _ = checkPool.Exec(ctx, `SELECT integration.meta_connect_mark_reauth($1::uuid)`, operationID)
	}
	// The manual private reply (message_type manual_private_reply, live-console-v1 §4) shares this route: its Check / loader / dispatch /
	// Finish come from the send adapter (send_dm.go, manual_reply.go).
	send, err := newSendAdapterFor(checkPool, pageKeys, v2, cfg)
	if err != nil {
		return nil, err
	}
	return newRoutesWithSend(check, reauth, linkKey, pageKeys, v2, cfg, send)
}

func newRoutes(check checkFunc, linkKey claims.ReplyLinkKey, pageKeys *PageTokenKeyring, cfg Config) ([]core.DispatchRoute, error) {
	return newRoutesWith(check, nil, linkKey, pageKeys, nil, cfg)
}

func newRoutesWith(check checkFunc, reauth func(context.Context, string), linkKey claims.ReplyLinkKey, pageKeys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) ([]core.DispatchRoute, error) {
	return newRoutesWithSend(check, reauth, linkKey, pageKeys, v2, cfg, nil)
}

// newRoutesWithSend is newRoutesWith plus the optional send adapter that serves manual private replies on the same route.
func newRoutesWithSend(check checkFunc, reauth func(context.Context, string), linkKey claims.ReplyLinkKey, pageKeys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config, send *sendAdapter) ([]core.DispatchRoute, error) {
	if check == nil || linkKey.ID() == "" || pageKeys == nil || cfg.Validate() != nil {
		return nil, ErrConfig
	}
	client := &http.Client{}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		client = &copied
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	a := &adapter{check: check, reauth: reauth, linkKey: linkKey, keys: pageKeys, v2: v2, cfg: cfg, client: client, send: send}
	routes := make([]core.DispatchRoute, 0, 2)
	for _, provider := range []string{"facebook", "instagram"} {
		route := core.DispatchRoute{
			Provider: provider, Action: "meta.private_reply", Purpose: "service",
			Check:              a.checkRoute,
			LoadSecret:         a.loadSecretFor(provider),
			DispatchWithSecret: a.dispatch,
			Reconcile:          a.reconcile,
		}
		if send != nil {
			// A manual private reply carries a sealed dispatch copy beside the token (the automatic claim-link reply has none and keeps the
			// raw token); Finish wipes the copy and records the bundle↔peer link (live-console-v1 §3.4/§3.7/§4.3).
			route.LoadSecret = send.withDispatchCopy(route.LoadSecret)
			route.Finish = send.finish
		}
		routes = append(routes, route)
	}
	return routes, nil
}

type adapter struct {
	check   checkFunc
	reauth  func(ctx context.Context, operationID string) // nil in unit tests; flips the connect card on a Graph 190
	linkKey claims.ReplyLinkKey
	keys    *PageTokenKeyring
	v2      *pageopen.Keyring // nil: v2 (HPKE) credentials are denied
	cfg     Config
	client  *http.Client
	send    *sendAdapter // nil: manual private replies are refused (unit tests of the automatic reply only)
}

// replyRequest is the part of the frozen operation request (§6.2, no token/text/name) the adapter reads.
type replyRequest struct {
	V           int    `json:"v"`
	AssetID     string `json:"asset_id"`
	CommentRef  string `json:"comment_ref"`
	BundleID    string `json:"bundle_id"`
	LinkKeyID   string `json:"link_key_id"`
	Locale      string `json:"locale"`
	Template    string `json:"template"`
	MessageType string `json:"message_type"`
	Origin      string `json:"origin"`
}

var errBadRequest = errors.New("metareply: malformed operation request")

func parseRequest(req core.DispatchRequest) (r replyRequest, err error) {
	if err = json.Unmarshal(req.Request, &r); err != nil || r.V != 1 || r.Template != templateID || r.MessageType != messageType ||
		!command.ValidID(r.BundleID) || r.AssetID != req.ExternalAssetID {
		return replyRequest{}, errBadRequest
	}
	return r, nil
}

// linkToken re-derives the bundle's claim token; it exists only in memory for one call.
func (a *adapter) linkToken(req core.DispatchRequest, r replyRequest) (claims.LinkToken, error) {
	return claims.SystemLinkToken(a.linkKey, req.TenantID, req.StoreID, r.BundleID, req.OperationID)
}

// checkRoute is Check (every attempt): only a returned deny code is a policy denial (zero HTTP
// calls, BLOCKED_POLICY); any infrastructure error is a plain error, which the dispatcher records
// as UNKNOWN policy_check_failed (the reply is not sent; documented limit, §14).
func (a *adapter) checkRoute(ctx context.Context, req core.DispatchRequest) error {
	if isManualReply(req.Request) {
		if a.send == nil {
			return core.DenyPolicy(codeInvalidRequest)
		}
		return a.send.checkRoute(ctx, req)
	}
	r, err := parseRequest(req)
	if err != nil {
		return err
	}
	if r.LinkKeyID != a.linkKey.ID() {
		return fmt.Errorf("link_key_changed: %w", core.ErrPolicyDenied)
	}
	token, err := a.linkToken(req, r)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(token))
	code, err := a.check(ctx, req.OperationID, hash[:])
	if err != nil {
		return errors.New("metareply: check unavailable")
	}
	if code == "OK" {
		return nil
	}
	if checkDenyCodes[code] {
		if code == "human_takeover" || code == "takeover_changed" {
			return core.DenyPolicy(code) // recorded as the operation's result code (live-console-v1 §3.6, LCN10)
		}
		return fmt.Errorf("%s: %w", code, core.ErrPolicyDenied)
	}
	return errors.New("metareply: unexpected check result")
}

// loadSecretFor returns the LoadSecret hook of one provider route: its only body is one call to the
// lease-fenced loader integration.load_meta_page_token (definer commerce_integration_writer),
// inside the dispatcher's transaction. Zero rows or a missing attested scope is a pre-dispatch
// denial (capability evidence, arch §10.2/I07).
func (a *adapter) loadSecretFor(provider string) func(context.Context, pgx.Tx, core.SecretClaim) (core.Secret, error) {
	return pageSecretLoader(a.keys, a.v2, provider, requiredScopes[provider], `SELECT tenant_id::text,store_id::text,binding_id::text,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested
			FROM integration.load_meta_page_token($1::uuid,$2::bigint,$3::bytea)`)
}

// pageSecretLoader shares custody/opening, while each route supplies its own
// constant lease-fenced SQL loader and exact capability scopes.
func pageSecretLoader(keys *PageTokenKeyring, v2 *pageopen.Keyring, provider string, scopes []string, query string) func(context.Context, pgx.Tx, core.SecretClaim) (core.Secret, error) {
	return func(ctx context.Context, tx pgx.Tx, claim core.SecretClaim) (core.Secret, error) {
		var row struct {
			tenant, store, binding, provider, asset, keyID string
			version                                        int64
			nonce, ciphertext                              []byte
			scopes                                         []string
		}
		err := tx.QueryRow(ctx, query,
			claim.OperationID, claim.Generation, claim.LeaseToken).
			Scan(&row.tenant, &row.store, &row.binding, &row.provider, &row.asset, &row.version, &row.keyID, &row.nonce, &row.ciphertext, &row.scopes)
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Secret{}, fmt.Errorf("no page token: %w", core.ErrPolicyDenied)
		}
		if err != nil {
			return core.Secret{}, errors.New("metareply: credential load failed")
		}
		if row.provider != provider || !hasScopes(row.scopes, scopes) {
			return core.Secret{}, fmt.Errorf("page token lacks attested scope: %w", core.ErrPolicyDenied)
		}
		secret, err := openPageToken(keys, v2, PageTokenScope{TenantID: row.tenant, StoreID: row.store, BindingID: row.binding,
			Provider: row.provider, AssetID: row.asset, Version: row.version}, row.keyID, row.nonce, row.ciphertext)
		if errors.Is(err, errNoPrivateRing) {
			return core.Secret{}, fmt.Errorf("page token v2 without private ring: %w", core.ErrPolicyDenied)
		}
		return secret, err
	}
}

// openPageToken opens one stored credential: v2 (nonce = the 32-byte HPKE encapsulated key, sealed by the merchant connect to the public
// ring) with the private ring, else v1 (AES keyring, 12-byte nonce). Shared by the dispatcher's LoadSecret and the Unsubscriber.
func openPageToken(keys *PageTokenKeyring, v2 *pageopen.Keyring, s PageTokenScope, keyID string, nonce, ciphertext []byte) (core.Secret, error) {
	if len(nonce) != pagetoken.EncSize {
		return keys.Open(s, keyID, nonce, ciphertext)
	}
	if v2 == nil {
		return core.Secret{}, errNoPrivateRing
	}
	plain, err := v2.Open(pagetoken.Scope{TenantID: s.TenantID, StoreID: s.StoreID, BindingID: s.BindingID, Provider: s.Provider,
		AssetID: s.AssetID, Version: s.Version}, keyID, nonce, ciphertext)
	if err != nil {
		return core.Secret{}, ErrSecret
	}
	defer clear(plain)
	return core.NewSecret(plain), nil
}

var errNoPrivateRing = errors.New("metareply: no private ring")

func hasScopes(have, need []string) bool {
	for _, n := range need {
		found := false
		for _, h := range have {
			found = found || h == n
		}
		if !found {
			return false
		}
	}
	return true
}

type graphBody struct {
	Recipient struct {
		CommentID string `json:"comment_id"`
	} `json:"recipient"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
	AccessToken string `json:"access_token,omitempty"`
}

// dispatch sends the one POST. Every result other than 2xx-with-message_id is UNKNOWN
// graph_unconfirmed: the request may have been accepted, so it is never repeated (the private reply
// budget is one per comment) and only the query-only Reconcile follows. FAILED_FINAL stays unused
// until probe U3 documents the permanent error codes. A pre-send failure returns an error, which
// the dispatcher also records as UNKNOWN with zero calls.
func (a *adapter) dispatch(ctx context.Context, req core.DispatchRequest, secret core.Secret) (core.Outcome, error) {
	if isManualReply(req.Request) {
		if a.send == nil {
			return core.Outcome{}, errBadRequest
		}
		return a.send.dispatch(ctx, req, secret)
	}
	r, err := parseRequest(req)
	if err != nil {
		return core.Outcome{}, err
	}
	if r.LinkKeyID != a.linkKey.ID() || !validCommentRef(r.CommentRef) || len(secret.Reveal()) == 0 {
		return core.Outcome{}, errBadRequest
	}
	token, err := a.linkToken(req, r)
	if err != nil {
		return core.Outcome{}, err
	}
	text, err := RenderClaimLink(r.Locale, r.Origin, token)
	if err != nil {
		return core.Outcome{}, err
	}
	var body graphBody
	body.Recipient.CommentID, body.Message.Text = r.CommentRef, text
	if !a.cfg.AuthorizationHeader {
		body.AccessToken = string(secret.Reveal())
	}
	// ponytail: Go strings and net/http buffers cannot be zeroed end to end; the token is
	// process-memory only and short-lived. Upgrade path: none in stdlib.
	payload, err := json.Marshal(body)
	if err != nil {
		return core.Outcome{}, errBadRequest
	}
	url := a.cfg.GraphBaseURL + "/" + a.cfg.GraphVersion + "/" + req.ExternalAssetID + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return core.Outcome{}, errBadRequest
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if a.cfg.AuthorizationHeader {
		httpReq.Header.Set("Authorization", "Bearer "+string(secret.Reveal()))
	}
	unconfirmed := core.Outcome{State: "UNKNOWN", Code: codeUnconfirmed}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return unconfirmed, nil // transport error or timeout: the POST may have landed
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil || len(raw) > maxResponseBody || resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Graph error 190 = the Page token is invalid/expired/revoked: ask the merchant to reconnect. The outcome stays UNKNOWN
		// (never repeated: one private reply per comment); only the card status changes.
		if a.reauth != nil && err == nil && graphErrorCode(raw) == 190 {
			a.reauth(ctx, req.OperationID)
		}
		return unconfirmed, nil
	}
	var ok struct {
		MessageID   string `json:"message_id"`
		RecipientID string `json:"recipient_id"`
	}
	if json.Unmarshal(raw, &ok) != nil || !validMessageID(ok.MessageID) {
		return unconfirmed, nil
	}
	out := core.Outcome{State: "SUCCEEDED", Code: codeSent, ProviderReference: ok.MessageID}
	if psidPattern.MatchString(ok.RecipientID) { // the Send API's recipient_id links the buyer's thread to the bundle (§3.7, LC-U6)
		out.Detail = sendDetail{recipient: ok.RecipientID}
	}
	return out, nil
}

// graphErrorCode reads error.code of a Graph error envelope (0 when the body is not one).
func graphErrorCode(raw []byte) int {
	var e struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return 0
	}
	return e.Error.Code
}

// reconcile is query-only and proves nothing until probe U4 shows a read field that does; it
// never dispatches again.
func (a *adapter) reconcile(context.Context, core.DispatchRequest) (core.Outcome, error) {
	return core.Outcome{State: "UNKNOWN", Code: codeUnproven}, nil
}

func printable(s string, max int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func validMessageID(s string) bool { return printable(s, 200) }

// validCommentRef: the plain platform comment id (IR-4), 1..200 printable characters.
func validCommentRef(s string) bool { return printable(s, 200) }
