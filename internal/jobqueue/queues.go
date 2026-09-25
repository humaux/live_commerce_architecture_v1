package jobqueue

const (
	PaymentMock    = "payment_mock_v1"
	PaymentSandbox = "payment_sandbox_v1"
	PaymentLive    = "payment_live_v1"
)

// ForProfile chooses a server-owned queue. An unknown profile has no queue.
func ForProfile(profile string) string {
	switch profile {
	case "PROVIDER_MOCK":
		return PaymentMock
	case "SANDBOX":
		return PaymentSandbox
	case "LIVE":
		return PaymentLive
	default:
		return ""
	}
}
