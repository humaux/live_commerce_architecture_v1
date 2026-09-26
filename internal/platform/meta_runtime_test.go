package platform

import (
	"context"
	"strings"
	"testing"
)

// The real role, same/different database and cleanup checks live in the
// independent foundation suite. These inputs must fail without any connection.
func TestMetaRuntimePoolInvalidInputs(t *testing.T) {
	for _, open := range []func(context.Context, string) error{
		func(ctx context.Context, dsn string) error { _, err := OpenMetaIngressPool(ctx, dsn); return err },
		func(ctx context.Context, dsn string) error { _, err := OpenMetaConsumerPool(ctx, dsn); return err },
	} {
		for _, dsn := range []string{"", "postgres://sentinel-secret@invalid%", strings.Repeat("s", 8193)} {
			if err := open(context.Background(), dsn); err == nil || strings.Contains(err.Error(), "sentinel-secret") {
				t.Fatal("invalid DSN accepted or disclosed")
			}
		}
		if open(nil, "postgres://sentinel-secret@invalid%") == nil {
			t.Fatal("nil context accepted")
		}
	}
	if ValidateSameDatabase(context.Background(), nil, nil) == nil || ValidateSameDatabase(nil, nil, nil) == nil {
		t.Fatal("nil database identity inputs accepted")
	}
}
