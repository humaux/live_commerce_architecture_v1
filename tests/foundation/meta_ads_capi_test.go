package foundation_test

// MA03 / MA08 (contracts/meta-ads-v1.md §9 rows MA03, MA08; AD8, AD10, §3 CAPI bullet, §4.3, §6.1 CAPI Check, §6.4 sweeper, §7 feed;
// F14-F17; customers-billing-v1 CD5 and ads-capi C2..C7). Tier MOCK + REAL_PG: the CAPTURED fact comes from the real payments
// capture path (payments.apply_capture over a recorded observation), consent through the real buyer HTTP handler
// (buyerhttp -> customers.BuyerSetConsent -> attribution.PutCAPIContext), the operation from the real 5-minute sweeper, and the
// event through the real dispatcher + capiroute + metaads.Client against the fake Graph.
// Disclosed owner-pool fixtures: the attempt's execution_profile/environment before the observation is recorded (the fixture
// order is a PROVIDER_MOCK/SANDBOX attempt; SANDBOX and LIVE profiles need the flag), aged fact time (replica mode), erased-owner
// flip through the buyer erasure route (real), grant revocation, SET LOCAL ROLE probes.
//
// Contract vs CD5 (recorded, not hidden): §6.4 says user_data carries `ph` when normalizable; customers-billing CD5 (frozen, "to
// mirror in meta-ads") says the destination phone may be used only when checkout records recipient = buyer, else CAPI omits `ph`.
// Nothing records that, so this gate asserts `ph` is ABSENT and that neither the raw nor the hashed phone reaches PG (outside
// storefront.destination_snapshots), logs or the wire.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
	"livecommerce/tests/ads/fakegraph"
)

type capiEnv struct {
	*adsEnv
	p      pqFixture
	bh     bhHarness
	ua     string
	origin string
}

