// Purpose: the claims-worker side of the manual sends (live-console-v1 §3.3/§4.3): one shared sendAdapter that implements Check (SQL
// inbox.check_send, lock-free), the secret loader (Page token + sealed dispatch copy), Dispatch (one Graph POST per operation, never
// repeated), query-only Reconcile and the Finish hook (inbox.finish_send) for DM (meta.dm_send), manual private reply (the existing
// meta.private_reply route, message_type manual_private_reply), public reply and the offer recommend comment. SendRoutes registers the
// DM / public / recommend routes; RoutesV2 hands the private-reply route the same adapter.
// Depends on: internal/integrations/core (DispatchRoute, Secret, Outcome), pagetoken/pageopen (OpenSend), meta.SocialPeerKey, SQL
// inbox.check_send / load_send_secret / finish_send, integration.load_meta_page_token, integration.mark_capability_evidence (0125),
// integration.meta_connect_mark_reauth (0095); Graph POST /{asset}/messages, /{comment}/comments, /{ig_comment}/replies, /{live}/comments.
// Used by: cmd/claims-worker (SendRoutes), routes.go (RoutesV2 private-reply branch).
// Invariants: LCN06 (no message tag, RESPONSE only, Check never calls Graph), LCN11 (UNKNOWN is never re-POSTed; Reconcile is query-only and
// answers UNKNOWN with zero HTTP, also for a redacted request), I11 (token never in a URL or log; PSID and text only in memory).
// Status: MOCK (Graph permanent-4xx codes beyond 100/2018278 wait for LC-U9).

package metareply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/platform"
)

const (
	actionDM        = "meta.dm_send"
	actionPublic    = "meta.public_reply"
	actionRecommend = "meta.offer_recommend"
	actionPrivate   = "meta.private_reply"
	manualType      = "manual_private_reply"
	sendPolicy      = "lcn-policy/v1"

	codeInvalidRequest = "invalid_request"
)

var (
	sendIDPattern    = regexp.MustCompile(`^[0-9_]{1,80}$`)
	psidPattern      = regexp.MustCompile(`^[0-9A-Za-z_-]{1,200}$`)
	checkCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// sendRequest is the part of the frozen operation request (§4.1, ≤ 2 KiB, no text, no PSID) the adapter reads.
type sendRequest struct {
	V            int    `json:"v"`
	Kind         string `json:"kind"`
	MessageType  string `json:"message_type"`
	Origin       string `json:"origin"`
	Platform     string `json:"platform"`
	AssetID      string `json:"asset_id"`
	AppID        string `json:"app_id"`
	CommentRef   string `json:"comment_ref"`
	LiveObjectID string `json:"live_object_id"`
	Policy       string `json:"policy"`
	BundleID     string `json:"bundle_id"`
	Redacted     bool   `json:"redacted"`
}

// packedSecret is what the send LoadSecret hands DispatchWithSecret: the Page token plus the opened dispatch copy, in memory only.
type packedSecret struct {
	Token string `json:"t"`
	PSID  string `json:"p,omitempty"`
	Text  string `json:"x"`
}

// sendDetail is the route-private Outcome.Detail handed to Finish (recipient_id of the Send API response).
type sendDetail struct{ recipient string }

// sendAdapter is shared by the four send actions.
type sendAdapter struct {
	check  func(ctx context.Context, operationID string) (string, error)
	reauth func(ctx context.Context, operationID string)
	keys   *PageTokenKeyring
	v2     *pageopen.Keyring
	cfg    Config
	client *http.Client
}

func newSendAdapter(check func(context.Context, string) (string, error), reauth func(context.Context, string), keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) (*sendAdapter, error) {
	if check == nil || keys == nil || cfg.Validate() != nil {
		return nil, ErrConfig
	}
	client := &http.Client{}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		client = &copied
	}
	// A 307 would replay the POST body (with the token) to another host.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &sendAdapter{check: check, reauth: reauth, keys: keys, v2: v2, cfg: cfg, client: client}, nil
}

// SendRoutes registers the DM, public-reply and offer-recommend routes (facebook and instagram for DM/public, facebook for recommend)
// on the claims-worker pool. The manual private reply shares the existing meta.private_reply route (RoutesV2). pool must be the
// commerce_claims_worker pool (platform.ValidateWorkerPool).
func SendRoutes(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) ([]core.DispatchRoute, error) {
	a, err := newSendAdapterFor(pool, keys, v2, cfg)
	if err != nil {
		return nil, err
	}
	return a.routes(), nil
}

