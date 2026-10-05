// Purpose: the carrier CSV (contract amendment W3-02B §2) — the same order collection as the pick list
//   (fulfillment.read_pick_list), one fixed column set per carrier template (Go constants, <=15 columns),
//   RFC 4180 with UTF-8 BOM via the shared writeCSVLine/guardFormula, and the COD collect amount taken from
//   the reader's collect_minor (= total + cod_surcharge, never recomputed here).
// Depends on: fulfillment.read_pick_list (0130), identity.resolve_access (orders:read + orders:export),
//   ops.audit_events (orders.carrier_export), platform.WithScope, shared writeCSVLine/guardFormula.
// Used by: internal/httpapi/picklist.go (export route). Tests: TestCarrierExport.
// Invariants: collect_minor = total_minor + coalesce(cod_surcharge_minor,0); formula guard on a leading
//   =+-@; no carrier API call (a file for the merchant to upload); templates provisional (PL-OPEN-2).

package merchantorders

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// carrierTemplates is the ordered column list per template; the header line IS the contract (PL04).
var carrierTemplates = map[string][]string{
	"black_cat":     {"order_id", "recipient_name", "phone", "region", "city", "line1", "line2", "items", "collect_minor"},
	"hsinchu":       {"order_id", "order_number", "recipient_name", "phone", "region", "city", "line1", "line2", "items", "collect_minor"},
	"chunghwa_post": {"order_id", "recipient_name", "phone", "country", "region", "city", "postal_code", "line1", "line2", "items", "total_minor"},
	"generic": {"order_id", "order_number", "created_at_utc", "destination_kind", "recipient_name", "phone",
		"region", "city", "line1", "line2", "pickup_code", "items", "total_minor", "collect_minor"},
}

// CarrierExportFile is the finished file; it lives only in memory per request (MD9).
type CarrierExportFile struct {
	Body []byte
	Rows int
}

func ValidCarrierTemplate(template string) bool {
	_, ok := carrierTemplates[template]
	return ok
}

// CarrierExport renders the carrier CSV for the selection (orders:export + orders:read).
func CarrierExport(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, template string, in PickListSelection) (CarrierExportFile, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !ValidCarrierTemplate(template) {
		return CarrierExportFile{}, command.ErrInvalid
	}
	if err := validSelection(in); err != nil {
		return CarrierExportFile{}, err
	}
	raw, err := readPickListJSON(ctx, tx, scope, token, in)
	if err != nil {
		return CarrierExportFile{}, err
	}
	// The reader only checks orders:read; the export is bulk PII and needs the export permission too.
	if err := platform.RequirePermission(ctx, tx, scope, token, "orders:export"); err != nil {
		return CarrierExportFile{}, mapPickListError(err)
	}
	if err := command.Audit(ctx, tx, scope, "orders.carrier_export"); err != nil {
		return CarrierExportFile{}, err
	}
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF")
	header := carrierTemplates[template]
	writeCSVLine(&b, header)
	for _, row := range raw.Orders {
		cells := make([]string, len(header))
		for i, column := range header {
			cells[i] = carrierCell(column, row)
		}
		writeCSVLine(&b, cells)
	}
	return CarrierExportFile{Body: b.Bytes(), Rows: len(raw.Orders)}, nil
}

// carrierCell maps one frozen order row to one template column; phone goes through the shared mask.
func carrierCell(column string, row pickListRow) string {
	switch column {
	case "order_id":
		return row.OrderID
	case "order_number":
		return row.OrderNumber
	case "created_at_utc":
		return row.CreatedAtUTC
	case "destination_kind":
		return row.DestinationKind
	case "recipient_name":
		return row.RecipientName
	case "phone":
		return exportPhone(row.Phone)
	case "country":
		return row.Country
	case "region":
		return row.Region
	case "city":
		return row.City
	case "postal_code":
		return row.PostalCode
	case "line1":
		return row.Line1
	case "line2":
		return row.Line2
	case "pickup_code":
		return row.PickupCode
	case "items":
		parts := make([]string, len(row.Lines))
		for i, line := range row.Lines {
			parts[i] = line.SKUCode + "×" + strconv.FormatInt(line.Qty, 10)
		}
		return strings.Join(parts, "; ")
	case "total_minor":
		return strconv.FormatInt(row.TotalMinor, 10)
	case "collect_minor":
		return strconv.FormatInt(row.CollectMinor, 10)
	}
	return ""
}
