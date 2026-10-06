// Purpose: build the merchant inbox read/write service for the API process from the shared meta payload keyring
// (COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID + COMMERCE_META_PAYLOAD_KEYS_JSON, the same pair the consumer uses). The API
// process never holds the private page-token ring; this keyring only opens sealed message bodies (A9). A missing or
// invalid keyring degrades to unmounted with one fixed log line (never a secret value), so the inbox surface is off by
// default and never blocks API startup.
// Depends on: livecommerce/internal/inbox (LoadKeyring, NewService, EnableSend), pagetoken.LoadSealKeys (public ring), river.
// Used by: cmd/api/main.go (run), filling httpapi.Options.Inbox.

package main

import (
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/msgtemplates"
)

// errInboxConfig is every inbox build failure that is not the soft "keyring unavailable" degrade; it never carries a
// key, a path or an environment value.
var errInboxConfig = errors.New("inbox_invalid_config")

// newInbox returns nil, nil when the payload keyring is absent/invalid (the read side simply is not mounted, matching
// every other nil-able service in httpapi.Options). A nil getenv is a config error like the other new* builders.
func newInbox(getenv func(string) string) (*inbox.Service, error) {
	if getenv == nil {
		return nil, errInboxConfig
	}
	keys, err := inbox.LoadKeyring(getenv)
	if err != nil {
		slog.Warn("inbox_read_disabled", "reason", "payload_keyring_unavailable")
		return nil, nil
	}
	svc, err := inbox.NewService(keys)
	if err != nil {
		return nil, errInboxConfig
	}
	return svc, nil
}

// enableInboxSend turns the manual-send planners on (live-console-v1 §3.3/§4: A4/A5/A6/A12). It needs the Page HPKE PUBLIC ring that
// merchant connect already uses (COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON) to seal the dispatch copy and a River client to enqueue the
// operation job in the planning transaction. The API process never holds the private ring. A missing public ring degrades to "sends
// not mounted" with one fixed log line; a nil service or pool is a no-op.
func enableInboxSend(svc *inbox.Service, pool *pgxpool.Pool, getenv func(string) string) error {
	if svc == nil || pool == nil {
		return nil
	}
	if getenv == nil {
		return errInboxConfig
	}
	seal, err := pagetoken.LoadSealKeys(getenv)
	if err != nil {
		slog.Warn("inbox_send_disabled", "reason", "page_hpke_public_ring_unavailable")
		return nil
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return errInboxConfig
	}
	svc.EnableSend(seal, jobs, msgtemplates.NewService())
	return nil
}

// buildForBuyer builds the order-for-a-buyer service (live-console-v1 §5, A15/A16, unit LC-B6) on the manual-order pipeline and the inbox service
// (the pay-link DM planner). nil when the buyer surface is off (no pipeline): the routes then answer 503 manual_order_unavailable. A nil inbox
// service keeps orders working and makes every DM "not_sent: send_unavailable".
func buildForBuyer(manual *merchanttools.ManualOrders, svc *inbox.Service) (*merchanttools.ForBuyer, error) {
	if manual == nil {
		return nil, nil
	}
	var planner merchanttools.PayLinkPlanner
	if svc != nil {
		planner = svc // an untyped nil interface when the inbox is off, never a typed nil pointer
	}
	return merchanttools.NewForBuyer(manual, planner)
}