func newSendAdapterFor(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) (*sendAdapter, error) {
	if pool == nil {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), pool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	check := func(ctx context.Context, op string) (code string, err error) {
		// inbox.check_send (0128, owner commerce_integration_writer): lock-free re-check of the frozen operation.
		err = pool.QueryRow(ctx, `SELECT inbox.check_send($1::uuid)`, op).Scan(&code)
		return code, err
	}
	reauth := func(ctx context.Context, op string) {
		_, _ = pool.Exec(ctx, `SELECT integration.meta_connect_mark_reauth($1::uuid)`, op)
	}
	return newSendAdapter(check, reauth, keys, v2, cfg)
}

func (a *sendAdapter) routes() []core.DispatchRoute {
	var out []core.DispatchRoute
	add := func(provider, action string, scopes []string) {
		out = append(out, core.DispatchRoute{
			Provider: provider, Action: action, Purpose: "service",
			Check:              a.checkRoute,
			LoadSecret:         a.withDispatchCopy(a.loadSecretFor(provider, scopes)),
			DispatchWithSecret: a.dispatch,
			Reconcile:          a.reconcile,
			Finish:             a.finish,
		})
	}
	// Capability scopes per §6; the IG dm_session names no permission of its own (derive.go).
	add("facebook", actionDM, []string{"pages_messaging"})
	add("instagram", actionDM, nil)
	add("facebook", actionPublic, []string{"pages_manage_engagement"})
	add("instagram", actionPublic, []string{"instagram_manage_comments"})
	add("facebook", actionRecommend, []string{"pages_manage_engagement"})
	return out
}

// parseSend validates the frozen request against the dispatch envelope (kind/action agreement, ids, policy tag).
func parseSend(req core.DispatchRequest) (sendRequest, error) {
	var r sendRequest
	dec := json.NewDecoder(bytes.NewReader(req.Request))
	if err := dec.Decode(&r); err != nil || r.V != 1 || r.Policy != sendPolicy || r.AssetID != req.ExternalAssetID || r.Redacted {
		return sendRequest{}, errBadRequest
	}
	switch req.Action {
	case actionDM:
		if r.Kind != "dm" {
			return sendRequest{}, errBadRequest
		}
	case actionPrivate:
		if r.Kind != "private_reply" || r.MessageType != manualType || !sendIDPattern.MatchString(r.CommentRef) {
			return sendRequest{}, errBadRequest
		}
	case actionPublic:
		if r.Kind != "public_reply" || !sendIDPattern.MatchString(r.CommentRef) {
			return sendRequest{}, errBadRequest
		}
	case actionRecommend:
		if r.Kind != "recommend" || !sendIDPattern.MatchString(r.LiveObjectID) {
			return sendRequest{}, errBadRequest
		}
	default:
		return sendRequest{}, errBadRequest
	}
	if r.Platform != req.Provider {
		return sendRequest{}, errBadRequest
	}
	return r, nil
}

// checkRoute is Check (every attempt): zero Graph calls. Only a returned deny code is a policy denial (BLOCKED_POLICY); an
// infrastructure error is a plain error (recorded UNKNOWN policy_check_failed, the send is not repeated).
func (a *sendAdapter) checkRoute(ctx context.Context, req core.DispatchRequest) error {
	if _, err := parseSend(req); err != nil {
		return core.DenyPolicy(codeInvalidRequest)
	}
	code, err := a.check(ctx, req.OperationID)
	if err != nil {
		return errors.New("metareply: send check unavailable")
	}
	if code == "OK" {
		return nil
	}
	if !checkCodePattern.MatchString(code) {
		code = "policy_denied"
	}
	return core.DenyPolicy(code)
}

// loadSecretFor loads the Page token (integration.load_meta_page_token, lease-fenced) and then the sealed dispatch copy
// (inbox.load_send_secret, same fence), opens it with the HPKE private ring and packs both in one Secret. An operation without a
// dispatch copy (the automatic claim-link reply) gets the raw token, exactly as before.
func (a *sendAdapter) loadSecretFor(provider string, scopes []string) func(context.Context, pgx.Tx, core.SecretClaim) (core.Secret, error) {
	return pageSecretLoader(a.keys, a.v2, provider, scopes, `SELECT tenant_id::text,store_id::text,binding_id::text,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested
			FROM integration.load_meta_page_token($1::uuid,$2::bigint,$3::bytea)`)
}

