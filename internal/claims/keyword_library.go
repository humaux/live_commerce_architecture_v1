// Purpose: the store-level keyword library (one default kw-v1 keyword per SKU) and the one-action seeding of a session's offers from it or from another session (contracts/live-keyword-claims-v1.md, amendment "Live tools (R4)" rules 1-2). The library never claims anything by itself and seeding never overwrites/deactivates an existing offer, never opens a window, never touches carts/stock/orders.
// Depends on: live.offers, catalog SKUs, internal/claims/grammar; session_copy.go (importCandidates), the A5-2 copy path that keeps live_price_minor (ruling 1) while ImportOffers omits it (rule 2 preserved).
// Used by: internal/claims merchant tools, internal/live copy.go (claims.CopyContent), tests/foundation/live_*_test.go.
package claims

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims/grammar"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// defaultLibraryMaxQuantity is the max_quantity_per_claim of an offer seeded from the library
// (the library stores no limit; Studio's own offer form defaults to the same 3).
const defaultLibraryMaxQuantity = 3

// maxLibraryRows bounds the library list response (a store with more keyworded SKUs than this
// is far beyond the 200-offer-per-session design; ponytail: add paging if a store ever needs it).
const maxLibraryRows = 1000

// Import conflict reasons (data in the 200 result, never an error).
const (
	ConflictKeywordTaken   = "keyword_taken"   // the keyword is already used in the target session by another SKU
	ConflictAlreadyPresent = "already_present" // the same keyword on the same SKU already exists in the session
	ConflictSKUTaken       = "sku_taken"       // the SKU already has an active offer in the target session
	ConflictSKUUnavailable = "sku_unavailable" // SKU or product inactive, or not in the store currency
	ConflictSessionFull    = "session_full"    // the 200-offer cap would be exceeded
)

// Import sources, receipt operations and audit actions.
const (
	importSourceLibrary     = "library"
	importSourceSession     = "session"
	importAuditAction       = "live.claim.offers.imported"
	libraryAuditSet         = "live.keyword_library.set"
	libraryAuditCleared     = "live.keyword_library.cleared"
	libraryReceiptOperation = "live.keyword_library.set"
	importReceiptOperation  = "live.claim.offers.import"
)

