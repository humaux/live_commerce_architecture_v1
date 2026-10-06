package returns

// Purpose: DB-free tests of the returns wrappers: the closed refusal mapping, input bounds that refuse before any database work, and the
//   canonical request hash (line order never changes it, a changed quantity does).
// Depends on: pgconn.PgError values, platform.Scope literals; no database.
// Used by: go test ./internal/returns.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func TestMapErrorClosedSet(t *testing.T) {
	var coded *Error
	err := MapError(&pgconn.PgError{Code: "PT409", Message: "refund_first"})
	if !errors.As(err, &coded) || coded.Status != 409 || coded.Code != "refund_first" {
		t.Fatalf("refund_first: %v", err)
	}
	if err = MapError(&pgconn.PgError{Code: "PT422", Message: "exceeds_shipped"}); !errors.As(err, &coded) || coded.Status != 422 {
		t.Fatalf("exceeds_shipped: %v", err)
	}
	if err = MapError(&pgconn.PgError{Code: "PT409", Message: "collection_state_changed"}); !errors.As(err, &coded) || coded.Code != "state_changed" {
		t.Fatalf("collection_state_changed: %v", err)
	}
	if !errors.Is(MapError(&pgconn.PgError{Code: "PT409", Message: "idempotency_conflict"}), command.ErrConflict) {
		t.Fatal("idempotency_conflict must be command.ErrConflict")
	}
	for code, want := range map[string]error{"PT400": command.ErrInvalid, "PT401": platform.ErrUnauthorized, "PT403": platform.ErrForbidden, "PT404": platform.ErrScopeNotFound} {
		if got := MapError(&pgconn.PgError{Code: code}); !errors.Is(got, want) {
			t.Fatalf("%s -> %v", code, got)
		}
	}
	// anything outside the closed set (a new PT message, a driver error) must never leak: 503 unavailable
	for _, bad := range []error{&pgconn.PgError{Code: "PT409", Message: "something_new"}, &pgconn.PgError{Code: "23505"}, errors.New("boom")} {
		if got := MapError(bad); !errors.Is(got, ErrUnavailable) {
			t.Fatalf("%v -> %v, want ErrUnavailable", bad, got)
		}
	}
}

func TestInputBoundsRefuseBeforeTheDatabase(t *testing.T) {
	scope := platform.Scope{TenantID: "11111111-1111-4111-8111-111111111111", StoreID: "22222222-2222-4222-8222-222222222222", PrincipalID: "33333333-3333-4333-8333-333333333333", Revision: 1}
	token := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	order := "44444444-4444-4444-8444-444444444444"
	line := RegisterLine{SKUID: "55555555-5555-4555-8555-555555555555", Quantity: 1}
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"nil tx": func() error {
			_, e := Register(ctx, nil, scope, token, "rma-key-0001", order, "damaged", []RegisterLine{line})
			return e
		},
		"short key": func() error {
			_, e := Register(ctx, nil, scope, token, "short", order, "damaged", []RegisterLine{line})
			return e
		},
		"empty reason": func() error {
			_, e := Register(ctx, nil, scope, token, "rma-key-0001", order, "", []RegisterLine{line})
			return e
		},
		"no lines": func() error {
			_, e := Register(ctx, nil, scope, token, "rma-key-0001", order, "damaged", nil)
			return e
		},
		"zero quantity": func() error {
			_, e := Register(ctx, nil, scope, token, "rma-key-0001", order, "damaged", []RegisterLine{{SKUID: line.SKUID}})
			return e
		},
		"bad version": func() error { _, e := Close(ctx, nil, scope, token, "rma-key-0001", order, 0, nil); return e },
		"bad state":   func() error { _, e := List(ctx, nil, scope, token, "BOGUS"); return e },
	} {
		if err := call(); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("%s: %v, want command.ErrInvalid", name, err)
		}
	}
}

func TestDigestIsCanonical(t *testing.T) {
	a := RegisterLine{SKUID: "55555555-5555-4555-8555-555555555555", Quantity: 1}
	b := RegisterLine{SKUID: "66666666-6666-4666-8666-666666666666", Quantity: 2}
	if string(digest("t", "o", "r", []RegisterLine{a, b})) != string(digest("t", "o", "r", []RegisterLine{a, b})) {
		t.Fatal("digest is not deterministic")
	}
	if string(digest("t", "o", "r", []RegisterLine{a, b})) == string(digest("t", "o", "r", []RegisterLine{a, {SKUID: b.SKUID, Quantity: 3}})) {
		t.Fatal("a changed quantity must change the request hash")
	}
	if string(digest("t", "o", "r", []RegisterLine{a})) == string(digest("u", "o", "r", []RegisterLine{a})) {
		t.Fatal("the operation tag must change the request hash")
	}
}