func (a *sendAdapter) withDispatchCopy(page func(context.Context, pgx.Tx, core.SecretClaim) (core.Secret, error)) func(context.Context, pgx.Tx, core.SecretClaim) (core.Secret, error) {
	return func(ctx context.Context, tx pgx.Tx, claim core.SecretClaim) (core.Secret, error) {
		token, err := page(ctx, tx, claim)
		if err != nil {
			return core.Secret{}, err
		}
		var tenant, store string
		var sealed, enc []byte
		// inbox.load_send_secret (0128): zero rows = no dispatch copy.
		err = tx.QueryRow(ctx, `SELECT tenant_id::text, store_id::text, sealed, enc FROM inbox.load_send_secret($1::uuid,$2::bigint,$3::bytea)`,
			claim.OperationID, claim.Generation, claim.LeaseToken).Scan(&tenant, &store, &sealed, &enc)
		if errors.Is(err, pgx.ErrNoRows) {
			return token, nil
		}
		if err != nil {
			clear(token.Reveal())
			return core.Secret{}, errors.New("metareply: send secret load failed")
		}
		if a.v2 == nil {
			clear(token.Reveal())
			return core.Secret{}, fmt.Errorf("send secret without private ring: %w", core.ErrPolicyDenied)
		}
		plain, err := a.v2.OpenSend(tenant, store, claim.OperationID, enc, sealed)
		if err != nil {
			clear(token.Reveal())
			return core.Secret{}, fmt.Errorf("send secret unreadable: %w", core.ErrPolicyDenied)
		}
		defer clear(plain)
		var inner struct {
			PSID string `json:"recipient_psid"`
			Text string `json:"text"`
		}
		if json.Unmarshal(plain, &inner) != nil || inner.Text == "" {
			clear(token.Reveal())
			return core.Secret{}, fmt.Errorf("send secret malformed: %w", core.ErrPolicyDenied)
		}
		packed, err := json.Marshal(packedSecret{Token: string(token.Reveal()), PSID: inner.PSID, Text: inner.Text})
		clear(token.Reveal())
		if err != nil {
			return core.Secret{}, errors.New("metareply: send secret pack failed")
		}
		defer clear(packed)
		return core.NewSecret(packed), nil
	}
}

// sendCall is one prepared Graph POST.
type sendCall struct {
	path string // under /{version}/
	body map[string]any
}

// build derives the Graph POST of one operation from the frozen request and the opened dispatch copy.
func build(req core.DispatchRequest, r sendRequest, s packedSecret) (sendCall, error) {
	if s.Text == "" {
		return sendCall{}, errBadRequest
	}
	switch req.Action {
	case actionDM:
		if !psidPattern.MatchString(s.PSID) {
			return sendCall{}, errBadRequest
		}
		// messaging_type RESPONSE inside the 24 h window; there is deliberately no message tag and no UPDATE type, ever (§3.3).
		return sendCall{path: req.ExternalAssetID + "/messages", body: map[string]any{
			"recipient": map[string]any{"id": s.PSID}, "message": map[string]any{"text": s.Text}, "messaging_type": "RESPONSE"}}, nil
	case actionPrivate:
		return sendCall{path: req.ExternalAssetID + "/messages", body: map[string]any{
			"recipient": map[string]any{"comment_id": r.CommentRef}, "message": map[string]any{"text": s.Text}}}, nil
	case actionPublic:
		edge := "comments"
		if req.Provider == "instagram" {
			edge = "replies"
		}
		return sendCall{path: r.CommentRef + "/" + edge, body: map[string]any{"message": s.Text}}, nil
	case actionRecommend:
		return sendCall{path: r.LiveObjectID + "/comments", body: map[string]any{"message": s.Text}}, nil
	}
	return sendCall{}, errBadRequest
}

