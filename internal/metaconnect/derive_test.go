package metaconnect

import (
	"testing"
)

// derive_test.go: pure-unit gate MCH01 (contract meta-connection-health-v1 §3, §11.2): the Derive table covers every §3.2
// branch for both providers, first-match order, the §3.3 closed reason list, and that `unsupported` is never produced.

// allPerms is the union of every §6 permission; a fully-ok Reading grants all of them.
var allPerms = map[string]bool{
	"pages_read_engagement":     true,
	"pages_messaging":           true,
	"pages_manage_engagement":   true,
	"instagram_manage_comments": true,
	"instagram_manage_messages": true,
}

var allTasks = map[string]bool{"MODERATE": true, "MESSAGING": true}
var allFields = map[string]bool{"feed": true, "messages": true}

// cloneSet copies a set so one case's mutation never leaks into another case (or the shared literals).
func cloneSet(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

// good returns a Reading that derives to ok (ok_app_level_assumed for IG dm_session), perm_source graph.
// Every set is a fresh copy: the cases mutate it freely.
func good() Reading {
	return Reading{
		Token:       TokenValid,
		Perms:       cloneSet(allPerms),
		PermSource:  PermSourceGraph,
		Tasks:       cloneSet(allTasks),
		FBFields:    cloneSet(allFields),
		AppReview:   cloneSet(allPerms),
		DMConfirmed: true,
	}
}

func TestDerive(t *testing.T) {
	cases := []struct {
		name                 string
		provider, capability string
		mutate               func(*Reading)
		state, reason        string
	}{
		// ---- rule 1: token invalid -> reauth_required, subcode-mapped reason -----------------------------
		{"invalid 463 expired", "facebook", "read_comment", func(r *Reading) { r.Token, r.Subcode = TokenInvalid, 463 }, "reauth_required", "token_expired"},
		{"invalid 458 revoked", "facebook", "read_comment", func(r *Reading) { r.Token, r.Subcode = TokenInvalid, 458 }, "reauth_required", "token_revoked"},
		{"invalid 460 revoked", "instagram", "private_reply", func(r *Reading) { r.Token, r.Subcode = TokenInvalid, 460 }, "reauth_required", "token_revoked"},
		{"invalid 467 invalid", "facebook", "dm_session", func(r *Reading) { r.Token, r.Subcode = TokenInvalid, 467 }, "reauth_required", "token_invalid"},
		{"invalid other invalid", "facebook", "reply_public", func(r *Reading) { r.Token, r.Subcode = TokenInvalid, 190 }, "reauth_required", "token_invalid"},
		// ---- rule 2: page gone -----------------------------------------------------------------------------
		{"page gone", "facebook", "read_comment", func(r *Reading) { r.Token = TokenPageGone }, "reauth_required", "page_unavailable"},
		// ---- rule 3: unknown token (no probe, no snapshot) ------------------------------------------------
		{"unknown token", "facebook", "read_comment", func(r *Reading) { r.Token = TokenUnknown; r.Perms = nil }, "unknown", "not_probed"},
		// ---- rule 4: missing permission --------------------------------------------------------------------
		{"missing fb read perm", "facebook", "read_comment", func(r *Reading) { delete(r.Perms, "pages_read_engagement") }, "missing_permission", "perm_pages_read_engagement"},
		{"missing fb reply perm", "facebook", "private_reply", func(r *Reading) { delete(r.Perms, "pages_messaging") }, "missing_permission", "perm_pages_messaging"},
		{"missing fb dm perm", "facebook", "dm_session", func(r *Reading) { delete(r.Perms, "pages_messaging") }, "missing_permission", "perm_pages_messaging"},
		{"missing fb public perm", "facebook", "reply_public", func(r *Reading) { delete(r.Perms, "pages_manage_engagement") }, "missing_permission", "perm_pages_manage_engagement"},
		{"missing ig read perm", "instagram", "read_comment", func(r *Reading) { delete(r.Perms, "instagram_manage_comments") }, "missing_permission", "perm_instagram_manage_comments"},
		{"missing ig reply perm", "instagram", "private_reply", func(r *Reading) { delete(r.Perms, "instagram_manage_messages") }, "missing_permission", "perm_instagram_manage_messages"},
		{"missing ig public perm", "instagram", "reply_public", func(r *Reading) { delete(r.Perms, "instagram_manage_comments") }, "missing_permission", "perm_instagram_manage_comments"},
		// ---- rule 5: missing task --------------------------------------------------------------------------
		{"missing moderate task", "facebook", "read_comment", func(r *Reading) { delete(r.Tasks, "MODERATE") }, "missing_task", "task_moderate"},
		{"missing messaging task", "facebook", "private_reply", func(r *Reading) { delete(r.Tasks, "MESSAGING") }, "missing_task", "task_messaging"},
		{"missing ig messaging task", "instagram", "dm_session", func(r *Reading) { delete(r.Tasks, "MESSAGING") }, "missing_task", "task_messaging"},
		// ---- rule 6: subscription field --------------------------------------------------------------------
		{"fb fields unknown", "facebook", "read_comment", func(r *Reading) { r.FBFields = nil }, "unknown", "sub_unread"},
		{"fb fields unknown dm", "facebook", "dm_session", func(r *Reading) { r.FBFields = nil }, "unknown", "sub_unread"},
		{"ig read fields unknown", "instagram", "read_comment", func(r *Reading) { r.FBFields = nil }, "unknown", "sub_unread"},
		{"feed missing", "facebook", "read_comment", func(r *Reading) { delete(r.FBFields, "feed") }, "not_subscribed", "sub_feed"},
		{"ig feed missing", "instagram", "read_comment", func(r *Reading) { delete(r.FBFields, "feed") }, "not_subscribed", "sub_feed"},
		{"messages missing", "facebook", "dm_session", func(r *Reading) { delete(r.FBFields, "messages") }, "not_subscribed", "sub_messages"},
		{"ig dm field assumed present", "instagram", "dm_session", func(r *Reading) { r.FBFields = nil }, "ok", "ok_app_level_assumed"}, // app-level subscription: rule 6 skipped even when unread
		// ---- rule 7: dm_session LC-U11 ---------------------------------------------------------------------
		{"dm not confirmed fb", "facebook", "dm_session", func(r *Reading) { r.DMConfirmed = false }, "unknown", "lc_u11_open"},
		{"dm not confirmed ig", "instagram", "dm_session", func(r *Reading) { r.DMConfirmed = false }, "unknown", "lc_u11_open"},
		// ---- rule 8: standard access (required perm not in app_review) -------------------------------------
		{"standard access read", "facebook", "read_comment", func(r *Reading) { delete(r.AppReview, "pages_read_engagement") }, "review_required", "standard_access"},
		{"standard access reply", "instagram", "private_reply", func(r *Reading) { delete(r.AppReview, "instagram_manage_messages") }, "review_required", "standard_access"},
		{"ig dm no perm skips review", "instagram", "dm_session", func(r *Reading) { r.AppReview = nil }, "ok", "ok_app_level_assumed"},
		// ---- rule 9: ok ------------------------------------------------------------------------------------
		{"ok graph", "facebook", "read_comment", nil, "ok", "ok"},
		{"ok snapshot", "facebook", "read_comment", func(r *Reading) { r.PermSource = PermSourceSnapshot }, "ok", "ok_snapshot"},
		{"ok ig dm app-level", "instagram", "dm_session", nil, "ok", "ok_app_level_assumed"},
		// ---- first-match order -----------------------------------------------------------------------------
		{"invalid beats missing perm", "facebook", "read_comment", func(r *Reading) { r.Token = TokenInvalid; delete(r.Perms, "pages_read_engagement") }, "reauth_required", "token_invalid"},
		{"missing perm beats missing task", "facebook", "read_comment", func(r *Reading) { delete(r.Perms, "pages_read_engagement"); delete(r.Tasks, "MODERATE") }, "missing_permission", "perm_pages_read_engagement"},
		{"missing task beats field", "facebook", "dm_session", func(r *Reading) { delete(r.Tasks, "MESSAGING"); delete(r.FBFields, "messages") }, "missing_task", "task_messaging"},
		{"field beats lc-u11", "facebook", "dm_session", func(r *Reading) { delete(r.FBFields, "messages"); r.DMConfirmed = false }, "not_subscribed", "sub_messages"},
		{"lc-u11 beats standard access", "facebook", "dm_session", func(r *Reading) { r.DMConfirmed = false; delete(r.AppReview, "pages_messaging") }, "unknown", "lc_u11_open"},
		{"standard access beats ok", "facebook", "read_comment", func(r *Reading) { delete(r.AppReview, "pages_read_engagement"); r.PermSource = PermSourceSnapshot }, "review_required", "standard_access"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := good()
			if c.mutate != nil {
				c.mutate(&r)
			}
			state, reason := Derive(c.provider, c.capability, r)
			if state != c.state || reason != c.reason {
				t.Fatalf("Derive(%s,%s) = (%q,%q), want (%q,%q)", c.provider, c.capability, state, reason, c.state, c.reason)
			}
		})
	}
}

