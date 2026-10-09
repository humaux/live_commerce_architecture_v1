// Purpose: live-console-v1 §2 / §2.2 / §2.3 — the claims-worker's comment poller and the worker side
// of the internal bridge. One Console owns, per active Facebook claim source, a bounded in-memory ring
// buffer of the newest comments (≤2000, 2h age, 10min idle drop), a DB lease (live.comment_poll_leases
// via live.acquire_comment_poll_lease) that guarantees at most one Graph poller per source fleet-wide,
// and the Page-token custody (AES v1 / HPKE v2) loaded through integration.load_meta_page_token_for_poll.
// Comment text/author names live only in these process-memory buffers and in bridge responses; they are
// never logged and never persisted (I11). Instagram-live sessions are NOT polled here: the API-side
// console reads them from the encrypted webhook copy via social.read_comment_events (OPEN-4 fallback).
// comment-facts (Amendment 1 A1.2 1a) is the one IG-aware bridge read: a platform-branched single Graph read.
// Concurrency: the mutex protects the sources map and each source's state; Graph/DB credential calls are
// made with a zeroed local copy of the token and never while holding the mutex (no secret under lock).
// Depends on: metaoauth.Graph, openPageToken (routes.go), live.acquire_comment_poll_lease /
//
//	live.comment_poll_sources / live.console_source / integration.load_meta_page_token_for_poll (0123).
//
// Used by: cmd/claims-worker (NewConsole + Console.Handler); internal/integrations/metareply/bridge.go.
package metareply

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
)

// Poller constants (live-console-v1 §2.2): bounded, lease-fenced, cap-limited.
const (
	commentFields        = "id,message,from,created_time,parent,attachment"
	maxGraphPageSize     = 100
	defaultSweepInterval = 2 * time.Second
	defaultPollInterval  = 3 * time.Second
	defaultMaxBackoff    = 30 * time.Second
	defaultLeaseTTL      = 60 * time.Second
	defaultDemandTTL     = 10 * time.Second
	defaultCursorTTL     = 5 * time.Minute
	defaultCallTimeout   = 15 * time.Second
	defaultBufferCap     = 2000
	defaultBufferAge     = 2 * time.Hour
	defaultIdleDrop      = 10 * time.Minute
	defaultFleetCap      = 20
	defaultTenantCap     = 5
)

var (
	regexpHolder     = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	graphCursorShape = regexp.MustCompile(`^[A-Za-z0-9_=+/.-]{1,512}$`)
)

// ConsoleConfig is every tunable of the poller/bridge; all values are validated (no secret material).
type ConsoleConfig struct {
	Graph         Config        // GraphBaseURL/GraphVersion/HTTPClient (MOCK loopback in tests)
	HolderID      string        // stable replica id, ^[a-z0-9_-]{1,64}$ (Docker HOSTNAME)
	BridgeToken   []byte        // 32-byte shared bridge bearer
	CursorKey     []byte        // 32-byte older_cursor HMAC key
	FleetCap      int           // max polled sources fleet-wide (default 20)
	TenantCap     int           // max polled sources per tenant (default 5)
	SweepInterval time.Duration // default 2s
	PollInterval  time.Duration // default 3s
	MaxBackoff    time.Duration // default 30s
	LeaseTTL      time.Duration // default 60s
	DemandTTL     time.Duration // default 10s
	CursorTTL     time.Duration // default 5m
	CallTimeout   time.Duration // default 15s
	BufferCap     int           // default 2000
	BufferAge     time.Duration // default 2h
	IdleDrop      time.Duration // default 10m
}

// Console is the worker-side poller + bridge server. The zero value is invalid; use NewConsole.
type Console struct {
	pool        *pgxpool.Pool
	graph       *metaoauth.Graph
	keys        *PageTokenKeyring
	v2          *pageopen.Keyring
	cfg         ConsoleConfig
	holderID    string
	bridgeToken []byte
	cursorKey   []byte

	mu      sync.Mutex
	sources map[string]*consoleSource // source_id → buffer (owned only)
	pin     func(context.Context, string, string, string, string) (consolePin, bool, error)
}

// consolePin is one pinned live.console_source tuple (bridge re-check before any buffer access).
type consolePin struct {
	platform, object, assetID, sourceObjectID string
}

