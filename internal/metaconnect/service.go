package metaconnect

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/platform"
)

// service.go is the merchant-facing connect service called by internal/httpapi/meta_connect.go. Every method runs inside the
// caller's platform.WithScope transaction (commerce_runtime, READ COMMITTED) except Callback, Pick and Disconnect, which need
// network steps between transactions. Persistence is the integration.meta_connect_* definers of migration 0095; this file shapes
// input, seals the Page token to PUBLIC HPKE keys (pagetoken: this process can seal, never open), makes the Graph calls and writes the
// audit rows. No Graph call happens with a transaction open. The USER token between callback and pick is never persisted: it sits in
// this process's memory for the state's 10 minutes (pendingUser) and is zeroed on pick or expiry.

const (
	stateDomain    = "livecommerce/meta-connect-state/v1"
	stateKeyLabel  = "livecommerce/meta-connect-state-key/v1"
	connectTimeout = 25 * time.Second // the merchant waits on a browser redirect: a stuck Meta fails fast as ErrConnectFailed
	maxPending     = 256              // user tokens held in memory at once (each lives <= 10 minutes)
)

var (
	statePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	codePattern  = regexp.MustCompile(`^[\x21-\x7e]{1,1000}$`)
)

// Config is everything the service needs; every field is validated by New.
type Config struct {
	Graph *metaoauth.Graph
	// App is the Meta app 大梦 used for the code exchange (id + secret from the existing COMMERCE_META_APPS_JSON entry) and
	// the redirect URI registered in its dashboard.
	App metaoauth.App
	// ConfigID is the Facebook Login for Business configuration (COMMERCE_META_LOGIN_CONFIG_ID); GraphVersion the API version.
	ConfigID, GraphVersion string
	// PageAppID / IGAppID are the app ids of the webhook routes (meta_inbox.routes.app_id) for the page and instagram objects.
	PageAppID, IGAppID string
	// StateKey keys the OAuth state HMAC (derived from the app secret, never the secret itself).
	StateKey []byte
	// Seal holds the HPKE PUBLIC keys that seal the Page token (meta-page-token-v2); cmd/api has no private key and cannot open one.
	Seal *pagetoken.SealKeys
}

// Service is the merchant connect service.
type Service struct {
	cfg   Config
	graph *metaoauth.Graph
	core  *core.Service

	// pending holds each live state's USER token (state id -> token), in memory only. ponytail: one API process; with several replicas
	// a pick that lands on another replica than the callback answers state_expired and the merchant restarts the connect.
	mu      sync.Mutex
	pending map[string]*pendingUser
}

type pendingUser struct {
	token         []byte
	tenant, store string
	expires       time.Time
}

// hold keeps the user token for the state's lifetime (a copy; the caller zeroes its own). Expired entries are zeroed first.
func (s *Service) hold(state, tenant, store string, token []byte, expires time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, p := range s.pending {
		if !p.expires.After(now) {
			clear(p.token)
			delete(s.pending, id)
		}
	}
	if len(s.pending) >= maxPending {
		return false
	}
	s.pending[state] = &pendingUser{token: append([]byte(nil), token...), tenant: tenant, store: store, expires: expires}
	return true
}

// peek returns a copy of the state's user token (the caller zeroes it) when it belongs to this tenant and store and has not expired.
func (s *Service) peek(state, tenant, store string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending[state]
	if p == nil || p.tenant != tenant || p.store != store || !p.expires.After(time.Now()) {
		return nil, false
	}
	return append([]byte(nil), p.token...), true
}

// drop zeroes and forgets the state's user token (after a successful pick).
func (s *Service) drop(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.pending[state]; p != nil {
		clear(p.token)
		delete(s.pending, state)
	}
}

// StateKeyFor derives the OAuth state key from the app secret (domain separated; the API already holds the secret).
func StateKeyFor(secret []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(stateKeyLabel))
	return m.Sum(nil)
}

var digits = regexp.MustCompile(`^[0-9]{1,40}$`)

