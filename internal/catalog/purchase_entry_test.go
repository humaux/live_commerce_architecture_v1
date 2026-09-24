package catalog

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const purchaseID = "33333333-3333-4333-8333-333333333333"

var purchaseScope = platform.Scope{
	TenantID:    "11111111-1111-4111-8111-111111111111",
	StoreID:     "22222222-2222-4222-8222-222222222222",
	PrincipalID: "44444444-4444-4444-8444-444444444444",
}

type purchaseRow struct {
	values []any
	err    error
}

func (r purchaseRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("wrong scan arity")
	}
	for i, value := range r.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

type purchaseTx struct {
	pgx.Tx
	rows  []purchaseRow
	query []string
	args  [][]any
}

func (t *purchaseTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.query = append(t.query, sql)
	t.args = append(t.args, args)
	row := t.rows[0]
	t.rows = t.rows[1:]
	return row
}

func TestReadPurchaseEntryStates(t *testing.T) {
	for _, tc := range []struct {
		name, domainState, origin, productState string
		hasSKU                                  bool
		wantState, wantURL                      string
	}{
		{"configured", "configured", "https://shop.example", "active", true, "configured", "https://shop.example/en/products/" + purchaseID},
		{"archived", "configured", "https://shop.example", "archived", false, "product_inactive", ""},
		{"missing sku", "configured", "https://shop.example", "active", false, "no_active_sku", ""},
		{"no domain", "storefront_unavailable", "", "active", true, "storefront_unavailable", ""},
		{"ambiguous", "domain_selection_required", "", "active", true, "domain_selection_required", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &purchaseTx{rows: []purchaseRow{{values: []any{tc.domainState, tc.origin}}, {values: []any{tc.productState, tc.hasSKU}}}}
			entry, err := ReadPurchaseEntry(context.Background(), tx, purchaseScope, "secret-token", purchaseID, "en")
			if err != nil || entry != (PurchaseEntry{purchaseID, "en", tc.wantState, tc.wantURL}) {
				t.Fatalf("entry %#v, err %v", entry, err)
			}
			if len(tx.query) != 2 {
				t.Fatalf("queries %d", len(tx.query))
			}
			hash := sha256.Sum256([]byte("secret-token"))
			if !reflect.DeepEqual(tx.args[0][0], hash[:]) || tx.args[0][1] != purchaseScope.StoreID {
				t.Fatal("origin reader did not receive hashed token and resolved store")
			}
		})
	}
}

func TestReadPurchaseEntryFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		origin purchaseRow
		want   error
	}{
		{"revoked", purchaseRow{err: &pgconn.PgError{Code: "PT401"}}, platform.ErrUnauthorized},
		{"permission", purchaseRow{err: &pgconn.PgError{Code: "PT403"}}, platform.ErrForbidden},
		{"store", purchaseRow{err: &pgconn.PgError{Code: "PT404"}}, platform.ErrScopeNotFound},
		{"non RC", purchaseRow{err: &pgconn.PgError{Code: "PT503"}}, ErrPurchaseEntryUnavailable},
		{"invalid input", purchaseRow{err: &pgconn.PgError{Code: "PT400"}}, command.ErrInvalid},
		{"driver secret", purchaseRow{err: errors.New("sensitive connection details")}, ErrPurchaseEntryUnavailable},
		{"unknown state", purchaseRow{values: []any{"other", ""}}, ErrPurchaseEntryUnavailable},
		{"unexpected origin", purchaseRow{values: []any{"storefront_unavailable", "https://shop.example"}}, ErrPurchaseEntryUnavailable},
		{"unsafe origin", purchaseRow{values: []any{"configured", "https://shop.example/path"}}, ErrPurchaseEntryUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &purchaseTx{rows: []purchaseRow{tc.origin}}
			_, err := ReadPurchaseEntry(context.Background(), tx, purchaseScope, "secret-token", purchaseID, "zh-TW")
			if !errors.Is(err, tc.want) || len(tx.query) != 1 {
				t.Fatalf("error %v, queries %d", err, len(tx.query))
			}
		})
	}
	for _, locale := range []string{"", "fr", "en/../x"} {
		tx := &purchaseTx{}
		if _, err := ReadPurchaseEntry(context.Background(), tx, purchaseScope, "secret-token", purchaseID, locale); !errors.Is(err, command.ErrInvalid) || len(tx.query) != 0 {
			t.Fatalf("accepted locale %q: %v", locale, err)
		}
	}
	tx := &purchaseTx{rows: []purchaseRow{{values: []any{"configured", "https://shop.example"}}, {err: pgx.ErrNoRows}}}
	if _, err := ReadPurchaseEntry(context.Background(), tx, purchaseScope, "secret-token", purchaseID, "en"); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("missing product: %v", err)
	}
}
