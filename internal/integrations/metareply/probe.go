// Purpose: the meta connection-health probe sweep (contract meta-connection-health-v1 §4): a River periodic job
// `meta_health_sweep_v1` on the claims-worker that claims due Pages, opens each FB Page token through the shared custody
// (v2 HPKE / v1 AES), runs the three read-only Graph calls P1 (token) / P2 (permissions) / P3 (subscribed_apps of our app),
// derives the §6 capability states with metaconnect.Derive and records them lease-fenced through
// integration.record_meta_health. All Graph traffic is READ-ONLY and goes through the fake Graph in MOCK tests (loopback)
// or the real graph.facebook.com in LIVE; a rate-limited/unknown outcome changes no capability state (§4.3).
// Depends on: openPageToken (routes.go), graphErrorCode (routes.go), the metaoauth.Graph transport, metaconnect.Derive and
// the §6 vocabulary, the SECURITY DEFINER integration.claim_meta_health_probes / integration.record_meta_health (0125),
// platform.ValidateWorkerPool with the claims_worker authority, config COMMERCE_META_PAGE_APP_ID / ADVANCED_ACCESS / DM_RECEIVER_CONFIRMED.
// Used by: cmd/claims-worker (ProbePeriodicJob + the Worker), tests/foundation MOCK smoke tests (MCH02-MCH09, MCH12).
// Invariants: sweep args are `{}` (no token in River args); the token exists in plaintext only inside probeOne and is zeroed
// on return (I11); at most one sweep per fleet via UniqueOpts{ByArgs}; the whole probe is bounded below the 60 s lease and
// River's 1-minute rescue window; nothing from a response body is logged — only page id, call #, status, code and duration.
// Status: MOCK (REAL_PG + fake Graph; LIVE probe is MCH12, NOT_RUN).
// External: graph.facebook.com GET /{version}/{page-id}?fields=id (P1), /me/permissions (P2), /{page-id}/subscribed_apps (P3), all read-only;
//   https://developers.facebook.com/docs/graph-api/reference/page/ , https://developers.facebook.com/docs/graph-api/reference/user/permissions/
//   and https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/ (retrieved 2026-10-06).

package metareply

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

// ProbeJobKind is the River kind of the periodic probe sweep (main `river` schema, queue default).
const ProbeJobKind = "meta_health_sweep_v1"

const (
	probeInterval    = 5 * time.Minute             // §4.1 periodic cadence
	probeClaimLimit  = 20                          // §4.1 rows per sweep (bounded by the 10-Page cap, I23)
	probeLease       = 60 * time.Second            // §4.1 claim lease; a dead probe is re-claimable when it expires
	probeCallTimeout = 10 * time.Second            // §4.2 each Graph call ≤ 10 s
	probeTimeout     = probeLease - 10*time.Second // the whole sweep is bounded below the lease and the rescue window
)

// probeRateLimitCodes are the Graph codes §4.2 classifies as "rate-limited" (stop, keep previous states).
var probeRateLimitCodes = map[int]bool{4: true, 17: true, 32: true, 613: true, 429: true}

// pageAppIDPattern is the Meta app id shape (COMMERCE_META_PAGE_APP_ID); P3 matches our app by it.
var pageAppIDPattern = regexp.MustCompile(`^[0-9]{1,40}$`)

// ProbeJobArgs carries nothing: the probe rows and their leases live in the database (sweep args `{}`, §10).
type ProbeJobArgs struct{}

// Kind returns ProbeJobKind.
func (ProbeJobArgs) Kind() string { return ProbeJobKind }

// InsertOpts makes the job unique by its (empty) args, so at most one sweep is in flight per worker fleet (§4.1).
func (ProbeJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ProbePeriodicJob is the every-5-minutes schedule, also run once at claims-worker start.
func ProbePeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(probeInterval),
		func() (river.JobArgs, *river.InsertOpts) { return ProbeJobArgs{}, nil },
		&river.PeriodicJobOpts{ID: ProbeJobKind, RunOnStart: true})
}

// Prober runs one sweep: claim due probes, fan out the Graph probe of each, record each result lease-fenced. It holds the
// Page-token custody (v1 AES keyring + optional v2 HPKE private ring), never the tokens themselves.
type Prober struct {
	river.WorkerDefaults[ProbeJobArgs]
	pool      *pgxpool.Pool
	keys      *PageTokenKeyring
	v2        *pageopen.Keyring // nil: v2 (HPKE) credentials are denied, same as the dispatcher routes
	graph     *metaoauth.Graph
	pageAppID string
	rcfg      metaconnect.ReaderConfig
	evidence  string // metaconnect.EvidenceMock when the Graph base is loopback, else EvidenceDesign (§3.4)
}

