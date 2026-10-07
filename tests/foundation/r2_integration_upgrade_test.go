package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"livecommerce/migrations"
)

// TestR2IntegrationUpgradeFromReleaseHead is the R2 integration gate for the merged migration set: the six R2
// lanes each proved their own migration on their own branch; this proves all of them together, in ledger
// (numeric) order, as an UPGRADE of the release-head schema (numbered <= 0066, post-River <= 0014, commit 10c43ad)
// and that the result is the same schema a fresh database gets.
//
// It never asserts any lane's objects (those gates stay with the lanes); it fails when an R2 migration depends on
// something a later-numbered file creates, or is not re-runnable/idempotent through migrations.Apply.
func TestR2IntegrationUpgradeFromReleaseHead(t *testing.T) {
	ctx := context.Background()
	var r2 []string
	numbered, _ := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	post, _ := filepath.Glob("../../migrations/post_river/[0-9][0-9][0-9][0-9]_*.sql")
	for _, path := range numbered {
		if base := filepath.Base(path); base >= "0070" {
			r2 = append(r2, base)
		}
	}
	for _, path := range post {
		if base := filepath.Base(path); base >= "0015" {
			r2 = append(r2, "post_river/"+base)
		}
	}
	sort.Strings(r2)
	// 0070..0089 without 0076 (never allocated) and 0084 (worker-authority-split, not merged yet) = 18, + 0093 storefront-integration = 19, + 0095 meta-connect, + 0100 meta-connect D1/D2 fix, + 0090..0092 and post-River 0015..0018 (R3/R4 lanes add files): a lane that drops or adds a file
	// must update this. 0081 storefront-publish has its own upgrade gate TestStorefrontPublishSPW02UpgradeAfter0080. 0097
	// (buyer-comms order locale) is the 29th file; 0101 (promotion/live-tools volatility + buyer-principal fence) adds one more file; 0100 (meta-connect D1/D2) one more; 0102 (cvs collected guard) one more; 0103 (live-claim checkout) one more; 0104 (authz: store-scoped lookup throttle + order-link idempotent replay + regenerate) one more; 0105 (live-price consumption ledger, R4S-01) one more; 0106 (store domains: handle + platform subdomain + merchant self-service domain) one more; 0107 (home-cod cash-on-delivery, R5) one more; post-River 0020 (home-cod begin_hold) one more; 0108 (meta-multi-page: up to 10 Pages per store) one more.
	// 0110 adds only the orders-v2 read projection and its scoped indexes.
	// 0111 adds product image renditions (S1 B); keep an exact migration-set count.
	// 0112 adds the lease-fenced public Meta refusal projection (Amendment 2).
	// 0113 adds the scoped attribution projection and live-audience read helpers (AT7).
	// 0109 (product-core: A6 inventory_tracked + max_per_order, image cap 12) and post-River 0021 (product-core begin_hold rebuilt on 0020) add two more.
	// 0115 (claims contains mode), 0117 (delivery-allocation backfill), 0118 (live session flow), 0119 (live-console inbox),
	// 0120 (kwc-v2), 0121 (msg templates) and 0122 (live lifecycle) add seven more files: 49 -> 56.
	// 0123 (LC-B2 live-console comments), 0124 (LC-B1 readonly-archived), 0125 (meta connection health) and 0126 (tracking
	// import) add four more: 56 -> 60. 0127 (LC-R1 retention of the four send actions) adds one more: 60 -> 61.
	// 0128 (LC-B4 sends + takeover) and 0130 (W3-02B pick list) add one each: 61 -> 63.
	// 0139 (W6-01B customer tags and notes) adds one more: 63 -> 64.
	// 0116 (claim-direct-checkout sold-out read, merged late into its free slot) adds one: 64 -> 65.
	// 0129 (LC-B6 order for a buyer) adds one more: 65 -> 66.
	// 0136 (W4-01B PAYUNi notify receiver) adds one more: 66 -> 67.
	// 0143 (OPS-01B platform operator suspend/resume + operator audit) adds one more: 67 -> 68.
	// 0137 + post-River 0022 (W4-S1 platform Stripe) add two more: 68 -> 70.
	// 0144 (W3-03B checkout reminders) adds one more: 70 -> 71.
	// 0148 (LC-B7 console read model: console_session_facts + read_live_console_sales) adds one more: 71 -> 72.
	// 0147 (W6-02B reports) adds one more: 72 -> 73.
	// 0149 (product-media-v2: image roles, option-value images) adds one more: 73 -> 74.
	// 0146 (W3-07B parcel groups) adds one more: 74 -> 75.
	// 0151 (W3-04B sold-out reply) adds one more: 75 -> 76.
	// 0152 (W5-02B customer import) adds one more: 76 -> 77.
	// 0153 (OPS-02B support grants) adds one more: 77 -> 78.
	// 0150 (W4-S2 platform settlement ledger) adds one more: 78 -> 79.
	// 0155 (W3-08B returns + merchant cancel) adds one more: 79 -> 80.
	// 0154 (W3-05B buyer blocklist) adds one more: 80 -> 81.
	// 0157 (perf-dashboard-todos: stats-proof order lookup in order_money_shippable/manual_shipment_eligible) adds one more: 81 -> 82.
	// 0156 (W5-03B historical order import) adds one more: 82 -> 83.
	// 0158 (live-price-keep-on-pause: claim lines of a paused offer keep the live price, functions only) adds one more: 83 -> 84.
	// 0159 (W6-05B operations ledger) adds one more: 84 -> 85.
	// 0160 (W6-06B Meta ad-account unbind + catalog feed URL) adds one more: 85 -> 86.
	// 0161 (PAY-RM1 drops the never-deployed W4-01B notify receiver; forward-only, no data) adds one: 86 -> 87.
	// 0163 (S2-OPEN-1 settlement-resolve: append-only resolutions of unmapped_source rows + the in-place close patch) adds one more: 87 -> 88.
	if len(r2) != 88 {
		t.Fatalf("R2 migration set = %d files %v, want 88", len(r2), r2)
	}

	upgraded := mciStartPG(t)
	mustExec(t, upgraded, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	for _, version := range r2 {
		body, err := os.ReadFile(filepath.Join("../../migrations", version))
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, upgraded, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, version, fmt.Sprintf("%x", sha256.Sum256(body)))
	}
	// The current migrations.Apply grants River privileges to the 0096 worker authorities before 0096 itself can run
	// (it is held back in this release-head ledger), so pre-create them exactly as 0096 does (idempotent).
	waPrecreateRoles(t, upgraded)
	if err := migrations.Apply(ctx, upgraded); err != nil {
		t.Fatalf("release-head schema (everything but the R2 files): %v", err)
	}
	if n := countRows(t, upgraded, `SELECT count(*) FROM pg_namespace WHERE nspname IN ('ads','billing','customers')`); n != 0 {
		t.Fatalf("release-head database already has %d R2 schemas", n)
	}
	mustExec(t, upgraded, `DELETE FROM public.lc_schema_migrations WHERE version=ANY($1)`, r2)
	for i := 0; i < 2; i++ { // the second Apply must be a no-op
		if err := migrations.Apply(ctx, upgraded); err != nil {
			t.Fatalf("R2 upgrade apply %d: %v", i+1, err)
		}
	}
	if n := countRows(t, upgraded, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=ANY($1)`, r2); n != len(r2) {
		t.Fatalf("ledger has %d of %d R2 files after the upgrade", n, len(r2))
	}

	fresh := mciStartPG(t)
	if err := migrations.Apply(ctx, fresh); err != nil {
		t.Fatalf("fresh apply: %v", err)
	}
	// ACL items are compared as sets: an upgrade grants in a different order than a fresh apply (post-River before the R2 files).
	// Same objects either way: relations+columns, functions (with body hash), policies, triggers, table/function ACLs, roles.
	const catalog = `SELECT x FROM (
		SELECT 'col '||c.oid::regclass::text||' '||a.attname||' '||format_type(a.atttypid,a.atttypmod)||' '||a.attnotnull::text||' '||coalesce((SELECT string_agg(i::text, ',' ORDER BY i::text) FROM unnest(c.relacl) i),'')||' '||coalesce((SELECT string_agg(i::text, ',' ORDER BY i::text) FROM unnest(a.attacl) i),'')||' '||c.relrowsecurity::text||c.relforcerowsecurity::text AS x
		  FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
		  WHERE n.nspname NOT IN ('pg_catalog','information_schema','pg_toast') AND c.relkind IN ('r','p','v','m')
		UNION ALL SELECT 'fn '||p.oid::regprocedure::text||' '||md5(p.prosrc)||' '||p.prosecdef::text||' '||pg_get_userbyid(p.proowner)||' '||coalesce((SELECT string_agg(i::text, ',' ORDER BY i::text) FROM unnest(p.proacl) i),'')||' '||coalesce(p.proconfig::text,'')
		  FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema')
		UNION ALL SELECT 'pol '||polrelid::regclass::text||' '||polname||' '||polcmd::text||' '||coalesce(pg_get_expr(polqual,polrelid),'')||' '||coalesce(pg_get_expr(polwithcheck,polrelid),'')||' '||polroles::regrole[]::text FROM pg_policy
		UNION ALL SELECT 'trg '||tgrelid::regclass::text||' '||tgname||' '||tgfoid::regprocedure::text||' '||tgtype::text||' '||tgenabled::text FROM pg_trigger WHERE NOT tgisinternal
		UNION ALL SELECT 'con '||conrelid::regclass::text||' '||conname||' '||pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid<>0
		UNION ALL SELECT 'idx '||indexrelid::regclass::text||' '||pg_get_indexdef(indexrelid) FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
		UNION ALL SELECT 'role '||rolname||' '||rolcanlogin::text||rolinherit::text||rolbypassrls::text FROM pg_roles WHERE rolname LIKE 'commerce\_%'
		UNION ALL SELECT 'member '||roleid::regrole::text||' '||member::regrole::text FROM pg_auth_members WHERE roleid::regrole::text LIKE 'commerce\_%'
		UNION ALL SELECT 'ns '||nspname||' '||coalesce((SELECT string_agg(i::text, ',' ORDER BY i::text) FROM unnest(nspacl) i),'') FROM pg_namespace WHERE nspname NOT LIKE 'pg\_%' AND nspname<>'information_schema'
	) s ORDER BY x`
	a, b := lcStrings(t, upgraded, catalog), lcStrings(t, fresh, catalog)
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	var diff []string
	for x, n := range seen {
		if n != 0 {
			diff = append(diff, fmt.Sprintf("%+d %s", n, x)) // +1 only after the upgrade, -1 only on fresh
		}
	}
	sort.Strings(diff)
	if len(diff) != 0 || len(a) < 1000 {
		t.Fatalf("upgraded catalog (%d rows) differs from fresh (%d rows) in %d rows; first: %.2000v", len(a), len(b), len(diff), diff[:min(len(diff), 12)])
	}
}