// consoleSource is one owned source: the DB lease fence, the ring buffer and the poll state machine.
type consoleSource struct {
	tenant, store, session, source string
	platform, object, assetID      string
	sourceObjectID                 string

	generation int64
	pollEpoch  int64
	leaseToken []byte // 32 random bytes; hash stored in live.comment_poll_leases.lease_token_hash
	leaseUntil time.Time

	// ring buffer: newest first (comments[0] is the highest seq). seq is monotonic within one epoch.
	seq      int64
	comments []consoleComment
	byRef    map[string]consoleComment

	// poll state machine
	owned        bool
	state        string // not_started|live|throttled|reauth_required|unavailable
	reason       string
	lastOKAt     time.Time
	newestAt     time.Time
	lastAfter    string // Graph after-cursor for the next forward poll
	lastBefore   string // Graph before-cursor for older backfill
	pollInterval time.Duration
	nextPollAt   time.Time
	lastPollAt   time.Time
	lastReadAt   time.Time
	demandUntil  time.Time
	token        core.Secret
	windowOpen   bool
}

type consoleComment struct {
	BridgeComment
	seq int64
}

type consoleCandidate struct {
	tenant, store, session, source string
	platform, object, assetID      string
	sourceObjectID                 string
	windowOpen                     bool
}

// NewConsole validates the config and constructs the Graph transport. pool/keys must be non-nil.
// Zero tunables are filled from the §2.2 default* constants first (cmd/claims-worker builds a
// ConsoleConfig with only Graph/HolderID/BridgeToken/CursorKey).
func NewConsole(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg ConsoleConfig) (*Console, error) {
	if pool == nil || keys == nil {
		return nil, ErrConfig
	}
	cfg = cfg.withDefaults()
	if cfg.Validate() != nil {
		return nil, ErrConfig
	}
	graph, err := metaoauth.NewGraph(cfg.Graph.GraphBaseURL, cfg.Graph.GraphVersion, cfg.Graph.HTTPClient)
	if err != nil {
		return nil, ErrConfig
	}
	c := &Console{
		pool:        pool,
		graph:       graph,
		keys:        keys,
		v2:          v2,
		cfg:         cfg,
		holderID:    cfg.HolderID,
		bridgeToken: append([]byte(nil), cfg.BridgeToken...),
		cursorKey:   append([]byte(nil), cfg.CursorKey...),
		sources:     map[string]*consoleSource{},
	}
	c.pin = c.pinSourceSQL
	return c, nil
}

// withDefaults fills every zero tunable from the §2.2 default* constants, so a caller that
// configures only the transport + secrets still gets the documented caps and intervals.
func (c ConsoleConfig) withDefaults() ConsoleConfig {
	if c.FleetCap == 0 {
		c.FleetCap = defaultFleetCap
	}
	if c.TenantCap == 0 {
		c.TenantCap = defaultTenantCap
	}
	if c.SweepInterval == 0 {
		c.SweepInterval = defaultSweepInterval
	}
	if c.PollInterval == 0 {
		c.PollInterval = defaultPollInterval
	}
	if c.MaxBackoff == 0 {
		c.MaxBackoff = defaultMaxBackoff
	}
	if c.LeaseTTL == 0 {
		c.LeaseTTL = defaultLeaseTTL
	}
	if c.DemandTTL == 0 {
		c.DemandTTL = defaultDemandTTL
	}
	if c.CursorTTL == 0 {
		c.CursorTTL = defaultCursorTTL
	}
	if c.CallTimeout == 0 {
		c.CallTimeout = defaultCallTimeout
	}
	if c.BufferCap == 0 {
		c.BufferCap = defaultBufferCap
	}
	if c.BufferAge == 0 {
		c.BufferAge = defaultBufferAge
	}
	if c.IdleDrop == 0 {
		c.IdleDrop = defaultIdleDrop
	}
	return c
}

// Validate checks every field; it never carries secret material in its error.
func (c ConsoleConfig) Validate() error {
	if c.Graph.Validate() != nil {
		return ErrConfig
	}
	if !regexpHolder.MatchString(c.HolderID) {
		return ErrConfig
	}
	if len(c.BridgeToken) != 32 || len(c.CursorKey) != 32 {
		return ErrConfig
	}
	if c.FleetCap <= 0 || c.TenantCap <= 0 || c.TenantCap > c.FleetCap {
		return ErrConfig
	}
	if c.SweepInterval <= 0 || c.PollInterval <= 0 || c.MaxBackoff < c.PollInterval ||
		c.LeaseTTL <= 0 || c.DemandTTL <= 0 || c.CursorTTL <= 0 || c.CallTimeout <= 0 ||
		c.BufferCap <= 0 || c.BufferAge <= 0 || c.IdleDrop <= 0 {
		return ErrConfig
	}
	return nil
}

// Handler is the worker-side bridge: POST /internal/v1/comment-page and /comment-facts, a fixed
// 32-byte bearer, fixed safe error codes, no body logging. Served only on the backend Docker network.
func (c *Console) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/v1/comment-page", c.handlePage)
	mux.HandleFunc("POST /internal/v1/comment-facts", c.handleFacts)
	return mux
}

