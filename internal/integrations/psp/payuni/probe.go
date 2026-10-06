// Purpose: read-only credential probe for a PAYUNi connection: one signed trade query for a MerTradeNo that must not exist.
// Depends on: client.go (queryInner: the same signed /api/trade/query transport as Query); PAYUNi host sandbox-api/api.payuni.com.tw.
// Used by: internal/payments (payuni_activation.go LiveProbe) for the LIVE activation gate; probe_test.go.
// Invariants: starts no transaction and creates no hosted page; proof = the response authenticates with the connection's own
//   HashKey/HashIV (HashInfo + AES) and reports no trade; any other outcome is NOT proof and is never retried here.
// Status: wire shape of the "no such trade" reply is NOT_VERIFIED against official docs (docs.payuni.com.tw is a JS SPA,
//   fetched 2026-10-06 returned no content); the probe fails closed on anything but a signed reply without a trade row.

package payuni

import (
	"context"
	"errors"
	"strings"
)

// ErrProbeTradeFound is returned when the "must not exist" probe trade came back as a real trade: the probe proves nothing.
var ErrProbeTradeFound = errors.New("payuni: probe trade exists")

// Probe sends one signed query for merTradeNo (which the caller generated to be unique and which has never been used to
// create a trade) and returns nil only when the reply authenticated with this client's HashKey/HashIV, i.e. MerID, Key and IV
// are the ones PAYUNi holds, and carries no trade row. Errors: ErrTransport/ctx errors (UNKNOWN, do not auto-retry),
// ErrAuthentication/ErrMismatch/ErrProtocol/ErrUncertain (reply not authenticated or not understood), ErrProbeTradeFound.
// It performs one outbound HTTPS call to PAYUNi's query endpoint and nothing else.
func (c *Client) Probe(ctx context.Context, merTradeNo string, timestamp int64) error {
	if c == nil || c.notifyOnly || ctx == nil || !tradePattern.MatchString(merTradeNo) || !validTimestamp(timestamp) {
		return ErrInvalid
	}
	_, inner, err := c.queryInner(ctx, merTradeNo, "", timestamp)
	if err != nil {
		return err
	}
	// Authenticated. A reply that carries a trade row means the id was not unused; anything without one is "no trade".
	for key := range inner {
		if strings.HasPrefix(key, "Result") {
			return ErrProbeTradeFound
		}
	}
	if inner["Status"] == "SUCCESS" {
		// SUCCESS without a trade row is not the documented "no trade" shape: do not certify it.
		return ErrUncertain
	}
	return nil
}