// New validates cfg and the insert-only River client (core.New needs it for bindings; no queue starts here).
func New(cfg Config, jobs *river.Client[pgx.Tx]) (*Service, error) {
	inner, err := core.New(jobs)
	if err != nil {
		return nil, err
	}
	u, perr := url.Parse(cfg.App.RedirectURI)
	if cfg.Graph == nil || cfg.Seal == nil || perr != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" ||
		len(cfg.App.RedirectURI) > 512 || !digits.MatchString(cfg.App.ID) || len(cfg.App.Secret) < 1 || !digits.MatchString(cfg.ConfigID) ||
		!digits.MatchString(cfg.PageAppID) || !digits.MatchString(cfg.IGAppID) || len(cfg.StateKey) < 32 {
		return nil, command.ErrInvalid
	}
	cfg.StateKey = append([]byte(nil), cfg.StateKey...)
	cfg.App.Secret = append([]byte(nil), cfg.App.Secret...)
	return &Service{cfg: cfg, graph: cfg.Graph, core: inner, pending: map[string]*pendingUser{}}, nil
}

func tokenHash(token string) ([]byte, error) {
	if len(token) < 32 || len(token) > 512 {
		return nil, platform.ErrUnauthorized
	}
	h := sha256.Sum256([]byte(token))
	return h[:], nil
}

// ---------------------------------------------------------------------------------------------------------------------
// Start / Callback / GetState / Status
// ---------------------------------------------------------------------------------------------------------------------

// Started is the 201 body of POST meta-connect/start.
type Started struct {
	StateID   string    `json:"state_id"`
	DialogURL string    `json:"dialog_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// startReceipt is what command.Run saves for a replay: no state secret, so the receipt table never holds an OAuth state.
type startReceipt struct {
	StateID   string    `json:"state_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) dialog() metaoauth.Dialog {
	return metaoauth.Dialog{AppID: s.cfg.App.ID, ConfigID: s.cfg.ConfigID, RedirectURI: s.cfg.App.RedirectURI, GraphVersion: s.cfg.GraphVersion}
}

// Start stores the hashed state (meta_connect_begin: integration:manage, 10 min, single use) and returns the Login for
// Business dialog URL. Idempotent per Idempotency-Key (the state is re-derived, never stored).
func (s *Service) Start(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string) (out Started, err error) {
	hash, err := tokenHash(token)
	if err != nil || s == nil {
		return out, platform.ErrUnauthorized
	}
	param, digest := metaoauth.StateParam(s.cfg.StateKey, stateDomain, scope.TenantID, scope.StoreID, scope.PrincipalID, key)
	var receipt startReceipt
	err = command.Run(ctx, tx, scope, "meta.connect.start", key, struct {
		PrincipalID string `json:"principal_id"`
	}{scope.PrincipalID}, &receipt, func() error {
		// integration.meta_connect_begin: stores sha256(state) for this principal and store; at most 10 live states per store.
		if e := tx.QueryRow(ctx, `SELECT out_state::text,out_expires FROM integration.meta_connect_begin($1,$2,$3)`, hash, scope.StoreID, digest).
			Scan(&receipt.StateID, &receipt.ExpiresAt); e != nil {
			return e
		}
		return command.Audit(ctx, tx, scope, "meta.connect.started")
	})
	if err != nil {
		return out, mapError(err)
	}
	return Started{StateID: receipt.StateID, DialogURL: s.dialog().URL(param), ExpiresAt: receipt.ExpiresAt}, nil
}

