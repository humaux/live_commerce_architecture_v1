// ingest.go owns the D8 ingest entry point: Ingest/IngestParsed turn one parsed comment
// into one claims.events row and, when accepted, an absolute line target (contract §4.3).
//
// Non-goals: no permission check (callers must already hold authority; the only v1
// production caller is RecordManualClaim), no Meta source (T10c), no text storage, no
// cart, inventory or message effect.
//
// Depends on: claims/grammar (Result shape), command (sentinels); tables
// live.claim_windows (FOR SHARE window fence), live.offers (FOR SHARE offer fence),
// claims.bundles, claims.lines and claims.events as commerce_runtime under RLS.
//
// Lock order (§5.6, caller already holds its receipt advisory): claim-source advisory →
// live.claim_windows → live.offers → claims.bundles → claims.lines → claims.events insert.

package claims

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims/grammar"
	"livecommerce/internal/command"
)

var actorKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// IngestInput is one comment delivery. Scope fields are server-derived and must equal the
// transaction GUCs; nothing here comes from a request body.
type IngestInput struct {
	TenantID      string    // must equal app.tenant_id of tx (server-derived, never from request)
	StoreID       string    // must equal app.store_id of tx
	SessionID     string    // claim context; this session's window is locked
	SourceKind    string    // v1: "manual" only ("meta" reserved for T10c -> ErrInvalid)
	SourceEventID string    // canonical UUID, unique per store across kinds
	Platform      string    // v1: "manual" only
	ActorKey      string    // 64 lowercase hex, opaque, derived by the caller
	ActorLabel    string    // manual only; required only when the bundle does not exist yet, ignored otherwise
	PrincipalID   string    // manual: must equal app.principal_id
	Text          string    // Ingest only; parsed then discarded; must be "" for IngestParsed
	OccurredAt    time.Time // UTC µs; <= clock_timestamp()+120s
}

// Redaction: IngestInput carries comment text, an actor key and a label.
func (IngestInput) String() string               { return redacted }
func (IngestInput) GoString() string             { return redacted }
func (IngestInput) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte(redacted)) }
func (IngestInput) MarshalJSON() ([]byte, error) { return redactedJSON, nil }

// IngestResult is fully reconstructible from the stored event, so a duplicate delivery
// returns the original result with Duplicate=true (§0.1 P2(c)). WINDOW_CLOSED results are
// not persisted: EventID is "" and WindowGeneration is 0.
type IngestResult struct {
	Outcome          string `json:"outcome"`
	Reason           Reason `json:"reason"`
	Duplicate        bool   `json:"duplicate"`
	EventID          string `json:"event_id"` // "" for WINDOW_CLOSED
	SessionID        string `json:"session_id"`
	WindowGeneration int64  `json:"window_generation"`
	GrammarVersion   string `json:"grammar_version"`
	OfferID          string `json:"offer_id"`
	Keyword          string `json:"keyword"` // offer keyword only
	BundleID         string `json:"bundle_id"`
	Quantity         int64  `json:"quantity"`
	PreviousQuantity int64  `json:"previous_quantity"`
	LineVersion      int64  `json:"line_version"`
	BundleVersion    int64  `json:"bundle_version"`
}

// Ingest parses Text with kw-v1 and ingests the result (= IngestParsed(grammar.Parse(Text))).
// Text above 64 KiB is ErrInvalid; the text itself is discarded before any SQL. Same
// transaction and error contract as IngestParsed. No production caller in v1 (T10c).
func Ingest(ctx context.Context, tx pgx.Tx, in IngestInput) (IngestResult, error) {
	if len(in.Text) > maxIngestTextBytes {
		return IngestResult{}, command.ErrInvalid
	}
	p := grammar.Parse(in.Text)
	in.Text = ""
	return IngestParsed(ctx, tx, in, p)
}

