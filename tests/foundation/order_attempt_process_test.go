// Purpose: REAL_PG process acceptance of the frozen order-attempt pool boundary.
// Depends on: the packaged API, actual login authorities and two isolated clusters.
// Used by: OA1 CI before the versioned placement API is activated by OA3.
// Invariants: identical names/roles never prove one database; every API participant
// shares the main transaction database or startup refuses before listening.
package foundation_test

import (
	"context"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOrderAttemptProcessAPIRejectsSplitBuyerPools(t *testing.T) {
	f, clone := mrFixture(t), mrFixture(t)
	var primaryName, cloneName string
	if err := f.owner.QueryRow(context.Background(), `SELECT current_database()`).Scan(&primaryName); err != nil {
		t.Fatal(err)
	}
	if err := clone.owner.QueryRow(context.Background(), `SELECT current_database()`).Scan(&cloneName); err != nil || primaryName != cloneName {
		t.Fatal("negative fixture must use the same database name on another cluster", err)
	}
	main := f.runtime.Config().ConnString()
	names := []string{"COMMERCE_BUYER_ISSUER_DATABASE_URL", "COMMERCE_BUYER_DATABASE_URL", "COMMERCE_CHECKOUT_DATABASE_URL"}
	roles := []string{"commerce_buyer_issuer", "commerce_buyer_runtime", "commerce_checkout_runtime"}
	local, other := make([]string, len(roles)), make([]string, len(roles))
	for i, role := range roles {
		local[i], other[i] = miRole(t, f, role), miRole(t, clone, role)
	}
	binary := mrBuild(t, "../../cmd/api", "oa-api-pool-boundary")
	bff := strings.Repeat("A", 43) // Synthetic private-listener credential only.
	environment := func(address string, split int) []string {
		env := []string{
			"DATABASE_URL=" + main, "LISTEN_ADDR=" + address,
			"COMMERCE_BUYER_ENABLED=1", "COMMERCE_BUYER_BFF_KEY=" + bff,
			"COMMERCE_BUYER_SESSION_TTL=1h", "COMMERCE_BUYER_PAYMENT_ENABLED=0",
			"COMMERCE_IDENTITY_ENABLED=0", "COMMERCE_META_WEBHOOK_ENABLED=0",
		}
		for i, name := range names {
			dsn := local[i]
			if i == split {
				dsn = other[i]
			}
			env = append(env, name+"="+dsn)
		}
		return env
	}
	// Positive readiness proves every config/authority other than the one changed
	// database is valid; a generic startup error cannot satisfy the negative cases.
	t.Run("same-physical-database", func(t *testing.T) {
		address := "127.0.0.1:" + slxFreePort(t)
		p := mrLaunch(t, binary, "oa-api-same-db", environment(address, -1))
		lrHTTPReady(t, p, address)
		mrLogNoSecrets(t, p, bff, main, local[0], local[1], local[2])
		mrStop(t, p, syscall.SIGTERM, true)
	})
	for i, name := range names {
		t.Run(name, func(t *testing.T) {
			address := "127.0.0.1:" + slxFreePort(t)
			p := mrLaunch(t, binary, "oa-api-split-db", environment(address, i))
			mrFailsBeforeReady(t, p, "api stopped", bff, main, local[0], local[1], local[2], other[i])
			client := &http.Client{Timeout: time.Second}
			if response, err := client.Get("http://" + address + "/healthz"); err == nil {
				_ = response.Body.Close()
				t.Fatal("split-database API left a listener after failed admission")
			}
		})
	}
}