// Callback consumes the state, exchanges the code (and extends it to a long-lived user token so the Page token never expires),
// reads granted permissions and the Pages with no transaction open, stores the pick list in SQL and keeps the user token in memory
// (never persisted). It returns the state id. Retry rule: the state is single-use, so a failed exchange burns it and the merchant starts a new
// connect; the exchange is never retried here (a code is single-use at Meta).
func (s *Service) Callback(ctx context.Context, pool *pgxpool.Pool, token, storeID, code, state string) (string, error) {
	if s == nil || pool == nil {
		return "", platform.ErrUnauthorized
	}
	hash, err := tokenHash(token)
	if err != nil {
		return "", err
	}
	if !statePattern.MatchString(state) {
		return "", refusal("state_mismatch")
	}
	if !codePattern.MatchString(code) {
		return "", refusal("invalid_request")
	}
	var stateID, tenant string
	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, scope platform.Scope) error {
		tenant = scope.TenantID
		// integration.meta_connect_consume: principal + store must match the state (state_mismatch), unexpired, unused.
		return tx.QueryRow(ctx, `SELECT integration.meta_connect_consume($1,$2,$3)::text`, hash, storeID, metaoauth.StateHash(state)).Scan(&stateID)
	})
	if err != nil {
		return "", mapError(err)
	}
	netCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	short, err := s.graph.Exchange(netCtx, s.cfg.App, code)
	if err != nil {
		return "", ErrConnectFailed // no cause is returned or logged: the exchange URL carries client_secret
	}
	defer clear(short)
	user, err := s.graph.Extend(netCtx, s.cfg.App, short)
	if err != nil {
		return "", ErrConnectFailed
	}
	defer clear(user) // ponytail: the string copies inside json/net/http cannot be zeroed; process-memory only
	pages, scopes, err := s.listPages(netCtx, user)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(pages)
	if err != nil {
		return "", ErrConnectFailed
	}
	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, scope platform.Scope) error {
		// integration.meta_connect_put_result: pick list and granted permissions on the consumed state, once (no token reaches SQL).
		if _, e := tx.Exec(ctx, `SELECT integration.meta_connect_put_result($1,$2,$3,$4::jsonb,$5)`, hash, storeID, stateID, string(raw), scopes); e != nil {
			return e
		}
		return command.Audit(ctx, tx, scope, "meta.connect.callback")
	})
	if err != nil {
		return "", mapError(err)
	}
	if !s.hold(stateID, tenant, storeID, user, time.Now().Add(10*time.Minute)) {
		return "", ErrConnectFailed
	}
	return stateID, nil
}

// GetState returns the pick list of a state of this principal and store (meta_connect_get_state); never the sealed token.
func (s *Service) GetState(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, stateID string) (json.RawMessage, error) {
	hash, err := tokenHash(token)
	if err != nil || !command.ValidID(stateID) {
		return nil, mapError(orInvalid(err))
	}
	return queryJSON(ctx, tx, `SELECT integration.meta_connect_get_state($1,$2,$3)::text`, hash, scope.StoreID, stateID)
}

// Status is the card: Page, Instagram, granted permissions, token status, last routed webhook (meta_connect_status).
func (s *Service) Status(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (json.RawMessage, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return nil, mapError(err)
	}
	return queryJSON(ctx, tx, `SELECT integration.meta_connect_status($1,$2)::text`, hash, scope.StoreID)
}

func queryJSON(ctx context.Context, tx pgx.Tx, sql string, args ...any) (json.RawMessage, error) {
	var raw string
	if err := tx.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		return nil, mapError(err)
	}
	return json.RawMessage(raw), nil
}

func orInvalid(err error) error {
	if err != nil {
		return err
	}
	return command.ErrInvalid
}

// ---------------------------------------------------------------------------------------------------------------------
// Pick
// ---------------------------------------------------------------------------------------------------------------------

// PickInput is the POST meta-connect/pick body: the state, the chosen Page and whether to bind its Instagram account too.
type PickInput struct {
	StateID          string `json:"state_id"`
	PageID           string `json:"page_id"`
	IncludeInstagram bool   `json:"include_instagram"`
}

// Picked is the 201 body of a successful pick.
type Picked struct {
	PageID    string `json:"page_id"`
	Instagram bool   `json:"instagram"`
}

type prepared struct {
	Page pageEntry `json:"page"`
}