// IngestParsed records one already-parsed comment in the caller's READ COMMITTED
// commerce_runtime transaction (GUCs equal to in's tenant/store/principal). It performs
// no permission check. Steps (§4.3): validate → claim-source advisory + immutable-fact
// dedup (a duplicate writes nothing) → this session's OPEN window FOR SHARE (else
// WINDOW_CLOSED, not persisted) → NO_MATCH → offer FOR SHARE (UNKNOWN_KEYWORD, then
// OFFER_INACTIVE → INVALID_QUANTITY → QUANTITY_REQUIRED → QUANTITY_OVER_MAX) → bundle and
// line FOR NO KEY UPDATE (BUNDLE_LIMIT) → absolute line target + versions + ACCEPTED
// event. Every accepted command bumps line and bundle versions even if N is unchanged.
// Caller: RecordManualClaim (the only v1 production caller; KC15 guard).
func IngestParsed(ctx context.Context, tx pgx.Tx, in IngestInput, p grammar.Result) (IngestResult, error) {
	if ctx == nil || tx == nil || !validIngest(in, p) {
		return IngestResult{}, command.ErrInvalid
	}
	var isolation, tenantID, storeID, principalID string
	var inClock bool
	err := tx.QueryRow(ctx, `SELECT current_setting('transaction_isolation'),
		coalesce(current_setting('app.tenant_id',true),''),coalesce(current_setting('app.store_id',true),''),
		coalesce(current_setting('app.principal_id',true),''),$1::timestamptz<=clock_timestamp()+interval '120 seconds'`,
		in.OccurredAt).Scan(&isolation, &tenantID, &storeID, &principalID, &inClock)
	if err != nil {
		return IngestResult{}, mapError(err)
	}
	if isolation != "read committed" || tenantID != in.TenantID || storeID != in.StoreID || principalID != in.PrincipalID || !inClock {
		return IngestResult{}, command.ErrInvalid
	}
	if err := waitAdvisory(ctx, tx, "claim-source|"+in.TenantID+"|"+in.StoreID+"|"+in.SourceEventID); err != nil {
		return IngestResult{}, err
	}
	if prior, found, err := readSource(ctx, tx, in, p); err != nil || found {
		return prior, err
	}

	var w window
	err = tx.QueryRow(ctx, `SELECT generation,match_mode,opened_at FROM live.claim_windows
		WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND state='OPEN' FOR SHARE`,
		in.TenantID, in.StoreID, in.SessionID).Scan(&w.generation, &w.mode, &w.openedAt)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && in.OccurredAt.Before(w.openedAt)) {
		return IngestResult{Outcome: OutcomeRejected, Reason: ReasonWindowClosed, SessionID: in.SessionID, GrammarVersion: p.Version}, nil
	}
	if err != nil {
		return IngestResult{}, mapError(err)
	}
	if p.Kind == grammar.NoMatch {
		return record(ctx, tx, in, w, shape(p, ReasonNoMatch, offer{}))
	}

	var o offer
	err = tx.QueryRow(ctx, `SELECT id::text,keyword,sku_id::text,active,activated_at,max_quantity_per_claim FROM live.offers
		WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND keyword=$4 FOR SHARE`,
		in.TenantID, in.StoreID, in.SessionID, p.Keyword).Scan(&o.id, &o.keyword, &o.skuID, &o.active, &o.activatedAt, &o.maxQuantity)
	if errors.Is(err, pgx.ErrNoRows) {
		// Nothing about the unresolved head is stored: no keyword, quantity or offer (I03).
		return record(ctx, tx, in, w, shape(p, ReasonUnknownKeyword, offer{}))
	}
	if err != nil {
		return IngestResult{}, mapError(err)
	}
	if reason := offerReason(p, w.mode, o, in.OccurredAt); reason != "" {
		return record(ctx, tx, in, w, shape(p, reason, o))
	}

	b, err := lockBundle(ctx, tx, in)
	if err != nil {
		return IngestResult{}, err
	}
	var previous, lineVersion int64
	err = tx.QueryRow(ctx, `SELECT quantity,version FROM claims.lines
		WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3 AND offer_id=$4 FOR NO KEY UPDATE`,
		in.TenantID, in.StoreID, b.id, o.id).Scan(&previous, &lineVersion)
	newLine := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !newLine {
		return IngestResult{}, mapError(err)
	}
	if newLine && b.lineCount >= maxLinesPerBundle {
		return record(ctx, tx, in, w, shape(p, ReasonBundleLimit, o))
	}

	e := shape(p, "", o)
	e.bundleID = b.id
	if newLine {
		e.lineVersion = 1
		_, err = tx.Exec(ctx, `INSERT INTO claims.lines(tenant_id,store_id,session_id,bundle_id,offer_id,sku_id,quantity,version)
			VALUES($1,$2,$3,$4,$5,$6,$7,1)`, in.TenantID, in.StoreID, in.SessionID, b.id, o.id, o.skuID, p.Quantity)
	} else {
		e.previous = previous
		err = tx.QueryRow(ctx, `UPDATE claims.lines SET quantity=$5,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3 AND offer_id=$4 RETURNING version`,
			in.TenantID, in.StoreID, b.id, o.id, p.Quantity).Scan(&e.lineVersion)
	}
	if err != nil {
		return IngestResult{}, mapError(err)
	}
	added := 0
	if newLine {
		added = 1
	}
	err = tx.QueryRow(ctx, `UPDATE claims.bundles SET version=version+1,line_count=line_count+$4,updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND id=$3 RETURNING version`,
		in.TenantID, in.StoreID, b.id, added).Scan(&e.bundleVersion)
	if err != nil {
		return IngestResult{}, mapError(err)
	}
	return record(ctx, tx, in, w, e)
}

