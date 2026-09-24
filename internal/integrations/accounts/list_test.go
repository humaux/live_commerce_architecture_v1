package accounts

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/integrations/core"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

type listRow struct{ values []any }

func (r listRow) Scan(dest ...any) error {
	for i := range dest {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(r.values[i]))
	}
	return nil
}

type listRows struct {
	pgx.Rows
	items  [][]any
	index  int
	closed bool
}

func (r *listRows) Next() bool {
	if r.index >= len(r.items) {
		return false
	}
	r.index++
	return true
}
func (r *listRows) Scan(dest ...any) error { return listRow{r.items[r.index-1]}.Scan(dest...) }
func (r *listRows) Close()                 { r.closed = true }
func (r *listRows) Err() error             { return nil }

type listTx struct {
	pgx.Tx
	scope       platform.Scope
	rows        *listRows
	query       string
	args        []any
	authChecks  int
	revokeAfter bool
	authClosed  bool
}

func (tx *listTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.Contains(sql, "current_setting") {
		return listRow{[]any{tx.scope.TenantID, tx.scope.StoreID, tx.scope.PrincipalID}}
	}
	tx.authChecks++
	if tx.authChecks == 2 {
		tx.authClosed = tx.rows.closed
	}
	status := "ok"
	if tx.revokeAfter && tx.authChecks == 2 {
		status = "forbidden"
	}
	return listRow{[]any{status, tx.scope.TenantID, tx.scope.PrincipalID, tx.scope.Revision}}
}

func (tx *listTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.query, tx.args = sql, args
	return tx.rows, nil
}

func TestListScopedKeysetAndAfterWaitRevocation(t *testing.T) {
	scope := platform.Scope{
		TenantID: "11111111-1111-4111-8111-111111111111", StoreID: "22222222-2222-4222-8222-222222222222",
		PrincipalID: "33333333-3333-4333-8333-333333333333", Revision: 1,
	}
	now := time.Now()
	row := func(id string) []any {
		return []any{id, "payuni", "SANDBOX", "merchant", "44444444-4444-4444-8444-444444444444",
			int64(1), false, int64(1), now, now}
	}
	makeTx := func(revoke bool) *listTx {
		return &listTx{scope: scope, revokeAfter: revoke, rows: &listRows{items: [][]any{
			row("55555555-5555-4555-8555-555555555555"), row("66666666-6666-4666-8666-666666666666"),
		}}}
	}
	service := &Service{keys: &Keyring{}, bindings: &core.Service{}}
	token := strings.Repeat("t", 32)
	tx := makeTx(false)
	page, err := service.List(context.Background(), tx, scope, token, pagination.Request{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" || !tx.rows.closed || !tx.authClosed || tx.authChecks != 2 {
		t.Fatalf("list page=%+v err=%v close=%v checks=%d", page, err, tx.rows.closed, tx.authChecks)
	}
	if !strings.Contains(tx.query, "a.tenant_id=$1 AND a.store_id=$2") || !strings.Contains(tx.query, "ORDER BY a.id LIMIT $3") || strings.Contains(tx.query, "nonce") || strings.Contains(tx.query, "ciphertext") || strings.Contains(tx.query, "key_id") || tx.args[2] != 2 {
		t.Fatalf("unsafe query %q args=%v", tx.query, tx.args)
	}
	if page.Items[0].State != configuredUnverified {
		t.Fatalf("unsafe state %+v", page.Items[0])
	}

	tx = makeTx(true)
	page, err = service.List(context.Background(), tx, scope, token, pagination.Request{Limit: 1})
	if !errors.Is(err, platform.ErrForbidden) || len(page.Items) != 0 || page.NextCursor != "" || !tx.rows.closed || !tx.authClosed {
		t.Fatalf("revoked returned page=%+v err=%v", page, err)
	}

	tx = makeTx(false)
	otherStore := pagination.Binding{TenantID: scope.TenantID, StoreID: "77777777-7777-4777-8777-777777777777", Collection: "provider-accounts"}
	foreignCursor, err := pagination.Encode(otherStore, []string{"55555555-5555-4555-8555-555555555555"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(context.Background(), tx, scope, token, pagination.Request{Cursor: foreignCursor}); err == nil || tx.query != "" {
		t.Fatalf("foreign cursor queried data: err=%v query=%q", err, tx.query)
	}
}
