package merchantorders

// Purpose: DB-free pins of the W3-07B pure logic (parcels.go): group adjacency for the pick list / export and the SQL-message
//   to error mapping of the parcel definers.
// Depends on: pgconn.PgError fixtures, command, platform error sentinels.
// Used by: go test ./internal/merchantorders (check-gates).

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
)

func TestGroupAdjacent(t *testing.T) {
	rows := []pickListRow{{OrderID: "a"}, {OrderID: "b"}, {OrderID: "c"}, {OrderID: "d"}, {OrderID: "e"}}
	got := groupAdjacent(rows, map[string]string{"a": "G1", "c": "G1", "d": "G2", "e": "G2"})
	var order, groups string
	for _, r := range got {
		order += r.OrderID
		groups += "|" + r.ParcelGroupID
	}
	// G1 sits where a was (a, c), the ungrouped b keeps its place after it, G2 (d, e) stays together.
	if order != "acbde" || groups != "|G1|G1||G2|G2" {
		t.Fatalf("order=%s groups=%s", order, groups)
	}
	if plain := groupAdjacent(rows, map[string]string{}); len(plain) != 5 || plain[0].OrderID != "a" || plain[4].OrderID != "e" {
		t.Fatalf("no groups must keep the order: %v", plain)
	}
}

func TestMapParcelError(t *testing.T) {
	pg := func(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }
	var parcel *ParcelError
	for _, code := range []string{"cod_not_mergeable", "cvs_not_mergeable", "not_mergeable", "already_in_group", "owner_mismatch",
		"destination_mismatch", "group_not_open", "group_incomplete"} {
		if err := mapParcelError(pg("PT409", code)); !errors.As(err, &parcel) || parcel.Code != code {
			t.Errorf("PT409 %s -> %v", code, err)
		}
	}
	for _, tc := range []struct {
		err  error
		want error
	}{
		{pg("PT409", "version_changed"), ErrVersionChanged},
		{pg("PT409", "idempotency_conflict"), command.ErrConflict},
		{pg("PT422", "not_shippable"), ErrNotShippable},
		{pg("PT400", "x"), command.ErrInvalid},
		{pg("XX000", "boom"), ErrUnavailable},
	} {
		if got := mapParcelError(tc.err); !errors.Is(got, tc.want) {
			t.Errorf("%v -> %v, want %v", tc.err, got, tc.want)
		}
	}
	if err := mapParcelError(pg("23505", "dup")); !errors.Is(err, command.ErrConflict) {
		t.Errorf("23505 -> %v, want a generic conflict (never already_in_group)", err)
	}
}

// The open-groups read and the orders list share one mask rule: "—" or exactly one printable rune + "***"; anything else hides.
func TestNormalizeRecipientMask(t *testing.T) {
	for in, want := range map[string]string{
		"王***": "王***", "A***": "A***", "—": "—",
		"": "—", "***": "—", "王小***": "—", "王小明": "—", "王\x07***": "—", "王小明***": "—", "\xff***": "—",
	} {
		if got := normalizeRecipientMask(in); got != want {
			t.Errorf("normalizeRecipientMask(%q) = %q, want %q", in, got, want)
		}
	}
}