// NewProber returns the probe worker over the claims-worker pool the caller already validated. base and version are the
// same Graph configuration the dispatcher routes use; pageAppID is COMMERCE_META_PAGE_APP_ID (§4.2 P3). New rows start at
// evidence MOCK when base is a loopback fake Graph, else DESIGN (the probe never changes existing evidence).
func NewProber(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, base, version, pageAppID string, rcfg metaconnect.ReaderConfig) (*Prober, error) {
	if pool == nil || keys == nil || !pageAppIDPattern.MatchString(pageAppID) {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), pool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	graph, err := metaoauth.NewGraph(base, version, nil)
	if err != nil {
		return nil, err
	}
	evidence := metaconnect.EvidenceDesign
	if metaoauth.LoopbackURL(base) {
		evidence = metaconnect.EvidenceMock
	}
	return &Prober{pool: pool, keys: keys, v2: v2, graph: graph, pageAppID: pageAppID, rcfg: rcfg, evidence: evidence}, nil
}

// Timeout bounds one sweep below the claim lease so River's 1-minute rescue window never starts a second runner beside a
// probe whose lease is still held (§4.1).
func (p *Prober) Timeout(*river.Job[ProbeJobArgs]) time.Duration { return probeTimeout }

// Work claims the due probes and fans them out concurrently (≤ 20 goroutines, each ≤ 30 s of Graph), so the whole sweep
// finishes inside the lease. Each row's record is lease-fenced and idempotent; a `stale` result is a no-op, not an error.
// The first database error is returned for River to retry; rows already recorded are due again only hours later.
func (p *Prober) Work(ctx context.Context, _ *river.Job[ProbeJobArgs]) error {
	if p == nil || p.pool == nil {
		return ErrConfig
	}
	claims, err := claimProbes(ctx, p.pool)
	if err != nil {
		return err
	}
	if len(claims) == 0 {
		return nil
	}
	errs := make(chan error, len(claims))
	var wg sync.WaitGroup
	for _, c := range claims {
		wg.Add(1)
		go func(c probeClaim) {
			defer wg.Done()
			if err := p.probeAndRecord(ctx, c); err != nil {
				errs <- err
			}
		}(c)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

func (p *Prober) probeAndRecord(ctx context.Context, c probeClaim) error {
	res := p.probeOne(ctx, c)
	return recordProbe(ctx, p.pool, c, res)
}

// probeClaim is one row of integration.claim_meta_health_probes: the Page, its lease, and the FB head credential opened by
// the probe (nil credential fields when the head row is missing → outcome unknown).
type probeClaim struct {
	tenant, store, page string
	generation          int64
	lease               []byte
	fb, ig, igID        string
	scopes              []string
	version             int64
	keyID               string
	nonce, ciphertext   []byte
	hasCredential       bool
}

// probeResult is the outcome of one probe: the §4.2 classification plus the Go-derived §6 capability states (probed only).
type probeResult struct {
	outcome    string   // probed | rate_limited | unknown
	token      string   // valid | invalid | page_gone | unknown
	subcode    int      // Graph subcode of a 190 (only when token=invalid)
	perms      []string // granted permissions (nil = never read)
	permSource string   // graph | snapshot | "" (never read)
	fbFields   []string // Page webhook fields of our app (nil = unknown)
	states     []derivedState
	graphCodes []int
	evidence   string
}

// derivedState is one (binding, capability) row the probe derives and records (§4.3); the SQL CHECKs the vocabulary and
// that the binding belongs to the Page.
type derivedState struct {
	BindingID  string `json:"binding_id"`
	Provider   string `json:"provider"`
	Capability string `json:"capability"`
	State      string `json:"state"`
	Reason     string `json:"reason"`
}

// claimProbes leases up to probeClaimLimit due probes and returns their rows (including any missing-head-credential rows,
// which the probe records as unknown). The ciphertext leaves SQL only to commerce_claims_worker through the definer.
func claimProbes(ctx context.Context, pool *pgxpool.Pool) ([]probeClaim, error) {
	rows, err := pool.Query(ctx, `SELECT o_tenant::text, o_store::text, o_page, o_generation, o_lease,
			o_fb::text, o_ig::text, o_ig_id, o_scopes, o_version, o_key_id, o_nonce, o_ciphertext
		FROM integration.claim_meta_health_probes($1)`, probeClaimLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []probeClaim
	for rows.Next() {
		var c probeClaim
		var ig, igID, keyID *string
		var version *int64
		var nonce, ciphertext *[]byte
		if err := rows.Scan(&c.tenant, &c.store, &c.page, &c.generation, &c.lease,
			&c.fb, &ig, &igID, &c.scopes, &version, &keyID, &nonce, &ciphertext); err != nil {
			return nil, err
		}
		if ig != nil {
			c.ig = *ig
		}
		if igID != nil {
			c.igID = *igID
		}
		if version != nil && keyID != nil && nonce != nil && ciphertext != nil {
			c.version, c.keyID, c.nonce, c.ciphertext = *version, *keyID, *nonce, *ciphertext
			c.hasCredential = true
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// probeOne runs the §4.2 calls for one Page and derives the capability states. The token is opened here and zeroed when
// probeOne returns; nothing from a response body escapes (bodies are parsed into sets and dropped).
func (p *Prober) probeOne(ctx context.Context, c probeClaim) probeResult {
	res := probeResult{outcome: "unknown", token: metaconnect.TokenUnknown, graphCodes: []int{}, evidence: p.evidence}
	if !c.hasCredential {
		return res
	}
	secret, err := openPageToken(p.keys, p.v2, PageTokenScope{
		TenantID: c.tenant, StoreID: c.store, BindingID: c.fb, Provider: "facebook", AssetID: c.page, Version: c.version,
	}, c.keyID, c.nonce, c.ciphertext)
	if err != nil {
		return res
	}
	defer clear(secret.Reveal())
	token := secret.Reveal()

	// P1: token validity (cheapest equivalent of debug_token).
	rep, err := p.graphCall(ctx, http.MethodGet, c.page, url.Values{"fields": {"id"}}, token)
	if err != nil {
		return res // transport/oversize → unknown
	}
	res.graphCodes = append(res.graphCodes, graphErrorCode(rep.Body))
	if !rep.OK() {
		code := graphErrorCode(rep.Body)
		switch {
		case code == 190:
			res.outcome, res.token, res.subcode = "probed", metaconnect.TokenInvalid, graphErrorSubcode(rep.Body)
		case code == 100:
			res.outcome, res.token = "probed", metaconnect.TokenPageGone
		case probeRateLimitCodes[code]:
			res.outcome = "rate_limited"
		default: // 5xx or an unexpected 4xx: an outage/our egress, keep previous states
			res.outcome = "unknown"
		}
		return res
	}
	if !pageIDMatches(rep.Body, c.page) {
		return res // 2xx but garbled → unknown
	}
	res.token = metaconnect.TokenValid

	// P2: granted permissions (only after P1 valid).
	rep, err = p.graphCall(ctx, http.MethodGet, "me/permissions", nil, token)
	if err != nil {
		res.outcome = "unknown"
		return res
	}
	res.graphCodes = append(res.graphCodes, graphErrorCode(rep.Body))
	if rep.OK() {
		res.perms, res.permSource = grantedFromBody(rep.Body), metaconnect.PermSourceGraph
	} else {
		code := graphErrorCode(rep.Body)
		switch {
		case code == 100 || code == 10 || (code >= 200 && code <= 299):
			res.perms, res.permSource = c.scopes, metaconnect.PermSourceSnapshot // permission error → snapshot fallback (MCH-U1)
		case code == 190:
			res.outcome, res.token, res.subcode = "probed", metaconnect.TokenInvalid, graphErrorSubcode(rep.Body)
			return res
		case probeRateLimitCodes[code]:
			res.outcome = "rate_limited"
			return res
		default:
			res.outcome = "unknown"
			return res
		}
	}

	// P3: the Page webhook fields of our app; any failure reads as fb_fields=unknown (it never stops the sweep).
	rep, err = p.graphCall(ctx, http.MethodGet, c.page+"/subscribed_apps", nil, token)
	if err != nil {
		res.fbFields = nil
	} else {
		res.graphCodes = append(res.graphCodes, graphErrorCode(rep.Body))
		if rep.OK() {
			res.fbFields = subscribedFields(rep.Body, p.pageAppID)
		} else {
			res.fbFields = nil
		}
	}

	res.outcome = "probed"
	res.states = p.derive(c, res)
	return res
}

// graphCall is one bounded Graph GET with the token in the Authorization header (never the URL).
func (p *Prober) graphCall(ctx context.Context, method, path string, query url.Values, token []byte) (metaoauth.Reply, error) {
	callCtx, cancel := context.WithTimeout(ctx, probeCallTimeout)
	defer cancel()
	return p.graph.Do(callCtx, method, path, query, token, nil)
}

// derive turns a probed Reading into the §6 rows of the Page's bindings (FB and, when present, IG).
func (p *Prober) derive(c probeClaim, res probeResult) []derivedState {
	reading := metaconnect.Reading{
		Token:       res.token,
		Subcode:     res.subcode,
		Perms:       setMap(res.perms),
		PermSource:  res.permSource,
		Tasks:       map[string]bool{"MESSAGING": true, "MODERATE": true},
		FBFields:    setMap(res.fbFields),
		AppReview:   p.rcfg.AdvancedAccess,
		DMConfirmed: p.rcfg.DMConfirmed,
	}
	var out []derivedState
	for _, b := range []struct{ provider, binding string }{
		{"facebook", c.fb},
		{"instagram", c.ig},
	} {
		if b.binding == "" {
			continue
		}
		for _, capability := range metaconnect.Capabilities(b.provider) {
			state, reason := metaconnect.Derive(b.provider, capability, reading)
			out = append(out, derivedState{
				BindingID: b.binding, Provider: b.provider, Capability: capability, State: state, Reason: reason,
			})
		}
	}
	return out
}

// recordProbe calls integration.record_meta_health with the §4.3 p_result (no token, no body). A `stale` answer means the
// lease expired or the connection vanished: a designed no-op, not an error.
func recordProbe(ctx context.Context, pool *pgxpool.Pool, c probeClaim, res probeResult) error {
	result := map[string]any{
		"outcome":     res.outcome,
		"token":       res.token,
		"evidence":    res.evidence,
		"graph_codes": res.graphCodes,
	}
	if res.subcode != 0 {
		result["subcode"] = res.subcode
	}
	if res.outcome == "probed" {
		if res.permSource != "" {
			result["perm_source"] = res.permSource
		}
		if res.perms != nil {
			result["perms"] = res.perms
		}
		if res.fbFields != nil {
			result["fb_fields"] = res.fbFields
		}
		result["states"] = res.states
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	var got string
	if err := pool.QueryRow(ctx, `SELECT integration.record_meta_health($1, $2, $3, $4::jsonb)`,
		c.page, c.generation, c.lease, raw).Scan(&got); err != nil {
		return err
	}
	return nil
}

// setMap turns a []string into the set Derive reads: nil stays nil ("unknown"), an empty list becomes an empty set
// ("known, empty") — the distinction §3.1 requires for fb_fields and perms.
func setMap(list []string) map[string]bool {
	if list == nil {
		return nil
	}
	m := make(map[string]bool, len(list))
	for _, s := range list {
		m[s] = true
	}
	return m
}

// pageIDMatches reports whether a P1 body carries the expected Page id (`"id": "<page>"`, number or string).
func pageIDMatches(raw []byte, page string) bool {
	var doc struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	return strings.Trim(string(doc.ID), `"`) == page
}

// grantedFromBody returns the sorted de-duplicated permissions a /me/permissions body reports as "granted"; nil on a
// malformed body (caller treats that as unknown).
func grantedFromBody(raw []byte) []string {
	var doc struct {
		Data []struct {
			Permission string `json:"permission"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	seen := make(map[string]bool)
	for _, p := range doc.Data {
		if p.Status == "granted" && scopePattern.MatchString(p.Permission) {
			seen[p.Permission] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// subscribedFields returns the webhook field names of our app from a /subscribed_apps body: nil on a malformed body
// (fb_fields=unknown), the (possibly empty) field list when our app is present, and an empty list when our app is absent
// (not subscribed, §4.2 P3).
func subscribedFields(raw []byte, pageAppID string) []string {
	var doc struct {
		Data []struct {
			ID               string `json:"id"`
			SubscribedFields []struct {
				Name string `json:"name"`
			} `json:"subscribed_fields"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	for _, app := range doc.Data {
		if app.ID == pageAppID {
			out := make([]string, 0, len(app.SubscribedFields))
			for _, f := range app.SubscribedFields {
				if f.Name != "" {
					out = append(out, f.Name)
				}
			}
			return out
		}
	}
	return []string{}
}

// graphErrorSubcode reads error.error_subcode of a Graph error envelope (0 when absent or not an envelope); it maps a 190
// to token_expired (463) / token_revoked (458, 460) / token_invalid (other).
func graphErrorSubcode(raw []byte) int {
	var e struct {
		Error struct {
			Subcode int `json:"error_subcode"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return 0
	}
	return e.Error.Subcode
}