// newCapiEnv builds the ads surface on the payment fixture's private store (one paid-order candidate, owner O, attempt A).
func newCapiEnv(t *testing.T, o adsOpts) *capiEnv {
	t.Helper()
	p := pqSetupItems(t, false, 1)
	o.fx = p.f
	o.dataset = true
	e := newAdsEnv(t, o)
	c := &capiEnv{adsEnv: e, p: p, ua: "Mozilla/5.0 (synthetic-ads-tests) SENTINEL-UA-" + t04Tag(), origin: "https://capi-" + t04Tag() + ".example.test"}
	bhPublish(t, p.bcHarness, c.origin, p.f.tenantA, p.f.storeA1)
	key := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	handler, err := buyerhttp.New(context.Background(), p.a.issuer, p.a.runtime, p.bcHarness.service, key, time.Hour)
	if err != nil {
		t.Fatalf("buyerhttp.New: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c.bh = bhHarness{bcHarness: p.bcHarness, key: key, origin: c.origin, server: srv}
	return c
}

// setCapi enables/disables CAPI through the merchant API (ads.set_capi).
func (c *capiEnv) setCapi(enabled bool) adsResp {
	body := map[string]any{"enabled": enabled}
	if enabled {
		body["dataset_binding_id"] = c.dsBinding
		if !c.opts.live {
			body["test_event_code"] = "TEST12345"
		}
	}
	return c.api("PUT", "/capi", c.token, adsKey(), body)
}

// consent PUTs the buyer's ads_personalization consent (the real route, with the browser user agent).
func (c *capiEnv) consent(granted bool) bhResponse {
	c.t.Helper()
	body := map[string]any{"purpose": "ads_personalization", "channel": "meta_ads", "granted": granted, "context": "settings"}
	return c.bh.request(c.t, "PUT", "/v1/buyer/consents", c.p.cap.Token, t04Key("capi-consent"), body, func(r *http.Request) { r.Header.Set("User-Agent", c.ua) })
}

// capture runs the REAL capture path for the fixture order: profile/environment set on the attempt (owner, disclosed), one claimed
// query observation, payments.apply_capture. It returns the fact row's environment/profile.
func (c *capiEnv) capture(profile, environment string) {
	c.t.Helper()
	q := c.p
	mustExec(c.t, q.f.owner, `UPDATE checkout.payment_attempts SET execution_profile=$2,environment=$3 WHERE id=$1`, q.result.AttemptID, profile, environment)
	claim := q.claim(c.t)
	report := pcFull(q)
	if err := pqRecord(q.worker, q.result.OperationID, claim, profile, report); err != nil {
		c.t.Fatalf("record observation (%s/%s): %v", profile, environment, err)
	}
	hash := pcHash(c.t, q, report)
	if err := pcApply(q.worker, q.result.AttemptID, hash); err != nil {
		c.t.Fatalf("payments.apply_capture: %v", err)
	}
	var kind, env, prof string
	if err := q.f.owner.QueryRow(c.ctx, `SELECT kind,environment,execution_profile FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'`, q.result.AttemptID).Scan(&kind, &env, &prof); err != nil {
		c.t.Fatalf("no CAPTURED fact after apply_capture: %v", err)
	}
	if env != environment || prof != profile {
		c.t.Fatalf("fact %s/%s, want %s/%s", env, prof, environment, profile)
	}
}

func (c *capiEnv) capiOps() []adsOp {
	c.t.Helper()
	rows, err := c.f.owner.Query(c.ctx, `SELECT o.id::text,'capi',o.action,o.state,o.result_code,o.provider_reference,coalesce(j.state::text,''),coalesce(j.queue,''),1,1,coalesce(j.priority,0),o.job_id,o.generation
		FROM ads.capi_events c JOIN integration.operations o ON o.id=c.operation_id LEFT JOIN river.river_job j ON j.id=o.job_id WHERE c.store_id=$1 ORDER BY c.planned_at`, c.store)
	if err != nil {
		c.t.Fatal(err)
	}
	defer rows.Close()
	var out []adsOp
	for rows.Next() {
		var o adsOp
		if err := rows.Scan(&o.ID, &o.Kind, &o.Action, &o.State, &o.Code, &o.Ref, &o.JobState, &o.Queue, &o.Seq, &o.Attempt, &o.Priority, &o.Job, &o.Generation); err != nil {
			c.t.Fatal(err)
		}
		out = append(out, o)
	}
	return out
}

func (c *capiEnv) contextRows() int64 {
	return c.count(`SELECT count(*) FROM ads.capi_contexts WHERE store_id=$1`, c.store)
}

// events are the POST /{pixel}/events requests the fake received.
func (c *capiEnv) events() []fakegraph.Req {
	var out []fakegraph.Req
	for _, r := range c.g.Requests() {
		if r.Route == fakegraph.RouteEvents {
			out = append(out, r)
		}
	}
	return out
}

// primed = capture + consent + CAPI on, ready for the sweeper.
func (c *capiEnv) prime(profile, environment string) {
	c.t.Helper()
	if r := c.setCapi(true); r.Status != 200 {
		c.t.Fatalf("PUT capi: %d %s", r.Status, r.Raw)
	}
	c.capture(profile, environment)
	if r := c.consent(true); r.status != 200 {
		c.t.Fatalf("buyer consent: %d %s", r.status, r.body)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MA03
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaAdsMA03Consent(t *testing.T) {
	t.Run("no consent: no op, no row, no event", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		if r := c.setCapi(true); r.Status != 200 {
			t.Fatalf("PUT capi %d %s", r.Status, r.Raw)
		}
		c.capture("SANDBOX", "SANDBOX")
		c.sweep("capi")
		c.settle()
		if n := len(c.capiOps()); n != 0 || len(c.events()) != 0 || c.contextRows() != 0 {
			t.Fatalf("ops=%d events=%d contexts=%d without consent (G09/I07)", n, len(c.events()), c.contextRows())
		}
	})

	t.Run("granted: exactly one op per attempt, stable event_id and key, merchant actor + capi_enabled_by, queue ads, one event", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		for i := 0; i < 3; i++ {
			c.sweep("capi")
			c.settle()
		}
		ops := c.capiOps()
		if len(ops) != 1 || ops[0].State != "SUCCEEDED" {
			t.Fatalf("capi ops %+v%s", ops, c.dump())
		}
		var key, provider, purpose, actor, principal, eventID, reqJSON string
		if err := c.f.owner.QueryRow(c.ctx, `SELECT o.semantic_key,o.provider,o.purpose,o.actor_kind,o.principal_id::text,e.event_id,o.request::text
			FROM ads.capi_events e JOIN integration.operations o ON o.id=e.operation_id WHERE e.store_id=$1`, c.store).Scan(&key, &provider, &purpose, &actor, &principal, &eventID, &reqJSON); err != nil {
			t.Fatal(err)
		}
		attempt := c.p.result.AttemptID
		if key != "ads:capi:"+attempt || provider != "meta_dataset" || purpose != "marketing" || actor != "MERCHANT" || principal != c.creator || eventID != "lc-purchase-"+attempt {
			t.Errorf("op shape: key=%s provider=%s purpose=%s actor=%s principal=%s event_id=%s", key, provider, purpose, actor, principal, eventID)
		}
		if ops[0].Queue != "ads" || ops[0].Priority != 3 || ops[0].Action != "meta.capi.purchase" {
			t.Errorf("queue/priority/action: %s/%d/%s", ops[0].Queue, ops[0].Priority, ops[0].Action)
		}
		var req map[string]any
		_ = json.Unmarshal([]byte(reqJSON), &req)
		for k := range req {
			if k != "v" && k != "attempt_id" && k != "event_id" && k != "event_time" && k != "test_event_code" {
				t.Errorf("frozen request carries key %q (no PII, no tokens: §6.1)", k)
			}
		}
		if strings.Contains(reqJSON, c.ua) || strings.Contains(reqJSON, "@") {
			t.Errorf("frozen request carries user data: %s", reqJSON)
		}
		if n := len(c.events()); n != 1 {
			t.Fatalf("%d events sent for one attempt", n)
		}
		if n := c.count(`SELECT count(*) FROM ads.capi_events WHERE store_id=$1`, c.store); n != 1 {
			t.Errorf("%d capi_events rows (one per attempt, ever)", n)
		}
	})

	t.Run("consent withdrawn before the sweeper: no op and the context row is deleted", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		if c.contextRows() != 1 {
			t.Fatal("no context after grant")
		}
		if r := c.consent(false); r.status != 200 {
			t.Fatalf("withdraw: %d %s", r.status, r.body)
		}
		c.sweep("capi")
		c.settle()
		if len(c.capiOps()) != 0 || len(c.events()) != 0 {
			t.Fatalf("withdrawn consent produced ops=%d events=%d", len(c.capiOps()), len(c.events()))
		}
		if c.contextRows() != 0 {
			t.Fatal("the sweeper did not delete the context row of a withdrawn consent (§4.3)")
		}
	})

	t.Run("owner erased: no op and the context row is deleted", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		r := c.bh.request(t, "POST", "/v1/buyer/privacy/erasure", c.p.cap.Token, t04Key("capi-erase"), map[string]any{"confirm": "ERASE"}, nil)
		if r.status != 200 {
			t.Fatalf("erasure: %d %s", r.status, r.body)
		}
		c.sweep("capi")
		c.settle()
		if len(c.capiOps()) != 0 || len(c.events()) != 0 || c.contextRows() != 0 {
			t.Fatalf("erased owner: ops=%d events=%d contexts=%d", len(c.capiOps()), len(c.events()), c.contextRows())
		}
	})

	t.Run("withdraw after planning: Check answers BLOCKED_POLICY consent_withdrawn with zero HTTP (G09)", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		c.pauseDispatch()
		c.sweep("capi")
		ops := c.capiOps()
		if len(ops) != 1 || ops[0].State != "READY" {
			t.Fatalf("expected one READY CAPI op: %+v", ops)
		}
		if r := c.consent(false); r.status != 200 {
			t.Fatalf("withdraw %d", r.status)
		}
		mark := c.g.Mark()
		c.resumeDispatch()
		c.settle()
		c.wantBlocked(c.capiOps()[0], "consent_withdrawn")
		if n := len(c.g.RequestsSince(mark)); n != 0 {
			t.Fatalf("%d Graph requests after the withdrawal", n)
		}
		if len(c.events()) != 0 {
			t.Fatal("an event was sent after the withdrawal")
		}
	})

	t.Run("fact environment must equal the store environment and PROVIDER_MOCK facts are never sent", func(t *testing.T) {
		live := newCapiEnv(t, adsOpts{live: true})
		live.prime("SANDBOX", "SANDBOX") // a SANDBOX fact in a LIVE store
		live.sweep("capi")
		live.settle()
		if len(live.capiOps()) != 0 || len(live.events()) != 0 {
			t.Fatalf("SANDBOX fact in a LIVE store produced ops=%d events=%d (F17-ish: §6.4 environment filter)", len(live.capiOps()), len(live.events()))
		}
	})
	t.Run("PROVIDER_MOCK fact in a matching environment is skipped", func(t *testing.T) {
		mock := newCapiEnv(t, adsOpts{})
		mock.prime("PROVIDER_MOCK", "SANDBOX")
		mock.sweep("capi")
		mock.settle()
		if len(mock.capiOps()) != 0 || len(mock.events()) != 0 {
			t.Fatalf("PROVIDER_MOCK fact produced ops=%d events=%d (§6.4: execution_profile <> PROVIDER_MOCK)", len(mock.capiOps()), len(mock.events()))
		}
	})
	// NOT_RUN: a LIVE-profile captured fact (the "no test_event_code in LIVE" half of AD9) needs a LIVE PSP qualification the
	// disposable fixture cannot mint (payment_attempts FK to a LIVE qualification); fabricating a LIVE facts row is forbidden.
	// The SANDBOX half (`test_event_code` present) is asserted in MA08.

	checkDenial := func(name, code string, mutate func(c *capiEnv), zeroHTTP bool, states ...string) {
		t.Run(name, func(t *testing.T) {
			c := newCapiEnv(t, adsOpts{})
			c.prime("SANDBOX", "SANDBOX")
			c.pauseDispatch()
			c.sweep("capi")
			if ops := c.capiOps(); len(ops) != 1 || ops[0].State != "READY" {
				t.Fatalf("expected one READY CAPI op: %+v", ops)
			}
			mutate(c)
			mark := c.g.Mark()
			c.resumeDispatch()
			c.settle()
			o := c.capiOps()[0]
			if len(states) > 0 {
				ok := false
				for _, s := range states {
					ok = ok || o.State == s
				}
				if !ok {
					t.Fatalf("op %s/%s, want one of %v", o.State, o.Code, states)
				}
			} else {
				c.wantBlocked(o, code)
			}
			if zeroHTTP && len(c.g.RequestsSince(mark)) != 0 {
				t.Fatalf("%d Graph requests for a denied CAPI op", len(c.g.RequestsSince(mark)))
			}
		})
	}
	checkDenial("capi_enabled_by lost ads:manage -> capi_principal_revoked", "capi_principal_revoked", func(c *capiEnv) { c.revoke(c.creator, "ads:manage") }, true)
	checkDenial("CAPI switched off after planning -> capi_disabled", "capi_disabled", func(c *capiEnv) {
		if r := c.api("PUT", "/capi", c.token, adsKey(), map[string]any{"enabled": false}); r.Status != 200 {
			c.t.Fatalf("disable: %d %s", r.Status, r.Raw)
		}
	}, true)
	checkDenial("op that sat READY for 7 days (frozen event_time aged, hash kept consistent) -> event_too_old", "event_too_old", func(c *capiEnv) {
		old := time.Now().Add(-7 * 24 * time.Hour).Unix()
		c.ownerReplica(`UPDATE integration.operations SET request=jsonb_set(request,'{event_time}',to_jsonb($2::bigint)),
			request_hash=sha256(convert_to(jsonb_set(request,'{event_time}',to_jsonb($2::bigint))::text,'UTF8')) WHERE id=$1`, c.capiOps()[0].ID, old)
	}, true)
	checkDenial("dataset binding re-pointed after planning: STALE_BINDING, zero HTTP", "", func(c *capiEnv) { c.disableBinding(c.dsBinding) }, true, "STALE_BINDING", "BLOCKED_POLICY")

	t.Run("billing RESTRICTED never blocks CAPI (BD5)", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		cbxRestrict(t, c.f, mustStripeIngress(t, c.f), c.tenant, c.store)
		c.sweep("capi")
		c.settle()
		if ops := c.capiOps(); len(ops) != 1 || ops[0].State != "SUCCEEDED" {
			t.Fatalf("RESTRICTED store: %+v%s", ops, c.dump())
		}
	})

	t.Run("a fact older than 6 days is never planned; a context older than 7 days does not make an owner eligible; 8-day contexts are purged", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		c.ownerReplica(`UPDATE payments.facts SET received_at=received_at-interval '7 days' WHERE attempt_id=$1`, c.p.result.AttemptID)
		c.sweep("capi")
		c.settle()
		if len(c.capiOps()) != 0 {
			t.Fatal("a 7-day-old fact was planned (event_time > 6 days is rejected by Meta, F14)")
		}
		c2 := newCapiEnv(t, adsOpts{})
		c2.prime("SANDBOX", "SANDBOX")
		c2.ownerReplica(`UPDATE ads.capi_contexts SET captured_at=captured_at-interval '7 days 1 hour' WHERE store_id=$1`, c2.store)
		c2.sweep("capi")
		c2.settle()
		if len(c2.capiOps()) != 0 {
			t.Fatal("a context captured more than 7 days ago still made the owner eligible (§6.4)")
		}
		c2.ownerReplica(`UPDATE ads.capi_contexts SET captured_at=captured_at-interval '2 days' WHERE store_id=$1`, c2.store)
		c2.sweep("capi")
		if c2.contextRows() != 0 {
			t.Fatal("contexts older than 8 days were not purged")
		}
	})

	t.Run("consent without a browser context (no user agent): not eligible", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.prime("SANDBOX", "SANDBOX")
		mustExec(t, c.f.owner, `DELETE FROM ads.capi_contexts WHERE store_id=$1`, c.store)
		c.sweep("capi")
		c.settle()
		if len(c.capiOps()) != 0 || len(c.events()) != 0 {
			t.Fatalf("event sent without a client_user_agent (F14: website events need one): ops=%d", len(c.capiOps()))
		}
	})

	t.Run("CAPI off or no dataset: nothing is planned", func(t *testing.T) {
		c := newCapiEnv(t, adsOpts{})
		c.capture("SANDBOX", "SANDBOX")
		if r := c.consent(true); r.status != 200 {
			t.Fatalf("consent %d", r.status)
		}
		c.sweep("capi")
		c.settle()
		if len(c.capiOps()) != 0 {
			t.Fatal("planned although CAPI is not enabled for the store")
		}
		// the merchant cannot enable CAPI for a dataset binding that is not one of the store's meta_dataset bindings
		if r := c.api("PUT", "/capi", c.token, adsKey(), map[string]any{"enabled": true, "dataset_binding_id": c.adBinding, "test_event_code": "TEST12345"}); r.Status < 400 {
			t.Errorf("CAPI enabled on an ad-account binding: %d", r.Status)
		}
		if r := c.api("PUT", "/capi", c.token, adsKey(), map[string]any{"enabled": true, "dataset_binding_id": randomUUID(), "test_event_code": "TEST12345"}); r.Status < 400 {
			t.Errorf("CAPI enabled on an unknown binding: %d", r.Status)
		}
		if r := c.api("PUT", "/capi", c.token, adsKey(), map[string]any{"enabled": true, "dataset_binding_id": c.dsBinding}); r.Status < 400 {
			t.Errorf("SANDBOX store enabled CAPI without a test_event_code (§4.1 CHECK): %d", r.Status)
		}
	})
}