// LibraryEntry is one keyworded SKU. Version 0 with an empty Keyword is the answer to a removal.
type LibraryEntry struct {
	SKUID         string    `json:"sku_id"`
	SKUCode       string    `json:"sku_code"`
	ProductName   string    `json:"product_name"`
	Keyword       string    `json:"keyword"`
	Version       int64     `json:"version"`
	SKUPriceMinor int64     `json:"sku_price_minor"`
	Currency      string    `json:"currency"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LibraryInput sets (non-empty keyword) or removes (empty keyword) a SKU's default keyword with a
// version CAS; ExpectedVersion 0 means "no entry yet".
type LibraryInput struct {
	Keyword         string `json:"keyword"`
	ExpectedVersion int64  `json:"expected_version"`
}

// ImportInput selects the seed source: "library" (every library row) or "session" (the active
// offers of FromSessionID in the same store).
type ImportInput struct {
	Source        string `json:"source"`
	FromSessionID string `json:"from_session_id,omitempty"`
}

// ImportConflict reports one candidate that was NOT created and why.
type ImportConflict struct {
	Keyword string `json:"keyword"`
	SKUID   string `json:"sku_id"`
	Reason  string `json:"reason"`
}

// ImportResult is the one-action outcome: what was created and what was refused (never silent).
type ImportResult struct {
	Created   []Offer          `json:"created"`
	Conflicts []ImportConflict `json:"conflicts"`
}

// ListLibrary returns the store's library ORDER BY keyword (live:read), after a fresh authorize.
// Called by the Studio library panel through GET .../claims/library.
func ListLibrary(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string) ([]LibraryEntry, error) {
	if !command.ValidID(sessionID) {
		return nil, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return nil, err
	}
	if err := requireSession(ctx, tx, scope, sessionID); err != nil {
		return nil, err
	}
	out, err := readLibrary(ctx, tx, scope, "")
	if err != nil {
		return nil, err
	}
	return out, authorize(ctx, tx, scope, token, readPermission)
}

// SetLibraryKeyword creates, renames or removes the SKU's default keyword (live:manage), receipt
// live.keyword_library.set, audit live.keyword_library.set | .cleared. Keyword uniqueness per store
// and one keyword per SKU are database constraints (23505 -> ErrConflict). A new entry needs a
// sellable SKU (active product and SKU, store currency), exactly like CreateOffer.
func SetLibraryKeyword(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sessionID, skuID string, in LibraryInput) (LibraryEntry, error) {
	keyword := ""
	if in.Keyword != "" {
		var ok bool
		if keyword, ok = grammar.NormalizeKeyword(in.Keyword); !ok {
			return LibraryEntry{}, command.ErrInvalid
		}
	}
	if !command.ValidID(sessionID) || !command.ValidID(skuID) || in.ExpectedVersion < 0 || in.ExpectedVersion == math.MaxInt64 {
		return LibraryEntry{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return LibraryEntry{}, err
	}
	request := struct {
		PrincipalID     string `json:"principal_id"`
		SKUID           string `json:"sku_id"`
		Keyword         string `json:"keyword"`
		ExpectedVersion int64  `json:"expected_version"`
	}{scope.PrincipalID, skuID, keyword, in.ExpectedVersion}
	var out LibraryEntry
	err := command.Run(ctx, tx, scope, libraryReceiptOperation, key, request, &out, func() error {
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		if err := requireSession(ctx, tx, scope, sessionID); err != nil {
			return err
		}
		var version int64
		err := tx.QueryRow(ctx, `SELECT version FROM live.keyword_library
			WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3 FOR NO KEY UPDATE`, scope.TenantID, scope.StoreID, skuID).Scan(&version)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return mapError(err)
		}
		if version != in.ExpectedVersion { // absent row reads as version 0
			return command.ErrConflict
		}
		switch {
		case keyword == "" && !exists:
			return command.ErrNotFound
		case keyword == "":
			if _, err = tx.Exec(ctx, `DELETE FROM live.keyword_library WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`,
				scope.TenantID, scope.StoreID, skuID); err != nil {
				return mapError(err)
			}
			out = LibraryEntry{SKUID: skuID}
			return command.Audit(ctx, tx, scope, libraryAuditCleared)
		case exists:
			if _, err = tx.Exec(ctx, `UPDATE live.keyword_library SET keyword=$4,version=version+1,principal_id=$5,updated_at=clock_timestamp()
				WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, scope.TenantID, scope.StoreID, skuID, keyword, scope.PrincipalID); err != nil {
				return mapError(err)
			}
		default:
			ok, err := skuSellable(ctx, tx, scope, skuID)
			if err != nil {
				return err
			}
			if !ok {
				return command.ErrConflict
			}
			if _, err = tx.Exec(ctx, `INSERT INTO live.keyword_library(tenant_id,store_id,sku_id,keyword,principal_id) VALUES($1,$2,$3,$4,$5)`,
				scope.TenantID, scope.StoreID, skuID, keyword, scope.PrincipalID); err != nil {
				return mapError(err)
			}
		}
		entries, err := readLibrary(ctx, tx, scope, skuID)
		if err != nil {
			return err
		}
		out = entries[0]
		return command.Audit(ctx, tx, scope, libraryAuditSet)
	})
	if err != nil {
		return LibraryEntry{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return LibraryEntry{}, err
	}
	return out, nil
}