// Run sweeps until ctx is cancelled, then zeroes every held token and stops. The console is demand-
// driven: an API page read acquires a source on demand (fresh buffer → reset:true), and the sweep
// keeps it polled while its window is OPEN or its demand has not expired.
func (c *Console) Run(ctx context.Context) error {
	t := time.NewTicker(c.cfg.SweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			c.stopAll()
			return nil
		case <-t.C:
			c.SweepOnce(ctx)
		}
	}
}

func (c *Console) stopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, s := range c.sources {
		clear(s.token.Reveal())
		s.token = core.Secret{}
		delete(c.sources, id)
	}
}

// SweepOnce reconciles candidates, applies fleet/tenant caps and polls every owned source once. Graph
// calls run outside the mutex with a zeroed token copy.
func (c *Console) SweepOnce(ctx context.Context) {
	now := time.Now()
	cands, err := c.pollSources(ctx)
	if err != nil {
		return
	}
	ids := make([]string, 0, len(cands))
	for id := range cands {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var toPoll []*consoleSource
	c.mu.Lock()
	// Drop sources that are no longer candidates AND whose demand has lapsed (a handler-acquired
	// source always has a fresh demandUntil, so it survives between sweeps).
	for id, s := range c.sources {
		cand, ok := cands[id]
		if !ok && s.demandUntil.Before(now) {
			clear(s.token.Reveal())
			s.token = core.Secret{}
			delete(c.sources, id)
			continue
		}
		if ok {
			s.windowOpen = cand.windowOpen
		}
	}
	// Start new sources within caps (deterministic order).
	for _, id := range ids {
		if _, ok := c.sources[id]; ok {
			continue
		}
		cand := cands[id]
		if c.fleetCountLocked() >= c.cfg.FleetCap {
			break
		}
		if c.tenantCountLocked(cand.tenant) >= c.cfg.TenantCap {
			continue
		}
		s := c.newConsoleSource(cand)
		if c.acquireLocked(ctx, s, true) {
			s.owned = true
			c.sources[id] = s
		}
	}
	// Renew leases, drop idle buffers, collect sources due for a poll.
	for id, s := range c.sources {
		if !c.renewLocked(ctx, s, now) {
			delete(c.sources, id)
			continue
		}
		if !s.windowOpen && s.demandUntil.Before(now) {
			clear(s.token.Reveal())
			s.token = core.Secret{}
			delete(c.sources, id)
			continue
		}
		if now.Sub(s.lastReadAt) > c.cfg.IdleDrop {
			// Idle drop: stop entirely; the next page read re-acquires with a fresh buffer (new
			// lease → new poll_epoch → the console sees reset:true). Zero the token.
			clear(s.token.Reveal())
			s.token = core.Secret{}
			delete(c.sources, id)
			continue
		}
		if s.object == "instagram" {
			continue // owned on demand only for comment-facts (A1.2 1a); never forward-polled
		}
		if !now.Before(s.nextPollAt) {
			toPoll = append(toPoll, s)
		}
	}
	c.mu.Unlock()

	for _, s := range toPoll {
		c.pollOne(ctx, s, now)
	}
}

func (c *Console) fleetCountLocked() int { return len(c.sources) }

func (c *Console) tenantCountLocked(tenant string) int {
	n := 0
	for _, s := range c.sources {
		if s.tenant == tenant {
			n++
		}
	}
	return n
}

func (c *Console) newConsoleSource(cand consoleCandidate) *consoleSource {
	return &consoleSource{
		tenant: cand.tenant, store: cand.store, session: cand.session, source: cand.source,
		platform: cand.platform, object: cand.object, assetID: cand.assetID, sourceObjectID: cand.sourceObjectID,
		byRef:        map[string]consoleComment{},
		state:        "not_started",
		pollInterval: c.cfg.PollInterval,
		windowOpen:   cand.windowOpen,
	}
}

// pollSources reads the candidate set (SQL definer live.comment_poll_sources, only caller the poller).
func (c *Console) pollSources(ctx context.Context) (map[string]consoleCandidate, error) {
	rows, err := c.pool.Query(ctx, `SELECT tenant_id::text,store_id::text,session_id::text,source_id::text,
		platform,object,asset_id,source_object_id,window_open FROM live.comment_poll_sources()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]consoleCandidate{}
	for rows.Next() {
		var cand consoleCandidate
		if err := rows.Scan(&cand.tenant, &cand.store, &cand.session, &cand.source,
			&cand.platform, &cand.object, &cand.assetID, &cand.sourceObjectID, &cand.windowOpen); err != nil {
			return nil, err
		}
		if cand.object == "instagram" {
			continue // IG is served from the webhook copy (OPEN-4); the FB comment field set would not parse there
		}
		out[cand.source] = cand
	}
	return out, rows.Err()
}

// acquireLocked runs the lease definer: p_new_buffer=true only when starting a fresh empty buffer
// (first acquire or expired take-over), which bumps poll_epoch so a real buffer loss surfaces as
// reset:true. Requires the mutex.
func (c *Console) acquireLocked(ctx context.Context, s *consoleSource, newBuffer bool) bool {
	if s.leaseToken == nil {
		tok, err := newLeaseToken()
		if err != nil {
			return false
		}
		s.leaseToken = tok
	}
	hash := sha256.Sum256(s.leaseToken)
	demand := s.demandUntil
	if demand.IsZero() {
		demand = time.Now()
	}
	var gen, epoch int64
	var acquired bool
	err := c.pool.QueryRow(ctx,
		`SELECT generation,poll_epoch,acquired FROM live.acquire_comment_poll_lease($1::uuid,$2::uuid,$3::uuid,$4::text,$5::bytea,$6::timestamptz,$7::timestamptz,$8::boolean)`,
		s.tenant, s.store, s.source, c.holderID, hash[:], time.Now().Add(c.cfg.LeaseTTL), demand, newBuffer).
		Scan(&gen, &epoch, &acquired)
	if err != nil || !acquired {
		return false
	}
	s.generation, s.pollEpoch = gen, epoch
	s.leaseUntil = time.Now().Add(c.cfg.LeaseTTL)
	return true
}

// renewLocked keeps the lease fresh and the demand_until current; acquired=false (another holder owns
// an unexpired lease) or a hard failure drops the source. It reloads the token when the generation
// moved (take-over) or the token is empty.
func (c *Console) renewLocked(ctx context.Context, s *consoleSource, now time.Time) bool {
	oldGen := s.generation
	if !c.acquireLocked(ctx, s, false) {
		clear(s.token.Reveal())
		s.token = core.Secret{}
		return false
	}
	if s.generation != oldGen || len(s.token.Reveal()) == 0 {
		sec, err := c.loadTokenLocked(ctx, s)
		if err != nil {
			s.state, s.reason = "unavailable", "no_credential"
			s.nextPollAt = now.Add(c.cfg.MaxBackoff)
			return true
		}
		clear(s.token.Reveal())
		s.token = sec
	}
	return true
}

// pollOne performs one forward Graph read (after-cursor) outside the mutex and commits the result.
func (c *Console) pollOne(ctx context.Context, s *consoleSource, now time.Time) {
	c.mu.Lock()
	if !now.Before(s.nextPollAt) {
		s.nextPollAt = now.Add(c.cfg.PollInterval) // move now so a slow Graph call can't double-poll
	}
	s.lastPollAt = now
	tok := c.ensureTokenLocked(ctx, s)
	objID, assetID, after := s.sourceObjectID, s.assetID, s.lastAfter
	c.mu.Unlock()

	items, newAfter, newBefore, err := c.pollGraph(ctx, objID, assetID, after, tok)
	clear(tok)

	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case err == nil:
		if newAfter != "" {
			s.lastAfter = newAfter
		}
		if newBefore != "" {
			s.lastBefore = newBefore
		}
		c.ingest(s, items)
		c.evict(s, now)
		s.state, s.reason = "live", ""
		s.lastOKAt = now
		s.pollInterval = c.cfg.PollInterval
	case errors.Is(err, errGraphReauth):
		s.state, s.reason = "reauth_required", "token_invalid"
		s.pollInterval = c.cfg.MaxBackoff
	case errors.Is(err, errGraphGone):
		s.state, s.reason = "unavailable", "source_gone"
		s.pollInterval = c.cfg.MaxBackoff
	default:
		s.state, s.reason = "throttled", "graph_unreachable"
		s.pollInterval = minDuration(s.pollInterval*2, c.cfg.MaxBackoff)
	}
	s.nextPollAt = now.Add(s.pollInterval)
}

// ensureTokenLocked returns a zeroable local copy of the source token, loading it first if empty.
func (c *Console) ensureTokenLocked(ctx context.Context, s *consoleSource) []byte {
	if len(s.token.Reveal()) == 0 {
		if sec, err := c.loadTokenLocked(ctx, s); err == nil {
			clear(s.token.Reveal())
			s.token = sec
		}
	}
	return snapSecret(s.token)
}

var (
	errGraphReauth = errors.New("metareply: graph token invalid")
	errGraphGone   = errors.New("metareply: graph object gone")
)

// pollGraph reads one page of the source's comments (the source_object_id is the live video / post).
func (c *Console) pollGraph(ctx context.Context, objID, assetID, after string, tok []byte) ([]BridgeComment, string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
	defer cancel()
	q := url.Values{"fields": {commentFields}, "order": {"chronological"}, "limit": {strconv.Itoa(maxGraphPageSize)}}
	if after != "" {
		q.Set("after", after)
	}
	rep, err := c.graph.Do(ctx, http.MethodGet, objID+"/comments", q, tok, nil)
	if err != nil || !rep.OK() {
		return nil, "", "", classifyGraphErr(rep, err)
	}
	items, newAfter, newBefore, ok := normalizeComments(rep.Body, assetID, maxGraphPageSize)
	if !ok {
		return nil, "", "", errors.New("metareply: bad comment page")
	}
	return items, newAfter, newBefore, nil
}

// classifyGraphErr maps a failed comment read to one of the fixed poll states.
func classifyGraphErr(rep metaoauth.Reply, err error) error {
	if err != nil {
		return errors.New("metareply: graph unreachable")
	}
	if rep.Status == 404 {
		return errGraphGone
	}
	if graphErrorCode(rep.Body) == 190 {
		return errGraphReauth
	}
	return errors.New("metareply: graph unreachable")
}

// normalizeComments validates and normalizes one comment page (bounded, plain Meta comment id refs,
// no from.id in the output). A malformed row fails the whole page.
func normalizeComments(body []byte, assetID string, max int) ([]BridgeComment, string, string, bool) {
	var doc struct {
		Data   []json.RawMessage `json:"data"`
		Paging struct {
			Cursors struct {
				After  string `json:"after"`
				Before string `json:"before"`
			} `json:"cursors"`
		} `json:"paging"`
	}
	if json.Unmarshal(body, &doc) != nil || len(doc.Data) > max {
		return nil, "", "", false
	}
	items := make([]BridgeComment, 0, len(doc.Data))
	for _, raw := range doc.Data {
		var row struct {
			ID          string `json:"id"`
			Message     string `json:"message"`
			CreatedTime string `json:"created_time"`
			Parent      *struct {
				ID string `json:"id"`
			} `json:"parent"`
			Attachment *json.RawMessage `json:"attachment"`
			From       *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"from"`
		}
		if json.Unmarshal(raw, &row) != nil || !commentRefShape.MatchString(row.ID) {
			return nil, "", "", false
		}
		createdAt, ok := parseMetaTime(row.CreatedTime)
		if !ok {
			return nil, "", "", false
		}
		bc := BridgeComment{
			Ref:           row.ID,
			CreatedAt:     createdAt,
			Text:          sanitizeLiveToken(row.Message, 8000),
			IsPage:        row.From != nil && row.From.ID == assetID,
			HasAttachment: row.Attachment != nil,
		}
		if row.From != nil && row.From.Name != "" {
			name := sanitizeLiveToken(row.From.Name, 255)
			bc.AuthorName = &name
		}
		if row.Parent != nil && commentRefShape.MatchString(row.Parent.ID) {
			parent := row.Parent.ID
			bc.ParentRef = &parent
		}
		items = append(items, bc)
	}
	after, before := doc.Paging.Cursors.After, doc.Paging.Cursors.Before
	if after != "" && !graphCursorShape.MatchString(after) {
		after = ""
	}
	if before != "" && !graphCursorShape.MatchString(before) {
		before = ""
	}
	return items, after, before, true
}