// Pick binds the chosen Page. Order (a failure at any step leaves nothing half-enabled):
//  1. tx: meta_connect_prepare validates state, permissions, one-store-per-Page and returns the pick row;
//  2. network (no tx): re-read /me/accounts for the Page token and re-verify tasks, POST subscribed_apps;
//  3. tx: bindings (core), seal the Page token per binding, meta_connect_finish writes credentials + routes + connection.
//
// A crash after step 2 leaves a Meta-side subscription without a route: events for it are quarantined, harmless.
func (s *Service) Pick(ctx context.Context, pool *pgxpool.Pool, token, storeID string, in PickInput) (Picked, error) {
	if s == nil || pool == nil {
		return Picked{}, platform.ErrUnauthorized
	}
	hash, err := tokenHash(token)
	if err != nil {
		return Picked{}, err
	}
	if !command.ValidID(in.StateID) || !digits.MatchString(in.PageID) {
		return Picked{}, refusal("invalid_request")
	}
	var prep prepared
	var scope platform.Scope
	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, sc platform.Scope) error {
		scope = sc
		var raw string
		// integration.meta_connect_prepare: state ownership, pickable Page/IG, one-store-per-Page.
		if e := tx.QueryRow(ctx, `SELECT integration.meta_connect_prepare($1,$2,$3,$4,$5)::text`, hash, storeID, in.StateID, in.PageID, in.IncludeInstagram).Scan(&raw); e != nil {
			return e
		}
		return json.Unmarshal([]byte(raw), &prep)
	})
	if err != nil {
		return Picked{}, mapError(err)
	}
	// The user token lives only in this process's memory (callback -> pick); no replica memory = the connect must start again.
	user, ok := s.peek(in.StateID, scope.TenantID, storeID)
	if !ok {
		return Picked{}, refusal("state_expired")
	}
	defer clear(user)

	igID := ""
	if in.IncludeInstagram {
		igID = prep.Page.IGID
	}
	netCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	pageToken, err := s.pageToken(netCtx, user, in.PageID, igID)
	if err != nil {
		return Picked{}, err
	}
	defer clear(pageToken)
	if err = s.subscribe(netCtx, in.PageID, pageToken); err != nil {
		return Picked{}, err
	}

	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, sc platform.Scope) error {
		fb, err := s.bind(ctx, tx, sc, token, hash, in.StateID, "facebook", in.PageID, pageToken)
		if err != nil {
			return err
		}
		ig := sealed{}
		if igID != "" {
			if ig, err = s.bind(ctx, tx, sc, token, hash, in.StateID, "instagram", igID, pageToken); err != nil {
				return err
			}
		}
		// integration.meta_connect_finish: credentials (head CAS), routes, connection row, state done.
		var igBinding, igKey any
		var igExpected any
		var igNonce, igCT any
		if igID != "" {
			igBinding, igExpected, igKey, igNonce, igCT = ig.binding, ig.expected, ig.keyID, ig.nonce, ig.ct
		}
		if _, e := tx.Exec(ctx, `SELECT integration.meta_connect_finish($1,$2,$3,$4,$5::uuid,$6,$7,$8,$9,$10::uuid,$11::bigint,$12::text,$13::bytea,$14::bytea,$15,$16)`,
			hash, storeID, in.StateID, in.PageID, fb.binding, fb.expected, fb.keyID, fb.nonce, fb.ct,
			igBinding, igExpected, igKey, igNonce, igCT, s.cfg.PageAppID, s.cfg.IGAppID); e != nil {
			return e
		}
		return command.Audit(ctx, tx, sc, "meta.connect.page_connected")
	})
	if err != nil {
		return Picked{}, mapError(err)
	}
	s.drop(in.StateID) // the user token is zeroed as soon as the pick committed
	return Picked{PageID: in.PageID, Instagram: igID != ""}, nil
}

// sealed is one binding's sealed Page credential, ready for meta_connect_finish.
type sealed struct {
	binding  string
	expected int64
	keyID    string
	nonce    []byte
	ct       []byte
}

