package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

func studioInputPaths(h *brwHarness) (string, string) {
	input := "/v1/admin/stores/" + h.lp.f.storeA1 + "/live-sessions/" + h.session + "/input"
	return input, input + "/prepared"
}

func studioInputNull(t *testing.T, body []byte) {
	t.Helper()
	if !bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		t.Fatal("optional input projection was not JSON null")
	}
}

func studioInputPublic(t *testing.T, raw []byte, fields ...string) map[string]any {
	t.Helper()
	for _, forbidden := range []string{"project_lma", "unit.livekit.cloud", "endpoint_identity", "credential_version", "publisher_identity", "media_binding_id", "api_secret", `"token"`} {
		if bytes.Contains(bytes.ToLower(raw), []byte(strings.ToLower(forbidden))) {
			t.Fatalf("public input projection contains private field %q", forbidden)
		}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil || len(result) != len(fields) {
		t.Fatalf("public input projection shape: fields=%d err=%v", len(result), err)
	}
	for _, field := range fields {
		if _, ok := result[field]; !ok {
			t.Fatalf("public input projection missing %q", field)
		}
	}
	return result
}

func studioInputRead(t *testing.T, h *brwHarness, handler http.Handler, path, bearer string, want int) []byte {
	t.Helper()
	w := brwHTTPCall(t, handler, http.MethodGet, path, bearer, "", "", want)
	if w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Location") != "" || w.Header().Get("Set-Cookie") != "" {
		t.Fatal("input read response lost private no-store or redirected")
	}
	if want >= 400 {
		studioInputSafeError(t, w.Body.Bytes())
	}
	return w.Body.Bytes()
}

func studioInputSafeError(t *testing.T, body []byte) {
	t.Helper()
	for _, forbidden := range []string{"project_lma", "unit.livekit.cloud", "endpoint_identity", "credential_version", "publisher_identity", "api_secret", `"token"`, "eyJ"} {
		if bytes.Contains(bytes.ToLower(body), []byte(strings.ToLower(forbidden))) {
			t.Fatalf("denied input read exposed private data %q", forbidden)
		}
	}
}

func TestStudioInputPreparedAndACL(t *testing.T) {
	h := brwRegistered(t)
	input, preparedPath := studioInputPaths(h)
	disabled := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner})
	for _, path := range []string{input, preparedPath} {
		studioInputRead(t, h, disabled, path, h.logins.a, 404)
	}
	active := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
	studioInputNull(t, studioInputRead(t, h, active, input, h.logins.a, 200))
	prepared := studioInputPublic(t, studioInputRead(t, h, active, preparedPath, h.logins.a, 200),
		"authorization_id", "session_version", "start_before", "environment", "destinations")
	if prepared["authorization_id"] != h.input.AuthorizationID || prepared["session_version"] != float64(1) || prepared["environment"] != "MOCK" {
		t.Fatal("runtime prepared candidate is not bound to current authorization/version")
	}
	if _, err := time.Parse(time.RFC3339Nano, prepared["start_before"].(string)); err != nil {
		t.Fatal("prepared candidate start_before is not an absolute timestamp")
	}
	destinations, ok := prepared["destinations"].([]any)
	if !ok || len(destinations) == 0 {
		t.Fatal("prepared candidate omitted bounded destinations")
	}
	for _, destination := range destinations {
		item, ok := destination.(map[string]any)
		if !ok || len(item) != 2 || item["ordinal"] == nil || item["provider"] == nil {
			t.Fatal("prepared destination exposed non-public fields")
		}
	}
	legacy, err := studioGet(h.lp, h.logins.a, h.lp.f.storeA1, h.session)
	if err != nil || legacy.Prepared != nil {
		t.Fatalf("legacy prepared route offered runtime-input authorization: %v", err)
	}
	// The SECURITY DEFINER result may carry private mapping pins. HTTP must
	// strip them, while the actual runtime role alone can execute both ABIs.
	var private []byte
	err = platform.WithScope(context.Background(), h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		if _, err := tx.Exec(context.Background(), `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
			return err
		}
		return tx.QueryRow(context.Background(), `SELECT live.read_studio_input_prepared($1,$2,$3)`,
			tokenHash(h.logins.a), h.lp.f.storeA1, h.session).Scan(&private)
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if json.Unmarshal(private, &envelope) != nil || len(envelope) != 4 || envelope["prepared"] == nil ||
		envelope["project_id"] != "project_lma" || envelope["endpoint_identity"] != "https://unit.livekit.cloud" || envelope["credential_version"] != float64(1) {
		t.Fatal("private prepared binding envelope changed")
	}
	for _, fn := range []string{"live.read_studio_input(bytea,uuid,uuid)", "live.read_studio_input_prepared(bytea,uuid,uuid)"} {
		for _, role := range []string{"commerce_runtime", "commerce_media_worker", "commerce_media_executor", "commerce_media_registrar", "commerce_worker"} {
			var allowed bool
			if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT has_function_privilege($1,$2,'EXECUTE')`, role, fn).Scan(&allowed); err != nil || allowed != (role == "commerce_runtime") {
				t.Fatalf("input read ABI role=%s allowed=%t err=%v", role, allowed, err)
			}
		}
		var public bool
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_proc p,aclexplode(p.proacl) a WHERE p.oid=$1::regprocedure AND a.grantee=0 AND a.privilege_type='EXECUTE')`, fn).Scan(&public); err != nil || public {
			t.Fatalf("input read ABI has PUBLIC EXECUTE: %t err=%v", public, err)
		}
	}
	for _, role := range []string{"commerce_runtime", "commerce_media_worker", "commerce_media_executor"} {
		for _, table := range []string{"live.prepared_media_input_runtime_profiles", "live.media_input_custody", "live.media_execution_state"} {
			var allowed bool
			if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT has_table_privilege($1,$2,'SELECT')`, role, table).Scan(&allowed); err != nil || allowed {
				t.Fatalf("private input table SELECT role=%s table=%s allowed=%t err=%v", role, table, allowed, err)
			}
		}
	}
}