// window is the OPEN claim window row locked FOR SHARE for the rest of the ingest.
type window struct {
	generation int64
	mode       MatchMode
	openedAt   time.Time
}

// offer is the offer row locked FOR SHARE by ingest.
type offer struct {
	id, keyword, skuID string
	active             bool
	activatedAt        time.Time
	maxQuantity        int64
}

// event is one claims.events row in Go form. Zero values mean SQL NULL (no persisted
// quantity, version or count is ever 0); explicit is a pointer because false is a value.
type event struct {
	kind                                 grammar.Kind
	reason                               Reason // "" = ACCEPTED
	offerID, keyword                     string // keyword is the offer's, never an unresolved head
	quantity                             int64
	explicit                             *bool
	bundleID                             string
	lineVersion, previous, bundleVersion int64
}

// validIngest is the §4.3 step-1 shape check (everything but the SQL-side clock and GUCs).
func validIngest(in IngestInput, p grammar.Result) bool {
	if !command.ValidID(in.TenantID) || !command.ValidID(in.StoreID) || !command.ValidID(in.SessionID) ||
		!command.ValidID(in.SourceEventID) || !command.ValidID(in.PrincipalID) ||
		in.SourceKind != manualSource || in.Platform != manualSource || !actorKeyPattern.MatchString(in.ActorKey) ||
		in.Text != "" || in.OccurredAt.IsZero() || !in.OccurredAt.Equal(in.OccurredAt.Truncate(time.Microsecond)) ||
		in.OccurredAt.Year() < 2000 || in.OccurredAt.Year() > 2199 {
		return false
	}
	// A label reaches SQL only as a NormalizeLabel fixed point, so the bundles CHECK can
	// never reject it and echo it into the server log (§0.1 P2(b)).
	if in.ActorLabel != "" {
		if label, ok := grammar.NormalizeLabel(in.ActorLabel); !ok || label != in.ActorLabel {
			return false
		}
	}
	if p.Version != grammar.Version {
		return false
	}
	switch p.Kind {
	case grammar.Match:
		keyword, ok := grammar.NormalizeKeyword(p.Keyword)
		return ok && keyword == p.Keyword && p.Quantity >= 1 && p.Quantity <= grammar.MaxQuantity && (p.Explicit || p.Quantity == 1)
	case grammar.InvalidQuantity:
		keyword, ok := grammar.NormalizeKeyword(p.Keyword)
		return ok && keyword == p.Keyword && p.Quantity == 0 && !p.Explicit
	case grammar.NoMatch:
		return p.Keyword == "" && p.Quantity == 0 && !p.Explicit
	}
	return false
}