// ImportOffers seeds the session's offers from the library or from another session (live:manage),
// receipt live.claim.offers.import, audit live.claim.offers.imported. It takes the same
// 'claims-offers|t|s|session' advisory lock as CreateOffer, so the 200-offer cap, keyword and
// active-SKU checks below are race-free against concurrent offer creation. Candidates are
// classified by planImport (pure); only the creatable ones are inserted, in keyword order, with
// no live price. Conflicts are data. Called by the Studio "import" actions.
func ImportOffers(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sessionID string, in ImportInput) (ImportResult, error) {
	fromSession := in.FromSessionID
	if !command.ValidID(sessionID) ||
		(in.Source == importSourceLibrary && fromSession != "") ||
		(in.Source == importSourceSession && (!command.ValidID(fromSession) || fromSession == sessionID)) ||
		(in.Source != importSourceLibrary && in.Source != importSourceSession) {
		return ImportResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return ImportResult{}, err
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		SessionID   string `json:"session_id"`
		Source      string `json:"source"`
		From        string `json:"from_session_id"`
	}{scope.PrincipalID, sessionID, in.Source, fromSession}
	var out ImportResult
	err := command.Run(ctx, tx, scope, importReceiptOperation, key, request, &out, func() error {
		if err := waitAdvisory(ctx, tx, "claims-offers|"+scope.TenantID+"|"+scope.StoreID+"|"+sessionID); err != nil {
			return err
		}
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		if err := requireSession(ctx, tx, scope, sessionID); err != nil {
			return err
		}
		candidates, err := importCandidates(ctx, tx, scope, in.Source, fromSession)
		if err != nil {
			return err
		}
		existing, err := existingOffers(ctx, tx, scope, sessionID)
		if err != nil {
			return err
		}
		sellable, err := sellableSKUs(ctx, tx, scope, candidates)
		if err != nil {
			return err
		}
		create, conflicts := planImport(candidates, existing, sellable, maxOffersPerSession)
		out = ImportResult{Created: []Offer{}, Conflicts: conflicts}
		if len(create) > 0 {
			ids := map[string]bool{}
			for _, c := range create {
				var id string
				// live.offers (claims package table): same insert as CreateOffer, no live price (rule 2).
				if err = tx.QueryRow(ctx, `INSERT INTO live.offers(tenant_id,store_id,session_id,keyword,sku_id,max_quantity_per_claim,principal_id)
					VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text`,
					scope.TenantID, scope.StoreID, sessionID, c.Keyword, c.SKUID, c.MaxQuantity, scope.PrincipalID).Scan(&id); err != nil {
					return mapError(err)
				}
				ids[id] = true
			}
			all, err := readOffers(ctx, tx, scope, sessionID, "")
			if err != nil {
				return err
			}
			for _, o := range all {
				if ids[o.ID] {
					out.Created = append(out.Created, o)
				}
			}
		}
		return command.Audit(ctx, tx, scope, importAuditAction)
	})
	if err != nil {
		return ImportResult{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return ImportResult{}, err
	}
	return out, nil
}

// importCandidate is one offer to create: keyword + SKU + limit. LivePriceMinor is carried only
// for the whole-session copy path (internal/claims CopyContent, R5 A5); offer-import ignores it
// (Live tools (R4) rule 2: a live price is never copied by the keyword-import action).
type importCandidate struct {
	Keyword        string
	SKUID          string
	MaxQuantity    int64
	LivePriceMinor *int64
}

// existingOffer is one offer already in the target session (any active state).
type existingOffer struct {
	Keyword string
	SKUID   string
	Active  bool
}

// planImport classifies candidates against the target session. Order matters and is the contract:
// per candidate (keyword order) the first failing rule wins: already_present / keyword_taken,
// then sku_taken, then sku_unavailable, then session_full. Offers created earlier in the same run
// count (so two library keywords can never target one SKU twice). capacity is the session offer
// cap. Pure.
func planImport(candidates []importCandidate, existing []existingOffer, sellable map[string]bool, capacity int) ([]importCandidate, []ImportConflict) {
	keywords := map[string]string{} // keyword -> SKU of the offer holding it
	activeSKU := map[string]bool{}
	for _, e := range existing {
		keywords[e.Keyword] = e.SKUID
		if e.Active {
			activeSKU[e.SKUID] = true
		}
	}
	count := len(existing)
	sorted := append([]importCandidate{}, candidates...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Keyword < sorted[j].Keyword })
	create, conflicts := []importCandidate{}, []ImportConflict{}
	for _, c := range sorted {
		reason := ""
		switch {
		case keywords[c.Keyword] == c.SKUID && keywords[c.Keyword] != "":
			reason = ConflictAlreadyPresent
		case keywords[c.Keyword] != "":
			reason = ConflictKeywordTaken
		case activeSKU[c.SKUID]:
			reason = ConflictSKUTaken
		case !sellable[c.SKUID]:
			reason = ConflictSKUUnavailable
		case count >= capacity:
			reason = ConflictSessionFull
		}
		if reason != "" {
			conflicts = append(conflicts, ImportConflict{Keyword: c.Keyword, SKUID: c.SKUID, Reason: reason})
			continue
		}
		keywords[c.Keyword], activeSKU[c.SKUID] = c.SKUID, true
		count++
		create = append(create, c)
	}
	return create, conflicts
}

