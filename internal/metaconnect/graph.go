package metaconnect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// graph.go: the three Graph reads/writes of a connect beyond the shared exchange. Every call goes through metaoauth.Graph
// (host allowlist, no redirect, bounded body, token in the Authorization header or JSON body, never a URL).

const (
	// listFields asks /me/accounts for what the pick list needs; `tasks` proves the user can MESSAGE and MODERATE the Page.
	// Source: https://developers.facebook.com/docs/graph-api/reference/user/accounts/ (retrieved 2026-10-01).
	listFields   = "id,name,tasks,instagram_business_account{id,username}"
	pickFields   = "id,name,tasks,access_token,instagram_business_account{id,username}"
	maxPages     = 25 // pick_list CHECK: <= 25 Pages and <= 16 KiB
	accountEdges = 2  // /me/accounts is read at most 2 pages of 25
	nameRunes    = 60
	maxScopes    = 64
)

// Permissions the Login for Business configuration must have granted (config "直播SaaS主页连接", 2026-10-01). The Facebook set is
// required for every connect; the Instagram set only when an Instagram account is linked to the Page. The SQL (0095
// meta_connect_finish) re-checks the same two lists.
var (
	fbPermissions = []string{"pages_show_list", "pages_manage_metadata", "pages_read_engagement", "pages_messaging"}
	igPermissions = []string{"instagram_basic", "instagram_manage_comments", "instagram_manage_messages"}
)

// pageEntry is one pick-list row (ids and display names only, never a token). Missing lists what to re-grant: permission
// names and the Page tasks task_messaging / task_moderate; an entry with an empty Missing is pickable as Facebook-only, with an
// empty IGMissing as Facebook + Instagram.
type pageEntry struct {
	PageID     string   `json:"page_id"`
	Name       string   `json:"name"`
	IGID       string   `json:"ig_id,omitempty"`
	IGUsername string   `json:"ig_username,omitempty"`
	Missing    []string `json:"missing"`
	IGMissing  []string `json:"ig_missing"`
}

type accountDoc struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Token string   `json:"access_token"`
	Tasks []string `json:"tasks"`
	IG    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"instagram_business_account"`
}

func numeric(s string) bool {
	if len(s) < 1 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// shortName keeps a display name printable and short (the pick list is size-bounded).
func shortName(s string) string {
	if !utf8.ValidString(s) {
		return ""
	}
	out := make([]rune, 0, nameRunes)
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if len(out) == nameRunes {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

func missingOf(have map[string]bool, need []string) []string {
	out := []string{}
	for _, n := range need {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}

func hasTask(tasks []string, want string) bool {
	for _, t := range tasks {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

// entryOf builds the pick row of one account from the user's granted permissions.
func entryOf(a accountDoc, granted map[string]bool) (pageEntry, bool) {
	if !numeric(a.ID) {
		return pageEntry{}, false
	}
	e := pageEntry{PageID: a.ID, Name: shortName(a.Name), Missing: missingOf(granted, fbPermissions), IGMissing: []string{}}
	if !hasTask(a.Tasks, "MESSAGING") {
		e.Missing = append(e.Missing, "task_messaging")
	}
	if !hasTask(a.Tasks, "MODERATE") {
		e.Missing = append(e.Missing, "task_moderate")
	}
	if numeric(a.IG.ID) {
		e.IGID, e.IGUsername = a.IG.ID, shortName(a.IG.Username)
		e.IGMissing = missingOf(granted, igPermissions)
	}
	return e, true
}

// listPages returns the user's pick list (up to maxPages rows) and the granted permission names. Any failed call fails the
// whole callback (no partial pick list).
func (s *Service) listPages(ctx context.Context, user []byte) ([]pageEntry, []string, error) {
	scopes, err := s.graph.Granted(ctx, user, maxScopes)
	if err != nil || len(scopes) == 0 {
		return nil, nil, ErrConnectFailed
	}
	granted := make(map[string]bool, len(scopes))
	for _, p := range scopes {
		granted[p] = true
	}
	pages := []pageEntry{}
	err = s.graph.Edge(ctx, "me/accounts", url.Values{"fields": {listFields}, "limit": {"25"}}, user, accountEdges, func(raw json.RawMessage) bool {
		var a accountDoc
		if json.Unmarshal(raw, &a) == nil {
			if e, ok := entryOf(a, granted); ok {
				pages = append(pages, e)
			}
		}
		return len(pages) < maxPages
	})
	if err != nil {
		return nil, nil, ErrConnectFailed
	}
	return pages, scopes, nil
}

// pageToken re-reads /me/accounts with the user token and returns the picked Page's access token. It re-verifies the MESSAGING
// and MODERATE tasks and the Instagram account (a different one than the pick list showed is a failed connect). The caller
// zeroes the result.
func (s *Service) pageToken(ctx context.Context, user []byte, pageID, igID string) ([]byte, error) {
	var token []byte
	err := s.graph.Edge(ctx, "me/accounts", url.Values{"fields": {pickFields}, "limit": {"25"}}, user, accountEdges, func(raw json.RawMessage) bool {
		var a accountDoc
		if json.Unmarshal(raw, &a) != nil || a.ID != pageID {
			return true
		}
		if hasTask(a.Tasks, "MESSAGING") && hasTask(a.Tasks, "MODERATE") && (igID == "" || a.IG.ID == igID) && validToken(a.Token) {
			token = []byte(a.Token)
		}
		a.Token = ""
		return false
	})
	if err != nil || token == nil {
		return nil, ErrConnectFailed
	}
	return token, nil
}

// validToken: 1..4096 visible ASCII bytes (the metareply.validPageToken shape; Seal enforces it again).
func validToken(t string) bool {
	if len(t) < 1 || len(t) > 4096 {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] < 0x21 || t[i] > 0x7e {
			return false
		}
	}
	return true
}

// subscribe makes Meta deliver the Page's `feed` webhook field (comments) to the app: POST /{page_id}/subscribed_apps with the
// Page token. Instagram comments / live_comments arrive through the app-level instagram subscription, not here.
// Source: https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/ (retrieved 2026-10-01).
// Retry rule: not retried here; the merchant retries the whole pick (the call is idempotent at Meta). A failure aborts the
// bind before any database write, so the connect is never half-enabled.
func (s *Service) subscribe(ctx context.Context, pageID string, pageToken []byte) error {
	rep, err := s.graph.Do(ctx, http.MethodPost, pageID+"/subscribed_apps", url.Values{"subscribed_fields": {"feed"}}, pageToken, nil)
	if err != nil || !rep.OK() {
		return ErrConnectFailed
	}
	var ok struct {
		Success bool `json:"success"`
	}
	if json.Unmarshal(rep.Body, &ok) != nil || !ok.Success {
		return ErrConnectFailed
	}
	return nil
}