// ingest merges a poll page into the buffer (dedupe by ref, newest first, monotonic seq).
func (c *Console) ingest(s *consoleSource, items []BridgeComment) {
	fresh := make([]consoleComment, 0, len(items))
	for _, it := range items {
		if _, dup := s.byRef[it.Ref]; dup {
			continue
		}
		s.seq++
		cc := consoleComment{BridgeComment: it, seq: s.seq}
		fresh = append(fresh, cc)
		s.byRef[it.Ref] = cc
	}
	if len(fresh) == 0 {
		return
	}
	for i, j := 0, len(fresh)-1; i < j; i, j = i+1, j-1 {
		fresh[i], fresh[j] = fresh[j], fresh[i]
	}
	s.comments = append(fresh, s.comments...)
	if newest := fresh[0].CreatedAt; newest.After(s.newestAt) {
		s.newestAt = newest
	}
}

// evict bounds the buffer: 2h age and the cap; a deleted comment leaves the buffer within one age-out.
func (c *Console) evict(s *consoleSource, now time.Time) {
	cutoff := now.Add(-c.cfg.BufferAge)
	kept := s.comments[:0]
	for _, cc := range s.comments {
		if cc.CreatedAt.Before(cutoff) || len(kept) >= c.cfg.BufferCap {
			delete(s.byRef, cc.Ref)
			continue
		}
		kept = append(kept, cc)
	}
	s.comments = kept
}