// offerReason applies the §2.3 precedence once the offer exists: OFFER_INACTIVE (inactive
// or occurred before activated_at) → INVALID_QUANTITY → QUANTITY_REQUIRED
// (KEYWORD_QTY_ONLY without "+N") → QUANTITY_OVER_MAX. "" means the command may be
// accepted (BUNDLE_LIMIT is decided later on the locked bundle). Pure.
func offerReason(p grammar.Result, mode MatchMode, o offer, occurredAt time.Time) Reason {
	switch {
	case !o.active || occurredAt.Before(o.activatedAt):
		return ReasonOfferInactive
	case p.Kind == grammar.InvalidQuantity:
		return ReasonInvalidQuantity
	case mode == MatchKeywordQtyOnly && !p.Explicit:
		return ReasonQuantityRequired
	case p.Quantity > o.maxQuantity:
		return ReasonQuantityOverMax
	}
	return ""
}

// shape is the §3.1 persisted-field matrix: without an offer only the reason is kept;
// with an offer its id (and keyword for the result) is kept, and quantity/explicit only
// when the grammar produced a MATCH. Pure.
func shape(p grammar.Result, reason Reason, o offer) event {
	e := event{kind: p.Kind, reason: reason}
	if o.id == "" {
		return e
	}
	e.offerID, e.keyword = o.id, o.keyword
	if p.Kind == grammar.Match {
		explicit := p.Explicit
		e.quantity, e.explicit = p.Quantity, &explicit
	}
	return e
}

// result renders an event; the fresh path and the duplicate path both use it, so a
// redelivery returns exactly the original fields.
func (e event) result(eventID, sessionID string, generation int64, duplicate bool) IngestResult {
	outcome := OutcomeAccepted
	if e.reason != "" {
		outcome = OutcomeRejected
	}
	return IngestResult{Outcome: outcome, Reason: e.reason, Duplicate: duplicate, EventID: eventID, SessionID: sessionID,
		WindowGeneration: generation, GrammarVersion: grammar.Version, OfferID: e.offerID, Keyword: e.keyword,
		BundleID: e.bundleID, Quantity: e.quantity, PreviousQuantity: e.previous, LineVersion: e.lineVersion,
		BundleVersion: e.bundleVersion}
}

// record inserts the event (RLS: source_kind='manual' and principal_id=app.principal_id)
// and returns its result. Zero-valued fields are written as NULL.
func record(ctx context.Context, tx pgx.Tx, in IngestInput, w window, e event) (IngestResult, error) {
	outcome := OutcomeAccepted
	if e.reason != "" {
		outcome = OutcomeRejected
	}
	var eventID string
	err := tx.QueryRow(ctx, `INSERT INTO claims.events(tenant_id,store_id,session_id,window_generation,source_kind,
		source_event_id,platform,occurred_at,grammar_version,grammar_kind,match_mode,outcome,reason,offer_id,quantity,
		explicit_quantity,bundle_id,line_version,previous_quantity,bundle_version,principal_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,nullif($13,''),nullif($14,'')::uuid,nullif($15::integer,0),
		$16,nullif($17,'')::uuid,nullif($18::bigint,0),nullif($19::integer,0),nullif($20::bigint,0),$21)
		RETURNING id::text`,
		in.TenantID, in.StoreID, in.SessionID, w.generation, in.SourceKind, in.SourceEventID, in.Platform, in.OccurredAt,
		grammar.Version, string(e.kind), string(w.mode), outcome, string(e.reason), e.offerID, e.quantity, e.explicit,
		e.bundleID, e.lineVersion, e.previous, e.bundleVersion, in.PrincipalID).Scan(&eventID)
	if err != nil {
		return IngestResult{}, mapError(err)
	}
	return e.result(eventID, in.SessionID, w.generation, false), nil
}

