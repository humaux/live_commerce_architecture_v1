// Purpose: live-console-v1 §2.3/§2.4 (unit LC-B2) — the API-side comment read-through assembly: the
// bridge client (shared 32-byte bearer against the claims-worker's internal bridge listener) plus the
// optional Meta payload keyring for the Instagram webhook fallback (social.read_comment_events → the API
// decrypts, §2.4). This file must never import the page-token private keyring: the API only dials the
// bridge and decrypts webhook payloads; the claims-worker owns the Page token.
// Depends on: metabridge.NewBridgeClient (client.go), meta.LoadPayloadKeyring, live.NewCommentStream (stream.go).
// Variables: COMMERCE_CLAIMS_CONSOLE_BASE_URL (empty = console off), COMMERCE_CLAIMS_BRIDGE_TOKEN
// (std base64, 32 bytes, shared with claims-worker), optional COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID +
// COMMERCE_META_PAYLOAD_KEYS_JSON (the payload keyring; both-or-neither for the IG fallback).
// Used by: cmd/api main.go → httpapi.Options.CommentStream.
package main

import (
	"encoding/base64"
	"errors"

	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metabridge"
	"livecommerce/internal/live"
)

var errCommentStreamConfig = errors.New("comment_stream_invalid_config")

// buildCommentStream returns nil when COMMERCE_CLAIMS_CONSOLE_BASE_URL is unset (console off). When set,
// the base URL must be a valid bridge URL and the 32-byte token must decode; the payload keyring is
// optional and only loaded when at least one of its two variables is set (a half-set pair is an error).
func buildCommentStream(getenv func(string) string) (*live.CommentStream, error) {
	if getenv == nil {
		return nil, errCommentStreamConfig
	}
	baseURL := getenv("COMMERCE_CLAIMS_CONSOLE_BASE_URL")
	if baseURL == "" {
		return nil, nil
	}
	token, err := base64.StdEncoding.DecodeString(getenv("COMMERCE_CLAIMS_BRIDGE_TOKEN"))
	if err != nil || len(token) != 32 {
		return nil, errCommentStreamConfig
	}
	bridge, err := metabridge.NewBridgeClient(baseURL, token)
	if err != nil {
		return nil, errCommentStreamConfig
	}
	var keys *meta.PayloadKeyring
	if getenv("COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID") != "" || getenv("COMMERCE_META_PAYLOAD_KEYS_JSON") != "" {
		if keys, err = meta.LoadPayloadKeyring(getenv); err != nil {
			return nil, errCommentStreamConfig
		}
	}
	return live.NewCommentStream(bridge, keys)
}