// loadTokenLocked opens the poll-lease-fenced credential head and returns an opened Page token.
func (c *Console) loadTokenLocked(ctx context.Context, s *consoleSource) (core.Secret, error) {
	var row struct {
		tenant, store, binding, provider, asset, keyID string
		version                                        int64
		nonce, ciphertext                              []byte
		scopes                                         []string
	}
	err := c.pool.QueryRow(ctx,
		`SELECT tenant_id::text,store_id::text,binding_id::text,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested
		 FROM integration.load_meta_page_token_for_poll($1::uuid,$2::bigint,$3::bytea)`,
		s.source, s.generation, s.leaseToken).
		Scan(&row.tenant, &row.store, &row.binding, &row.provider, &row.asset, &row.version,
			&row.keyID, &row.nonce, &row.ciphertext, &row.scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Secret{}, errors.New("metareply: no poll credential")
	}
	if err != nil {
		return core.Secret{}, errors.New("metareply: credential load failed")
	}
	wantProvider := "facebook"
	if s.object == "instagram" {
		wantProvider = "instagram"
	}
	if row.provider != wantProvider || !hasScopes(row.scopes, []string{"pages_read_engagement"}) {
		return core.Secret{}, errors.New("metareply: page token lacks attested scope")
	}
	return openPageToken(c.keys, c.v2, PageTokenScope{TenantID: row.tenant, StoreID: row.store, BindingID: row.binding,
		Provider: row.provider, AssetID: row.asset, Version: row.version}, row.keyID, row.nonce, row.ciphertext)
}

