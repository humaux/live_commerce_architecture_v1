package buyerhttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
	"livecommerce/internal/promotions"
)

// storefront-v2 §F: a discount-code refusal from the quote request or BeginCheckout is answered with its own 422 code (never the generic
// retryable 503), through the one cvsHTTPError every route passes, and every promo_* code is in the httperror table (an unknown code would be
// rewritten to "internal").
func TestPromotionRefusalIsCodedNonRetryable422(t *testing.T) {
	for _, code := range []string{"promo_invalid", "promo_not_started", "promo_expired", "promo_min_subtotal", "promo_used_up", "promo_buyer_limit", "promo_changed"} {
		err := cvsHTTPError(fmt.Errorf("wrapped: %w", &promotions.Coded{Status: http.StatusUnprocessableEntity, Code: code}))
		status, got := classify(err)
		if status != http.StatusUnprocessableEntity || got != code {
			t.Fatalf("%s: classified %d %s", code, status, got)
		}
		rec := httptest.NewRecorder()
		httperror.Write(rec, status, got)
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) || !strings.Contains(rec.Body.String(), `"retryable":false`) || strings.Contains(rec.Body.String(), `"internal"`) {
			t.Fatalf("%s: body %s", code, rec.Body.String())
		}
	}
}