func TestStudioInputRuntimeStateAndJointStop(t *testing.T) {
	h := brwRegistered(t)
	input, preparedPath := studioInputPaths(h)
	active := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
	plan, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, t04Key("studio-input-start"), h.input)
	if err != nil {
		t.Fatal(err)
	}
	studioInputNull(t, studioInputRead(t, h, active, preparedPath, h.logins.a, 200))
	check := func(state string, held, canStop bool) {
		t.Helper()
		projected := studioInputPublic(t, studioInputRead(t, h, active, input, h.logins.a, 200),
			"attempt_id", "state", "admission_closed", "close_reason", "cleanup_held", "can_stop", "updated_at")
		if projected["attempt_id"] != plan.AttemptID || projected["state"] != state || projected["cleanup_held"] != held ||
			projected["can_stop"] != canStop || projected["admission_closed"] != false || projected["close_reason"] != "" {
			t.Fatal("runtime input projection lost state, admission, or independent Stop liability")
		}
		if _, err := time.Parse(time.RFC3339Nano, projected["updated_at"].(string)); err != nil {
			t.Fatal("runtime input updated_at is not an absolute timestamp")
		}
	}
	check("UNISSUED", false, true)
	if _, err := bicReserve(context.Background(), h.bicHarness, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1,
		t04Key("studio-input-grant"), live.MediaInputReserveInput{SessionID: h.session, AttemptID: plan.AttemptID, ExpectedSessionVersion: 1}); err != nil {
		t.Fatal(err)
	}
	check("RESERVED", false, true)
	// The read-only principal can observe the safe state but cannot Stop.
	mustExec(t, h.lp.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read')`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.limited)
	readOnly := studioInputPublic(t, studioInputRead(t, h, active, input, h.lp.limitedToken, 200),
		"attempt_id", "state", "admission_closed", "close_reason", "cleanup_held", "can_stop", "updated_at")
	if readOnly["attempt_id"] != plan.AttemptID || readOnly["can_stop"] != false {
		t.Fatal("read-only merchant acquired Stop authority")
	}
	// Projection-only fixture: a terminal Egress must not erase a still-issued
	// input liability, even after input cleanup is held and authority is revoked.
	mustExec(t, h.lp.f.owner, `INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,authorization_id,operation_id,project_id,
		wire_reserved_at,wire_generation,egress_id,resource_state,transport_status,ended_at_ns)
		VALUES($1,$2,$3,$4,$5,$6,'project_lma',clock_timestamp(),1,'EG_studio_input','TERMINAL','EGRESS_COMPLETE',1)`,
		plan.AttemptID, h.lp.f.tenantA, h.lp.f.storeA1, h.session, h.input.AuthorizationID, plan.OperationID)
	mustExec(t, h.lp.f.owner, `UPDATE live.media_input_custody SET input_cleanup_held_at=clock_timestamp(),input_cleanup_hold_reason='deadline' WHERE attempt_id=$1`, plan.AttemptID)
	check("RESERVED", true, true)
	if _, err := h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,'operator_revoke')`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID); err != nil {
		t.Fatal(err)
	}
	check("RESERVED", true, true)

	// Conversely, Egress wire liability alone keeps Stop available after an
	// unissued input has been closed. This owner seed tests only projection logic.
	egress := brwRegistered(t)
	other, err := brwPlan(context.Background(), egress, egress.lp.f.runtime, egress.logins.a, egress.lp.f.storeA1,
		t04Key("studio-egress-only"), egress.input)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, egress.lp.f.owner, `UPDATE live.media_input_custody SET state='CLOSED',admission_closed_at=clock_timestamp(),close_reason='permission_lost' WHERE attempt_id=$1`, other.AttemptID)
	otherPath, _ := studioInputPaths(egress)
	otherHandler := httpapi.NewHandler(egress.lp.f.runtime, httpapi.Options{Live: egress.planner, BrowserInput: egress.runtime})
	closed := studioInputPublic(t, studioInputRead(t, egress, otherHandler, otherPath, egress.logins.a, 200),
		"attempt_id", "state", "admission_closed", "close_reason", "cleanup_held", "can_stop", "updated_at")
	if closed["can_stop"] != false || closed["state"] != "CLOSED" || closed["admission_closed"] != true {
		t.Fatal("closed input without Egress liability still offered Stop")
	}
	mustExec(t, egress.lp.f.owner, `INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,authorization_id,operation_id,project_id,
		wire_reserved_at,wire_generation,egress_id,resource_state,transport_status)
		VALUES($1,$2,$3,$4,$5,$6,'project_lma',clock_timestamp(),1,'EG_studio_egress','OBSERVED','EGRESS_ACTIVE')`,
		other.AttemptID, egress.lp.f.tenantA, egress.lp.f.storeA1, egress.session, egress.input.AuthorizationID, other.OperationID)
	withEgress := studioInputPublic(t, studioInputRead(t, egress, otherHandler, otherPath, egress.logins.a, 200),
		"attempt_id", "state", "admission_closed", "close_reason", "cleanup_held", "can_stop", "updated_at")
	if withEgress["can_stop"] != true || withEgress["state"] != "CLOSED" || withEgress["close_reason"] != "permission_lost" {
		t.Fatal("Egress-only wire liability did not preserve Stop")
	}
}

func TestStudioInputCandidateFencesAndStrictRoutes(t *testing.T) {
	for _, candidate := range []struct {
		name   string
		mutate func(*testing.T, *brwHarness)
	}{
		{"expired", func(t *testing.T, h *brwHarness) {
			mustExec(t, h.lp.f.owner, `UPDATE live.prepared_media_authorizations SET start_before=clock_timestamp()-interval '1 second' WHERE id=$1`, h.input.AuthorizationID)
		}},
		{"revoked", func(t *testing.T, h *brwHarness) {
			if _, err := h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,'operator_revoke')`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID); err != nil {
				t.Fatal(err)
			}
		}},
		{"disabled-binding", func(t *testing.T, h *brwHarness) {
			mustExec(t, h.lp.f.owner, `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.media)
		}},
		{"changed-session-version", func(t *testing.T, h *brwHarness) {
			mustExec(t, h.lp.f.owner, `UPDATE live.sessions SET version=version+1 WHERE id=$1`, h.session)
		}},
		{"changed-aspect", func(t *testing.T, h *brwHarness) {
			mustExec(t, h.lp.f.owner, `UPDATE live.programs SET aspect_ratio='9:16' WHERE session_id=$1`, h.session)
		}},
		{"used", func(t *testing.T, h *brwHarness) {
			if _, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, t04Key("studio-used"), h.input); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			h := brwRegistered(t)
			candidate.mutate(t, h)
			_, preparedPath := studioInputPaths(h)
			handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
			studioInputNull(t, studioInputRead(t, h, handler, preparedPath, h.logins.a, 200))
		})
	}
	t.Run("private-mapping-mismatch", func(t *testing.T) {
		h := brwRegistered(t)
		config := lmeConfig()
		config.Endpoint = "https://other.livekit.cloud"
		mismatch, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{
			ProjectID: "project_lma", CredentialVersion: 1, Config: config,
			Transport: lmeTransport("127.0.0.1:1"), BrowserURL: brwHTTPURL,
		}})
		if err != nil {
			t.Fatal(err)
		}
		_, preparedPath := studioInputPaths(h)
		handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: mismatch})
		studioInputRead(t, h, handler, preparedPath, h.logins.a, 409)
		if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1} {
			t.Fatalf("mapping mismatch wrote media artifacts: %v", got)
		}
	})
	t.Run("legacy-non-input-is-null", func(t *testing.T) {
		h := lmpSetup(t, false)
		if _, err := h.start(t04Key("studio-legacy-start")); err != nil {
			t.Fatal(err)
		}
		runtime, _ := brwPlanNoIOMap(t)
		handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: runtime})
		path := "/v1/admin/stores/" + h.lp.f.storeA1 + "/live-sessions/" + h.session + "/input"
		studioInputNull(t, brwHTTPCall(t, handler, http.MethodGet, path, h.lp.token, "", "", 200).Body.Bytes())
	})
	t.Run("marker-zero-and-private-routing", func(t *testing.T) {
		h, _, _ := bicStarted(t)
		runtime, _ := brwPlanNoIOMap(t)
		handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: runtime})
		base := "/v1/admin/stores/" + h.lp.f.storeA1 + "/live-sessions/" + h.session + "/input"
		for _, path := range []string{base, base + "/prepared"} {
			studioInputRead(t, nil, handler, path, h.logins.a, 409)
		}
	})
	t.Run("strict-http", func(t *testing.T) {
		h := brwRegistered(t)
		input, preparedPath := studioInputPaths(h)
		handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
		for _, path := range []string{input, preparedPath} {
			for _, bad := range []struct {
				method, path, bearer, key, body string
				status                          int
			}{
				{"GET", path + "?x=1", h.logins.a, "", "", 422},
				{"GET", path + "?", h.logins.a, "", "", 422},
				{"GET", path, h.logins.a, t04Key("studio-read-key"), "", 422},
				{"GET", path, h.logins.a, "", `{}`, 422},
				{"POST", path, h.logins.a, "", "", 405},
				{"GET", path, "", "", "", 401},
				{"GET", strings.Replace(path, h.lp.f.storeA1, h.lp.f.storeA2, 1), h.logins.a, "", "", 404},
				{"GET", path, h.lp.limitedToken, "", "", 403},
			} {
				w := brwHTTPCall(t, handler, bad.method, bad.path, bad.bearer, bad.key, bad.body, bad.status)
				studioInputSafeError(t, w.Body.Bytes())
			}
		}
	})
}

func TestStudioInputReadAuthorizationFreshness(t *testing.T) {
	h := brwRegistered(t)
	for _, function := range []string{"live.read_studio_input", "live.read_studio_input_prepared"} {
		// Wrong principal cannot reuse the scoped runtime transaction to read a
		// private projection, even if its token is otherwise valid.
		err := platform.WithScope(context.Background(), h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := tx.Exec(context.Background(), `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
				return err
			}
			var raw []byte
			return tx.QueryRow(context.Background(), `SELECT `+function+`($1::bytea,$2::uuid,$3::uuid)`, tokenHash(h.lp.otherToken), h.lp.f.storeA1, h.session).Scan(&raw)
		})
		if sqlState(err) != "MP403" && sqlState(err) != "MP401" {
			t.Fatalf("%s accepted foreign principal in scoped runtime transaction: %v", function, err)
		}
		// Use a real clock: a session valid at BEGIN must be rejected before
		// its projection can be returned after expiry.
		mustExec(t, h.lp.f.owner, `UPDATE identity.sessions SET expires_at=clock_timestamp()+interval '1 second' WHERE token_hash=$1`, tokenHash(h.lp.token))
		entered := false
		err = platform.WithScope(context.Background(), h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			entered = true
			if _, err := tx.Exec(context.Background(), `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
				return err
			}
			if _, e := tx.Exec(context.Background(), `SELECT pg_sleep(1.15)`); e != nil {
				return e
			}
			var raw []byte
			return tx.QueryRow(context.Background(), `SELECT `+function+`($1::bytea,$2::uuid,$3::uuid)`, tokenHash(h.lp.token), h.lp.f.storeA1, h.session).Scan(&raw)
		})
		if !entered || (!errors.Is(err, platform.ErrUnauthorized) && sqlState(err) != "MP401") {
			t.Fatalf("%s did not fence expiry after BEGIN: entered=%t err=%v", function, entered, err)
		}
	}
}