// pinSourceSQL is the default live.console_source re-check (zero rows = 404).
func (c *Console) pinSourceSQL(ctx context.Context, tenant, store, session, source string) (consolePin, bool, error) {
	var p consolePin
	err := c.pool.QueryRow(ctx,
		`SELECT platform,object,asset_id,source_object_id FROM live.console_source($1::uuid,$2::uuid,$3::uuid,$4::uuid)`,
		tenant, store, session, source).Scan(&p.platform, &p.object, &p.assetID, &p.sourceObjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return consolePin{}, false, nil
	}
	if err != nil {
		return consolePin{}, false, err
	}
	return p, true, nil
}

// ensureOwnedLocked returns the owned source for a validated tuple, acquiring on demand when this
// replica does not yet own it (a console read IS the demand). ErrBridgeNotOwner when another replica
// holds an unexpired lease or a cap is reached. Requires the mutex.
func (c *Console) ensureOwnedLocked(ctx context.Context, tenant, store, session, source string, p consolePin) (*consoleSource, error) {
	if s, ok := c.sources[source]; ok && s.owned && s.tenant == tenant && s.store == store && s.session == session {
		return s, nil
	}
	if c.fleetCountLocked() >= c.cfg.FleetCap || c.tenantCountLocked(tenant) >= c.cfg.TenantCap {
		return nil, ErrBridgeNotOwner
	}
	s := c.newConsoleSource(consoleCandidate{
		tenant: tenant, store: store, session: session, source: source,
		platform: p.platform, object: p.object, assetID: p.assetID, sourceObjectID: p.sourceObjectID,
	})
	s.demandUntil = time.Now().Add(c.cfg.DemandTTL)
	if !c.acquireLocked(ctx, s, true) {
		return nil, ErrBridgeNotOwner
	}
	s.owned = true
	c.sources[source] = s
	return s, nil
}

// pageLocked reads one buffer page (after-cursor). Requires the mutex.
func (c *Console) pageLocked(s *consoleSource, req BridgePageRequest, now time.Time) BridgePage {
	out := BridgePage{Epoch: s.pollEpoch, Stream: c.streamState(s, now)}
	afterSeq := int64(-1)
	if req.After != nil && req.After.Epoch == s.pollEpoch {
		afterSeq = req.After.Seq
	}
	items := make([]BridgeComment, 0, req.Limit)
	lastSeq := afterSeq
	for _, cc := range s.comments {
		if cc.seq <= afterSeq {
			break
		}
		item := cc.BridgeComment
		seq := cc.seq // §2.6: arrival position, never the page cursor or created_at; Graph history stays nil.
		item.Seq = &seq
		items = append(items, item)
		lastSeq = cc.seq
		if len(items) >= req.Limit {
			break
		}
	}
	out.Items = items
	if lastSeq < 0 {
		lastSeq = 0
	}
	out.NextSeq = lastSeq
	if s.lastBefore != "" {
		cur := c.sealCursor(req.TenantID, req.StoreID, req.SessionID, req.SourceID, s.lastBefore, now.Add(c.cfg.CursorTTL))
		out.OlderCursor = &cur
	}
	return out
}

// graphOlder is one bounded backfill read of comments older than a signed Graph before-cursor.
func (c *Console) graphOlder(ctx context.Context, objID, assetID, before string, limit int, tok []byte) ([]BridgeComment, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
	defer cancel()
	q := url.Values{"fields": {commentFields}, "order": {"chronological"}, "before": {before}, "limit": {strconv.Itoa(limit)}}
	rep, err := c.graph.Do(ctx, http.MethodGet, objID+"/comments", q, tok, nil)
	if err != nil || !rep.OK() {
		return nil, "", ErrBridgeUnavailable
	}
	items, _, nextBefore, ok := normalizeComments(rep.Body, assetID, limit)
	if !ok {
		return nil, "", ErrBridgeUnavailable
	}
	return items, nextBefore, nil
}

