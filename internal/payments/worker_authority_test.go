package payments

import (
	"testing"

	"livecommerce/internal/platform"
)

// T21-03 startup half of the profile fence: LIVE is served only by commerce_payment_live, SANDBOX and PROVIDER_MOCK only by
// commerce_payment_worker, and anything else has no authority (platform refuses an empty one). The SQL half is proven in
// tests/foundation WAS05.
func TestWorkerAuthorityPerProfile(t *testing.T) {
	for profile, want := range map[string]platform.WorkerAuthority{
		"LIVE": platform.WorkerPaymentLive, "SANDBOX": platform.WorkerPayment, "PROVIDER_MOCK": platform.WorkerPayment,
		"": "", "live": "", "PRODUCTION": "",
	} {
		if got := WorkerAuthority(profile); got != want {
			t.Errorf("WorkerAuthority(%q)=%q want %q", profile, got, want)
		}
	}
}
