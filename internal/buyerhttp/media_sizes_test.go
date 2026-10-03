package buyerhttp

import (
	"net/http/httptest"
	"testing"
)

// TestMediaSizesAdmission keeps malformed queries forbidden while admitting only the three product widths.
func TestMediaSizesAdmission(t *testing.T) {
	for _, query := range []string{"?w=360", "?w=720", "?w=1080"} {
		r := httptest.NewRequest("GET", "http://internal"+mediaPath+query, nil)
		if forbiddenInput(r) {
			t.Errorf("valid rendition rejected: %s", query)
		}
	}
	for _, query := range []string{"?", "?w=", "?w=0", "?w=640", "?w=0360", "?w=360.0", "?w=360&w=360", "?w=360&x=1", "?%77=360", "?w=%33%36%30"} {
		r := httptest.NewRequest("GET", "http://internal"+mediaPath+query, nil)
		if !forbiddenInput(r) {
			t.Errorf("invalid rendition admitted: %s", query)
		}
	}
}