// readSource is the §4.3 step-2 dedup under the claim-source advisory lock. It compares
// only immutable delivery/parse facts (never re-resolved offer state, S03): source kind,
// platform, session, occurred_at (µs), grammar version and kind; the stored offer's
// immutable keyword; the stored quantity/explicit; and for ACCEPTED the bundle's actor
// key. A mismatch is ErrConflict; a match returns the stored outcome and writes nothing.
func readSource(ctx context.Context, tx pgx.Tx, in IngestInput, p grammar.Result) (IngestResult, bool, error) {
	var (
		e                                                 event
		eventID, sessionID, sourceKind, platform, version string
		kind, outcome, actorKey                           string
		generation                                        int64
		occurredAt                                        time.Time
		quantity, lineVersion, previous, bundleVersion    *int64
	)
	err := tx.QueryRow(ctx, `SELECT e.id::text,e.session_id::text,e.window_generation,e.source_kind,e.platform,e.occurred_at,
		e.grammar_version,e.grammar_kind,e.outcome,coalesce(e.reason,''),coalesce(e.offer_id::text,''),coalesce(o.keyword,''),
		e.quantity,e.explicit_quantity,coalesce(e.bundle_id::text,''),coalesce(b.actor_key,''),e.line_version,
		e.previous_quantity,e.bundle_version
		FROM claims.events e
		LEFT JOIN live.offers o ON o.tenant_id=e.tenant_id AND o.store_id=e.store_id AND o.id=e.offer_id
		LEFT JOIN claims.bundles b ON b.tenant_id=e.tenant_id AND b.store_id=e.store_id AND b.id=e.bundle_id
		WHERE e.tenant_id=$1 AND e.store_id=$2 AND e.source_event_id=$3`,
		in.TenantID, in.StoreID, in.SourceEventID).Scan(&eventID, &sessionID, &generation, &sourceKind, &platform, &occurredAt,
		&version, &kind, &outcome, &e.reason, &e.offerID, &e.keyword, &quantity, &e.explicit, &e.bundleID, &actorKey,
		&lineVersion, &previous, &bundleVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return IngestResult{}, false, nil
	}
	if err != nil {
		return IngestResult{}, false, mapError(err)
	}
	same := sourceKind == in.SourceKind && platform == in.Platform && sessionID == in.SessionID &&
		occurredAt.Equal(in.OccurredAt) && version == p.Version && kind == string(p.Kind)
	if e.offerID != "" {
		same = same && e.keyword == p.Keyword
	}
	if quantity != nil {
		same = same && *quantity == p.Quantity && e.explicit != nil && *e.explicit == p.Explicit
	}
	if outcome == OutcomeAccepted {
		same = same && actorKey == in.ActorKey
	}
	if !same {
		return IngestResult{}, false, command.ErrConflict
	}
	e.kind = grammar.Kind(kind)
	e.quantity, e.lineVersion, e.previous, e.bundleVersion = value(quantity), value(lineVersion), value(previous), value(bundleVersion)
	return e.result(eventID, sessionID, generation, true), true, nil
}

func value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// bundleRow is the actor's bundle locked FOR NO KEY UPDATE.
type bundleRow struct {
	id        string
	lineCount int
}

// lockBundle locks the actor's bundle, creating it (with the label) only if absent.
// An existing bundle is never INSERTed again: CHECK and RLS WITH CHECK run on the proposed
// row before conflict arbitration, so a NULL-label manual INSERT would raise 23514 (S05).
// The INSERT uses a targetless ON CONFLICT DO NOTHING so that a label already taken in the
// session (claims_bundle_label) is resolved without raising 23505, whose DETAIL would copy
// the label into the PostgreSQL server log (§0.1 P2(b)); the follow-up SELECT then finds
// no bundle for this actor key and the command fails with ErrConflict.
func lockBundle(ctx context.Context, tx pgx.Tx, in IngestInput) (bundleRow, error) {
	var b bundleRow
	selectBundle := func() error {
		return tx.QueryRow(ctx, `SELECT id::text,line_count FROM claims.bundles
			WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND platform=$4 AND actor_key=$5 FOR NO KEY UPDATE`,
			in.TenantID, in.StoreID, in.SessionID, in.Platform, in.ActorKey).Scan(&b.id, &b.lineCount)
	}
	err := selectBundle()
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return bundleRow{}, mapError(err)
	}
	if in.ActorLabel == "" {
		return bundleRow{}, command.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `INSERT INTO claims.bundles(tenant_id,store_id,session_id,platform,actor_key,label)
		VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
		in.TenantID, in.StoreID, in.SessionID, in.Platform, in.ActorKey, in.ActorLabel); err != nil {
		return bundleRow{}, mapError(err)
	}
	if err := selectBundle(); errors.Is(err, pgx.ErrNoRows) {
		return bundleRow{}, command.ErrConflict
	} else if err != nil {
		return bundleRow{}, mapError(err)
	}
	return b, nil
}
