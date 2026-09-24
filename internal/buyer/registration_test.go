package buyer

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestRegisterForTrustedStoreRejectsInvalidInputBeforeDatabase(t *testing.T) {
	const storeID = "00000000-0000-0000-0000-000000000001"
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	if !validToken(token) {
		t.Fatal("canonical 32-byte token rejected")
	}
	for _, value := range []string{"", token + "=", strings.Repeat("a", 43), "merchant-token"} {
		if validToken(value) {
			t.Fatalf("noncanonical token accepted: %q", value)
		}
	}
	for _, tc := range []struct {
		name    string
		service *Service
		storeID string
		token   string
	}{
		{"nil service", nil, storeID, token},
		{"missing issuer", &Service{ttlSeconds: 60}, storeID, token},
		{"bad store", &Service{ttlSeconds: 60}, "not-a-uuid", token},
		{"bad token", &Service{ttlSeconds: 60}, storeID, "not-a-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.service.RegisterForTrustedStore(context.Background(), tc.storeID, tc.token)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want invalid input", err)
			}
		})
	}
}