// factsFields is the platform-branched single-comment field set of live-console-v1 Amendment 1 A1.2 1a:
// FB created_time/from{id}/parent{id}; IG timestamp/from{id}/parent_id (IG field set pending probe R3 /
// LC-U12, so the IG branch is MOCK). Deliberately no message/name: facts carry no comment text.
func factsFields(object string) string {
	if object == "instagram" {
		return "timestamp,from{id},parent_id"
	}
	return "created_time,from{id},parent{id}"
}

// parseCommentFacts derives {created_at,is_page,is_reply} from ONE Graph comment object (a bare object,
// not a {data:[]} page). ok=false when the body is malformed, the id is not the asked ref, or the author
// id is missing: an author that cannot be confirmed is "not found" so the API falls back / disables the
// manual reply instead of guessing is_page=false. from.id is compared to the source asset and dropped.
func parseCommentFacts(body []byte, object, assetID, ref string) (CommentFacts, bool) {
	var row struct {
		ID          string `json:"id"`
		CreatedTime string `json:"created_time"` // FB
		Timestamp   string `json:"timestamp"`    // IG
		ParentID    string `json:"parent_id"`    // IG
		Parent      *struct {
			ID string `json:"id"`
		} `json:"parent"` // FB
		From *struct {
			ID string `json:"id"`
		} `json:"from"`
	}
	if json.Unmarshal(body, &row) != nil || row.ID != ref || row.From == nil || row.From.ID == "" {
		return CommentFacts{}, false
	}
	when := row.CreatedTime
	if object == "instagram" {
		when = row.Timestamp
	}
	created, ok := parseMetaTime(when)
	if !ok {
		return CommentFacts{}, false
	}
	isReply := row.ParentID != "" || (row.Parent != nil && row.Parent.ID != "")
	return CommentFacts{Found: true, CreatedAt: &created, IsPage: row.From.ID == assetID, IsReply: isReply}, true
}

// facts resolves one comment's facts: the ring buffer first, else one platform-branched Graph read of the
// comment id (A1.2 1a). {found:false} when neither has it; the API then tries the IG webhook copy.
func (c *Console) facts(ctx context.Context, s *consoleSource, ref string, now time.Time) (CommentFacts, error) {
	c.mu.Lock()
	if cc, ok := s.byRef[ref]; ok {
		created := cc.CreatedAt
		c.mu.Unlock()
		return CommentFacts{Found: true, CreatedAt: &created, IsPage: cc.IsPage, IsReply: cc.ParentRef != nil}, nil
	}
	tok := c.ensureTokenLocked(ctx, s)
	assetID, object := s.assetID, s.object
	c.mu.Unlock()
	if len(tok) == 0 {
		return CommentFacts{}, ErrBridgeUnavailable
	}
	defer clear(tok)

	ctx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
	defer cancel()
	// Calls Graph GET /{comment_id} (live-console-v1 §2.3 / Amendment 1 A1.2 1a); token in the header only.
	rep, err := c.graph.Do(ctx, http.MethodGet, ref, url.Values{"fields": {factsFields(object)}}, tok, nil)
	if err != nil {
		return CommentFacts{}, ErrBridgeUnavailable
	}
	if rep.Status == 404 {
		return CommentFacts{Found: false}, nil
	}
	if !rep.OK() {
		return CommentFacts{}, ErrBridgeUnavailable
	}
	f, ok := parseCommentFacts(rep.Body, object, assetID, ref)
	if !ok {
		return CommentFacts{Found: false}, nil
	}
	return f, nil
}

// streamState renders §2.4 StreamState; reason is the fixed code and only set on a non-live state.
func (c *Console) streamState(s *consoleSource, now time.Time) BridgeStreamState {
	st := BridgeStreamState{
		State:           s.state,
		PollIntervalMs:  int(s.pollInterval.Milliseconds()),
		SourcePlatform:  s.platform,
		VideoEmbeddable: s.object == "page",
	}
	if !s.lastOKAt.IsZero() {
		t := s.lastOKAt
		st.LastOKAt = &t
	}
	if !s.newestAt.IsZero() {
		lag := now.Sub(s.newestAt).Milliseconds()
		st.LagMs = &lag
	}
	if s.state != "live" && s.reason != "" {
		st.Reason = s.reason
	}
	return st
}