func TestDeriveNeverUnsupportedAndClosedReasons(t *testing.T) {
	closed := map[string]bool{
		"ok": true, "ok_snapshot": true, "ok_app_level_assumed": true,
		"token_expired": true, "token_revoked": true, "token_invalid": true, "page_unavailable": true,
		"not_probed": true, "task_moderate": true, "task_messaging": true, "sub_feed": true,
		"sub_messages": true, "sub_unread": true, "lc_u11_open": true, "standard_access": true,
	}
	readings := []Reading{
		good(),
		{Token: TokenUnknown, Perms: allPerms, PermSource: PermSourceGraph, Tasks: allTasks, FBFields: allFields, AppReview: allPerms, DMConfirmed: true},
		{Token: TokenValid, Perms: allPerms, PermSource: PermSourceGraph, Tasks: allTasks, FBFields: nil, AppReview: allPerms, DMConfirmed: true},
		{Token: TokenValid, Perms: allPerms, PermSource: PermSourceSnapshot, Tasks: allTasks, FBFields: allFields, AppReview: nil, DMConfirmed: false},
	}
	for _, provider := range []string{"facebook", "instagram"} {
		for _, capability := range Capabilities(provider) {
			for _, r := range readings {
				state, reason := Derive(provider, capability, r)
				if state == "unsupported" {
					t.Fatalf("Derive(%s,%s) produced the reserved state unsupported", provider, capability)
				}
				if state == "" || reason == "" {
					t.Fatalf("Derive(%s,%s) produced an empty state/reason", provider, capability)
				}
				if !closed[reason] {
					t.Fatalf("Derive(%s,%s) reason %q is outside the §3.3 closed list", provider, capability, reason)
				}
				if capability == "read_comment" && reason == "sub_messages" || capability == "dm_session" && reason == "sub_feed" {
					t.Fatalf("Derive(%s,%s) mapped a field to the wrong reason code %q", provider, capability, reason)
				}
			}
		}
	}
	if st, re := Derive("bogus", "read_comment", good()); st != "" || re != "" {
		t.Fatalf("Derive for an unknown provider = (%q,%q), want empty", st, re)
	}
	if st, re := Derive("facebook", "bogus", good()); st != "" || re != "" {
		t.Fatalf("Derive for an unknown capability = (%q,%q), want empty", st, re)
	}
}

func TestCapabilitiesVocabulary(t *testing.T) {
	for _, provider := range []string{"facebook", "instagram"} {
		got := Capabilities(provider)
		want := []string{"read_comment", "private_reply", "dm_session", "reply_public"}
		if len(got) != len(want) {
			t.Fatalf("Capabilities(%s) = %v, want %v", provider, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Capabilities(%s)[%d] = %q, want %q", provider, i, got[i], want[i])
			}
		}
	}
	if Capabilities("bogus") != nil {
		t.Fatal("Capabilities for an unknown provider must be nil")
	}
}
