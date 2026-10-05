// Purpose: build the merchant inbox read/write service for the API process from the shared meta payload keyring
// (COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID + COMMERCE_META_PAYLOAD_KEYS_JSON, the same pair the consumer uses). The API
// process never holds the private page-token ring; this keyring only opens sealed message bodies (A9). A missing or
// invalid keyring degrades to unmounted with one fixed log line (never a secret value), so the inbox surface is off by
// default and never blocks API startup.
// Depends on: livecommerce/internal/inbox (LoadKeyring, NewService).
// Used by: cmd/api/main.go (run), filling httpapi.Options.Inbox.

package main

import (
	"errors"
	"log/slog"

	"livecommerce/internal/inbox"
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