// handlePage serves POST /internal/v1/comment-page.
func (c *Console) handlePage(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	if !c.authorized(r) {
		writeBridgeErr(w, http.StatusUnauthorized, "auth")
		return
	}
	var req BridgePageRequest
	if !decodeBridgeBody(w, r, &req) {
		return
	}
	if !command.ValidID(req.TenantID) || !command.ValidID(req.StoreID) || !command.ValidID(req.SessionID) ||
		!command.ValidID(req.SourceID) || req.Limit < 1 || req.Limit > 100 {
		writeBridgeErr(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	pin, ok, err := c.pin(r.Context(), req.TenantID, req.StoreID, req.SessionID, req.SourceID)
	if err != nil {
		writeBridgeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	if !ok {
		writeBridgeErr(w, http.StatusNotFound, "no_source")
		return
	}
	var graphCur string
	if req.BeforeCursor != nil {
		graphCur, err = c.openCursor(*req.BeforeCursor, req.TenantID, req.StoreID, req.SessionID, req.SourceID, now)
		if err != nil {
			writeBridgeErr(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
	}

	c.mu.Lock()
	s, err := c.ensureOwnedLocked(r.Context(), req.TenantID, req.StoreID, req.SessionID, req.SourceID, pin)
	if err != nil {
		c.mu.Unlock()
		writeBridgeErr(w, http.StatusMisdirectedRequest, "not_owner")
		return
	}
	s.lastReadAt = now
	s.demandUntil = now.Add(c.cfg.DemandTTL)

	if req.BeforeCursor != nil {
		tok := c.ensureTokenLocked(r.Context(), s)
		objID, assetID := s.sourceObjectID, s.assetID
		epoch := s.pollEpoch
		c.mu.Unlock()
		if len(tok) == 0 {
			writeBridgeErr(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		defer clear(tok)
		items, nextBefore, gerr := c.graphOlder(r.Context(), objID, assetID, graphCur, req.Limit, tok)
		if gerr != nil {
			writeBridgeErr(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		out := BridgePage{Epoch: epoch, Items: items, NextSeq: 0}
		c.mu.Lock()
		out.Stream = c.streamState(s, now)
		c.mu.Unlock()
		if nextBefore != "" {
			cur := c.sealCursor(req.TenantID, req.StoreID, req.SessionID, req.SourceID, nextBefore, now.Add(c.cfg.CursorTTL))
			out.OlderCursor = &cur
		}
		writeBridgeJSON(w, http.StatusOK, out)
		return
	}

	page := c.pageLocked(s, req, now)
	c.mu.Unlock()
	writeBridgeJSON(w, http.StatusOK, page)
}

// handleFacts serves POST /internal/v1/comment-facts.
func (c *Console) handleFacts(w http.ResponseWriter, r *http.Request) {
	if !c.authorized(r) {
		writeBridgeErr(w, http.StatusUnauthorized, "auth")
		return
	}
	var req BridgeFactsRequest
	if !decodeBridgeBody(w, r, &req) {
		return
	}
	if !command.ValidID(req.TenantID) || !command.ValidID(req.StoreID) || !command.ValidID(req.SessionID) ||
		!command.ValidID(req.SourceID) || !commentRefShape.MatchString(req.CommentRef) {
		writeBridgeErr(w, http.StatusBadRequest, "invalid_ref")
		return
	}
	pin, ok, err := c.pin(r.Context(), req.TenantID, req.StoreID, req.SessionID, req.SourceID)
	if err != nil {
		writeBridgeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	if !ok {
		writeBridgeErr(w, http.StatusNotFound, "no_source")
		return
	}
	c.mu.Lock()
	s, err := c.ensureOwnedLocked(r.Context(), req.TenantID, req.StoreID, req.SessionID, req.SourceID, pin)
	if err != nil {
		c.mu.Unlock()
		writeBridgeErr(w, http.StatusMisdirectedRequest, "not_owner")
		return
	}
	c.mu.Unlock()

	facts, err := c.facts(r.Context(), s, req.CommentRef, time.Now())
	if err != nil {
		writeBridgeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	writeBridgeJSON(w, http.StatusOK, facts)
}

func (c *Console) authorized(r *http.Request) bool {
	tok, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if err != nil {
		return false
	}
	return constantTokenEqual(tok, c.bridgeToken)
}

func decodeBridgeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	var extra json.RawMessage
	if dec.Decode(out) != nil || dec.Decode(&extra) == nil {
		writeBridgeErr(w, http.StatusBadRequest, "invalid_cursor")
		return false
	}
	return true
}

func writeBridgeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeBridgeErr(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func newLeaseToken() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// snapSecret returns a local copy of a Secret's bytes so a Graph call can run outside the mutex and be
// zeroed afterwards; the stored Secret is only ever cleared under the mutex.
func snapSecret(s core.Secret) []byte {
	b := s.Reveal()
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// parseMetaTime accepts RFC 3339 and Meta's "+0000" offset form; anything else is rejected.
func parseMetaTime(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
