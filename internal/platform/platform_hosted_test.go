package platform

import (
	"context"
	"testing"
)

func TestHostedPoolRequiresDedicatedLogin(t *testing.T) {
	if _, err := OpenHostedPool(context.Background(), ""); err == nil {
		t.Fatal("empty hosted DSN accepted")
	}
	if err := ValidateHostedPool(context.Background(), nil); err == nil {
		t.Fatal("nil hosted pool accepted")
	}
	if err := ValidateHostedPool(nil, nil); err == nil {
		t.Fatal("nil hosted context accepted")
	}
}
