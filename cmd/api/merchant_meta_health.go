// Purpose: builds the meta connection-health banner read model for the API process (contract meta-connection-health-v1
//   §9) behind B1/B2 — the §7.4 reader swap, a TableReader (probed integration.binding_capabilities) over a
//   SnapshotReader fallback, so no caller signature changes.
// Depends on: internal/metaconnect (Health/TableReader/SnapshotReader/NewReaderConfig); the GUC-scoped definer
//   integration.meta_health_snapshot() (commerce_runtime has no meta_connections SELECT policy); env vars (names only)
//   COMMERCE_META_LOGIN_CONFIG_ID, COMMERCE_META_ADVANCED_ACCESS, COMMERCE_META_DM_RECEIVER_CONFIRMED (also read by the
//   claims-worker probe, which derives states with the same ReaderConfig).
// Used by: cmd/api (newMetaHealth; internal/httpapi/meta_health.go mounts only when the connect surface is on).
// Non-goals: no route (internal/httpapi/meta_health.go), no rule (internal/metaconnect.Health + 0125 definers), no Graph
//   call, and NO token custody — never import metareply/pagetoken/pageopen or name a Page-token opening key (guarded by
//   TestMetaConnectAPIHoldsNoPagePrivateKey).

package main

import "livecommerce/internal/metaconnect"

// newMetaHealth returns nil, nil when the meta-connect surface is off (COMMERCE_META_LOGIN_CONFIG_ID unset): the banner
// shares the connect snapshot, so there is nothing to show and the routes stay unmounted like meta-connect's own.
func newMetaHealth(getenv func(string) string) (*metaconnect.Health, error) {
	if getenv == nil {
		return nil, errMetaConnectConfig
	}
	if getenv("COMMERCE_META_LOGIN_CONFIG_ID") == "" {
		return nil, nil
	}
	cfg := metaconnect.NewReaderConfig(getenv("COMMERCE_META_ADVANCED_ACCESS"), getenv("COMMERCE_META_DM_RECEIVER_CONFIRMED") == "1")
	return &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{Config: cfg}}}, nil
}
