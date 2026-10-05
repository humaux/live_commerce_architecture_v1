// Purpose: the pure derivation of one binding's capability state from a probe Reading (contract meta-connection-health-v1 §3).
// One first-match-wins rule set (§3.2) shared by every reader (SnapshotReader §7.2, the probe's recording §4.3) so the §6
// vocabulary of contracts/live-console-v1.md can never drift between readers. `unsupported` is reserved by §6 and never
// produced in v1 (§3.2).
// Depends on: nothing but the frozen §6 capability requirement table (live-console-v1 §6) and the §3.3 closed reason list.
// Used by: internal/metaconnect/capability.go (SnapshotReader), internal/integrations/metareply/probe.go (probe recording),
// cmd/api (the CapabilityReader swap); tests internal/metaconnect/derive_test.go (MCH01).
// Invariants: first match wins; reason codes are exactly the §3.3 list; a required permission missing from app_review is
// review_required (Standard Access) unless it is missing entirely (missing_permission, rule 4 beats rule 8).
// Status: MOCK (the same Derive runs against the fake Graph and the connect snapshot; LIVE probe is MCH12, NOT_RUN).

package metaconnect

import "strings"

// Token and permission-source vocabulary of a Reading (contract §3.1). The probe and SnapshotReader produce these;
// Derive never invents them.
const (
	TokenValid    = "valid"
	TokenInvalid  = "invalid"
	TokenPageGone = "page_gone"
	TokenUnknown  = "unknown"

	PermSourceGraph    = "graph"
	PermSourceSnapshot = "snapshot"
)

// Reading is the set of facts one probe (or the connect snapshot) observed about a single binding — the sole input to
// Derive. A nil set means "unknown" (not probed); an empty non-nil set means "known, and empty". Subcode is the Graph
// error subcode of a 190 (463 expired, 458/460 revoked, 467 invalid); it is meaningful only when Token is TokenInvalid.
type Reading struct {
	Token       string          // TokenValid | TokenInvalid | TokenPageGone | TokenUnknown
	Subcode     int             // Graph subcode when Token == TokenInvalid
	Perms       map[string]bool // granted permissions (nil = unknown)
	PermSource  string          // PermSourceGraph | PermSourceSnapshot
	Tasks       map[string]bool // granted tasks, snapshot minus demoted (§4.4); nil = unknown
	FBFields    map[string]bool // Page webhook fields from P3 (nil = unknown)
	AppReview   map[string]bool // permissions with Advanced Access (nil/empty = none: every grant is Standard Access)
	DMConfirmed bool            // COMMERCE_META_DM_RECEIVER_CONFIRMED (LC-U11 closed), rule 7
}

// Capabilities returns the live-console §6 capability vocabulary for a provider, in a stable order, or nil for an
// unknown provider. Every binding has exactly these rows (4 per binding, §2).
func Capabilities(provider string) []string {
	switch provider {
	case "facebook", "instagram":
		return []string{"read_comment", "private_reply", "dm_session", "reply_public"}
	}
	return nil
}

// requirement is one (provider, capability) row of the frozen §6 table: the permission, task and Page webhook field that
// must all be present for state=ok. appLevel marks the one IG case whose subscription is app-level (not readable with a
// Page token) and is therefore assumed present (rule 6, MCH-OPEN-4): IG dm_session needs app-level `messages`.
type requirement struct {
	perm     string // required permission ("" = none)
	task     string // required task, MODERATE|MESSAGING ("" = none)
	field    string // required Page webhook field, feed|messages ("" = none)
	appLevel bool   // field is an IG app-level subscription: assumed present, reason ok_app_level_assumed
}

// requirements is the frozen live-console §6 table ("Requires (all of them set state=ok)"). The IG column is read
// per-binding: where §6 names an IG permission it replaces the Facebook one; dm_session names no IG permission (its IG
// difference is the app-level `messages` subscription), so an IG dm_session binding requires no permission.
var requirements = map[string]map[string]requirement{
	"facebook": {
		"read_comment":  {perm: "pages_read_engagement", task: "MODERATE", field: "feed"},
		"private_reply": {perm: "pages_messaging", task: "MESSAGING"},
		"dm_session":    {perm: "pages_messaging", task: "MESSAGING", field: "messages"},
		"reply_public":  {perm: "pages_manage_engagement", task: "MODERATE"},
	},
	"instagram": {
		"read_comment":  {perm: "instagram_manage_comments", task: "MODERATE", field: "feed"},
		"private_reply": {perm: "instagram_manage_messages", task: "MESSAGING"},
		"dm_session":    {task: "MESSAGING", field: "messages", appLevel: true},
		"reply_public":  {perm: "instagram_manage_comments", task: "MODERATE"},
	},
}

// Derive applies the §3.2 rule (first match wins) to one (provider, capability) reading and returns the §6 state and the
// §3.3 reason code. It never returns the reserved state "unsupported". A provider/capability outside the §6 table (only
// possible via a programming error) returns ("", ""); every caller only feeds the Capabilities vocabulary.
func Derive(provider, capability string, r Reading) (state, reason string) {
	req, ok := requirements[provider][capability]
	if !ok {
		return "", ""
	}
	switch r.Token {
	case TokenInvalid:
		switch r.Subcode {
		case 463:
			return "reauth_required", "token_expired"
		case 458, 460:
			return "reauth_required", "token_revoked"
		default:
			return "reauth_required", "token_invalid"
		}
	case TokenPageGone:
		return "reauth_required", "page_unavailable"
	case TokenUnknown:
		return "unknown", "not_probed"
	}
	// token == valid. A nil set is unreachable here (the probe and snapshot always fill perms/tasks once the token is
	// known-valid), but fail safe: a nil permission/task set is read as "the required one is not present" (rule 4/5), a
	// nil fb_fields set keeps its own explicit branch (rule 6).
	if req.perm != "" && !r.Perms[req.perm] {
		return "missing_permission", "perm_" + req.perm
	}
	if req.task != "" && !r.Tasks[req.task] {
		return "missing_task", "task_" + strings.ToLower(req.task)
	}
	if req.field != "" && !req.appLevel {
		if r.FBFields == nil {
			return "unknown", "sub_unread"
		}
		if !r.FBFields[req.field] {
			return "not_subscribed", "sub_" + req.field
		}
	}
	if capability == "dm_session" && !r.DMConfirmed {
		return "unknown", "lc_u11_open"
	}
	if req.perm != "" && !r.AppReview[req.perm] {
		return "review_required", "standard_access"
	}
	if req.appLevel {
		return "ok", "ok_app_level_assumed"
	}
	if r.PermSource == PermSourceSnapshot {
		return "ok", "ok_snapshot"
	}
	return "ok", "ok"
}
