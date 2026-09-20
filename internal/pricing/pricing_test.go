package pricing

import (
	"errors"
	"testing"

	"livecommerce/internal/command"
)

func TestDeliveryMethod(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"home_tw", "cvs-711", "a"} {
		method, err := DeliveryMethod(code)
		if err != nil {
			t.Fatalf("DeliveryMethod(%q): %v", code, err)
		}
		if want := "delivery:" + code; method != want {
			t.Fatalf("DeliveryMethod(%q) = %q, want %q", code, method, want)
		}
		if !ValidMethod(method) {
			t.Fatalf("generated method %q was rejected", method)
		}
	}

	for _, code := range []string{"", "UPPER", "-bad", "contains:colon", "abcdefghijklmnopqrstuvwxyzabcdefghijklmno"} {
		if _, err := DeliveryMethod(code); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("DeliveryMethod(%q) error = %v, want ErrInvalid", code, err)
		}
	}
}

func TestValidMethodRemainsClosed(t *testing.T) {
	t.Parallel()

	for _, method := range []string{"home", "cvs_711", "cvs_familymart", "delivery:tw_home"} {
		if !ValidMethod(method) {
			t.Fatalf("ValidMethod(%q) = false", method)
		}
	}
	for _, method := range []string{"delivery:", "delivery:UPPER", "delivery:x:y", "carrier:tw_home", "pickup"} {
		if ValidMethod(method) {
			t.Fatalf("ValidMethod(%q) = true", method)
		}
	}
}
