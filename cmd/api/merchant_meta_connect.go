// merchant_meta_connect.go builds the merchant Facebook Page / Instagram connect service for the API process
// (contracts/meta-claims-intake-v1.md "Merchant connect (R4)"): the Login for Business dialog and code exchange for the existing
// Meta app (id + secret from the same COMMERCE_META_APPS_JSON entry the webhook verifier uses, so no second secret exists),
// the Page-token keyring that seals the user token between callback and pick and the Page token for good, and an insert-only
// River client (core.RegisterBinding needs one; no queue or worker starts here).
//
// Non-goals: no route (internal/httpapi/meta_connect.go), no rule (internal/metaconnect and migration 0095), no Graph call
// except through internal/metaconnect. This process now holds the Page-token keyring (seal + the disconnect-time open), which
// contract §7 had reserved for cmd/claims-worker: the price of self-serve sealing, recorded in the contract amendment.
// Variables (names only): COMMERCE_META_LOGIN_CONFIG_ID (empty = surface off, nothing else is read),
// COMMERCE_META_LOGIN_REDIRECT_URI, COMMERCE_META_LOGIN_GRAPH_VERSION, COMMERCE_META_APPS_JSON,
// COMMERCE_META_PAGE_TOKEN_KEYS_JSON, COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID.

package main

import (
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/metaconnect"
)

var errMetaConnectConfig = errors.New("meta_connect_invalid_config")

// metaApp is one COMMERCE_META_APPS_JSON entry (strictly validated by meta.LoadWebhookEndpoints at API start when the webhook is
// on; here only the three fields the connect needs are read).
type metaApp struct {
	AppID     string `json:"app_id"`
	Object    string `json:"object"`
	AppSecret string `json:"app_secret"`
}

// newMetaConnect returns nil, nil when COMMERCE_META_LOGIN_CONFIG_ID is unset. When set, every other value is required and any
// missing or invalid one is the one fixed errMetaConnectConfig (never a value, secret or path).
func newMetaConnect(pool *pgxpool.Pool, getenv func(string) string) (*metaconnect.Service, error) {
	if getenv == nil {
		return nil, errMetaConnectConfig
	}
	configID := getenv("COMMERCE_META_LOGIN_CONFIG_ID")
	if configID == "" {
		return nil, nil
	}
	if pool == nil {
		return nil, errMetaConnectConfig
	}
	var doc struct {
		Apps []metaApp `json:"apps"`
	}
	if json.Unmarshal([]byte(getenv("COMMERCE_META_APPS_JSON")), &doc) != nil {
		return nil, errMetaConnectConfig
	}
	var page, ig *metaApp
	for i := range doc.Apps {
		switch doc.Apps[i].Object {
		case "page":
			if page == nil {
				page = &doc.Apps[i]
			}
		case "instagram":
			if ig == nil {
				ig = &doc.Apps[i]
			}
		}
	}
	if page == nil || page.AppSecret == "" {
		return nil, errMetaConnectConfig
	}
	igApp := page.AppID // the instagram webhook object defaults to the same app
	if ig != nil {
		igApp = ig.AppID
	}
	secret := []byte(page.AppSecret)
	defer clear(secret)
	keys, err := metareply.LoadPageTokenKeyring(getenv)
	if err != nil {
		return nil, errMetaConnectConfig
	}
	// Graph base URL stays the fixed production host (empty = GraphHost): no env can redirect the code exchange, which carries the
	// app secret, to another host.
	graph, err := metaoauth.NewGraph("", getenv("COMMERCE_META_LOGIN_GRAPH_VERSION"), nil)
	if err != nil {
		return nil, errMetaConnectConfig
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, errMetaConnectConfig
	}
	svc, err := metaconnect.New(metaconnect.Config{
		Graph: graph, App: metaoauth.App{ID: page.AppID, RedirectURI: getenv("COMMERCE_META_LOGIN_REDIRECT_URI"), Secret: secret},
		ConfigID: configID, GraphVersion: getenv("COMMERCE_META_LOGIN_GRAPH_VERSION"), PageAppID: page.AppID, IGAppID: igApp,
		StateKey: metaconnect.StateKeyFor(secret), Keys: keys,
	}, jobs)
	if err != nil {
		return nil, errMetaConnectConfig
	}
	return svc, nil
}