// bind ensures the store's enabled facebook/instagram binding for the asset (core.RegisterBinding for a new one,
// core.SetBindingEnabled to re-enable a disconnected one: no second binding path), reads its credential head and seals the Page
// token to the public HPKE ring under info(tenant, store, binding, provider, asset, head+1) (meta-page-token-v2).
func (s *Service) bind(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, hash []byte, state, provider, asset string, pageToken []byte) (sealed, error) {
	var id string
	var version int64
	var enabled bool
	err := tx.QueryRow(ctx, `SELECT id::text,semantic_version,enabled FROM integration.bindings
		WHERE tenant_id=$1 AND store_id=$2 AND provider=$3 AND external_asset_id=$4 ORDER BY enabled DESC,created_at,id LIMIT 1`,
		scope.TenantID, scope.StoreID, provider, asset).Scan(&id, &version, &enabled)
	// Command keys are per state and binding version: a replayed request (same state, same version) is a byte-identical replay,
	// a later reconnect (new state, new version) never collides with an earlier command.
	sum := sha256.Sum256([]byte(state + "|" + provider + "|" + strconv.FormatInt(version, 10)))
	key := hex.EncodeToString(sum[:16])
	switch {
	case err == pgx.ErrNoRows:
		b, e := s.core.RegisterBinding(ctx, tx, scope, token, "mcb-"+key, provider, asset)
		if e != nil {
			return sealed{}, mapError(e)
		}
		id = b.ID
	case err != nil:
		return sealed{}, err
	case !enabled:
		if _, e := s.core.SetBindingEnabled(ctx, tx, scope, token, "mce-"+key, id, version, true); e != nil {
			return sealed{}, mapError(e)
		}
	}
	var head int64
	// integration.meta_connect_head: the credential version CAS input (0 = none), hash-authenticated.
	if err = tx.QueryRow(ctx, `SELECT integration.meta_connect_head($1,$2,$3::uuid)`, hash, scope.StoreID, id).Scan(&head); err != nil {
		return sealed{}, err
	}
	// meta-page-token-v2: HPKE to the public ring; the API can seal this token but never open it again.
	keyID, nonce, ct, err := s.cfg.Seal.Seal(pagetoken.Scope{TenantID: scope.TenantID, StoreID: scope.StoreID, BindingID: id,
		Provider: provider, AssetID: asset, Version: head + 1}, pageToken)
	if err != nil {
		return sealed{}, ErrConnectFailed
	}
	return sealed{binding: id, expected: head, keyID: keyID, nonce: nonce, ct: ct}, nil
}

// ---------------------------------------------------------------------------------------------------------------------
// Disconnect
// ---------------------------------------------------------------------------------------------------------------------

type disconnected struct {
	PageID    string  `json:"page_id"`
	FBBinding string  `json:"fb_binding"`
	IGBinding *string `json:"ig_binding"`
}

// Disconnect destroys the sealed Page credentials, disables the routes and the bindings and deletes the connection row in ONE
// transaction (meta_connect_disconnect + core.SetBindingEnabled). This process makes no Graph unsubscribe (it can seal a Page token but
// never open one): meta_connect_disconnect enqueues a durable job (migration 0100) that the claims-worker, the only holder of the private
// ring, executes best effort (metareply.Unsubscriber). Until it runs, the disabled route only yields quarantined events.
func (s *Service) Disconnect(ctx context.Context, pool *pgxpool.Pool, token, storeID string) error {
	if s == nil || pool == nil {
		return platform.ErrUnauthorized
	}
	hash, err := tokenHash(token)
	if err != nil {
		return err
	}
	var out disconnected
	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, sc platform.Scope) error {
		var raw string
		// integration.meta_connect_disconnect: deletes heads and versions, disables routes, deletes the connection row (ids only back).
		if e := tx.QueryRow(ctx, `SELECT integration.meta_connect_disconnect($1,$2)::text`, hash, storeID).Scan(&raw); e != nil {
			return e
		}
		if e := json.Unmarshal([]byte(raw), &out); e != nil {
			return e
		}
		ids := []string{out.FBBinding}
		if out.IGBinding != nil {
			ids = append(ids, *out.IGBinding)
		}
		for _, id := range ids {
			var version int64
			var enabled bool
			if e := tx.QueryRow(ctx, `SELECT semantic_version,enabled FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
				sc.TenantID, sc.StoreID, id).Scan(&version, &enabled); e != nil {
				return e
			}
			if !enabled {
				continue
			}
			sum := sha256.Sum256([]byte(id + "|" + strconv.FormatInt(version, 10) + "|disable"))
			if _, e := s.core.SetBindingEnabled(ctx, tx, sc, token, "mcd-"+hex.EncodeToString(sum[:16]), id, version, false); e != nil {
				return e
			}
		}
		return command.Audit(ctx, tx, sc, "meta.connect.disconnected")
	})
	if err != nil {
		return mapError(err)
	}
	return nil
}