// importCandidates reads the seed rows: every library row (default limit) or the ACTIVE offers of
// the source session (their own limit). The source session must exist in the store.
func importCandidates(ctx context.Context, tx pgx.Tx, scope platform.Scope, source, fromSession string) ([]importCandidate, error) {
	var rows pgx.Rows
	var err error
	if source == importSourceLibrary {
		rows, err = tx.Query(ctx, `SELECT keyword,sku_id::text,$3::bigint,NULL::bigint FROM live.keyword_library WHERE tenant_id=$1 AND store_id=$2 ORDER BY keyword`,
			scope.TenantID, scope.StoreID, int64(defaultLibraryMaxQuantity))
	} else {
		if err = requireSession(ctx, tx, scope, fromSession); err != nil {
			return nil, err
		}
		rows, err = tx.Query(ctx, `SELECT keyword,sku_id::text,max_quantity_per_claim,live_price_minor FROM live.offers
			WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND active ORDER BY keyword`, scope.TenantID, scope.StoreID, fromSession)
	}
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []importCandidate{}
	for rows.Next() {
		var c importCandidate
		if err := rows.Scan(&c.Keyword, &c.SKUID, &c.MaxQuantity, &c.LivePriceMinor); err != nil {
			return nil, mapError(err)
		}
		out = append(out, c)
	}
	return out, mapError(rows.Err())
}

func existingOffers(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string) ([]existingOffer, error) {
	rows, err := tx.Query(ctx, `SELECT keyword,sku_id::text,active FROM live.offers WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
		scope.TenantID, scope.StoreID, sessionID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []existingOffer{}
	for rows.Next() {
		var e existingOffer
		if err := rows.Scan(&e.Keyword, &e.SKUID, &e.Active); err != nil {
			return nil, mapError(err)
		}
		out = append(out, e)
	}
	return out, mapError(rows.Err())
}

// sellableSKUs reports, per candidate SKU, whether it sells now in the store currency
// (catalog.skus/products, control.stores: read-only, the same predicate as CreateOffer).
func sellableSKUs(ctx context.Context, tx pgx.Tx, scope platform.Scope, candidates []importCandidate) (map[string]bool, error) {
	skus := make([]string, 0, len(candidates))
	for _, c := range candidates {
		skus = append(skus, c.SKUID)
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.status='active' AND p.status='active' AND s.currency=st.currency
		FROM catalog.skus s
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.id=ANY($3::uuid[])`, scope.TenantID, scope.StoreID, skus)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		var ok bool
		if err := rows.Scan(&id, &ok); err != nil {
			return nil, mapError(err)
		}
		out[id] = ok
	}
	return out, mapError(rows.Err())
}

// skuSellable is CreateOffer's predicate for one SKU; an unknown SKU is ErrNotFound.
func skuSellable(ctx context.Context, tx pgx.Tx, scope platform.Scope, skuID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT s.status='active' AND p.status='active' AND s.currency=st.currency
		FROM catalog.skus s
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.id=$3`, scope.TenantID, scope.StoreID, skuID).Scan(&ok)
	return ok, mapError(err)
}

// readLibrary returns the library rows ORDER BY keyword, or exactly one (ErrNotFound if absent)
// when skuID is set. SKU code, product name and catalog price are a read-only display join.
func readLibrary(ctx context.Context, tx pgx.Tx, scope platform.Scope, skuID string) ([]LibraryEntry, error) {
	rows, err := tx.Query(ctx, `SELECT l.sku_id::text,s.code,p.name,l.keyword,l.version,s.price_minor,s.currency,l.updated_at
		FROM live.keyword_library l
		JOIN catalog.skus s ON s.tenant_id=l.tenant_id AND s.store_id=l.store_id AND s.id=l.sku_id
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		WHERE l.tenant_id=$1 AND l.store_id=$2 AND ($3='' OR l.sku_id=nullif($3,'')::uuid)
		ORDER BY l.keyword LIMIT $4`, scope.TenantID, scope.StoreID, skuID, maxLibraryRows)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []LibraryEntry{}
	for rows.Next() {
		var e LibraryEntry
		if err := rows.Scan(&e.SKUID, &e.SKUCode, &e.ProductName, &e.Keyword, &e.Version, &e.SKUPriceMinor, &e.Currency, &e.UpdatedAt); err != nil {
			return nil, mapError(err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	if skuID != "" && len(out) != 1 {
		return nil, command.ErrNotFound
	}
	return out, nil
}
