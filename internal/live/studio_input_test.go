package live

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeStudioInputStrictStatus(t *testing.T) {
	const closed = `{"attempt_id":"11111111-1111-4111-8111-111111111111","state":"CLOSED","admission_closed":true,"close_reason":"merchant_stop","cleanup_held":false,"can_stop":true,"updated_at":"2026-09-27T17:00:00Z"}`
	open := strings.Replace(closed, `"state":"CLOSED","admission_closed":true,"close_reason":"merchant_stop"`, `"state":"RESERVED","admission_closed":false,"close_reason":""`, 1)
	tests := []struct {
		name string
		raw  string
		bad  bool
	}{
		{"closed_with_egress_liability", closed, false},
		{"open", open, false},
		{"null_admission", strings.Replace(closed, `"admission_closed":true`, `"admission_closed":null`, 1), true},
		{"null_cleanup", strings.Replace(closed, `"cleanup_held":false`, `"cleanup_held":null`, 1), true},
		{"null_can_stop", strings.Replace(closed, `"can_stop":true`, `"can_stop":null`, 1), true},
		{"null_close_reason", strings.Replace(open, `"close_reason":""`, `"close_reason":null`, 1), true},
		{"closed_without_admission", strings.Replace(closed, `"admission_closed":true,"close_reason":"merchant_stop"`, `"admission_closed":false,"close_reason":""`, 1), true},
		{"open_with_admission", strings.Replace(closed, `"state":"CLOSED"`, `"state":"RESERVED"`, 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeStudioInput([]byte(tt.raw))
			if tt.bad {
				if got != nil || !errors.Is(err, ErrStudioProjection) {
					t.Fatalf("got (%+v, %v), want projection error", got, err)
				}
				return
			}
			if err != nil || got == nil || !got.CanStop {
				t.Fatalf("got (%+v, %v), want valid status", got, err)
			}
		})
	}
	if got, err := decodeStudioInput([]byte("null")); got != nil || err != nil {
		t.Fatalf("null input = (%+v, %v), want nil", got, err)
	}
}