// dispatch sends the one POST. 2xx with an id → SUCCEEDED; a 400 whose Graph error.code is 100 or 2018278 and whose body carries no
// message_id/id is a request Meta provably did not accept → FAILED_FINAL invalid_request; EVERYTHING else (other 4xx before LC-U9,
// 5xx, 429, timeout, transport, unparsable) is UNKNOWN: the POST may have landed, so it is never repeated and only the query-only
// Reconcile follows. A 190 additionally flips the connect card to "reconnect" (the outcome stays UNKNOWN).
func (a *sendAdapter) dispatch(ctx context.Context, req core.DispatchRequest, secret core.Secret) (core.Outcome, error) {
	r, err := parseSend(req)
	if err != nil {
		return core.Outcome{}, err
	}
	var s packedSecret
	if err := json.Unmarshal(secret.Reveal(), &s); err != nil || s.Token == "" {
		return core.Outcome{}, errBadRequest
	}
	defer func() { s.Token, s.PSID, s.Text = "", "", "" }()
	call, err := build(req, r, s)
	if err != nil {
		return core.Outcome{}, err
	}
	if !a.cfg.AuthorizationHeader {
		call.body["access_token"] = s.Token
	}
	// ponytail: Go strings and net/http buffers cannot be zeroed end to end; the token is process-memory only and short-lived.
	payload, err := json.Marshal(call.body)
	if err != nil {
		return core.Outcome{}, errBadRequest
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.GraphBaseURL+"/"+a.cfg.GraphVersion+"/"+call.path, bytes.NewReader(payload))
	if err != nil {
		return core.Outcome{}, errBadRequest
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if a.cfg.AuthorizationHeader {
		httpReq.Header.Set("Authorization", "Bearer "+s.Token)
	}
	unconfirmed := core.Outcome{State: "UNKNOWN", Code: codeUnconfirmed}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return unconfirmed, nil // transport error or timeout: the POST may have landed
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil || len(raw) > maxResponseBody {
		return unconfirmed, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		code := graphErrorCode(raw)
		if code == 190 && a.reauth != nil {
			a.reauth(ctx, req.OperationID)
		}
		if resp.StatusCode == http.StatusBadRequest && (code == 100 || code == 2018278) && !bodyHasID(raw) {
			return core.Outcome{State: "FAILED_FINAL", Code: codeInvalidRequest}, nil
		}
		return unconfirmed, nil
	}
	var ok struct {
		MessageID   string `json:"message_id"`
		ID          string `json:"id"`
		RecipientID string `json:"recipient_id"`
	}
	if json.Unmarshal(raw, &ok) != nil {
		return unconfirmed, nil
	}
	ref := ok.MessageID
	if ref == "" {
		ref = ok.ID
	}
	if !validMessageID(ref) {
		return unconfirmed, nil
	}
	out := core.Outcome{State: "SUCCEEDED", Code: codeSent, ProviderReference: ref}
	if ok.RecipientID != "" && psidPattern.MatchString(ok.RecipientID) {
		out.Detail = sendDetail{recipient: ok.RecipientID}
	}
	return out, nil
}

// bodyHasID reports whether a Graph body carries a message_id or id (an accepted request can answer an error-shaped envelope).
func bodyHasID(raw []byte) bool {
	var b struct {
		MessageID string `json:"message_id"`
		ID        string `json:"id"`
	}
	return json.Unmarshal(raw, &b) == nil && (b.MessageID != "" || b.ID != "")
}

// reconcile is query-only and proves nothing (MCI U4 unresolved): UNKNOWN with zero HTTP calls, for a redacted request too (A1.4 clause 4).
func (a *sendAdapter) reconcile(context.Context, core.DispatchRequest) (core.Outcome, error) {
	return core.Outcome{State: "UNKNOWN", Code: codeUnproven}, nil
}

// finish is the completion-transaction hook: it wipes the dispatch copy on every path, advances last_outbound_at (DM), records the
// bundle↔peer link of a SUCCEEDED private reply and upgrades the capability evidence label. peer_key is computed here from the Send API's
// recipient_id (meta.SocialPeerKey) because it is an unkeyed tuple hash the database cannot reproduce.
func (a *sendAdapter) finish(ctx context.Context, tx pgx.Tx, claim core.SecretClaim, out core.Outcome) error {
	var peer *string
	var action, platformName string
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT action, provider, request FROM integration.operations WHERE id=$1::uuid`, claim.OperationID).Scan(&action, &platformName, &raw); err != nil {
		return err
	}
	if d, ok := out.Detail.(sendDetail); ok && out.State == "SUCCEEDED" && action == actionPrivate {
		var r sendRequest
		if json.Unmarshal(raw, &r) == nil && r.AppID != "" && r.AssetID != "" {
			object := "page"
			if platformName == "instagram" {
				object = "instagram"
			}
			key := meta.SocialPeerKey(r.AppID, object, r.AssetID, d.recipient)
			peer = &key
		}
	}
	// inbox.finish_send (0128): lease-fenced; wipes inbox.send_secrets, last_outbound_at, inbox.bundle_peers ON CONFLICT DO NOTHING.
	if _, err := tx.Exec(ctx, `SELECT inbox.finish_send($1::uuid,$2::bigint,$3::bytea,$4,$5)`, claim.OperationID, claim.Generation, claim.LeaseToken, out.State, peer); err != nil {
		return err
	}
	if out.State == "SUCCEEDED" {
		evidence := "MOCK"
		if a.cfg.GraphBaseURL == graphHost {
			evidence = "LIVE_SEND"
		}
		capability := map[string]string{actionDM: "dm_session", actionPrivate: "private_reply", actionPublic: "reply_public", actionRecommend: "reply_public"}[action]
		if capability != "" {
			// integration.mark_capability_evidence (0125): monotonic label upgrade; a first send never needs it to pass.
			if _, err := tx.Exec(ctx, `SELECT integration.mark_capability_evidence($1::uuid,$2,$3)`, claim.OperationID, capability, evidence); err != nil {
				return err
			}
		}
	}
	return nil
}
